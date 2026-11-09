#!/usr/bin/env node
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

// npx tfg runs the binary this package installed, with the same arguments, the
// same streams and the same exit code, so a script cannot tell it from tfg run
// directly.

const { spawn } = require('node:child_process');
const { binaryPath } = require('../lib/binary.js');
const { notStarted } = require('../lib/run.js');

// RUNTIME in docs/CLI.md section 3 - the tool could not do its work, and the
// arguments were never looked at.
const cannotRun = 1;

let binary;
try {
  binary = binaryPath();
} catch (err) {
  process.stderr.write(`tfg: ${err.message}\n`);
  process.exit(cannotRun);
}

const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit', windowsHide: false });

// A Ctrl+C in a terminal reaches tfg as well as this process. This one waits,
// so tfg can stop the way it stops on its own and its exit code is the one a
// script sees. A signal sent to this process alone is passed on.
for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  process.on(signal, () => {
    if (signal !== 'SIGINT') {
      child.kill(signal);
    }
  });
}

child.on('error', (err) => {
  process.stderr.write(`tfg: ${notStarted(binary, err).message}\n`);
  process.exit(cannotRun);
});

child.on('close', (status, signal) => {
  if (signal) {
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
    return;
  }
  process.exit(status ?? cannotRun);
});
