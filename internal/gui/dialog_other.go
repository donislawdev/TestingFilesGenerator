//go:build !windows

package gui

// sayInADialog has nothing to add outside Windows: a binary there keeps its
// standard error, so the sentence that the window could not open reaches
// the terminal it was started from. A person who started it from a file
// manager sees nothing, and that is written down as not done rather than
// papered over - the system dialog that would answer it differs by desktop.
func sayInADialog(string, string) {}
