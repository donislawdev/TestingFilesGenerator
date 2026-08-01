// Package format defines the generator interface and the registry each format
// announces itself in.
//
// A format declares where its padding channel sits and how much it holds. How
// many bytes are missing is worked out by core, not here.
package format
