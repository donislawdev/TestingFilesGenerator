package guard

import "github.com/donislawdev/TestingFilesGenerator/internal/engine"

// cutLabelGoldenCases are pictures too low for the whole of their label, one
// per format that draws it into pixels.
//
// Wide enough that the label is drawn, so the band and the top rows of the
// text are in the picture, and too low for the rest - four rows, where the
// text starts on the third. Until 2026-10-27 the manifest said such a picture
// carried its label. It now says it does not, and the owner's decision was
// that the pixels stay as they were (docs/STABILITY-REVIEW-2026-10-26.md
// section 4a.2). These values were measured on the code from before that
// change, so they are the proof that the manifest moved and the bytes did not.
//
// A film is cut at the same place by a different arithmetic - it is wide, so
// its label is drawn four times larger and needs 22 rows of its 16.
func cutLabelGoldenCases() map[string]engine.Target {
	low := func(format, width, height string, size int64) engine.Target {
		return engine.Target{ID: "g", Format: format, Sizes: engine.Uniform(1, size), Label: true,
			Properties: map[string]string{"width": width, "height": height}}
	}
	film := func(format string) engine.Target {
		t := low(format, "1920", "16", 65536)
		t.Properties["duration"] = "1s"
		return t
	}
	return map[string]engine.Target{
		"avif_label_cut": low("avif", "512", "4", 16384),
		"bmp_label_cut":  low("bmp", "512", "4", 16384),
		"gif_label_cut":  low("gif", "512", "4", 16384),
		"ico_label_cut":  low("ico", "256", "4", 16384),
		"jpg_label_cut":  low("jpg", "512", "4", 16384),
		"jxl_label_cut":  low("jxl", "512", "4", 16384),
		"png_label_cut":  low("png", "512", "4", 16384),
		"tiff_label_cut": low("tiff", "512", "4", 16384),
		"webp_label_cut": low("webp", "512", "4", 16384),
		"mp4_label_cut":  film("mp4"),
		"webm_label_cut": film("webm"),
	}
}
