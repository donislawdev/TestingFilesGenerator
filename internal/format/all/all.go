// Package all registers every built in format.
//
// Importing it once is what puts the registry together. Without a single
// place doing this, each entry point would carry its own list and they would
// drift apart - one surface would know about a format the other did not,
// which is exactly the split the shared engine exists to prevent.
package all

import (
	// Each format registers itself when its package is loaded.
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/avif"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/bmp"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/csvfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/docx"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/gif"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/htmlfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/ico"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/jpg"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/jsonfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/logfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/md"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/pdf"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/png"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/pptx"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/svgfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/targz"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/tiff"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/txt"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/wav"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/webp"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/xlsx"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/xmlfile"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/zip"
)
