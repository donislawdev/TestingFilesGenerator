// Package webm generates WebM films: AV1 in the WebM subset of Matroska, with
// no sound.
//
// The pictures and the stream are internal/format/video's, shared with MP4.
// What is here is the container: EBML written as a stream, a SeekHead, the one
// track, clusters, a Void that carries the padding, and the Cues last.
//
// The padding channel is a Void element, the one Matroska defines for bytes
// that mean nothing, between the last cluster and the Cues. The segment's size
// is always written in eight bytes and the Cues are the same length whatever
// they point at, so the Void can grow a byte at a time and nothing before it
// moves. It sat after the Cues until the pictures began to change
// (docs/WIDEO-2026-10-06.md section 15): a cluster's length is known only
// once its picture is coded, and the SeekHead, written first, has to say where
// the Cues are - at the end, less their length, is the one place that does not
// depend on the pictures. The Void at the end was measured on 2026-10-06
// (section 9.6) at 2, 3, 128, 129, 16 385, 16 386 and 1 048 576 bytes and with
// none. A Void is always written, at least its two bytes, which leaves no size
// above the minimum that cannot be reached. A film a damage will cut splits
// the padding around the clusters, so the cut keeps the share of the pictures
// asked for, and is coded twice for it (cut.go).
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
		MediaType:   "video/webm", // not in the IANA registry, WHATWG MIME Sniffing
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "a Void element before the index at the end of the file",
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
// every second - with its Void. Worked out from the same layout planning uses
// rather than written down, so the two cannot disagree.
func minimumBytes() int64 {
	t, err := video.NewTimeline(id, 10_000, 60_000, 1_000, 30)
	if err != nil {
		panic(err)
	}
	smallest := video.Ladder[len(video.Ladder)-1]
	return newLayout(video.NewStream(t, smallest.Ceiling, smallest.Width, smallest.Height, "")).boundBytes() + minVoid
}

type generator struct{}

// AllocCeiling is how many objects this generator may allocate for one
// file, for the resource guard - see format.AllocCeiling.
//
// Every picture of a film is coded on its own, and gav1d allocates
// per picture - about forty objects each with the frames around it.
// The flat ceiling counts what a generator allocates per file, and a
// default film is ten pictures. Measured on 2026-10-06, lowest of the
// guard's rounds: 408 objects for the default film, and 698 with one
// object allocated per frame, the defect the ceiling exists for. The
// number below sits between the two, and like every ceiling here it
// goes down when work makes it lowerable, never up to turn a run green.
// Since the pictures are coded beside each other (internal/format/video,
// ahead.go) the default film is 474 to 477 objects at sixteen, four and
// one threads alike - each picture is a job, each helper one painter of
// four allocations - measured the same day, still under the ceiling,
// which was not moved.
//
// Raised to 1024 on 2026-10-07 by the owner's decision, the one time it
// went up, and why: a picture is cut into AV1 tiles since then, each
// tile one call to gav1d, and gav1d allocates 21 to 22 objects a call
// whatever the size (tools/probes/videotiles/allocs). The default film,
// 640x360 in six tiles, makes 27 calls - about 594 objects of gav1d's
// alone, over the old ceiling before this package allocates anything.
// With this package's own allocations cut down it is 886 to 891 at one,
// four and sixteen threads, and one object a frame, the defect the
// ceiling exists for, takes it to 1177. 1024 sits between the two, as
// 512 sat between 474 and 698 (docs/WEBM-WYDAJNOSC-2026-10-06.md
// section 10).
//
// Raised to 1536 on 2026-11-11 by the owner's decision, with gav1d v0.2.6,
// which no longer reserves its entropy coder's buffer up front and grows it
// instead: 36 to 57 objects a call where v0.2.5 made 21 to 22
// (tools/probes/videotiles/allocs, docs/GAV1D-026-2026-11-11.md). The
// default film is 1341 to 1350 objects at one, four and sixteen threads, and
// one object a frame takes it to 1634 to 1644. 1536 sits between the two.
func (generator) AllocCeiling() int64 { return 1536 }

func (generator) Plan(r format.Request) (format.Plan, error) {
	need := func(st video.Stream) int64 { return newLayout(st).boundBytes() + minVoid }
	return video.Plan(id, r, need, belowMinimum)
}

