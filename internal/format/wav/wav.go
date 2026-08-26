// Package wav generates WAV audio.
//
// The one format whose size follows from the parameters of the signal rather
// than from any content - sample rate, bit depth, channels and length settle
// it before a single sample exists.
package wav

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	generatorVersion = "1"

	// A RIFF header is twelve bytes, the format chunk twenty four, and the
	// data chunk header eight. Forty four is the number every WAV starts
	// with.
	riffHeader = 12
	fmtChunk   = 24
	dataHeader = 8
	baseSize   = riffHeader + fmtChunk + dataHeader

	// chunkHeader is the length and identifier every RIFF chunk carries.
	chunkHeader = 8

	// maxFileBytes is the largest file this format can describe honestly.
	//
	// Three lengths go into a WAV as four byte fields - the RIFF size, the
	// data chunk and the JUNK chunk - and until 2026-08-26 nothing checked
	// them. A request for 8 GiB produced a file of exactly 8 GiB whose header
	// announced 4 GiB, and every part of this tool agreed it was fine: the
	// size guard compares the writer's count against the plan and both said
	// 8 GiB, the hash went into the manifest, and verify called it a match.
	// A corrupt file that the tool actively certifies is worse than a loud
	// failure, which is why this is a refusal rather than a note.
	//
	// The number is not 1<<32-1 as it is in BMP and ICO, and the difference
	// is measured rather than tidy. The RIFF field counts everything AFTER
	// itself, so it holds total-8: a file of exactly 4 GiB writes 4294967288
	// and comes back correct. Rounding this down to the BMP number would
	// refuse eight sizes this format delivers perfectly well, and the rule
	// that the number a tool announces is the number it accepts cuts both
	// ways. The data and JUNK lengths are each smaller than the RIFF one, so
	// this single bound covers all three.
	maxFileBytes = math.MaxUint32 + 8

	writeChunk = 32 * 1024

	defaultRate     = 44100
	defaultBits     = 16
	defaultChannels = 2
)

// The padding channel is a JUNK chunk placed after the data, and the final
// alignment byte is left off when its payload is odd.
//
// RIFF pads an odd length chunk to an even boundary, which means every chunk
// contributes an even number of bytes and a strictly padded file always has
// an even size. That would put every odd size out of reach - and the boundary
// set, which asks for limit-1, limit and limit+1, needs consecutive sizes.
//
// Measured on four independent parsers - the Python wave module, FFmpeg, the
// .NET SoundPlayer and Windows Media Foundation. All four read the file, the
// frame count and the duration, with the padding chunk last and its payload
// odd, at sizes from one byte up to a hundred kilobytes.
//
// Putting the chunk before the data works too and is the more conventional
// spot, but there the alignment byte cannot be dropped, so odd sizes stay
// unreachable. That is why it goes last.

