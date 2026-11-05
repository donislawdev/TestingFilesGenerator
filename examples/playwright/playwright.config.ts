import { defineConfig } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';

import { limitBytes, port } from './shared';

// Inside the repository the package is linked rather than installed, so its
// binary comes from the repository's own build. A copy of this example outside
// the repository installs testing-files-generator from npm and needs none of this.
const built = path.join(__dirname, '..', '..', process.platform === 'win32' ? 'tfg.exe' : 'tfg');
if (!process.env.TFG_BINARY && fs.existsSync(built)) {
  process.env.TFG_BINARY = built;
}

export default defineConfig({
  testDir: 'tests',
  fullyParallel: true,
  reporter: [['list'], ['json', { outputFile: 'results/report.json' }], ['html', { open: 'never', outputFolder: 'results/html' }]],
  use: { baseURL: `http://localhost:${port}` },
  webServer: {
    command: 'node app/server.mjs',
    url: `http://localhost:${port}/`,
    reuseExistingServer: false,
    env: { UPLOAD_LIMIT_BYTES: String(limitBytes), PORT: String(port), FIXED: process.env.FIXED ?? '' },
  },
});
