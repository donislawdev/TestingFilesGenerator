package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// A companion is somebody else's program shipped beside a binary of ours
// rather than inside it - the software renderer in the Windows archive,
// since 2026-09-17. The registry's first two classes are held to the build:
// a module by what the build reports, a font by what the compiler embeds.
// Nothing in a build knows what an archive was packed with, so this class
// is held to its consumers instead - the notices, the two licence lists and
// the bill of materials - and to the code that loads it.

// The registry names the files the program loads, in the order it loads
// them, and nothing else about the entry is left blank.
//
// The order is the one measured on 2026-09-17: the renderer first, because
// the loader imports it by name and finds it only when it is already mapped.
// gui.SoftwareFiles is where the program takes the order from, so the two
// are compared rather than both written out.
func TestTheRegistryNamesTheFilesTheWindowLoads(t *testing.T) {
	companions := legal.Companions()
	if len(companions) != 1 {
		t.Fatalf("the registry holds %d companion(s) and this guard knows the shape of one", len(companions))
	}
	c := companions[0]
	if c.Binary != "tfg-gui" || c.Platform != "windows/amd64" {
		t.Errorf("the companion accompanies %q on %q, and the renderer ships beside the window on windows/amd64 only", c.Binary, c.Platform)
	}
	var loaded []string
	for _, path := range gui.SoftwareFiles("") {
		loaded = append(loaded, filepath.ToSlash(path))
	}
	var listed []string
	for _, f := range c.Files {
		listed = append(listed, f.Path)
	}
	if strings.Join(loaded, "|") != strings.Join(listed, "|") {
		t.Errorf("the registry lists %q and the window loads %q.\n"+
			"They have to be the same files in the same order, or the archive is packed with\n"+
			"something other than what the program looks for.", listed, loaded)
	}
	hex64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, f := range c.Files {
		if f.Size <= 0 || !hex64.MatchString(f.SHA256) {
			t.Errorf("%s has size %d and sum %q - both are read from the pinned archive and neither can be blank", f.Path, f.Size, f.SHA256)
		}
	}
	s := c.Source
	if s.Project == "" || s.Version == "" || s.Archive == "" || !hex64.MatchString(s.SHA256) || s.Inside == "" {
		t.Errorf("the source is not fully named: %+v", s)
	}
	if !strings.Contains(s.URL, s.Project) || !strings.HasSuffix(s.URL, s.Archive) || !strings.Contains(s.URL, s.Version) {
		t.Errorf("the download address %q does not name the project, the version and the archive it says it is", s.URL)
	}
	if c.SPDX == "" || c.Copyright == "" || c.Note == "" {
		t.Error("a companion needs a licence expression, a copyright line and a note - a blank reads like an oversight")
	}
}

// The window's list names the companion and the command line's never does.
//
// The window is what a person may be looking at while the renderer draws
// it, so its About screen names what ships beside it. The command line
// binary has nothing beside it, and its list is read from its own build,
// which cannot see an archive - so the guard asks a build report holding
// every module and expects no companion in the answer.
func TestTheWindowListsTheCompanionAndTheCommandLineDoesNot(t *testing.T) {
	beside := 0
	for _, item := range legal.Reviewed() {
		if item.Beside {
			beside++
			if item.Version == "" {
				t.Errorf("%s is listed with no version, and a companion's version is a fact of the registry", item.Name)
			}
		}
	}
	if beside != len(legal.Companions()) {
		t.Errorf("the window's list carries %d companion(s) and the registry %d", beside, len(legal.Companions()))
	}

	info := &debug.BuildInfo{GoVersion: "go1.27.0"}
	for _, m := range legal.Modules() {
		info.Deps = append(info.Deps, &debug.Module{Path: m.Path, Version: "v0.0.0"})
	}
	for _, item := range legal.Carried(info) {
		if item.Beside {
			t.Errorf("the command's list names %q, which ships beside no command line binary", item.Name)
		}
		for _, c := range legal.Companions() {
			if item.Name == c.Name {
				t.Errorf("the command's list names %q under another flag", c.Name)
			}
		}
	}
}

