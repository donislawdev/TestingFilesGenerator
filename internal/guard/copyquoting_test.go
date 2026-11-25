package guard

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/commandline"
)

// A copied line is pasted into a real shell, so its quoting is asked of real
// shells rather than of a model of them. A small program stands in for tfg and
// prints every argument it was handed, the shells on this machine run a line
// written for them, and each value has to arrive exactly as it was typed.
//
// The values are the ones that broke something in a shell somewhere: quotes of
// every kind, the marks that start a second command, a variable, a pattern, a
// dash that makes a flag, and the stop-parsing token of PowerShell. Measured on
// 2026-11-25 before this was written - docs, the analysis of that day, section 2.
//
// Windows PowerShell 5.1 is asked separately. It hands a double quote and a
// last backslash after a space to a program differently from PowerShell 7, the
// line says so instead of pretending, and this guard holds both halves of that
// sentence: every value the line does NOT warn about arrives whole in 5.1, and
// every value it does warn about really arrives changed - a warning nobody
// needs any more is a sentence on the screen that is no longer true.
func TestEveryValueReachesTheProgramAsItWasWritten(t *testing.T) {
	program := echoProgram(t)
	values := hostileValues()
	ran := 0
	for _, dialect := range []commandline.Shell{commandline.PowerShell, commandline.POSIX} {
		for _, shell := range shellsFor(dialect) {
			ran++
			t.Run(filepath.Base(shell), func(t *testing.T) {
				if olderPowerShell(shell) {
					checkOlderPowerShell(t, shell, program, values)
					return
				}
				line := lineOf(t, program, dialect, values)
				expectArrived(t, shell, dialect, line.Text, wordsFor(values))
				canary(t, shell, dialect, program)
			})
		}
	}
	if ran == 0 {
		t.Skip("NOT CHECKED: no PowerShell and no sh on this machine. CI runs this on Windows, Linux and macOS, each with PowerShell 7.")
	}
}

// hostileValues is the table. Characters outside ASCII are written as numbers
// so that this file stays plain ASCII.
func hostileValues() []string {
	r := func(code rune) string { return string(code) }
	return []string{
		"plain", "with space", "two  spaces", " lead", "trail ", "it's", "'", "''", `a\'b`,
		"typographic " + r(0x2019) + " quote", r(0x2018) + "a" + r(0x2019), r(0x201A) + r(0x201B),
		r(0x201C) + "double" + r(0x201D), r(0x2013) + "x", r(0x2014) + "x",
		"$HOME", "$(echo x)", "${HOME}", "`x`", "a;b", "a|b", "a&b", "R&D", "a&&b", ">out", "<in", "2>&1",
		"#c", "@x", "@(1)", "{a}", "(x)", "a,b", "1,000",
		"-foo.txt", "--%", "--% x", "-", "--", "-5", "--x=y", "=x", "~/x", "~",
		"*", "?", "[a]", "!x", "!!", "%PATH%", "%", "50%",
		`C:\x\y`, `\\server\share`, `\`, "/usr/bin", "1mb", "1e3", "0x10", "1kb-8kb", "1..5", "+5", "a=b",
		"za" + r(0x017C) + r(0x00F3) + r(0x0142) + r(0x0107) + " g" + r(0x0119) + r(0x015B) + "l" + r(0x0105),
		`a"b`, `"quoted"`, `C:\My Dir\`,
	}
}

// lineOf writes one flag per value, each under a setting named after its
// place, so a warning can be traced back to the value it is about.
func lineOf(t *testing.T, program commandline.Program, s commandline.Shell, values []string) commandline.Written {
	t.Helper()
	l := commandline.New("generate")
	for i, v := range values {
		l.Flag("--v", v, fmt.Sprint(i))
	}
	w, err := l.Write(program, s)
	if err != nil {
		t.Fatalf("writing the line: %v", err)
	}
	return w
}

// wordsFor is what the program should receive for the values: the verb, then
// each value after its flag, or joined to it when the value starts with a dash.
func wordsFor(values []string) []string {
	words := []string{"generate"}
	for _, v := range values {
		if strings.HasPrefix(v, "-") {
			words = append(words, "--v="+v)
			continue
		}
		words = append(words, "--v", v)
	}
	return words
}

