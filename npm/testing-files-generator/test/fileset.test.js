'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

const { fileSet, readable } = require('../lib/fileset.js');
const { TfgError } = require('../lib/run.js');
const { node, pkg, small, tempDir, tfg } = require('./helpers.js');

function plannedIn(cacheDir, spec = small) {
  return fileSet({ ...spec, binary: tfg(), cacheDir });
}

// The script a fresh process runs to write a set, printing its directory.
function writer(cacheDir, spec = small) {
  return `require(${pkg}).fileSet({ ...${JSON.stringify(spec)}, cacheDir: ${JSON.stringify(cacheDir)} })` +
    '.write().then((dir) => console.log(dir), (err) => { console.error(err.message); process.exit(3); });';
}

function entries(cacheDir) {
  return fs.readdirSync(cacheDir).sort();
}

test('a plan writes nothing, and says every file with what is expected of it', (t) => {
  const cacheDir = tempDir(t);
  const set = plannedIn(cacheDir);
  assert.deepEqual(entries(cacheDir), [], 'a plan left something in the cache');
  assert.equal(set.files.length, set.plan.summary.file_count);
  assert.ok(set.files.length >= 20, `the set this test asks for holds ${set.files.length} files`);
  assert.equal(new Set(set.files.map((f) => f.title)).size, set.files.length, 'two files share a title');
  assert.equal(set.only({ outcome: 'unspecified' }).files.length, set.plan.summary.by_expected.unspecified);
  assert.equal(set.only({ group: 'bulk-upload' }).files.length, 2);
  assert.equal(set.except({ group: 'bulk-upload' }).files.length, set.files.length - 2);
});

test('a set is written once, whole, and its files are where their paths say', async (t) => {
  const cacheDir = tempDir(t);
  const set = plannedIn(cacheDir);
  const dir = await set.write();
  assert.equal(await set.write(), dir);
  assert.deepEqual(entries(cacheDir), [path.basename(dir)]);
  for (const file of set.files) {
    assert.equal(fs.statSync(await file.path()).size, file.bytes, file.relativePath);
  }
});

// racer plans a set, waits until every racer has planned, and only then
// writes, so that all of them find the set unwritten and write it at once. A
// racer that fails before it reaches the barrier would hold the others there
// for ever, so the wait has an end.
function racer(cacheDir, barrier, racers) {
  const lib = JSON.stringify(path.join(__dirname, '..', 'lib', 'fileset.js'));
  const at = JSON.stringify(barrier);
  return `const fs = require('node:fs');
const { fileSet, events } = require(${lib});
const said = [];
for (const e of ['wrote', 'moved', 'took']) events.on(e, () => said.push(e));
const set = fileSet({ ...${JSON.stringify(small)}, cacheDir: ${JSON.stringify(cacheDir)} });
fs.writeFileSync(require('node:path').join(${at}, String(process.pid)), '');
const until = Date.now() + 30000;
const ready = () => fs.readdirSync(${at}).length >= ${racers} ? Promise.resolve()
  : Date.now() > until ? Promise.reject(new Error('not every racer planned within 30 s'))
  : new Promise((r) => setTimeout(r, 5)).then(ready);
ready().then(() => set.write()).then((dir) => console.log(JSON.stringify({ dir, said })),
  (err) => { console.error(err.message); process.exit(3); });`;
}

test('workers writing one set at once end with one copy and nothing left over', { timeout: 120000 }, async (t) => {
  const cacheDir = tempDir(t);
  const barrier = tempDir(t);
  const racers = 4;
  const runs = await Promise.all(Array.from({ length: racers }, () => node(racer(cacheDir, barrier, racers))));
  // Every failed racer is shown, because the first to fail may only be one
  // that waited for another.
  assert.deepEqual(runs.filter((r) => r.status !== 0).map((r) => r.stderr), [], 'a racer failed');
  const results = runs.map((r) => JSON.parse(r.stdout));
  const said = results.flatMap((r) => r.said);
  const count = (e) => said.filter((s) => s === e).length;
  assert.equal(count('wrote'), racers, `not every worker wrote, so there was no race: ${said.join(', ')}`);
  assert.equal(count('moved'), 1, said.join(', '));
  assert.equal(count('took'), racers - 1, said.join(', '));
  const dirs = new Set(results.map((r) => r.dir));
  assert.equal(dirs.size, 1, `the workers were given ${[...dirs].join(', ')}`);
  assert.deepEqual(entries(cacheDir), [path.basename([...dirs][0])]);
  const set = plannedIn(cacheDir);
  assert.equal(await set.write(), [...dirs][0]);
});

