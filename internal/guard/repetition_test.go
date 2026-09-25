package guard

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// repetition is one screen doing the same round of work again and again, and
// what each round came to. See TestTheSameWorkDoneAgainLeavesTheWindowAsItWas.
type repetition struct {
	t      *testing.T
	host   *fakeHost
	screen any
	// work is one round. It has to end where it began, because every round
	// after the first is compared with the first.
	work func(r *repetition)
	// before is every goroutine alive when the guard began, which a round is
	// not asked about - a worker of an earlier guard finishing late is that
	// guard's business.
	before map[string]bool
	rounds []roundCost
	now    *roundCost
	// sawWorker is whether the count of goroutines ever saw the worker of a
	// run while it was held. Asked only of a screen whose round runs.
	sawWorker bool
	runs      bool
}

// roundCost is what one round came to: the readings of the form and expansions
// of a preset each change caused, in the order the changes were made, and the
// things the screen could reach afterwards, by type.
type roundCost struct {
	order     []string
	readings  map[string][2]int
	reachable map[string]int
}

func newRepetition(t *testing.T, host *fakeHost, screen any) *repetition {
	return &repetition{t: t, host: host, screen: screen}
}

// change does one thing to the screen and keeps what it cost.
func (r *repetition) change(what string, act func()) {
	r.t.Helper()
	if _, again := r.now.readings[what]; again {
		r.t.Fatalf("%q is done twice in one round, and the second would hide what the first cost", what)
	}
	r.host.settles, r.host.expansions = 0, 0
	act()
	r.now.readings[what] = [2]int{r.host.settles, r.host.expansions}
	r.now.order = append(r.now.order, what)
}

// typeAndTypeBack types into a box and then puts back what it held, as two
// changes.
func (r *repetition) typeAndTypeBack(what string, box *parts.Entry, typed string) {
	r.t.Helper()
	was := box.Text
	if was == typed {
		r.t.Fatalf("%s already holds %q, so typing it would change nothing", what, typed)
	}
	r.change("typing into "+what, func() { box.SetText(typed) })
	r.change("typing "+what+" back", func() { box.SetText(was) })
}

// round does the work once and counts what it left.
func (r *repetition) round(n int) {
	r.t.Helper()
	r.now = &roundCost{readings: map[string][2]int{}}
	r.work(r)
	r.now.reachable = reachableByType(r.screen)
	r.rounds = append(r.rounds, *r.now)
	if left := r.goroutinesLeft(); len(left) > 0 {
		r.t.Errorf("round %d finished and %d goroutine(s) started during this guard are still running our code, "+
			"so every round leaves one more behind. The first of them:\n%s", n, len(left), firstLines(left[0], 24))
	}
}

// lookAtTheWorker is called while a run is held just before it reports. The
// worker is plainly running then, so a count that does not see it would not
// see one left behind either.
func (r *repetition) lookAtTheWorker() {
	r.runs = true
	for _, g := range goroutinesOfOurs(r.before) {
		if strings.Contains(g, modulePath+"/internal/gui/window.") {
			r.sawWorker = true
		}
	}
}

