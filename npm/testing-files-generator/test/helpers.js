'use strict';

const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..', '..');
const built = path.join(root, process.platform === 'win32' ? 'tfg.exe' : 'tfg');

// tfg is the binary these tests run: TFG_BINARY, or the one the repository's
// build command leaves at its root. Neither is a failure and not a skip - a
// suite that skips without a binary is green while testing nothing.
function tfg() {
  const binary = process.env.TFG_BINARY || built;
  if (!fs.existsSync(binary)) {
    throw new Error(`No tfg binary at ${binary}. Build one from the repository root with ` +
      'go build -tags "$(cat .github/build-tags)" -o tfg.exe ./cmd/tfg, or set TFG_BINARY.');
  }
  return binary;
}

function tempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tfg-npm-test-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

// node runs a script in a fresh process, with this package and a binary, and
// gives its exit code and both streams. A variable given as undefined is taken
// out of the child's environment rather than set to an empty string.
function node(script, { env = {}, file = '', args = [] } = {}) {
  const childEnv = { ...process.env, TFG_BINARY: tfg(), ...env };
  for (const [key, value] of Object.entries(env)) {
    if (value === undefined) {
      delete childEnv[key];
    }
  }
  return new Promise((resolve) => {
    const child = spawn(process.execPath, file ? [file, ...args] : ['-e', script, ...args], {
      env: childEnv,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (chunk) => { stdout += chunk; });
    child.stderr.on('data', (chunk) => { stderr += chunk; });
    child.on('close', (status) => resolve({ status, stdout, stderr, pid: child.pid }));
  });
}

const pkg = JSON.stringify(path.resolve(__dirname, '..'));

// A small set that still holds every kind of case: one either side of the
// limit, unspecified files and a group of several files.
const small = { preset: 'upload-validation', options: { limit: '1mb', bulk: 2, 'far-over': 'off' } };

module.exports = { node, pkg, small, tempDir, tfg };
