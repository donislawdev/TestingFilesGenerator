package gui

import (
	"errors"
	"os"
	"os/exec"
)

// startAgain starts this executable again with args, waits for it, and
// answers its exit code.
//
// The one place in the window binary that starts a process, and the process
// it starts is itself: the path comes from os.Executable and nowhere else,
// so nothing on the search path and nothing beside the program can be what
// runs. The guard over shipped code that refuses every other spawn names
// this file for that reason - notelemetry_test.go.
//
// The second process inherits the streams of the first, so whatever started
// this program reads what the window says wherever it is said. Under the
// windows subsystem those streams are not there, and the toolchain skips a
// handle that is not there rather than failing on it - measured in the
// syscall package rather than assumed, because a fallback that could not
// start from a double click would be a fallback for the terminal only.
//
// The driver variable is handed over as well, so that the renderer in the
// second process reads it however it reads its environment - the second
// process sets it for itself too, and this is the half that does not depend
// on when the C runtime takes its copy.
func startAgain(args []string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), softwareDriver+"="+softwareLLVMPipe)
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// The second process ran and answered. Its code is the answer, and
		// a non zero one is a refusal it has already put in front of the
		// person, not an error of ours to wrap.
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}
