// Part of package cli. See cli.go.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// Ten thousand files take about eighteen seconds and a large run takes
// minutes, and all of it used to happen in silence. Past ten seconds a person
// cannot tell a working tool from a hung one - see docs/UX.md, rule UX5.
//
// The bar goes to the error channel so the data channel stays usable in a
// pipe, and it goes nowhere at all when that channel is not a terminal.
// Thousands of redrawn lines in a CI log are worse than no bar.

// progressInterval is how often the line may be redrawn. The engine reports
// far more often than this, once per write inside a file, and rate limiting
// what reaches a screen is the caller's job.
const progressInterval = 100 * time.Millisecond

type progressBar struct {
	w       io.Writer
	started time.Time
	last    time.Time
	printed int
}

// newProgressBar returns a bar, or nil when there is nobody to show one to.
func newProgressBar(errOut io.Writer) *progressBar {
	f, ok := errOut.(*os.File)
	if !ok {
		// A buffer, which is what the tests pass. Nothing to draw on.
		return nil
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		// Redirected to a file or a pipe.
		return nil
	}
	now := time.Now()
	return &progressBar{w: errOut, started: now, last: now}
}

func (p *progressBar) report(pr engine.Progress) {
	now := time.Now()
	if now.Sub(p.last) < progressInterval {
		return
	}
	p.last = now

	line := fmt.Sprintf("  %d/%d files  %s of %s  %d%%%s",
		pr.FilesDone, pr.FilesTotal,
		core.HumanBytes(pr.BytesDone), core.HumanBytes(pr.BytesTotal),
		core.Percent(pr.BytesDone, pr.BytesTotal),
		p.remaining(pr))

	// Pad to cover whatever the last line left behind, so a shorter line does
	// not read as the tail of the one before it.
	pad := p.printed - len(line)
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(p.w, "\r%s%s", line, strings.Repeat(" ", pad))
	p.printed = len(line)
}

// clear takes the line back before anything else is written, so the summary
// does not land on top of a half drawn bar.
func (p *progressBar) clear() {
	if p.printed == 0 {
		return
	}
	fmt.Fprintf(p.w, "\r%s\r", strings.Repeat(" ", p.printed))
	p.printed = 0
}

// remaining is an estimate and says so by staying quiet until it has enough to
// go on. A number that swings wildly for the first second is worse than none.
func (p *progressBar) remaining(pr engine.Progress) string {
	elapsed := time.Since(p.started)
	if elapsed < time.Second || pr.BytesDone <= 0 || pr.BytesDone >= pr.BytesTotal {
		return ""
	}
	left := time.Duration(float64(elapsed) *
		float64(pr.BytesTotal-pr.BytesDone) / float64(pr.BytesDone))
	return "  " + core.Roughly(left) + " left"
}

// Putting a size, a percentage and a duration into words lives in core, because
// the window draws a progress bar too and the two surfaces cannot import each
// other. A copy here would be the two bars rounding differently, which is the
// kind of drift nobody goes looking for.
