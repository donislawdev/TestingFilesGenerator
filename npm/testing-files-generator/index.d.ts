// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

/**
 * What a system under test is expected to do with a file. accept is to take it, reject to turn it away,
 * sanitize to take it and change it, and unspecified means there is no right answer until you decide one.
 */
export type Outcome = 'accept' | 'reject' | 'sanitize' | 'unspecified';

/** The rule a file is about, from a closed list. It names the rule and not what a system did with the file. */
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

/** What a file expects of the system under test, as the manifest has it. */
export interface Expected {
  /** What the system should do with the file. */
  outcome: Outcome;
  /** The rule the file is about. A file to be turned away usually names one. */
  reason?: Reason;
  /** A sentence about the expectation, such as why a file expects nothing. */
  detail?: string;
  /** certain when the expectation was declared, policy_dependent when nobody declared one. */
  confidence?: string;
}

/** One entry of a manifest, as tfg writes it. It carries more fields than are named here. */
export interface ManifestFile {
  /** The file's identity within the run. */
  id: string;
  /** The file's path within the set, with forward slashes. */
  path: string;
  /** The file's name. */
  name: string;
  /** Whether the file is on the disk. Every file of a plan has false. */
  materialized: boolean;
  /** The recipe target that made the file. */
  target_id: string;
  /** The file's exact size in bytes. */
  bytes: number;
  /** The file's format, such as jpg. */
  format: string;
  /** The part of the set the file belongs to, such as size-limit. */
  group?: string;
  /** One sentence on what the file is for and what it means when a system gets it wrong. */
  purpose?: string;
  /** What the system under test should do with the file. */
  expected: Expected;
  [field: string]: unknown;
}

/** A manifest, as tfg writes it. For a plan nothing is written yet and every file has materialized false. */
export interface Manifest {
  /** The version of the manifest's format. This package reads major version 1. */
  manifest_version: string;
  /** The program that wrote the manifest and its version. */
  tool: { name: string; version: string; generators: Record<string, string>; [field: string]: unknown };
  /** The run that made the set, and the seed every file's bytes come from. */
  run: { id: string; seed: number; complete: boolean; recipe_hash?: string; [field: string]: unknown };
  /** How many files the set has and how many of them were written. */
  summary: { file_count: number; total_bytes: number; materialized: number; [field: string]: unknown };
  /** Every file of the set, in order. */
  files: ManifestFile[];
  [field: string]: unknown;
}

/** The value of one option. true stands for a flag with no value, and an array repeats the flag. */
export type OptionValue = string | number | boolean | ReadonlyArray<string | number>;

/** What a set is made of, and where it is kept. Name a preset, a recipe or the options of a set. */
export interface FileSetSpec {
  /** A preset, as npx tfg preset list names them, such as upload-validation. */
  preset?: string;
  /**
   * A recipe file. A relative path is taken from the directory the tests were started in,
   * so path.join(__dirname, 'uploads.yaml') names one beside the spec file. In an ES module,
   * import.meta.dirname takes the place of __dirname.
   */
  recipe?: string;
  /**
   * The parameters of the preset and the flags of tfg generate, without the dashes, such as
   * { limit: '2mb', bulk: 0, seed: 7 }. npx tfg preset show lists what a preset takes.
   */
  options?: Record<string, OptionValue>;
  /** Where sets are kept. TFG_CACHE_DIR, or node_modules/.cache/testing-files-generator, or a directory of your own in the system's temporary directory. */
  cacheDir?: string;
  /** The tfg binary to run. TFG_BINARY, or the one this package installed. */
  binary?: string;
}

/** Which files to take from a set. Each field takes one name or a list, and a file has to match every field given. */
export interface Filter {
  /** The group, such as bulk-upload. */
  group?: string | readonly string[];
  /** The recipe target. */
  target?: string | readonly string[];
  /** The expected outcome. */
  outcome?: Outcome | readonly Outcome[];
}

/** What the system under test did with a file: one word, or the outcome with the reason the system gave. */
export type Observed =
  | Exclude<Outcome, 'unspecified'>
  | { outcome: Exclude<Outcome, 'unspecified'>; reason?: string };

/** The shape test(title, details, body) takes in Playwright. */
export interface TestDetails {
  /** The expected outcome and the group, such as @reject and @size-limit, for --grep. */
  tag: string[];
  /** What the file expects and its purpose, which the report shows beside the test. */
  annotation: Array<{ type: string; description: string }>;
}

/** One file of a set, with what the system under test should do with it. */
export interface TestFile {
  /** The file's whole entry in the manifest. */
  readonly entry: ManifestFile;
  /** The file's path within the set. */
  readonly relativePath: string;
  /** The file's name. */
  readonly name: string;
  /** The file's format, such as jpg. */
  readonly format: string;
  /** The file's exact size in bytes. */
  readonly bytes: number;
  /** The part of the set the file belongs to, or an empty string. */
  readonly group: string;
  /** The recipe target that made the file. */
  readonly target: string;
  /** One sentence on what the file is for and what it means when a system gets it wrong. A failure shows it. */
  readonly purpose: string;
  /** What the system under test should do with the file. */
  readonly expected: Expected;
  /** True for a file with no right answer. Skip it with test.skip(file.unspecified, file.whyUnspecified). */
  readonly unspecified: boolean;
  /** The sentence a skip shows for a file with no right answer. */
  readonly whyUnspecified: string;
  /** The file's path and what it expects, such as 2mb_over_1b.jpg - expected reject (size_limit). Unique within a set. */
  readonly title: string;
  /** Tags and annotations, as the second argument of test(title, details, body). */
  readonly details: TestDetails;
  /** Writes the set the first time any of its files is asked for, and gives this file's absolute path. */
  path(): Promise<string>;
  /**
   * Compares what the system did with what the file expects, and throws when they differ, with the sentence
   * that says what the difference means. A reason is compared only for a file to be turned away, and only when
   * one is given. On a file with no right answer it always throws.
   */
  check(observed: Observed): void;
}

/** A set of files, or a part of one. Every part shares the set's one write. */
export interface FileSet {
  /** Every file of this set or part, in order. */
  readonly files: readonly TestFile[];
  /** The manifest tfg gave for the plan of the whole set. Nothing in it is written yet. */
  readonly plan: Manifest;
  /** The files that match the filter. */
  only(filter: Filter): FileSet;
  /** The files that do not match the filter. */
  except(filter: Filter): FileSet;
  /** Writes the whole set once per process, or takes a set kept from an earlier run, and gives its directory. */
  write(): Promise<string>;
  /** Writes the set if it is not written yet, and gives the absolute paths of this part's files. */
  paths(): Promise<string[]>;
}

/**
 * Plans a set of files, writing nothing, so tests can be declared from it while the spec file loads.
 * The files are written the first time one of them is asked for.
 */
export function fileSet(spec: FileSetSpec): FileSet;

/** The tfg binary this package runs. */
export function binaryPath(): string;

/** tfg ending with a code other than nought, or stopped by a signal. */
export class TfgError extends Error {
  /** The code tfg ended with, or the name of the signal that stopped it. */
  readonly status: number | string;
  /** The end of what tfg wrote to its error stream. */
  readonly stderr: string;
}
