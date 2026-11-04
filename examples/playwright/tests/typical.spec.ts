import { test, expect } from '@playwright/test';

import { limitBytes } from '../shared';

// The tests an upload form usually gets: one real picture that should go
// through, and files made in the test that should not. Against this app every
// one of them passes, while the app has the four mistakes upload.spec.ts finds.
//
// The file over the limit is the reason. Buffer.alloc gives zeros, not a PDF,
// so the server turns it away for what it holds - and the test that was meant
// to prove a size limit passes on a server that has none.

// A picture of one pixel, the kind of fixture kept as text in a test.
const photo = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==', 'base64');
const tooLarge = { name: 'big.pdf', mimeType: 'application/pdf', buffer: Buffer.alloc(limitBytes + 1) };

test('a photo is uploaded', async ({ page }) => {
  await page.goto('/');
  await page.getByLabel('File').setInputFiles({ name: 'photo.png', mimeType: 'image/png', buffer: photo });
  await page.getByRole('button', { name: 'Upload' }).click();
  await expect(page.getByRole('status')).toHaveText('Uploaded');
});

test('a file over the limit is refused in the page', async ({ page }) => {
  await page.goto('/');
  await page.getByLabel('File').setInputFiles(tooLarge);
  await page.getByRole('button', { name: 'Upload' }).click();
  await expect(page.getByRole('status')).toHaveText(/^Refused/);
});

test('a file over the limit is refused by the server', async ({ request }) => {
  const response = await request.post('/upload', { multipart: { file: tooLarge } });
  expect(response.ok()).toBe(false);
});

test('an executable is refused', async ({ request }) => {
  const response = await request.post('/upload', {
    multipart: { file: { name: 'setup.exe', mimeType: 'application/octet-stream', buffer: Buffer.from('MZ') } },
  });
  expect(response.ok()).toBe(false);
});
