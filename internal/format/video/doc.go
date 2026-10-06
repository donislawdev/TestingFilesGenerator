// Package video is what the video formats share: the timeline, the pictures,
// and the AV1 stream they carry. WebM and MP4 are thin containers around it,
// the way docx, xlsx and pptx are thin around opc.
//
// The coding is gav1d's, and gav1d codes still pictures only - its own header
// says "one all-intra key frame" and declares a reduced still picture header.
// So this package keeps gav1d's tile data and writes everything around it
// itself: a full sequence header, the frame headers, and the frames that show
// a picture again without coding it again. The probe that settled this and
// every measurement under it is docs/WIDEO-2026-10-06.md sections 9 to 11.
//
// A film shows a new picture at every change - the clock in it moves on and a
// square takes a step, every change_interval (section 15) - and between
// changes it shows the same picture again. Each picture is coded on its own,
// because nothing in gav1d predicts one picture from another. The stream has
// one shape, and the shape is the point:
//
//	a key frame, shown          - where a player can start
//	the picture as a hidden     - an intra only frame, not shown, kept in a
//	  copy, then shown              slot, and the frame that shows it: after
//	                                each key frame and at each change
//	show_existing_frame         - every other frame, three bytes each
//
// A shown key frame cannot be shown a second time - the specification sets its
// showable_frame to 0 and libaom refuses the stream that tries ("Buffer does
// not contain a showable frame"). gav1d's own decoder and Chromium both play
// that stream without a word, which is why neither of them is a witness here.
// The hidden copy after a key frame costs the picture's bytes once more and no
// second encoding, because it carries the very same tile.
//
// That shape is what makes a length independent of the size: frames between
// changes are three bytes each, so the bytes go to the pictures, and a film
// that changes rarely or never is small whatever its length.
package video
