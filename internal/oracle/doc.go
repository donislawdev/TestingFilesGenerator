// Package oracle wraps the external reference tools that tests compare our
// output against.
//
// Production code never imports it. A missing tool skips a test loudly - a
// quietly skipped oracle is a green run that checked nothing.
//
// The point of these is that our own tests are written by whoever wrote the
// generator, so they cannot be the only judge of whether a file is correct.
// An independent implementation can.
package oracle
