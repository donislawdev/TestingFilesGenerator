// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');

// How much of what tfg says on its error stream is kept for a message. Its
// refusals are a few lines, and a run of many files can print a note for each.
const keptError = 64 * 1024;

// TfgError is tfg ending with a code other than nought. The message carries
// what tfg said, because its refusals already name the value and the way out.
class TfgError extends Error {
  constructor(args, status, stderr) {
    const said = stderr.trim() || 'tfg said nothing about why.';
    super(`tfg ${args.join(' ')} ended with code ${status}.\n${said}`);
    this.name = 'TfgError';
    this.status = status;
    this.stderr = stderr;
  }
}

function notStarted(binary, err) {
  return new Error(`tfg could not be started from ${binary}: ${err.message}`);
}

// runToFile runs tfg to the end, writing its standard output to a file. A file
// and not memory, because the manifest of a large set outgrows the buffer Node
// gives a child process by default.
function runToFile(binary, args, outFile) {
  const fd = fs.openSync(outFile, 'w');
  let done;
  try {
    done = spawnSync(binary, args, { stdio: ['ignore', fd, 'pipe'], windowsHide: true, maxBuffer: keptError * 16 });
  } finally {
    fs.closeSync(fd);
  }
  if (done.error) {
    throw notStarted(binary, done.error);
  }
  if (done.status !== 0) {
    throw new TfgError(args, done.status, done.stderr.toString('utf8').slice(-keptError));
  }
}

// run runs tfg without holding the event loop, so a test runner keeps its own
// timers and messages going while files are written.
function run(binary, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true });
    let stderr = '';
    child.stderr.setEncoding('utf8');
    child.stderr.on('data', (chunk) => {
      stderr = (stderr + chunk).slice(-keptError);
    });
    child.on('error', (err) => reject(notStarted(binary, err)));
    child.on('close', (status, signal) => {
      if (status === 0) {
        resolve();
        return;
      }
      reject(new TfgError(args, status ?? signal, stderr));
    });
  });
}

module.exports = { TfgError, run, runToFile };
