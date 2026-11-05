import { test, expect } from '@playwright/test';
import { createReadStream } from 'node:fs';
import { fileSet, type Observed } from 'testing-files-generator';

import { limit } from '../shared';

// The set the upload-validation preset builds around the form's limit: either
// side of it, types the form takes and turns away, content under the wrong
// name, names that are hard to store. The form takes one file at a time, so
// the mass upload is left out.
const uploads = fileSet({ preset: 'upload-validation', options: { limit, bulk: 0 } });

test.beforeAll(() => uploads.write());

test.describe('through the page', () => {
  for (const file of uploads.files) {
    test(file.title, file.details, async ({ page }) => {
      test.skip(file.unspecified, file.whyUnspecified);
      await page.goto('/');
      await page.getByLabel('File').setInputFiles(await file.path());
      await page.getByRole('button', { name: 'Upload' }).click();
      const status = page.getByRole('status');
      await expect(status).not.toBeEmpty();
      file.check(shown(await status.textContent()));
    });
  }
});

// The same files sent to the server directly, the way anything other than this
// page can send them.
test.describe('through the API', () => {
  for (const file of uploads.files) {
    test(file.title, file.details, async ({ request }) => {
      test.skip(file.unspecified, file.whyUnspecified);
      const response = await request.post('/upload', { multipart: { file: createReadStream(await file.path()) } });
      const answer = await response.json();
      file.check(response.ok() ? 'accept' : { outcome: 'reject', reason: answer.reason });
    });
  }
});

function shown(text: string | null): Observed {
  const refused = /^Refused \((\w+)\)$/.exec(text ?? '');
  return refused ? { outcome: 'reject', reason: refused[1] } : 'accept';
}
