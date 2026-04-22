const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const lib = require('./lib');
const { handleStop } = require('./stop');

function makeTranscriptFile(lines) {
  const filePath = path.join(
    os.tmpdir(),
    `engram-stop-hook-test-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}.jsonl`
  );
  fs.writeFileSync(filePath, `${lines.join('\n')}\n`, 'utf8');
  return filePath;
}

test('handleStop does not early-return when numeric session lookup fails', async () => {
  const claudeSessionID = 'stop-lookup-fail-session';
  const getCalls = [];
  const postCalls = [];

  const originalRequestGet = lib.requestGet;
  const originalRequestPost = lib.requestPost;
  const originalRequestUpload = lib.requestUpload;

  lib.requestGet = async (endpoint) => {
    getCalls.push(endpoint);
    if (endpoint === '/api/health') return {};
    if (endpoint.startsWith('/api/sessions?claudeSessionId=')) {
      throw new Error('session not found');
    }
    if (endpoint === `/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`) {
      return { injections: [] };
    }
    if (endpoint === '/api/observations?limit=100&offset=0') {
      return { observations: [] };
    }
    throw new Error(`Unexpected GET endpoint in test: ${endpoint}`);
  };

  lib.requestPost = async (endpoint, body) => {
    postCalls.push({ endpoint, body });
    return {};
  };

  lib.requestUpload = async (endpoint) => {
    throw new Error(`Unexpected upload endpoint in test: ${endpoint}`);
  };

  try {
    await handleStop(
      {
        SessionID: claudeSessionID,
        RawInput: '',
        Project: '',
      },
      {}
    );

    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${claudeSessionID}/outcome`),
      'Expected outcome endpoint to be called even when DB lookup fails'
    );

    assert.ok(
      getCalls.includes(`/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`),
      'Expected Claude-session-keyed injections endpoint to be called'
    );

    assert.equal(
      postCalls.some((call) => call.endpoint.includes('/summarize')),
      false,
      'Should skip summarize when numeric DB session ID is unavailable'
    );
    assert.equal(
      postCalls.some((call) => call.endpoint.includes('/extract-learnings')),
      false,
      'Should skip extract-learnings when numeric DB session ID is unavailable'
    );
  } finally {
    lib.requestGet = originalRequestGet;
    lib.requestPost = originalRequestPost;
    lib.requestUpload = originalRequestUpload;
    lib.clearSessionSignals(claudeSessionID);
  }
});

test('handleStop uses Claude session ID for injections and mark-cited endpoints', async () => {
  const claudeSessionID = 'stop-claude-keyed-session';
  const numericSessionID = 123;

  const transcriptPath = makeTranscriptFile([
    JSON.stringify({
      type: 'assistant',
      message: { content: 'engram__search used due to missing injected context' },
    }),
  ]);

  const getCalls = [];
  const postCalls = [];

  const originalRequestGet = lib.requestGet;
  const originalRequestPost = lib.requestPost;
  const originalRequestUpload = lib.requestUpload;

  lib.requestGet = async (endpoint) => {
    getCalls.push(endpoint);
    if (endpoint === '/api/health') return {};
    if (endpoint.startsWith('/api/sessions?claudeSessionId=')) {
      return { id: numericSessionID };
    }
    if (endpoint === `/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`) {
      return {
        injections: [
          {
            observation_id: 7,
            title: 'A title that does not need citation for this test',
            facts: [],
          },
        ],
      };
    }
    if (endpoint === '/api/observations?limit=100&offset=0') {
      return { observations: [] };
    }
    throw new Error(`Unexpected GET endpoint in test: ${endpoint}`);
  };

  lib.requestPost = async (endpoint, body) => {
    postCalls.push({ endpoint, body });
    return {};
  };

  lib.requestUpload = async () => {
    throw new Error('requestUpload should not be called in this test');
  };

  try {
    await handleStop(
      {
        SessionID: claudeSessionID,
        RawInput: '',
        Project: 'engram',
      },
      { transcript_path: transcriptPath }
    );

    assert.ok(
      getCalls.includes(`/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`),
      'Expected Claude-session-keyed injections endpoint'
    );
    assert.equal(
      getCalls.includes(`/api/sessions/${numericSessionID}/injections`),
      false,
      'Should not call numeric-session-keyed injections endpoint'
    );

    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${encodeURIComponent(claudeSessionID)}/mark-cited`),
      'Expected Claude-session-keyed mark-cited endpoint'
    );
    assert.equal(
      postCalls.some((call) => call.endpoint === `/api/sessions/${numericSessionID}/mark-cited`),
      false,
      'Should not call numeric-session-keyed mark-cited endpoint'
    );

    const insufficientCall = postCalls.find(
      (call) => call.endpoint === '/api/observations/feedback/insufficient-injection'
    );
    assert.ok(insufficientCall, 'Expected insufficient-injection feedback call');
    assert.equal(insufficientCall.body.session_id, claudeSessionID);

    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${numericSessionID}/summarize`),
      'Expected summarize to keep numeric DB session ID'
    );
    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${numericSessionID}/extract-learnings`),
      'Expected extract-learnings to keep numeric DB session ID'
    );
  } finally {
    lib.requestGet = originalRequestGet;
    lib.requestPost = originalRequestPost;
    lib.requestUpload = originalRequestUpload;
    lib.clearSessionSignals(claudeSessionID);
    fs.unlinkSync(transcriptPath);
  }
});

