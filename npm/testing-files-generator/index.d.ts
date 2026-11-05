// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/** What a system under test is expected to do with a file. */
export type Outcome = 'accept' | 'reject' | 'sanitize' | 'unspecified';

/** The rule a file is about. It names the rule and not what the system did, so a file a byte under a limit is expected to be accepted with size_limit. */
export type Reason =
  | 'content_malformed'
  | 'count_limit'
  | 'dimensions_limit'
  | 'duplicate'
  | 'encoding_invalid'
  | 'extension_rule'
  | 'filename_invalid'
  | 'filename_too_long'
  | 'filename_traversal'
  | 'malware_signature'
  | 'mime_mismatch'
  | 'nesting_depth'
  | 'none'
  | 'size_limit'
  | 'size_zero';

export interface Expected {
  outcome: Outcome;
  reason?: Reason;
  detail?: string;
  confidence?: string;
}

/** One entry of a manifest, as tfg writes it. */
export interface ManifestFile {
  id: string;
  path: string;
  name: string;
  materialized: boolean;
  target_id: string;
  bytes: number;
  format: string;
  group?: string;
  purpose?: string;
  expected: Expected;
  [field: string]: unknown;
}

/** A manifest, as tfg writes it. For a plan nothing is written yet and every file has materialized false. */
export interface Manifest {
  manifest_version: string;
  tool: { name: string; version: string; generators: Record<string, string>; [field: string]: unknown };
  run: { id: string; seed: number; complete: boolean; recipe_hash?: string; [field: string]: unknown };
  summary: { file_count: number; total_bytes: number; materialized: number; [field: string]: unknown };
  files: ManifestFile[];
  [field: string]: unknown;
}

export type OptionValue = string | number | boolean | ReadonlyArray<string | number>;

export interface FileSetSpec {
  /** A preset, as tfg preset list names it. */
  preset?: string;
  /** A recipe file. A relative path is taken from the directory the tests were started in. */
  recipe?: string;
  /** Flags of tfg generate and of the preset, without the dashes: { limit: '2mb', seed: 7, clean: true }. */
  options?: Record<string, OptionValue>;
  /** Where sets are written. TFG_CACHE_DIR, or node_modules/.cache/testing-files-generator, or the system's temporary directory. */
  cacheDir?: string;
  /** The tfg binary to run. TFG_BINARY, or the one this package installed. */
  binary?: string;
}

export interface Filter {
  group?: string | readonly string[];
  target?: string | readonly string[];
  outcome?: Outcome | readonly Outcome[];
}

export type Observed =
  | Exclude<Outcome, 'unspecified'>
  | { outcome: Exclude<Outcome, 'unspecified'>; reason?: string };

/** The shape test(title, details, body) takes in Playwright. */
export interface TestDetails {
  tag: string[];
  annotation: Array<{ type: string; description: string }>;
}

export interface TestFile {
  readonly entry: ManifestFile;
  readonly relativePath: string;
  readonly name: string;
  readonly format: string;
  readonly bytes: number;
  readonly group: string;
  readonly target: string;
  readonly purpose: string;
  readonly expected: Expected;
  readonly unspecified: boolean;
  readonly whyUnspecified: string;
  /** The file's path and what it is expected to meet. Unique within a set. */
  readonly title: string;
  readonly details: TestDetails;
  /** Writes the set the first time any of its files is asked for, and gives this file's absolute path. */
  path(): Promise<string>;
  /** Throws when what the system did differs from what is expected, with the sentence that says what the difference means. */
  check(observed: Observed): void;
}

export interface FileSet {
  readonly files: readonly TestFile[];
  readonly plan: Manifest;
  only(filter: Filter): FileSet;
  except(filter: Filter): FileSet;
  /** Writes the whole set once per process and gives its directory. */
  write(): Promise<string>;
  paths(): Promise<string[]>;
}

/** Plans a set of files, writing nothing. The files are written the first time one of them is asked for. */
export function fileSet(spec: FileSetSpec): FileSet;

/** The tfg binary this package runs. */
export function binaryPath(): string;

export class TfgError extends Error {
  readonly status: number | string;
  readonly stderr: string;
}
