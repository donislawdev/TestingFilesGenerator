// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

const { spawnSync } = require('node:child_process');
const crypto = require('node:crypto');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { inspect } = require('node:util');

const { binaryPath } = require('./binary.js');
const { TfgError, ended, run, runToFile, said } = require('./run.js');
const { TestFile, outcomes } = require('./testfile.js');

// The manifest major this package reads. Another major is refused rather than
// read as if it were this one - docs/MANIFEST.md, the version of the contract.
const manifestMajor = '1';

// The code tfg verify ends with when a directory does not match its manifest,
// from the exit codes in docs/CLI.md. Any other code is verify failing to look.
const exitVerify = 7;

// Flags this package sets itself. An option of the same name would fight it.
const reserved = {
  out: 'this package writes every set into a directory of its own',
  json: 'this package reads the manifest from the directory it writes',
  'dry-run': 'this package plans each set with a dry run and then writes it',
  preset: 'a preset is named by the preset property beside the options',
};

// Everything this package leaves while it works starts with this, followed by
// the process id, so a later run can tell a set being written now from one a
// stopped run left behind.
const unfinished = '.tmp-';

// A rename the system refuses for a moment. On Windows a scanner holding a new
// file open is enough, so these are tried again before anything is said.
const busy = new Set(['EPERM', 'EACCES', 'EBUSY']);
const renameTries = 10;

// sources keeps one plan per set and process. A spec file is loaded once in the
// runner and again in every worker, and two spec files can ask for one set.
const sources = new Map();
let noted = false;

// events says what a write did - wrote a copy, moved it to its name, or took
// one another process moved first. Not part of the package's interface: the
// tests listen, because a race that never happened passes as well as one won.
const events = new EventEmitter();

function fileSet(spec = {}) {
  if (typeof spec !== 'object' || spec === null) {
    throw new TypeError('fileSet takes an object, for example { preset: \'upload-validation\', options: { limit: \'2mb\' } }.');
  }
  // Told from the flags and not from the spec: an option set to false or to an
  // empty list adds no flag, and tfg asked for nothing answers in flags.
  const args = argsOf(spec);
  if (args.length === 1) {
    throw new Error('fileSet was not told which files to write. Name a preset, a recipe or the options of a set, ' +
      'for example { preset: \'upload-validation\' }. npx tfg preset list lists the presets.');
  }
  const binary = spec.binary === undefined ? binaryPath() : named(spec.binary, 'binary',
    'Point it at a tfg binary, or leave it out to use the one this package installs.');
  if (spec.recipe) {
    named(path.resolve(spec.recipe), 'recipe',
      `A relative path is taken from the directory the tests started in, here ${process.cwd()}, ` +
      'so path.join(__dirname, ...) names a recipe beside the spec file, or path.join(import.meta.dirname, ...) in an ES module.');
  }
  const cacheDir = path.resolve(spec.cacheDir ?? defaultCacheDir());
  const recipe = spec.recipe ? fs.readFileSync(path.resolve(spec.recipe), 'utf8') : '';
  const key = digest([identity(binary), args, recipe, cacheDir]);
  let source = sources.get(key);
  if (!source) {
    source = new Source(binary, args, cacheDir, spec);
    sources.set(key, source);
  }
  return new FileSet(source, source.files);
}

function argsOf(spec) {
  const args = ['generate'];
  if (spec.recipe) {
    args.push(path.resolve(spec.recipe));
  }
  if (spec.preset) {
    args.push(`--preset=${spec.preset}`);
  }
  for (const [key, value] of Object.entries(spec.options ?? {})) {
    if (Object.hasOwn(reserved, key)) {
      throw new Error(`The option ${key} cannot be set here - ${reserved[key]}.`);
    }
    if (!/^[a-z][a-z0-9-]*$/.test(key)) {
      throw new Error(`${JSON.stringify(key)} is not a flag tfg has. Options are flag names without the dashes, such as limit or far-over.`);
    }
    for (const one of Array.isArray(value) ? value : [value]) {
      if (one === true) {
        args.push(`--${key}`);
      } else if (typeof one === 'string' || typeof one === 'number') {
        args.push(`--${key}=${one}`);
      } else if (one !== false && one !== undefined && one !== null) {
        throw new TypeError(`The option ${key} takes a string, a number, true or a list of them. It was given a ${typeof one}.`);
      }
    }
  }
  return args;
}

