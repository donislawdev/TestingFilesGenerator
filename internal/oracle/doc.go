// Package oracle wraps the external reference tools that tests compare our
// output against.
//
// Production code never imports it. A missing tool skips a test loudly - a
// quietly skipped oracle is a green run that checked nothing.
package oracle
