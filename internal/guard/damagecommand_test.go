package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The command line says what a damage TAKES, not only that it exists.
//
// This is the other half of TestTheWindowDrawsAFieldForEveryDamageParameter.
// The window drew the settings of a chosen damage from the declaration on the
// day damage arrived, and the command line named the damages and stopped
// there: to learn that zero-head takes bytes, and that bytes runs from 4 to
// 4096, the only way was to type a wrong value and read the refusal.
//
// The parity guard cannot see that gap. It counts recipe keys, formats and
// presets, and a damage parameter is none of the three - so this is a parity
// defect in quality rather than in reach, and it needed a guard of its own.
// Written up in docs/CORRUPTION-ARCHITECTURE-2026-09-08.md section 14.5.
//
// The expectation is REBUILT from the declaration rather than written out. A
// list of strings would prove today's damage and go quiet on the next one,
// which is the lesson the imagedim guard cost: a guard that reconstructs what
// the registry declares covers a field nobody has thought of yet.
func TestTheCommandLineSaysWhatEveryDamageTakes(t *testing.T) {
	all := damage.All()
	if len(all) == 0 {
		t.Fatal("no damage is registered, so this guard would pass against any build")
	}

	code, listing, errOut := run(t, "damage")
	if code != cli.ExitOK {
		t.Fatalf("listing the damages ended with %d: %s", code, errOut)
	}

	settings := 0
	for _, d := range all {
		if !strings.Contains(listing, d.ID) {
			t.Errorf("the list does not name %q:\n%s", d.ID, listing)
		}
		// The sentence, not only the name. A damage id is not self describing
		// the way a format id is - nobody reads "zero-head" and knows what
		// comes out - so a list of names alone would say no more than the
		// --damage flag help already says.
		if !strings.Contains(listing, d.Detail) {
			t.Errorf("the list names %q and does not say what it does:\n%s", d.ID, listing)
		}

		code, described, errOut := run(t, "damage", d.ID)
		if code != cli.ExitOK {
			t.Fatalf("describing %q ended with %d: %s", d.ID, code, errOut)
		}
		for _, p := range d.Parameters {
			// What a value may be, asked of the declaration itself. Allowed is
			// the sentence the window puts under the same field, so the two
			// surfaces cannot describe one setting in two ways.
			for _, want := range []string{p.Name, p.Allowed()} {
				if !strings.Contains(described, want) {
					t.Errorf("%s declares %q and the command line does not say %q:\n%s",
						d.ID, p.Name, want, described)
				}
			}
			settings++
		}
	}
	if settings == 0 {
		t.Fatal("no damage declares a setting, so the half that matters proved nothing")
	}

	// The detail view has to be a different answer from the list, or the
	// second command is a longer way of asking the first. What only it carries
	// is the range: with one damage registered there is no other way to tell
	// "it honoured the argument" from "it printed everything".
	ranged := 0
	for _, d := range all {
		_, described, _ := run(t, "damage", d.ID)
		for _, p := range d.Parameters {
			if strings.Contains(listing, p.Allowed()) {
				t.Errorf("the list already carries the range of %s.%s, so asking about "+
					"one damage adds nothing", d.ID, p.Name)
			}
			if strings.Contains(described, p.Allowed()) {
				ranged++
			}
		}
	}
	if ranged == 0 {
		t.Fatal("no range reached the detail view, so the comparison above proved nothing")
	}
	t.Logf("%d damage setting(s) described from the registry", settings)
}

