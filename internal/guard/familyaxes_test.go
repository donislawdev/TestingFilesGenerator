package guard

import (
	"reflect"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagedim"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// Two families of formats share a declaration through a package, and this is
// what stops a member writing its own copy instead.
//
// The containers have had this since 2026-09-01 - archiveaxes_test.go compares
// every axis a container offers against the one archive declares. The other two
// families had nothing, and the gap was measured rather than suspected. On
// 2026-09-09 md was given a declaration of its own that agreed on kind, unit
// and default and offered one encoding fewer, and bmp was given one whose width
// started at zero. ELEVEN guards stayed green on the first and TEN on the
// second, including the ones that look closest: TestOneSettingNameMeansOneKind
// OfSetting asks only about Kind and Unit, and the README and format-document
// guards compare the prose against whatever the registry happens to say.
//
// What each of the two ran into is different, and both are worth naming:
//
//   - md kept writing utf-16be perfectly well and stopped OFFERING it, so txt
//     and xml answered one question and md answered another.
//   - bmp printed "whole number of pixels from 0 to 20000" and refused width=0
//     with "it has to be between 1 and 20000". The declaration contradicted the
//     program, and what caught it was the backstop in imagedim.Value - the one
//     picturesides_test.go says a person cannot reach. A person reaches it
//     exactly when a declaration drifts, which is the case with no guard.
//
// Both guards skip a name listed in deliberateHomonyms, so the one list this
// package already keeps stays the one place a shared name can be excused. A
// second list of excuses beside it would be a second place to look away from.
//
// The cost of sharing that list is worth stating rather than leaving to be
// found: an entry there turns the name off for EVERY format, not only for the
// one it excuses, so writing "encoding" into it would silence half of the first
// guard. That is a deliberate act and a visible one - the entry is in the diff,
// and TestEveryDeliberateHomonymStillNamesTwoMeanings makes it prove that two
// formats really do mean different things by the name - but it is a door, and
// archiveaxes_test.go has none because it keeps no list at all. Narrowing the
// skip to the one format that earned it would mean changing the shape of
// deliberateHomonyms, which is another guard's, so it is written down here
// instead of done quietly.

// Every text format declares the encoding settings the way textenc declares
// them.
//
// Asked by name against textenc.Names(), so a third axis added to that package
// is covered on the day it arrives rather than the day somebody remembers this
// file. A format is free not to carry an axis at all - that is what
// textenc.Axes narrows, and HTML is the named case, since its specification
// leaves it no encoding to choose and only the mark to declare. What is
// refused is carrying one and describing it differently.
func TestEveryTextFormatDeclaresTheEncodingSettingsAsTheyAreDeclaredOnce(t *testing.T) {
	compared := 0
	for _, d := range format.All() {
		declared := byName(d.Properties)
		for _, axis := range textenc.Names() {
			if _, excused := deliberateHomonyms[axis]; excused {
				continue
			}
			p, offered := declared[axis]
			if !offered {
				continue
			}
			compared++
			want := textenc.Axes(axis)[0]
			if !reflect.DeepEqual(p, want) {
				t.Errorf("%s declares %q its own way rather than the way textenc declares it\n"+
					"  format: %+v\n"+
					"  shared: %+v",
					d.ID, axis, p, want)
			}
		}
	}

	if compared == 0 {
		t.Fatal("no format declares a text encoding setting, so this proved nothing")
	}
	t.Logf("%d text encoding declaration(s) compared against the one definition", compared)
}

// Every picture format declares its sides the way imagedim would build them.
//
// This one cannot be a straight comparison, because three of the fields are the
// format's own and are SUPPOSED to differ: the largest side is the ceiling of
// the thing itself, the default is declared by SVG alone, and the sentence is
// per format on purpose - four of the ten are correctly different, and a shared
// one would have made three of them wrong.
//
// So it asks the question the other way round: rebuild the declaration from the
// parts a format supplies and require the result to be what is registered.
// Whatever the format did NOT supply - the name, that it is a whole number,
// that the smallest side is one pixel, that the number counts pixels - has to
// come out of the package, and anything a format added by hand shows up as a
// difference. That covers a field nobody has thought of yet, which a list of
// four field comparisons would not.
func TestEveryPictureFormatDeclaresItsSidesAsTheImageDimensionPackageWould(t *testing.T) {
	rebuild := map[string]func(imagedim.Side) format.Property{
		imagedim.SettingWidth:  imagedim.Width,
		imagedim.SettingHeight: imagedim.Height,
	}

	compared := 0
	for _, d := range format.All() {
		for _, p := range d.Properties {
			build, isSide := rebuild[p.Name]
			if !isSide {
				continue
			}
			if _, excused := deliberateHomonyms[p.Name]; excused {
				continue
			}
			compared++
			want := build(imagedim.Side{Largest: p.Max, Default: p.Default, Detail: p.Detail})
			if !reflect.DeepEqual(p, want) {
				t.Errorf("%s declares %q its own way rather than through imagedim\n"+
					"  format:      %+v\n"+
					"  the package: %+v",
					d.ID, p.Name, p, want)
			}
		}
	}

	if compared == 0 {
		t.Fatal("no format declares a picture side, so this proved nothing")
	}
	t.Logf("%d picture side declaration(s) rebuilt from what the format supplies", compared)
}
