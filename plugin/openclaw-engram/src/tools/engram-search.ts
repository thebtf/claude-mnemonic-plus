/**
 * engram_search + memory_search — search engram memory.
 */

import { z } from 'zod';
import { Type } from '@sinclair/typebox';
import type { EngramRestClient } from '../client.js';
import type { PluginConfig } from '../config.js';
import { resolveIdentity } from '../identity.js';
import { formatContext } from '../context/formatter.js';
import type { AnyAgentTool, OpenClawPluginToolContext } from '../types/openclaw.js';

const SearchParamsSchema = z.object({
  query: z.string().min(1),
  limit: z.number().int().positive().max(50).optional(),
  scope: z.enum(['personal', 'shared', 'all']).optional(),
});

const searchParameters = Type.Object({
  query: Type.String({ description: 'Search query for engram memory' }),
  limit: Type.Optional(Type.Number({ description: 'Maximum number of results (default: 10)' })),
  scope: Type.Optional(Type.Union([
    Type.Literal('personal'), Type.Literal('shared'), Type.Literal('all'),
  ], { description: 'Search scope: personal (agent only), shared (project+global), all (default)' })),
});

function createSearchTool(
  name: string,
  ctx: OpenClawPluginToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): AnyAgentTool {
  return {
    name,
    description:
      'Search engram persistent memory for observations relevant to your query. ' +
      'Use this before starting work on a topic to surface prior decisions, patterns, and changes. ' +
      'Results include project, global, and agent-private observations.',
    parameters: searchParameters,

    async execute(_toolCallId: string, params: Record<string, unknown>): Promise<string> {
      const parsed = SearchParamsSchema.safeParse(params);
      if (!parsed.success) {
        return `Invalid parameters: ${parsed.error.message}`;
      }

      if (!client.isAvailable()) {
        return 'engram is currently unreachable — memory search unavailable';
      }

      const identity = resolveIdentity(ctx.agentId ?? '', ctx.workspaceDir);
      const project = config.project ?? identity.projectId;

      // TODO: Pass scope filter to server when dedicated endpoint supports it.
      // Currently all searches include agent-scoped results via agent_id.
      const response = await client.searchContext({
        project,
        query: parsed.data.query,
        cwd: ctx.workspaceDir,
        agent_id: ctx.agentId,
      });

      if (!response) {
        return 'engram search failed — server returned no response';
      }

      const allObservations = Array.isArray(response.observations) ? response.observations : [];
      if (allObservations.length === 0) {
        return 'No relevant observations found for this query.';
      }

      const limit = parsed.data.limit ?? allObservations.length;
      const observations = allObservations.slice(0, limit);

      const { context } = formatContext(observations, { tokenBudget: config.tokenBudget });
      return context || `Found ${observations.length} observation(s) but could not format context.`;
    },
  };
}

export function createEngramSearchTool(
  ctx: OpenClawPluginToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): AnyAgentTool {
  return createSearchTool('engram_search', ctx, client, config);
}

export function createMemorySearchTool(
  ctx: OpenClawPluginToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): AnyAgentTool {
  return createSearchTool('memory_search', ctx, client, config);
}
