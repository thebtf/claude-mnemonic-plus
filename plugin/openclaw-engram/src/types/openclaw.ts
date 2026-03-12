/**
 * Assumed OpenClaw Plugin SDK interfaces.
 *
 * These are locally defined since no SDK package is available.
 * The shapes are inferred from the plugin spec and architectural analysis.
 * They will need adjustment once the real SDK is available.
 */

// ---------------------------------------------------------------------------
// Core SDK types
// ---------------------------------------------------------------------------

export interface OpenClawPluginApi {
  /** Register a lifecycle hook handler. */
  registerHook<E extends HookEventName>(
    event: E,
    handler: HookHandler<HookEventMap[E]>,
  ): void;

  /** Register a native agent tool. */
  registerTool(tool: ToolDefinition): void;

  /** Register a slash command. */
  registerCommand(command: CommandDefinition): void;

  /** Log a message at the given level. */
  log(level: 'debug' | 'info' | 'warn' | 'error', message: string): void;
}

export interface OpenClawPluginDefinition {
  name: string;
  version: string;
  kind: 'memory' | 'tool' | 'formatter' | 'generic';
  /**
   * Called once when the plugin is loaded. The plugin should register all
   * hooks, tools, and commands here.
   */
  initialize(api: OpenClawPluginApi, config: Record<string, unknown>): void | Promise<void>;
}

// ---------------------------------------------------------------------------
// Hook event types
// ---------------------------------------------------------------------------

export type HookEventName =
  | 'session_start'
  | 'before_prompt_build'
  | 'after_tool_call'
  | 'before_compaction'
  | 'session_end';

/** Base fields present in every hook event. */
export interface BaseHookEvent {
  /** Unique ID for this agent session. */
  agentId: string;
  /** Optional workspace directory for the session. */
  workspaceDir?: string;
  /** ISO timestamp when the event fired. */
  timestamp: string;
}

export interface SessionStartEvent extends BaseHookEvent {
  /** Optional user-supplied prompt that initiated the session. */
  initialPrompt?: string;
}

export interface BeforePromptBuildEvent extends BaseHookEvent {
  /** The user's current prompt text. */
  prompt: string;
  /** Conversation turn index (0-based). */
  turnIndex: number;
}

export interface AfterToolCallEvent extends BaseHookEvent {
  /** Name of the tool that was called. */
  toolName: string;
  /** Tool input arguments (already JSON-serialized or object). */
  toolInput: unknown;
  /** Tool result (already JSON-serialized or object). */
  toolResult: unknown;
  /** Whether the tool call succeeded. */
  success: boolean;
  /** Error message if the tool call failed. */
  error?: string;
}

export interface BeforeCompactionEvent extends BaseHookEvent {
  /**
   * Recent conversation messages before compaction.
   * Shape is assumed — may be an array of {role, content} objects.
   */
  messages: ConversationMessage[];
  /** Reason for compaction (context limit, manual, etc.). */
  reason?: string;
}

export interface SessionEndEvent extends BaseHookEvent {
  /** Final messages in the session. */
  messages?: ConversationMessage[];
  /** Reason for session end. */
  reason?: 'normal' | 'error' | 'timeout' | 'manual';
}

export interface ConversationMessage {
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
}

// ---------------------------------------------------------------------------
// Hook event → type map
// ---------------------------------------------------------------------------

export interface HookEventMap {
  session_start: SessionStartEvent;
  before_prompt_build: BeforePromptBuildEvent;
  after_tool_call: AfterToolCallEvent;
  before_compaction: BeforeCompactionEvent;
  session_end: SessionEndEvent;
}

// ---------------------------------------------------------------------------
// Hook handler return types
// ---------------------------------------------------------------------------

/** Return value for before_prompt_build: context to inject. */
export interface PromptBuildResult {
  /** Text prepended before each user prompt turn (per-turn dynamic context). */
  prependContext?: string;
  /** Text appended to the system prompt (static session-level context). */
  appendSystemContext?: string;
}

/** Return value for session_start. */
export interface SessionStartResult {
  /** Text to append to the system prompt for the session. */
  appendSystemContext?: string;
}

/** Generic hook return — void or a result object. */
export type HookResult =
  | void
  | PromptBuildResult
  | SessionStartResult
  | undefined;

export type HookHandler<E> = (event: E) => HookResult | Promise<HookResult>;

// ---------------------------------------------------------------------------
// Tool types
// ---------------------------------------------------------------------------

export interface ToolDefinition {
  name: string;
  description: string;
  /** JSON Schema for the tool's input parameters. */
  parameters: Record<string, unknown>;
  /** Called when the agent invokes this tool. */
  execute(params: Record<string, unknown>, context: ToolContext): Promise<ToolExecuteResult>;
}

export interface ToolContext {
  agentId: string;
  workspaceDir?: string;
}

export interface ToolExecuteResult {
  content: string;
  /** Whether the tool call was successful. */
  success: boolean;
}

// ---------------------------------------------------------------------------
// Command types
// ---------------------------------------------------------------------------

export interface CommandDefinition {
  /** The slash command trigger, e.g. "/memory". */
  command: string;
  description: string;
  /** Optional usage hint. */
  usage?: string;
  /** Called when the user runs this command. */
  execute(args: string[], context: CommandContext): Promise<CommandResult>;
}

export interface CommandContext {
  agentId: string;
  workspaceDir?: string;
}

export interface CommandResult {
  /** Output to display to the user. */
  output: string;
}
