package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// printFlags is fs.PrintDefaults with the backticks of every usage left as
// they are, written where fs writes.
//
// PrintDefaults takes the first word of a usage written between backticks for
// the name of the flag's value, prints it after the flag and drops the marks
// (flag.UnquoteUsage). No flag of ours names its value that way, and the
// usages of a preset's and a tool's settings mark the values a person types
// (core.Literal) - so a preset's groups printed as "-groups screens", with
// screens the one value left unmarked, measured on 2026-11-18
// (docs/HINT-VALUES-2026-11-18.md). The marks stand aside while the flags are
// printed and come back in what was printed.
func printFlags(fs *flag.FlagSet) {
	out := fs.Output()
	var printed strings.Builder
	var marked []*flag.Flag
	fs.VisitAll(func(f *flag.Flag) {
		if strings.Contains(f.Usage, core.Backtick) {
			f.Usage = strings.ReplaceAll(f.Usage, core.Backtick, markStandIn)
			marked = append(marked, f)
		}
	})
	fs.SetOutput(&printed)
	fs.PrintDefaults()
	fs.SetOutput(out)
	for _, f := range marked {
		f.Usage = strings.ReplaceAll(f.Usage, markStandIn, core.Backtick)
	}
	fmt.Fprint(out, strings.ReplaceAll(printed.String(), markStandIn, core.Backtick))
}

// markStandIn holds a backtick's place while PrintDefaults reads a usage - a
// character no usage carries, so putting the marks back finds only theirs.
const markStandIn = "\x00"
