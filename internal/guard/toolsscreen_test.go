package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// The Tools tab as somebody uses it - the owner's list of 2026-10-05 after
// looking at the window with the folder tools in it.

// chooseToolIn is a step of a scene: the tool with this ID chosen in the menu
// at the head of the Tools tab, the way a person picks it.
func chooseToolIn(id string) func(t *testing.T, s scene) {
	return func(t *testing.T, s scene) {
		d, err := tool.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		menuUnder(t, s.tab, text.FieldTool()).SetSelected(text.ToolChoice(d.ID, d.Question))
	}
}

// TestChoosingAToolLaysTheScreenOutAsIfItOpenedWithIt chooses every tool on a
// screen already laid out with another one, and holds it to the screen that
// was laid out with that tool from the start - every box where it would be,
// every section as tall as what it holds.
//
// The owner's window of 2026-10-05: after a change of tool the section of
// boxes kept the height of the tool before. Algorithm stood under its section
// with its list hidden by the next one, and a tool with fewer boxes left an
// empty band - the toolkit did not lay the screen out again by itself
// (Tools.relay).
func TestChoosingAToolLaysTheScreenOutAsIfItOpenedWithIt(t *testing.T) {
	ids := tool.Names()
	if len(ids) < 2 {
		t.Fatalf("%d tools, so there is no change of tool to ask about", len(ids))
	}
	fresh := map[string]string{}
	for _, id := range ids {
		_, fresh[id] = renderScene(t, screenScene{tab: text.TabTools(), set: chooseToolIn(id)})
	}
	for _, from := range ids {
		for _, to := range ids {
			if from == to {
				continue
			}
			// Asserted rather than assumed: two tools drawing one screen would
			// make this agree with itself whatever the layout did.
			if fresh[from] == fresh[to] {
				t.Fatalf("%s and %s draw the same screen, so choosing one after the other proves nothing", from, to)
			}
			_, switched := renderScene(t, screenScene{tab: text.TabTools(), set: chooseToolIn(from), after: chooseToolIn(to)})
			if switched != fresh[to] {
				diff, n := differingLines(fresh[to], switched, 8)
				t.Errorf("%s chosen after %s is drawn unlike the screen laid out with it - %d lines differ:\n%s", to, from, n, diff)
			}
		}
	}
}

// TestTheMenuOfToolsNamesEachToolAsTheCommandLineDoes holds every line of the
// menu of tools to starting with the name the tool has after tfg tool, then
// its question. Choosing among three questions, the owner could not tell that
// all three were about checksums (2026-10-05).
func TestTheMenuOfToolsNamesEachToolAsTheCommandLineDoes(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	menu := chooserUnder(t, selectTab(t, host.content, text.TabTools()), text.FieldTool())
	all := tool.All()
	if len(all) == 0 || len(menu.Options) != len(all) {
		t.Fatalf("%d tools and %d lines in the menu, so the lines cannot be held to the tools", len(all), len(menu.Options))
	}
	for i, d := range all {
		if line := menu.Options[i]; !strings.HasPrefix(line, d.ID+" ") || !strings.Contains(line, d.Question) {
			t.Errorf("the menu line of %s is %q - it should start with the name tfg tool knows it by, then ask its question", d.ID, line)
		}
	}
}

// runOnToolsTab chooses a tool on the Tools tab, fills its boxes by their
// names, presses Run and waits for the answer.
func runOnToolsTab(t *testing.T, host *fakeHost, screen fyne.CanvasObject, id string, boxes map[string]string) {
	t.Helper()
	d, err := tool.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	chooserUnder(t, screen, text.FieldTool()).SetSelected(text.ToolChoice(d.ID, d.Question))
	for name, value := range boxes {
		fillField(t, screen, text.SettingLabel(name), value)
	}
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
}

