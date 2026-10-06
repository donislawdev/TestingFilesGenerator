// Package webm generates WebM films: AV1 in the WebM subset of Matroska, with
// no sound.
//
// The picture and the stream are internal/format/video's, shared with MP4.
// What is here is the container: EBML written as a stream, a SeekHead, the one
// track, clusters, Cues, and a Void at the end that carries the padding.
//
// The padding channel is a Void element, the one Matroska defines for bytes
// that mean nothing, after the Cues and inside the segment. The segment's size
// is always written in eight bytes, so the Void can grow a byte at a time and
// nothing before it moves. Measured on 2026-10-06 (docs/WIDEO-2026-10-06.md
// section 9.6) at a Void of 2, 3, 128, 129, 16 385, 16 386 and 1 048 576
// bytes and with none: every file came out at the size asked for, libaom
// decoded every frame of every one, and Chromium played and seeked them all.
// A Void is always written, at least its two bytes, which leaves no size above
// the minimum that cannot be reached.
package webm

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

const (
	id               = "webm"
	generatorVersion = "1"
)

func init() {
	format.Register(format.Descriptor{
		ID:          id,
		Name:        "WebM video",
		Extension:   ".webm",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "a Void element at the end of the file",
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
// rung's bound, ten seconds at thirty frames, one key frame - with its Void.
// Worked out from the same layout planning uses rather than written down, so
// the two cannot disagree.
func minimumBytes() int64 {
	t, err := video.NewTimeline(id, 10_000, 60_000, 30)
	if err != nil {
		panic(err)
	}
	smallest := video.Ladder[len(video.Ladder)-1]
	return newLayout(video.Bound(t, smallest.Ceiling, smallest.Width, smallest.Height)).fileBytes() + minVoid
}

type generator struct{}

type memo struct {
	choice   video.Choice
	settings video.Settings
	total    int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label(id, r.Bytes, r.Seed)
	}
	s, err := video.Read(id, r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	fits := func(st video.Stream) bool { return newLayout(st).fileBytes()+minVoid <= r.Bytes }
	c, st, err := video.Choose(id, r, s, label, fits)
	if err != nil {
		return format.Plan{}, err
	}
	if need := newLayout(st).fileBytes() + minVoid; need > r.Bytes {
		return format.Plan{}, belowMinimum(r.Bytes, need, c, s)
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties:  video.Facts(c, s),
		Memo:        memo{choice: c, settings: s, total: r.Bytes},
	}
	if r.Label && !c.Labelled() {
		p.Notes = append(p.Notes, format.Note{
			Code:   "label_omitted",
			Detail: core.Says("format.ThePictureIsPxWideAnd", "The picture is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.", core.A("Width", c.Width)),
		})
	}
	return p, nil
}

// belowMinimum names the four things D6 asks for, and the knobs that make a
// film smaller - every one of them is a setting, so each is named.
func belowMinimum(requested, need int64, c video.Choice, s video.Settings) error {
	return &format.BelowMinimumError{
		Format:    "WebM",
		Requested: requested,
		Minimum:   need,
		Reason: core.Says("webm.MinimumReason",
			"a film of %s at %d frames a second with a key frame every %s and a %dx%d picture takes %d B, and the file always carries a Void element, which costs %d B even when it holds nothing",
			core.A("Duration", core.FormatDuration(s.DurationMs)), core.A("FPS", s.FPS),
			core.A("Interval", core.FormatDuration(s.KeyEvery*1000/int64(s.FPS))),
			core.A("Width", c.Width), core.A("Height", c.Height),
			core.A("Bytes", need-minVoid), core.A("Void", minVoid)),
		Hint: hint(need, c),
	}
}

// hint offers a smaller picture only when the request named one. A picture
// chosen to fit is already the smallest that would, so offering a smaller
// one would send somebody to a setting that changes nothing.
func hint(need int64, c video.Choice) core.Said {
	if c.Named {
		return core.Says("webm.AskForBOrMore",
			"Ask for %d B or more, or a shorter film, fewer frames a second, key frames further apart or a smaller picture",
			core.A("Floor", need))
	}
	return core.Says("webm.AskForBOrMoreFilm",
		"Ask for %d B or more, or a shorter film, fewer frames a second or key frames further apart",
		core.A("Floor", need))
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("webm: the plan was not produced by this generator"))
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	coded, err := m.choice.Code()
	if err != nil {
		return err
	}
	st := video.NewStream(m.settings.Timeline, coded, m.choice.Width, m.choice.Height)
	l := newLayout(st)
	void := m.total - l.fileBytes()
	if void < minVoid {
		return core.Defect(fmt.Errorf("webm: the film came to %d B and the file was to be %d B, which leaves no room for the Void every one of these carries",
			l.fileBytes(), m.total))
	}

	out := &sticky{w: w}
	out.write(l.head)
	out.write(idBytes(idSegment))
	out.write(sizeField(l.body+uint64(void), segmentSizeField))
	out.write(l.seekHead)
	out.write(l.info)
	out.write(l.tracks)
	if err := writeClusters(ctx, out, l); err != nil {
		return err
	}
	writeCues(out, l)
	if out.err != nil {
		return out.err
	}
	return writeVoid(ctx, w, void, m.choice.Seed)
}

// sticky keeps the first error, so a long run of small writes checks once,
// and carries the few bytes a header takes so writing one allocates nothing.
// A film's frames number in the millions, and a header built fresh for each
// was an allocation per frame - measured 2026-10-06, 853 objects for a ten
// second film where the guard on generators allows 128.
type sticky struct {
	w       io.Writer
	err     error
	scratch [24]byte
}

func (s *sticky) write(b []byte) {
	if s.err == nil {
		_, s.err = s.w.Write(b)
	}
}

func (s *sticky) header(id uint32, content uint64) {
	s.write(appendHeader(s.scratch[:0], id, content))
}

func (s *sticky) uint(id uint32, v uint64) {
	s.write(appendUint(s.scratch[:0], id, v))
}

func writeClusters(ctx context.Context, out *sticky, l layout) error {
	for _, c := range l.clusters {
		if err := ctx.Err(); err != nil {
			return err
		}
		writeCluster(out, l.stream, c)
		if out.err != nil {
			return out.err
		}
	}
	return nil
}

// writeCluster writes one cluster: its time, then a SimpleBlock for each of
// its frames - track 1, the frame's time from the cluster's, and the key flag
// on a key frame.
func writeCluster(out *sticky, st video.Stream, c cluster) {
	out.header(idCluster, c.content)
	out.uint(idTimestamp, uint64(c.ts))
	var block [4]byte
	block[0] = 0x81
	for i := c.first; i < c.last; i++ {
		sample := st.Sample(i)
		rel := st.StartMs(i) - c.ts
		block[1], block[2], block[3] = byte(rel>>8), byte(rel), 0
		if st.IsKey(i) {
			block[3] = 0x80
		}
		out.header(idSimpleBlock, uint64(4+len(sample)))
		out.write(block[:])
		out.write(sample)
	}
}

// writeCues points at every cluster that opens with a key frame, with the
// same arithmetic the layout counted them by.
func writeCues(out *sticky, l layout) {
	out.header(idCues, l.cuesBody)
	for _, c := range l.clusters {
		if !l.stream.IsKey(c.first) {
			continue
		}
		positions := uintElementLen(idCueTrack, 1) + uintElementLen(idCueClusterPosition, c.position)
		out.header(idCuePoint, uintElementLen(idCueTime, uint64(c.ts))+elementLen(idCueTrackPositions, positions))
		out.uint(idCueTime, uint64(c.ts))
		out.header(idCueTrackPositions, positions)
		out.uint(idCueTrack, 1)
		out.uint(idCueClusterPosition, c.position)
	}
}

// writeVoid fills the rest of the file with one Void element of exactly total
// bytes, without ever holding its content.
func writeVoid(ctx context.Context, w io.Writer, total int64, seed uint64) error {
	k := 1
	for ; k < 8; k++ {
		if payload := uint64(total) - 1 - uint64(k); payload < 1<<(7*uint(k))-1 {
			break
		}
	}
	payload := total - 1 - int64(k)
	if _, err := w.Write(append([]byte{idVoid}, sizeField(uint64(payload), k)...)); err != nil {
		return err
	}
	rng := core.NewRand(seed)
	buf := make([]byte, 32*1024)
	for left := payload; left > 0; {
		n := min(int64(len(buf)), left)
		if err := ctx.Err(); err != nil {
			return err
		}
		core.FillRandomLE(buf[:n], rng)
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		left -= n
	}
	return nil
}