// The number this command announces as the smallest file is a number a run
// accepts.
//
// The same promise TestTheSmallestSizeIsAcceptedForEveryFormat makes about tfg
// formats, and it is here because that column got it wrong once: until
// 2026-08-04 the MINIMUM of tfg formats was the structural floor of the format
// rather than what a run would take, so pdf announced 3265 and refused it.
//
// The floor is DERIVED from the declaration rather than typed, which is what
// separates this from the engine guards beside it - those name 8 because
// zero-head defaults to eight bytes, and would go quiet if the default moved.
// Pressed through the whole command line rather than through Plan, because the
// claim is about what the tool takes and not about what one function believes.
func TestTheSmallestFileTheDamageCommandAnnouncesIsOneARunAccepts(t *testing.T) {
	all := damage.All()
	if len(all) == 0 {
		t.Fatal("no damage is registered, so this guard would pass against any build")
	}

	carrier, err := format.Get("txt")
	if err != nil {
		t.Fatalf("txt is the format small enough to sit on these floors: %v", err)
	}
	smallestFile := carrier.SmallestAccepted(format.Request{})

	for _, d := range all {
		floor := announcedFloor(t, d.ID)
		if floor < smallestFile {
			t.Fatalf("%s says it needs %d B and the smallest txt is %d B, so this "+
				"guard can no longer put the question", d.ID, floor, smallestFile)
		}

		at := strconv.FormatInt(floor, 10)
		code, _, errOut := run(t, "generate", "--format", "txt", "--size", at,
			"--damage", d.ID, "--clean", "--out", t.TempDir())
		if code != cli.ExitOK {
			t.Errorf("%s announces %s B as the smallest file and a run of exactly that "+
				"ended with %d: %s", d.ID, at, code, errOut)
		}

		// The other end, so this cannot be satisfied by a build that accepts
		// everything. One byte under has to be refused, and the refusal has to
		// name the number that was announced rather than only saying no.
		under := strconv.FormatInt(floor-1, 10)
		code, _, errOut = run(t, "generate", "--format", "txt", "--size", under,
			"--damage", d.ID, "--clean", "--out", t.TempDir())
		if code == cli.ExitOK {
			t.Errorf("%s announces %s B as the smallest file and a run of %s B was accepted",
				d.ID, at, under)
		}
		if !strings.Contains(errOut, at) {
			t.Errorf("%s refused %s B without naming the %s B it announced: %s",
				d.ID, under, at, errOut)
		}
	}
}

// announcedFloor is the smallest file the command prints for one damage, read
// out of the machine readable form.
func announcedFloor(t *testing.T, id string) int64 {
	t.Helper()
	code, stdout, errOut := run(t, "damage", id, "--json")
	if code != cli.ExitOK {
		t.Fatalf("asking about %q as JSON ended with %d: %s", id, code, errOut)
	}
	var entries []struct {
		SmallestFileBytes int64 `json:"smallest_file_bytes"`
	}
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("the JSON for %q does not parse: %v\n%s", id, err, stdout)
	}
	if len(entries) != 1 {
		t.Fatalf("asking about one damage returned %d entries", len(entries))
	}
	return entries[0].SmallestFileBytes
}

