#!/usr/bin/env node
'use strict';

const lib = require('./lib');

function renderStatusline() {
  return '[engram] ○ v5 cleanup in progress';
}

if (require.main === module) {
  lib.RunStatuslineHook(renderStatusline, renderStatusline);
}

module.exports = {
  renderStatusline,
};
