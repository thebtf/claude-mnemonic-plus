/**
 * OpenClaw Engram Plugin
 *
 * Connects OpenClaw's AI gateway to engram's persistent memory server via REST API.
 * Provides:
 *   - Session-level static context injection (appendSystemContext)
 *   - Per-turn dynamic context search (prependContext)
 *   - Automatic self-learning via tool event ingestion
 *   - Transcript backfill on compaction / session end
 *   - Agent tools: engram_search, engram_remember, engram_decisions
 *   - Slash commands: /memory, /remember
 */

import type { OpenClawPluginDefinition, OpenClawPluginApi } from './types/openclaw.js';
import { parseConfig } from './config.js';
import { EngramRestClient } from './client.js';

import { handleSessionStart } from './hooks/session-start.js';
import { handleBeforePromptBuild } from './hooks/before-prompt-build.js';
import { handleAfterToolCall } from './hooks/after-tool-call.js';
import { handleBeforeCompaction } from './hooks/before-compaction.js';
import { handleSessionEnd } from './hooks/session-end.js';

import { buildEngramSearchTool } from './tools/engram-search.js';
import { buildEngramRememberTool } from './tools/engram-remember.js';
import { buildEngramDecisionsTool } from './tools/engram-decisions.js';

import { buildMemoryCommand } from './commands/memory.js';
import { buildRememberCommand } from './commands/remember.js';

// ---------------------------------------------------------------------------
// Plugin definition
// ---------------------------------------------------------------------------

const plugin: OpenClawPluginDefinition = {
  name: 'engram',
  version: '0.1.0',
  kind: 'memory',

  async initialize(api: OpenClawPluginApi, rawConfig: Record<string, unknown>): Promise<void> {
    // Parse and validate config — throws ZodError with a clear message on misconfiguration
    const config = parseConfig(rawConfig);
    const client = new EngramRestClient(config);

    api.log('info', `[engram] initializing — server: ${config.url}`);

    // ------------------------------------------------------------------
    // Hooks
    // ------------------------------------------------------------------

    api.registerHook('session_start', (event) =>
      handleSessionStart(event, client, config),
    );

    api.registerHook('before_prompt_build', (event) =>
      handleBeforePromptBuild(event, client, config),
    );

    api.registerHook('after_tool_call', (event) => {
      handleAfterToolCall(event, client, config);
    });

    api.registerHook('before_compaction', (event) => {
      handleBeforeCompaction(event, client, config);
    });

    api.registerHook('session_end', (event) => {
      handleSessionEnd(event, client, config);
    });

    // ------------------------------------------------------------------
    // Tools
    // ------------------------------------------------------------------

    api.registerTool(buildEngramSearchTool(client, config));
    api.registerTool(buildEngramRememberTool(client, config));
    api.registerTool(buildEngramDecisionsTool(client, config));

    // ------------------------------------------------------------------
    // Commands
    // ------------------------------------------------------------------

    api.registerCommand(buildMemoryCommand(client, config));
    api.registerCommand(buildRememberCommand(client, config));

    api.log('info', '[engram] plugin initialized successfully');
  },
};

export default plugin;

// Named exports for consumers that prefer explicit imports
export { EngramRestClient } from './client.js';
export { parseConfig, getJsonSchema } from './config.js';
export { resolveIdentity, projectIDFromWorkspace } from './identity.js';
export { formatContext } from './context/formatter.js';
export { AvailabilityTracker } from './availability.js';
