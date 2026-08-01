module github.com/donislawdev/TestingFilesGenerator

// The compiler takes part in producing bytes, so its version is part of the
// byte stability contract (D11). This line is a minimum - the exact version
// used for tests and releases is pinned in the CI workflow, and the guard
// test in internal/guard reports any drift it causes. See docs/STACK.md.
go 1.26.5
