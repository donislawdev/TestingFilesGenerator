// Package all registers every built in tool.
//
// The same arrangement as internal/format/all, for the same reason: one place
// that puts the registry together, imported by both surfaces, so neither can
// know a tool the other does not.
package all

import (
	// Each tool registers itself when its package is loaded.
	_ "github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)
