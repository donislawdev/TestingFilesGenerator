// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

// assemble lays out the packages a release puts on npm: this one, and one per
// system carrying the tfg binary of that release. .github/workflows/npm.yml
// runs it on the archives it has downloaded and checked, and nothing here
// fetches or publishes anything. It is not part of the package - "files" in
// package.json leaves it out.
//
// The systems come from lib/binary.js and from nowhere else. A guard holds that
// list to the targets of the release, so a second list here could only ever
// disagree with it. docs/NPM-PUBLISH-2026-11-09.md.

const fs = require('node:fs');
const path = require('node:path');
const { binaryIn, platforms } = require('../lib/binary.js');

const packageDir = path.resolve(__dirname, '..');

// How the release names a system and an architecture in its archives, against
// how Node names them in process.platform and process.arch.
const releaseSystem = { win32: 'windows', darwin: 'macos', linux: 'linux' };
const releaseArch = { x64: 'amd64', arm64: 'arm64' };
const systemLabel = { win32: 'Windows', darwin: 'macOS', linux: 'Linux' };

// A version npm takes and the release tags carry: three numbers, and a
// prerelease after a hyphen. Build metadata (+...) is left out, because npm
// drops it and two releases would then claim one version.
const versionShape = /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/;

function checkedVersion(version) {
  if (typeof version !== 'string' || !versionShape.test(version)) {
    throw new Error(`${JSON.stringify(version)} is not a release version. ` +
      'Give the version of a release tag without its v, for example 0.6.0 or 0.6.0-rc1.');
  }
  return version;
}

// targets lists every system this package installs a binary for, in the order
// lib/binary.js gives them, with what each is called on both sides.
function targets() {
  return Object.entries(platforms).map(([key, name]) => {
    const [platform, arch] = key.split(' ');
    if (!releaseSystem[platform] || !releaseArch[arch]) {
      throw new Error(`lib/binary.js names ${key}, and assemble does not know what the release calls it. ` +
        'Add the release name of the system and of the architecture beside releaseSystem and releaseArch.');
    }
    // binary is where lib/binary.js looks for it, and entry is what holds it at
    // the top of the package - the binary itself, or the app on macOS.
    const binary = binaryIn(platform);
    return {
      platform,
      arch,
      name,
      dir: name.split('/')[1],
      binary,
      entry: binary.split('/')[0],
      label: `${systemLabel[platform]} on ${arch}`,
    };
  });
}

// archiveOf is the release asset that carries the binary for a target.
function archiveOf(version, target) {
  const ext = target.platform === 'win32' ? 'zip' : 'tar.gz';
  return `tfg_${checkedVersion(version)}_${releaseSystem[target.platform]}_${releaseArch[target.arch]}.${ext}`;
}

// distTag is the tag npm files a version under. A prerelease must not become
// latest, so that npm install keeps giving people the last release.
function distTag(version) {
  return checkedVersion(version).includes('-') ? 'next' : 'latest';
}

function mainManifest() {
  return JSON.parse(fs.readFileSync(path.join(packageDir, 'package.json'), 'utf8'));
}

// platformManifest is the package.json of the package that carries one
// binary. os and cpu are what npm reads to install it on that system alone,
// and repository has to match this package's, because npm checks a
// provenance statement against it.
function platformManifest(version, target, main) {
  return {
    name: target.name,
    version: checkedVersion(version),
    description: `The tfg command line for ${target.label}, installed by testing-files-generator. ` +
      'Install testing-files-generator rather than this package.',
    license: 'GPL-3.0-only',
    author: main.author,
    homepage: main.homepage,
    repository: main.repository,
    bugs: main.bugs,
    os: [target.platform],
    cpu: [target.arch],
    files: [target.entry, 'LICENSE', 'THIRD-PARTY-NOTICES.md'],
    publishConfig: { access: 'public' },
  };
}

function platformReadme(target) {
  return `# ${target.name}\n\n` +
    `The tfg command line for ${target.label}. ` +
    '[testing-files-generator](https://www.npmjs.com/package/testing-files-generator) installs it ' +
    'on the system it belongs to, so install that package rather than this one:\n\n' +
    '```sh\nnpm install --save-dev testing-files-generator\n```\n\n' +
    'tfg is licensed under GPL-3.0-only, in LICENSE. THIRD-PARTY-NOTICES.md names the code built into it ' +
    'and the licence of each part.\n';
}

// releasedManifest is this package's package.json as the release publishes it:
// the version of the release, and the package of every system at exactly that
// version, so a binary of another release is never installed beside it.
function releasedManifest(version, main) {
  const optionalDependencies = {};
  for (const target of targets()) {
    optionalDependencies[target.name] = checkedVersion(version);
  }
  return { ...main, version: checkedVersion(version), optionalDependencies };
}

