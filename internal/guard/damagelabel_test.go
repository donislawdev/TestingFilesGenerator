package guard

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// labelNotGuaranteed is the note a damaged file carries in place of a claim
// to its label.
const labelNotGuaranteed = "label_not_guaranteed"

// TestADamagedFileNeverClaimsItsLabel holds the manifest to what it can know
// about a label after a damage.
//
// Measured 2026-11-09: zero-head on txt overwrote the label at the start of
// the text, and the manifest still said label_embedded: true. Where a label
// sits differs by format, so the rule is the same for all of them. A damaged
// file says false, in both places the manifest says it, and a note says why.
//
// Every format, each beside a healthy file of the same format and size. The
// healthy one is the control: a format that never claims its label would pass
// this guard without the rule being there, so the guard also counts the
// formats where the claim was really taken away.
func TestADamagedFileNeverClaimsItsLabel(t *testing.T) {
	dir := t.TempDir()
	chain := damage.Chain{{ID: damage.ZeroHead}}

	withdrawn := 0
	for _, f := range format.All() {
		out := filepath.Join(dir, f.ID)
		targets := []engine.Target{
			{ID: "good", Format: f.ID, Sizes: engine.Uniform(1, witnessSize), Label: true},
			{ID: "broken", Format: f.ID, Sizes: engine.Uniform(1, witnessSize), Damage: chain, Label: true},
		}
		opt := engine.Options{OutDir: out, Seed: 5, ManifestName: engine.DefaultManifestName}
		planned, err := engine.Plan(targets, opt)
		if err != nil {
			t.Fatalf("%s: planning: %v", f.ID, err)
		}
		res, err := engine.Run(t.Context(), planned, opt)
		if err != nil {
			t.Fatalf("%s: running: %v", f.ID, err)
		}
		entries := map[string]manifest.File{}
		for _, m := range res.Manifest.Files {
			entries[m.TargetID] = m
		}
		good, broken := entries["good"], entries["broken"]
		if good.Name == "" || broken.Name == "" {
			t.Fatalf("%s: the run did not write both files", f.ID)
		}

		if broken.LabelEmbedded {
			t.Errorf("%s: a damaged file claims label_embedded: true.\n"+
				"Reason: the damage came after the label was written and may have overwritten it,\n"+
				"so the manifest cannot know the label is still there.", f.ID)
		}
		if claim, ok := broken.Properties[format.PropertyLabelEmbedded].(bool); ok && claim {
			t.Errorf("%s: a damaged file claims label_embedded: true in its properties, "+
				"while the field beside them says false", f.ID)
		}
		noted := slices.ContainsFunc(broken.Notes, func(n manifest.Note) bool { return n.Code == labelNotGuaranteed })
		if noted != good.LabelEmbedded {
			t.Errorf("%s: the healthy file claims its label: %t, and the damaged one carries the note %s: %t.\n"+
				"Reason: the note says why a label that was written is no longer claimed, so it\n"+
				"belongs exactly where a claim was taken away.", f.ID, good.LabelEmbedded, labelNotGuaranteed, noted)
		}
		if good.LabelEmbedded {
			withdrawn++
		}
	}
	if withdrawn == 0 {
		t.Fatal("no healthy file claimed its label, so this guard took nothing away and proved nothing")
	}
	t.Logf("the claim was taken away in %d format(s)", withdrawn)
}

