// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

'use strict';

// Each export is assigned on its own line, the form Node reads the names of
// when an ES module imports this file, so `import { fileSet }` works as well as
// require does.
exports.fileSet = require('./lib/fileset.js').fileSet;
exports.binaryPath = require('./lib/binary.js').binaryPath;
exports.TfgError = require('./lib/run.js').TfgError;
