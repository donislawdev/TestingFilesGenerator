package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// TestTheManifestSaysTheOutcomeDependsWhereReadersDiffer holds what the
// manifest says about a damaged file on both sides of the line.
//
// A file cut short is still text, so a txt cut short says unspecified, policy
// dependent, both answers offered and why - the shape MF5 gives a question
// with no single right answer. A PNG cut short has lost its end, which its
// rules forbid, and says reject with certainty. The settings count as well as
// the format: random bytes after UTF-8 text are not UTF-8, after UTF-16 text
// they mostly are. Owner's call on 2026-11-10.
//
// Read from the entry as it is written, because what a consumer finds there
// is the whole point of it.
func TestTheManifestSaysTheOutcomeDependsWhereReadersDiffer(t *testing.T) {
	for _, c := range []struct {
		format     string
		properties map[string]string
		damage     string
		certain    bool
	}{
		{"txt", nil, damage.Truncate, false},
		{"png", nil, damage.Truncate, true},
		{"png", nil, damage.AppendJunk, false},
		{"txt", nil, damage.AppendJunk, true},
		{"txt", map[string]string{"encoding": "utf-16le"}, damage.AppendJunk, false},
		{"png", nil, damage.ZeroHead, true},
	} {
		dir := t.TempDir()
		target := engine.Target{ID: "d", Format: c.format, Properties: c.properties,
			Sizes: engine.Uniform(1, 20<<10), Damage: damage.Chain{{ID: c.damage}}}
		entries, _ := lengthRun(t, dir, []engine.Target{target})
		e := entries["d"].Expected
		written, err := json.Marshal(entries["d"])
		if err != nil {
			t.Fatal(err)
		}
		where := c.format + " with " + c.damage
		if len(c.properties) > 0 {
			where += " in utf-16le"
		}
		if c.certain {
			if e.Outcome != manifest.OutcomeReject || e.Confidence != "certain" || bytes.Contains(written, []byte("policy_options")) {
				t.Errorf("%s breaks the rules of the format and the manifest says %s", where, written)
			}
			continue
		}
		d, _ := damage.Get(c.damage)
		pc, _ := d.PolicyFor(damage.Kind{Format: c.format, Settings: c.properties})
		if e.Outcome != manifest.OutcomeUnspecified || e.Confidence != "policy_dependent" ||
			!slices.Equal(e.PolicyOptions, []string{manifest.OutcomeAccept, manifest.OutcomeReject}) {
			t.Errorf("%s leaves the outcome to the reader and the manifest says %+v", where, e)
		}
		if !bytes.Contains(written, []byte(`"policy_options":["accept","reject"]`)) {
			t.Errorf("%s: the entry as it is written carries no policy_options: %s", where, written)
		}
		if e.Detail == "" || e.Detail != pc.Why {
			t.Errorf("%s: the manifest explains it with %q, and the declaration says %q", where, e.Detail, pc.Why)
		}
	}
}

// TestAcceptIsAllowedOnlyWhereTheOutcomeDependsOnTheReader asks both
// surfaces about the one expectation damage can rule out.
//
// Accept is refused beside a damage that breaks the rules of the format - a
// cut PNG - because nothing that follows them could accept it. Beside a
// damage that leaves the outcome to the reader - a cut txt - accept is one of
// the two answers the manifest offers, and a recipe stating it is writing
// down its own policy (owner's call, 2026-11-10, by the rule of 2026-09-09:
// refuse only what cannot be true). Both halves are asked, because a build
// refusing accept everywhere and a build refusing it nowhere would each pass
// one of them.
func TestAcceptIsAllowedOnlyWhereTheOutcomeDependsOnTheReader(t *testing.T) {
	body := func(f string) []byte {
		return []byte("version: 1\ntargets:\n  - id: t\n    format: " + f +
			"\n    size: 20kb\n    damage: [truncate]\n    expected: accept\n")
	}
	if _, err := recipe.Parse(body("png"), "r.yaml"); err == nil {
		t.Error("a recipe cutting a png short and expecting it to be accepted was read without a word")
	}
	if _, err := recipe.Parse(body("txt"), "r.yaml"); err != nil {
		t.Errorf("a recipe cutting a txt short and accepting it was refused, and accept is one of the answers its manifest offers: %v", err)
	}

	run := func(f string) (int, int) {
		dir := t.TempDir()
		code := cli.Run(context.Background(), []string{"generate", "--format", f, "--size", "1kb",
			"--damage", damage.Truncate, "--expected", "accept", "--out", dir}, io.Discard, io.Discard)
		left, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		return code, len(left)
	}
	if code, files := run("png"); code != cli.ExitUsage || files != 0 {
		t.Errorf("png cut short with --expected accept ended with %d and left %d file(s), expected %d and none",
			code, files, cli.ExitUsage)
	}
	if code, _ := run("txt"); code != cli.ExitOK {
		t.Errorf("txt cut short with --expected accept ended with %d rather than %d", code, cli.ExitOK)
	}
}