// TestADamagedFileStatesItsSizeOnDiskAndItsDamage holds the label of a damaged
// file to what is on disk.
//
// Measured 2026-11-11 before this held (O333): a text of 10 MiB cut with
// missing=50 said "20971520 B" in its first line - the size of the whole file
// before the cut - and named no damage, so a file cut by half and one cut by
// nine tenths looked the same from the inside. append-junk stated the size
// without the junk.
//
// Every format whose healthy file carries its label as text is asked, with
// junk appended: that damage keeps every byte the generator wrote, so the
// label is where the generator put it and can be read back. Which formats
// those are is read from the healthy file, not listed here - a picture draws
// its label in pixels and is left to the guards on pictures. The expected text
// is written out here rather than asked of format.Request, so a wording that
// drops the damage cannot agree with itself.
func TestADamagedFileStatesItsSizeOnDiskAndItsDamage(t *testing.T) {
	chain := damage.Chain{{ID: damage.AppendJunk}}
	const size = witnessSize
	before, err := chain.Before(size)
	if err != nil {
		t.Fatal(err)
	}
	brokenSeed := core.FileSeed(core.TargetSeed(lengthSeed, "broken"), 0)
	// The canary: the label this guard was written against, which states the
	// size the generator wrote and no damage, must not pass it.
	planted := fmt.Sprintf("tfg - txt - %d B - seed %s", before, core.SeedLabel(brokenSeed))
	if statesItsDamage([]byte(planted), "txt", size, chain, brokenSeed) == "" {
		t.Fatal("the check took a label naming the size before the damage, and no damage, for the label it asks for")
	}

	dir := t.TempDir()
	read := 0
	for _, f := range format.All() {
		good := engine.Target{ID: "good", Format: f.ID, Sizes: engine.Uniform(1, size), Label: true}
		broken := engine.Target{ID: "broken", Format: f.ID, Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
		_, paths := lengthRun(t, filepath.Join(dir, f.ID), []engine.Target{good, broken})
		healthy, err := os.ReadFile(paths["good"])
		if err != nil {
			t.Fatal(err)
		}
		goodSeed := core.FileSeed(core.TargetSeed(lengthSeed, "good"), 0)
		if !bytes.Contains(healthy, fmt.Appendf(nil, "tfg - %s - %d B - seed %s", f.ID, size, core.SeedLabel(goodSeed))) {
			continue
		}
		got, err := os.ReadFile(paths["broken"])
		if err != nil {
			t.Fatal(err)
		}
		if why := statesItsDamage(got, f.ID, size, chain, brokenSeed); why != "" {
			t.Errorf("%s: %s", f.ID, why)
		}
		read++
	}
	if read < 10 {
		t.Fatalf("only %d format(s) carry their label as text the guard could find - it did not reach the formats it is about", read)
	}
	t.Logf("the label was read back from %d format(s)", read)
}

// TestATruncatedTextSaysHowMuchWasCut is the case the owner met: two texts of
// the same size cut by different shares used to begin with the same line.
// Their labels are the first thing a person opening them sees, so the share
// has to be there.
func TestATruncatedTextSaysHowMuchWasCut(t *testing.T) {
	const size = 64 << 10
	seed := core.FileSeed(core.TargetSeed(lengthSeed, "cut"), 0)
	for _, missing := range []string{"50", "90"} {
		chain := damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: missing}}}
		target := engine.Target{ID: "cut", Format: "txt", Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
		_, paths := lengthRun(t, filepath.Join(t.TempDir(), missing), []engine.Target{target})
		got, err := os.ReadFile(paths["cut"])
		if err != nil {
			t.Fatal(err)
		}
		first, _, _ := bytes.Cut(got, []byte("\n"))
		want := fmt.Sprintf("tfg - txt - %d B - damage truncate:missing=%s - seed %s", size, missing, core.SeedLabel(seed))
		if string(first) != want {
			t.Errorf("a text cut with missing=%s begins %q, expected %q", missing, first, want)
		}
	}
}

