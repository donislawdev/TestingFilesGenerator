# An upload form, the tests it usually gets, and the tests a generated set gives it

`app/server.mjs` is an upload form with four mistakes, each one a mistake real upload forms have:

1. The server never checks the size. Only the page does.
2. The content and the extension are each checked against the allowed types, and never against each other, so a PDF named `.jpg` goes through.
3. An allowed extension anywhere in the name counts, so `invoice.jpg.exe` is taken for a picture.
4. The page refuses a file of exactly the limit, which is the largest size allowed.

`tests/typical.spec.ts` is what such a form usually gets: a real picture that should go through, and files made in the test with `Buffer.alloc` that should not. Every one of those tests passes. The file over the limit is zeros rather than a PDF, so the server turns it away for what it holds, and the test meant to prove the size limit passes on a server that has none.

`tests/upload.spec.ts` sends the `upload-validation` set from [testing-files-generator](../../npm/testing-files-generator) through the page and straight to the server. It finds all four mistakes, and each failure says what it means.

## Run it

From the root of the repository, build tfg first. The example finds it there.

```bash
go build -tags "$(cat .github/build-tags)" -o tfg.exe ./cmd/tfg
```

```bash
cd examples/playwright && npm install && npx playwright install chromium
```

```bash
npx playwright test
```

Eleven tests fail, five through the page and six through the API, and every failure names its file and the mistake it shows:

```
1mb_over_1b.jpg was taken, and it should have been turned away (size_limit).
A JPEG file 1 B over the limit of 1mb. Your system should turn it away - taking it means
the limit is not enforced, or enforced only in the browser.
```

Ten are skipped, because what a form does with an empty file or a name full of emoji is its own policy, and the set says so rather than pass them.

`FIXED=1 npx playwright test` corrects all four mistakes, and nothing fails. `npm run check` runs both and says whether the example still shows what it is for. The repository's CI runs that.
