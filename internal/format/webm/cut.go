package webm

import (
	"context"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// A film a damage will cut (format.Request.CutAt) has its padding split around
// the clusters, so that the cut keeps the share of the clusters it keeps of the
// whole file: a Void between the Tracks and the first cluster, and the rest in
// the Void before the Cues - none there when the share wants all of it in
// front, and the Cues follow the last cluster.
//
// With all of the padding behind the clusters, as an uncut film has it, a
// large film cut at any share kept every picture and lost only padding. A Void
// is an EBML element allowed at every level, and readers skip it in front of
// the clusters as they do behind them - mkvmerge writes one there
// (docs/TRUNCATE-2026-11-11.md section 14.4). The one thing a Void there
// changes is where every cluster starts, which the Cues say from the start of
// the segment's content, and they are written last from where the clusters
// went. The SeekHead's places stay: Info and Tracks stand in front of the
// Void, and the Cues end the file.
//
// How much goes in front depends on how long the clusters come to, and a
// cluster's length is known only once its picture is coded. So a film a damage
// will cut is coded twice, as every MP4 is: once for the lengths (measure) and
// again to write it, the second pass taking every tile the first one kept
// (video.Pictures.Again). A film nothing cuts is coded once, as before.

// measure is the first pass of a film a damage will cut: every cluster's
// length, nothing written, so the padding can be split before the file has a
// byte. It gives back what the clusters come to and the coder of the second
// pass. The work coding is worth is reported here, and the second pass, which
// codes only what this one could not keep, reports none.
//
// first is closed on every way out, by Again or by the caller's deferred
// Close - closing it twice is harmless.
func measure(ctx context.Context, l layout, first *video.Pictures) (int64, *video.Pictures, error) {
	var content uint64
	err := eachCluster(ctx, l, first, true, func(from, to int64) error {
		content += elementLen(idCluster, l.clusterContent(from, to, first.SampleLens()))
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	again, err := first.Again()
	if err != nil {
		return 0, nil, err
	}
	return int64(content), again, nil
}

// splitForACut is how much of the padding pad goes in front of the clusters,
// which come to content bytes after before bytes of the file, for a cut after
// the file's first cutAt bytes of whole. Each side is nothing or a Void of at
// least its two bytes (format.SplitFront).
func splitForACut(pad, before, content, cutAt, whole int64) int64 {
	return format.SplitFront(pad, before, content, cutAt, whole, minVoid)
}
