package oracle

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StrictRequest is one file handed to StrictBatch.
type StrictRequest struct {
	// Format is the ID of the check to run, as for Strict.
	Format string `json:"format"`
	// Path is the file to judge.
	Path string `json:"path"`
	// Settings are the key=value words Strict takes after the path.
	Settings map[string]string `json:"settings,omitempty"`
}

// StrictJudgement is the checker's answer about one file.
type StrictJudgement struct {
	// Path is the file the answer is about, as it was asked.
	Path string `json:"path"`
	// Verdict is one of the Strict* verdict constants.
	Verdict string `json:"verdict"`
	// Reason is what the check said.
	Reason string `json:"reason"`
}

// The four answers the checker gives in batch mode.
const (
	// StrictOK is a file the check accepts.
	StrictOK = "OK"
	// StrictFail is a file the check refuses, for the reason given.
	StrictFail = "FAIL"
	// StrictCrash is the check stopping on an exception instead of giving
	// a verdict. It is a fault in the checker, never a refusal of the file:
	// counted as one, a broken checker passes for a working one
	// (docs/DAMAGE-ETAP-2-2026-11-09.md section 3.3).
	StrictCrash = "CRASH"
	// StrictError is a request the checker could not act on.
	StrictError = "ERROR"
)

// StrictBatch judges many files in one run of the structural checker.
//
// Strict starts Python once per file. A guard that judges every format
// against every damage asks a few hundred times, so this hands the whole list
// to one process and reads one answer per file back, in the order asked.
//
// The second result is false when there is no Python or no checker to run,
// which the caller reports as a skip rather than passing quietly. An error
// means the run itself went wrong - the process failed, or the answers do not
// match the questions one to one - and no verdict in it can be trusted.
//
// The context is the caller's, so a guard that is stopped stops the checker
// with it. A time limit of its own sits under it.
func StrictBatch(ctx context.Context, reqs []StrictRequest) ([]StrictJudgement, bool, error) {
	python, ok := inPath("python")()
	if !ok {
		return nil, false, nil
	}
	script, ok := strictScriptPath()
	if !ok {
		return nil, false, nil
	}

	lines, err := batchLines(ctx, python, script, "the structural checker", reqs)
	if err != nil {
		return nil, true, err
	}
	got := make([]StrictJudgement, 0, len(lines))
	for _, line := range lines {
		var j StrictJudgement
		if err := json.Unmarshal([]byte(line), &j); err != nil {
			return nil, true, fmt.Errorf("the structural checker answered %q, which is not an answer: %w", line, err)
		}
		got = append(got, j)
	}
	for i := range got {
		if got[i].Path != reqs[i].Path {
			return nil, true, fmt.Errorf("answer %d is about %q and the question was about %q", i+1, got[i].Path, reqs[i].Path)
		}
	}
	return got, true, nil
}

// batchLines hands every request to one run of script --batch, a JSON line
// each, and reads one answer line back for each, in the order asked.
//
// Shared by the structural checker and the reader of maps, which speak the
// same protocol. An error means the run itself went wrong - the process
// failed, or the answers do not match the questions one to one.
func batchLines[T any](ctx context.Context, python, script, who string, reqs []T) ([]string, error) {
	var in strings.Builder
	for _, r := range reqs {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("writing a request for %s: %w", who, err)
		}
		in.Write(line)
		in.WriteByte('\n')
	}

	// A minute for the process, and a second for every file on top.
	limit := time.Minute + time.Duration(len(reqs))*time.Second
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	//nolint:gosec // our own script, run against files this tool wrote
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, python, script, "--batch")
	cmd.Stdin = strings.NewReader(in.String())
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s stopped: %w: %s", who, err, strings.TrimSpace(errOut.String()))
	}

	var lines []string
	scan := bufio.NewScanner(strings.NewReader(out.String()))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		if line := strings.TrimSpace(scan.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("reading the answers of %s: %w", who, err)
	}
	if len(lines) != len(reqs) {
		return nil, fmt.Errorf("the answers of %s do not match its questions: asked %d, answered %d", who, len(reqs), len(lines))
	}
	return lines, nil
}
