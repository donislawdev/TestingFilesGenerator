package guard

import (
	"reflect"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// Two containers offering one setting have to be offering the same setting.
//
// Until 2026-09-01 nothing said so. ZIP and TAR.GZ each wrote out the entries,
// entry_format and entry_size declarations in full, and the two copies were
// identical because somebody had kept them that way - the whole mechanism was a
// comment in the second one saying it matched the first on purpose. Measured
// that day: the two blocks were the same to the byte apart from the wording of
// one comment, and six constants behind them agreed the same way.
//
// A setting that drifts here does not fail loudly. tfg formats zip and tfg
// formats targz would simply print different defaults for a key with one name,
// a window would draw two different fields for it, and a recipe moved from one
// container to the other would behave differently with nothing said. That is
// the quiet kind, which is the kind this project writes guards for.
//
// Asked of every registered container and of every axis the archive package
// declares, so a third container and a fourth setting are both covered on the
// day they arrive rather than the day somebody remembers this file.
func TestEveryContainerDeclaresTheSharedAxesAsTheyAreDeclaredOnce(t *testing.T) {
	containers := 0
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		containers++
		declared := byName(d.Properties)

		for _, axis := range archive.Names() {
			p, offered := declared[axis]
			if !offered {
				// Not every container has to carry every axis - tar takes an
				// owner and a mode that zip has nowhere to put. What is
				// forbidden is carrying one and meaning something else by it.
				continue
			}
			want := archive.Axes(axis)[0]
			if !reflect.DeepEqual(p, want) {
				t.Errorf("%s declares %q its own way rather than the way the archive package declares it\n"+
					"  container: %+v\n"+
					"  shared:    %+v",
					d.ID, axis, p, want)
			}
		}
	}

	if containers == 0 {
		t.Fatal("no container is registered, so this proved nothing")
	}
}

// There was a second guard here on 2026-09-01 and it is worth saying why it is
// not, because the reason is a measurement rather than a change of mind.
//
// It said that a format which is not a container may not declare any of these
// names at all, on the argument that a PNG offering entries would grow a field
// asking how many files a picture holds. Run against the registry it went red
// at once: log declares entry_format, and means the shape of a log line by it -
// apache-combined, nginx, syslog - where an archive means the format of the
// files it holds.
//
// That is not the collision it looked like. The window draws each field from
// the declaring format's own Detail, and SettingLabel only spaces and
// capitalises the key, so neither surface shows one sentence for two meanings.
// What it did show is that the guard's premise was false: "entry" is ordinary
// English and the archive package has no claim on it. entries and entry_size
// are just as reasonable for a log - "how many entries" is a log setting this
// project has already named as deferred, in docs/MVP-FORMATS.md section 5.1 -
// so the guard would have gone red the day that work started, on a format doing
// nothing wrong.
//
// A defence that reddens on the legitimate case is worse than none, so it went.
// The naming hazard it was reaching for is real and belongs in GLOSSARY.md,
// where distinctions that have cost something already live, rather than in a
// test asserting a rule the tree disproves.

// byName is a format's declarations keyed by the name a recipe writes.
func byName(props []format.Property) map[string]format.Property {
	out := make(map[string]format.Property, len(props))
	for _, p := range props {
		out[p.Name] = p
	}
	return out
}
