package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// What this defends. A control you press to open a list is not drawn as a
// control you click into and type.
//
// Why it needed a guard. Measured off the stored trees on 2026-08-25, which is
// what named the defect: the format menu was 808x25 filled with
// inputBackground at #38383D, and the box for a size on the same screen was
// filled with inputBackground at #38383D. The same colour to the byte, the same
// corner radius, the same width. On the preset screen a box to type a spread
// into and a menu of file kinds sat one directly above the other, and the only
// thing telling them apart was a 20 px arrow at x=782 - 746 px from the value
// it belonged to.
//
// Why width and not colour. The surfaces of this window are page 11.3, panel
// 17.2, field 23.7 and open list 30.8 L*, four levels inside 19.6, and the
// button colour sits 1.84 L* off the panel it would be drawn on. That is under
// the 10 this project holds itself to, so a menu painted as a button would have
// disappeared instead. There is nowhere to put a fifth surface - counted on
// 2026-08-24 - which is why this is a question about shape.
//
// How it is asked. Against the boxes on the SAME screen rather than against a
// number written here, because the claim is a relationship: whatever the column
// is that day, a menu is not the width of it. A menu that goes back to taking
// the column is a menu as wide as the widest box beside it, and that is the
// state this refuses.
func TestAMenuIsNotDrawnAsWideAsABoxToTypeIn(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	checked := 0
	for _, tab := range allTabs() {
		screen := selectTab(t, content, tab)
		menus, boxes := menusOn(screen), typingBoxesOn(screen)
		if len(menus) == 0 {
			continue
		}
		widest := float32(0)
		for _, box := range boxes {
			if w := box.Size().Width; w > widest {
				widest = w
			}
		}
		if widest == 0 {
			t.Fatalf("the %s screen has %d menus and no box to type in that was laid out,"+
				" so there is nothing to compare them against", tab, len(menus))
		}
		for _, menu := range menus {
			got := menu.Size().Width
			if got == 0 {
				t.Fatalf("a menu on the %s screen was never laid out, so its width says nothing", tab)
			}
			if got >= widest {
				t.Errorf("a menu of %v on the %s screen is %.0f px, and the widest box to type in"+
					" beside it is %.0f. At that width the two are one shape and the only thing"+
					" saying which is which is the arrow at the far end.\n"+
					"What to do: parts.Menu holds a menu to what its values need.",
					menu.Options, tab, got, widest)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no screen has a menu, so this guard checked nothing")
	}
	t.Logf("%d menus, none of them the width of the boxes beside them", checked)
}

// And the width it is held to still shows every value it has.
//
// This is the other half and it is not decoration: the first cut of the narrow
// menu on 2026-08-25 sized the box for a CLOSED menu, and the list it opens
// puts a tick column and a picture in front of the word. The format box came
// out at 119 px and handed the longest format id a slot of 55 px for 67 px of
// word - twelve pixels of the name cut off, in the list that exists to show it.
//
// Asked of the slot the row was actually given against the width the toolkit
// says each value needs, so it repeats neither parts.menuWidth nor
// parts.RowWidthFor. A guard that recomputed the production arithmetic would
// agree with a wrong answer.
func TestEveryValueInAMenuFitsInTheListItOpens(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	checked := 0
	tightest, tightestIn := float32(0), float32(0)
	for _, tab := range allTabs() {
		screen := selectTab(t, content, tab)
		for _, menu := range menusOn(screen) {
			if len(menu.Options) == 0 {
				continue
			}
			menu.Tapped(&fyne.PointEvent{})
			list := menu.Opened()
			if list == nil {
				t.Fatalf("pressing a menu of %v on the %s screen opened no list", menu.Options, tab)
			}
			slot := wordSlotOf(t, list, menu.Options)
			for _, option := range menu.Options {
				needs := fyne.MeasureText(option, theme.TextSize(), fyne.TextStyle{}).Width
				if needs > tightest {
					tightest, tightestIn = needs, slot
				}
				if needs > slot {
					t.Errorf("%q in the menu on the %s screen needs %.0f px and its row gives it %.0f,"+
						" so it is cut off in the list that exists to show it.\n"+
						"What to do: parts.menuWidth takes the wider of what the closed box needs"+
						" and what a row of the open list needs - see parts.RowWidthFor.",
						option, tab, needs, slot)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no menu opened a list, so this guard checked nothing")
	}
	t.Logf("%d menus opened, every value fits. The tightest is %.0f px of word in a %.0f px slot", checked, tightest, tightestIn)
}

// wordSlotOf is the room a row of this list gives its words.
//
// Read off a row that is being drawn rather than worked out, because what a
// person sees is the size the layout gave the text. Every row of one list is
// laid out to the same width, so one is enough - and it has to be one the list
// is showing, since a list with a ceiling draws only the rows under it.
func wordSlotOf(t *testing.T, list *parts.OpenList, options []string) float32 {
	t.Helper()
	for _, option := range options {
		row := list.RowShowing(option)
		if row == nil {
			continue
		}
		for _, drawn := range test.WidgetRenderer(row).Objects() {
			words, is := drawn.(*canvas.Text)
			if !is || words.Text != option {
				continue
			}
			if words.Size().Width == 0 {
				t.Fatalf("the row for %q was never laid out, so the room it gives its words says nothing", option)
			}
			return words.Size().Width
		}
	}
	t.Fatalf("the open list is drawing no row for any of %v, so there is no slot to measure", options)
	return 0
}

// menusOn is every menu of a screen, and typingBoxesOn every box to type in.
//
// Both by walking the tree rather than by asking the screen, so a menu added
// tomorrow is held to the same rule without anybody remembering to add it.
func menusOn(screen fyne.CanvasObject) []*parts.Chooser {
	var found []*parts.Chooser
	walk(screen, func(o fyne.CanvasObject) {
		if menu, is := o.(*parts.Chooser); is {
			found = append(found, menu)
		}
	})
	return found
}

func typingBoxesOn(screen fyne.CanvasObject) []*parts.Entry {
	var found []*parts.Entry
	walk(screen, func(o fyne.CanvasObject) {
		if box, is := o.(*parts.Entry); is {
			found = append(found, box)
		}
	})
	return found
}
