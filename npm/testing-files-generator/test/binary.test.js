'use strict';

const assert = require('node:assert/strict');
const path = require('node:path');
const { test } = require('node:test');

const { binaryPath, platforms } = require('../lib/binary.js');

// What require.resolve throws for a package that is not installed.
const nothing = (id) => { throw Object.assign(new Error(`Cannot find module '${id}'`), { code: 'MODULE_NOT_FOUND' }); };

test('TFG_BINARY is taken as given when there is a file there', () => {
  assert.equal(binaryPath({ env: { TFG_BINARY: __filename }, resolve: nothing }), __filename);
});

test('TFG_BINARY naming nothing is refused with the path it named', () => {
  const missing = path.join(__dirname, 'no-such-tfg');
  assert.throws(() => binaryPath({ env: { TFG_BINARY: missing }, resolve: nothing }),
    (err) => err.message.includes(missing) && err.message.includes('unset it'));
});

test('each platform a release builds resolves to its own package', () => {
  for (const [key, pkg] of Object.entries(platforms)) {
    const [platform, arch] = key.split(' ');
    // Written out rather than asked of binaryIn, so a wrong place there shows.
    const exe = { win32: 'tfg.exe', darwin: 'tfg.app/Contents/MacOS/tfg', linux: 'tfg' }[platform];
    const asked = [];
    const found = binaryPath({ env: {}, platform, arch, resolve: (id) => { asked.push(id); return `/resolved/${id}`; } });
    assert.deepEqual(asked, [`${pkg}/${exe}`]);
    assert.equal(found, `/resolved/${pkg}/${exe}`);
  }
});

test('an Intel Mac is told what it can do instead', () => {
  assert.throws(() => binaryPath({ env: {}, platform: 'darwin', arch: 'x64', resolve: nothing }),
    (err) => err.message.includes('no binary for darwin x64') && err.message.includes('TFG_BINARY') &&
      err.message.includes('in a Linux container') && err.message.includes('darwin arm64'));
});

test('a platform package left out by --omit=optional is named, with the way back', () => {
  assert.throws(() => binaryPath({ env: {}, platform: 'linux', arch: 'x64', resolve: nothing }),
    (err) => err.message.includes('@testing-files-generator/linux-x64') && err.message.includes('--omit=optional'));
});

test('a platform package that is there and still gives no binary is told as it is, not as left out', () => {
  const hidden = () => {
    throw Object.assign(new Error('Package subpath \'./tfg\' is not defined by "exports"'), { code: 'ERR_PACKAGE_PATH_NOT_EXPORTED' });
  };
  assert.throws(() => binaryPath({ env: {}, platform: 'linux', arch: 'x64', resolve: hidden }),
    (err) => err.message.includes('is not defined by "exports"') && !err.message.includes('--omit=optional') &&
      err.cause?.code === 'ERR_PACKAGE_PATH_NOT_EXPORTED');
});
