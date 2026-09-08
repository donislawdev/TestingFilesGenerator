package guard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// HTML gained a shape on 2026-09-08: a whole page, or only the blocks that
// would sit inside one.
//
// The axis is structural on purpose. HTML is the weakest format in this project
// for checking, because the specification requires a parser to recover from
// almost anything - so "it parsed" carries close to no information, and an axis
// visible as the ABSENCE of named elements is worth more here than one visible
// only in the content.
//
// Encoding was the obvious next setting after XML and it is deliberately NOT
// here. The standard says the document encoding must be UTF-8 and the charset
// attribute must match "utf-8", so a UTF-16 page would break the specification,
// and nothing in this project could go red on one. See docs/OBSERVATIONS.md
// O191 and docs/HTML-STRUCTURE-2026-09-08.md.

const (
	shapeSetting  = "structure"
	shapeDocument = "document"
	shapeFragment = "fragment"
)

func shapeProps(shape string) map[string]string {
	return map[string]string{shapeSetting: shape}
}

// skeleton is what a whole page carries and a fragment must not.
var skeleton = []string{"<!doctype html>", "<html", "<head", "<body", "</html>"}

// TestAFragmentCarriesNoSkeletonAndAPageCarriesOne is the claim itself.
//
// Read from the bytes rather than through a parser, and that is the point: the
// tolerant reader beside this one accepts both shapes happily, so it cannot
// tell them apart. What separates them is which elements are THERE.
func TestAFragmentCarriesNoSkeletonAndAPageCarriesOne(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}

	for _, shape := range []string{shapeDocument, shapeFragment} {
		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: shapeProps(shape)})
		for _, size := range []int64{floor, floor + 400, 4096} {
			body := strings.ToLower(string(writeEncoded(t, "html", size, shapeProps(shape))))

			for _, part := range skeleton {
				has := strings.Contains(body, part)
				if shape == shapeDocument && !has {
					t.Errorf("a page of %d B is missing %q", size, part)
				}
				if shape == shapeFragment && has {
					t.Errorf("a fragment of %d B holds %q, which belongs to a whole page", size, part)
				}
			}
		}
	}
}

// TestEachHTMLShapeAnswersForItsOwnMinimum holds the arithmetic.
//
// The registry declares the DEFAULT shape's minimum, the way it does for the
// JSON layouts, and every other shape answers for its own. Without this a
// fragment would inherit a page's floor and 110 B of every file would be a
// skeleton that is not there.
func TestEachHTMLShapeAnswersForItsOwnMinimum(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}

	page := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: shapeProps(shapeDocument)})
	part := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: shapeProps(shapeFragment)})

	if page != d.MinBytes {
		t.Errorf("the registry declares %d B and a page starts at %d - the declared minimum is the default shape's",
			d.MinBytes, page)
	}
	if part >= page {
		t.Errorf("a fragment starts at %d B and a page at %d - a fragment carries less, so it has to start lower",
			part, page)
	}

	// "Lower" is not enough, and the mutation run said so rather than the
	// reading: a fragment that inherited a page's prologue still starts lower
	// than a page, so the check above stayed green on exactly the defect it
	// names. What has to hold is that the gap IS the skeleton.
	//
	// Derived from the two files rather than repeated from the generator's
	// constants, which would be the same number written twice and would agree
	// with itself however wrong it was.
	pageBody := writeEncoded(t, "html", page, shapeProps(shapeDocument))
	partBody := writeEncoded(t, "html", part, shapeProps(shapeFragment))

	stripped := pageBody
	if i := bytes.Index(stripped, []byte("<body>\n")); i >= 0 {
		stripped = stripped[i+len("<body>\n"):]
	}
	stripped = bytes.TrimSuffix(stripped, []byte("</body>\n</html>\n"))
	if !bytes.Equal(stripped, partBody) {
		t.Errorf("the smallest page without its skeleton is %q and the smallest fragment is %q - "+
			"a fragment is the blocks of a page and nothing else, so at the floor the two have to meet",
			stripped, partBody)
	}

	// Both floors are actually writable, and one below each is refused. A floor
	// the format announces and then turns down is the defect this pins.
	for _, c := range []struct {
		shape string
		floor int64
	}{{shapeDocument, page}, {shapeFragment, part}} {
		if n := int64(len(writeEncoded(t, "html", c.floor, shapeProps(c.shape)))); n != c.floor {
			t.Errorf("%s: asked for its own floor of %d B and got %d", c.shape, c.floor, n)
		}
		_, err := d.Generator.Plan(format.Request{
			Bytes: c.floor - 1, Seed: 7741, Label: true, Properties: shapeProps(c.shape)})
		var below *format.BelowMinimumError
		if !errors.As(err, &below) {
			t.Errorf("%s: one byte under its floor was answered with %v, not a BelowMinimumError", c.shape, err)
			continue
		}
		if below.Minimum != c.floor {
			t.Errorf("%s: refusing %d B named %d as its minimum, not %d", c.shape, c.floor-1, below.Minimum, c.floor)
		}
	}
}

