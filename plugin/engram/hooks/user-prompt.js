#!/usr/bin/env node
'use strict';

const lib = require('./lib');

function buildSearchRequest(project, prompt, cwd, filesBeingEdited) {
  const request = {
    project,
    query: prompt,
    cwd,
  };

  if (Array.isArray(filesBeingEdited) && filesBeingEdited.length > 0) {
    request.files_being_edited = filesBeingEdited;
  }

  return request;
}

async function handleUserPrompt() {
  return '';
}

if (require.main === module) {
  (async () => {
    await lib.RunHook('UserPromptSubmit', handleUserPrompt);
  })();
}

module.exports = {
  buildSearchRequest,
  handleUserPrompt,
};
