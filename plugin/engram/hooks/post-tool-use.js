#!/usr/bin/env node
'use strict';

const lib = require('./lib');

async function handlePostToolUse() {
  return '';
}

(async () => {
  await lib.RunHook('PostToolUse', handlePostToolUse);
})();
