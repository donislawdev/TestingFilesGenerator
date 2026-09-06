//go:build !noasm

// Part of package avif. See avif.go.
package avif

// A build without the tag does not compile, on purpose.
//
// The package comment on avif.go has said since 2026-08-29 that the encoder's
// AVX2 path reads past the end of a buffer and takes the process down on some
// picture sizes - a 640x256 picture does it in two runs out of three. The tag
// that turns that path off lives in .github/build-tags, every workflow step
// passes it, and two guards keep those steps honest.
//
// None of that reaches somebody who builds this themselves. README.md offered
// three ways to do it and not one of them carried the tag, so "go install
// ...@latest", a distribution packager and a contributor running the program
// from a checkout all got the path this project describes as reading outside
// its buffer. Both binaries reach it: internal/format/all registers AVIF
// unconditionally, and go list -deps says cmd/tfg links it.
//
// Failing here is the honest place to fail. The alternative shapes were
// weighed on 2026-09-06 and turned down: registering AVIF as unavailable
// without the tag keeps go install working but makes "tfg formats" answer
// differently depending on how the binary was made, and inverting the tag so
// the assembly is opt-in is a larger change to CI and to the byte contract for
// no more safety than this. Owner's call.
//
// The message is written as a value rather than as a comment because the
// compiler prints it. Nothing here is reached at run time, and with the tag
// this file does not exist at all.
type buildWithoutTheNoasmTag struct{}

var _ buildWithoutTheNoasmTag = "this build is missing its build tags, and without them the AVIF encoder reads past the end of a buffer and takes the process down. Build with -tags \"$(cat .github/build-tags)\", or see the install section of README.md"
