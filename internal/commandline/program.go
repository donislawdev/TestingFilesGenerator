package commandline

import (
	"io/fs"
	"strings"
)

// Name is what the command line is called, and what a line starts with when
// the shell can find it by that name.
const Name = "tfg"

// Where is where a line found tfg on this computer.
type Where int

const (
	// Missing is no tfg anywhere the window looked. The line still starts
	// with the name, and the window says the command line is not installed.
	Missing Where = iota
	// OnPath is a tfg the shell finds by its name.
	OnPath
	// Beside is a tfg next to the window and not on PATH - the two archives
	// unpacked into one folder - and the line names it by its full path.
	Beside
)

// Program is how a line starts tfg.
type Program struct {
	Where Where
	path  string
}

// System is what Locate reads of the computer, handed in so that a guard can
// ask about a Windows machine from any machine.
type System struct {
	// GOOS is the name Go gives the system: windows, darwin, linux.
	GOOS string
	// Getenv reads PATH, and PATHEXT on Windows.
	Getenv func(string) string
	// Executable is the full path of the running window.
	Executable string
	// Stat follows links, the way the shell does when it starts a program.
	Stat func(string) (fs.FileInfo, error)
}

// Locate finds tfg the way the shell will, and then next to the window.
//
// The window and the command line are separate downloads on every system, so
// the window cannot know where tfg is without asking. Asked by reading PATH and
// the file system rather than by starting anything, so this package stays a
// writer and the window keeps its one approved process (internal/gui/again.go).
//
// A relative entry of PATH is skipped, and so is the current directory: both
// are read from wherever the person's terminal is, which is not where the window
// is - Go's own exec.LookPath refuses them for the same reason since 1.19.
func Locate(sys System) Program {
	if onPath(sys) {
		return Program{Where: OnPath}
	}
	for _, dir := range besideDirs(sys) {
		p := join(sys.GOOS, dir, programFile(sys.GOOS))
		if runnable(sys, p) {
			return Program{Where: Beside, path: p}
		}
	}
	return Program{Where: Missing}
}

// word is how a line starts this program.
//
// In PowerShell a quoted path is a string rather than a command, and the call
// operator in front is what runs it. A path with nothing to quote runs as it is.
func (p Program) word(s Shell) string {
	if p.Where != Beside {
		return Name
	}
	quoted := quote(p.path, s)
	if s == PowerShell && quoted != p.path {
		return "& " + quoted
	}
	return quoted
}

// onPath reports whether the shell finds tfg by its name.
func onPath(sys System) bool {
	separator := ":"
	if sys.GOOS == "windows" {
		separator = ";"
	}
	for _, dir := range strings.Split(sys.Getenv("PATH"), separator) {
		// Windows allows an entry in quotes, for a folder with a separator in
		// its name, and the shell reads it without them.
		dir = strings.Trim(dir, `"`)
		if absolute(sys.GOOS, dir) && holdsTfg(sys, dir) {
			return true
		}
	}
	return false
}

// holdsTfg reports whether a folder holds a file the shell would take for tfg.
func holdsTfg(sys System, dir string) bool {
	for _, name := range namesOnPath(sys) {
		if runnable(sys, join(sys.GOOS, dir, name)) {
			return true
		}
	}
	return false
}

// namesOnPath is every file name the shell would take for tfg.
//
// On Windows that is the name with each ending PATHEXT lists, which is how the
// .cmd that npm installs is found as well as the .exe of an archive. Elsewhere
// it is the name alone.
func namesOnPath(sys System) []string {
	if sys.GOOS != "windows" {
		return []string{Name}
	}
	endings := sys.Getenv("PATHEXT")
	if endings == "" {
		endings = ".COM;.EXE;.BAT;.CMD"
	}
	var names []string
	for _, e := range strings.Split(endings, ";") {
		if e != "" {
			names = append(names, Name+strings.ToLower(e))
		}
	}
	return names
}

// programFile is the file a release archive holds tfg in.
func programFile(goos string) string {
	if goos == "windows" {
		return Name + ".exe"
	}
	return Name
}

// besideDirs is where tfg would be if it was unpacked next to the window.
//
// On macOS the window runs from inside its bundle, tfg-gui.app/Contents/MacOS,
// and the archive of the command line puts a link called tfg next to its own
// bundle - so the folder that holds the window's bundle comes first. The folder
// of the program itself is asked as well, which is where a build from source
// puts both.
func besideDirs(sys System) []string {
	dir := parent(sys.GOOS, sys.Executable)
	if dir == "" {
		return nil
	}
	const inside = "/Contents/MacOS"
	if sys.GOOS == "darwin" && strings.HasSuffix(dir, ".app"+inside) {
		bundle := strings.TrimSuffix(dir, inside)
		return []string{parent(sys.GOOS, bundle), dir}
	}
	return []string{dir}
}

// runnable reports whether a file is there and the system would start it: any
// file of a listed ending on Windows, a file with a bit for running it
// elsewhere.
func runnable(sys System, path string) bool {
	info, err := sys.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return sys.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}

// absolute reports whether a path names the same place from any directory.
func absolute(goos, path string) bool {
	if goos != "windows" {
		return strings.HasPrefix(path, "/")
	}
	if strings.HasPrefix(path, `\\`) {
		return true
	}
	return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

// join puts a name in a folder with the separator of the named system, which
// is not filepath.Join: that one joins for the system the guard runs on.
func join(goos, dir, name string) string {
	if goos == "windows" {
		return strings.TrimRight(dir, `\/`) + `\` + name
	}
	return strings.TrimRight(dir, "/") + "/" + name
}

// parent is the folder a path is in, by the separators of the named system.
func parent(goos, path string) string {
	cut := strings.LastIndex(path, "/")
	if goos == "windows" {
		cut = strings.LastIndexAny(path, `\/`)
	}
	if cut <= 0 {
		return ""
	}
	return path[:cut]
}