// TestCopyAllPutsTheWholeResultOnTheClipboard presses Copy all under two
// results: every checksum of one file, which was five presses of Copy, and a
// check whose lines had no Copy at all (the owner's list, 2026-10-05). The
// second lists more files than the screen draws, and the clipboard has to
// hold every one of them - a list somebody copies is one they want whole.
func TestCopyAllPutsTheWholeResultOnTheClipboard(t *testing.T) {
	dir := folderOf(t, map[string]string{"abc.txt": "abc"})
	host := newFakeHost(t)
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())

	runOnToolsTab(t, host, screen, checksum.ID, map[string]string{
		checksum.InputFile: filepath.Join(dir, "abc.txt"), checksum.SettingAlgorithm: "all"})
	pressNamed(t, screen, text.ButtonCopyAll())
	held := 0
	for _, k := range knownChecksums {
		if k.content != "abc" {
			continue
		}
		held++
		if want := k.algorithm + "  " + k.sum; !strings.Contains(host.copied, want) {
			t.Errorf("Copy all did not put %q on the clipboard. It put:\n%s", want, host.copied)
		}
	}
	if held < 4 {
		t.Fatalf("only %d known checksums of abc, so this asked about too few rows", held)
	}
	if words := allText(host.content); !strings.Contains(words, text.ToolCopiedAll()) {
		t.Errorf("Copy all was pressed and the window does not say what it did. It says:\n%s", words)
	}

	var body strings.Builder
	listed := parts.NoteItemsShown + 5
	for i := 1; i <= listed; i++ {
		body.WriteString(fmt.Sprintf("%s  missing-%02d.txt\n", abcSHA256, i))
	}
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	runOnToolsTab(t, host, screen, checksum.CheckID, map[string]string{checksum.InputChecksumFile: sums})
	if words := allText(screen); strings.Contains(words, fmt.Sprintf("missing-%02d.txt", listed)) {
		t.Fatalf("the screen draws all %d missing files, so it does not show what this guard asks about", listed)
	}
	pressNamed(t, screen, text.ButtonCopyAll())
	for i := 1; i <= listed; i++ {
		if name := fmt.Sprintf("missing-%02d.txt", i); !strings.Contains(host.copied, name) {
			t.Errorf("Copy all left %s off the clipboard - the screen cuts the list short, the clipboard must not", name)
		}
	}
	if want := text.ToolListDoesNotMatch("SHA256SUMS"); !strings.Contains(host.copied, want) {
		t.Errorf("Copy all left the verdict %q off the clipboard. It put:\n%s", want, host.copied)
	}
}

// TestOpenFolderLeadsToTheFolderTheToolWorkedIn runs the tools and presses
// Open folder after each: the folder checksum-write was given, the folder of
// the file checksum was given. Nothing is offered before a run, after another
// tool is chosen, or after a run that was refused.
func TestOpenFolderLeadsToTheFolderTheToolWorkedIn(t *testing.T) {
	dir := folderOf(t, map[string]string{"a.txt": "abc"})
	host := newFakeHost(t)
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	if visibleButtonNamed(screen, text.ButtonOpenFolder()) != nil {
		t.Fatal("Open folder is offered before anything has run")
	}
	opens := func(how, want string) {
		t.Helper()
		open := visibleButtonNamed(screen, text.ButtonOpenFolder())
		if open == nil {
			t.Fatalf("%s and Open folder is not offered. The screen says:\n%s", how, allText(screen))
		}
		open.OnTapped()
		if host.folder != want {
			t.Errorf("%s and Open folder showed %q, not %q", how, host.folder, want)
		}
	}

	runOnToolsTab(t, host, screen, checksum.WriteID, map[string]string{checksum.InputFolder: dir})
	opens("checksum-write wrote into a folder", dir)
	runOnToolsTab(t, host, screen, checksum.ID, map[string]string{checksum.InputFile: filepath.Join(dir, "a.txt")})
	opens("checksum read a file", dir)

	check, _ := tool.Get(checksum.CheckID)
	chooserUnder(t, screen, text.FieldTool()).SetSelected(text.ToolChoice(check.ID, check.Question))
	if visibleButtonNamed(screen, text.ButtonOpenFolder()) != nil {
		t.Error("another tool was chosen and Open folder still leads to where the last one worked")
	}
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	if visibleButtonNamed(screen, text.ButtonOpenFolder()) != nil {
		t.Error("a run refused for want of a checksum file offers Open folder")
	}
}
