// Package mp4 generates MP4 films: AV1 in ISO base media boxes, with no sound.
//
// The pictures and the stream are internal/format/video's, shared with WebM.
// What is here is the container: the file type, the movie box with the one
// track and its sample tables, the media data, and free boxes that carry the
// padding.
//
// MP4 describes every sample before the media data - its length in stsz, its
// place in stco - and the movie box goes first, so a browser can start playing
// before the file is in (the probe of docs/WIDEO-2026-10-06.md section 10).
// The pictures of a film differ, and how long each one is is known only once
// it is coded. So a film is coded twice: once for the lengths, which the movie
// box is written from, and again for the bytes, taking every tile the first
// pass kept (video.Pictures.Again). Coding a film is seconds since its pictures
// are cut into tiles - an hour of 1080p changing every second in under one -
// so the second pass costs seconds (docs/MP4-2026-10-07.md section 2, the
// owner's decision of the same day).
//
// The padding channel is free boxes at the end, after the media data: nothing
// in the movie box points past it, so it can grow a byte at a time. Measured
// with the probe at 8, 9, 15, 16 and 1 048 576 bytes and with none. A free box
// is always written, at least its eight bytes, which leaves no size above the
// minimum that cannot be reached, and one larger than a box length can say is
// several (internal/format/isobmff).
package mp4

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

const (
	id               = "mp4"
	generatorVersion = "1"

	// minFree is the smallest free box: its header and nothing in it.
	minFree = isobmff.BoxHeader
)

