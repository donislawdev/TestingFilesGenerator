// Command sbom writes the bill of materials for a release.
//
// It is a build tool and never ships: a main package cannot be imported, so it
// cannot reach a binary by accident, and nothing in cmd/ refers to it.
//
// It asks the compiler what each binary links and hands that to the renderer in
// internal/legal, which joins it with the reviewed licences. The two halves are
// deliberately separate - the build knows versions and no licences, the
// registry knows licences and holds no versions - so neither can drift into
// being a copy of the other.
//
// Usage:
//
//	go run ./internal/legal/cmd/sbom -seed v0.1.0 -o tfg-0.1.0.spdx.json
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

func main() {
	out := flag.String("o", "", "write here instead of standard output")
	seed := flag.String("seed", "", "what makes the document namespace unique, normally the tag")
	created := flag.String("created", "", "creation time in RFC3339, defaults to now in UTC")
	flag.Parse()

	stamp := *created
	if stamp == "" {
		stamp = time.Now().UTC().Format(time.RFC3339)
	}

	binaries, err := describeBinaries()
	if err != nil {
		fail(err)
	}
	document, err := legal.SPDX(legal.Document{
		Version:  version.Version,
		Created:  stamp,
		Seed:     *seed,
		Binaries: binaries,
	})
	if err != nil {
		fail(err)
	}

	if *out == "" {
		if _, err := os.Stdout.Write(document); err != nil {
			fail(err)
		}
		return
	}
	// 0600 rather than 0644, and the linter asked for it rather than taste:
	// this file is written by a build job and read by the same one, and the
	// copy people download is the release asset rather than this one.
	if err := os.WriteFile(*out, document, 0o600); err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
}

// describeBinaries asks what each of the two binaries links.
//
// CGO is on for both, because with it off the toolkit hides behind build
// constraints and the window lists as the stub that has no window - a document
// that would then claim the window ships two libraries.
func describeBinaries() ([]legal.Binary, error) {
	var binaries []legal.Binary
	for _, target := range []struct{ name, path string }{
		{"tfg", "./cmd/tfg"},
		{"tfg-gui", "./cmd/tfg-gui"},
	} {
		versions, err := linked(target.path)
		if err != nil {
			return nil, err
		}
		binaries = append(binaries, legal.Binary{
			Name:      target.name,
			Modules:   versions,
			GoVersion: runtime.Version(),
		})
	}
	return binaries, nil
}

// linked is what one target links on EVERY system this project releases for,
// rather than on the one running the generator.
//
// Measured while writing this, on the generator's own first output: asked on
// Windows alone the window came back with twenty-seven modules instead of
// twenty-eight. github.com/rymdport/portal is linked on Linux only, so a
// document generated on this machine would have shipped without naming it -
// which is the same trap THIRD-PARTY-NOTICES.md records having fallen into,
// in almost the same words.
func linked(target string) (map[string]string, error) {
	versions := map[string]string{}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		found, err := linkedOn(target, goos)
		if err != nil {
			return nil, err
		}
		if err := merge(versions, found, goos); err != nil {
			return nil, err
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("%s reported no modules at all, which cannot be right", target)
	}
	return versions, nil
}

// merge folds one system's answer into the set, refusing a disagreement rather
// than picking a side. Two systems reporting different versions of one module
// is not something one document can describe.
func merge(into, found map[string]string, goos string) error {
	for path, moduleVersion := range found {
		if seen, ok := into[path]; ok && seen != moduleVersion {
			return fmt.Errorf("%s is %s on %s and %s elsewhere, so one document "+
				"cannot describe both", path, moduleVersion, goos, seen)
		}
		into[path] = moduleVersion
	}
	return nil
}

// linkedOn asks one system. CGO is on because with it off the toolkit hides
// behind build constraints and the window lists as the stub that has no window.
func linkedOn(target, goos string) (map[string]string, error) {
	//nolint:gosec // the command is the go tool and the target is one of the
	// two literals in describeBinaries - nothing here comes from outside.
	cmd := exec.Command("go", "list", "-deps", "-f",
		"{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}", target)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOOS="+goos)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("asking what %s links on %s: %w", target, goos, err)
	}
	versions := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		path, moduleVersion, found := strings.Cut(strings.TrimSpace(line), "@")
		if !found || strings.Contains(path, "donislawdev") {
			continue
		}
		versions[path] = moduleVersion
	}
	return versions, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "sbom: %v\n", err)
	os.Exit(1)
}