test('handleStop does not crash when transcript_path is missing', async () => {
  const claudeSessionID = 'stop-missing-transcript-session';
  const numericSessionID = 456;
  const missingTranscriptPath = path.join(
    os.tmpdir(),
    'engram-worktree-like-slug',
    `${claudeSessionID}.jsonl`
  );
  const getCalls = [];
  const postCalls = [];
  const uploadCalls = [];

  const originalRequestGet = lib.requestGet;
  const originalRequestPost = lib.requestPost;
  const originalRequestUpload = lib.requestUpload;

  lib.requestGet = async (endpoint) => {
    getCalls.push(endpoint);
    if (endpoint === '/api/health') return {};
    if (endpoint.startsWith('/api/sessions?claudeSessionId=')) {
      return { id: numericSessionID };
    }
    if (endpoint === `/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`) {
      return { injections: [] };
    }
    if (endpoint === '/api/observations?limit=100&offset=0') {
      return { observations: [] };
    }
    throw new Error(`Unexpected GET endpoint in test: ${endpoint}`);
  };

  lib.requestPost = async (endpoint, body) => {
    postCalls.push({ endpoint, body });
    return {};
  };

  lib.requestUpload = async (endpoint, body) => {
    uploadCalls.push({ endpoint, body });
    return {};
  };

  try {
    await assert.doesNotReject(async () => {
      await handleStop(
        {
          SessionID: claudeSessionID,
          RawInput: '',
          Project: 'engram',
        },
        { transcript_path: missingTranscriptPath }
      );
    });

    assert.ok(
      getCalls.includes(`/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`),
      'Expected stop hook to continue past transcript read failure'
    );
    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${numericSessionID}/summarize`),
      'Expected summarize endpoint to still be called'
    );
    assert.ok(
      postCalls.some((call) => call.endpoint === `/api/sessions/${claudeSessionID}/outcome`),
      'Expected outcome endpoint to still be called'
    );
    assert.equal(uploadCalls.length, 0, 'Should not attempt transcript upload when file is missing');
  } finally {
    lib.requestGet = originalRequestGet;
    lib.requestPost = originalRequestPost;
    lib.requestUpload = originalRequestUpload;
    lib.clearSessionSignals(claudeSessionID);
  }
});

test('handleStop falls back to canonical transcript path resolved by session id', async () => {
  const claudeSessionID = 'stop-canonical-fallback-session';
  const numericSessionID = 789;
  const fakeHome = fs.mkdtempSync(path.join(os.tmpdir(), 'engram-stop-home-'));
  const canonicalDir = path.join(fakeHome, '.claude', 'projects', 'D--Dev-engram');
  const canonicalTranscriptPath = path.join(canonicalDir, `${claudeSessionID}.jsonl`);
  const missingTranscriptPath = path.join(
    fakeHome,
    '.claude',
    'projects',
    'D--Dev-aimux--claude-worktrees-aimux-8-schema-hardening',
    `${claudeSessionID}.jsonl`
  );
  const getCalls = [];
  const postCalls = [];
  const uploadCalls = [];

  fs.mkdirSync(canonicalDir, { recursive: true });
  fs.writeFileSync(
    canonicalTranscriptPath,
    `${JSON.stringify({ type: 'user', message: { content: 'Need fallback path' } })}\n${JSON.stringify({ type: 'assistant', message: { content: 'Fallback assistant text from canonical path' } })}\n`,
    'utf8'
  );

  const originalHomeDir = os.homedir;
  const originalRequestGet = lib.requestGet;
  const originalRequestPost = lib.requestPost;
  const originalRequestUpload = lib.requestUpload;

  os.homedir = () => fakeHome;

  lib.requestGet = async (endpoint) => {
    getCalls.push(endpoint);
    if (endpoint === '/api/health') return {};
    if (endpoint.startsWith('/api/sessions?claudeSessionId=')) {
      return { id: numericSessionID };
    }
    if (endpoint === `/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`) {
      return { injections: [] };
    }
    if (endpoint === '/api/observations?limit=100&offset=0') {
      return { observations: [] };
    }
    throw new Error(`Unexpected GET endpoint in test: ${endpoint}`);
  };

  lib.requestPost = async (endpoint, body) => {
    postCalls.push({ endpoint, body });
    return {};
  };

  lib.requestUpload = async (endpoint, body) => {
    uploadCalls.push({ endpoint, body });
    return { status: 'ok', exchange_count: 1 };
  };

  try {
    await handleStop(
      {
        SessionID: claudeSessionID,
        RawInput: '',
        Project: 'engram',
      },
      { transcript_path: missingTranscriptPath }
    );

    const summarizeCall = postCalls.find(
      (call) => call.endpoint === `/api/sessions/${numericSessionID}/summarize`
    );
    assert.ok(summarizeCall, 'Expected summarize endpoint to be called');
    assert.equal(summarizeCall.body.lastUserMessage, 'Need fallback path');
    assert.equal(
      summarizeCall.body.lastAssistantMessage,
      'Fallback assistant text from canonical path'
    );

    assert.equal(uploadCalls.length, 1, 'Expected transcript upload using resolved canonical path');
    assert.match(uploadCalls[0].body, /Fallback assistant text from canonical path/);
    assert.ok(
      getCalls.includes(`/api/sessions/${encodeURIComponent(claudeSessionID)}/injections`),
      'Expected downstream endpoints to keep using Claude session ID'
    );
  } finally {
    os.homedir = originalHomeDir;
    lib.requestGet = originalRequestGet;
    lib.requestPost = originalRequestPost;
    lib.requestUpload = originalRequestUpload;
    lib.clearSessionSignals(claudeSessionID);
    fs.rmSync(fakeHome, { recursive: true, force: true });
  }
});
