package guard

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The bill of materials is the document somebody else's compliance process
// reads, and the one place where being quietly incomplete is worst: a list that
// omits what it cannot see still claims to be a list of everything.
//
// So it is generated from the registry rather than scanned out of a build, and
// these ask whether it says what this project ships.

// spdxDoc is the part of the document these guards read back.
type spdxDoc struct {
	SPDXVersion       string `json:"spdxVersion"`
	DataLicense       string `json:"dataLicense"`
	SPDXID            string `json:"SPDXID"`
	Name              string `json:"name"`
	DocumentNamespace string `json:"documentNamespace"`
	CreationInfo      struct {
		Created  string   `json:"created"`
		Creators []string `json:"creators"`
	} `json:"creationInfo"`
	Packages []struct {
		SPDXID           string `json:"SPDXID"`
		Name             string `json:"name"`
		VersionInfo      string `json:"versionInfo"`
		DownloadLocation string `json:"downloadLocation"`
		LicenseConcluded string `json:"licenseConcluded"`
		LicenseDeclared  string `json:"licenseDeclared"`
		CopyrightText    string `json:"copyrightText"`
	} `json:"packages"`
	Relationships []struct {
		SPDXElementID      string `json:"spdxElementId"`
		RelationshipType   string `json:"relationshipType"`
		RelatedSPDXElement string `json:"relatedSpdxElement"`
	} `json:"relationships"`
}

func TestTheSBOMNamesEverythingTheBinariesCarry(t *testing.T) {
	doc := renderedSBOM(t)

	licences := map[string]string{}
	for _, p := range doc.Packages {
		licences[p.Name] = p.LicenseDeclared
	}

	for _, m := range legal.Modules() {
		name := m.Path
		if name == "std" {
			name = "Go runtime and standard library"
		}
		if licences[name] == "" {
			t.Errorf("the SBOM never names %s, which this project ships", name)
			continue
		}
		if licences[name] != m.SPDX {
			t.Errorf("the SBOM says %s is %s and the registry says %s", name, licences[name], m.SPDX)
		}
	}
	for _, a := range legal.Assets() {
		if licences[a.Name] != a.SPDX {
			t.Errorf("the SBOM says %s is %q and the registry says %s.\n"+
				"A font that ships and is not in the document is the defect this whole "+
				"registry exists for.", a.Name, licences[a.Name], a.SPDX)
		}
	}
}

// The command line binary has no toolkit in it, so it has no fonts in it. A
// document saying otherwise would be describing a file nobody ships - and this
// is the assertion that catches a renderer attaching everything to everything.
func TestTheSBOMKeepsTheTwoBinariesApart(t *testing.T) {
	doc := renderedSBOM(t)

	name := map[string]string{}
	for _, p := range doc.Packages {
		name[p.SPDXID] = p.Name
	}
	fonts := map[string]bool{}
	for _, a := range legal.Assets() {
		fonts[a.Name] = true
	}

	described := 0
	for _, r := range doc.Relationships {
		if r.RelationshipType == "DESCRIBES" {
			described++
			continue
		}
		if name[r.SPDXElementID] != "tfg" {
			continue
		}
		if fonts[name[r.RelatedSPDXElement]] {
			t.Errorf("the SBOM says the command line binary carries %q, and it carries no toolkit at all",
				name[r.RelatedSPDXElement])
		}
	}
	if described != 2 {
		t.Errorf("the document describes %d packages and this project releases two binaries", described)
	}
}

// SPDX has fields a consumer is entitled to find. An empty one is worse than a
// missing document, because a reader takes it for an answer.
func TestTheSBOMCarriesTheFieldsSPDXRequires(t *testing.T) {
	doc := renderedSBOM(t)

	for field, value := range map[string]string{
		"spdxVersion":       doc.SPDXVersion,
		"dataLicense":       doc.DataLicense,
		"SPDXID":            doc.SPDXID,
		"name":              doc.Name,
		"documentNamespace": doc.DocumentNamespace,
		"created":           doc.CreationInfo.Created,
	} {
		if value == "" {
			t.Errorf("the document has no %s, which SPDX requires", field)
		}
	}
	if doc.SPDXVersion != "SPDX-2.3" {
		t.Errorf("the document says it is %s and these guards were written for SPDX-2.3", doc.SPDXVersion)
	}
	if len(doc.CreationInfo.Creators) == 0 {
		t.Error("the document names nobody as its creator")
	}
	for _, p := range doc.Packages {
		for field, value := range map[string]string{
			"SPDXID": p.SPDXID, "name": p.Name, "versionInfo": p.VersionInfo,
			"downloadLocation": p.DownloadLocation, "licenseConcluded": p.LicenseConcluded,
			"licenseDeclared": p.LicenseDeclared, "copyrightText": p.CopyrightText,
		} {
			if value == "" {
				t.Errorf("the package %q has no %s", p.Name, field)
			}
		}
	}
}

// The same inputs have to give the same bytes. A release attaches this file and
// a statement is made about it, so a document that differs every time it is
// generated is one nobody can check against anything.
func TestTheSBOMIsTheSameBytesForTheSameInputs(t *testing.T) {
	first, err := legal.SPDX(sbomInput(t))
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	second, err := legal.SPDX(sbomInput(t))
	if err != nil {
		t.Fatalf("rendering again: %v", err)
	}
	if string(first) != string(second) {
		t.Error("two renderings of the same inputs are not the same bytes")
	}
}

// sbomInput is the document as a release would ask for it, with the versions
// read from the build on every system - the Linux only module is why.
func sbomInput(t *testing.T) legal.Document {
	t.Helper()
	window, _ := shipped(t, "../../cmd/tfg-gui")
	command, _ := shipped(t, "../../cmd/tfg")
	if len(window) < 5 || len(command) < 2 {
		t.Skipf("the build reported %d and %d modules, too few to be the real sets", len(window), len(command))
	}
	return legal.Document{
		Version: version.Version,
		// Fixed rather than the clock, because a guard comparing a document
		// against itself has to render the same thing twice.
		Created: "2026-08-28T00:00:00Z",
		Seed:    "guard",
		Binaries: []legal.Binary{
			{Name: "tfg", Modules: command, GoVersion: runtime.Version()},
			{Name: "tfg-gui", Modules: window, GoVersion: runtime.Version()},
		},
	}
}

func renderedSBOM(t *testing.T) spdxDoc {
	t.Helper()
	raw, err := legal.SPDX(sbomInput(t))
	if err != nil {
		t.Fatalf("rendering the SBOM: %v", err)
	}
	var doc spdxDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the SBOM is not JSON a consumer can read: %v", err)
	}
	if len(doc.Packages) < 10 {
		t.Fatalf("the document holds %d packages, too few to be the real set", len(doc.Packages))
	}
	if !strings.HasPrefix(doc.DocumentNamespace, "https://") {
		t.Errorf("the namespace is %q, which is not a URI anybody can resolve", doc.DocumentNamespace)
	}
	return doc
}
