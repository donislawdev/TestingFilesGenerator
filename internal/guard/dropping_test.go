package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// SetOnDropped keeps the function the window gives for a drop. The toolkit's
// own test window throws it away (fyne v2.8.2, test/window.go), so without
// this no guard could drop anything onto a screen.
func (h *fakeHost) SetOnDropped(fn func(fyne.Position, []fyne.URI)) { h.onDropped = fn }

// drop drops paths onto the window the way the toolkit's driver hands them
// over - each one through storage.NewFileURI - and waits for the window to
// answer, the disk included.
func (h *fakeHost) drop(t *testing.T, paths ...string) {
	t.Helper()
	h.dropNow(t, paths...)
	join(h)
}

// dropNow drops without waiting, for a guard looking at the middle of a run,
// which cannot join it. Only a drop of several things is answered at once,
// because only that one is refused without asking the disk.
func (h *fakeHost) dropNow(t *testing.T, paths ...string) {
	t.Helper()
	if h.onDropped == nil {
		t.Fatal("the window gave the host no function for a drop, so nothing dropped onto it can arrive")
	}
	uris := make([]fyne.URI, len(paths))
	for i, p := range paths {
		uris[i] = storage.NewFileURI(p)
	}
	h.onDropped(fyne.NewPos(10, 10), uris)
}

// dropAim is the shape of window.AimDrop, so the planted defects below can
// stand in for it.
type dropAim func([]window.DroppedItem, []window.DropBox, float32, float32, bool) window.DropAnswer

// dropCase is one row: what was dropped, which boxes the screen has, where it
// landed, and what has to come of it.
type dropCase struct {
	name  string
	items []window.DroppedItem
	boxes []window.DropBox
	x, y  float32
	busy  bool
	want  window.DropAnswer
}

// dropCases are the rules, one row each - see dropaim.go for why each is
// where it is.
func dropCases() []dropCase {
	aFolder := []window.DroppedItem{{Path: "dir", Takes: parts.TakesFolder}}
	aFile := []window.DroppedItem{{Path: "a.zip", Takes: parts.TakesFile}}
	upper, lower := window.Area{X: 0, Y: 100, W: 300, H: 40}, window.Area{X: 0, Y: 200, W: 300, H: 40}
	oneFolderBox := []window.DropBox{{Takes: parts.TakesFolder}}
	twoFolderBoxes := []window.DropBox{{Takes: parts.TakesFolder, Area: upper}, {Takes: parts.TakesFolder, Area: lower}}
	none := window.DropAnswer{Box: -1}
	return []dropCase{
		{"nothing dropped", nil, oneFolderBox, 0, 0, false, none},
		{"a screen with no box", aFolder, nil, 0, 0, false, none},
		{"work owns the screen", aFolder, oneFolderBox, 0, 0, true, none},
		{"two things", append(aFolder, aFolder...), oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedMany}},
		{"a name that is not text", []window.DroppedItem{{NotText: true}}, oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedNotText}},
		{"a path the disk cannot read", []window.DroppedItem{{Path: "x", Unread: errors.New("gone")}}, oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedUnread}},
		{"neither a folder nor a file", []window.DroppedItem{{Path: "pipe"}}, oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedNeither}},
		{"a folder into the one folder box", aFolder, oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Put: true}},
		{"a file onto a screen that takes a folder", aFile, oneFolderBox, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedKind}},
		{"the one box of its kind, wherever it landed", aFolder,
			[]window.DropBox{{Takes: parts.TakesFile, Area: upper}, {Takes: parts.TakesFolder, Area: lower}}, 10, 110, false,
			window.DropAnswer{Box: 1, Put: true}},
		{"between two of a kind, the one under it", aFolder, twoFolderBoxes, 10, 210, false,
			window.DropAnswer{Box: 1, Put: true}},
		{"between two of a kind, on neither", aFolder, twoFolderBoxes, 10, 10, false,
			window.DropAnswer{Box: 0, Why: window.RefusedAimless}},
		{"between two of a kind, onto a box of the other kind", aFolder,
			append([]window.DropBox{{Takes: parts.TakesFile, Area: window.Area{X: 0, Y: 300, W: 300, H: 40}}}, twoFolderBoxes...), 10, 310, false,
			window.DropAnswer{Box: 0, Why: window.RefusedKind}},
		{"a box that is not drawn is under nothing", aFolder,
			[]window.DropBox{{Takes: parts.TakesFolder}, {Takes: parts.TakesFolder, Area: lower}}, 0, 0, false,
			window.DropAnswer{Box: 0, Why: window.RefusedAimless}},
	}
}

// dropFailures is every row an aim answers wrongly.
func dropFailures(decide dropAim) []string {
	var out []string
	for _, c := range dropCases() {
		if got := decide(c.items, c.boxes, c.x, c.y, c.busy); got != c.want {
			out = append(out, fmt.Sprintf("%s: got %+v, want %+v", c.name, got, c.want))
		}
	}
	return out
}

