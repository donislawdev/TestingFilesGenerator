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
func StrictBatch(reqs []StrictRequest) ([]StrictJudgement, bool, error) {
	python, ok := inPath("python")()
	if !ok {
		return nil, false, nil
	}
	script, ok := strictScriptPath()
	if !ok {
		return nil, false, nil
	}

	var in strings.Builder
	for _, r := range reqs {
		line, err := json.Marshal(r)
		if err != nil {
			return nil, true, fmt.Errorf("writing the request for %s: %w", r.Path, err)
		}
		in.Write(line)
		in.WriteByte('\n')
	}

	// A minute for the process, and a second for every file on top.
	limit := time.Minute + time.Duration(len(reqs))*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	//nolint:gosec // our own script, run against files this tool wrote
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, python, script, "--batch")
	cmd.Stdin = strings.NewReader(in.String())
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return nil, true, fmt.Errorf("the structural checker stopped: %w: %s", err, strings.TrimSpace(errOut.String()))
	}

	got := make([]StrictJudgement, 0, len(reqs))
	scan := bufio.NewScanner(strings.NewReader(out.String()))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		var j StrictJudgement
		if err := json.Unmarshal([]byte(line), &j); err != nil {
			return nil, true, fmt.Errorf("the structural checker answered %q, which is not an answer: %w", line, err)
		}
		got = append(got, j)
	}
	if err := scan.Err(); err != nil {
		return nil, true, fmt.Errorf("reading the structural checker's answers: %w", err)
	}
	if len(got) != len(reqs) {
		return nil, true, fmt.Errorf("the structural checker was asked about %d file(s) and answered %d", len(reqs), len(got))
	}
	for i := range got {
		if got[i].Path != reqs[i].Path {
			return nil, true, fmt.Errorf("answer %d is about %q and the question was about %q", i+1, got[i].Path, reqs[i].Path)
		}
	}
	return got, true, nil
}