// The bill of materials ships the companion beside the window and beside
// nothing else, with the archive it comes from and that archive's sum.
func TestTheSBOMShipsTheCompanionBesideTheWindowOnly(t *testing.T) {
	raw, err := legal.SPDX(sbomInput(t))
	if err != nil {
		t.Fatalf("rendering the SBOM: %v", err)
	}
	var doc struct {
		Packages []struct {
			SPDXID          string `json:"SPDXID"`
			Name            string `json:"name"`
			VersionInfo     string `json:"versionInfo"`
			Download        string `json:"downloadLocation"`
			PackageFileName string `json:"packageFileName"`
			Checksums       []struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"checksumValue"`
			} `json:"checksums"`
			Comment string `json:"comment"`
		} `json:"packages"`
		Relationships []struct {
			From string `json:"spdxElementId"`
			Type string `json:"relationshipType"`
			To   string `json:"relatedSpdxElement"`
		} `json:"relationships"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the SBOM is not JSON: %v", err)
	}
	name := map[string]string{}
	for _, p := range doc.Packages {
		name[p.SPDXID] = p.Name
	}
	for _, c := range legal.Companions() {
		var id string
		for _, p := range doc.Packages {
			if p.Name != c.Name {
				continue
			}
			id = p.SPDXID
			if p.VersionInfo != c.Source.Version || p.Download != c.Source.URL || p.PackageFileName != c.Source.Archive {
				t.Errorf("%s is described as version %q from %q as %q - the registry says %q, %q, %q",
					c.Name, p.VersionInfo, p.Download, p.PackageFileName, c.Source.Version, c.Source.URL, c.Source.Archive)
			}
			if len(p.Checksums) != 1 || p.Checksums[0].Algorithm != "SHA256" || p.Checksums[0].Value != c.Source.SHA256 {
				t.Errorf("%s carries checksums %+v and it has to carry the archive's SHA256, %s", c.Name, p.Checksums, c.Source.SHA256)
			}
			for _, f := range c.Files {
				if !strings.Contains(p.Comment, f.Path) {
					t.Errorf("%s's comment does not name %s, so a reader cannot tell which files of the archive are meant", c.Name, f.Path)
				}
			}
		}
		if id == "" {
			t.Fatalf("the SBOM has no package for %q", c.Name)
		}
		from := map[string]string{}
		for _, r := range doc.Relationships {
			if r.To == id {
				from[name[r.From]] = r.Type
			}
		}
		if from[c.Binary] != "DEPENDS_ON" {
			t.Errorf("%s is related to %s as %q and it has to be DEPENDS_ON - shipped beside, not contained", c.Binary, c.Name, from[c.Binary])
		}
		for binary, kind := range from {
			if binary != c.Binary {
				t.Errorf("the SBOM relates %s to %s (%s), and only %s ships it", binary, c.Name, kind, c.Binary)
			}
		}
	}
}

// The notices name the companion, its files, where it comes from, and
// reproduce the text of every licence in its expression.
//
// A companion's licence expression is the licence of four projects at once,
// so it is checked part by part: each identifier in it has a section of
// licence text in the notices, because a licence named and not reproduced
// is a notice that fails at its only job. The archive itself carries no
// licence file - measured on 2026-09-17, the pinned archive holds one
// readme pointing at a web page - which is why the texts have to travel
// with ours.
func TestTheNoticesNameWhatShipsBesideTheWindow(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Skipf("no notices file here: %v", err)
	}
	body := string(raw)
	headings := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if h, found := strings.CutPrefix(line, "### "); found {
			headings[strings.TrimSpace(h)] = true
		}
	}
	for _, c := range legal.Companions() {
		for _, want := range []string{c.Name, c.Source.Version, c.Source.Archive, c.Source.SHA256, c.SPDX, c.Source.Project} {
			if !strings.Contains(body, want) {
				t.Errorf("the notices never say %q about %s", want, c.Name)
			}
		}
		for _, f := range c.Files {
			if !strings.Contains(body, f.Path) || !strings.Contains(body, f.SHA256) {
				t.Errorf("the notices do not name %s with its sum %s", f.Path, f.SHA256)
			}
		}
		for _, id := range licenceParts(c.SPDX) {
			if !headings[id] {
				t.Errorf("%s ships under %s and the notices reproduce no text under a \"### %s\" heading", c.Name, id, id)
			}
		}
	}
}

// licenceParts are the identifiers in a licence expression: what stands
// between the ANDs and WITHs. "MIT AND Apache-2.0 WITH LLVM-exception" is
// three texts a notice has to carry.
func licenceParts(expression string) []string {
	var parts []string
	for _, word := range strings.Fields(expression) {
		if word == "AND" || word == "WITH" || word == "OR" {
			continue
		}
		parts = append(parts, word)
	}
	return parts
}
