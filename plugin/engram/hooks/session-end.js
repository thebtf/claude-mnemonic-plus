#!/usr/bin/env node
'use strict';

const lib = require('./lib');

async function handleSessionEnd(ctx, input) {
  const sessionID = ctx.SessionID || input.session_id || input.SessionID || '';
  if (!sessionID) {
    console.error('[session-end] No session_id in hook input — skipping');
    return '';
  }

  try {
    await lib.requestPost(
      `/api/sessions/${encodeURIComponent(sessionID)}/propagate-outcome`,
      {},
      1200
    );
    console.error(`[session-end] propagate-outcome fired for session=${sessionID}`);
  } catch (err) {
    console.error(`[session-end] propagate-outcome failed: ${err.message}`);
  }

  return '';
}

(async () => {
  await lib.RunHook('SessionEnd', handleSessionEnd);
})();