test('a later run takes the set an earlier one wrote', async (t) => {
  const cacheDir = tempDir(t);
  const first = await node(writer(cacheDir));
  assert.equal(first.status, 0, first.stderr);
  const dir = first.stdout.trim();
  const before = fs.statSync(path.join(dir, 'manifest.json')).mtimeMs;
  const second = await node(writer(cacheDir));
  assert.equal(second.stdout.trim(), dir);
  assert.equal(fs.statSync(path.join(dir, 'manifest.json')).mtimeMs, before, 'the set was written again');
});

test('another seed or --clean is another set, though the recipe hash is the same', async (t) => {
  const cacheDir = tempDir(t);
  const plain = plannedIn(cacheDir);
  const seeded = plannedIn(cacheDir, { ...small, options: { ...small.options, seed: 7 } });
  const clean = plannedIn(cacheDir, { ...small, options: { ...small.options, clean: true } });
  assert.equal(seeded.plan.run.recipe_hash, plain.plan.run.recipe_hash, 'the hash now covers the seed - this test is no longer about anything');
  const dirs = new Set([await plain.write(), await seeded.write(), await clean.write()]);
  assert.equal(dirs.size, 3);
});

test('a set changed by hand is refused, and left where it is', async (t) => {
  const cacheDir = tempDir(t);
  const first = await node(writer(cacheDir));
  const dir = first.stdout.trim();
  const gone = path.join(dir, '1mb_at_limit.jpg');
  fs.rmSync(gone);
  const second = await node(writer(cacheDir));
  assert.equal(second.status, 3);
  assert.match(second.stderr, /does not hold it - 1mb_at_limit\.jpg is missing\. Nothing here removes that directory/);
  assert.ok(fs.existsSync(path.join(dir, 'manifest.json')), 'the changed set was removed');
});

test('a file of the set cut short by hand is refused', async (t) => {
  const cacheDir = tempDir(t);
  const dir = (await node(writer(cacheDir))).stdout.trim();
  fs.truncateSync(path.join(dir, '1mb_at_limit.jpg'), 10);
  const again = await node(writer(cacheDir));
  assert.equal(again.status, 3);
  assert.match(again.stderr, /1mb_at_limit\.jpg is 10 B and its manifest says 1048576 B/);
});

test('a file of the set written over by hand, its size the same, is refused', async (t) => {
  const cacheDir = tempDir(t);
  const dir = (await node(writer(cacheDir))).stdout.trim();
  const changed = path.join(dir, '1mb_at_limit.jpg');
  const fd = fs.openSync(changed, 'r+');
  fs.writeSync(fd, Buffer.from('changed'), 0, 7, 0);
  fs.closeSync(fd);
  const again = await node(writer(cacheDir));
  assert.equal(again.status, 3, again.stderr);
  assert.match(again.stderr, /its files are not the ones it was written with\. Nothing here removes that directory/);
  assert.match(again.stderr, /wrong-hash\s+1mb_at_limit\.jpg/);
  assert.ok(fs.existsSync(changed), 'the changed set was removed');
});

test('a binary or a recipe the options name and that is not there is named, with what to do', (t) => {
  const cacheDir = tempDir(t);
  const missing = path.join(cacheDir, 'no-such-tfg');
  assert.throws(() => fileSet({ ...small, binary: missing, cacheDir }),
    (err) => err.message.includes(`The binary option names ${missing}, and there is no file there.`) && err.message.includes('leave it out'));
  assert.throws(() => fileSet({ recipe: 'no-such-recipe.yaml', binary: tfg(), cacheDir }),
    (err) => err.message.includes(path.resolve('no-such-recipe.yaml')) && err.message.includes('path.join(__dirname'));
});

test('a set a stopped run left behind is named once outside the workers, and never used', async (t) => {
  const cacheDir = tempDir(t);
  const gone = await node('');
  const left = path.join(cacheDir, `.tmp-${gone.pid}-0bad0bad`);
  fs.mkdirSync(left);
  const plan = `require(${pkg}).fileSet({ ...${JSON.stringify(small)}, cacheDir: ${JSON.stringify(cacheDir)} })`;
  const runner = await node(plan, { env: { TEST_WORKER_INDEX: undefined } });
  assert.equal(runner.status, 0, runner.stderr);
  const worker = await node(plan, { env: { TEST_WORKER_INDEX: '0' } });
  assert.equal(worker.status, 0, worker.stderr);
  assert.equal(worker.stderr, '', 'a worker spoke about the cache');
  assert.match(runner.stderr, new RegExp(`1 unfinished set was left in .*\\.tmp-${gone.pid}-0bad0bad\\. Nothing uses them`));
  assert.ok(fs.existsSync(left), 'the unfinished set was removed');
});

