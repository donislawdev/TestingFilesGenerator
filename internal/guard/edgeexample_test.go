package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// The three files every first page shows - one byte under a 1 MB limit, the
// limit, one byte over - are what size-boundaries actually writes.
//
// They stand in the places a stranger reads before anything else: the social
// card, the README, and the site's first page in every language. All of them
// carry the files as literals, and the card says outright that everything on it
// is real. Nothing asked the program. An outside review of #148 named it, and
// asking showed it was already half untrue: the card before that one printed
// "tfg generate --preset size-boundaries --limit 1mb", which the program
// refuses, because the default spread would need a file of 0 B.
//
// So this runs the command the README and the site print, as a dry run, and
// holds each place to the answer: every file on the line of its name, with its
// byte count and the outcome the manifest declares for it.
func TestTheLimitExampleIsWhatThePresetWrites(t *testing.T) {
	const command = "tfg generate --preset size-boundaries --limit 1mb --spread 1B --format pdf --out ./edges"
	args := append(strings.Fields(strings.TrimPrefix(command, "tfg ")), "--dry-run", "--json")
	args[len(args)-3] = filepath.Join(t.TempDir(), "edges")
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the command the pages print ended with %d:\n%s", code, errOut.String())
	}
	var manifest struct {
		Files []struct {
			Name     string `json:"name"`
			Bytes    int64  `json:"bytes"`
			Expected struct {
				Outcome string `json:"outcome"`
			} `json:"expected"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &manifest); err != nil {
		t.Fatalf("reading the dry run's manifest: %v", err)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("the command writes %d files, and every page shows three", len(manifest.Files))
	}

	root := repoRoot(t)
	type place struct {
		file     string
		spaced   bool
		outcomes map[string]string
		command  bool
	}
	places := []place{
		{"web/templates/social.html", true, map[string]string{"accept": "accept", "reject": "reject"}, false},
		{"README.md", false, map[string]string{"accept": "**accept**", "reject": "**reject**"}, true},
		{"web/content/en/index.html", false, map[string]string{"accept": "accept", "reject": "reject"}, true},
		{"web/content/pl/index.html", false, map[string]string{"accept": "przyjąć", "reject": "odrzucić"}, true},
	}
	// Every other translation of the first page is held to the same answer. The
	// words are asked for rather than guessed: a language whose first page shows
	// the three files in a form nobody listed here is a page this guard cannot
	// read, and silence about it would be the defect the guard exists for.
	for _, l := range languagesOnDisk(t) {
		if l.Code == "en" || l.Code == "pl" {
			continue
		}
		words, ok := translatedLimitWords[l.Code]
		if !ok {
			t.Errorf("the first page in %s is not held to the limit example - add the words it uses for accept and reject to translatedLimitWords", l.Code)
			continue
		}
		places = append(places, place{"web/content/" + l.Code + "/index.html", false, map[string]string{"accept": words[0], "reject": words[1]}, true})
	}
	for _, at := range places {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(at.file)))
		if err != nil {
			t.Fatalf("reading %s: %v", at.file, err)
		}
		text := string(body)
		if at.command && !strings.Contains(text, command) {
			t.Errorf("%s no longer prints %q, so the files it shows come from a command this guard does not run",
				at.file, command)
		}
		for _, f := range manifest.Files {
			line := lineNaming(text, f.Name)
			if line == "" {
				t.Errorf("%s does not show %s, and the command writes it", at.file, f.Name)
				continue
			}
			size := strconv.FormatInt(f.Bytes, 10)
			if at.spaced {
				size = spacedBytes(f.Bytes) + " B"
			}
			if !strings.Contains(line, size) {
				t.Errorf("%s shows %s without its %s:\n%s", at.file, f.Name, size, line)
			}
			word, ok := at.outcomes[f.Expected.Outcome]
			if !ok {
				t.Errorf("the manifest declares %q for %s, and %s has no word for it", f.Expected.Outcome, f.Name, at.file)
				continue
			}
			if !strings.Contains(line, word) {
				t.Errorf("%s shows %s without the outcome the manifest declares (%s):\n%s",
					at.file, f.Name, word, line)
			}
		}
	}
}

// lineNaming is the one line of a page that shows a file, or nothing when no
// line or more than one does - a name twice on a page is two claims, and this
// checks one.
func lineNaming(text, name string) string {
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, name) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

// spacedBytes writes a byte count the way the social card does, in groups of
// three.
func spacedBytes(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// translatedLimitWords are the two words each translated first page uses for
// what the manifest declares - accept first, reject second - in the form that
// stands on the line naming a file.
//
// A word is not unique to a language, and a translation inflects it. Polish
// writes the infinitive in the table and so the list says przyjąć rather than
// przyjmie, and each entry is exactly the form the page shows. The key is the
// language tag, the same as the directory under web/content.
var translatedLimitWords = map[string][2]string{
	"de": {"akzeptieren", "ablehnen"},
}
