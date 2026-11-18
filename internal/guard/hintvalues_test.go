package guard

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/tool/all"
)

// The start of a setting's explanation - what it takes, its default and when
// it applies - marks the values a person types, as the sentences after it do:
// the owner's decision of 2026-11-18, everywhere and on the command line too
// (O358, docs/HINT-VALUES-2026-11-18.md). Before it, `groups` read "group names
// separated by commas, or all, default all" above a sentence that marked all.

// TestEveryValueASettingTakesIsMarked reads what every setting of every format,
// preset, damage and tool takes, as tfg formats prints it, and asks each value
// a person types in it to stand between backticks: every choice, the default,
// true and false, and in a condition the setting and its values. A value
// holding a backtick of its own could not be marked at all, so that is asked
// too, and so is a mark left without its partner.
//
// The canaries are the phrase as it read before the decision, three values of
// a choice with a default, none of them marked, and the same phrase with the
// choices marked and the default not: the default is one of the choices, and
// the choice marked earlier must not answer for it. The phrase marked in full
// has to pass, or the check finds faults that are not there.
func TestEveryValueASettingTakesIsMarked(t *testing.T) {
	canary := format.Property{Name: "x", Kind: format.PropertyChoice, Choices: []string{"a", "b"}, Default: "a"}
	for phrase, want := range map[string]int{
		"one of: a, b, default a":       3,
		"one of: `a`, `b`, default a":   1,
		"one of: `a`, `b`, default `a`": 0,
	} {
		if got := unmarkedValues(canary, phrase); len(got) != want {
			t.Fatalf("canary: a choice of a and b with the default a, read in %q, gave %d fault(s) where %d were due: %q",
				phrase, len(got), want, got)
		}
	}

	checked := 0
	check := func(owner string, props []format.Property) {
		for _, p := range props {
			phrase := p.Allowed()
			if !p.OnlyWhen.IsZero() {
				phrase += ", " + p.OnlyWhen.Describe()
			}
			for _, fault := range unmarkedValues(p, phrase) {
				t.Errorf("%s %s: %s", owner, p.Name, fault)
			}
			checked++
		}
	}
	for _, d := range format.All() {
		check("format "+d.ID, d.Properties)
	}
	for _, p := range preset.All() {
		check("preset "+p.ID, p.Parameters)
	}
	for _, d := range damage.All() {
		check("damage "+d.ID, d.Parameters)
	}
	for _, d := range tool.All() {
		check("tool "+d.ID, d.Settings)
	}
	if checked < 150 {
		t.Fatalf("%d settings read - 200 were declared on 2026-11-18, so the registries were not filled here", checked)
	}
}

// unmarkedValues is each value of p that phrase does not mark, each value that
// holds a backtick, and a mark in phrase left without its partner. The default
// is asked for in its own clause, "default" and the value marked, because it
// is often one of the choices and a choice marked earlier would answer for it.
func unmarkedValues(p format.Property, phrase string) []string {
	values := slices.Clone(p.Choices)
	if p.Kind == format.PropertyBool {
		values = append(values, "true", "false")
	}
	if !p.OnlyWhen.IsZero() {
		values = append(append(values, p.OnlyWhen.Setting), p.OnlyWhen.Values...)
	}
	marked := make([]string, len(values))
	for i, v := range values {
		marked[i] = core.Literal(v)
	}
	if p.Default != "" {
		values = append(values, p.Default)
		marked = append(marked, "default "+core.Literal(p.Default))
	}
	var faults []string
	for i, v := range values {
		switch {
		case strings.Contains(v, core.Backtick):
			faults = append(faults, fmt.Sprintf("the value %q holds a backtick, so no mark can carry it", v))
		case !strings.Contains(phrase, marked[i]):
			faults = append(faults, fmt.Sprintf("%q is not marked in %q", v, phrase))
		}
	}
	if strings.Count(phrase, core.Backtick)%2 != 0 {
		faults = append(faults, fmt.Sprintf("a mark is left without its partner in %q", phrase))
	}
	return faults
}

// TestTheHelpOfAFlagKeepsTheMarksOfItsValues asks the help a command prints
// for its flags to keep the backticks of a usage as they are written.
//
// flag.PrintDefaults takes the first word of a usage written between backticks
// for the name of the flag's value, prints it after the flag and drops the
// marks. A preset's settings are flags of generate, and their usage marks
// values: on 2026-11-18 "tfg generate --preset picture-sizes" with a flag it
// does not know printed "-groups screens", with screens the one value bare.
//
// The canary is that habit itself, on a set of one flag: printed by
// PrintDefaults, the check has to find the marks lost. Then the real command,
// the help it prints when a flag is wrong.
func TestTheHelpOfAFlagKeepsTheMarksOfItsValues(t *testing.T) {
	fs := flag.NewFlagSet("canary", flag.ContinueOnError)
	fs.String("groups", "", "group names, or `all`. Leave it at `all`, or name `screens`.")
	var plain bytes.Buffer
	fs.SetOutput(&plain)
	fs.PrintDefaults()
	if len(flagMarksLost(plain.String(), "groups", []string{"all", "screens"})) == 0 {
		t.Fatalf("canary: PrintDefaults kept every mark of the usage, so this check cannot see one dropped:\n%s", plain.String())
	}

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"generate", "--preset", "picture-sizes", "--no-such-flag"}, &out, &errOut); code != cli.ExitUsage {
		t.Fatalf("a flag generate does not know ended %d rather than %d, so its help was not asked for:\n%s", code, cli.ExitUsage, errOut.String())
	}
	for _, lost := range flagMarksLost(errOut.String(), "groups", []string{"all", "screens", "avatars", "social"}) {
		t.Error(lost)
	}
}

// flagMarksLost is what the help of one flag lost of its marks: the line of the
// flag naming a value rather than its type, and each value printed bare.
func flagMarksLost(help, name string, values []string) []string {
	var lost []string
	head := "  -" + name + " "
	at := strings.Index(help, head)
	if at < 0 {
		return []string{fmt.Sprintf("the help names no -%s:\n%s", name, help)}
	}
	line, _, _ := strings.Cut(help[at:], "\n")
	if strings.TrimPrefix(line, head) != "string" {
		lost = append(lost, fmt.Sprintf("-%s is printed as %q, a value taken for the name of its type", name, line))
	}
	usage, _, _ := strings.Cut(help[at+len(line)+1:], "\n  -")
	for _, v := range values {
		if !strings.Contains(usage, core.Literal(v)) {
			lost = append(lost, fmt.Sprintf("%q is printed without its marks under -%s: %s", v, name, strings.TrimSpace(usage)))
		}
	}
	return lost
}