// checkOlderPowerShell holds the two halves of the warning in 5.1.
func checkOlderPowerShell(t *testing.T, shell string, program commandline.Program, values []string) {
	warned := map[string]bool{}
	for _, setting := range lineOf(t, program, commandline.PowerShell, values).OlderPowerShell {
		warned[setting] = true
	}
	if len(warned) == 0 {
		t.Fatalf("the line warns about none of the values, and the table holds three that 5.1 changes - so the warning is not reached")
	}
	var calm []string
	for i, v := range values {
		if !warned[fmt.Sprint(i)] {
			calm = append(calm, v)
			continue
		}
		one := lineOf(t, program, commandline.PowerShell, []string{v})
		if got := arrived(t, shell, commandline.PowerShell, one.Text); reflect.DeepEqual(got, wordsFor([]string{v})) {
			t.Errorf("Windows PowerShell 5.1 handed %q to the program whole, so the line warns about a value it does not change", v)
		}
	}
	expectArrived(t, shell, commandline.PowerShell, lineOf(t, program, commandline.PowerShell, calm).Text, wordsFor(calm))
}

// canary hands the shell a line written badly on purpose and asks that the
// value does NOT arrive whole. A guard that cannot see a broken line would pass
// every line.
func canary(t *testing.T, shell string, s commandline.Shell, program commandline.Program) {
	start, err := commandline.New().Write(program, s)
	if err != nil {
		t.Fatal(err)
	}
	bad, broken := start.Text+" --v -foo.txt", []string{"--v", "-foo.txt"}
	if s == commandline.POSIX {
		bad, broken = start.Text+" --v $HOME", []string{"--v", "$HOME"}
	}
	if got := arrived(t, shell, s, bad); reflect.DeepEqual(got, broken) {
		t.Errorf("CANARY: %s handed the badly written %q over whole, so this guard cannot tell a broken line from a good one", shell, bad)
	}
}

// expectArrived runs a line and compares what the program received.
func expectArrived(t *testing.T, shell string, s commandline.Shell, line string, want []string) {
	t.Helper()
	got := arrived(t, shell, s, line)
	if reflect.DeepEqual(got, want) {
		return
	}
	for i := 0; i < len(want) || i < len(got); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			t.Errorf("%s: argument %d arrived as %q, written as %q", shell, i, g, w)
		}
	}
	t.Logf("the line was:\n%s", line)
}

// arrived runs the line from a script file - a file rather than an argument,
// so nothing between this guard and the shell quotes it a second time - and
// answers the arguments the program printed.
func arrived(t *testing.T, shell string, s commandline.Shell, line string) []string {
	t.Helper()
	dir := t.TempDir()
	var cmd *exec.Cmd
	if s == commandline.PowerShell {
		// With a byte order mark, without which 5.1 reads the script in the
		// code page of the machine and changes every letter outside ASCII
		// before the line is even parsed.
		script := filepath.Join(dir, "line.ps1")
		writeScript(t, script, "\xef\xbb\xbf"+line+"\n")
		cmd = exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
	} else {
		script := filepath.Join(dir, "line.sh")
		writeScript(t, script, line+"\n")
		cmd = exec.Command(shell, script)
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s did not run the line: %v\n%s", shell, err, line)
	}
	var args []string
	for _, h := range strings.Fields(string(out)) {
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Fatalf("%s printed %q, which is not what the stand in prints", shell, h)
		}
		args = append(args, string(b))
	}
	return args
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// echoProgram builds the stand in for tfg next to a window that is not there,
// and finds it the way the window would. Asserted to be found BESIDE the
// window, because that is the path the line writes in full - the hardest word
// of the line to get right - and a stand in found any other way would leave it
// out.
func echoProgram(t *testing.T) commandline.Program {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "echo.go")
	writeScript(t, src, "package main\n\nimport (\n\t\"encoding/hex\"\n\t\"fmt\"\n\t\"os\"\n)\n\n"+
		"func main() {\n\tfor _, a := range os.Args[1:] {\n\t\tfmt.Println(hex.EncodeToString([]byte(a)))\n\t}\n}\n")
	name := "tfg"
	if runtime.GOOS == "windows" {
		name = "tfg.exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, name), src)
	build.Dir = dir
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOFLAGS=")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the stand in for tfg: %v\n%s", err, out)
	}
	p := commandline.Locate(commandline.System{
		GOOS:       runtime.GOOS,
		Getenv:     func(string) string { return "" },
		Executable: filepath.Join(dir, "tfg-gui"),
		Stat:       os.Stat,
	})
	if p.Where != commandline.Beside {
		t.Fatalf("the stand in at %s was not found beside the window (found: %v)", dir, p.Where)
	}
	return p
}

