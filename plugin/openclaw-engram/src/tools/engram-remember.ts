/**
 * engram_remember — native SDK tool for storing observations in engram memory.
 *
 * The project scope is automatically set to the agentId.
 */

import { z } from 'zod';
import type { EngramRestClient, BulkImportRequest } from '../client.js';
import type { PluginConfig } from '../config.js';
import { resolveIdentity } from '../identity.js';
import type { ToolDefinition, ToolContext, ToolExecuteResult } from '../types/openclaw.js';

const CONTENT_MAX_CHARS = 900;

const RememberParamsSchema = z.object({
  title: z.string().min(1).describe('Short descriptive title for the observation'),
  content: z.string().min(1).describe('Content/narrative to remember'),
  type: z
    .enum(['decision', 'feature', 'change', 'refactor', 'discovery', 'context'])
    .default('context')
    .describe('Observation type'),
  scope: z
    .enum(['project', 'global'])
    .default('project')
    .describe('Scope: project-local or global'),
  tags: z
    .array(z.string())
    .optional()
    .describe('Optional tags for the observation'),
});

type RememberParams = z.infer<typeof RememberParamsSchema>;

/**
 * Build the engram_remember tool definition.
 */
export function buildEngramRememberTool(
  client: EngramRestClient,
  config: PluginConfig,
): ToolDefinition {
  return {
    name: 'engram_remember',
    description:
      'Store an observation in engram persistent memory. ' +
      'Use this to record decisions, discoveries, patterns, or important context for future sessions.',
    parameters: {
      type: 'object',
      properties: {
        title: {
          type: 'string',
          description: 'Short descriptive title for the observation',
        },
        content: {
          type: 'string',
          description: 'Content/narrative to remember (max 900 chars)',
        },
        type: {
          type: 'string',
          enum: ['decision', 'feature', 'change', 'refactor', 'discovery', 'context'],
          description: 'Observation type',
          default: 'context',
        },
        scope: {
          type: 'string',
          enum: ['project', 'global'],
          description: 'Scope: project-local or global',
          default: 'project',
        },
        tags: {
          type: 'array',
          items: { type: 'string' },
          description: 'Optional tags for the observation',
        },
      },
      required: ['title', 'content'],
    },

    async execute(
      params: Record<string, unknown>,
      context: ToolContext,
    ): Promise<ToolExecuteResult> {
      const parsed = RememberParamsSchema.safeParse(params);
      if (!parsed.success) {
        return {
          success: false,
          content: `Invalid parameters: ${parsed.error.message}`,
        };
      }
      return runRemember(parsed.data, context, client, config);
    },
  };
}

async function runRemember(
  params: RememberParams,
  context: ToolContext,
  client: EngramRestClient,
  config: PluginConfig,
): Promise<ToolExecuteResult> {
  if (!client.isAvailable()) {
    return {
      success: false,
      content: 'engram is currently unreachable — memory store unavailable',
    };
  }

  const identity = resolveIdentity(context.agentId, context.workspaceDir);
  const project = config.project ?? identity.projectId;

  const content = params.content.length > CONTENT_MAX_CHARS
    ? params.content.slice(0, CONTENT_MAX_CHARS)
    : params.content;

  const observation: BulkImportRequest = {
    title: params.title,
    content,
    type: params.type,
    project,
    scope: params.scope,
    tags: params.tags,
  };

  const response = await client.bulkImport([observation]);

  if (!response) {
    return {
      success: false,
      content: 'engram store failed — server returned no response',
    };
  }

  if (response.imported > 0) {
    return {
      success: true,
      content: `Stored: "${params.title}" (type: ${params.type}, scope: ${params.scope})`,
    };
  }

  if (response.skipped > 0) {
    return {
      success: true,
      content: `Observation skipped (likely a near-duplicate): "${params.title}"`,
    };
  }

  const errMsg = response.errors?.join(', ') ?? 'unknown error';
  return {
    success: false,
    content: `Failed to store observation: ${errMsg}`,
  };
}