// TestTheDefaultHTMLShapeIsTheBytesItAlwaysWrote is the way back.
//
// A setting whose default changes the file is a breaking change wearing the
// clothes of a feature. Saying nothing and saying "document" out loud are two
// different routes through the parser and have to meet.
func TestTheDefaultHTMLShapeIsTheBytesItAlwaysWrote(t *testing.T) {
	for _, size := range []int64{118, 119, 400, 4096} {
		silent := writeEncoded(t, "html", size, nil)
		spoken := writeEncoded(t, "html", size, shapeProps(shapeDocument))
		if !bytes.Equal(silent, spoken) {
			t.Errorf("at %d B: saying nothing and saying %s produce different bytes", size, shapeDocument)
		}
	}
}

// TestTheLabelIsVisibleInBothHTMLShapes holds the label across the change.
//
// A page carries it twice, in the title for the tab and the heading for the
// reader. A fragment has no head to put a title in, so it carries the heading
// alone - and the format still declares its label VISIBLE, which stays true
// only because a heading is.
func TestTheLabelIsVisibleInBothHTMLShapes(t *testing.T) {
	for _, shape := range []string{shapeDocument, shapeFragment} {
		body := string(writeEncoded(t, "html", 4096, shapeProps(shape)))

		if !strings.Contains(body, "<h1>") {
			t.Errorf("%s: there is no heading, so the label is not visible", shape)
		}
		hasTitle := strings.Contains(body, "<title>tfg")
		if shape == shapeDocument && !hasTitle {
			t.Error("a page carries the label in its title as well, and this one does not")
		}
		if shape == shapeFragment && hasTitle {
			t.Error("a fragment has no head, so a title in it belongs to a page")
		}
	}
}

// TestTheStructuralCheckIsToldWhichHTMLShapeToExpect is the canary.
//
// Every other guard here hands the checker a file of the shape it asked for, so
// all of them would stay green if it stopped looking at the shape at all. The
// two defects this setting can cause are a fragment where a page was ordered
// and a page where a fragment was, and only handing it the wrong one proves
// they would be caught.
func TestTheStructuralCheckIsToldWhichHTMLShapeToExpect(t *testing.T) {
	dir := t.TempDir()
	written := map[string]string{}
	for _, shape := range []string{shapeDocument, shapeFragment} {
		path := filepath.Join(dir, shape+".html")
		if err := os.WriteFile(path, writeEncoded(t, "html", 4096, shapeProps(shape)), 0o600); err != nil {
			t.Fatal(err)
		}
		written[shape] = path
	}

	// The controls first. A check that refused everything would pass the half
	// below without seeing anything at all.
	for _, shape := range []string{shapeDocument, shapeFragment} {
		res := oracle.Strict("html", written[shape], shapeSetting+"="+shape)
		if !res.Available {
			t.Skip("the structural check needs python")
		}
		if res.Err != nil {
			t.Fatalf("%s handed to the check as %s was refused: %v", shape, shape, res.Err)
		}
	}

	for _, c := range []struct{ file, told string }{
		{shapeDocument, shapeFragment},
		{shapeFragment, shapeDocument},
	} {
		if res := oracle.Strict("html", written[c.file], shapeSetting+"="+c.told); res.Err == nil {
			t.Errorf("a %s handed to the check as a %s was called correct, so nothing here can see the shape",
				c.file, c.told)
		}
	}
}
