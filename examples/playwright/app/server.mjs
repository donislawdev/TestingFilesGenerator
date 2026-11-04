// An upload form with four mistakes made on purpose, each one a mistake real
// upload forms have. playwright.config.ts starts it. FIXED=1 corrects all four,
// which is how check.mjs shows that the red tests come from the mistakes and
// not from the tests.
//
// It uses nothing outside Node, so the example has no server dependency.

import { readFileSync } from 'node:fs';
import http from 'node:http';
import { Readable } from 'node:stream';

const limit = Number(process.env.UPLOAD_LIMIT_BYTES);
const port = Number(process.env.PORT);
const fixed = process.env.FIXED === '1';
const allowed = ['jpg', 'png', 'pdf'];

if (!Number.isInteger(limit) || !Number.isInteger(port)) {
  throw new Error('Set UPLOAD_LIMIT_BYTES and PORT. playwright.config.ts sets both.');
}

// What a file holds, read from its first bytes rather than from its name.
function sniff(bytes) {
  if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) {
    return 'jpg';
  }
  if (bytes.subarray(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]))) {
    return 'png';
  }
  if (bytes.subarray(0, 5).toString('latin1') === '%PDF-') {
    return 'pdf';
  }
  return '';
}

function extensionOf(name) {
  const lower = name.toLowerCase();
  if (fixed) {
    const dot = lower.lastIndexOf('.');
    return dot > 0 ? lower.slice(dot + 1) : '';
  }
  // Mistake 3: an allowed extension anywhere in the name counts, so
  // invoice.jpg.exe is taken for a picture.
  return allowed.find((ext) => lower.includes(`.${ext}`)) ?? '';
}

function verdict(name, bytes) {
  // Mistake 1: the server never looks at the size. The page does, so a test
  // through the page sees a limit that a request straight to the server does not.
  if (fixed && bytes.length > limit) {
    return { status: 413, reason: 'size_limit' };
  }
  const extension = extensionOf(name);
  if (!allowed.includes(extension)) {
    return { status: 422, reason: 'extension_rule' };
  }
  const content = sniff(bytes);
  if (!allowed.includes(content)) {
    return { status: 415, reason: 'content_malformed' };
  }
  // Mistake 2: the content and the extension are each checked against the
  // allowed list and never against each other, so a PDF named .jpg goes through.
  if (fixed && content !== extension) {
    return { status: 415, reason: 'mime_mismatch' };
  }
  return { status: 201 };
}

// The page carries the limit and the mode, and page.js reads them from it.
const page = readFileSync(new URL('index.html', import.meta.url), 'utf8')
  .replace('{{limit}}', String(limit))
  .replace('{{fixed}}', fixed ? '1' : '');
const script = readFileSync(new URL('page.js', import.meta.url), 'utf8');

async function upload(req, res) {
  const request = new Request(`http://localhost${req.url}`, {
    method: 'POST', headers: req.headers, body: Readable.toWeb(req), duplex: 'half',
  });
  const file = (await request.formData()).get('file');
  if (!file || typeof file === 'string') {
    return answer(res, 400, { reason: 'none' });
  }
  const { status, reason } = verdict(file.name, Buffer.from(await file.arrayBuffer()));
  return answer(res, status, reason ? { reason } : { stored: file.name });
}

function answer(res, status, body) {
  res.writeHead(status, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/') {
    res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' });
    res.end(page);
    return;
  }
  if (req.method === 'GET' && req.url === '/page.js') {
    res.writeHead(200, { 'content-type': 'text/javascript; charset=utf-8' });
    res.end(script);
    return;
  }
  if (req.method === 'POST' && req.url === '/upload') {
    upload(req, res).catch((err) => answer(res, 500, { reason: 'none', error: String(err) }));
    return;
  }
  res.writeHead(404);
  res.end();
}).listen(port, () => console.log(`upload form on http://localhost:${port}${fixed ? ', mistakes fixed' : ''}`));