func init() {
	format.Register(format.Descriptor{
		ID:          "wav",
		Extension:   ".wav",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    baseSize,

		Padding: format.PaddingChannel{
			Name:     "JUNK chunk after the audio data",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelInternal,
		Oracle: "ffprobe",
		Properties: []format.Property{
			{
				Name: "sample_rate", Kind: format.PropertyInt,
				Min: 8000, Max: 192000, Unit: "hertz",
				Default: strconv.Itoa(defaultRate),
				Detail:  "How many samples a second the signal carries.",
			},
			{
				Name: "bit_depth", Kind: format.PropertyChoice,
				// A closed set rather than a range, because it is one. As a
				// range 8 to 32 a request for 20 passed the first check and
				// was refused later, in different words.
				Choices: []string{"8", "16", "24", "32"},
				Default: strconv.Itoa(defaultBits),
				Detail:  "How many bits each sample uses.",
			},
			{
				Name: "channels", Kind: format.PropertyInt,
				Min: 1, Max: 8,
				Default: strconv.Itoa(defaultChannels),
				Detail:  "How many channels the signal has, so 1 for mono and 2 for stereo.",
			},
			{
				Name: "content", Kind: format.PropertyChoice,
				Choices: []string{"tone", "silence", "noise", "sweep"},
				Default: "tone",
				Detail:  "What the signal sounds like.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	rate     int
	bits     int
	channels int
	content  string
	seed     uint64

	frames  int64
	dataLen int64
	info    []byte // the LIST INFO chunk carrying the label, empty when absent
	junkLen int64  // payload of the padding chunk
	withPad bool
}

func (m memo) frameSize() int64 { return int64(m.channels) * int64(m.bits) / 8 }

// readProperties turns the request properties into the part of the memo that
// describes the sound itself.
func readProperties(r format.Request) (memo, error) {
	m := memo{seed: r.Seed}
	var err error

	if m.rate, err = intProperty(r.Properties, "sample_rate", defaultRate, 8000, 192000); err != nil {
		return memo{}, err
	}
	if m.channels, err = intProperty(r.Properties, "channels", defaultChannels, 1, 8); err != nil {
		return memo{}, err
	}
	if m.bits, err = intProperty(r.Properties, "bit_depth", defaultBits, 8, 32); err != nil {
		return memo{}, err
	}
	switch m.bits {
	case 8, 16, 24, 32:
	default:
		return memo{}, fmt.Errorf("wav: bit_depth must be 8, 16, 24 or 32, got %d", m.bits)
	}

	m.content = "tone"
	if v, ok := r.Properties["content"]; ok && v != "" {
		switch v {
		case "tone", "silence", "noise", "sweep":
			m.content = v
		default:
			return memo{}, fmt.Errorf("wav: content must be tone, silence, noise or sweep, got %q", v)
		}
	}
	return m, nil
}

// layout splits what is left after the headers between whole audio frames and
// the padding chunk.
//
// The audio takes as much of the file as whole frames allow, and the padding
// chunk takes the remainder. Working the other way round - fixing the length
// first - would leave most sizes unreachable, because a frame is two to sixteen
// bytes and the remainder rarely lands on a boundary.
func layout(m *memo, r format.Request, fixed int64) error {
	free := r.Bytes - fixed
	fs := m.frameSize()

	if free%fs == 0 {
		// The audio fills the file exactly. No padding chunk at all.
		m.frames = free / fs
		m.dataLen = free
		return nil
	}

	// Leave room for the padding chunk, then give the rest to the audio.
	if free < chunkHeader {
		// Too little room for both a frame and a chunk header, so the file is
		// header only plus padding.
		m.frames = 0
		m.dataLen = 0
		m.withPad = true
		m.junkLen = free - chunkHeader
		if m.junkLen < 0 {
			return &format.BelowMinimumError{
				Format:    "WAV",
				Requested: r.Bytes,
				Minimum:   fixed + chunkHeader,
				Reason: fmt.Sprintf(
					"this size needs a padding chunk and the smallest one costs %d B, so nothing between %d and %d B can be reached",
					chunkHeader, fixed, fixed+chunkHeader),
				Hint: fmt.Sprintf("Ask for exactly %d B or for %d B or more.", fixed, fixed+chunkHeader),
			}
		}
		return nil
	}

	audio := free - chunkHeader
	m.frames = audio / fs
	m.dataLen = m.frames * fs
	m.withPad = true
	m.junkLen = free - m.dataLen - chunkHeader
	return nil
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	m, err := readProperties(r)
	if err != nil {
		return format.Plan{}, err
	}

	if r.Label {
		m.info = infoChunk(core.Label("wav", r.Bytes, r.Seed))
	}

	fixed := int64(baseSize) + int64(len(m.info))

	if r.Bytes < fixed {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "WAV",
			Requested: r.Bytes,
			Minimum:   fixed,
			Reason:    "a WAV needs its RIFF header, its format chunk and a data chunk header before a single sample",
			Hint:      fmt.Sprintf("Ask for %d B or more%s.", fixed, labelHint(r.Label)),
		}
	}

	if r.Bytes > maxFileBytes {
		return format.Plan{}, &format.AboveMaximumError{
			Format:    "WAV",
			Requested: r.Bytes,
			Maximum:   maxFileBytes,
			Reason:    "a RIFF file states its own length in a four byte field, so the format cannot describe a file this large",
			Hint:      "Ask for 4 GiB or less, or pick a format with no length field of its own such as txt.",
		}
	}

	if err := layout(&m, r, fixed); err != nil {
		return format.Plan{}, err
	}

	durationMs := float64(m.frames) / float64(m.rate) * 1000

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"sample_rate":                m.rate,
			"bit_depth":                  m.bits,
			"channels":                   m.channels,
			"content":                    m.content,
			"frames":                     m.frames,
			"duration_ms":                math.Round(durationMs*1000) / 1000,
			format.PropertyLabelEmbedded: len(m.info) > 0,
		},
		Memo: m,
	}
	if m.frames == 0 {
		p.Notes = append(p.Notes, format.Note{
			Code: "no_audio_frames",
			Detail: fmt.Sprintf(
				"At %d B this file has room for the headers but not for a single audio frame, so it holds no sound. It is still a valid WAV.",
				r.Bytes),
		})
	}
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("wav: the plan was not produced by this generator")
	}

	total := p.Bytes
	// The RIFF size field counts everything after itself.
	if err := writeAll(w, []byte("RIFF")); err != nil {
		return err
	}
	if err := putU32(w, uint32(total-8)); err != nil {
		return err
	}
	if err := writeAll(w, []byte("WAVE")); err != nil {
		return err
	}

	if err := m.writeFmt(w); err != nil {
		return err
	}
	if len(m.info) > 0 {
		if err := writeAll(w, m.info); err != nil {
			return err
		}
	}

	if err := writeAll(w, []byte("data")); err != nil {
		return err
	}
	if err := putU32(w, uint32(m.dataLen)); err != nil {
		return err
	}
	if err := m.writeSamples(ctx, w); err != nil {
		return err
	}

	if m.withPad {
		if err := writeAll(w, []byte("JUNK")); err != nil {
			return err
		}
		if err := putU32(w, uint32(m.junkLen)); err != nil {
			return err
		}
		// The alignment byte an odd payload would normally get is left off,
		// because this is the last chunk in the file and adding it would put
		// every odd size out of reach. Measured against four parsers.
		if err := fill(ctx, w, m.seed, m.junkLen); err != nil {
			return err
		}
	}
	return nil
}

func (m memo) writeFmt(w io.Writer) error {
	if err := writeAll(w, []byte("fmt ")); err != nil {
		return err
	}
	if err := putU32(w, 16); err != nil {
		return err
	}
	fs := m.frameSize()
	var b [16]byte
	binary.LittleEndian.PutUint16(b[0:], 1) // PCM
	binary.LittleEndian.PutUint16(b[2:], uint16(m.channels))
	binary.LittleEndian.PutUint32(b[4:], uint32(m.rate))
	binary.LittleEndian.PutUint32(b[8:], uint32(int64(m.rate)*fs))
	binary.LittleEndian.PutUint16(b[12:], uint16(fs))
	binary.LittleEndian.PutUint16(b[14:], uint16(m.bits))
	return writeAll(w, b[:])
}

func (m memo) writeSamples(ctx context.Context, w io.Writer) error {
	if m.dataLen == 0 {
		return nil
	}
	bytesPerSample := m.bits / 8
	rng := core.NewRand(m.seed)
	buf := make([]byte, 0, writeChunk+16)

	// A fixed pitch, nudged by the seed so two seeds sound different.
	base := 220.0 + float64(m.seed%880)

	var written int64
	for frame := int64(0); frame < m.frames; frame++ {
		if frame%4096 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		t := float64(frame) / float64(m.rate)
		var v float64
		switch m.content {
		case "silence":
			v = 0
		case "noise":
			v = rng.Float64()*2 - 1
		case "sweep":
			v = math.Sin(2 * math.Pi * (base + base*t) * t)
		default:
			v = math.Sin(2 * math.Pi * base * t)
		}

		for c := 0; c < m.channels; c++ {
			buf = appendSample(buf, v, bytesPerSample)
		}
		written += m.frameSize()

		if len(buf) >= writeChunk {
			if err := writeAll(w, buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		if err := writeAll(w, buf); err != nil {
			return err
		}
	}
	if written != m.dataLen {
		return fmt.Errorf("wav: wrote %d B of audio where the plan said %d B", written, m.dataLen)
	}
	return nil
}

// appendSample writes one sample at the requested width. Eight bit WAV is
// unsigned and everything wider is signed - a quirk of the format, not a
// choice.
func appendSample(buf []byte, v float64, width int) []byte {
	if v > 1 {
		v = 1
	} else if v < -1 {
		v = -1
	}
	switch width {
	case 1:
		return append(buf, byte(int(v*127)+128))
	case 2:
		n := int16(v * 32767)
		return append(buf, byte(n), byte(n>>8))
	case 3:
		n := int32(v * 8388607)
		return append(buf, byte(n), byte(n>>8), byte(n>>16))
	default:
		n := int32(v * 2147483647)
		return append(buf, byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
	}
}

// infoChunk builds a LIST INFO chunk carrying the label in the software
// field, which is where a player looks for it.
func infoChunk(label string) []byte {
	text := append([]byte(label), 0)
	if len(text)%2 == 1 {
		text = append(text, 0)
	}
	inner := make([]byte, 0, 4+chunkHeader+len(text))
	inner = append(inner, []byte("INFO")...)
	inner = append(inner, []byte("ISFT")...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(text)))
	inner = append(inner, n[:]...)
	inner = append(inner, text...)

	out := make([]byte, 0, chunkHeader+len(inner))
	out = append(out, []byte("LIST")...)
	binary.LittleEndian.PutUint32(n[:], uint32(len(inner)))
	out = append(out, n[:]...)
	out = append(out, inner...)
	return out
}

func fill(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n <= 0 {
		return nil
	}
	rng := core.NewRand(seed)
	buf := make([]byte, writeChunk)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		chunk := buf[:size]
		for i := range chunk {
			chunk[i] = byte(rng.UintN(256))
		}
		if err := writeAll(w, chunk); err != nil {
			return err
		}
		remaining -= size
	}
	return nil
}

func putU32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return writeAll(w, b[:])
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

func intProperty(props map[string]string, key string, fallback, min, max int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("wav: %s must be a whole number, got %q", key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("wav: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return n, nil
}

func labelHint(label bool) string {
	if label {
		return ", or drop the label"
	}
	return ""
}