// A drop goes where the rules say, for every rule.
//
// Asked of the decision itself rather than through a screen, because no
// screen today has two boxes of one kind - and the day one does, the rows
// about the position are already answered. The canaries are three defects
// this table has to see: a drop that always goes into the first box, a busy
// screen taking it anyway, and the position asked when only one box takes the
// kind - the defect that would put a folder in the wrong box on a Linux
// desktop where the toolkit's position is stale.
func TestADropGoesWhereItsRulesSay(t *testing.T) {
	inside := func(a window.Area, x, y float32) bool { return x >= a.X && x < a.X+a.W && y >= a.Y && y < a.Y+a.H }
	planted := map[string]dropAim{
		"always the first box": func(items []window.DroppedItem, boxes []window.DropBox, _, _ float32, _ bool) window.DropAnswer {
			if len(items) == 0 || len(boxes) == 0 {
				return window.DropAnswer{Box: -1}
			}
			return window.DropAnswer{Box: 0, Put: true}
		},
		"busy ignored": func(items []window.DroppedItem, boxes []window.DropBox, x, y float32, _ bool) window.DropAnswer {
			return window.AimDrop(items, boxes, x, y, false)
		},
		"the position always asked": func(items []window.DroppedItem, boxes []window.DropBox, x, y float32, busy bool) window.DropAnswer {
			got := window.AimDrop(items, boxes, x, y, busy)
			if got.Put && len(boxes) > 1 && !inside(boxes[got.Box].Area, x, y) {
				return window.DropAnswer{Box: got.Box, Why: window.RefusedAimless}
			}
			return got
		},
	}
	for name, decide := range planted {
		if len(dropFailures(decide)) == 0 {
			t.Errorf("the planted defect %q passes every row, so this table cannot see it", name)
		}
	}
	for _, f := range dropFailures(window.AimDrop) {
		t.Error(f)
	}
}

// awkwardFolder is a folder whose name carries what a path loses on the way
// through an address: a hash starts a fragment, a percent sign an escape, a
// space is escaped, and a letter outside ASCII is several bytes.
func awkwardFolder(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a#b %41 ś")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A folder dropped onto each screen that writes files becomes its output
// directory, exactly as dropped, and the screens that write nothing take
// nothing.
//
// Through the window as built, with the path turned into the toolkit's address
// and back the way its driver does it, so a path that came out of the box
// different from the one dropped - a hash cut off, a slash turned - is red
// here. About and Preferences have no path box, and the owner's decision of
// 2026-11-14 is that a drop there does nothing: the directory on the work
// screens is asked afterwards, so a drop that went to the wrong screen shows.
func TestAFolderDroppedOnAWorkScreenBecomesItsOutputDirectory(t *testing.T) {
	host, _ := screen(t)
	before := entryUnder(t, tabNamed(t, host.content, text.TabOneTarget()), text.FieldOutputDir()).Text
	for _, tab := range []string{text.TabAbout(), text.TabPreferences()} {
		selectTab(t, host.content, tab)
		host.drop(t, awkwardFolder(t))
	}
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		if got := entryUnder(t, tabNamed(t, host.content, tab), text.FieldOutputDir()).Text; got != before {
			t.Errorf("a folder dropped onto About or Preferences changed the output directory on %q to %q", tab, got)
		}
	}
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		content := selectTab(t, host.content, tab)
		dir := awkwardFolder(t)
		host.drop(t, dir)
		if got := entryUnder(t, content, text.FieldOutputDir()).Text; got != dir {
			t.Errorf("a folder dropped onto %q left the output directory as %q rather than %q", tab, got, dir)
		}
		if said := sayingUnder(t, content, text.FieldOutputDir()); said != "" {
			t.Errorf("a folder dropped onto %q was taken and the box still says %q", tab, said)
		}
	}
}

