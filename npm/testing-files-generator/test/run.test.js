'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

const { TfgError, run, runToFile } = require('../lib/run.js');
const { tempDir } = require('./helpers.js');

// Node stands in for tfg here, so a child can be made to end any way at all.
const child = (script) => [process.execPath, ['-e', script]];

test('a child stopped by a signal is named by the signal, from both ways of running it', {
  skip: process.platform === 'win32' && 'Windows has no signals to stop a process with',
}, async (t) => {
  const [binary, args] = child("process.kill(process.pid, 'SIGTERM')");
  const out = path.join(tempDir(t), 'out.json');
  assert.throws(() => runToFile(binary, args, out),
    (err) => err instanceof TfgError && err.status === 'SIGTERM' && /was stopped by SIGTERM/.test(err.message));
  await assert.rejects(run(binary, args),
    (err) => err instanceof TfgError && err.status === 'SIGTERM' && /was stopped by SIGTERM/.test(err.message));
});

test('a child that says a great deal on its error stream runs to its own end, and the end is kept', (t) => {
  const [binary, args] = child("process.stderr.write('x'.repeat(4 * 1024 * 1024) + 'the last words'); process.exitCode = 3");
  const out = path.join(tempDir(t), 'out.json');
  assert.throws(() => runToFile(binary, args, out),
    (err) => err instanceof TfgError && err.status === 3 && err.stderr.endsWith('the last words') &&
      err.stderr.length <= 64 * 1024);
});

test('the output file is made new and private, and a file already under its name is never written through', (t) => {
  const [binary, args] = child('process.stdout.write("planned")');
  const out = path.join(tempDir(t), 'out.json');
  runToFile(binary, args, out);
  assert.equal(fs.readFileSync(out, 'utf8'), 'planned');
  if (process.platform !== 'win32') {
    assert.equal(fs.statSync(out).mode & 0o777, 0o600);
  }
  const planted = path.join(tempDir(t), 'planted.json');
  fs.writeFileSync(planted, 'left by someone else');
  assert.throws(() => runToFile(binary, args, planted), (err) => err.code === 'EEXIST');
  assert.equal(fs.readFileSync(planted, 'utf8'), 'left by someone else');
});
