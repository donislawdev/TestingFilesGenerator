// Runs the example twice and says whether it shows what it is for.
//
// With the four mistakes, the tests from the generated set fail on exactly the
// files each mistake lets through or turns away, and every typical test passes.
// With FIXED=1, nothing fails. The second run is the control: without it, red
// tests could come from the tests themselves rather than from the app.

import { spawnSync } from 'node:child_process';
import { existsSync, readFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';

// Playwright's own command line, run by this Node rather than through npx and
// a shell, which Windows would need for npx.
const playwright = createRequire(import.meta.url).resolve('@playwright/test/cli');

// Each mistake in app/server.mjs, and the tests it turns red.
const mistakes = {
  'through the page': {
    '1mb_at_limit.jpg': 4,
    'pdf_as_jpg.jpg': 2,
    'png_as_pdf.pdf': 2,
    'jpg_as_png.png': 2,
    'invoice.jpg.exe': 3,
  },
  'through the API': {
    '1mb_over_1b.jpg': 1,
    '1mb_far_over_2x.jpg': 1,
    'pdf_as_jpg.jpg': 2,
    'png_as_pdf.pdf': 2,
    'jpg_as_png.png': 2,
    'invoice.jpg.exe': 3,
  },
};

const reportFile = 'results/check.json';

// run runs the tests once and reads their report. Red tests end the run with
// a code other than nought, which is what this is here to see, so the code says
// nothing. A report left by an earlier run is removed first, or a run that
// never got as far as writing one would be read as that earlier run.
function run(fixed) {
  rmSync(reportFile, { force: true });
  const done = spawnSync(process.execPath, [playwright, 'test', '--reporter=json'], {
    stdio: ['ignore', 'ignore', 'inherit'],
    env: { ...process.env, FIXED: fixed ? '1' : '', PLAYWRIGHT_JSON_OUTPUT_NAME: reportFile },
  });
  if (done.error || done.signal || !existsSync(reportFile)) {
    const why = done.error?.message ?? (done.signal ? `it was stopped by ${done.signal}` : `it ended with code ${done.status} and wrote no ${reportFile}`);
    throw new Error(`Playwright did not run the tests - ${why}. Run npm install and npx playwright install chromium, then try again.`);
  }
  const report = JSON.parse(readFileSync(reportFile, 'utf8'));
  const tests = [];
  const walk = (suite, titles) => {
    for (const spec of suite.specs ?? []) {
      for (const one of spec.tests) {
        tests.push({ file: spec.file, describe: titles.at(-1) ?? '', title: spec.title, status: one.status });
      }
    }
    for (const child of suite.suites ?? []) {
      walk(child, [...titles, child.title]);
    }
  };
  for (const suite of report.suites) {
    walk(suite, []);
  }
  if (report.errors?.length) {
    throw new Error(`The run did not get as far as its tests: ${report.errors.map((e) => e.message).join('\n')}`);
  }
  return tests;
}

const problems = [];
const say = (problem) => problems.push(problem);

for (const fixed of [false, true]) {
  const mode = fixed ? 'with the mistakes fixed' : 'with the four mistakes';
  const tests = run(fixed);
  const ran = tests.filter((t) => t.status !== 'skipped');
  const generated = tests.filter((t) => t.file.endsWith('upload.spec.ts'));
  if (ran.length === 0 || generated.every((t) => t.status === 'skipped')) {
    say(`${mode}: no test from the generated set ran`);
  }
  for (const t of tests) {
    const name = t.title.split(' - expected ')[0];
    const mistake = mistakes[t.describe]?.[name];
    const shouldFail = !fixed && mistake !== undefined && t.file.endsWith('upload.spec.ts');
    const failed = t.status === 'unexpected';
    if (shouldFail && !failed) {
      say(`${mode}: ${t.describe} - ${name} should be red for mistake ${mistake} and is ${t.status}`);
    }
    if (!shouldFail && failed) {
      say(`${mode}: ${t.describe ? `${t.describe} - ` : ''}${t.title} failed, and nothing in the app should make it`);
    }
  }
  const red = tests.filter((t) => t.status === 'unexpected').length;
  const skipped = tests.filter((t) => t.status === 'skipped').length;
  console.log(`${mode}: ${red} red, ${tests.length - red - skipped} green, ${skipped} skipped as unspecified`);
}

const found = new Set(Object.values(mistakes).flatMap((files) => Object.values(files)));
for (const n of [1, 2, 3, 4]) {
  if (!found.has(n)) {
    say(`mistake ${n} has no test that finds it`);
  }
}

if (problems.length > 0) {
  console.error(`The example does not show what it is for:\n- ${problems.join('\n- ')}`);
  process.exit(1);
}
console.log('The generated set finds all four mistakes, the typical tests find none, and with the mistakes fixed nothing is red.');
