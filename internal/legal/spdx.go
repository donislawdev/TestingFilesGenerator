// The bill of materials, rendered from the registry rather than scanned out of
// a build.
//
// Both were measured on 2026-08-27 before this was written, and neither half is
// enough alone. A scanner reading the window binary names all thirty modules
// with exact versions - Go records them in the executable - and attaches a
// licence to exactly one of them. The registry carries the licences, the SPDX
// identifiers and the copyright lines, which are the half no scanner can infer,
// and it carries the seven fonts and ninety-seven drawings that arrive inside a
// module and are invisible to any question asked about modules.
//
// So the document is generated from the registry, and the scanner's job is to
// GUARD it: a scan that names something the registry does not account for is a
// hole in the registry. That direction is the one that has already paid - it is
// how the fonts were found.
//
// A document that omits what it cannot see is worse than no document at all,
// because it claims to be complete. This one is as complete as the registry,
// and the registry is held to the build by guards in both directions.

package legal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Where this project lives. Written here because an SBOM without a download
// location is a document nobody can act on.
const repository = "https://github.com/donislawdev/TestingFilesGenerator"

// ourLicence is what this program is under.
//
// GPL-3.0-only rather than GPL-3.0-or-later, and that is read from what the
// project says about itself rather than assumed: the licence notice the program
// prints says "the GNU General Public License, version 3", the readme badge
// says GPL-3.0, and no file anywhere offers a later version. The two spellings
// are different licences to a machine, so guessing the generous one would be
// publishing a permission nobody granted.
const ourLicence = "GPL-3.0-only"

// A Binary is one artefact the document describes, and what its build reported.
type Binary struct {
	// Name is what the artefact is called - the command, not the file name of
	// one platform's archive.
	Name string

	// Modules is what this binary links: module path to the version the build
	// recorded. The runtime is not in here, because it is not a module - it is
	// added from GoVersion.
	Modules map[string]string

	// GoVersion is the toolchain the binary was built with, which is also the
	// version of the runtime and standard library inside it.
	GoVersion string
}

// A Document is everything the renderer needs that is not in the registry.
//
// Created and Seed are inputs rather than readings of the clock, and that is
// deliberate: a generated document that differs every time it is generated
// cannot be compared with anything, and the two fields SPDX requires are
// exactly the two a clock would move.
type Document struct {
	Version  string // the tool version, without a leading v
	Created  string // RFC3339, in UTC
	Seed     string // what makes the namespace unique - a tag is the useful one
	Binaries []Binary
}

