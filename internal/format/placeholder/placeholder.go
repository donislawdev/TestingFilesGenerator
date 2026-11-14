// Package placeholder draws the picture a designer or a front end puts where a
// real image will go: a flat background, what the picture is for written
// large across the middle, and on request the guides that show after an
// upload whether the picture was cropped or stretched.
//
// Every picture format of this build draws it through here, so the look, the
// settings and what reaches the manifest are one declaration rather than ten
// kept alike (docs/PLACEHOLDER-2026-11-14.md). The letters are Inter, as
// outlines in package glyphs, and are drawn by package raster with whole
// numbers only, so the pixels are the same on every system (D11).
package placeholder