// named gives back a file an option names, or says that there is none, before
// anything reads it and fails with only the system's word for it.
function named(file, option, wayOut) {
  if (!fs.existsSync(file)) {
    throw new Error(`The ${option} option names ${file}, and there is no file there. ${wayOut}`);
  }
  return file;
}

function defaultCacheDir() {
  if (process.env.TFG_CACHE_DIR) {
    return process.env.TFG_CACHE_DIR;
  }
  const modules = path.join(process.cwd(), 'node_modules');
  if (fs.existsSync(modules)) {
    return path.join(modules, '.cache', 'testing-files-generator');
  }
  return path.join(os.tmpdir(), 'testing-files-generator');
}

function labelOf(spec) {
  const name = spec.preset ?? (spec.recipe ? path.basename(spec.recipe).replace(/\.[^.]*$/, '') : 'flags');
  return name.replace(/[^A-Za-z0-9_-]+/g, '-').slice(0, 40);
}

// described is the call as the test wrote it, for a message - what makes the
// set, without where it is kept or which binary writes it.
function described(spec) {
  const shown = {};
  for (const key of ['preset', 'recipe', 'options']) {
    if (spec[key] !== undefined) {
      shown[key] = spec[key];
    }
  }
  return `fileSet(${inspect(shown, { breakLength: Infinity, depth: Infinity })})`;
}

// identity is what tells two binaries apart without running them, so a new
// build or another version plans again rather than reusing an old plan.
function identity(binary) {
  const stat = fs.statSync(binary);
  return [fs.realpathSync(binary), stat.size, stat.mtimeMs];
}

function digest(value) {
  return crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
}

function scratch(cacheDir) {
  return path.join(cacheDir, `${unfinished}${process.pid}-${crypto.randomBytes(4).toString('hex')}`);
}

// Source is one set: its plan, where it is written, and the one write a process
// makes of it however many files ask.
class Source {
  constructor(binary, args, cacheDir, spec) {
    this.binary = binary;
    this.args = args;
    this.cacheDir = cacheDir;
    this.preset = spec.preset;
    this.shown = described(spec);
    fs.mkdirSync(cacheDir, { recursive: true });
    noteUnfinished(cacheDir);
    this.plan = this._planned();
    // The recipe hash leaves out the seed and --clean, measured 2026-11-04, so
    // the arguments are part of the name and two seeds never share a set.
    const run = this.plan.run;
    this.dir = path.join(cacheDir, `${labelOf(spec)}-${digest([args, run.recipe_hash, run.seed, this.plan.tool]).slice(0, 20)}`);
    this.files = this.plan.files.map((entry) => new TestFile(this, entry));
    this._written = undefined;
  }

  // _planned asks tfg what the set holds without writing any of it. The dry run
  // names a directory that is never made, because a dry run into a directory
  // holding a manifest is refused like a real one.
  _planned() {
    const out = `${scratch(this.cacheDir)}.json`;
    let plan;
    try {
      runToFile(this.binary, [...this.args, `--out=${scratch(this.cacheDir)}`, '--dry-run', '--json'], out);
      plan = JSON.parse(fs.readFileSync(out, 'utf8'));
    } catch (err) {
      throw err instanceof TfgError ? refusedPlan(this.shown, this.preset, err) : err;
    } finally {
      fs.rmSync(out, { force: true });
    }
    readable(plan);
    return plan;
  }

  write() {
    if (!this._written) {
      this._written = this._write().catch((err) => {
        this._written = undefined;
        throw err;
      });
    }
    return this._written;
  }

  async _write() {
    if (fs.existsSync(this.dir)) {
      await this._taken();
      return this.dir;
    }
    const tmp = scratch(this.cacheDir);
    fs.mkdirSync(tmp);
    try {
      await run(this.binary, [...this.args, `--out=${tmp}`]);
      const problem = mismatch(tmp, this.plan);
      if (problem) {
        throw new Error(`tfg wrote a set that does not match its own plan - ${problem}. This is a fault in tfg or in this package. Please report it.`);
      }
    } catch (err) {
      discard(this.binary, tmp);
      throw err;
    }
    events.emit('wrote', tmp);
    return this._settle(tmp);
  }