// shellsFor is every shell on this machine that reads a dialect. sh is not
// asked on Windows: the one there translates arguments that look like paths
// before a program sees them, which is not what a terminal on macOS or Linux
// does, and those systems run this guard in CI.
func shellsFor(s commandline.Shell) []string {
	names := []string{"pwsh"}
	if runtime.GOOS == "windows" {
		names = append(names, "powershell")
	}
	if s == commandline.POSIX {
		if runtime.GOOS == "windows" {
			return nil
		}
		names = []string{"sh", "bash", "zsh"}
	}
	var found []string
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// olderPowerShell reports whether a shell is Windows PowerShell 5.1, which is
// powershell.exe, where 7 is pwsh.
func olderPowerShell(shell string) bool {
	return strings.EqualFold(strings.TrimSuffix(filepath.Base(shell), ".exe"), "powershell")
}

// Where tfg is found, asked of machines described rather than of this one, so
// that every system is asked on every system. Each case carries the same
// machine without the file as its canary: found there too would mean the
// answer came from somewhere other than the file.
func TestTheLineStartsTfgTheWayThisComputerFindsIt(t *testing.T) {
	cases := []struct {
		label   string
		goos    string
		path    string
		exe     string
		file    string
		mode    fs.FileMode
		where   commandline.Where
		starts  string
		pathext string
	}{
		{"on PATH on Windows", "windows", `C:\Windows;C:\tools`, `C:\apps\tfg-gui.exe`, `C:\tools\tfg.exe`, 0o644, commandline.OnPath, "tfg", ""},
		{"the .cmd npm installs, in a quoted PATH entry", "windows", `C:\Windows;"C:\node bin"`, `C:\apps\tfg-gui.exe`, `C:\node bin\tfg.cmd`, 0o644, commandline.OnPath, "tfg", ".COM;.EXE;.BAT;.CMD"},
		{"beside the window on Windows", "windows", `C:\Windows`, `C:\apps\tfg\tfg-gui.exe`, `C:\apps\tfg\tfg.exe`, 0o644, commandline.Beside, `C:\apps\tfg\tfg.exe`, ""},
		{"beside the window in a folder with a space", "windows", ``, `C:\My Apps\tfg-gui.exe`, `C:\My Apps\tfg.exe`, 0o644, commandline.Beside, `& 'C:\My Apps\tfg.exe'`, ""},
		{"a relative PATH entry is the terminal's folder, not one to trust", "windows", `bin`, `C:\apps\tfg-gui.exe`, `bin\tfg.exe`, 0o644, commandline.Missing, "tfg", ""},
		{"on PATH on Linux", "linux", "/usr/bin:/usr/local/bin", "/opt/w/tfg-gui", "/usr/local/bin/tfg", 0o755, commandline.OnPath, "tfg", ""},
		{"a file nobody may run is not a program", "linux", "/usr/bin", "/opt/w/tfg-gui", "/usr/bin/tfg", 0o644, commandline.Missing, "tfg", ""},
		{"an empty PATH entry is the terminal's folder", "linux", ":/usr/bin", "/home/me/tfg/tfg-gui", "tfg", 0o755, commandline.Missing, "tfg", ""},
		{"beside the window on Linux", "linux", "/usr/bin", "/home/me/tfg/tfg-gui", "/home/me/tfg/tfg", 0o755, commandline.Beside, "/home/me/tfg/tfg", ""},
		{"beside the bundle on macOS", "darwin", "/usr/bin", "/Users/me/tfg/tfg-gui.app/Contents/MacOS/tfg-gui", "/Users/me/tfg/tfg", 0o755, commandline.Beside, "/Users/me/tfg/tfg", ""},
		{"beside the bundle in a folder with a space", "darwin", "/usr/bin", "/Users/me/My tfg/tfg-gui.app/Contents/MacOS/tfg-gui", "/Users/me/My tfg/tfg", 0o755, commandline.Beside, "'/Users/me/My tfg/tfg'", ""},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			sys := describedSystem(c.goos, c.path, c.pathext, c.exe, map[string]fs.FileMode{c.file: c.mode})
			p := commandline.Locate(sys)
			if p.Where != c.where {
				t.Fatalf("found %v, want %v", p.Where, c.where)
			}
			w, err := commandline.New("generate").Write(p, commandline.ShellFor(c.goos))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSuffix(w.Text, " generate"); got != c.starts {
				t.Errorf("the line starts %q, want %q", got, c.starts)
			}
			if c.where == commandline.Missing {
				return
			}
			if commandline.Locate(describedSystem(c.goos, c.path, c.pathext, c.exe, nil)).Where != commandline.Missing {
				t.Errorf("CANARY: the same machine without %s still finds tfg", c.file)
			}
		})
	}
}

