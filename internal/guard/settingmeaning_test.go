package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// declaredSetting is one format's declaration of one setting.
type declaredSetting struct {
	format   string
	property format.Property
}

// deliberateHomonyms are the names two formats spell alike and mean
// differently on purpose, with the reason each one is allowed.
//
// There is one, and it is here rather than absent because the guard that would
// have banned it was written on 2026-09-01 and deleted the same day for going
// red on it. "entry" is ordinary English and no package owns it - the note at
// the foot of archiveaxes_test.go carries that measurement in full. What this
// file refuses is an ACCIDENT wearing the same shape, so the deliberate case
// has to be sayable.
//
// A name written here is checked from both sides. It has to still be shared,
// and the formats sharing it have to still disagree, because an excuse that
// outlived its case is read by the next person as a rule.
var deliberateHomonyms = map[string]string{
	"entry_format": "log means the shape of a log line - apache-combined, nginx, syslog - " +
		"where a container means the format of the files it holds",
}

// One name has to describe one kind of setting.
//
// Twenty four formats declare seventy two settings between them under thirty
// nine names, and thirteen of those names are written by more than one format.
// Nothing compared them until 2026-09-08. What held them together was that
// somebody kept them alike by hand, which is the mechanism archiveaxes_test.go
// replaced for containers - and the one that let thirteen packages carry four
// different versions of a single filler loop.
//
// The drift this closes is quiet on both surfaces. "tfg formats" prints each
// declaration from the format that made it, and a window builds each field
// from that format's own Detail, so a width offered as free text by one format
// and as a bounded number by another draws two different fields with nothing
// said. A recipe moved between two formats then behaves differently for a
// reason nobody can see.
//
// What it asks is narrow, and the narrowness is measured rather than cautious.
// Three fields diverge legitimately today and are deliberately NOT asked about:
//
//   - Max. width runs to 20000 for five formats, 16384 for AVIF and JXL, 16383
//     for WEBP and 256 for ICO, and each is the ceiling of the thing itself.
//   - Default. quality is 60 for AVIF and 90 for JPG and JXL, and both carry
//     weight: the AVIF ladder ceilings were measured at its default and the
//     JPG minimum is the floor of an ordinary run at its own.
//   - Min. columns starts at 2 for CSV and at 1 for XLSX.
//
// Kind and Unit are what is left, and they are where a typo lands: a unit of
// "px" beside nine of "pixels", or an eleventh picture format declaring width
// as free text. Neither moves a byte, and both would sit unnoticed until
// somebody read ten declarations side by side.
//
// Asked of every registered format and of every name any two of them share, so
// the fifteenth picture format and the twentieth text format are covered on the
// day they arrive rather than the day somebody remembers this file.
func TestOneSettingNameMeansOneKindOfSetting(t *testing.T) {
	shared := settingsMoreThanOneFormatDeclares()
	if len(shared) == 0 {
		t.Fatal("no setting name is written by two formats, so this proved nothing")
	}

	compared := 0
	for name, declared := range shared {
		if _, deliberate := deliberateHomonyms[name]; deliberate {
			continue
		}
		compared++
		first := declared[0]
		for _, other := range declared[1:] {
			if other.property.Kind != first.property.Kind {
				t.Errorf("%q is a %s in %s and a %s in %s - one name has to describe one kind of setting, "+
					"or it is a homonym and belongs in deliberateHomonyms with the reason",
					name, first.property.Kind, first.format, other.property.Kind, other.format)
			}
			if other.property.Unit != first.property.Unit {
				t.Errorf("%q counts %q in %s and %q in %s - the same setting cannot count two things",
					name, first.property.Unit, first.format, other.property.Unit, other.format)
			}
		}
	}

	if compared == 0 {
		t.Fatal("every shared name was excused as a homonym, so this proved nothing")
	}
}

// An excuse has to still describe the tree it excuses.
//
// The list above is the one place this guard can be told to look away, so it is
// the one place a stale entry turns into silence. Two ways it can go stale and
// they fail differently: the name stops being shared at all, which is an excuse
// with nothing behind it, or the formats sharing it come into agreement, which
// is an excuse still pointing at a case that has been fixed. Either way the
// next person reads it as a rule saying these names may drift.
//
// The same shape as the exception registry in notelemetry_test.go, for the same
// reason: consent that outlived its subject is consent nobody gave.
func TestEveryDeliberateHomonymStillNamesTwoMeanings(t *testing.T) {
	shared := settingsMoreThanOneFormatDeclares()

	for name, why := range deliberateHomonyms {
		declared, isShared := shared[name]
		if !isShared {
			t.Errorf("%q is excused as a homonym (%s) and no two formats declare it - "+
				"take the excuse off, an excuse with nothing behind it reads as a rule", name, why)
			continue
		}
		if oneMeaning(declared) {
			t.Errorf("%q is excused as a homonym (%s) and every format now declares it the same way - "+
				"take the excuse off, the case it names has been closed", name, why)
		}
	}
}

// settingsMoreThanOneFormatDeclares is every setting name at least two
// registered formats write, with each declaration and the format that made it.
func settingsMoreThanOneFormatDeclares() map[string][]declaredSetting {
	everywhere := map[string][]declaredSetting{}
	for _, d := range format.All() {
		for _, p := range d.Properties {
			everywhere[p.Name] = append(everywhere[p.Name], declaredSetting{format: d.ID, property: p})
		}
	}

	shared := make(map[string][]declaredSetting, len(everywhere))
	for name, declared := range everywhere {
		if len(declared) > 1 {
			shared[name] = declared
		}
	}
	return shared
}

// oneMeaning says whether every format declaring this name describes the same
// kind of setting by it.
func oneMeaning(declared []declaredSetting) bool {
	first := declared[0].property
	for _, other := range declared[1:] {
		if other.property.Kind != first.Kind || other.property.Unit != first.Unit {
			return false
		}
	}
	return true
}