test('what tfg refuses reaches the test with its own words and code', (t) => {
  const cacheDir = tempDir(t);
  assert.throws(() => plannedIn(cacheDir, { preset: 'upload-validation', options: { limit: '1kb' } }),
    (err) => err instanceof TfgError && err.status === 4 && /smallest PDF/.test(err.message));
});

test('a refused plan is told in the words of the call, without the flags this package adds', (t) => {
  const cacheDir = tempDir(t);
  assert.throws(() => plannedIn(cacheDir, { preset: 'no-such-preset' }),
    (err) => err instanceof TfgError && err.status === 2 &&
      err.message.startsWith("fileSet({ preset: 'no-such-preset' }) could not be planned - tfg ended with code 2.\n") &&
      /unknown preset "no-such-preset"/.test(err.message) && !/dry-run|--json|\.tmp-/.test(err.message));
});

test('an option tfg does not know is named with the options it takes, and what tfg said stays on the error', (t) => {
  const cacheDir = tempDir(t);
  const spec = { ...small, options: { ...small.options, limt: '2mb' } };
  assert.throws(() => plannedIn(cacheDir, spec), (err) => {
    assert.ok(err instanceof TfgError, err);
    assert.equal(err.status, 2);
    assert.ok(err.message.includes('gives the option limt, and tfg generate has no flag of that name.'), err.message);
    assert.ok(!err.message.includes('flag provided'), 'the help of tfg is in the message');
    assert.ok(err.stderr.includes('flag provided but not defined: -limt'), err.stderr);
    const taken = /The options it takes here are ([^.]+)\./.exec(err.message)?.[1].split(', ') ?? [];
    for (const name of ['limit', 'far-over', 'bulk', 'seed']) {
      assert.ok(taken.includes(name), `${name} is not among ${taken}`);
    }
    for (const name of ['out', 'json', 'dry-run', 'preset']) {
      assert.ok(!taken.includes(name), `${name} is set by this package and is offered as an option`);
    }
    assert.ok(err.message.includes('npx tfg preset show upload-validation'), err.message);
    return true;
  });
});

test('a call that names no files to write is told so before tfg runs', (t) => {
  const cacheDir = tempDir(t);
  for (const spec of [{}, { options: {} }]) {
    assert.throws(() => fileSet({ ...spec, binary: tfg(), cacheDir }), /fileSet was not told which files to write/);
  }
  assert.deepEqual(entries(cacheDir), [], 'something ran');
});

test('the flags this package sets itself cannot be set again', (t) => {
  const cacheDir = tempDir(t);
  for (const flag of ['out', 'json', 'dry-run', 'preset']) {
    assert.throws(() => plannedIn(cacheDir, { preset: 'upload-validation', options: { [flag]: 'x' } }),
      new RegExp(`The option ${flag} cannot be set here`));
  }
  assert.throws(() => plannedIn(cacheDir, { preset: 'upload-validation', options: { '--limit': '1mb' } }), /not a flag tfg has/);
  assert.throws(() => plannedIn(cacheDir, { preset: 'upload-validation', options: { limit: { mb: 1 } } }), TypeError);
});

test('a manifest of another major, or an outcome this package does not know, is refused', () => {
  const plan = { manifest_version: '1.0', files: [{ path: 'a', expected: { outcome: 'accept' } }] };
  readable(plan);
  assert.throws(() => readable({ ...plan, manifest_version: '2.0' }), /manifest of version 2\.0, and this package reads version 1/);
  assert.throws(() => readable({ ...plan, files: [{ path: 'a', expected: { outcome: 'quarantine' } }] }), /"quarantine", which this package does not know/);
});

test('npx tfg is tfg - the same output and the same exit code', async () => {
  const bin = path.join(__dirname, '..', 'bin', 'tfg.js');
  const viaPackage = await node('', { file: bin, args: ['generate', '--format', 'nope', '--dry-run'] });
  const direct = await new Promise((resolve) => {
    const child = require('node:child_process').spawn(tfg(), ['generate', '--format', 'nope', '--dry-run']);
    let stderr = '';
    child.stderr.on('data', (c) => { stderr += c; });
    child.on('close', (status) => resolve({ status, stderr }));
  });
  assert.notEqual(direct.status, 0);
  assert.equal(viaPackage.status, direct.status);
  assert.equal(viaPackage.stderr, direct.stderr);
});
