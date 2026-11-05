// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

const { spawn, spawnSync } = require('node:child_process');
const fs = require('node:fs');

// How much of what tfg says on its error stream is kept for a message. Its
// refusals are a few lines, and a run of many files can print a note for each.
const keptError = 64 * 1024;

// ended says how tfg stopped - with a code, or by a signal named as it is.
function ended(status) {
  return typeof status === 'string' ? `was stopped by ${status}` : `ended with code ${status}`;
}

// TfgError is tfg ending with a code other than nought, or stopped by a signal,
// whose name is then the status. The message carries what tfg said, because
// its refusals already name the value and the way out. A caller that can say
// what went wrong in the words the test was written in passes its own message,
// and the status and everything tfg said stay on the error.
class TfgError extends Error {
  constructor(args, status, stderr, message = `tfg ${args.join(' ')} ${ended(status)}.\n${said(stderr)}`) {
    super(message);
    this.name = 'TfgError';
    this.status = status;
    this.stderr = stderr;
  }
}

function said(stderr) {
  return stderr.trim() || 'tfg said nothing about why.';
}

function notStarted(binary, err) {
  return new Error(`tfg could not be started from ${binary}: ${err.message}`);
}

// runToFile runs tfg to the end, writing its standard output to a file. A file
// and not memory, because the manifest of a large set outgrows the buffer Node
// gives a child process by default. The error stream goes to a file of its own
// for the same reason - a buffer that fills stops tfg part way - and only its
// end is kept.
function runToFile(binary, args, outFile) {
  const errFile = `${outFile}.err`;
  const out = fs.openSync(outFile, 'w');
  let err;
  try {
    err = fs.openSync(errFile, 'w+');
    const done = spawnSync(binary, args, { stdio: ['ignore', out, err], windowsHide: true });
    if (done.error) {
      throw notStarted(binary, done.error);
    }
    if (done.status !== 0) {
      throw new TfgError(args, done.status ?? done.signal, tail(err));
    }
  } finally {
    fs.closeSync(out);
    if (err !== undefined) {
      fs.closeSync(err);
    }
    fs.rmSync(errFile, { force: true });
  }
}

// tail reads the last keptError bytes of an open file.
function tail(fd) {
  const size = fs.fstatSync(fd).size;
  const kept = Buffer.alloc(Math.min(size, keptError));
  fs.readSync(fd, kept, 0, kept.length, size - kept.length);
  return kept.toString('utf8');
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

module.exports = { TfgError, ended, run, runToFile, said };
