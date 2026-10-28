package guard

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	pic "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// Every format whose writing holds more memory for a larger picture says so in
// its plan - docs/STABILITY-REVIEW-2026-10-26.md sections 10.4 and 10.8.
//
// The memory setting holds the writers back by what their files declare
// (format.Plan.Holds), so a format that forgets to declare is a format the
// setting cannot see: sixteen of its pictures go out at once whatever the
// setting says, which is R02 back again for that one format.
//
// Asked of every format with a width and a height, from the registry, so a
// format added later is asked without being named here. The question is put
// to what writing allocates rather than to the code: the same file planned
// twice, the second picture with twice the pixels, and the bytes allocated
// while each is written. A format whose writing allocates more for the larger
// picture holds the picture, and has to declare more than nothing and more for
// the larger one. A format that streams allocates the same for both and may
// declare nothing - bmp, tiff and webp do, and so do jxl and avif with a size
// named, because they code the picture while planning.
//
// The allocation is a bound from above on what is held, not a measure of it,
// so this cannot say a declaration is too small - only that one is missing or
// does not grow. The numbers themselves are measured in the comment beside
// each.
func TestEveryPictureSaysWhatWritingItHolds(t *testing.T) {
	const w, h = 128, 96
	growing := map[string]bool{}
	for _, d := range format.All() {
		if !takes(d, "width") || !takes(d, "height") {
			continue
		}
		small, smallAlloc := writtenAt(t, d, w, h)
		large, largeAlloc := writtenAt(t, d, 2*w, h)
		if largeAlloc < smallAlloc+uint64(w*h) {
			continue
		}
		growing[d.ID] = true
		if small.Holds <= 0 || large.Holds <= small.Holds {
			t.Errorf("%s allocates %d B writing a %dx%d picture and %d B writing one twice as wide, so writing it holds the picture - and it declares %d B and %d B. The memory setting cannot hold back what a plan does not declare",
				d.ID, smallAlloc, w, h, largeAlloc, small.Holds, large.Holds)
		}
	}
	// The three that hold their picture while writing, measured on
	// 2026-10-27 - if the instrument stops seeing them, it is the instrument
	// that broke, and the guard says so rather than passing on nothing.
	for _, id := range []string{"jpg", "png", "gif"} {
		if !growing[id] {
			t.Errorf("%s did not allocate more for a larger picture, so this measurement no longer sees a format holding its picture", id)
		}
	}
	t.Logf("formats whose writing grows with the picture: %v", growing)
}

// The same question with no size named, where the format chooses the picture.
//
// jxl and avif choose from a ladder without coding anything while planning,
// and code the picture while writing - so the declaration that matters for
// them is on the path the guard above never takes, since it names a size.
// Doubling the pixels cannot be asked here, because the format picks them, so
// the question is the plainer one: writing allocated at least a byte for each
// pixel of the picture it chose, and then the plan has to declare something.
func TestEveryPictureItChoosesSaysWhatWritingItHolds(t *testing.T) {
	coded := map[string]bool{}
	for _, d := range format.All() {
		if !takes(d, "width") || !takes(d, "height") {
			continue
		}
		p, alloc := written(t, d, format.Request{Bytes: 1 << 20, Seed: 1})
		w, _ := p.Properties["width"].(int)
		h, _ := p.Properties["height"].(int)
		if w*h == 0 || alloc < uint64(w*h) {
			continue
		}
		coded[d.ID] = true
		if p.Holds <= 0 {
			t.Errorf("%s chose a %dx%d picture and allocated %d B writing it, and declares it holds %d B", d.ID, w, h, alloc, p.Holds)
		}
	}
	for _, id := range []string{"jxl", "avif"} {
		if !coded[id] {
			t.Errorf("%s with no size named did not allocate a byte a pixel while writing, so this no longer reaches the picture it codes then", id)
		}
	}
	t.Logf("formats that hold the picture they chose: %v", coded)
}

// A container holds what the largest picture inside it holds, because it
// writes them one after another and codes each while it writes.
func TestAContainerHoldsWhatItsLargestPictureHolds(t *testing.T) {
	png := planned(t, "png", format.Request{Bytes: 64 << 10, Seed: 1})
	if png.Holds <= 0 {
		t.Fatalf("a png declares it holds %d B, so there is nothing for a container to pass up", png.Holds)
	}
	inside := []format.Content{{Format: "txt", Count: 2, Bytes: 1024}, {Format: "png", Count: 2, Bytes: 64 << 10}}
	for _, id := range []string{"zip", "targz"} {
		p := planned(t, id, format.Request{Bytes: 1 << 20, Seed: 1, Contains: inside})
		if p.Holds != png.Holds {
			t.Errorf("%s holding two pngs of %d B declares %d B, and each png declares %d B - a container writes them one at a time, so it holds the largest of them",
				id, 64<<10, p.Holds, png.Holds)
		}
	}
	pages := 0
	for _, d := range format.All() {
		if !takes(d, pic.Count) {
			continue
		}
		pages++
		p := planned(t, d.ID, format.Request{Bytes: 1 << 20, Seed: 1, Properties: map[string]string{pic.Count: "3", "inline_size": "16kb"}})
		if p.Holds <= 0 {
			t.Errorf("%s with three pictures in it declares %d B, though it codes each picture while it writes", d.ID, p.Holds)
		}
	}
	if pages == 0 {
		t.Fatal("no format takes inline pictures, so the second half of this guard asked nothing")
	}
}

// takes says whether a format has a setting of this name.
func takes(d format.Descriptor, name string) bool {
	for _, p := range d.Properties {
		if p.Name == name {
			return true
		}
	}
	return false
}

// planned is a plan of one file, or the end of the test.
func planned(t *testing.T, id string, r format.Request) format.Plan {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := format.PlanWith(context.Background(), d.Generator, r)
	if err != nil {
		t.Fatalf("planning %s: %v", id, err)
	}
	return p
}

// writtenAt is written with the picture's width and height named.
func writtenAt(t *testing.T, d format.Descriptor, w, h int) (format.Plan, uint64) {
	t.Helper()
	return written(t, d, format.Request{Bytes: 1 << 20, Seed: 1, Properties: map[string]string{"width": strconv.Itoa(w), "height": strconv.Itoa(h)}})
}

// written plans one file of the request at a size the format can hold, writes
// it, and says what the plan declared and what writing allocated.
func written(t *testing.T, d format.Descriptor, r format.Request) (format.Plan, uint64) {
	t.Helper()
	p, err := format.PlanWith(context.Background(), d.Generator, r)
	var below *format.BelowMinimumError
	if errors.As(err, &below) {
		r.Bytes = below.Minimum
		p, err = format.PlanWith(context.Background(), d.Generator, r)
	}
	var above *format.AboveMaximumError
	if errors.As(err, &above) {
		r.Bytes = above.Maximum
		p, err = format.PlanWith(context.Background(), d.Generator, r)
	}
	if err != nil {
		t.Fatalf("planning a %s of %v: %v", d.ID, r.Properties, err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := d.Generator.Write(context.Background(), io.Discard, p); err != nil {
		t.Fatalf("writing a %s of %v: %v", d.ID, r.Properties, err)
	}
	runtime.ReadMemStats(&after)
	return p, after.TotalAlloc - before.TotalAlloc
}
