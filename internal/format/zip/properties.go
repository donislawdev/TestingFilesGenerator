package zip

import (
	"fmt"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading the settings a request carries.
//
// Split out of zip.go on 2026-08-26, when the guard against files creeping
// towards the size ceiling went red after the contains ceiling was added. The
// split is by subject rather than by line count - these two read a declared
// property out of the request and say so in the format's own voice, and
// nothing else in the package does that.

// sizeProperty reads a byte count written the way --size accepts it.
func sizeProperty(props map[string]string, key string, fallback int64) (int64, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := core.ParseSize(raw)
	if err != nil {
		return 0, fmt.Errorf("zip: %s: %w", key, err)
	}
	return n, nil
}

func intProperty(props map[string]string, key string, fallback, min, max int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("zip: %s must be a whole number, got %q", key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("zip: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return n, nil
}