// TestALongDamageLabelLeavesEveryFormatWhole holds every format to the longest
// damage a label can name.
//
// Nothing limits how many damages a chain lists, so the label is input a
// person controls, and a format whose label lives in a field with a limit of
// its own can break on it before any damage is applied: a gzip comment Go's
// reader refuses from 512 bytes, a ZIP comment past its padding, a line of a
// mail. Found in review on 2026-11-12 (docs/REVIEW-234-2026-11-12.md). The
// answer is damage.LabelLimit, and this asks every format at it: each
// generator directly, with the label alone and no damage, so the file has to
// come out whole - planned, written, taken by the structural check, and a
// gzip stream opened by Go's own reader, which the structural check in Python
// does not hold to that limit.
func TestALongDamageLabelLeavesEveryFormatWhole(t *testing.T) {
	long := longestDamageLabel(t)

	dir := t.TempDir()
	var reqs []oracle.StrictRequest
	for _, f := range format.All() {
		path := filepath.Join(dir, f.ID)
		if err := writeWithLabel(t, f, long, path); err != nil {
			t.Errorf("%s with a label of %d B: %v", f.ID, len(long), err)
			continue
		}
		reqs = append(reqs, oracle.StrictRequest{Format: f.ID, Path: path})
	}
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatal(err)
	}
	gzipped := 0
	for i, j := range got {
		if j.Verdict != oracle.StrictOK {
			t.Errorf("%s with a label of %d B is refused by the structural check: %s", reqs[i].Format, len(long), j.Reason)
		}
		if err := opensAsGzipWhereItIsOne(reqs[i].Path); err != nil {
			t.Errorf("%s with a label of %d B: Go's gzip reader refuses it: %v", reqs[i].Format, len(long), err)
		} else if isGzip(reqs[i].Path) {
			gzipped++
		}
	}
	if len(got) < len(format.All())*3/4 || gzipped == 0 {
		t.Fatalf("only %d of %d formats were judged, %d of them gzip streams", len(got), len(format.All()), gzipped)
	}
}

// longestDamageLabel is the damage text of a chain far longer than the label
// takes, after asserting that the label was shortened to what it takes and
// that the whole text would have passed the gzip limit - so the guard above is
// asking about the limit rather than about a label that never reached it.
func longestDamageLabel(t *testing.T) string {
	t.Helper()
	chain := damage.Chain{}
	for range 40 {
		chain = append(chain, damage.Spec{ID: damage.AppendJunk, Values: damage.Values{damage.SettingBytes: "65536"}})
	}
	long := chain.Label()
	whole := len(chain) * len(chain[0].String())
	if whole < 512 || len(long) > damage.LabelLimit || len(long) < damage.LabelLimit*3/4 {
		t.Fatalf("forty damages name %d B whole and %d B in the label, with the label held to %d B - the guard does not reach the limit it is about",
			whole, len(long), damage.LabelLimit)
	}
	return long
}

// isGzip says whether the file at path starts as a gzip stream.
func isGzip(path string) bool {
	head := make([]byte, 2)
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	n, _ := f.Read(head)
	return n == 2 && head[0] == 0x1f && head[1] == 0x8b
}

