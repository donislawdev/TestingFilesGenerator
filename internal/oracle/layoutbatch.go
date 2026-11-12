package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// LayoutRequest is one file handed to LayoutBatch.
type LayoutRequest struct {
	// Format is the ID of the format the file is in.
	Format string `json:"format"`
	// Path is the file to read.
	Path string `json:"path"`
	// Settings are the settings the file was made with, for a reader whose
	// map follows one of them.
	Settings map[string]string `json:"settings,omitempty"`
}

// LayoutAnswer is the map read back from one file's structure: a region a
// list of [role, at, length], or an error saying why there is none.
type LayoutAnswer struct {
	Path    string   `json:"path"`
	Regions [][3]any `json:"regions,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// LayoutBatch reads the map of many files in one run of layout.py, which sits
// beside the structural checker and borrows its walkers.
//
// The second result is false when there is no Python or no script to run,
// which the caller reports as a skip. An error means the run itself went wrong.
func LayoutBatch(ctx context.Context, reqs []LayoutRequest) ([]LayoutAnswer, bool, error) {
	python, ok := inPath("python")()
	if !ok {
		return nil, false, nil
	}
	strict, ok := strictScriptPath()
	if !ok {
		return nil, false, nil
	}
	script := filepath.Join(filepath.Dir(strict), "layout.py")

	lines, err := batchLines(ctx, python, script, "the reader of maps", reqs)
	if err != nil {
		return nil, true, err
	}
	got := make([]LayoutAnswer, 0, len(lines))
	for i, line := range lines {
		var a LayoutAnswer
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, true, fmt.Errorf("the reader of maps answered %q, which is not an answer: %w", line, err)
		}
		if a.Path != reqs[i].Path {
			return nil, true, fmt.Errorf("answer %d is about %q and the question was about %q", i+1, a.Path, reqs[i].Path)
		}
		got = append(got, a)
	}
	return got, true, nil
}
