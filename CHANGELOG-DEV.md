# Technical changelog

Changes a user of the tool never sees: layer boundaries, guard tests, CI
configuration, architectural decisions. Anything a user would notice goes to
`CHANGELOG.md` instead.

## Unreleased

### 2026-08-01 - repository skeleton

- Directory layout and layer boundaries. Packages are empty and carry only
  their package comment. No feature is implemented.
- **Four guards armed before the first line of feature code**, each one
  verified by mutation - broken deliberately and watched turn red:
  - **layering** - nothing imports upwards, and the command line binary can
    never import the window, which would drag in CGO and end cross
    compilation
  - **network isolation** - no `net` or `os/exec` imports in the lower layers
  - **English only** - no non ASCII characters in the command line packages
  - **byte stability** - six standard library paths that produce our bytes,
    pinned against measured values, plus a second test that the same path
    called twice in one process returns the same bytes
- Two guards are partial and say so in place. The ASCII scan does not catch
  another language written without accents - measured, not assumed. The
  import check does not prove that no traffic leaves the machine, and it
  excludes the window because the graphics toolkit brings a dependency tree
  we do not control.
- `LICENSE` holds the canonical GPL-3.0 text, SHA-256 `8ceb4b9e...`, copied
  from a local distribution rather than reproduced from memory. Source and
  copy were compared by hash.
- `.gitignore` was checked by measurement against 21 paths. It caught a bug
  in itself - the `testdata` negation was anchored to the repository root,
  which would have dropped the per package `testdata` directories that Go
  conventionally uses. `internal/engine/testdata/manifest.json` would have
  left the repository silently.
- `.gitattributes` normalises line endings to LF on every system and excludes
  `testdata` and `LICENSE` from any conversion. Without it, git on Windows
  would rewrite files that are compared byte for byte, and the failure would
  look like the compressor drifting rather than like git touching a file.
- Byte stability values were **measured fresh** on go1.26.5 instead of being
  reused from an earlier probe, because the input of that probe was never
  recorded and cannot be reconstructed. The test now defines its own input in
  its own source, so it depends on nothing outside the repository.
- Coverage gate starts at a threshold of zero, held in exactly one file. It
  rises with coverage and is never lowered to turn a red run green.
