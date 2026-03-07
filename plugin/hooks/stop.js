#!/usr/bin/env node
'use strict';

const fs = require('fs');
const os = require('os');
const readline = require('readline');

const lib = require('./lib');

function extractTextContent(content) {
  if (typeof content === 'string') {
    return content;
  }

  if (!Array.isArray(content)) {
    return '';
  }

  let out = '';
  for (const part of content) {
    if (!part || typeof part !== 'object') {
      continue;
    }

    if (part.type === 'text' && typeof part.text === 'string') {
      out += part.text;
    }
  }

  return out;
}

function expandTranscriptPath(transcriptPath) {
  if (typeof transcriptPath !== 'string' || transcriptPath === '') {
    return transcriptPath;
  }

  if (!transcriptPath.startsWith('~')) {
    return transcriptPath;
  }

  const home = os.homedir();
  if (!home) {
    return transcriptPath;
  }

  if (transcriptPath === '~') {
    return home;
  }

  const separator = transcriptPath[1];
  if (separator === '/' || separator === '\\') {
    return `${home}${transcriptPath.slice(1)}`;
  }

  return transcriptPath;
}

async function parseTranscript(transcriptPath) {
  const expandedPath = expandTranscriptPath(transcriptPath);
  if (!expandedPath) {
    return { lastUser: '', lastAssistant: '' };
  }

  return new Promise((resolve) => {
    let lastUser = '';
    let lastAssistant = '';

    const stream = fs.createReadStream(expandedPath, {
      encoding: 'utf8',
      highWaterMark: 1024 * 1024,
    });

    stream.on('error', (error) => {
      console.error(`[stop] Failed to read transcript: ${error.message}`);
      resolve({ lastUser, lastAssistant });
    });

    const rl = readline.createInterface({
      input: stream,
      crlfDelay: Infinity,
    });

    rl.on('line', (line) => {
      if (!line || !line.trim()) {
        return;
      }

      let item = null;
      try {
        item = JSON.parse(line);
      } catch (error) {
        return;
      }

      const messageRole =
        typeof item.type === 'string'
          ? item.type.toLowerCase()
          : item.message && typeof item.message.role === 'string'
          ? item.message.role.toLowerCase()
          : '';

      const messageText =
        item.message && Object.prototype.hasOwnProperty.call(item.message, 'content')
          ? extractTextContent(item.message.content)
          : '';

      if (messageRole === 'user') {
        lastUser = messageText;
      } else if (messageRole === 'assistant') {
        lastAssistant = messageText;
      }
    });

    rl.on('close', () => {
      resolve({ lastUser, lastAssistant });
    });
  });
}

async function handleStop(ctx, input) {
  console.error(`[stop] Raw input: ${String(ctx.RawInput || '')}`);

  let sessionResult;
  try {
    sessionResult = await lib.requestGet(
      `/api/sessions?claudeSessionId=${encodeURIComponent(ctx.SessionID)}`
    );
  } catch (error) {
    return '';
  }

  const sessionID = Number(sessionResult && sessionResult.id);
  if (!Number.isFinite(sessionID)) {
    return '';
  }

  const transcriptPath =
    typeof input.transcript_path === 'string'
      ? input.transcript_path
      : typeof input.TranscriptPath === 'string'
      ? input.TranscriptPath
      : '';

  const { lastUser, lastAssistant } = await parseTranscript(transcriptPath);

  console.error(`[stop] Transcript path: ${transcriptPath}`);
  console.error(`[stop] Last user message length: ${String(lastUser).length}`);
  console.error(`[stop] Last assistant message length: ${String(lastAssistant).length}`);
  if (String(lastAssistant).length > 300) {
    console.error(`[stop] Last assistant preview: ${String(lastAssistant).slice(0, 300)}...`);
  }

  console.error(
    `[stop] Requesting summary for session ${sessionID} (transcript: ${
      transcriptPath !== ''
    })`
  );

  try {
    await lib.requestPost(`/sessions/${sessionID}/summarize`, {
      lastUserMessage: lastUser,
      lastAssistantMessage: lastAssistant,
    });
  } catch (error) {
    console.error(`[stop] Warning: summary request failed: ${error.message}`);
  }

  return '';
}

(async () => {
  await lib.RunHook('Stop', handleStop);
})();