  // _settle moves a finished set to its name. Several workers can write the
  // same set at once, and the first to move it wins - the others find the
  // winner whole, take it, and remove their own copy through its manifest.
  async _settle(tmp) {
    for (let attempt = 1; ; attempt++) {
      try {
        fs.renameSync(tmp, this.dir);
        events.emit('moved', this.dir);
        return this.dir;
      } catch (err) {
        if (fs.existsSync(this.dir)) {
          discard(this.binary, tmp);
          await this._taken();
          events.emit('took', this.dir);
          return this.dir;
        }
        if (!busy.has(err.code) || attempt === renameTries) {
          discard(this.binary, tmp);
          throw new Error(`The set written in ${tmp} could not be moved to ${this.dir} - ${err.message}`);
        }
        await new Promise((resolve) => setTimeout(resolve, attempt * 100));
      }
    }
  }

  // _taken checks a set this process did not write before it is used: against
  // the plan, and then every byte against the manifest through tfg verify. A
  // test that writes into a file it was given changes the set for every later
  // run, and the size alone does not show it.
  async _taken() {
    const problem = mismatch(this.dir, this.plan);
    if (problem) {
      throw stale(this.dir, problem);
    }
    try {
      await run(this.binary, ['verify', path.join(this.dir, 'manifest.json')]);
    } catch (err) {
      if (err instanceof TfgError && err.status === exitVerify) {
        throw stale(this.dir, 'its files are not the ones it was written with', err.stderr.trim());
      }
      throw err;
    }
  }
}

// refusedPlan tells a plan tfg refused in the words the test was written in.
// The command this package ran names a scratch directory and flags nobody
// wrote, and an option tfg does not know brings the whole help of tfg
// generate, every flag behind a dash, where one sentence names the mistake.
// Everything tfg said stays on the error, in stderr. The help lists every
// flag of generate, and a preset or a recipe refuses some of them, so the
// list is not offered as what this call takes.
function refusedPlan(shown, preset, err) {
  const unknown = /^flag provided but not defined: -+(\S+)/m.exec(err.stderr);
  const known = [...err.stderr.matchAll(/^\s+-([a-z][a-z0-9-]*)\b/gm)]
    .map((m) => m[1])
    .filter((name) => !Object.hasOwn(reserved, name));
  if (unknown && known.length > 0) {
    const help = preset
      ? `npx tfg preset show ${preset} says what the preset's own options do, and npx tfg generate --help the rest.`
      : 'npx tfg generate --help says what each one does.';
    return new TfgError([], err.status, err.stderr,
      `${shown} gives the option ${unknown[1]}, and tfg generate has no flag of that name. ` +
      `Its flags are ${known.join(', ')}. Not every one goes with every set, and tfg says why when one does not. ${help}`);
  }
  return new TfgError([], err.status, err.stderr, `${shown} could not be planned - tfg ${ended(err.status)}.\n${said(err.stderr)}`);
}

// readable refuses a manifest this package cannot read for what it says.
function readable(plan) {
  const version = String(plan?.manifest_version ?? '');
  if (version.split('.')[0] !== manifestMajor) {
    throw new Error(`tfg wrote a manifest of version ${version || 'none'}, and this package reads version ${manifestMajor}. ` +
      'Install the version of testing-files-generator that comes with that tfg, or point TFG_BINARY at a tfg of the same version as this package.');
  }
  for (const entry of plan.files) {
    if (!outcomes.includes(entry.expected?.outcome)) {
      throw new Error(`${entry.path} expects ${JSON.stringify(entry.expected?.outcome)}, which this package does not know. ` +
        `It knows ${outcomes.join(', ')}. Use the version of this package that comes with the tfg it runs.`);
    }
  }
}