// opensAsGzipWhereItIsOne opens the header of a gzip stream with Go's reader,
// and says nothing about a file that is not one.
func opensAsGzipWhereItIsOne(path string) error {
	if !isGzip(path) {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	return z.Close()
}

// TestADamageLabelIsBoundedAndSaysHowManyItLeftOut holds damage.Chain.Label
// to its limit for every length of chain, and to an honest count of what it
// did not name: the entries it names and the number it gives add up to the
// chain. The canary is the shortest chain over the limit, which has to be
// shortened, so a limit nothing reaches cannot pass.
func TestADamageLabelIsBoundedAndSaysHowManyItLeftOut(t *testing.T) {
	entry := damage.Spec{ID: damage.AppendJunk, Values: damage.Values{damage.SettingBytes: "65536"}}
	shortened := 0
	for n := 1; n <= 200; n++ {
		chain := slices.Repeat(damage.Chain{entry}, n)
		label := chain.Label()
		if len(label) > damage.LabelLimit {
			t.Fatalf("%d damages: the label is %d B, past the %d B it is held to", n, len(label), damage.LabelLimit)
		}
		named := strings.Count(label, damage.AppendJunk)
		left := 0
		if before, more, cut := strings.Cut(label, " and "); cut {
			left, _ = strconv.Atoi(strings.TrimSuffix(more, " more"))
			named = strings.Count(before, damage.AppendJunk)
			shortened++
		}
		if named+left != n || named == 0 {
			t.Errorf("%d damages: the label %q names %d and says %d more", n, label, named, left)
		}
	}
	if shortened == 0 {
		t.Fatal("no chain up to 200 damages was shortened, so the limit was never reached")
	}
}

// TestADamagedMailCarriesItsLabelWholeInEveryTextEncoding reads the label of
// a mail back the way a mail reader does, in every encoding of its text and
// both parts that carry it.
//
// Found 2026-11-12 while answering a review: the label was written as it is
// into quoted-printable, where its = starts an escape, so
// append-junk:bytes=64 read back as bytesd - and the structural check took it,
// because =64 is a valid escape. A label naming two damages is also longer
// than the 78 characters RFC 5322 asks a line to keep to. The encodings and
// parts are read from the declaration, and the label is decoded with Go's own
// readers rather than searched for in the bytes.
func TestADamagedMailCarriesItsLabelWholeInEveryTextEncoding(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	encodings, eok := settingNamed(d, "text_encoding")
	bodies, bok := settingNamed(d, "body")
	if !eok || !bok || len(encodings.Choices) < 4 {
		t.Fatalf("eml declares text_encoding %v and body %v - the guard does not reach the encodings it is about", encodings.Choices, bodies.Choices)
	}
	// One damage, whose label fits a line, so the = is asked on its own. And
	// two, whose label does not, so the breaking is asked as well.
	one := damage.Chain{{ID: damage.AppendJunk, Values: damage.Values{damage.SettingBytes: "1024"}}}
	two := damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "10"}}, one[0]}
	for _, chain := range []damage.Chain{one, two} {
		for _, encoding := range encodings.Choices {
			for _, body := range []string{"plain", "html"} {
				mailCarriesItsLabel(t, d, chain, encoding, body)
			}
		}
	}
	if !fitsALine(d, one) || fitsALine(d, two) {
		t.Fatal("the label of one damage has to fit a line and the label of two must not, or one half of this guard asks nothing")
	}
}

// fitsALine says whether the label of a 64 KiB mail damaged by chain fits the
// 78 characters a line keeps to.
func fitsALine(d format.Descriptor, chain damage.Chain) bool {
	return len(format.Request{Bytes: 64 << 10, Seed: 3, Damage: chain.Label()}.LabelText(d.ID)) <= 78
}

// mailCarriesItsLabel writes one mail damaged by chain in one encoding and
// part, and reads its label back.
func mailCarriesItsLabel(t *testing.T, d format.Descriptor, chain damage.Chain, encoding, body string) {
	t.Helper()
	r := format.Request{Seed: 3, Label: true, Damage: chain.Label(),
		Properties: map[string]string{"text_encoding": encoding, "body": body}}
	r.Bytes = max(int64(64<<10), d.SmallestAccepted(r))
	want := fmt.Sprintf("tfg - eml - %d B - damage %s - seed %s", r.Bytes, chain.Label(), core.SeedLabel(r.Seed))
	text, err := mailText(t, d, r)
	if err != nil {
		t.Errorf("%s, text_encoding=%s body=%s: %v", chain.Label(), encoding, body, err)
		return
	}
	if !strings.Contains(text, want) {
		t.Errorf("%s, text_encoding=%s body=%s: the text read back does not carry %q", chain.Label(), encoding, body, want)
	}
}

