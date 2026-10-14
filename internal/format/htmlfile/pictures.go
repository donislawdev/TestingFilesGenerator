package htmlfile

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// Pictures inside the page: real files of a picture format, each in a data:
// URL (RFC 2397) on a line of its own, the way a page saved as one file
// carries them. The settings are the ones a mail message has, with the same
// names and the same meaning (package picture).
//
// Measured on 2026-10-14 on pages from this program (docs/HTML-3-2026-10-14.md
// section 9), and this is what the details below say: Chromium shows every
// picture format but tiff, the Nu HTML Checker calls a line break inside src an
// error - so the base64 is one line - and lxml keeps no attribute past ten
// million characters unless told to read huge trees.

// picturesGroup is the block the picture settings sit in, on both surfaces.
const picturesGroup = "Pictures in the page"

// maxPage is the largest page this format plans with pictures in it. Far above
// anything a disk holds, and far enough below the int64 every size is counted
// in that adding the pictures up cannot wrap round - eml's maxMessage, for its
// reason.
const maxPage = int64(1) << 60

// pictureClose ends the line a picture sits on.
const pictureClose = `"></p>` + "\n"

// pictureProperties are the settings of the pictures the page shows.
func pictureProperties() []format.Property {
	return picture.Properties(picturesGroup, picture.Details{
		Count: "How many pictures the page shows, each one a real file of its format written into the page as a data: URL, so opening the page fetches nothing. " +
			"They sit above the blocks, one to a line, and each says which it is in its alt text.",
		Format: "The format of the pictures. Any format whose files are pictures, such as png, jpg or gif. Chromium shows every one of them but tiff.",
		Size: "How big each picture is. Inside the page it is base64 on one line, about a third larger. " +
			"lxml, and BeautifulSoup on top of it, leave out a picture past about 7.1 MB unless told to read huge trees. " +
			"On Linux lxml took over 20 seconds over such a page, and with a 15 MB picture it lost the rest of the page as well.",
	})
}

// pictureLine is one picture, planned: the file, and the line it sits on up
// to where the base64 starts.
type pictureLine struct {
	desc format.Descriptor
	plan format.Plan
	open string
}

// bytes is what the line takes in the page.
func (p pictureLine) bytes() int64 {
	return int64(len(p.open)) + base64Length(p.plan.Bytes) + int64(len(pictureClose))
}

// planPictures plans the pictures, each with the seed of its place (RC4), so
// asking for one more changes none of the others. The format is asked whether
// it is a picture even when none are asked for: a value the setting cannot
// take is a mistake either way, and eml refuses it the same way.
func planPictures(r format.Request, s picture.Settings) ([]pictureLine, int64, error) {
	desc, ok, err := picture.Of(s.Format)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, &format.PropertyValueError{
			Format: "html", Key: picture.Format, Value: s.Format,
			Reason: core.Says("html.APictureInThePageIs", "a picture in the page has to be a file of a picture format"),
			Remedy: core.Says("html.UseOneOfThePictureFormats", "Use one of: %s.", core.A("Formats", strings.Join(picture.Formats(), ", "))),
		}
	}
	out := make([]pictureLine, 0, s.Count)
	var sum int64
	for i := 0; i < s.Count; i++ {
		cp, err := desc.Generator.Plan(format.Request{Bytes: s.Size, Seed: core.FileSeed(r.Seed, i), Label: r.Label})
		if err != nil {
			return nil, 0, core.Refuse(core.Says("html.ThePictureCannotBe", "html: the %s picture cannot be made: %w",
				core.A("Format", desc.ID), core.A("Err", err)))
		}
		if cp.Bytes > maxPage {
			return nil, 0, pageTooLarge(cp.Bytes)
		}
		line := pictureLine{desc: desc, plan: cp, open: pictureOpen(desc, i+1)}
		if sum += line.bytes(); sum > maxPage {
			return nil, 0, pageTooLarge(sum)
		}
		out = append(out, line)
	}
	return out, sum, nil
}

// pictureOpen is a picture's line up to its base64. The alt text is the name
// the picture has in a mail message, inline_0001.png, so a tester reading
// either finds the same picture by the same name.
func pictureOpen(desc format.Descriptor, n int) string {
	name := fmt.Sprintf("inline_%04d%s", n, desc.Extension)
	return `<p><img alt="` + name + `" src="data:` + desc.MediaType + `;base64,`
}

// pageTooLarge refuses pictures that add up past maxPage.
func pageTooLarge(asked int64) error {
	return &format.AboveMaximumError{
		Format:    "HTML",
		Requested: asked,
		Maximum:   maxPage,
		Reason:    core.Says("html.APageThisLargeCannotBe", "a page this large cannot be counted to the byte in this build"),
		Hint:      core.Says("html.AskForSmallerPictures", "Ask for fewer or smaller pictures."),
	}
}

// base64Length is how many characters n bytes take in base64 on one line.
func base64Length(n int64) int64 { return (n + 2) / 3 * 4 }

// describePictures adds the pictures to what reaches the manifest, in the words
// eml uses for its own.
func describePictures(props map[string]any, s picture.Settings, lines []pictureLine) {
	var raw, encoded int64
	for _, p := range lines {
		raw += p.plan.Bytes
		encoded += base64Length(p.plan.Bytes)
	}
	picture.Describe(props, s, len(lines), raw, encoded)
}

// writePictures writes every picture's line, each file made by its own
// generator as it is written, so a picture of a hundred megabytes never sits in
// memory.
func writePictures(ctx context.Context, w io.Writer, lines []pictureLine) error {
	for _, p := range lines {
		if err := core.WriteAll(w, []byte(p.open)); err != nil {
			return err
		}
		c := counted{w: w}
		enc := base64.NewEncoder(base64.StdEncoding, &c)
		if err := p.desc.Generator.Write(ctx, enc, p.plan); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		// The size of the whole page rests on each picture coming out as its
		// plan says, so one that does not is a defect, not a page of the
		// wrong size.
		if want := base64Length(p.plan.Bytes); c.n != want {
			return core.Defect(fmt.Errorf("html: the %s picture came out as %d B of base64 where its plan gives %d B",
				p.desc.ID, c.n, want))
		}
		if err := core.WriteAll(w, []byte(pictureClose)); err != nil {
			return err
		}
	}
	return nil
}

// counted is a writer that keeps count.
type counted struct {
	w io.Writer
	n int64
}

func (c *counted) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