func describedSystem(goos, path, pathext, exe string, files map[string]fs.FileMode) commandline.System {
	return commandline.System{
		GOOS: goos,
		Getenv: func(name string) string {
			switch name {
			case "PATH":
				return path
			case "PATHEXT":
				return pathext
			}
			return ""
		},
		Executable: exe,
		Stat: func(p string) (fs.FileInfo, error) {
			mode, ok := files[p]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return describedFile{mode: mode}, nil
		},
	}
}

// describedFile is a file that exists only in a case of the guard above.
type describedFile struct{ mode fs.FileMode }

func (f describedFile) Name() string       { return "tfg" }
func (f describedFile) Size() int64        { return 1 }
func (f describedFile) Mode() fs.FileMode  { return f.mode }
func (f describedFile) ModTime() time.Time { return time.Time{} }
func (f describedFile) IsDir() bool        { return false }
func (f describedFile) Sys() any           { return nil }

// A value no line can carry is refused under the setting it came from, and a
// value that starts with a dash with no flag to join it to is refused too.
func TestAValueNoLineCanCarryIsRefusedUnderItsSetting(t *testing.T) {
	missing := commandline.Program{}
	for _, c := range []struct {
		label string
		line  *commandline.Line
		why   commandline.Reason
	}{
		{"a line break", commandline.New("generate").Flag("--name", "a\nb", "name"), commandline.ControlCharacter},
		{"a tab", commandline.New("generate").Flag("--name", "a\tb", "name"), commandline.ControlCharacter},
		{"a line separator", commandline.New("generate").Flag("--name", "a"+string(rune(0x2028))+"b", "name"), commandline.ControlCharacter},
		{"bytes that are not text", commandline.New("generate").Flag("--name", "a\xffb", "name"), commandline.NotText},
		{"an argument starting with a dash", commandline.New("tool", "checksum").Argument("-x", "name"), commandline.LeadingDash},
	} {
		for _, s := range []commandline.Shell{commandline.PowerShell, commandline.POSIX} {
			_, err := c.line.Write(missing, s)
			var u *commandline.Unwritable
			if !errors.As(err, &u) || u.Reason != c.why || u.Setting != "name" {
				t.Errorf("%s: Write answered %v, want a refusal of kind %d under name", c.label, err, c.why)
			}
		}
	}
	if _, err := commandline.DamageValue("zero-head", map[string]string{"bytes": "1,6"}); err == nil {
		t.Errorf("a comma inside a setting of a damage was written, and --damage would split it in two")
	}
	if got, _ := commandline.DamageValue("zero-head", map[string]string{"z": "1", "bytes": "16"}); got != "zero-head:bytes=16,z=1" {
		t.Errorf("DamageValue wrote %q, want the settings in the order of their names", got)
	}
}
