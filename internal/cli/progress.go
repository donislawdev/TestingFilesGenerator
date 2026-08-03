// Part of package cli. See cli.go.
package cli

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

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
		humanBytes(pr.BytesDone), humanBytes(pr.BytesTotal),
		percent(pr.BytesDone, pr.BytesTotal),
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
	return "  " + roughly(left) + " left"
}

// percent divides before multiplying where it has to, so a very large run does
// not wrap on the way to a number between nought and a hundred.
//
// done*100 leaves the range of an int64 above about 92 PB. No disk holds that
// today, and the arithmetic that produces it is free to be right anyway.
func percent(done, total int64) int {
	switch {
	case total <= 0:
		return 100
	case done >= total:
		return 100
	case done > math.MaxInt64/100:
		return int(done / (total / 100))
	}
	return int(done * 100 / total)
}

// roughly keeps the estimate at the precision it deserves. Seconds on a two
// minute estimate are noise that changes every redraw.
func roughly(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds())+1)
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes())+1)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// humanBytes counts in 1024s, the same as every size this tool accepts and the
// same as what Explorer and ls show. See docs/RECIPE.md section 9.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
