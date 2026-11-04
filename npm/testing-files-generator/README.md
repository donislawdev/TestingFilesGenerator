# testing-files-generator

Real files of exact sizes and formats for your tests, and what the system under test should do with each one.

A PDF of exactly 1 MiB and one byte is a real PDF, so a server that looks inside it can only turn it away for its size. A file made with `Buffer.alloc` is zeros, so the same server turns it away for what it holds, and a test meant to prove a size limit passes on a server that has none. [The example](../../examples/playwright) shows exactly that.

## In a Playwright test

```ts
import { test, expect } from '@playwright/test';
import { fileSet } from 'testing-files-generator';

const uploads = fileSet({ preset: 'upload-validation', options: { limit: '2mb' } });

for (const file of uploads.except({ group: 'bulk-upload' }).files) {
  test(file.title, file.details, async ({ page }) => {
    test.skip(file.unspecified, file.whyUnspecified);
    await page.goto('/upload');
    await page.getByLabel('File').setInputFiles(await file.path());
    await page.getByRole('button', { name: 'Upload' }).click();
    const status = page.getByRole('status');
    await expect(status).not.toBeEmpty();
    file.check((await status.textContent())?.includes('Uploaded') ? 'accept' : 'reject');
  });
}
```

You write the two things only you know: how a file is uploaded in your app, and how it shows that a file was taken or turned away. The set brings the rest.

- **One test for each file.** The title names the file and what it should meet, such as `1mb_over_1b.jpg - expected reject (size_limit)`. The report shows what the file is for, and tags such as `@reject` and `@size-limit` narrow a run with `--grep`.
- **A failure says what it means.** `check` throws with the sentence the set carries for that file:

  ```
  1mb_over_1b.jpg was taken, and it should have been turned away (size_limit).
  A JPEG file 1 B over the limit of 1mb. Your system should turn it away - taking it means
  the limit is not enforced, or enforced only in the browser.
  ```

- **Files whose outcome is your policy are skipped, never passed.** An empty file or a name with an emoji in it has no right answer until you decide one. `whyUnspecified` says so in the report, and `check` refuses to pass such a file.
- **The same files can go straight to your server**, past the page and its checks:

  ```ts
  import { createReadStream } from 'node:fs';

  const response = await request.post('/upload', { multipart: { file: createReadStream(await file.path()) } });
  file.check(response.ok() ? 'accept' : 'reject');
  ```

  `check` also takes `{ outcome: 'reject', reason: 'size_limit' }`. A reason is compared only for a refusal, because it names the rule a file is about - a file a byte under a limit is expected to be accepted with `size_limit` as the rule in play.

## The sets

`fileSet` takes a preset, a recipe file or plain flags of `tfg generate`:

```ts
fileSet({ preset: 'size-boundaries', options: { limit: '10mb', format: 'pdf' } });
fileSet({ recipe: 'fixtures/uploads.yaml' });
fileSet({ options: { format: 'png', size: '5mb', count: 3, seed: 7, expected: 'accept' } });
```

Files made from flags expect nothing unless `expected` says what, so without it every one of them is skipped.

| preset | the question it answers |
|---|---|
| `upload-validation` | Does my upload form take what it should and turn the rest away? |
| `size-boundaries` | Is a size limit enforced exactly where it is declared? |
| `filename-handling` | Will my system store, show and give back a file name it did not expect? |
| `tabular-import` | Does my table import survive what real tools export? |
| `text-encoding` | Does my reader know which encoding a file is in, or is it guessing? |
| `empty-and-minimal` | Does a file that is valid and as small as the format allows get through? |

`npx tfg preset show <id>` lists what a preset takes, and `npx tfg formats` lists the formats.

`only` and `except` take part of a set by `group`, `target` or `outcome`, and `paths()` gives the files of a part together, for a form that takes several at once:

```ts
const together = await uploads.only({ group: 'bulk-upload' }).paths();
await page.getByLabel('Files').setInputFiles(together);
```

## When the files are written

1. **While Playwright loads the spec file, nothing is written.** `fileSet` asks for the plan, the list of files and what each should meet, and Playwright declares the tests from it. Listing the tests with `--list` writes nothing either.
2. **The first test that asks for a path writes the whole set**, once, however many workers ask at the same moment. The default `upload-validation` set, 71 files and 116 MB, was written in under a second on the machine it was measured on.
3. **Every later run takes the same files.** The same options give the same bytes, so a set is written once and kept, in `node_modules/.cache/testing-files-generator` or the directory `TFG_CACHE_DIR` names.

A large set is better written before the tests start, so its time does not count against the first test:

```ts
test.beforeAll(async () => {
  test.setTimeout(120_000);
  await uploads.write();
});
```

Nothing is ever removed on its own. Delete the cache directory whenever you like. A run stopped part way leaves an unfinished set, which nothing uses, and the next run names it so it does not sit there unseen.

## Where tfg comes from

The package installs the `tfg` program for your system: Windows on x64 and Arm, Linux on x64 and Arm, macOS on Apple silicon. `npx tfg` runs it, with the same arguments, output and exit codes as tfg run directly.

To use a tfg installed another way, set `TFG_BINARY` to its path, or pass `binary` to `fileSet`. On a system with no binary of its own, such as an Intel Mac, the container image `ghcr.io/donislawdev/tfg` runs the same program.

## Not only Playwright

`fileSet` gives plain data and plain promises, and imports nothing from Playwright. The same loop works in Vitest, Jest or a script, with `file.title` as a test name.

## Licences

This package is under the [Mozilla Public License 2.0](LICENSE). The `tfg` program it runs is under the GNU General Public License 3.0 and runs as a separate program. The files it generates are yours: they are output of the program and not derived works of it, so you can commit them and ship them inside a closed source product with no obligation of any kind.
