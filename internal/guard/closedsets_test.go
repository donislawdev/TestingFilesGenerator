package guard

import (
	"errors"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// A closed set is closed, and the registry is what says so.
//
// It did not. Property.Allows compared a choice with EqualFold and a boolean
// with ToLower, so a value spelled in a case the declaration does not use went
// straight past the registry and arrived at the generator - and what happened
// next was decided by whichever generator caught it. Measured on 2026-09-02
// across all eight formats that declare a closed set, twenty one settings,
// there were FOUR different answers and not one of them was declared:
//
//	refuses in its own words   csv, ico, log, wav - the two voices of O168
//	folds and understands it   pdf page_size, directory_entries in both archives
//	swallows it and makes the  targz entry_owner - USER and ROOT come out byte
//	  DEFAULT file, exit 0     for byte identical to unset, with nothing said
//	reads it as another value  zip encryption=NONE, which then demands a
//	                           password "to lock with NONE"
//
// The third one is rule 6 broken outright: a file that is not what was ordered,
// reported as success. The fourth quotes a value the format does not have.
//
// So the fix is one place rather than twenty one: the registry stops folding,
// and no generator is ever handed a value its declaration does not contain.
// These guards are the reason that stays true for the format added tomorrow.
func TestAValueTheDeclarationDoesNotSpellIsRefusedByTheRegistry(t *testing.T) {
	asked, skipped := 0, 0

	for _, d := range format.All() {
		for _, p := range d.Properties {
			values := p.Choices
			if p.Kind == format.PropertyBool {
				values = []string{"true", "false"}
			}
			if p.Kind != format.PropertyChoice && p.Kind != format.PropertyBool {
				continue
			}
			for _, v := range values {
				shouted := strings.ToUpper(v)
				if shouted == v {
					// A value with no letters in it - a permission like 644, a
					// bit depth like 16 - cannot be spelled in another case at
					// all. Counted rather than passed over silently, because a
					// day when every value is a number is a day this guard
					// walks nothing and says so.
					skipped++
					continue
				}
				asked++
				t.Run(d.ID+" "+p.Name+"="+shouted, func(t *testing.T) {
					err := d.CheckProperties(map[string]string{p.Name: shouted})
					if err == nil {
						t.Fatalf("%s took %s=%s, which its declaration spells %q. "+
							"Whatever the generator does with it next is its own "+
							"decision, and there are four of those in this tree",
							d.ID, p.Name, shouted, v)
					}

					// Refused is not enough. It has to be refused HERE, in the
					// declaration's words, or the two voices are back with one
					// of them further away.
					var bad *format.PropertyValueError
					if !errors.As(err, &bad) {
						t.Fatalf("%s refused %s=%s with %T, not with the "+
							"registry's own refusal: %v", d.ID, p.Name, shouted, err, err)
					}
					if bad.Key != p.Name {
						t.Errorf("the refusal names %q, and the setting is %q", bad.Key, p.Name)
					}
					if bad.Value != shouted {
						t.Errorf("the refusal quotes %q, and the value was %q", bad.Value, shouted)
					}
					// The declared spelling still works, or this guard would be
					// green on a registry that refuses everything.
					if err := d.CheckProperties(map[string]string{p.Name: v}); err != nil {
						t.Errorf("%s refuses %s=%s, which is its own declared value: %v",
							d.ID, p.Name, v, err)
					}
				})
			}
		}
	}

	if asked == 0 {
		t.Fatalf("no closed set carried a letter, so this guard asked nothing "+
			"(%d values had no letters to shout)", skipped)
	}
	t.Logf("%d values asked in a case the declaration does not use, %d had no letters", asked, skipped)
}

// An owner the archive does not know is refused, rather than quietly becoming
// no owner at all.
//
// ReadOwnership was a switch with no default: anything that was not root or
// user fell through to the zero value, which is the same archive as unset. The
// registry now stops a misspelling before it gets here, so this is about the
// other door - the function is callable directly, and a guard calling it is
// exactly such a caller. Silence there would be a file that is not what was
// asked for, which is rule 6 whichever door it came through.
func TestAnOwnerTheArchiveDoesNotKnowIsRefusedRatherThanDropped(t *testing.T) {
	for _, raw := range []string{"USER", "nobody", "root "} {
		t.Run(raw, func(t *testing.T) {
			own, err := archive.ReadOwnership("targz", map[string]string{archive.EntryOwner: raw})
			if err == nil {
				t.Fatalf("an owner of %q was taken and turned into %+v, so the archive "+
					"is not the one that was ordered and nothing said so", raw, own)
			}
			var bad *format.PropertyValueError
			if !errors.As(err, &bad) {
				t.Fatalf("refused %q with %T, not with the refusal every other "+
					"setting uses: %v", raw, err, err)
			}
			if bad.Key != archive.EntryOwner {
				t.Errorf("the refusal names %q, and the setting is %q", bad.Key, archive.EntryOwner)
			}
		})
	}

	// Every declared owner still passes, or the guard above would be satisfied
	// by a function that refuses everything.
	for _, owner := range []string{archive.OwnerRoot, archive.OwnerUser, archive.OwnerUnset} {
		if _, err := archive.ReadOwnership("targz", map[string]string{archive.EntryOwner: owner}); err != nil {
			t.Errorf("%q is a declared owner and was refused: %v", owner, err)
		}
	}
	// And an absent setting is not a misspelling. It is the common case.
	if _, err := archive.ReadOwnership("targz", map[string]string{}); err != nil {
		t.Errorf("an archive with no owner setting was refused: %v", err)
	}
}

// The manifest says who owns the entries and what mode they carry.
//
// It said neither, and that is why the swallowed owner above was invisible: the
// file was wrong, the run said success, and the one document a test suite reads
// had nothing to disagree with. Measured on 2026-09-02 - a run with
// entry_owner=user and one with entry_owner=USER produced manifests that were
// identical, because both were silent.
//
// Written every time rather than only when set, like depth and compression
// beside them, so a harness never has to read a missing key as "nobody owns
// this".
func TestTheArchiveManifestSaysWhoOwnsTheEntries(t *testing.T) {
	d, err := format.Get("targz")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		props      map[string]string
		mode, wner string
	}{
		{map[string]string{}, "644", archive.OwnerUnset},
		{map[string]string{archive.EntryOwner: archive.OwnerRoot}, "644", archive.OwnerRoot},
		{map[string]string{archive.EntryMode: "755", archive.EntryOwner: archive.OwnerUser},
			"755", archive.OwnerUser},
	} {
		name := tc.mode + "/" + tc.wner
		t.Run(name, func(t *testing.T) {
			p, err := d.Generator.Plan(format.Request{
				Bytes: 20480, Seed: 3, Label: true, Properties: tc.props,
			})
			if err != nil {
				t.Fatalf("planning %v: %v", tc.props, err)
			}
			if got := p.Properties[archive.EntryMode]; got != tc.mode {
				t.Errorf("the manifest says entry_mode %v, and the archive was built with %s",
					got, tc.mode)
			}
			if got := p.Properties[archive.EntryOwner]; got != tc.wner {
				t.Errorf("the manifest says entry_owner %v, and the archive was built with %s",
					got, tc.wner)
			}
		})
	}
}
