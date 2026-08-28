// Package legal is the reviewed list of what a built binary of this project
// carries that somebody else wrote: the modules it links, and the files those
// modules compile into it.
//
// Why a written list at all, when a scanner can read the binary. Measured on
// 2026-08-27 with syft 1.51.0: a scan of the window binary names all thirty
// artefacts with exact versions and attaches a licence to one of them. The
// inventory half is free, because Go records it in the binary itself. The
// licence half is the half no scanner can infer, and this list is that half.
//
// It also carries what no module list can carry: the files a module embeds.
// Seven fonts and ninety-seven icons reach the window binary from inside
// fyne.io/fyne/v2, under licences of their own, and a question asked about
// modules cannot see them. Nothing here noticed for as long as the window
// existed - see docs/OBSERVATIONS.md, O150.
//
// Nothing in this package is a version. A version is a fact about a build, so
// it is read from the build: by go list when a document is generated, and by
// debug.ReadBuildInfo when a binary is asked about itself. A written copy would
// be a third thing to keep true, and the one already written here drifted -
// see O151.
package legal

import "strings"

// A Module is one module that a binary of this project links.
type Module struct {
	// Path is the module path as go list reports it. The Go runtime and
	// standard library are carried as "std", which is not a module and which
	// every binary here contains.
	Path string

	// SPDX is the licence identifier, taken from the published SPDX list and
	// read out of the module's own licence file rather than from a package
	// index. Checked against the list on 2026-08-28.
	SPDX string

	// Copyright is the line that licence file carries, quoted rather than
	// summarised. Empty only when the file carries none, and then Note says so
	// - a blank that reads like an oversight is worse than a sentence.
	Copyright string

	// Note is the sentence an entry needs when the fields above do not speak
	// for themselves: what the module does here, which system it appears on,
	// or why a field is empty.
	Note string
}

// An Asset is bytes that are not code: a file that some package compiles into
// a binary of this project through a go:embed directive.
//
// These are the entries a module list cannot hold. A font is not a dependency,
// it is a file inside one, and its licence is not the module's licence.
type Asset struct {
	// Name is what a person calls it, not what the file is called.
	Name string

	// Package is the import path that embeds the files, as go list reports it.
	Package string

	// Files are the embedded paths this entry accounts for, spelled as go list
	// spells them. A path ending in a slash accounts for everything embedded
	// beneath it, which is how ninety-seven icons take one entry rather than
	// ninety-seven.
	Files []string

	// SPDX, Copyright and Note carry the same meaning as on Module. Note is
	// where a disagreement goes: one font here says one thing in the module
	// and another in its own metadata, and both belong in the notices.
	SPDX      string
	Copyright string
	Note      string
}

// Modules returns the reviewed list of modules.
//
// The slice is shared rather than copied, and callers read it. That is the
// same contract the format registry states about its own tables: this list
// exists to be rendered and compared, and every caller in this tree does one
// of those two things.
func Modules() []Module { return modules }

// Assets returns the reviewed list of embedded files.
func Assets() []Asset { return assets }

// Covers reports whether this entry accounts for an embedded path, spelled the
// way go list spells it.
func (a Asset) Covers(embedded string) bool {
	for _, file := range a.Files {
		if covers(file, embedded) {
			return true
		}
	}
	return false
}

// covers answers for one declared path. A trailing slash means a directory and
// everything embedded beneath it.
func covers(declared, embedded string) bool {
	if strings.HasSuffix(declared, "/") {
		return strings.HasPrefix(embedded, declared)
	}
	return embedded == declared
}
