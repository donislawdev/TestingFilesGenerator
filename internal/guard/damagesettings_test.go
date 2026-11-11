package guard

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// A damage setting its declaration does not allow is refused by the engine,
// for the command line and the window, which hand it a chain without a recipe.
//
// Measured 2026-11-11 before this held: truncate:missing=95, append-junk with
// bytes=65537 and zero-head with bytes=99999999999 each ended with code 0, the
// file was cut or zeroed by the default, and the manifest recorded the value
// as written (O332). The recipe reader refused all of them, so the rule
// existed - the paths without a recipe never reached it.
//
// Every damage and every setting comes from the registry, with values just
// outside the declared range, one that is not a value at all, and a setting the
// damage does not take. Each is asked of the engine, which must refuse it at
// the address the recipe reader gives (the damage is second in its chain, so
// a position read off the wrong entry shows), and of the command line, which
// must end with USAGE, name the setting and write nothing.
//
// The control is the default of every setting in the same chain: planned. A
// build refusing every damage setting whatever its value would pass the half
// above, and this half is what tells the two apart.
func TestEveryDamageSettingItsDeclarationForbidsIsRefusedWithoutARecipe(t *testing.T) {
	// The canary: the check below on two answers it must not take - nothing
	// refused, and the right refusal about the first entry of the chain.
	planted := &damage.SettingError{Position: 1, Err: &format.PropertyValueError{Format: damage.ZeroHead, Key: damage.SettingBytes}}
	if refusedAsTheSetting(nil, damage.SettingBytes) == "" || refusedAsTheSetting(planted, damage.SettingBytes) == "" {
		t.Fatal("the check took a plan that went through, or a refusal about the wrong entry, for the refusal it asks for")
	}

	asked := 0
	for _, d := range damage.All() {
		type stated struct{ name, value string }
		cases := []stated{{"no-such-setting", "1"}}
		for _, p := range d.Parameters {
			for _, bad := range valuesTheDeclarationForbids(p) {
				cases = append(cases, stated{p.Name, bad})
			}
		}
		for _, c := range cases {
			spec := damage.Spec{ID: d.ID, Values: damage.Values{c.name: c.value}}
			if why := refusedAsTheSetting(planSecondInAChain(t, spec), c.name); why != "" {
				t.Errorf("%s %s", spec, why)
			}
			code, said, files := damageFromTheCommandLine(t, spec.String())
			if code != cli.ExitUsage || files != 0 || !strings.Contains(said, c.name) {
				t.Errorf("--damage %s ended with %d and wrote %d file(s), expected %d and none, with the setting named: %s",
					spec, code, files, cli.ExitUsage, said)
			}
			asked++
		}
		for _, p := range d.Parameters {
			spec := damage.Spec{ID: d.ID, Values: damage.Values{p.Name: p.Default}}
			if err := planSecondInAChain(t, spec); err != nil {
				t.Errorf("%s states the declared default and was refused - the rule refuses what it should let through: %v", spec, err)
			}
		}
	}
	if asked < len(damage.All())*2 {
		t.Fatalf("only %d value(s) were asked across %d damage(s) - the registry was not read", asked, len(damage.All()))
	}
}

// valuesTheDeclarationForbids is what a person can type that p does not take:
// one step past each end of a bounded number, and a word where a value is
// expected. Free text has none.
func valuesTheDeclarationForbids(p format.Property) []string {
	switch p.Kind {
	case format.PropertyInt:
		out := []string{"abc"}
		if p.Min != 0 || p.Max != 0 {
			out = append(out, strconv.FormatInt(p.Min-1, 10), strconv.FormatInt(p.Max+1, 10))
		}
		return out
	case format.PropertyText:
		return nil
	}
	return []string{"no-such-value"}
}

// planSecondInAChain plans a txt file damaged by zero-head and then spec.
func planSecondInAChain(t *testing.T, spec damage.Spec) error {
	t.Helper()
	target := engine.Target{ID: "t", Format: "txt", Sizes: engine.Uniform(1, 64<<10), Label: true,
		Damage: damage.Chain{{ID: damage.ZeroHead}, spec}}
	_, err := engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 1})
	return err
}

// refusedAsTheSetting says what is wrong with err as the refusal of setting in
// the second entry of a chain, or nothing when that is what it is.
func refusedAsTheSetting(err error, setting string) string {
	if err == nil {
		return "was planned"
	}
	var wrong *damage.SettingError
	if !errors.As(err, &wrong) {
		return "was refused, but not as a damage setting: " + err.Error()
	}
	var about interface{ AboutSetting() string }
	want := core.TargetAddress(1, core.DamageAddress(2, setting))
	if !errors.As(err, &about) || about.AboutSetting() != want {
		return "was refused somewhere other than " + want + ", where the recipe reader puts the same mistake: " + err.Error()
	}
	return ""
}

// damageFromTheCommandLine runs generate with one --damage and says how it
// ended, what it said and how many files it left.
func damageFromTheCommandLine(t *testing.T, spec string) (int, string, int) {
	t.Helper()
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{
		"generate", "--format", "txt", "--size", "64kb", "--damage", spec, "--out", dir,
	}, &out, &errOut)
	written, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the output directory: %v", err)
	}
	return code, errOut.String(), len(written)
}
