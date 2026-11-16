package guard

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// What this defends. Every button under a refusal of a size puts in the box a
// value the next run accepts, for every file of the batch.
//
// Why it matters to somebody using the tool. Reported by the owner from the
// running window on 2026-11-16 (docs/SIZE-FIXES-2026-11-16.md): a JPG with its
// width and height set, asked for 1 B, offered "Use the smallest size", and
// pressing it and generating was refused again. Measured that day in ten
// formats of thirty two. A button that promises the smallest size and leads to
// another refusal is worse than none, and the owner's words were that this must
// not happen again.
//
// How. Every registered format, at its defaults, at 160x120 where it declares a
// width and a height, and drawing a placeholder where it offers one - each as
// one file and as three, because the files of a batch are not alike and the
// first one to refuse is not the one that needs the most. Each is planned at
// 1 B. The values the buttons would put in the box come from parts.Remedies,
// the function the window's buttons are made from, and each value is planned
// again and has to be accepted. A picture with its width and height set has to
// be offered smallest as well (section 3.3).
func TestEveryFixUnderASizeRefusalLeadsToARunThatIsAccepted(t *testing.T) {
	theFirstFilesMinimumIsCaught(t)
	cases := fixCases(t)
	refused, offeredSmallest := 0, 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := planFixCase(t, c, "1")
			if err == nil {
				// 1 B is a size this format makes, so there is nothing to put
				// right. txt is such a format.
				return
			}
			remedies := parts.Remedies(err, "1")
			if len(remedies) == 0 {
				t.Fatalf("1 B was refused and no button is offered:\n%v", err)
			}
			refused++
			smallest := slices.ContainsFunc(remedies, func(r parts.Remedy) bool { return r.Value == core.SizeSmallest })
			if smallest {
				offeredSmallest++
			}
			if c.wantsSmallest && !smallest {
				t.Errorf("a picture with its width and height set is offered %v, without smallest", remedies)
			}
			for _, r := range remedies {
				if err := planFixCase(t, c, r.Value); err != nil {
					t.Errorf("the button %q puts %q in the box, and the run is refused again:\n%v", r.Label, r.Value, err)
				}
			}
		})
	}
	// Asserted rather than assumed: a registry where every format took 1 B, or
	// an engine that stopped marking a picture, would leave this green while
	// pressing nothing.
	if refused == 0 || offeredSmallest == 0 {
		t.Fatalf("of %d cases %d were refused at 1 B and %d offered smallest - this guard pressed nothing it exists to press",
			len(cases), refused, offeredSmallest)
	}
}

// theFirstFilesMinimumIsCaught plants the button as it was before 2026-11-16 -
// the minimum of the first file of the batch, which is what a target of one
// file names - and asks that the check above refuses it for at least one
// batch of three pictures. Without that, the check could be accepting
// everything.
func theFirstFilesMinimumIsCaught(t *testing.T) {
	t.Helper()
	for _, c := range fixCases(t) {
		if c.count != 3 || !c.wantsSmallest {
			continue
		}
		one := c
		one.count = 1
		remedies := parts.Remedies(planFixCase(t, one, "1"), "1")
		if len(remedies) == 0 {
			continue
		}
		if planFixCase(t, c, remedies[0].Value) != nil {
			return
		}
	}
	t.Fatal("canary: no batch of three pictures refuses the minimum of its first file, so the check above cannot tell the button that was reported from one that works")
}

// fixCase is one target a size refusal is asked about.
type fixCase struct {
	name    string
	format  string
	setting map[string]string
	count   int
	// wantsSmallest is a picture with its width and height set, which section
	// 3.3 promises a second button.
	wantsSmallest bool
}

// fixCases is every format at its defaults, at 160x120 where it declares a
// width and a height, and drawing a placeholder at 320x240 where it offers
// one, each as one file and as three. 160x120 because one format of them, ico,
// stops at 256 pixels.
func fixCases(t *testing.T) []fixCase {
	t.Helper()
	var out []fixCase
	for _, d := range format.All() {
		props := map[string]format.Property{}
		for _, p := range d.Properties {
			props[p.Name] = p
		}
		_, w := props["width"]
		_, h := props["height"]
		picture := strings.HasPrefix(d.MediaType, "image/")
		shapes := []fixCase{{name: d.ID}}
		if w && h {
			shapes = append(shapes, fixCase{name: d.ID + "-160x120", setting: map[string]string{"width": "160", "height": "120"}, wantsSmallest: picture})
		}
		if content, ok := props["content"]; ok && slices.Contains(content.Choices, "placeholder") {
			shapes = append(shapes, fixCase{name: d.ID + "-placeholder",
				setting: map[string]string{"width": "320", "height": "240", "content": "placeholder"}, wantsSmallest: picture})
		}
		for _, shape := range shapes {
			for _, n := range []int{1, 3} {
				c := shape
				c.name, c.format, c.count = fmt.Sprintf("%s-x%d", shape.name, n), d.ID, n
				out = append(out, c)
			}
		}
	}
	return out
}

// planFixCase plans c with the size box holding value, read the way the
// window reads it - smallest by name, anything else as a size - and answers
// the refusal, or nothing when the run is accepted.
func planFixCase(t *testing.T, c fixCase, value string) error {
	t.Helper()
	target := engine.Target{ID: "fix", Format: c.format, Label: true, Properties: c.setting}
	if value == core.SizeSmallest {
		target.Smallest, target.Sizes = true, engine.Uniform(c.count, 0)
	} else {
		bytes, err := core.ParseSize(value)
		if err != nil {
			t.Fatalf("the button put %q in the box, which is not a size: %v", value, err)
		}
		target.Sizes = engine.Uniform(c.count, bytes)
	}
	_, err := engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "guard", ManifestName: "manifest.json"})
	return err
}

// TestBothButtonsUnderABatchOfSmallPicturesAreAcceptedInTheWindow presses both
// buttons on the screen the owner reported from: three PNGs of 320x240 asked
// for 1 B. The function the buttons are made from is asked for every format
// above. This asks that the window shows both, in that order, and that what
// each puts in the box is accepted by the next preview.
func TestBothButtonsUnderABatchOfSmallPicturesAreAcceptedInTheWindow(t *testing.T) {
	screen := window.NewGenerate(newFakeHost(t))
	body := screen.Object()
	fields := screen.Fields()
	chooserIn(t, fields, "format").SetSelected("png")
	setBox(t, fields, "width", "320")
	setBox(t, fields, "height", "240")
	setBox(t, fields, engine.SettingCount, "3")

	refused := func() []string {
		t.Helper()
		setBox(t, fields, core.SettingSize, "1")
		pressNamed(t, body, text.ButtonPreview())
		screen.Settled()
		offered := fields.Lookup(core.SettingSize).Offered()
		if len(offered) != 2 || offered[1] != text.UseSmallestForEach() {
			t.Fatalf("three 320x240 PNGs at 1 B offer %q, where the smallest size and %q were due.\n%s",
				offered, text.UseSmallestForEach(), allSaid(fields))
		}
		return offered
	}
	// Refused afresh before each press, so the second button is pressed under
	// a refusal of its own rather than after the first one has put it right.
	for i := 0; i < 2; i++ {
		label := refused()[i]
		pressNamed(t, body, label)
		put := boxText(t, fields, core.SettingSize)
		pressNamed(t, body, text.ButtonPreview())
		screen.Settled()
		if said := saidBy(t, fields, core.SettingSize); said != "" {
			t.Errorf("the button %q put %q in the size box and the preview refused it: %q", label, put, said)
		}
	}
}
