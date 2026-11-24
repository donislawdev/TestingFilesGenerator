package guard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// osvKeyValue is one key = value line of a TOML table.
var osvKeyValue = regexp.MustCompile(`^([A-Za-z]+)\s*=\s*(.*)$`)

// osvTableHeader is the header of one held back advisory. TOML allows spaces
// inside the brackets and a comment after them.
var osvTableHeader = regexp.MustCompile(`(?m)^[ \t]*\[\[[ \t]*IgnoredVulns[ \t]*\]\][ \t]*(?:#.*)?$`)

// osvExceptionProblems is what is wrong with the held back advisories of an
// OSV-Scanner configuration: an entry without an id, without a date it stops
// working on, or without a reason.
//
// Read line by line rather than with a TOML parser, which this module only has
// as a dependency of a dependency. The file is flat: one [[IgnoredVulns]]
// table per advisory, with one key per line. An entry written any other way,
// as an inline table, is refused rather than skipped, so it cannot slip past.
func osvExceptionProblems(body string) []string {
	var problems []string
	heads := osvTableHeader.FindAllStringIndex(body, -1)
	if mentions := osvMentions(body); mentions != len(heads) {
		problems = append(problems, fmt.Sprintf("IgnoredVulns appears on %d line(s) and %d of them are a table header "+
			"this reads, so an entry is written in a layout this guard does not check", mentions, len(heads)))
	}
	for i, h := range heads {
		end := len(body)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		problems = append(problems, osvEntryProblems(i+1, osvKeys(body[h[1]:end]))...)
	}
	return problems
}

// osvMentions is how many lines that are not a comment name IgnoredVulns.
func osvMentions(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, "IgnoredVulns") {
			n++
		}
	}
	return n
}

// osvKeys is the keys of one table, up to the next header of any table.
func osvKeys(block string) map[string]string {
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
	return keys
}

// osvEntryProblems is what is wrong with entry n.
func osvEntryProblems(n int, keys map[string]string) []string {
	var problems []string
	id, _ := tomlString(keys["id"])
	name := strconv.Quote(id)
	if id == "" {
		name = "entry " + strconv.Itoa(n)
		problems = append(problems, name+" names no advisory")
	}
	until := keys["ignoreUntil"]
	if len(until) < 10 {
		problems = append(problems, name+" has no ignoreUntil, so it holds the advisory back for good")
	} else if _, err := time.Parse(time.DateOnly, until[:10]); err != nil {
		problems = append(problems, name+" has an ignoreUntil that is not a date: "+until)
	}
	if reason, ok := tomlString(keys["reason"]); !ok || strings.TrimSpace(reason) == "" {
		problems = append(problems, name+" gives no reason, and the scan prints the reason beside every advisory it holds back")
	}
	return problems
}

// tomlString is the text of a TOML string value, with what follows it, a
// comment among them, left out. False when the value is not a string.
func tomlString(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "'") {
		end := strings.Index(v[1:], "'")
		return v[1 : 1+max(end, 0)], end >= 0
	}
	if !strings.HasPrefix(v, `"`) {
		return "", false
	}
	var b strings.Builder
	for i := 1; i < len(v); i++ {
		switch c := v[i]; {
		case c == '\\' && i+1 < len(v):
			b.WriteByte(v[i+1])
			i++
		case c == '"':
			return b.String(), true
		default:
			b.WriteByte(c)
		}
	}
	return "", false
}

// Every advisory the OSV scan holds back has a date it stops working on and a
// reason.
//
// The date is what brings an advisory back: when it passes, the weekly scan
// goes red again and the entry has to be read again. An entry without one holds
// the advisory back for good and nothing says so. The header of the file
// promises a date for every entry, and nothing else checks that promise.
//
// Its own canary: entries without a date, with an empty reason, with spaces in
// the header and with a comment after an empty reason have to be reported, an
// entry with a # inside its reason must not be, and an inline table is refused.
func TestEveryHeldBackAdvisoryHasAnEndDateAndAReason(t *testing.T) {
	canary := `[[IgnoredVulns]]
id = "GHSA-aaaa-aaaa-aaaa"
reason = "held back"

[[IgnoredVulns]]
id = "GHSA-bbbb-bbbb-bbbb"
ignoreUntil = 2026-12-14
reason = ""

[[ IgnoredVulns ]]
id = "GHSA-cccc-cccc-cccc"
reason = "held back"

[[IgnoredVulns]]
id = "GHSA-dddd-dddd-dddd"
ignoreUntil = 2026-12-14
reason = "" # explain later

[[IgnoredVulns]]
id = "GHSA-eeee-eeee-eeee"
ignoreUntil = 2026-12-14
reason = "held back # until the fix ships"
`
	if got := osvExceptionProblems(canary); len(got) != 4 {
		t.Fatalf("the canary, four broken entries and one whole one, gave %d problem(s) and has four: %v", len(got), got)
	}
	if got := osvExceptionProblems("IgnoredVulns = [{ id = \"GHSA-ffff-ffff-ffff\" }]\n"); len(got) != 1 {
		t.Fatalf("an entry written as an inline table was not refused: %v", got)
	}

	body := readRepoFile(t, filepath.Join(".github", "osv-scanner.toml"))
	for _, p := range osvExceptionProblems(body) {
		t.Error(p)
	}
}
