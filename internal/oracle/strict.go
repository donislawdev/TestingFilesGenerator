package oracle

// The structural checker: the layer written to the specification rather than
// borrowed from somebody's reader. It stood at the end of oracle.go until
// 2026-11-11, when the list of formats it covers took that file past the
// length a person can follow.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Strict runs the structural checker written against the specification.
//
// The tolerant readers above answer "would a real viewer accept this". This
// answers "is it actually well formed". Measured, the two are different
// questions: ffprobe ignores a wrong size in the RIFF header, Pillow does not
// verify the checksum of an ancillary PNG chunk, and a PDF reader rebuilds a
// cross reference table whose offset is off by one.
//
// It is written in another language, to the specification, so it is not our
// own code judging our own code.
// The optional settings are handed to the checker after the path, as
// key=value words. They exist for the formats whose file SHAPE is a setting -
// a CSV separated by semicolons is well formed and a checker told to split on
// commas would call it a single column table. The checker is told rather than
// left to work it out, because a checker that guesses the separator would
// happily agree with a file that used the wrong one, which is the question a
// guard has to answer instead.
func Strict(formatID, path string, settings ...string) Result {
	python, ok := inPath("python")()
	if !ok {
		return Result{Available: false, Tool: "strict structural check"}
	}
	script, ok := strictScriptPath()
	if !ok {
		return Result{Available: false, Tool: "strict structural check"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	//nolint:gosec // same as above - our own script, run against a file this
	// tool wrote a moment ago
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, python, append([]string{script, formatID, path}, settings...)...)
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	_ = cmd.Run()

	text := strings.TrimSpace(out.String())
	res := Result{Available: true, Tool: "strict structural check", Output: text}
	switch {
	case strings.HasPrefix(text, "OK"):
	case strings.HasPrefix(text, "FAIL"):
		res.Err = fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(text, "FAIL")))
	default:
		res.Err = fmt.Errorf("the checker said nothing useful: %s %s", text, strings.TrimSpace(errOut.String()))
	}
	return res
}

// StrictKnows says whether the structural checker covers a format.
func StrictKnows(formatID string) bool {
	switch formatID {
	case "png", "wav", "pdf", "zip", "targz", "log", "csv", "json", "xml", "svg", "html",
		// From its first day, because GDAL alone took files this one refuses
		// (docs/GEOJSON-2026-10-07.md section 3).
		"geojson",
		// From its first day as well: the Collector takes an enum written as a
		// name, keys in snake case and a span that ends before it starts
		// (docs/OTLP-2026-11-10.md section 3).
		"otlp",
		// The same reason for mail: four readers took a message with no From
		// line (docs/EML-2026-10-07.md section 5.1).
		"eml",
		"bmp", "gif", "ico", "jpg", "tiff", "webp", "avif", "jxl", "docx", "xlsx", "pptx",
		// The two text formats joined on 2026-09-07, when they gained an
		// encoding. Before that there was nothing here to check against
		// beyond "these are the bytes we meant" - a claim about UTF-16 is a
		// claim somebody else's decoder can settle.
		"txt", "md",
		// The two configuration formats, from the day they arrived. This layer
		// carries more of them than usual: measured 2026-09-22, PyYAML takes a
		// duplicate key that every other implementation refuses, and neither a
		// parser nor a hash would report a field that moved.
		"yaml", "toml",
		// The first film, from the day it arrived (2026-10-06). The decoder
		// says whether the frames that are there play, and only the container
		// says whether they are all there.
		"webm",
		// The second, from the day it arrived (2026-10-07). The sample tables
		// say how long every sample is, and only reading them against the
		// media data says the tables and the samples agree.
		//
		// And rtf, from its first day, for the reason mail gave: five readers
		// took eleven of twelve broken documents (docs/RTF-2026-10-15.md
		// section 6.1 f), and none of them says whether a fallback agrees
		// with the count before it.
		"mp4", "rtf":
		return true
	}
	return false
}

// strictScriptPath finds the checker next to this source file, so the tests
// work wherever the repository is checked out.
// strictScriptPath finds the checker script, and has to look twice.
//
// The first way works on the machine that compiled the binary: the script sits
// beside this file, and runtime.Caller says where that is.
//
// The second way exists because that path is COMPILED IN, so a binary built
// here and carried anywhere else looks for a directory that is not there.
// Measured on 2026-09-09: the guard suite cross compiled on Windows and run in
// a Linux container reported "0 file(s) decoded strictly by Python" with python
// sitting in that container's PATH the whole time. Every structural check had
// been quietly unavailable since the container runs began, and it took two
// guards that refuse to pass on nothing to make it visible - an unavailable
// checker is a skip, and a skip reads like a check that ran.
//
// So the second way asks the working directory instead and walks up looking for
// the script at its place in the tree. A test binary runs from inside the tree
// it tests, which is exactly the case the compiled in path cannot cover.
func strictScriptPath() (string, bool) {
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		p := filepath.Join(filepath.Dir(thisFile), "strict.py")
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return StrictScriptUnder(dir)
}

// StrictScriptUnder walks up from dir looking for the checker script.
//
// Exported for the guard that holds it. The compiled in path above is chosen
// first on any machine that built the binary, so a guard calling
// strictScriptPath would prove that one and never reach this.
func StrictScriptUnder(dir string) (string, bool) {
	for {
		p := filepath.Join(dir, "internal", "oracle", "strict.py")
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
		up := filepath.Dir(dir)
		if up == dir {
			// The top of the disk. Whatever sits above this belongs to
			// something else, so the answer is no rather than the first
			// checker that happens to turn up outside the tree.
			return "", false
		}
		dir = up
	}
}
