//go:build !linux && !darwin && !windows

package core

import "errors"

// renameNoReplace has no call behind it on a system this project does not
// build for, so Publish goes straight to its fallbacks there.
func renameNoReplace(from, to string) error { return errors.ErrUnsupported }

// linkUnsupported knows only the answer Publish's own chain gives.
func linkUnsupported(err error) bool { return errors.Is(err, errors.ErrUnsupported) }
