/**
 * engram_search — native SDK tool that lets the agent query engram memory.
 *
 * The project scope is automatically set to the agentId so agents can only
 * search their own project's observations by default.
 */

import { z } from 'zod';
import type { EngramRestClient } from '../client.js';
import type { PluginConfig } from '../config.js';
import { resolveIdentity } from '../identity.js';
import { formatContext } from '../context/formatter.js';
import type { ToolDefinition, ToolContext, ToolExecuteResult } from '../types/openclaw.js';

const SearchParamsSchema = z.object({
  query: z.string().min(1).describe('Search query for engram memory'),
  limit: z
    .number()
    .int()
    .positive()
    .max(50)
    .optional()
    .describe('Maximum number of results (default: 10)'),
});

type SearchParams = z.infer<typeof SearchParamsSchema>;

/**
 * Build the engram_search tool definition.
 */
export function buildEngramSearchTool(
  client: EngramRestClient,
  config: PluginConfig,
): ToolDefinition {
  return {
    name: 'engram_search',
    description:
      'Search engram persistent memory for observations relevant to your query. ' +
      'Use this before starting work on a topic to surface prior decisions, patterns, and changes.',
    parameters: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Search query for engram memory',
        },
        limit: {
          type: 'number',
          description: 'Maximum number of results (default: 10)',
        },
      },
      required: ['query'],
    },

    async execute(
      params: Record<string, unknown>,
      context: ToolContext,
    ): Promise<ToolExecuteResult> {
      const parsed = SearchParamsSchema.safeParse(params);
      if (!parsed.success) {
        return {
          success: false,
          content: `Invalid parameters: ${parsed.error.message}`,
        };
      }
      return runSearch(parsed.data, context, client, config);
    },
  };
}

async function runSearch(
  params: SearchParams,
  context: ToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): Promise<ToolExecuteResult> {
  if (!client.isAvailable()) {
    return {
      success: false,
      content: 'engram is currently unreachable — memory search unavailable',
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
      content: 'engram search failed — server returned no response',
    };
  }

  const observations = Array.isArray(response.observations)
    ? response.observations
    : [];

  if (observations.length === 0) {
    return {
      success: true,
      content: 'No relevant observations found for this query.',
    };
  }

  const { context: formatted } = formatContext(observations, {
    tokenBudget: config.tokenBudget,
  });

  return {
    success: true,
    content: formatted || `Found ${observations.length} observation(s) but could not format context.`,
  };
}
