package guard

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The scanner guards the registry, not the other way round.
//
// internal/legal is the reviewed list of what this project ships and the bill
// of materials is generated from it, so the question worth asking on every
// change is the other one: does what we are about to publish contain something
// that list does not know? It has an answer already - seven fonts and
// ninety-seven drawings shipped inside the window binary for as long as it
// existed, named nowhere, because every check here asked about modules.

// runSBOMGate runs the gate over one pair of reports and returns its ending.
func runSBOMGate(t *testing.T, python string, scan, ours any) int {
	t.Helper()
	dir := t.TempDir()
	scanPath := writeJSON(t, filepath.Join(dir, "scan.json"), scan)
	oursPath := writeJSON(t, filepath.Join(dir, "ours.json"), ours)

	out, err := exec.Command(python, sbomGatePath(t), scanPath, oursPath).CombinedOutput()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("running the gate: %v\n%s", err, out)
	}
	t.Logf("gate said:\n%s", out)
	return exit.ExitCode()
}

func writeJSON(t *testing.T, path string, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("building a report: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing a report: %v", err)
	}
	return path
}

func sbomGatePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "sbom_gate.py")
}

// ourDocument is a small document of the shape the generator produces.
func ourDocument(names ...string) map[string]any {
	packages := []any{}
	for _, name := range names {
		packages = append(packages, map[string]any{"name": name})
	}
	return map[string]any{"packages": packages}
}

func scanOf(names ...string) map[string]any {
	artifacts := []any{}
	for _, name := range names {
		artifacts = append(artifacts, map[string]any{"name": name, "version": "1.0"})
	}
	return map[string]any{"artifacts": artifacts, "files": []any{map[string]any{}}}
}

// A dozen names, because the gate refuses a document too small to be the real
// one - a registry that lost its contents would otherwise account for nothing
// and pass everything.
func enoughNames() []string {
	names := []string{}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		names = append(names, "github.com/example/"+n)
	}
	return names
}

func TestTheBillOfMaterialsGateRefusesWhatTheRegistryDoesNotKnow(t *testing.T) {
	python := pythonForGate(t)
	known := enoughNames()

	if got := runSBOMGate(t, python, scanOf(known...), ourDocument(known...)); got != 0 {
		t.Errorf("a scan naming only known components ended %d, and everything in it is accounted for", got)
	}

	// The real shape of the defect this exists for: something ships that no
	// list knows. zlib is the name the sibling project found this way.
	dirty := append(append([]string{}, known...), "zlib")
	if got := runSBOMGate(t, python, scanOf(dirty...), ourDocument(known...)); got != 1 {
		t.Errorf("a scan naming a component the registry does not know ended %d, and it has to refuse", got)
	}

	// The runtime and the program itself are not dependencies and must not be
	// reported as unaccounted, or the gate cries wolf on every run and gets
	// switched off - which is worse than never having had it.
	fine := append(append([]string{}, known...), "stdlib", "tfg", "Testing Files Generator")
	if got := runSBOMGate(t, python, scanOf(fine...), ourDocument(known...)); got != 0 {
		t.Errorf("a scan naming the runtime and the program itself ended %d, and neither is a dependency", got)
	}
}

// A scanner that fell over is not a clean tree. Measured in this repository on
// 2026-08-27 with a different scanner: it printed four errors, exited zero and
// wrote a file that was not JSON. A gate reading the exit code would have
// called that green.
func TestAnUnusableScanIsNotACleanScan(t *testing.T) {
	python := pythonForGate(t)
	known := enoughNames()

	if got := runSBOMGate(t, python, map[string]any{}, ourDocument(known...)); got != 2 {
		t.Errorf("a scan naming no artifact at all ended %d, and it scanned nothing", got)
	}
	if got := runSBOMGate(t, python, scanOf(known...), map[string]any{"packages": []any{}}); got != 2 {
		t.Errorf("an empty document of our own ended %d, and it would account for nothing", got)
	}

	dir := t.TempDir()
	broken := filepath.Join(dir, "scan.json")
	if err := os.WriteFile(broken, []byte("<ERROR: missing output>"), 0o600); err != nil {
		t.Fatalf("writing a broken report: %v", err)
	}
	ours := writeJSON(t, filepath.Join(dir, "ours.json"), ourDocument(known...))
	err := exec.Command(python, sbomGatePath(t), broken, ours).Run()
	if err == nil {
		t.Error("a report that is not JSON was accepted as a clean scan")
	}
}

// And the workflow has to do what the gate assumes: build both binaries, scan
// what was built, and read the report rather than the scanner's exit code.
func TestTheWorkflowScansBothBinariesAndReadsTheReport(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skipf("no workflow here: %v", err)
	}
	workflow := string(body)

	// The flags between "go build" and the output path are not this guard's
	// business - build tags arrived there on 2026-08-29 and broke a check that
	// was quoting a whole command line to ask a question about two binaries.
	for _, want := range []string{
		"-trimpath -o dist/tfg ./cmd/tfg",
		"-trimpath -o dist/tfg-gui ./cmd/tfg-gui",
		"go run ./internal/legal/cmd/sbom",
		"python .github/scripts/sbom_gate.py scan.json ours.spdx.json",
	} {
		if !strings.Contains(workflow, want) {
			t.Errorf("the workflow does not run %q.\n"+
				"A gate nothing triggers is a gate that says nothing, which is the shape this "+
				"project has already paid for once.", want)
		}
	}

	// The window is where the fonts are. A job scanning only the command line
	// would pass while the thing the registry exists for went unchecked.
	if !strings.Contains(workflow, "libgl1-mesa-dev") {
		t.Error("the workflow does not install the headers the window needs, so it cannot have built it")
	}
}
