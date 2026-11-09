'use strict';

const assert = require('node:assert/strict');
const { execSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { test } = require('node:test');

const { archiveOf, build, distTag, main, targets } = require('../release/assemble.js');
const { platforms } = require('../lib/binary.js');
const ours = require('../package.json');

function tempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tfg-assemble-test-'));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

// binaries lays out what the release archives unpack to, one directory per
// system, each binary holding the name of its system so a mix-up shows. The
// macOS one sits in an app beside the Info.plist its signature covers, as in
// the release.
function binaries(t) {
  const dir = tempDir(t);
  for (const target of targets()) {
    const into = path.join(dir, target.dir);
    fs.mkdirSync(path.dirname(path.join(into, target.binary)), { recursive: true });
    fs.writeFileSync(path.join(into, target.binary), `binary for ${target.dir}`);
    if (target.entry !== target.binary) {
      fs.writeFileSync(path.join(into, target.entry, 'Contents', 'Info.plist'), 'plist');
    }
    fs.writeFileSync(path.join(into, 'LICENSE'), 'GNU GENERAL PUBLIC LICENSE');
    fs.writeFileSync(path.join(into, 'THIRD-PARTY-NOTICES.md'), 'notices');
    fs.writeFileSync(path.join(into, 'README.md'), 'the README of the program, not of a package');
  }
  return dir;
}

function readJson(file) {
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}

// What npm itself would put in the tarball, asked of npm rather than of the
// files list, because the list is what this test would otherwise be trusting.
// npm 10 and 11 answer with a list of packages, npm 12 with an object keyed by
// name (measured 2026-11-09), and CI runs the first while a person may run the
// last.
function packed(dir) {
  const out = JSON.parse(execSync('npm pack --dry-run --json --ignore-scripts', { cwd: dir, encoding: 'utf8' }));
  const [only] = Array.isArray(out) ? out : Object.values(out);
  return only.files.map((f) => f.path).sort();
}

test('the archive of each system is named as the release names it', () => {
  // The names of the assets of v0.5.0, as the release page lists them.
  const released = [
    'tfg_0.5.0_linux_amd64.tar.gz',
    'tfg_0.5.0_linux_arm64.tar.gz',
    'tfg_0.5.0_macos_arm64.tar.gz',
    'tfg_0.5.0_windows_amd64.zip',
    'tfg_0.5.0_windows_arm64.zip',
  ];
  assert.deepEqual(targets().map((target) => archiveOf('0.5.0', target)).sort(), released);
  assert.deepEqual(main(['archives', '0.5.0']).map((line) => line.split(' ')[1]).sort(), released);
  assert.ok(main(['archives', '0.5.0']).includes('darwin-arm64 tfg_0.5.0_macos_arm64.tar.gz tfg.app/Contents/MacOS/tfg'));
});

test('every system lib/binary.js knows is a target, under the package it names', () => {
  assert.deepEqual(targets().map((target) => `${target.platform} ${target.arch}`), Object.keys(platforms));
  assert.deepEqual(targets().map((target) => target.name), Object.values(platforms));
});

test('a prerelease is filed under next, a release under latest', () => {
  assert.equal(distTag('0.6.0'), 'latest');
  assert.equal(distTag('1.0.0-rc1'), 'next');
  assert.deepEqual(main(['tag', '0.6.0-rc.2']), ['next']);
});

test('a version that is not a release version is refused before anything is written', (t) => {
  const out = path.join(tempDir(t), 'out');
  for (const version of ['v0.6.0', '0.6', '0.6.0+build.1', '', undefined, '0.6.0\n']) {
    assert.throws(() => build(version, binaries(t), out), (err) => err.message.includes('is not a release version'));
    assert.throws(() => distTag(version), /is not a release version/);
  }
  assert.equal(fs.existsSync(out), false);
});

test('each system gets a package with its own binary, installed on that system alone', (t) => {
  const from = binaries(t);
  const out = path.join(tempDir(t), 'out');
  const order = build('0.6.0', from, out);
  assert.equal(order.length, targets().length + 1);
  for (const [i, target] of targets().entries()) {
    const dir = order[i];
    assert.equal(dir, path.join(out, target.dir));
    const manifest = readJson(path.join(dir, 'package.json'));
    assert.equal(manifest.name, target.name);
    assert.equal(manifest.version, '0.6.0');
    assert.deepEqual(manifest.os, [target.platform]);
    assert.deepEqual(manifest.cpu, [target.arch]);
    assert.equal(manifest.license, 'GPL-3.0-only');
    assert.deepEqual(manifest.repository, ours.repository);
    assert.deepEqual(manifest.publishConfig, { access: 'public' });
    assert.equal(fs.readFileSync(path.join(dir, target.binary), 'utf8'), `binary for ${target.dir}`);
    assert.equal(fs.readFileSync(path.join(dir, 'LICENSE'), 'utf8'), 'GNU GENERAL PUBLIC LICENSE');
    const readme = fs.readFileSync(path.join(dir, 'README.md'), 'utf8');
    assert.ok(readme.includes('npm install --save-dev testing-files-generator'), readme);
    if (process.platform !== 'win32' && target.platform !== 'win32') {
      assert.equal(fs.statSync(path.join(dir, target.binary)).mode & 0o777, 0o755);
    }
  }
});

test('this package comes last, at the version of the release and asking for every binary at exactly it', (t) => {
  const order = build('0.6.0', binaries(t), path.join(tempDir(t), 'out'));
  const dir = order[order.length - 1];
  const manifest = readJson(path.join(dir, 'package.json'));
  assert.equal(manifest.version, '0.6.0');
  const wanted = Object.fromEntries(Object.values(platforms).map((name) => [name, '0.6.0']));
  assert.deepEqual(manifest.optionalDependencies, wanted);
  const { version, optionalDependencies, ...rest } = manifest;
  const { version: _v, optionalDependencies: _o, ...kept } = ours;
  assert.deepEqual(rest, kept);
});

test('npm packs each package with what it needs and nothing else', (t) => {
  const order = build('0.6.0', binaries(t), path.join(tempDir(t), 'out'));
  for (const [i, target] of targets().entries()) {
    const app = target.entry === target.binary ? [] : [`${target.entry}/Contents/Info.plist`];
    assert.deepEqual(packed(order[i]), [target.binary, ...app, 'LICENSE', 'README.md', 'THIRD-PARTY-NOTICES.md', 'package.json'].sort());
  }
  const files = packed(order[order.length - 1]);
  assert.ok(files.includes('lib/binary.js') && files.includes('bin/tfg.js') && files.includes('index.d.ts'), files.join(' '));
  assert.deepEqual(files.filter((f) => f.startsWith('test/') || f.startsWith('release/')), []);
});

test('a directory that is not empty is refused, so nothing from an earlier run goes out', (t) => {
  const out = tempDir(t);
  fs.writeFileSync(path.join(out, 'left-over.tgz'), '');
  assert.throws(() => build('0.6.0', binaries(t), out), (err) => err.message.includes('is not empty'));
});

test('an archive without its licence is refused, naming the system', (t) => {
  const from = binaries(t);
  const target = targets()[0];
  fs.rmSync(path.join(from, target.dir, 'LICENSE'));
  assert.throws(() => build('0.6.0', from, path.join(tempDir(t), 'out')),
    (err) => err.message.includes(`the archive for ${target.label} has to carry LICENSE`));
});

test('an archive without its binary is refused, naming the binary', (t) => {
  const from = binaries(t);
  const target = targets()[targets().length - 1];
  fs.rmSync(path.join(from, target.dir, target.binary));
  assert.throws(() => build('0.6.0', from, path.join(tempDir(t), 'out')),
    (err) => err.message.includes(`has to carry ${target.binary}`));
});

test('the command line refuses what it does not know and says how it is used', () => {
  assert.throws(() => main(['publish', '0.6.0']), /Usage: node assemble.js/);
  assert.throws(() => main(['build', '0.6.0']), /Usage: node assemble.js/);
});

test('the macOS package carries the whole app, where lib/binary.js looks for the binary', (t) => {
  const order = build('0.6.0', binaries(t), path.join(tempDir(t), 'out'));
  const mac = targets().findIndex((target) => target.platform === 'darwin');
  assert.ok(mac >= 0, 'no macOS target, so this test checked nothing');
  const dir = order[mac];
  assert.equal(fs.readFileSync(path.join(dir, 'tfg.app', 'Contents', 'MacOS', 'tfg'), 'utf8'), 'binary for darwin-arm64');
  assert.equal(fs.readFileSync(path.join(dir, 'tfg.app', 'Contents', 'Info.plist'), 'utf8'), 'plist');
  assert.deepEqual(readJson(path.join(dir, 'package.json')).files, ['tfg.app', 'LICENSE', 'THIRD-PARTY-NOTICES.md']);
});

test('a symbolic link in what is packed is refused, because npm would leave it out', (t) => {
  const from = binaries(t);
  const target = targets().find((x) => x.entry !== x.binary);
  const link = path.join(from, target.dir, target.entry, 'Contents', 'Link');
  try {
    fs.symlinkSync('Info.plist', link);
  } catch (err) {
    t.skip(`this system does not let the test make a link: ${err.code}`);
    return;
  }
  assert.throws(() => build('0.6.0', from, path.join(tempDir(t), 'out')),
    (err) => err.message.includes('is a symbolic link') && err.message.includes(link));
});
