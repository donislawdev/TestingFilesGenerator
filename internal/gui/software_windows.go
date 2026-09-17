//go:build windows

package gui

import (
	"os"
	"syscall"
)

// LoadSoftwareRenderer loads the renderer shipped beside the executable, so
// that the toolkit's later request for OpenGL by name is answered by it.
//
// Called before the toolkit exists, because the loader hands a library
// already mapped under a name to every later request for that name - and
// the window binary no longer imports opengl32.dll at load time (see
// third_party/go-gl-gl/PATCH.md), so at this point nothing of that name is
// mapped yet and the first request is this one.
//
// By absolute path, worked out from where the executable is, which is the
// same trust the executable itself already has. syscall.LoadDLL is the only
// loader syscall offers and it goes through LoadLibraryW, which for a path
// loads that file and nothing else. The renderer's own dependency is then
// found by name, which is why the renderer goes first - SoftwareFiles says
// so and every caller takes the order from it.
//
// The driver variable is set in this process before the first load, because
// the renderer reads it when it initialises. The first process sets it for
// the second as well, in the environment it hands over, and both are needed:
// the second is what a person gets when they ask for the flag by hand.
func LoadSoftwareRenderer(exeDir string) error {
	if err := os.Setenv(softwareDriver, softwareLLVMPipe); err != nil {
		return loadFailed{softwareDriver, err}
	}
	for _, path := range SoftwareFiles(exeDir) {
		// The toolchain's own error names the file and repeats what the
		// system said, so it is passed on as it is rather than wrapped in
		// a second copy of the path.
		if _, err := syscall.LoadDLL(path); err != nil {
			return err
		}
	}
	return nil
}
