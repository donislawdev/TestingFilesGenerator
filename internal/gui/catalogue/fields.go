package catalogue

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// form is a set of fields the way a screen builds them.
func form() *parts.Fields { return parts.NewFields() }

// A field of the form: the name over the control, and under the control
// whatever the field has to say about its value.
func fields() Entry {
	sizeRow := func(s *parts.Fields) fyne.CanvasObject {
		e := parts.NewEntry()
		e.SetText("10mb")
		return s.Add("size", "Size", "10mb", parts.NoDetail, parts.Numeric(e))
	}
	return Entry{Name: "Fields", Covers: []string{"FieldSaying", "CellSaying", "FieldStack", "RequiredMark", "Table"}, States: []State{
		{"a row", func() fyne.CanvasObject { return sizeRow(form()) }},
		{"a row that has to be filled in", func() fyne.CanvasObject {
			s := form()
			s.Require("size")
			return sizeRow(s)
		}},
		{"a row the run refused", func() fyne.CanvasObject {
			s := form()
			row := sizeRow(s)
			s.Mark("size", errors.New("a size of 3 B is below the smallest png, which is 73 B"))
			return row
		}},
		{"a row frozen while a run goes", func() fyne.CanvasObject {
			s := form()
			row := sizeRow(s)
			s.Freeze(true)
			return row
		}},
		{"a switch under its name", func() fyne.CanvasObject {
			s := form()
			return s.AddToggle("label", "Label in each file", "", parts.NoDetail, parts.NewToggle(func(bool) {}))
		}},
		{"a table: the names once over the columns, cells under them", func() fyne.CanvasObject {
			// Two rows, because one row cannot show what the header is for:
			// until 2026-09-16 every cell drew its own name, and the second
			// row of a table repeated the first row's names a control's
			// height below them. The header is derived from the first row,
			// star and explanation included.
			s := form()
			s.Require("size-1", "size-2")
			table := s.Table()
			first := tableRow(table, "1", "report.txt", "10kb")
			return parts.Column(parts.GapTight,
				table.Header(first...), table.Row(first...), table.Row(tableRow(table, "2", "invoice.pdf", "2mb")...))
		}},
		{"a row of arbitrary cells", func() fyne.CanvasObject {
			s := form()
			return s.Table().Row(parts.Prose("Anything"), parts.NewEntry(), parts.NewButton(parts.Secondary, "Choose...", func() {}))
		}},
		{"a long name", func() fyne.CanvasObject {
			s := form()
			return s.Add("size", longText, "10mb", parts.NoDetail, parts.NewEntry())
		}},
	}}
}

// tableRow is one row of the catalogue's table: a name, a size that has to be
// filled in, and the button that would take the row away.
func tableRow(table *parts.Table, n, name, size string) []fyne.CanvasObject {
	e := parts.NewEntry()
	e.SetText(name)
	z := parts.NewEntry()
	z.SetText(size)
	return []fyne.CanvasObject{
		table.Cell("name-"+n, "Name", "", parts.NoDetail, e),
		table.Cell("size-"+n, "Size", "How big each file inside is.", parts.NoDetail, parts.Numeric(z)),
		parts.BesideFields(parts.NewButton(parts.Secondary, "Remove", func() {})),
	}
}

// A field drawn from a declaration - what a format's settings and a preset's
// parameters look like, one state per kind of setting the registry can declare.
func propertyField() Entry {
	declared := []struct {
		caption string
		p       format.Property
	}{
		{"a number with a range", format.Property{Name: "width", Kind: format.PropertyInt, Min: 1, Max: 20000, Unit: "px", Default: "800", Detail: "How wide the picture is."}},
		{"a choice from a closed set", format.Property{Name: "compression", Kind: format.PropertyChoice, Choices: []string{"deflate", "store"}, Default: "deflate", Detail: "How each file inside is packed."}},
		{"a yes or no", format.Property{Name: "bom", Kind: format.PropertyBool, Default: "false", Detail: "Whether the file starts with a byte order mark."}},
		{"a size", format.Property{Name: "member_size", Kind: format.PropertySize, Default: "1kb", Detail: "How big each file inside is."}},
		{"free text", format.Property{Name: "password", Kind: format.PropertyText, Shape: "text", Detail: "What the archive is locked with."}},
	}
	var states []State
	for _, d := range declared {
		d := d
		states = append(states, State{d.caption, func() fyne.CanvasObject {
			s := form()
			tips := parts.NewTips()
			_, objects := parts.DeclaredFields([]format.Property{d.p}, s, tips)
			return tips.Over(parts.FieldColumn(objects...))
		}})
	}
	return Entry{Name: "PropertyField", Covers: []string{"FromProperty", "DeclaredFields", "PropertyFields", "ShapedFor", "PropertyDetail"}, States: states}
}

