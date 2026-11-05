// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

const path = require('node:path');
const { inspect } = require('node:util');

// The closed list of outcomes a manifest declares, from docs/MANIFEST.md. A
// guard holds this copy, and the one in index.d.ts, to recipe.Outcomes in the
// program, because two copies of a closed list is how they drift.
const outcomes = ['accept', 'reject', 'sanitize', 'unspecified'];

// How each outcome reads in a sentence about what a system did with a file.
const done = {
  accept: 'taken',
  reject: 'turned away',
  sanitize: 'taken and cleaned',
};

// A tag is a word Playwright filters on, so anything else in a group's name
// becomes a hyphen rather than breaking the filter.
function tag(word) {
  return `@${String(word).replace(/[^A-Za-z0-9_-]+/g, '-')}`;
}

function expectedText(expected) {
  return expected.reason ? `${expected.outcome} (${expected.reason})` : expected.outcome;
}

// TestFile is one file of a set, with what the system under test should do
// with it. It is what a test declares itself from: a title, the details a test
// takes, the path once the set is written, and a check against the manifest.
class TestFile {
  constructor(source, entry) {
    this._source = source;
    this.entry = entry;
  }

  get relativePath() { return this.entry.path; }
  get name() { return this.entry.name; }
  get format() { return this.entry.format; }
  get bytes() { return this.entry.bytes; }
  get group() { return this.entry.group ?? ''; }
  get target() { return this.entry.target_id; }
  get purpose() { return this.entry.purpose ?? ''; }
  get expected() { return this.entry.expected; }
  get unspecified() { return this.entry.expected.outcome === 'unspecified'; }

  // The path, which is unique in a set and is what a test title has to be. It
  // equals the name today, because a name cannot hold a separator, and the
  // path stays the right one if a set ever has directories.
  get title() {
    return `${this.entry.path} - expected ${expectedText(this.entry.expected)}`;
  }

  // details has the shape test(title, details, body) takes, so a report shows
  // what each file is for and a run can be narrowed by its tags.
  get details() {
    const tags = [tag(this.entry.expected.outcome)];
    if (this.group) {
      tags.push(tag(this.group));
    }
    const annotation = [{ type: 'expected', description: expectedText(this.entry.expected) }];
    if (this.purpose) {
      annotation.push({ type: 'purpose', description: this.purpose });
    }
    return { tag: tags, annotation };
  }

  get whyUnspecified() {
    const why = `${this.entry.path} has no expected outcome - what your system does with it is its own policy.`;
    return this.purpose ? `${why} ${this.purpose}` : why;
  }

  // path writes the set the first time any of its files is asked for, and
  // gives the absolute path of this one.
  async path() {
    const dir = await this._source.write();
    return path.join(dir, this.entry.path);
  }

  // check compares what the system did with what the manifest expects, and
  // throws with the file's purpose when they differ - that sentence says what
  // the difference means. A reason is compared only for a refusal: a reason
  // names the rule a file is about, so a file a byte under a limit is accepted
  // with size_limit and there is nothing to compare it with.
  check(observed) {
    try {
      this._compare(observed);
    } catch (err) {
      // The stack starts at whoever called check, so a test runner shows the
      // line of the test rather than a line of this package.
      Error.captureStackTrace(err, TestFile.prototype.check);
      throw err;
    }
  }

  _compare(observed) {
    const want = this.entry.expected;
    if (want.outcome === 'unspecified') {
      throw new Error(`${this.entry.path} has no expected outcome, so there is nothing to check it against. ` +
        'Skip it with test.skip(file.unspecified, file.whyUnspecified), or decide what your system should do with it and check that.');
    }
    const got = typeof observed === 'string' ? { outcome: observed } : observed;
    if (!got || typeof got !== 'object' || !Object.hasOwn(done, got.outcome)) {
      throw new TypeError('check takes accept, reject or sanitize, or an object with an outcome and a reason. ' +
        `It was given ${inspect(observed)}.`);
    }
    if (got.outcome !== want.outcome) {
      throw mismatch(this, `${this.entry.path} was ${done[got.outcome]}, and it should have been ${done[want.outcome]}` +
        (want.reason ? ` (${want.reason}).` : '.'), got);
    }
    if (want.outcome === 'reject' && want.reason && got.reason && got.reason !== want.reason) {
      throw mismatch(this, `${this.entry.path} was turned away for ${got.reason}, and the rule it is about is ${want.reason}.`, got);
    }
  }
}

function mismatch(file, sentence, observed) {
  const err = new Error(file.purpose ? `${sentence}\n${file.purpose}` : sentence);
  err.name = 'ExpectationError';
  err.expected = file.entry.expected;
  err.observed = observed;
  return err;
}

module.exports = { TestFile, outcomes };