func init() {
	format.Register(format.Descriptor{
		ID:          id,
		Name:        "MP4 video",
		Extension:   ".mp4",
		MediaType:   "video/mp4", // IANA, RFC 4337
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "free boxes after the pictures at the end of the file",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:            format.LabelVisible,
		Oracle:           "libaom",
		Properties:       video.Properties(),
		JointLimits:      video.JointLimits(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

// minimumBytes is the smallest film at the default settings - the smallest
// rung's bound, ten seconds at thirty frames, one key frame, a new picture
// every second - with its free box. Worked out from the same layout planning
// uses rather than written down, so the two cannot disagree.
func minimumBytes() int64 {
	t, err := video.NewTimeline(id, 10_000, 60_000, 1_000, 30)
	if err != nil {
		panic(err)
	}
	smallest := video.Ladder[len(video.Ladder)-1]
	return newLayout(video.NewStream(t, smallest.Ceiling, smallest.Width, smallest.Height, "")).need()
}

type generator struct{}

// AllocCeiling is how many objects this generator may allocate for one
// file, for the resource guard - see format.AllocCeiling.
//
// WebM's ceiling and for WebM's reason: every picture is coded on its
// own, a tile a call to gav1d, and gav1d allocates twenty odd objects
// a call (internal/format/webm). The second pass takes every tile the
// first one kept, so the default film calls gav1d no more often than
// its WebM does. Measured on 2026-10-07, lowest of the guard's rounds:
// 938 to 941 objects at one, four and sixteen threads (WebM 898 to
// 904), the sample lengths and the second pass's crew the difference -
// and 1241 with one object allocated per frame, the defect the ceiling
// exists for. 1024 sits between the two.
func (generator) AllocCeiling() int64 { return 1024 }

func (generator) Plan(r format.Request) (format.Plan, error) {
	need := func(st video.Stream) int64 { return newLayout(st).need() }
	return video.Plan(id, r, need, belowMinimum)
}

// belowMinimum names the four things D6 asks for, and the knobs that make a
// film smaller (video.Hint).
func belowMinimum(requested, need int64, c video.Choice, s video.Settings) error {
	return &format.BelowMinimumError{
		Format:    "MP4",
		Requested: requested,
		Minimum:   need,
		Reason: core.Says("mp4.MinimumReasonPictures",
			"a film of %s at %d frames a second with a key frame every %s and a new %dx%d picture every %s takes %d B, and the file always carries a free box, which costs %d B even when it holds nothing",
			core.A("Duration", core.FormatDuration(s.DurationMs)), core.A("FPS", s.FPS),
			core.A("Interval", core.FormatDuration(s.KeyEvery*1000/int64(s.FPS))),
			core.A("Width", c.Width), core.A("Height", c.Height), core.A("Change", core.FormatDuration(s.ChangeMs())),
			core.A("Bytes", need-minFree), core.A("Free", minFree)),
		Hint: video.Hint(need, c),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	f, ok := p.Memo.(video.Film)
	if !ok {
		return core.Defect(fmt.Errorf("mp4: the plan was not produced by this generator"))
	}
	st := f.Stream()
	l := newLayout(st)
	z, pics, err := measure(ctx, f, st)
	if err != nil {
		return err
	}
	defer pics.Close()

	o := &out{w: w, buf: make([]byte, 0, 64<<10)}
	content := l.writeMovie(o, z)
	o.mediaHeader(l.bigData, content)
	if err := writeSamples(ctx, o, l, pics); err != nil {
		return err
	}
	if o.flush(); o.err != nil {
		return o.err
	}
	// The padding is what the first pass's lengths leave, not what the second
	// pass wrote, so a second pass that came out different leaves the file a
	// different size from the plan - which the engine refuses - rather than a
	// movie box that describes samples it does not carry.
	pad := p.Bytes - l.front() - content
	if pad < minFree {
		return core.Defect(fmt.Errorf("mp4: the pictures came to %d B of media data and the file is %d B, which leaves no room for the free box every one of these carries",
			content, p.Bytes))
	}
	return isobmff.WritePadding(ctx, w, f.Choice.Seed, pad)
}

// sizes is how long each picture's two samples came out in the first pass, by
// change - a hundred thousand changes at most, eight bytes each.
type sizes struct {
	t    video.Timeline
	show int64
	// pictures is the key sample and the copy sample of each change.
	pictures [][2]uint32
}

// of is how long frame i's sample is.
func (z sizes) of(i int64) int64 {
	k := z.t.SampleAt(i)
	if k == video.ShowSample {
		return z.show
	}
	return int64(z.pictures[z.t.ChangeOf(i)][k])
}

// chunk is the media data of the chunk of samples frames opening at frame
// first. Only its first two frames can carry a picture (layout.eachChunk).
func (z sizes) chunk(first, samples int64) int64 {
	n := z.of(first)
	if samples > 1 {
		n += z.of(first + 1)
	}
	if rest := samples - 2; rest > 0 {
		n += rest * z.show
	}
	return n
}

// measure is the first pass: every picture coded once, in order, for how long
// its samples are, and the coder of the second pass. Nothing of the file is
// written yet, so a picture over its reserve is refused before the file has a
// byte. The work coding is worth is reported here, and the second pass,
// which codes only what this one could not keep, reports none.
//
// The first pass is closed on every way out - an error, a stop, and a panic
// in coding, which the engine turns into the run's error and which would
// otherwise leave this pass's helpers running. Closing it twice is harmless,
// and Again closes it too.
func measure(ctx context.Context, f video.Film, st video.Stream) (sizes, *video.Pictures, error) {
	first := f.Choice.Pictures(st)
	defer first.Close()
	z := sizes{t: st.Timeline, show: int64(st.BoundBytes()[video.ShowSample]), pictures: make([][2]uint32, st.Changes())}
	for c := range st.Changes() {
		if err := ctx.Err(); err != nil {
			return sizes{}, nil, err
		}
		if err := first.At(c); err != nil {
			return sizes{}, nil, err
		}
		format.Worked(ctx, first.Work())
		n := first.SampleLens()
		z.pictures[c] = [2]uint32{uint32(n[video.KeySample]), uint32(n[video.CopySample])}
	}
	again, err := first.Again()
	if err != nil {
		return sizes{}, nil, err
	}
	return z, again, nil
}

// writeMovie writes the file type and the movie box, with the sample tables
// from the first pass's lengths, and gives back how long the media data is.
func (l layout) writeMovie(o *out, z sizes) int64 {
	o.write(l.ftyp)
	o.header("moov", l.moovLen())
	o.write(l.mvhd)
	o.header("trak", l.trakLen())
	o.write(l.tkhd)
	o.header("mdia", l.mdiaLen())
	o.write(l.mdhd)
	o.write(l.hdlr)
	o.header("minf", l.minfLen())
	o.write(l.vmhd)
	o.write(l.dinf)
	o.header("stbl", l.stblLen())
	o.write(l.stsd)
	o.write(l.stts)
	l.writeSyncs(o)
	l.writeDependencies(o)
	l.writeChunkRuns(o)
	l.writeSizes(o, z)
	return l.writeChunkPlaces(o, z)
}

// writeSyncs is stss: every key frame, numbered from one. Each is a sample a
// player can start from - a shown key frame with the sequence header before
// it, as the AV1 binding asks of a sync sample (section 2.4).
func (l layout) writeSyncs(o *out) {
	t := l.stream.Timeline
	o.fullHeader("stss", l.stssLen())
	o.u32(uint32(l.counts[video.KeySample]))
	for k := range l.counts[video.KeySample] {
		o.u32(uint32(k*t.KeyEvery + 1))
	}
}

// Dependencies, as sdtp writes sample_depends_on: a key frame and a hidden
// intra only copy depend on no other sample - the AV1 binding says an intra
// only frame SHOULD be signalled so (section 2.4) - and a frame that shows a
// picture again depends on the copy before it.
const (
	dependsOnNone   = 2 << 4
	dependsOnOthers = 1 << 4
)

func (l layout) writeDependencies(o *out) {
	t := l.stream.Timeline
	o.fullHeader("sdtp", l.sdtpLen())
	for i := range t.Frames {
		v := byte(dependsOnNone)
		if t.SampleAt(i) == video.ShowSample {
			v = dependsOnOthers
		}
		o.u8(v)
	}
}

// writeChunkRuns is stsc: one entry for each run of chunks holding the same
// number of samples, the way newLayout counted them.
func (l layout) writeChunkRuns(o *out) {
	o.fullHeader("stsc", l.stscLen())
	o.u32(uint32(l.runs))
	chunk, last := int64(0), int64(-1)
	l.eachChunk(func(_, samples int64) {
		chunk++
		if samples != last {
			o.u32(uint32(chunk))
			o.u32(uint32(samples))
			o.u32(1)
			last = samples
		}
	})
}

func (l layout) writeSizes(o *out, z sizes) {
	t := l.stream.Timeline
	o.fullHeader("stsz", l.stszLen())
	o.u32(0)
	o.u32(uint32(t.Frames))
	for i := range t.Frames {
		o.u32(uint32(z.of(i)))
	}
}

// writeChunkPlaces is stco, or co64 when the layout asked for eight bytes, and
// gives back where the last chunk ends, counted from the media data's start.
func (l layout) writeChunkPlaces(o *out, z sizes) int64 {
	kind := "stco"
	if l.wide {
		kind = "co64"
	}
	o.fullHeader(kind, l.stcoLen())
	o.u32(uint32(l.chunks))
	front, content := l.front(), int64(0)
	l.eachChunk(func(first, samples int64) {
		if l.wide {
			o.u64(uint64(front + content))
		} else {
			o.u32(uint32(front + content))
		}
		content += z.chunk(first, samples)
	})
	return content
}

// writeSamples is the second pass: every frame's sample, chunk by chunk, the
// picture of each chunk made as it opens.
func writeSamples(ctx context.Context, o *out, l layout, pics *video.Pictures) error {
	t := l.stream.Timeline
	var err error
	l.eachChunk(func(first, samples int64) {
		if err != nil {
			return
		}
		if err = ctx.Err(); err != nil {
			return
		}
		if err = pics.At(t.ChangeOf(first)); err != nil {
			return
		}
		frame := pics.Samples()
		for i := first; i < first+samples; i++ {
			o.write(frame[t.SampleAt(i)])
		}
	})
	if err == nil {
		err = o.err
	}
	return err
}

// out keeps the first error, so a long run of small writes checks once, and
// gathers the sample tables - four bytes a frame, five million frames in a
// day - into one buffer rather than a call each.
type out struct {
	w   io.Writer
	err error
	buf []byte
}

func (o *out) flush() {
	if o.err == nil && len(o.buf) > 0 {
		_, o.err = o.w.Write(o.buf)
	}
	o.buf = o.buf[:0]
}

// room makes room for n more bytes in the buffer.
func (o *out) room(n int) {
	if len(o.buf)+n > cap(o.buf) {
		o.flush()
	}
}

// write takes a piece larger than half the buffer straight through, after
// what the buffer holds - a picture can be megabytes.
func (o *out) write(b []byte) {
	if len(b) > cap(o.buf)/2 {
		o.flush()
		if o.err == nil {
			_, o.err = o.w.Write(b)
		}
		return
	}
	o.room(len(b))
	o.buf = append(o.buf, b...)
}

func (o *out) u8(v byte) {
	o.room(1)
	o.buf = append(o.buf, v)
}

func (o *out) u32(v uint32) {
	o.room(4)
	o.buf = binary.BigEndian.AppendUint32(o.buf, v)
}

func (o *out) u64(v uint64) {
	o.room(8)
	o.buf = binary.BigEndian.AppendUint64(o.buf, v)
}

// header is a box's header, its length counted rather than built.
func (o *out) header(kind string, length int64) {
	o.u32(uint32(length))
	o.room(len(kind))
	o.buf = append(o.buf, kind...)
}

// fullHeader is a full box's header, version nought and no flags - every
// table here is one.
func (o *out) fullHeader(kind string, length int64) {
	o.header(kind, length)
	o.u32(0)
}

// mediaHeader is the media data's header: its length in four bytes, or in
// eight after a length of one when the layout said a film of this stream could
// need them.
func (o *out) mediaHeader(big bool, content int64) {
	if !big {
		o.header("mdat", isobmff.BoxHeader+content)
		return
	}
	o.header("mdat", 1)
	o.u64(uint64(isobmff.BoxHeader + 8 + content))
}
