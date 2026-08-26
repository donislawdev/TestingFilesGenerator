package guard

import (
	"errors"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// One quantity asked two ways has to get one answer.
//
// A container takes the number of files it holds either as the entries
// property or as a contains list, and until 2026-08-26 only one of them had a
// ceiling. Measured that day: a recipe asking for fifty thousand entries
// through contains validated clean and dry ran clean, while entries=50000 was
// refused with "it takes a whole number from 0 to 10000". Same number, same
// format, two answers - and the door with no ceiling built a format.Plan for
// every child, which is what the ceiling exists to stop.
//
// The exit code counted too, and was the part that nearly slipped through. A
// plain error on the contains path lands on 1, the general failure, while the
// property path lands on 4, the code the frozen table gives to "no format here
// can deliver that". A script branching on the code would have seen two
// different kinds of failure for one mistake.
//
// Asked of every registered container rather than of zip alone, so a third
// container is covered on the day it is added.
func TestBothWaysOfAskingForEntriesShareOneCeiling(t *testing.T) {
	containers := 0
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		containers++

		ceiling := declaredCeiling(t, d)
		over := ceiling + 1

		// The property door. Asked through CheckEachProperty rather than
		// through Plan, because that is the layer a caller meets: the declared
		// range is enforced from the declaration before a generator sees the
		// value, and the generator's own reading of it is a second line the
		// engine never reaches. Asking Plan here would compare the two doors at
		// a layer neither surface uses.
		propBad := d.CheckEachProperty(map[string]string{"entries": itoa(over)})
		var viaProperty error
		if len(propBad) > 0 {
			viaProperty = propBad[0]
		}

		// The contains door, asking for the same number.
		req := planRequestWith(nil)
		req.Contains = []format.Content{{Format: "txt", Count: over, Bytes: 1}}
		_, viaContains := d.Generator.Plan(req)

		if viaProperty == nil {
			t.Errorf("%s: entries=%d was accepted and the declared ceiling is %d", d.ID, over, ceiling)
		}
		if viaContains == nil {
			t.Errorf("%s: contains asking for %d entries was accepted and the declared ceiling is %d",
				d.ID, over, ceiling)
			continue
		}

		// Both have to be the same class of refusal, because that is what
		// decides the exit code and what a window marks.
		var propErr, containsErr *format.PropertyValueError
		if !errors.As(viaProperty, &propErr) {
			t.Errorf("%s: the entries refusal is %T, not a refusal about a property value", d.ID, viaProperty)
		}
		if !errors.As(viaContains, &containsErr) {
			t.Errorf("%s: the contains refusal is %T, not a refusal about a property value - "+
				"so it lands on a different exit code than the same request through entries", d.ID, viaContains)
			continue
		}

		// And they have to name the same ceiling, in the same words. Comparing
		// the reason rather than the whole sentence, because the key differs
		// on purpose - one says entries and the other says contains.
		if propErr != nil && propErr.Reason != containsErr.Reason {
			t.Errorf("%s: the two doors give different reasons\n  entries:  %s\n  contains: %s",
				d.ID, propErr.Reason, containsErr.Reason)
		}
		if !strings.Contains(containsErr.Reason, itoa(ceiling)) {
			t.Errorf("%s: the contains refusal does not name the ceiling %d: %q",
				d.ID, ceiling, containsErr.Reason)
		}
	}

	if containers == 0 {
		t.Fatal("no container is registered, so this proved nothing")
	}
}

// The number under the ceiling still has to work, on both doors.
//
// A ceiling that refuses everything would pass the test above while making the
// format useless, which is the shape this project has thrown away defensive
// code for before.
func TestTheEntryCeilingStillAcceptsTheNumberBelowIt(t *testing.T) {
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		ceiling := declaredCeiling(t, d)

		req := planRequestWith(nil)
		req.Contains = []format.Content{{Format: "txt", Count: ceiling, Bytes: 1}}
		req.SizeFromContents = true
		if _, err := d.Generator.Plan(req); err != nil {
			var value *format.PropertyValueError
			if errors.As(err, &value) && strings.Contains(value.Key, "contains") {
				t.Errorf("%s: contains asking for exactly the ceiling of %d was refused: %v",
					d.ID, ceiling, err)
			}
		}
	}
}

// planRequestWith is a request big enough that size is never the reason.
func planRequestWith(props map[string]string) format.Request {
	return format.Request{
		Bytes:      1 << 30,
		Seed:       7,
		Label:      true,
		Properties: props,
	}
}

// declaredCeiling reads the ceiling out of the registry rather than repeating
// the number here. A guard carrying its own copy of a limit goes stale the day
// somebody changes the real one, and says nothing while it does.
func declaredCeiling(t *testing.T, d format.Descriptor) int {
	t.Helper()
	for _, p := range d.Properties {
		if p.Name == "entries" {
			return int(p.Max)
		}
	}
	t.Fatalf("%s is a container with no declared entries property", d.ID)
	return 0
}