// TestAChainThatCutsAwayWhatItAddedIsRefused asks both surfaces about a
// truncate placed after an append-junk.
//
// Each damage moves bytes, so the check that a damage changed something
// passes both, and the manifest would list junk the file no longer holds.
// Refused in the plan, with nothing written. The same two the other way round
// are a legitimate file - cut, then junk after the cut - and the guard asks
// that too, so a build refusing every chain of the two would be red.
func TestAChainThatCutsAwayWhatItAddedIsRefused(t *testing.T) {
	eaten := damage.Chain{{ID: damage.AppendJunk}, {ID: damage.Truncate}}
	fine := damage.Chain{{ID: damage.Truncate}, {ID: damage.AppendJunk}}
	plan := func(chain damage.Chain) error {
		_, err := engine.Plan([]engine.Target{{ID: "t", Format: "png", Sizes: engine.Uniform(1, 20<<10), Damage: chain}},
			engine.Options{OutDir: t.TempDir(), Seed: 5})
		return err
	}
	var refusal *damage.EatenError
	if err := plan(eaten); !errors.As(err, &refusal) {
		t.Errorf("truncate after append-junk was planned (%v), and the cut takes away what the junk added", err)
	}
	if err := plan(fine); err != nil {
		t.Errorf("truncate before append-junk was refused, and it is a file with junk after a cut: %v", err)
	}

	if _, err := recipe.Parse([]byte("version: 1\ntargets:\n  - id: t\n    format: png\n    size: 20kb\n"+
		"    damage: [append-junk, truncate]\n"), "r.yaml"); err == nil || !strings.Contains(err.Error(), "cuts away") {
		t.Errorf("the recipe reader read truncate after append-junk without the refusal: %v", err)
	}

	dir := t.TempDir()
	var errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"generate", "--format", "png", "--size", "20kb",
		"--damage", damage.AppendJunk, "--damage", damage.Truncate, "--out", dir}, io.Discard, &errOut)
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if code != cli.ExitUsage || len(left) != 0 {
		t.Errorf("the command line ended with %d and left %d file(s) for truncate after append-junk: %s",
			code, len(left), errOut.String())
	}
}

// TestAnArchiveSizedByItsContentsRefusesADamageThatChangesTheLength holds the
// line between two definitions of one number.
//
// An archive whose size comes from what it holds has no size until it is
// written, so a damage that needs to know the size on disk - to cut to it,
// or to leave room for junk - has nothing to work from, and would have to
// make one up. zero-head keeps the length and is allowed there, which is the
// control: a build refusing every damage on such an archive would be a wall.
func TestAnArchiveSizedByItsContentsRefusesADamageThatChangesTheLength(t *testing.T) {
	body := func(d string) []byte {
		return []byte("version: 1\ntargets:\n  - id: t\n    format: zip\n    contains:\n      - format: txt\n" +
			"        size: 1kb\n        count: 2\n    damage: [" + d + "]\n")
	}
	for _, c := range []struct {
		damage  string
		refused bool
	}{{damage.Truncate, true}, {damage.AppendJunk, true}, {damage.ZeroHead, false}} {
		_, err := recipe.Parse(body(c.damage), "r.yaml")
		if c.refused != (err != nil) || (err != nil && !strings.Contains(err.Error(), "changes the length")) {
			t.Errorf("an archive sized by its contents with %s: refused %t, and the reader said %v", c.damage, c.refused, err)
		}

		// The engine answers the same, for a caller that never read a recipe.
		target := engine.Target{ID: "t", Format: "zip", SizeFromContents: true, Sizes: engine.Uniform(1, 0),
			Contains: []format.Content{{Format: "txt", Bytes: 1024, Count: 2}},
			Damage:   damage.Chain{{ID: c.damage}}}
		_, err = engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 5})
		var refusal *damage.NeedsSizeError
		if c.refused != errors.As(err, &refusal) || (!c.refused && err != nil) {
			t.Errorf("the engine planned an archive sized by its contents with %s: %v", c.damage, err)
		}
	}
}

// TestEveryPolicyCaseNamesAFormatAndSettingsThatExist keeps the declarations
// of where a damage depends on the reader honest about what they name.
//
// They are written by hand, as format ids and setting values, so a format
// renamed or a value dropped would leave a case that matches nothing - and
// the file it was written for would be called certain without anybody
// deciding it (pitfall 15).
func TestEveryPolicyCaseNamesAFormatAndSettingsThatExist(t *testing.T) {
	cases := 0
	for _, d := range damage.All() {
		for _, c := range d.Policy {
			cases++
			f, err := format.Get(c.Format)
			if err != nil {
				t.Errorf("%s declares a policy case for %q, which is not a format", d.ID, c.Format)
				continue
			}
			for name, values := range c.When {
				i := slices.IndexFunc(f.Properties, func(p format.Property) bool { return p.Name == name })
				if i < 0 {
					t.Errorf("%s on %s is narrowed to the setting %q, which %s does not have", d.ID, c.Format, name, c.Format)
					continue
				}
				for _, v := range values {
					if !slices.Contains(f.Properties[i].Choices, v) {
						t.Errorf("%s on %s is narrowed to %s=%s, which is not one of its choices %v",
							d.ID, c.Format, name, v, f.Properties[i].Choices)
					}
				}
			}
		}
	}
	if cases == 0 {
		t.Fatal("no damage declares a policy case, so this guard checked nothing")
	}
}