// SPDX renders the document as SPDX 2.3 JSON.
func SPDX(d Document) ([]byte, error) {
	if len(d.Binaries) == 0 {
		return nil, fmt.Errorf("an SBOM describing no binary describes nothing")
	}
	if d.Created == "" || d.Version == "" {
		return nil, fmt.Errorf("an SBOM needs a version and a creation time, and they are the caller's to supply")
	}

	doc := spdxDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "TestingFilesGenerator-" + d.Version,
		DocumentNamespace: namespace(d),
		CreationInfo: spdxCreation{
			Created: d.Created,
			Creators: []string{
				"Tool: tfg-sbom",
				"Organization: DonislawDev",
			},
			Comment: "Generated from internal/legal, the reviewed list of what this " +
				"project ships, with versions read from the build. See internal/legal/spdx.go " +
				"for why that is the source rather than a scan of the artefact.",
		},
	}
	for _, binary := range d.Binaries {
		doc.add(binary, d.Version)
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// namespace is unique per document and stable for the same inputs, so two runs
// on one tag produce the same URI instead of two nobody can correlate.
func namespace(d Document) string {
	seed := d.Seed
	if seed == "" {
		seed = d.Created
	}
	sum := sha256.Sum256([]byte(d.Version + "|" + seed))
	return fmt.Sprintf("%s/spdx/%s-%x", repository, d.Version, sum[:8])
}

// add puts one binary in the document, with everything it carries beneath it.
func (doc *spdxDocument) add(binary Binary, version string) {
	root := identifier("Package", binary.Name)
	doc.Packages = append(doc.Packages, spdxPackage{
		SPDXID:           root,
		Name:             binary.Name,
		VersionInfo:      version,
		DownloadLocation: repository,
		FilesAnalyzed:    false,
		LicenseConcluded: ourLicence,
		LicenseDeclared:  ourLicence,
		CopyrightText:    "Copyright (C) 2026 DonislawDev",
	})
	doc.Relationships = append(doc.Relationships, spdxRelationship{
		SPDXElementID:      "SPDXRef-DOCUMENT",
		RelationshipType:   "DESCRIBES",
		RelatedSPDXElement: root,
	})
	for _, item := range carriedBy(binary) {
		doc.contain(root, item)
	}
}

// contain adds one component and the relationship saying which binary has it.
//
// The same component in two binaries is one package with two relationships,
// which is what SPDX means by containment - repeating the package would say
// the two binaries carry different copies of it.
func (doc *spdxDocument) contain(root string, item Item) {
	id := identifier("Package", item.Name)
	if !doc.has(id) {
		doc.Packages = append(doc.Packages, spdxPackage{
			SPDXID:           id,
			Name:             item.Name,
			VersionInfo:      orNoAssertion(item.Version),
			DownloadLocation: "NOASSERTION",
			FilesAnalyzed:    false,
			LicenseConcluded: item.SPDX,
			LicenseDeclared:  item.SPDX,
			CopyrightText:    orNoAssertion(item.Copyright),
			ExternalRefs:     externalRefs(item),
		})
	}
	doc.Relationships = append(doc.Relationships, spdxRelationship{
		SPDXElementID:      root,
		RelationshipType:   "CONTAINS",
		RelatedSPDXElement: id,
	})
}

func (doc *spdxDocument) has(id string) bool {
	for _, p := range doc.Packages {
		if p.SPDXID == id {
			return true
		}
	}
	return false
}

// carriedBy is what one binary contains, in the order the list is printed.
func carriedBy(binary Binary) []Item {
	reviewed := map[string]Module{}
	for _, m := range modules {
		reviewed[m.Path] = m
	}
	items := []Item{moduleItem(runtimeName, binary.GoVersion, reviewed)}
	paths := make([]string, 0, len(binary.Modules))
	for path := range binary.Modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, known := reviewed[path]; !known {
			continue
		}
		items = append(items, moduleItem(path, binary.Modules[path], reviewed))
	}
	return append(items, embeddedItems(linkedSet(binary.Modules))...)
}

func linkedSet(versions map[string]string) map[string]bool {
	linked := map[string]bool{}
	for path := range versions {
		linked[path] = true
	}
	return linked
}

// externalRefs carries the package URL, which is how a machine looks a module
// up. Only modules have one - a font inside a module is not a package anybody
// can fetch.
func externalRefs(item Item) []spdxExternalRef {
	if item.Embedded || item.Version == "" || !strings.Contains(item.Name, "/") {
		return nil
	}
	return []spdxExternalRef{{
		ReferenceCategory: "PACKAGE-MANAGER",
		ReferenceType:     "purl",
		ReferenceLocator:  "pkg:golang/" + item.Name + "@" + item.Version,
	}}
}

func orNoAssertion(value string) string {
	if value == "" {
		return "NOASSERTION"
	}
	return value
}

// notIdentifier is everything the SPDX identifier grammar does not allow. It
// permits letters, digits, dots and hyphens, and a module path is full of
// slashes.
var notIdentifier = regexp.MustCompile(`[^A-Za-z0-9.-]`)

// identifier builds an SPDXRef that follows that grammar.
func identifier(kind, name string) string {
	return "SPDXRef-" + kind + "-" + strings.Trim(notIdentifier.ReplaceAllString(name, "-"), "-")
}