// mailText writes the mail r asks for and reads its text back: the body
// decoded by its Content-Transfer-Encoding, every line break read as the space
// it was broken at. A line longer than 78 characters in the file is an error.
func mailText(t *testing.T, d format.Descriptor, r format.Request) (string, error) {
	t.Helper()
	p, err := d.Generator.Plan(r)
	if err != nil {
		return "", err
	}
	var file bytes.Buffer
	if err := d.Generator.Write(t.Context(), &file, p); err != nil {
		return "", err
	}
	for i, line := range strings.Split(file.String(), "\n") {
		if n := utf8.RuneCountInString(strings.TrimSuffix(line, "\r")); n > 78 {
			return "", fmt.Errorf("line %d of the file is %d characters, past the 78 RFC 5322 asks for", i+1, n)
		}
	}
	msg, err := mail.ReadMessage(&file)
	if err != nil {
		return "", err
	}
	var body io.Reader = msg.Body
	switch msg.Header.Get("Content-Transfer-Encoding") {
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, newlineless{body})
	}
	text, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer("\r\n", " ", "\n", " ").Replace(string(text)), nil
}

// newlineless is a reader with its line breaks taken out, which base64 in a
// mail carries every 76 characters and the decoder does not skip.
type newlineless struct{ r io.Reader }

func (n newlineless) Read(p []byte) (int, error) {
	k, err := n.r.Read(p)
	out := p[:0]
	for _, c := range p[:k] {
		if c != '\r' && c != '\n' {
			out = append(out, c)
		}
	}
	return len(out), err
}

// writeWithLabel plans and writes one file of f carrying the damage text long
// in its label, and turns a panic into the error it is.
func writeWithLabel(t *testing.T, f format.Descriptor, long, path string) (err error) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panicked: %v", p)
		}
	}()
	r := format.Request{Seed: 1, Label: true, Damage: long}
	r.Bytes = max(int64(witnessSize), f.SmallestAccepted(r))
	p, err := f.Generator.Plan(r)
	if err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return f.Generator.Write(t.Context(), out, p)
}

// statesItsDamage says what is wrong with the label in got, or nothing when
// it states size on disk and every entry of chain with its settings resolved.
func statesItsDamage(got []byte, id string, size int64, chain damage.Chain, seed uint64) string {
	want := fmt.Sprintf("tfg - %s - %d B - damage %s - seed %s", id, size, chain.Label(), core.SeedLabel(seed))
	if !strings.Contains(chain.Label(), "=") {
		return "the chain " + chain.Label() + " does not name its settings, so the label cannot say how much was broken"
	}
	if !bytes.Contains(got, []byte(want)) {
		return fmt.Sprintf("the file does not carry the label %q", want)
	}
	return ""
}

// TestEveryCharacterOfADamagedLabelCanBeDrawn holds the label a picture draws
// to the characters its font has.
//
// The font is 3 by 5 pixels and drawn by hand, and a character it lacks comes
// out as a box the shape of a 0. Before the label named a damage it never held
// an = or a comma, and the font had neither - a picture cut with missing=50
// would have said MISSING050. Every damage from the registry, at each end of
// every setting it takes, so a damage added with a new character in its name
// or settings turns this red.
func TestEveryCharacterOfADamagedLabelCanBeDrawn(t *testing.T) {
	if imagelabel.HasGlyph('~') {
		t.Fatal("the font draws ~, which it never had - the question below cannot tell a glyph from the stand-in")
	}
	asked := 0
	for _, d := range damage.All() {
		values := []damage.Values{{}}
		for _, p := range d.Parameters {
			values = append(values,
				damage.Values{p.Name: strconv.FormatInt(p.Min, 10)},
				damage.Values{p.Name: strconv.FormatInt(p.Max, 10)})
		}
		for _, v := range values {
			chain := damage.Chain{{ID: d.ID, Values: v}, {ID: d.ID, Values: v}}
			label := format.Request{Bytes: 1 << 40, Seed: 1, Damage: chain.Label()}.LabelText("png")
			for _, r := range label {
				if !imagelabel.HasGlyph(r) {
					t.Errorf("%s: the picture font has no %q, so the label %q is drawn with a box in its place", d.ID, r, label)
				}
			}
			asked++
		}
	}
	if asked == 0 {
		t.Fatal("no damage was asked - the registry was not read")
	}
}
