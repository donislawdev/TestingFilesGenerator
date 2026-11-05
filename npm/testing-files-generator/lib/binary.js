// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

const fs = require('node:fs');

// The systems a release builds a command line for, as Node names them, and the
// package that carries each binary. A guard holds this to the list of targets
// in .github/workflows/release.yml, because a platform here with nothing behind
// it installs a package that cannot run, and a platform there missing here is a
// binary nobody can reach. Intel Macs are not on either list, by the owner's
// decision of 2026-08-20.
const platforms = {
  'win32 x64': '@testing-files-generator/win32-x64',
  'win32 arm64': '@testing-files-generator/win32-arm64',
  'linux x64': '@testing-files-generator/linux-x64',
  'linux arm64': '@testing-files-generator/linux-arm64',
  'darwin arm64': '@testing-files-generator/darwin-arm64',
};

// binaryPath finds the tfg binary this package runs.
//
// TFG_BINARY comes first and is taken as given - a tfg installed another way,
// or one built from the repository. PATH is never searched: a different tfg
// found there would answer with its own version and say nothing about it.
// Everything is passed in so the tests can ask about a system they are not on.
function binaryPath({ env = process.env, platform = process.platform, arch = process.arch, resolve = require.resolve } = {}) {
  const given = env.TFG_BINARY;
  if (given) {
    if (!fs.existsSync(given)) {
      throw new Error(`TFG_BINARY is set to ${given} and there is no file there. ` +
        'Point it at a tfg binary, or unset it to use the one this package installs.');
    }
    return given;
  }
  const pkg = platforms[`${platform} ${arch}`];
  const known = Object.keys(platforms).join(', ');
  if (!pkg) {
    throw new Error(`testing-files-generator has no binary for ${platform} ${arch}. ` +
      `It comes for ${known}. ` +
      'On another system, run tfg from its container image ghcr.io/donislawdev/tfg, ' +
      'or install it another way and set TFG_BINARY to its path.');
  }
  const exe = platform === 'win32' ? 'tfg.exe' : 'tfg';
  try {
    return resolve(`${pkg}/${exe}`);
  } catch (err) {
    // Only a package that is not there is a matter of --omit. Anything else,
    // such as a package that does not export the binary, is told as it is,
    // because installing again would not change it.
    if (err?.code !== 'MODULE_NOT_FOUND') {
      throw new Error(`The binary for ${platform} ${arch} in the package ${pkg} could not be found - ${err?.message}. ` +
        'Set TFG_BINARY to the path of a tfg binary to go on.', { cause: err });
    }
    throw new Error(`The binary for ${platform} ${arch} comes in the package ${pkg}, and it is not installed. ` +
      'npm leaves it out when it installs with --omit=optional or --no-optional. ' +
      'Install again without that option, or set TFG_BINARY to the path of a tfg binary.');
  }
}

module.exports = { binaryPath, platforms };