// The toolkit's address keeps the paths a drop or a picker can bring, network
// paths on Windows among them, and window.PathOf gives back the same path.
//
// Asked of the toolkit rather than of the disk, because a network path cannot
// be made on a runner. The window relies on storage.NewFileURI keeping the
// path whole and on the slashes being the only change (fyne v2.8.2,
// storage/repository/parse.go), and a toolkit that began escaping would be
// red here before it reached a box. PathOf is what both pickers use too, so
// a folder picked and the same folder dropped put the same text in the box.
func TestADroppedPathComesBackOutOfTheToolkitAsItWent(t *testing.T) {
	paths := []string{filepath.Join(t.TempDir(), "a#b %41 ś", "x.zip")}
	if runtime.GOOS == "windows" {
		paths = append(paths, `\\server\share\a#b\x`, `C:\`)
	}
	changed := func(back func(fyne.URI) string) []string {
		var out []string
		for _, p := range paths {
			if got := back(storage.NewFileURI(p)); got != p {
				out = append(out, fmt.Sprintf("%q came back as %q", p, got))
			}
		}
		return out
	}
	// The canary: the whole address, escaped, is not the path - so a check
	// that let it through could not see a toolkit that started escaping.
	if len(changed(func(u fyne.URI) string { return u.String() })) == 0 {
		t.Fatal("the escaped address came back as every path, so this check cannot see escaping")
	}
	for _, c := range changed(window.PathOf) {
		t.Errorf("a path went into the toolkit's address and %s", c)
	}
}

// A drop on the Tools tab goes into the box of the chosen tool when the tool
// takes that kind, and is refused under it when it does not.
//
// The kind comes from the tool's declaration in the engine, so this asks two
// tools that declare different kinds rather than one, and a box that took
// whatever was dropped would be red on the second.
func TestADropOnTheToolsTabFollowsWhatTheChosenToolTakes(t *testing.T) {
	host, _ := screen(t)
	content := selectTab(t, host.content, text.TabTools())
	folder := awkwardFolder(t)
	file := filepath.Join(folder, "x.zip")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	chooseTool(t, content, checksum.ID)
	box := entryUnder(t, content, text.SettingLabel(checksum.InputFile))
	host.drop(t, folder)
	if box.Text != "" {
		t.Errorf("a folder dropped onto a tool that reads a file went into its box as %q", box.Text)
	}
	if said := sayingUnder(t, content, text.SettingLabel(checksum.InputFile)); !strings.Contains(said, "x.zip") && !strings.Contains(said, filepath.Base(folder)) {
		t.Errorf("a folder dropped onto a tool that reads a file was refused with %q, which does not name what was dropped", said)
	}
	host.drop(t, file)
	if box.Text != file {
		t.Errorf("a file dropped onto a tool that reads a file left its box as %q rather than %q", box.Text, file)
	}
	if said := sayingUnder(t, content, text.SettingLabel(checksum.InputFile)); said != "" {
		t.Errorf("the file was taken and the box still says %q from the drop before it", said)
	}

	chooseTool(t, content, checksum.WriteID)
	host.drop(t, folder)
	if got := entryUnder(t, content, text.SettingLabel(checksum.InputFolder)).Text; got != folder {
		t.Errorf("a folder dropped onto a tool that lists a folder left its box as %q rather than %q", got, folder)
	}
}

// A drop the box cannot take says why under the box and leaves what was in it.
//
// Three ways, each a sentence of its own: more than one thing, a path that is
// not there, and a file where a folder goes. The value is asked after each,
// because a refusal shown over a value that changed anyway is the worst of
// both.
func TestADropTheBoxCannotTakeSaysWhyAndChangesNothing(t *testing.T) {
	host, content := screen(t)
	before := entryUnder(t, content, text.FieldOutputDir()).Text
	folder := awkwardFolder(t)
	file := filepath.Join(folder, "report.pdf")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		dropped []string
		says    string
	}{
		{"two folders", []string{folder, folder}, text.DropOneAtATime(2)},
		{"a path that is not there", []string{filepath.Join(folder, "gone")}, "gone"},
		{"a file where a folder goes", []string{file}, text.DropTakesFolder(text.FieldOutputDir(), "report.pdf")},
	}
	for _, c := range cases {
		host.drop(t, c.dropped...)
		said := sayingUnder(t, content, text.FieldOutputDir())
		if !strings.Contains(said, c.says) {
			t.Errorf("%s: the box says %q, which does not say %q", c.name, said, c.says)
		}
		if got := entryUnder(t, content, text.FieldOutputDir()).Text; got != before {
			t.Errorf("%s: the drop was refused and the output directory changed to %q", c.name, got)
		}
	}
}

// A drop while work owns the screen changes nothing and says nothing.
//
// The work read the boxes when it began. Asked during a preview held in the
// middle, with two folders, because that drop is answered at once - a single
// one would be answered after the disk, by when the preview may be over. The
// same drop with the screen idle is the positive control: refused with a
// sentence, so a guard that saw no sentence during the preview saw the rule
// and not a drop that never arrived.
func TestADropWhileWorkOwnsTheScreenChangesNothing(t *testing.T) {
	host, content, hold := heldScreen(t)
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldCount(), "20000")
	before := entryUnder(t, content, text.FieldOutputDir()).Text
	folder := awkwardFolder(t)
	press(t, content, text.ButtonPreview())
	hold.look(func() {
		host.dropNow(t, folder, folder)
		if said := sayingUnder(t, content, text.FieldOutputDir()); said != "" {
			t.Errorf("a drop during a preview put %q under the output directory, which nothing takes away when the preview ends", said)
		}
		if got := entryUnder(t, content, text.FieldOutputDir()).Text; got != before {
			t.Errorf("a drop during a preview changed the output directory to %q", got)
		}
	})
	join(host)
	host.drop(t, folder, folder)
	if said := sayingUnder(t, content, text.FieldOutputDir()); said == "" {
		t.Error("the same drop with the screen idle said nothing, so the silence during the preview proves nothing")
	}
}
