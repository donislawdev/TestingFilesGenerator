// Package all registers every built in format.
//
// Importing it once is what puts the registry together. Without a single
// place doing this, each entry point would carry its own list and they would
// drift apart - one surface would know about a format the other did not,
// which is exactly the split the shared engine exists to prevent.
package all

import (
	// Each format registers itself when its package is loaded.
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/pdf"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/png"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/txt"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/wav"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/zip"
)
