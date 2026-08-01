// Package engine plans a run, writes the files and backs verify and cleanup.
//
// It knows nothing about the command line or the window. That rule erodes one
// exception at a time, so a test enforces it instead of good intentions.
package engine