// Every declared setting reaches the machine readable form, filled in.
//
// The keys being present is the easy half and was once the only half: tfg
// formats emptied every value and kept all five keys, so a script would have
// drawn a field with no unit, no default and no help text and nothing would
// have said so. This asks for the values.
func TestTheMachineReadableDamageListCarriesTheWholeDeclaration(t *testing.T) {
	code, stdout, errOut := run(t, "damage", "--json")
	if code != cli.ExitOK {
		t.Fatalf("the machine readable list ended with %d: %s", code, errOut)
	}

	var entries []struct {
		ID                string `json:"id"`
		Detail            string `json:"detail"`
		SmallestFileBytes int64  `json:"smallest_file_bytes"`
		Parameters        []struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			Min     int64  `json:"min"`
			Max     int64  `json:"max"`
			Unit    string `json:"unit"`
			Default string `json:"default"`
			Detail  string `json:"detail"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("the list does not parse: %v\n%s", err, stdout)
	}
	if len(entries) != len(damage.All()) {
		t.Fatalf("the registry holds %d damage(s) and the list carries %d",
			len(damage.All()), len(entries))
	}

	checked := 0
	for _, e := range entries {
		d, err := damage.Get(e.ID)
		if err != nil {
			t.Errorf("the list carries %q, which the registry does not know", e.ID)
			continue
		}
		if e.Detail != d.Detail {
			t.Errorf("%s: the list says %q and the registry says %q", e.ID, e.Detail, d.Detail)
		}
		if e.SmallestFileBytes != d.Floor(d.Defaults()) {
			t.Errorf("%s: the list says %d B and the declaration says %d B",
				e.ID, e.SmallestFileBytes, d.Floor(d.Defaults()))
		}
		if len(e.Parameters) != len(d.Parameters) {
			t.Errorf("%s declares %d setting(s) and the list carries %d",
				e.ID, len(d.Parameters), len(e.Parameters))
			continue
		}
		for i, p := range d.Parameters {
			got := e.Parameters[i]
			if got.Name != p.Name || got.Kind != string(p.Kind) || got.Default != p.Default {
				t.Errorf("%s.%s arrives as name %q kind %q default %q",
					e.ID, p.Name, got.Name, got.Kind, got.Default)
			}
			if got.Detail != p.Detail {
				t.Errorf("%s.%s arrives with detail %q, and the declaration says %q",
					e.ID, p.Name, got.Detail, p.Detail)
			}
			if p.Kind == format.PropertyInt && (got.Min != p.Min || got.Max != p.Max) {
				t.Errorf("%s.%s runs from %d to %d and arrives as %d to %d",
					e.ID, p.Name, p.Min, p.Max, got.Min, got.Max)
			}
			if got.Unit != p.Unit {
				t.Errorf("%s.%s has unit %q and arrives with %q", e.ID, p.Name, p.Unit, got.Unit)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no setting was compared, so this proved nothing")
	}
}

// Every command that gets its own arguments is watched for --help.
//
// commandsTakingHelp is written out by hand, and the comment above it says
// that adding a command without adding it there leaves that command unwatched.
// Nothing held that. Adding tfg damage on 2026-09-09 meant remembering two
// hand kept lists in two files, which is the shape this project has been
// bitten by before - the mutation list went stale on twenty eight entries the
// same way.
//
// The rule is mechanical rather than a list of exceptions. A case in the
// dispatch that hands args[1:] to something takes flags of its own and
// therefore takes --help. version, license and help do not: they answer and
// return, so they are not exempted here, they simply do not match. A list of
// exceptions would have had nine entries against ten commands, and a ratchet
// that is mostly excuse teaches its reader to skip it.
func TestEveryCommandTakingItsOwnArgumentsIsWatchedForHelp(t *testing.T) {
	dispatched := commandsTakingTheirOwnArguments(t)
	if len(dispatched) == 0 {
		t.Fatal("not one verb refused an unknown flag, so this guard would pass against any tree")
	}

	watched := map[string]bool{}
	for _, cmd := range commandsTakingHelp {
		watched[cmd[0]] = true
	}
	for _, name := range dispatched {
		if !watched[name] {
			t.Errorf("tfg %s takes its own arguments and is not in commandsTakingHelp, "+
				"so nothing asks whether it answers --help on stdout with code 0", name)
		}
	}
	t.Logf("%d command(s) take their own arguments", len(dispatched))
}

// commandsTakingTheirOwnArguments is every verb whose own flag set reads what
// follows it, found by ASKING each one rather than by reading the source.
//
// It read the dispatch out of cli.go with go/ast until 2026-09-09, and the
// switch it parsed no longer exists - the verbs live in one declaration now,
// which is what O201 was about. Rewriting the parser to walk that declaration
// instead would have kept a reader of syntax where a measurement will do.
//
// The measurement is an unknown flag. A command with its own flag set refuses
// it with ExitUsage, and one that ignores what follows answers normally -
// measured 2026-09-09 across all eleven verbs, splitting them eight to three
// with no exception to write down. That is the same split the parser produced,
// arrived at by watching behaviour instead of shape.
func commandsTakingTheirOwnArguments(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, c := range cli.Commands() {
		var stdout, stderr bytes.Buffer
		code := cli.Run(context.Background(), []string{c.Verb, "--nosuchflag"}, &stdout, &stderr)
		if code == cli.ExitUsage {
			out = append(out, c.Verb)
		}
	}
	return out
}