// belowMinimum names the four things D6 asks for, and the knobs that make a
// film smaller (video.Hint).
func belowMinimum(requested, need int64, c video.Choice, s video.Settings) error {
	return &format.BelowMinimumError{
		Format:    "WebM",
		Requested: requested,
		Minimum:   need,
		Reason: core.Says("webm.MinimumReasonPictures",
			"a film of %s at %d frames a second with a key frame every %s and a new %dx%d picture every %s takes %d B, and the file always carries a Void element, which costs %d B even when it holds nothing",
			core.A("Duration", core.FormatDuration(s.DurationMs)), core.A("FPS", s.FPS),
			core.A("Interval", core.FormatDuration(s.KeyEvery*1000/int64(s.FPS))),
			core.A("Width", c.Width), core.A("Height", c.Height), core.A("Change", core.FormatDuration(s.ChangeMs())),
			core.A("Bytes", need-minVoid), core.A("Void", minVoid)),
		Hint: video.Hint(need, c),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	f, ok := p.Memo.(video.Film)
	if !ok {
		return core.Defect(fmt.Errorf("webm: the plan was not produced by this generator"))
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	st := f.Stream()
	l := newLayout(st)
	body := uint64(p.Bytes - l.outside())
	cuesAt := body - l.cuesLen()
	pics := f.Choice.Pictures(st)
	defer func() { pics.Close() }()

	// A film a damage will cut is measured first, so part of its padding can
	// stand in front of the clusters (cut.go). One nothing cuts is coded once,
	// as it is written, and content stays unknown until the writing is done.
	content, gap := int64(-1), int64(0)
	if f.CutAt > 0 {
		n, again, err := measure(ctx, l, pics)
		if err != nil {
			return err
		}
		content, pics = n, again
		gap = splitForACut(int64(cuesAt-l.front())-content, l.outside()+int64(l.front()), content, f.CutAt, p.Bytes)
	}
	m := filmMap{w: w, l: l, cuesAt: cuesAt}
	m.head(gap, content)

	out := &sticky{w: w}
	writeHead(out, l, body, cuesAt)
	// Both Voids draw their filler from the film's seed. Nobody reads padding.
	if gap > 0 {
		if out.err != nil {
			return out.err
		}
		if err := writeVoid(ctx, w, gap, f.Choice.Seed); err != nil {
			return err
		}
	}
	keys, end, err := writeClusters(ctx, out, l, pics, l.front()+uint64(gap), content < 0)
	if err != nil {
		return err
	}
	void, err := voidBehind(l, cuesAt, end, gap, content)
	if err != nil {
		return err
	}
	if content < 0 {
		m.tail(end)
	}
	if void > 0 {
		if err := writeVoid(ctx, w, void, f.Choice.Seed); err != nil {
			return err
		}
	}
	writeCues(out, l, keys)
	return out.err
}

// writeHead writes everything in front of the clusters: the EBML header, the
// segment's ID and size, the seek head, the info and the tracks.
func writeHead(out *sticky, l layout, body, cuesAt uint64) {
	out.write(l.head)
	out.write(idBytes(idSegment))
	out.write(sizeField(body, segmentSizeField))
	out.write(seekHead(l.seekHeadLen, l.seekHeadLen+uint64(len(l.info)), cuesAt))
	out.write(l.info)
	out.write(l.tracks)
}

// voidBehind is the Void between the last cluster, which ends at end, and the
// Cues at cuesAt, both from the start of the segment's content. It is refused
// when it cannot be a Void, or when the clusters of a film measured first
// came to other than the first pass measured.
func voidBehind(l layout, cuesAt, end uint64, gap, content int64) (int64, error) {
	if wrote := int64(end-l.front()) - gap; content >= 0 && wrote != content {
		return 0, core.Defect(fmt.Errorf("webm: the clusters came to %d B in the first pass and %d B in the second", content, wrote))
	}
	void := int64(cuesAt) - int64(end)
	if void < 0 || void > 0 && void < minVoid || gap+void < minVoid {
		return 0, core.Defect(fmt.Errorf("webm: the clusters came to %d B and the Cues are to start at %d B, which leaves no room for the Void every one of these carries",
			end, cuesAt))
	}
	return void, nil
}

// filmMap marks a film's map (format.Mark): the headers, the seek head, the
// info and the tracks, the Void in front of the clusters when a damage will
// cut the film, the clusters, then the Void that makes up the size and the
// Cues at the end. A film measured first is marked whole before its first
// byte. One coded as it goes has the clusters' end marked when the writing
// reaches it, and a cut stops it before then.
type filmMap struct {
	w      io.Writer
	l      layout
	cuesAt uint64
}

// head marks what goes in front of the clusters, the clusters - content
// bytes, -1 while unknown - and, when they are known, what follows them.
func (m filmMap) head(gap, content int64) {
	start := m.l.outside() + int64(m.l.front())
	format.Mark(m.w, format.RoleStructure, 0, start)
	if gap > 0 {
		format.Mark(m.w, format.RolePadding, start, gap)
	}
	format.Mark(m.w, format.RoleContent, start+gap, content)
	if content >= 0 {
		m.tail(m.l.front() + uint64(gap+content))
	}
}

// tail marks what follows the clusters, which end at end from the start of
// the segment's content: the Void, when there is one, and the Cues.
func (m filmMap) tail(end uint64) {
	seg := m.l.outside()
	if void := int64(m.cuesAt) - int64(end); void > 0 {
		format.Mark(m.w, format.RolePadding, seg+int64(end), void)
	}
	format.Mark(m.w, format.RoleEnd, seg+int64(m.cuesAt), int64(m.l.cuesLen()))
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

// writeClusters codes the pictures one change at a time and writes the
// clusters as it goes, the first at pos from the start of the segment's
// content, giving back where each key frame's cluster starts, for the Cues,
// and where the last one ends. report says whether this pass reports the work
// - the only pass of a film nothing cuts does, the second of one a damage
// cuts does not (cut.go).
func writeClusters(ctx context.Context, out *sticky, l layout, pics *video.Pictures, pos uint64, report bool) ([]uint64, uint64, error) {
	s := l.stream
	keys := make([]uint64, 0, s.Keys())
	err := eachCluster(ctx, l, pics, report, func(first, last int64) error {
		if s.IsKey(first) {
			keys = append(keys, pos)
		}
		pos += writeCluster(out, l, pics, first, last)
		return out.err
	})
	if err != nil {
		return nil, 0, err
	}
	return keys, pos, nil
}

// eachCluster walks the film's clusters in order, by their frames [first,
// last), with the picture each carries made the current one first, and stops
// at the first error cluster gives back. Every change opens a cluster, so when
// report says so each picture is reported as worked once - what its new tiles
// were worth - which adds up to the Work the plan counted.
func eachCluster(ctx context.Context, l layout, pics *video.Pictures, report bool, cluster func(first, last int64) error) error {
	s := l.stream
	worked := int64(-1)
	for first := int64(0); first < s.Frames; {
		if err := ctx.Err(); err != nil {
			return err
		}
		// A cluster thirty seconds into a picture asks for the picture it
		// already has, which At answers without coding anything.
		change := s.ChangeOf(first)
		if err := pics.At(change); err != nil {
			return err
		}
		if report && change != worked {
			format.Worked(ctx, pics.Work())
			worked = change
		}
		last := l.next(first)
		if err := cluster(first, last); err != nil {
			return err
		}
		first = last
	}
	return nil
}

// writeCluster writes the cluster of frames [first, last) and returns its
// length: its time, then a SimpleBlock for each frame - track 1, the frame's
// time from the cluster's, and the key flag on a key frame. Which sample each
// frame carries is video.Timeline.SampleAt, the same answer the cluster's
// length was counted from.
func writeCluster(out *sticky, l layout, pics *video.Pictures, first, last int64) uint64 {
	s := l.stream
	samples := pics.Samples()
	content := l.clusterContent(first, last, pics.SampleLens())
	ts := s.StartMs(first)
	out.header(idCluster, content)
	out.uint(idTimestamp, uint64(ts))
	var block [4]byte
	block[0] = 0x81
	for i := first; i < last; i++ {
		sample := samples[s.SampleAt(i)]
		rel := s.StartMs(i) - ts
		block[1], block[2], block[3] = byte(rel>>8), byte(rel), 0
		if s.IsKey(i) {
			block[3] = 0x80
		}
		out.header(idSimpleBlock, uint64(4+len(sample)))
		out.write(block[:])
		out.write(sample)
	}
	return elementLen(idCluster, content)
}

// writeCues points at every cluster that opens with a key frame, each position
// in eight bytes, as the layout counted them.
func writeCues(out *sticky, l layout, keys []uint64) {
	s := l.stream
	out.header(idCues, l.cuesBody)
	positions := uintElementLen(idCueTrack, 1) + fixedUintElementLen(idCueClusterPosition)
	for g, pos := range keys {
		ts := uint64(s.StartMs(int64(g) * s.KeyEvery))
		out.header(idCuePoint, uintElementLen(idCueTime, ts)+elementLen(idCueTrackPositions, positions))
		out.uint(idCueTime, ts)
		out.header(idCueTrackPositions, positions)
		out.uint(idCueTrack, 1)
		out.write(appendFixedUint(out.scratch[:0], idCueClusterPosition, pos))
	}
}

// writeVoid writes one Void element of exactly total bytes, without ever
// holding its content.
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
