package guard

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// osvKeyValue is one key = value line of a TOML table.
var osvKeyValue = regexp.MustCompile(`^([A-Za-z]+)\s*=\s*(.*)$`)

// osvExceptionProblems is what is wrong with the held back advisories of an
// OSV-Scanner configuration: an entry without an id, without a date it stops
// working on, or without a reason.
//
// Read line by line rather than with a TOML parser, which this module only has
// as a dependency of a dependency. The file is flat: one [[IgnoredVulns]]
// table per advisory, with one key per line.
func osvExceptionProblems(body string) []string {
	var problems []string
	blocks := strings.Split(body, "[[IgnoredVulns]]")
	for i, block := range blocks[1:] {
		keys := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") {
				break
			}
			if m := osvKeyValue.FindStringSubmatch(line); m != nil {
				keys[m[1]] = strings.TrimSpace(m[2])
			}
		}
		name := keys["id"]
		if strings.Trim(name, `"`) == "" {
			name = "entry " + strconv.Itoa(i+1)
			problems = append(problems, name+" names no advisory")
		}
		until := keys["ignoreUntil"]
		if len(until) < 10 {
			problems = append(problems, name+" has no ignoreUntil, so it holds the advisory back for good")
		} else if _, err := time.Parse(time.DateOnly, until[:10]); err != nil {
			problems = append(problems, name+" has an ignoreUntil that is not a date: "+until)
		}
		if strings.Trim(keys["reason"], `" `) == "" {
			problems = append(problems, name+" gives no reason, and the scan prints the reason beside every advisory it holds back")
		}
	}
	return problems
}

// Every advisory the OSV scan holds back has a date it stops working on and a
// reason.
//
// The date is what brings an advisory back: when it passes, the weekly scan
// goes red again and the entry has to be read again. An entry without one holds
// the advisory back for good and nothing says so. The header of the file
// promises a date for every entry, and nothing else checks that promise.
//
// Its own canary: an entry with no ignoreUntil and an entry with an empty reason
// have to give two problems.
func TestEveryHeldBackAdvisoryHasAnEndDateAndAReason(t *testing.T) {
	canary := `[[IgnoredVulns]]
id = "GHSA-aaaa-aaaa-aaaa"
reason = "held back"

[[IgnoredVulns]]
id = "GHSA-bbbb-bbbb-bbbb"
ignoreUntil = 2026-12-14
reason = ""
`
	if got := osvExceptionProblems(canary); len(got) != 2 {
		t.Fatalf("the canary, one entry without a date and one without a reason, gave %d problem(s) and has two: %v", len(got), got)
	}

	body := readRepoFile(t, filepath.Join(".github", "osv-scanner.toml"))
	for _, p := range osvExceptionProblems(body) {
		t.Error(p)
	}
}
