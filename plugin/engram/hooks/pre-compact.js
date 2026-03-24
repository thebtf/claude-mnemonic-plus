#!/usr/bin/env node
'use strict';

const lib = require('./lib');
const fs = require('fs');
const path = require('path');

async function handlePreCompact(ctx, input) {
  // Phase 1: Discovery — log what fields are available + write to file
  const inputKeys = input ? Object.keys(input) : [];
  const ctxKeys = ctx ? Object.keys(ctx) : [];

  const report = {
    timestamp: new Date().toISOString(),
    ctx_keys: ctxKeys,
    input_keys: inputKeys,
    has_transcript_path: !!(input.transcript_path || input.TranscriptPath),
    transcript_path: input.transcript_path || input.TranscriptPath || null,
    input_sample: {},
    ctx_sample: {}
  };

  // Capture first-level values (truncated for safety)
  for (const key of inputKeys) {
    const val = input[key];
    report.input_sample[key] = typeof val === 'string'
      ? val.substring(0, 200)
      : typeof val;
  }
  for (const key of ctxKeys) {
    const val = ctx[key];
    report.ctx_sample[key] = typeof val === 'string'
      ? val.substring(0, 200)
      : typeof val;
  }

  // Write to file so agent can read it
  const logPath = path.join(__dirname, '..', '..', '..', '.agent', 'pre-compact-discovery.json');
  try {
    fs.mkdirSync(path.dirname(logPath), { recursive: true });
    fs.writeFileSync(logPath, JSON.stringify(report, null, 2));
    console.error(`[pre-compact] Discovery written to ${logPath}`);
  } catch (e) {
    console.error(`[pre-compact] Failed to write log: ${e.message}`);
  }

  // Also log to stderr for immediate visibility
  console.error(`[pre-compact] ctx keys: ${ctxKeys.join(', ')}`);
  console.error(`[pre-compact] input keys: ${inputKeys.join(', ')}`);

  if (report.has_transcript_path) {
    console.error(`[pre-compact] transcript_path FOUND: ${report.transcript_path}`);
  } else {
    console.error(`[pre-compact] transcript_path NOT in input`);
  }

  return '';
}

(async () => {
  await lib.RunHook('PreCompact', handlePreCompact);
})();
