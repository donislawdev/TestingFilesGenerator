package core

import "os"

// pathExists says whether something is at path. Used when walking up to find
// a directory that already exists, so the free space check works for an
// output directory that has not been created yet.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
