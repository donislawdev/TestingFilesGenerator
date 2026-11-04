'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');

const { TestFile } = require('../lib/testfile.js');

function file(expected, extra = {}) {
  return new TestFile({ write: async () => '/set' }, {
    path: 'big.jpg', name: 'big.jpg', target_id: 'over', bytes: 9, format: 'jpg',
    group: 'size limit', purpose: 'Taking it means the limit is not enforced.', expected, ...extra,
  });
}

test('a file says what it is expected to meet in its title, its tags and its annotations', () => {
  const f = file({ outcome: 'reject', reason: 'size_limit' });
  assert.equal(f.title, 'big.jpg - expected reject (size_limit)');
  assert.deepEqual(f.details.tag, ['@reject', '@size-limit']);
  assert.deepEqual(f.details.annotation, [
    { type: 'expected', description: 'reject (size_limit)' },
    { type: 'purpose', description: 'Taking it means the limit is not enforced.' },
  ]);
});

test('a difference is thrown with the sentence that says what it means', () => {
  const f = file({ outcome: 'reject', reason: 'size_limit' });
  assert.throws(() => f.check('accept'), (err) => err.name === 'ExpectationError' &&
    err.message === 'big.jpg was taken, and it should have been turned away (size_limit).\nTaking it means the limit is not enforced.' &&
    err.observed.outcome === 'accept');
  f.check('reject');
  f.check({ outcome: 'reject', reason: 'size_limit' });
  f.check({ outcome: 'reject' });
});

test('a refusal for another rule is a difference, a reason on an accepted file is not', () => {
  assert.throws(() => file({ outcome: 'reject', reason: 'size_limit' }).check({ outcome: 'reject', reason: 'mime_mismatch' }),
    /turned away for mime_mismatch, and the rule it is about is size_limit/);
  file({ outcome: 'accept', reason: 'size_limit' }).check({ outcome: 'accept', reason: 'anything' });
});

test('an unspecified file is never passed by check', () => {
  const f = file({ outcome: 'unspecified', reason: 'size_zero' });
  assert.equal(f.unspecified, true);
  assert.match(f.whyUnspecified, /^big\.jpg has no expected outcome/);
  assert.throws(() => f.check('accept'), /nothing to check it against/);
  assert.throws(() => f.check('reject'), /nothing to check it against/);
});

test('check refuses what is not an outcome, the names an object inherits included', () => {
  const f = file({ outcome: 'accept' });
  for (const bad of ['toString', 'unspecified', 'ACCEPT', undefined, null, 3, {}]) {
    assert.throws(() => f.check(bad), TypeError, String(bad));
  }
});

test('the path is the set directory joined with the file', async () => {
  const path = require('node:path');
  assert.equal(await file({ outcome: 'accept' }).path(), path.join('/set', 'big.jpg'));
});