// The count of bytes beside a size, which the row shows when the field is one
// of the ones that hold a size.
func byteCount() Entry {
	counted := func(size string) fyne.CanvasObject {
		s := form()
		s.InBytes("size")
		e := parts.NewEntry()
		e.SetText(size)
		return s.Add("size", "Size", "10mb", parts.NoDetail, parts.Numeric(e))
	}
	return Entry{Name: "ByteCount", States: []State{
		{"a size counted out", func() fyne.CanvasObject { return counted("10mb") }},
		{"nothing to count yet", func() fyne.CanvasObject { return counted("") }},
		{"a large size", func() fyne.CanvasObject { return counted("2gb") }},
	}}
}

// The small button beside a name that shows the field's longer explanation.
func tips() Entry {
	explained := func() (fyne.CanvasObject, *parts.DetailButton) {
		tips := parts.NewTips()
		detail := tips.Say("The exact size every file will have, to the byte. Units count in 1024s, so 10mb is 10 485 760 B.")
		s := form()
		row := s.Add("size", "Size", "10mb", detail, parts.Numeric(parts.NewEntry()))
		button, _ := find(row, func(o fyne.CanvasObject) bool {
			_, ok := o.(*parts.DetailButton)
			return ok
		}).(*parts.DetailButton)
		return tips.Over(row), button
	}
	return Entry{Name: "Tips", Covers: []string{"DetailButton"}, States: []State{
		{"the button beside a name", func() fyne.CanvasObject {
			o, _ := explained()
			return o
		}},
		{"the button under the pointer", func() fyne.CanvasObject {
			o, b := explained()
			if b != nil {
				b.MouseIn(&desktop.MouseEvent{})
			}
			return o
		}},
	}}
}

func errorArea() Entry {
	return Entry{Name: "ErrorArea", States: []State{
		{"silent", func() fyne.CanvasObject { return parts.NewErrorArea().Object() }},
		{"one line", func() fyne.CanvasObject {
			a := parts.NewErrorArea()
			a.Say("size 3 B is below the smallest png, which is 73 B")
			return a.Object()
		}},
		{"a long message", func() fyne.CanvasObject {
			a := parts.NewErrorArea()
			a.Say(longText + ". " + longText)
			return a.Object()
		}},
	}}
}

func progress() Entry {
	at := func(value float64) fyne.CanvasObject {
		p := parts.NewProgress()
		p.Max = 100
		p.SetValue(value)
		return p
	}
	return Entry{Name: "Progress", States: []State{
		{"nothing done yet", func() fyne.CanvasObject { return at(0) }},
		{"half done", func() fyne.CanvasObject { return at(50) }},
		{"done", func() fyne.CanvasObject { return at(100) }},
	}}
}

func folding() Entry {
	// A fold is built open, which the first render of this catalogue found
	// out: "open" drew exactly as "closed" did, because the closed one had
	// never been closed. Guarded now, and closed here on purpose.
	// The head row is one control since O221, so it has the pointer and the
	// keyboard states a control has - and FoldHead is covered here rather
	// than as an entry of its own, because it is never on a screen without
	// the fold it heads.
	return Entry{Name: "Folding", Covers: []string{"InnerFolding", "FoldHead"}, States: []State{
		{"open", func() fyne.CanvasObject {
			return parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside the fold")).Object()
		}},
		{"closed", func() fyne.CanvasObject {
			f := parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside the fold"))
			f.Set(false)
			return f.Object()
		}},
		{"closed, saying what it holds", func() fyne.CanvasObject {
			f := parts.NewFolding("Settings for png", nil, parts.Prose("inside the fold"))
			f.Say("width 800, height 600")
			f.Set(false)
			return f.Object()
		}},
		{"inner, open", func() fyne.CanvasObject {
			return parts.NewInnerFolding("Advanced", parts.Prose("inside the inner fold")).Object()
		}},
		{"inner, closed", func() fyne.CanvasObject {
			f := parts.NewInnerFolding("Advanced", parts.Prose("inside the inner fold"))
			f.Set(false)
			return f.Object()
		}},
		{"a long title", func() fyne.CanvasObject {
			return parts.NewFolding(longText, nil, parts.Prose("inside the fold")).Object()
		}},
		{"the head under the pointer", func() fyne.CanvasObject {
			f := parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside the fold"))
			f.Head().MouseIn(&desktop.MouseEvent{})
			return f.Object()
		}},
		{"the head holding the keyboard", func() fyne.CanvasObject {
			f := parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside the fold"))
			f.Head().FocusGained()
			return f.Object()
		}},
		{"shut, the head under the pointer", func() fyne.CanvasObject {
			f := parts.NewFolding("Settings for png", nil, parts.Prose("inside the fold"))
			f.Say("width 800, height 600")
			f.Set(false)
			f.Head().MouseIn(&desktop.MouseEvent{})
			return f.Object()
		}},
	}}
}

// find walks containers for the first object the predicate accepts. The rows
// a form builds are containers of containers, so this is all a builder needs
// to lay hands on the button inside one.
func find(o fyne.CanvasObject, accept func(fyne.CanvasObject) bool) fyne.CanvasObject {
	if accept(o) {
		return o
	}
	c, ok := o.(*fyne.Container)
	if !ok {
		return nil
	}
	for _, child := range c.Objects {
		if found := find(child, accept); found != nil {
			return found
		}
	}
	return nil
}
