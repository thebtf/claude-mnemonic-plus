#!/usr/bin/env node
'use strict';

const lib = require('./lib');

const skipTools = {
  Task: true,
  TaskOutput: true,
  Glob: true,
  ListDir: true,
  LS: true,
  KillShell: true,
  AskUserQuestion: true,
  EnterPlanMode: true,
  ExitPlanMode: true,
  Skill: true,
  SlashCommand: true,
  Read: true,
  Grep: true,
  WebSearch: true,
};

async function handlePostToolUse(ctx, input) {
  const toolName =
    typeof input.tool_name === 'string'
      ? input.tool_name
      : typeof input.ToolName === 'string'
      ? input.ToolName
      : '';

  if (toolName && skipTools[toolName]) {
    return '';
  }

  console.error(`[post-tool-use] ${toolName}`);

  try {
    await lib.requestPost('/api/sessions/observations', {
      claudeSessionId: ctx.SessionID,
      project: ctx.Project,
      tool_name: toolName,
      tool_input: input.tool_input,
      tool_response: input.tool_response,
      cwd: ctx.CWD,
    });
  } catch (error) {
    console.error(
      `[post-tool-use] Warning: failed to notify worker: ${error.message}`
    );
  }

  return '';
}

(async () => {
  await lib.RunHook('PostToolUse', handlePostToolUse);
})();
