package bmp

import "github.com/donislawdev/TestingFilesGenerator/internal/format"

// splitForACut splits the padding of a BMP around its pixels for a cut after
// its first cutAt bytes (format.Request.CutAt), so that the cut keeps the
// share of the pixels it keeps of the whole file: in front of them, in the gap
// bfOffBits points past, as much as the share wants, and the rest behind them,
// where no field points. It is raw bytes on either side, so every length
// lands. Ten decoders give the pixels of a BMP nothing cuts from the gap split
// and from all of it behind, Go's x/image/bmp only with no gap in front - it
// refuses every BMP with one (docs/TRUNCATE-2026-11-11.md section 15.3).
//
// A BMP nothing cuts keeps all of its padding in front, byte for byte what it
// was (D11).
func splitForACut(m *memo, total, cutAt int64) {
	pad := m.gap
	m.gap = format.SplitFront(pad, headers, pixelBytes(m.width, m.height), cutAt, total, 1)
	m.back = pad - m.gap
}
