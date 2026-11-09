'use strict';

// What a file says of itself and what check makes of what a system did, read
// from the file the Python package's tests read as well, so a sentence changed
// in one package and not in the other fails here or there.

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

const { TestFile } = require('../lib/testfile.js');

const vectors = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', '..', 'testdata', 'wrappers', 'vectors.json'), 'utf8'));

function file(name) {
  const entry = vectors.entries[name];
  assert.ok(entry, `vectors.json has no entry ${name}`);
  return new TestFile({ write: async () => '/set' }, entry);
}

test('every file says what the vectors say it says', () => {
  assert.ok(vectors.files.length >= 4);
  for (const want of vectors.files) {
    const f = file(want.entry);
    assert.equal(f.title, want.title, want.entry);
    assert.deepEqual(f.details.tag, want.marks.map((m) => `@${m}`), want.entry);
    assert.equal(f.whyUnspecified, want.why_unspecified, want.entry);
  }
});

test('check makes of every observation what the vectors say it makes', () => {
  const kinds = new Set();
  for (const c of vectors.checks) {
    const label = `${c.entry} ${JSON.stringify(c.observed)}`;
    kinds.add(c.result);
    const f = file(c.entry);
    let thrown;
    try {
      f.check(c.observed);
    } catch (err) {
      thrown = err;
    }
    switch (c.result) {
      case 'pass':
        assert.equal(thrown, undefined, label);
        break;
      case 'mismatch':
        assert.equal(thrown?.name, 'ExpectationError', label);
        assert.equal(thrown.message, c.message, label);
        break;
      case 'unspecified':
        assert.ok(thrown && thrown.name === 'Error', label);
        assert.match(thrown.message, /has no expected outcome, so there is nothing to check it against/, label);
        break;
      case 'bad-input':
        assert.ok(thrown instanceof TypeError, label);
        break;
      default:
        assert.fail(`vectors.json has a result ${c.result} this test does not know`);
    }
  }
  // Every kind is there, so a vectors file that lost one is not green by
  // having nothing left to compare.
  assert.deepEqual([...kinds].sort(), ['bad-input', 'mismatch', 'pass', 'unspecified']);
});
