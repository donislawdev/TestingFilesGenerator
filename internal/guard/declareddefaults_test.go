package guard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder"
)

// A declared default has to be the one the format actually uses.
//
// AR9 makes the registry the single place a consumer asks what a format
// accepts, and the default is part of that answer: it is what tfg formats
// prints, what a window will fill a field with, and what somebody reads before
// deciding not to pass the setting at all.
//
// Nothing checked that the printed answer was true. Found on 2026-08-04:
// ZIP declared entry_size as 8kb and its generator used 4096, so
//
//	tfg formats zip          says the files inside default to 8 kB
//	tfg generate ... zip     puts 4 kB files inside
//
// Neither number was wrong on its own. They were written in two places and one
// of them drifted, which is the failure this project keeps finding, and the
// declaration is the half that consumers believe.
//
// Asked on the bytes rather than on the plan. A default that changed only a
// value carried into the manifest would slip past a comparison of sizes, and
// the question here is whether leaving a setting out gives the same file as
// writing down what the tool said it would use.
func TestADeclaredDefaultIsTheOneTheFormatUses(t *testing.T) {
	descriptors := format.All()
	if len(descriptors) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	checked := 0
	for _, d := range descriptors {
		for _, p := range d.Properties {
			// An empty default means the format works the value out from the
			// size it was given - a picture choosing its own dimensions, say -
			// so there is no promise to keep.
			if p.Default == "" {
				continue
			}

			// Left out and written out, both beside what the setting needs to
			// do anything at all.
			left := companionOf(p)
			stated := map[string]string{p.Name: p.Default}
			for k, v := range left {
				stated[k] = v
			}
			size := sizeBothWaysAccept(d, left, stated)
			if size < 0 {
				t.Errorf("%s: cannot find a size that works both with and without %s=%s",
					d.ID, p.Name, p.Default)
				continue
			}

			silent, err := produceBytes(d, size, left)
			if err != nil {
				t.Errorf("%s: generating with %s left out failed: %v", d.ID, p.Name, err)
				continue
			}
			spelled, err := produceBytes(d, size, stated)
			if err != nil {
				t.Errorf("%s: generating with %s=%s written out failed: %v",
					d.ID, p.Name, p.Default, err)
				continue
			}

			checked++
			if silent != spelled {
				t.Errorf("%s: %s is declared to default to %q and the generator does something else.\n"+
					"  with the setting left out:    %s\n"+
					"  with %s=%s written out: %s\n"+
					"  tfg formats prints the declaration, so the printed answer is the one users believe",
					d.ID, p.Name, p.Default, silent, p.Name, p.Default, spelled)
			}
		}
	}

	if checked == 0 {
		t.Fatal("no declared default was compared - this guard would pass without checking anything")
	}
	// Compared, not matched. The counter rises before the comparison, so
	// calling these matches would have this line claim agreement in the same
	// run that reports a difference - which it did, once, before anybody read
	// it closely.
	t.Logf("%d declared default(s) compared against what the generator does", checked)
}

// sizeBothWaysAccept finds a size both shapes will produce, so a difference in
// the bytes is a difference in the content rather than in what was refused.
func sizeBothWaysAccept(d format.Descriptor, left, stated map[string]string) int64 {
	base := format.Request{Seed: 909, Label: true}
	withOut := base
	withOut.Properties = left
	without := d.SmallestAccepted(withOut)

	withIt := base
	withIt.Properties = stated
	withThem := d.SmallestAccepted(withIt)

	size := without
	if withThem > size {
		size = withThem
	}
	// Well clear of both floors, so neither shape is answering from a band it
	// cannot reach.
	size += 300 * 1024

	for _, props := range []map[string]string{left, stated} {
		r := base
		r.Bytes = size
		r.Properties = props
		if _, err := d.Generator.Plan(r); err != nil {
			return -1
		}
	}
	return size
}

// companionOf is what a setting needs beside it to do anything at all: the
// settings of a placeholder need a placeholder, and placeholder.Read refuses
// them on a test picture. Read off the declaration's group rather than kept
// as a list here, so a setting added to the group is compared where it works.
func companionOf(p format.Property) map[string]string {
	if p.Group == placeholder.Group && p.Name != placeholder.SettingContent {
		return map[string]string{placeholder.SettingContent: placeholder.ContentPlaceholder}
	}
	return nil
}

func produceBytes(d format.Descriptor, size int64, props map[string]string) (string, error) {
	plan, err := d.Generator.Plan(format.Request{
		Bytes: size, Seed: 909, Label: true, Properties: props,
	})
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])[:16], nil
}
