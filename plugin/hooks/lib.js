#!/usr/bin/env node
'use strict';

const crypto = require('crypto');
const path = require('path');

function getServerURL() {
  // ENGRAM_URL may include a path (e.g. http://server:37777/mcp for MCP transport).
  // Hooks use REST API endpoints at the server root (/api/...), so we extract just the origin.
  const customURL = process.env.ENGRAM_URL;
  if (customURL && customURL.trim() !== '') {
    try {
      const parsed = new URL(customURL.trim());
      return `${parsed.protocol}//${parsed.host}`;
    } catch {
      // If URL parsing fails, use as-is but strip trailing path
      return customURL.trim().replace(/\/[^/]*$/, '');
    }
  }

  const host = process.env.ENGRAM_WORKER_HOST || '127.0.0.1';
  const port = process.env.ENGRAM_WORKER_PORT || '37777';
  return `http://${host}:${port}`;
}

function isInternalHook() {
  return process.env.ENGRAM_INTERNAL === '1';
}

function ProjectIDWithName(cwd) {
  const resolvedPath = path.resolve(cwd || '');
  const dirName = path.basename(resolvedPath);
  const hash = crypto.createHash('sha256').update(resolvedPath).digest('hex');
  const shortHash = hash.slice(0, 6);
  return `${dirName}_${shortHash}`;
}

function buildRequestHeaders(includeJsonBody = false) {
  const headers = {};
  const token = process.env.ENGRAM_API_TOKEN;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  if (includeJsonBody) {
    headers['Content-Type'] = 'application/json';
  }

  return headers;
}

function resolveRequestURL(endpoint) {
  const base = getServerURL().replace(/\/+$/, '');
  if (!endpoint) {
    return base;
  }
  if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
    return endpoint;
  }
  const normalizedEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
  return `${base}${normalizedEndpoint}`;
}

function readAllStdin() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => {
      data += chunk;
    });
    process.stdin.on('end', () => {
      resolve(data);
    });
  });
}

function writeResponse(hookName, additionalContext) {
  try {
    const response = { continue: true };
    if (typeof additionalContext === 'string' && additionalContext !== '') {
      response.hookSpecificOutput = {
        hookEventName: hookName,
        additionalContext,
      };
    }

    process.stdout.write(`${JSON.stringify(response)}\n`);
  } catch (error) {
    // Never throw during response output.
  }
}

async function requestGet(endpoint, timeoutMs = 10000) {
  return request('GET', endpoint, undefined, timeoutMs);
}

async function requestPost(endpoint, body, timeoutMs = 10000) {
  return request('POST', endpoint, body, timeoutMs);
}

async function request(method, endpoint, body, timeoutMs = 10000) {
  const url = resolveRequestURL(endpoint);
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const headers = buildRequestHeaders(body !== undefined);
    const response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    });

    const text = await response.text();
    if (!response.ok) {
      throw new Error(`HTTP ${response.status} ${response.statusText}: ${text}`);
    }

    if (!text) {
      return {};
    }

    return JSON.parse(text);
  } finally {
    clearTimeout(timer);
  }
}

async function RunHook(hookName, handler) {
  if (isInternalHook()) {
    writeResponse(hookName);
    return;
  }

  let rawInput = '';
  let input = {};

  try {
    rawInput = await readAllStdin();
    if (rawInput && rawInput.trim()) {
      input = JSON.parse(rawInput);
    }
  } catch (error) {
    console.error(`[engram] Failed to parse hook input JSON: ${error.message}`);
  }

  const context = {
    SessionID: typeof input.session_id === 'string' ? input.session_id : '',
    CWD: typeof input.cwd === 'string' ? input.cwd : '',
    PermissionMode: typeof input.permission_mode === 'string' ? input.permission_mode : '',
    HookEventName: typeof input.hook_event_name === 'string' ? input.hook_event_name : hookName,
    Project: ProjectIDWithName(typeof input.cwd === 'string' ? input.cwd : ''),
    RawInput: rawInput,
  };

  try {
    const additionalContext =
      typeof handler === 'function' ? await handler(context, input) : '';
    writeResponse(hookName, additionalContext);
  } catch (error) {
    console.error(`[engram] ${hookName} hook failed: ${error.message}`);
    writeResponse(hookName);
  }
}

async function RunStatuslineHook(handler, offlineRenderer) {
  try {
    const rawInput = await readAllStdin();
    let input = null;

    if (rawInput && rawInput.trim()) {
      try {
        input = JSON.parse(rawInput);
      } catch (error) {
        console.error(`[engram] Failed to parse statusline input JSON: ${error.message}`);
      }
    }

    const output = await handler(input);
    console.log(typeof output === 'undefined' ? '' : output);
  } catch (error) {
    console.error(`[engram] statusline hook failed: ${error.message}`);
    const offline =
      typeof offlineRenderer === 'function'
        ? offlineRenderer()
        : '[engram] offline';
    console.log(offline);
  }
}

module.exports = {
  getServerURL,
  ProjectIDWithName,
  requestGet,
  requestPost,
  RunHook,
  RunStatuslineHook,
  writeResponse,
};