// goroutinesLeft is every goroutine started during this guard that is still
// running our code, waited for a little. The wait for a run is the worker
// closing a channel, and the worker returns only after that, so for a moment
// it is still there with nothing wrong - asking once would be a guard that
// fails on a busy machine and on nothing else.
func (r *repetition) goroutinesLeft() []string {
	deadline := time.Now().Add(5 * time.Second)
	for {
		left := goroutinesOfOurs(r.before)
		if len(left) == 0 || time.Now().After(deadline) {
			return left
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// compare asks every round after the first whether it came to the same as the
// first, and first whether the counts can see anything at all.
func (r *repetition) compare() {
	r.t.Helper()
	first := r.rounds[0]
	for _, what := range first.order {
		if first.readings[what][0] == 0 {
			r.t.Fatalf("%s read the form nought times in the first round - either nothing changed or the screen reads its form "+
				"without telling the host, and a change that grew from nought to nought would pass", what)
		}
	}
	if first.reachable["*parts.Entry"] == 0 || first.reachable["*fyne.Container"] == 0 {
		r.t.Fatalf("the screen reaches no box or no container through its fields (%v), so the count cannot see anything grow", first.reachable)
	}
	if r.runs && !r.sawWorker {
		r.t.Fatal("the count of goroutines never saw the worker of a run while it was held, so it would not see one left behind")
	}
	readings, reached := 0, 0
	for _, what := range first.order {
		readings += first.readings[what][0]
	}
	for _, n := range first.reachable {
		reached += n
	}
	r.t.Logf("%d rounds of %d changes: %d readings of the form a round, %d things reached after it, a run seen: %v",
		len(r.rounds), len(first.order), readings, reached, r.sawWorker)
	for i, later := range r.rounds[1:] {
		for _, what := range first.order {
			if got, want := later.readings[what], first.readings[what]; got != want {
				r.t.Errorf("%s read the form %d time(s) and expanded a preset %d time(s) in round %d, against %d and %d in round 1. "+
					"The same change costing more each time it is repeated is how a chain of listeners looks from outside",
					what, got[0], got[1], i+2, want[0], want[1])
			}
		}
		if grew := differences(first.reachable, later.reachable); grew != "" {
			r.t.Errorf("after round %d the screen reaches a different set of things than after round 1, in the same state: %s", i+2, grew)
		}
	}
}

// differences lists the types whose counts differ between two rounds, as
// "+3 *parts.Entry", sorted, or nothing when they agree.
func differences(was, is map[string]int) string {
	var out []string
	for kind := range union(was, is) {
		if d := is[kind] - was[kind]; d != 0 {
			out = append(out, fmt.Sprintf("%+d %s", d, kind))
		}
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func union(a, b map[string]int) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// canvasObject is the interface every drawn thing of the toolkit implements.
var canvasObject = reflect.TypeOf((*fyne.CanvasObject)(nil)).Elem()

// reachableByType counts the drawn things a screen can reach through its own
// fields, by type, each once.
//
// Through its fields rather than through what is drawn, because a defect of
// this kind does not have to be on the screen: a map keeping the controls of a
// removed batch is as much a leak as a box keeping its old panels. Pointers,
// interfaces, structs, slices, arrays and maps of the toolkit and of this
// module are followed. Functions are not - a closure is out of reach of
// reflection, which is written down as what this cannot see.
//
// A thing is told apart by its address AND its type. A widget of ours embeds
// the toolkit's widget first, so the two share an address, and counting by the
// address alone would count whichever the walk happened to meet first - and a
// walk through maps meets things in a different order every time.
func reachableByType(root any) map[string]int {
	type key struct {
		at   uintptr
		kind reflect.Type
	}
	counts := map[string]int{}
	seen := map[key]bool{}
	var visit func(v reflect.Value)
	visit = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface:
			if !v.IsNil() {
				visit(v.Elem())
			}
		case reflect.Pointer:
			k := key{at: v.Pointer(), kind: v.Type()}
			if v.IsNil() || !followed(v.Type().Elem()) || seen[k] {
				return
			}
			seen[k] = true
			if v.Type().Implements(canvasObject) {
				counts[v.Type().String()]++
			}
			visit(v.Elem())
		case reflect.Struct:
			if followed(v.Type()) {
				for i := 0; i < v.NumField(); i++ {
					visit(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			if mayHoldPointers(v.Type().Elem()) {
				for i := 0; i < v.Len(); i++ {
					visit(v.Index(i))
				}
			}
		case reflect.Map:
			if mayHoldPointers(v.Type().Key()) || mayHoldPointers(v.Type().Elem()) {
				for it := v.MapRange(); it.Next(); {
					visit(it.Key())
					visit(it.Value())
				}
			}
		}
	}
	visit(reflect.ValueOf(root))
	return counts
}

// followed is whether the walk goes inside a type: the toolkit's and this
// module's, and types with no package, like a struct written in place. Not
// this package's, because the stand in host holds the whole window, and not
// the standard library's, whose locks and clocks hold nothing drawn.
func followed(t reflect.Type) bool {
	p := t.PkgPath()
	if strings.HasPrefix(p, modulePath+"/internal/guard") {
		return false
	}
	return p == "" || strings.HasPrefix(p, "fyne.io/fyne/v2") || strings.HasPrefix(p, modulePath)
}

// mayHoldPointers is whether a value of this type can lead anywhere, so that a
// slice of bytes - a font is a few hundred thousand of them - is not walked
// one byte at a time.
func mayHoldPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return true
	case reflect.Struct:
		return followed(t)
	case reflect.Array:
		return mayHoldPointers(t.Elem())
	}
	return false
}

// goroutineIDs is every goroutine alive now, by the number the runtime gives it.
func goroutineIDs() map[string]bool {
	ids := map[string]bool{}
	for _, g := range allGoroutines() {
		ids[goroutineID(g)] = true
	}
	return ids
}

// goroutinesOfOurs is every goroutine, other than the one asking and those in
// before, with a function of this module on its stack outside this package.
func goroutinesOfOurs(before map[string]bool) []string {
	var out []string
	for _, g := range allGoroutines()[1:] {
		if before[goroutineID(g)] {
			continue
		}
		for _, line := range strings.Split(g, "\n") {
			if strings.HasPrefix(line, modulePath+"/internal/") && !strings.HasPrefix(line, modulePath+"/internal/guard") {
				out = append(out, g)
				break
			}
		}
	}
	return out
}

// allGoroutines is the stack of every goroutine, the one asking first - which
// is the order runtime.Stack promises.
func allGoroutines() []string {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Split(strings.TrimSpace(string(buf[:n])), "\n\n")
		}
		buf = make([]byte, 2*len(buf))
	}
}

// goroutineID is the number in "goroutine 42 [running]:".
func goroutineID(stack string) string {
	fields := strings.Fields(stack)
	if len(fields) < 2 {
		return stack
	}
	return fields[1]
}