// mismatch says how a written set differs from its plan, or nothing when it
// does not. It compares names and sizes, which is enough for a set tfg has just
// written. The bytes of a set written earlier are checked by _taken, through
// tfg verify - measured 2026-11-05, about 80 ms for the default upload set of
// 115 MB, against about 590 ms to write it.
//
// Every file has to be on the disk. A run that ends with code nought writes
// every file it plans, and one that cannot ends with the partial code, which
// run turns into an error. That holds until a run can describe a file without
// writing it on purpose - the archive and manifest-only output of D10.
function mismatch(dir, plan) {
  let written;
  try {
    written = JSON.parse(fs.readFileSync(path.join(dir, 'manifest.json'), 'utf8'));
  } catch (err) {
    return `its manifest cannot be read (${err.message})`;
  }
  if (written.run?.recipe_hash !== plan.run.recipe_hash || written.run?.seed !== plan.run.seed) {
    return 'it was written from another recipe or seed';
  }
  if (written.run?.complete !== true) {
    return 'the run that wrote it did not finish';
  }
  if (!Array.isArray(written.files) || written.files.length !== plan.files.length) {
    return `it holds ${written.files?.length ?? 'no'} files where the plan has ${plan.files.length}`;
  }
  for (let i = 0; i < plan.files.length; i++) {
    const want = plan.files[i];
    const got = written.files[i];
    if (got.path !== want.path || got.bytes !== want.bytes) {
      return `its file ${i + 1} is ${got.path} of ${got.bytes} B where the plan has ${want.path} of ${want.bytes} B`;
    }
    if (!got.materialized) {
      return `${got.path} was not written`;
    }
    let stat;
    try {
      stat = fs.statSync(path.join(dir, got.path));
    } catch {
      return `${got.path} is missing`;
    }
    if (stat.size !== got.bytes) {
      return `${got.path} is ${stat.size} B and its manifest says ${got.bytes} B`;
    }
  }
  return '';
}

function stale(dir, problem, details = '') {
  return new Error(`${dir} is where this set belongs, and it does not hold it - ${problem}. ` +
    'Nothing here removes that directory, because a person may have changed it on purpose. Delete it and run again.' +
    (details ? `\n${details}` : ''));
}

// discard removes a set this process wrote and will not use. The files go
// through tfg cleanup, which removes only what the manifest lists, and then
// the manifest and the directories it leaves empty. Whatever is left stays,
// and a later run says so.
function discard(binary, dir) {
  const manifest = path.join(dir, 'manifest.json');
  if (fs.existsSync(manifest)) {
    const done = spawnSync(binary, ['cleanup', manifest, '--yes'], { stdio: 'ignore', windowsHide: true });
    if (done.status !== 0) {
      return;
    }
    fs.rmSync(manifest, { force: true });
    fs.rmSync(path.join(dir, 'manifest.instructions.md'), { force: true });
  }
  removeEmpty(dir);
}

function removeEmpty(dir) {
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    if (entry.isDirectory()) {
      removeEmpty(path.join(dir, entry.name));
    }
  }
  try {
    fs.rmdirSync(dir);
  } catch {
    // Not empty: left where it is.
  }
}

// noteUnfinished says, once per process and only outside a test worker, which
// sets a stopped run left behind. Nothing reads them and nothing removes them
// on its own, so without this they would fill the disk in silence.
function noteUnfinished(cacheDir) {
  if (noted || process.env.TEST_WORKER_INDEX !== undefined) {
    return;
  }
  noted = true;
  const left = fs.readdirSync(cacheDir).filter((name) => {
    const pid = Number(name.slice(unfinished.length).split('-')[0]);
    return name.startsWith(unfinished) && Number.isInteger(pid) && pid > 0 && !alive(pid);
  });
  if (left.length > 0) {
    console.warn(`testing-files-generator: ${left.length} unfinished ${left.length === 1 ? 'set was' : 'sets were'} left in ${cacheDir} ` +
      `by a run that stopped part way - ${left.join(', ')}. Nothing uses them and they can be deleted.`);
  }
}

function alive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return err.code === 'EPERM';
  }
}

// FileSet is a set, or a part of one. Every part shares the set's one write.
class FileSet {
  constructor(source, files) {
    this._source = source;
    this.files = files;
  }

  get plan() { return this._source.plan; }

  only(filter) { return new FileSet(this._source, this.files.filter((file) => matches(file, filter))); }

  except(filter) { return new FileSet(this._source, this.files.filter((file) => !matches(file, filter))); }

  // write writes the whole set, once per process, and gives its directory.
  write() { return this._source.write(); }

  async paths() {
    const dir = await this._source.write();
    return this.files.map((file) => path.join(dir, file.relativePath));
  }
}

function matches(file, filter = {}) {
  const has = (wanted, value) => wanted === undefined || [].concat(wanted).includes(value);
  return has(filter.group, file.group) && has(filter.target, file.target) && has(filter.outcome, file.expected.outcome);
}

module.exports = { events, fileSet, readable };
