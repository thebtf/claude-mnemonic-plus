/**
 * engram_decisions — native SDK tool for querying architectural decisions stored in engram.
 *
 * Specifically targets observations of type "decision" with formatted output.
 */

import { z } from 'zod';
import type { EngramRestClient } from '../client.js';
import type { PluginConfig } from '../config.js';
import { resolveIdentity } from '../identity.js';
import type { ToolDefinition, ToolContext, ToolExecuteResult } from '../types/openclaw.js';
import type { Observation } from '../client.js';

const DecisionsParamsSchema = z.object({
  query: z
    .string()
    .min(1)
    .describe('Query to search for relevant architectural decisions'),
});

type DecisionsParams = z.infer<typeof DecisionsParamsSchema>;

/**
 * Build the engram_decisions tool definition.
 */
export function buildEngramDecisionsTool(
  client: EngramRestClient,
  config: PluginConfig,
): ToolDefinition {
  return {
    name: 'engram_decisions',
    description:
      'Query architectural decisions and design choices stored in engram. ' +
      'Use this before making architectural decisions to surface prior reasoning and constraints.',
    parameters: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Query to search for relevant architectural decisions',
        },
      },
      required: ['query'],
    },

    async execute(
      params: Record<string, unknown>,
      context: ToolContext,
    ): Promise<ToolExecuteResult> {
      const parsed = DecisionsParamsSchema.safeParse(params);
      if (!parsed.success) {
        return {
          success: false,
          content: `Invalid parameters: ${parsed.error.message}`,
        };
      }
      return runDecisions(parsed.data, context, client, config);
    },
  };
}

async function runDecisions(
  params: DecisionsParams,
  context: ToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): Promise<ToolExecuteResult> {
  if (!client.isAvailable()) {
    return {
      success: false,
      content: 'engram is currently unreachable — decisions query unavailable',
    };
  }

  const identity = resolveIdentity(context.agentId, context.workspaceDir);
  const project = config.project ?? identity.projectId;

  const response = await client.searchContext({
    project,
    query: params.query,
    cwd: context.workspaceDir,
    agent_id: context.agentId,
  });

  if (!response) {
    return {
      success: false,
      content: 'engram decisions query failed — server returned no response',
    };
  }

  const observations = (Array.isArray(response.observations) ? response.observations : []).filter(
    (obs) => obs.type?.toLowerCase() === 'decision',
  );

  if (observations.length === 0) {
    return {
      success: true,
      content: 'No architectural decisions found for this query.',
    };
  }

  const content = formatDecisions(observations);
  return { success: true, content };
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

function formatDecisions(decisions: Observation[]): string {
  let out = '# Relevant Architectural Decisions\n\n';
  decisions.forEach((d, i) => {
    const score = typeof d.similarity === 'number' ? ` [relevance: ${d.similarity.toFixed(2)}]` : '';
    const scopeTag = d.scope === 'global' ? ' [GLOBAL]' : '';
    out += `## ${i + 1}. ${d.title}${scopeTag}${score}\n`;
    const facts = Array.isArray(d.facts) ? d.facts : [];
    if (facts.length > 0) {
      out += 'Rationale:\n';
      for (const fact of facts) {
        if (typeof fact === 'string' && fact) out += `- ${fact}\n`;
      }
      out += '\n';
    }
    if (d.narrative) out += `${d.narrative}\n\n`;
  });
  return out.trimEnd();
}