function emptyDirectory(dir) {
  if (fs.existsSync(dir) && fs.readdirSync(dir).length > 0) {
    throw new Error(`${dir} is not empty. assemble writes into an empty directory, so that nothing left ` +
      'from an earlier run is published with this one. Give it a new directory, or empty this one.');
  }
  fs.mkdirSync(dir, { recursive: true });
}

// missing says that a file a package needs is not where it should be, whose it
// was - the archive of a release, or this package - and where to look.
function missing(source, whose, name) {
  return new Error(`${source} is not there, and ${whose} has to carry ${name}. ` +
    'Check that the directory given for it holds what the release archive unpacked to, ' +
    'or that this package still has the file it lists.');
}

// copyFrom copies one file or directory, and says whose it was when it is not
// there.
function copyFrom(from, to, name, whose) {
  const source = path.join(from, name);
  if (!fs.existsSync(source)) {
    throw missing(source, whose, name);
  }
  refuseLinks(source);
  fs.cpSync(source, path.join(to, name), { recursive: true });
}

// refuseLinks stops at a symbolic link anywhere in what is to be packed. npm
// leaves links out of a package without a word - the tfg beside the macOS app
// is one, and the package made with it carried no binary at all (measured
// 2026-11-09).
function refuseLinks(source) {
  const stat = fs.lstatSync(source);
  if (stat.isSymbolicLink()) {
    throw new Error(`${source} is a symbolic link, and npm leaves links out of a package. ` +
      'Copy what it points at instead, or change where lib/binary.js looks for the binary.');
  }
  if (stat.isDirectory()) {
    for (const name of fs.readdirSync(source)) {
      refuseLinks(path.join(source, name));
    }
  }
}

function writeJson(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

// build writes one directory per package under out and gives them in the
// order they are to be published: every system first, this package last, so
// that the package which asks for a binary never comes out before the binary.
// binaries holds what each archive unpacked to, one directory per target
// named as archives() names it.
//
// The directories are given as absolute paths, because npm reads a relative
// one such as packages/win32-x64 as a GitHub repository and asks git for it
// (measured 2026-11-09, npm 10.9.9).
function build(version, binaries, out) {
  checkedVersion(version);
  out = path.resolve(out);
  emptyDirectory(out);
  const main = mainManifest();
  const order = [];
  for (const target of targets()) {
    const from = path.join(binaries, target.dir);
    const to = path.join(out, target.dir);
    fs.mkdirSync(to);
    const archive = `the archive for ${target.label}`;
    copyFrom(from, to, target.entry, archive);
    if (!fs.existsSync(path.join(to, target.binary))) {
      throw missing(path.join(from, target.binary), archive, target.binary);
    }
    if (target.platform !== 'win32') {
      // Set rather than trusted to the archive, because a binary npm unpacks
      // without it is a package that cannot run on the one system it is for.
      fs.chmodSync(path.join(to, target.binary), 0o755);
    }
    copyFrom(from, to, 'LICENSE', archive);
    copyFrom(from, to, 'THIRD-PARTY-NOTICES.md', archive);
    fs.writeFileSync(path.join(to, 'README.md'), platformReadme(target));
    writeJson(path.join(to, 'package.json'), platformManifest(version, target, main));
    order.push(to);
  }
  const to = path.join(out, main.name);
  fs.mkdirSync(to);
  for (const name of main.files) {
    copyFrom(packageDir, to, name.replace(/\/$/, ''), 'this package');
  }
  writeJson(path.join(to, 'package.json'), releasedManifest(version, main));
  order.push(to);
  return order;
}

// archives gives one line per system: the directory its archive is to be
// unpacked into, the archive, and where the binary sits in it.
const usage = 'Usage: node assemble.js archives <version> | tag <version> | build <version> <binaries> <out>';

function main(argv) {
  const [command, version, ...rest] = argv;
  switch (command) {
    case 'archives':
      return targets().map((target) => `${target.dir} ${archiveOf(version, target)} ${target.binary}`);
    case 'tag':
      return [distTag(version)];
    case 'build':
      if (rest.length !== 2) {
        throw new Error(usage);
      }
      return build(version, rest[0], rest[1]);
    default:
      throw new Error(usage);
  }
}

if (require.main === module) {
  try {
    process.stdout.write(`${main(process.argv.slice(2)).join('\n')}\n`);
  } catch (err) {
    process.stderr.write(`assemble: ${err.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { archiveOf, build, distTag, main, platformManifest, releasedManifest, targets };
