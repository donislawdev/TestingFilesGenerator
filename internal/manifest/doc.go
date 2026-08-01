// Package manifest defines the manifest schema and writes entries while a run
// is still going.
//
// Writing at the end would lose every entry at the moment the disk fills up,
// which is the most common failure of this particular tool.
package manifest
