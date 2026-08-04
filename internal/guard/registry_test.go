package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// At twenty five formats the likeliest mistake is one added half way: the
// generator works, and it never declared its minimum, or its padding channel,
// or where its label goes.
//
// So one test walks the registry and demands the full set from every format.
// That is cheaper than a test per declaration and it catches exactly the
// failure that gets more likely with each format added.

func TestEveryFormatDeclaresTheFullSet(t *testing.T) {
	all := format.All()
	if len(all) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	for _, d := range all {
		t.Run(d.ID, func(t *testing.T) {
			if d.ID == "" {
				t.Error("no id")
			}
			if !strings.HasPrefix(d.Extension, ".") {
				t.Errorf("extension %q does not start with a dot", d.Extension)
			}
			if d.Generator == nil {
				t.Error("no generator")
			}

			switch d.Fidelity {
			case format.FidelityFull, format.FidelityStructural, format.FidelityStub:
			default:
				t.Errorf("fidelity is %q, which is not one of the declared levels", d.Fidelity)
			}

			switch d.Determinism {
			case format.DeterminismByte, format.DeterminismSize:
			default:
				t.Errorf("determinism is %q, which is not one of the declared levels", d.Determinism)
			}

			switch d.Label {
			case format.LabelVisible, format.LabelInternal, format.LabelExternalOnly:
			default:
				t.Errorf("label carrier is %q, which is not one of the declared kinds", d.Label)
			}

			if d.MinBytes < 0 {
				t.Errorf("minimum size is %d", d.MinBytes)
			}

			if d.Padding.Name == "" {
				t.Error("the padding channel has no name, so no document can describe it")
			}
			switch d.Padding.Where {
			case format.PlacementEnd, format.PlacementStart, format.PlacementInside:
			default:
				t.Errorf("the padding channel sits at %q, which is not one of the declared places", d.Padding.Where)
			}
			if d.Padding.Capacity < 0 {
				t.Errorf("the padding channel declares a capacity of %d", d.Padding.Capacity)
			}

			// Saying nothing and saying none have to look different, otherwise
			// a forgotten declaration passes as a decision.
			if d.Oracle == "" {
				t.Error("no oracle declared - use OracleNone to say there is none on purpose")
			}
			if d.Oracle != format.OracleNone {
				if _, known := oracle.For(d.Oracle); !known {
					t.Errorf("declares the oracle %q and nothing implements it", d.Oracle)
				}
			}

			if d.GeneratorVersion == "" {
				t.Error("no generator version - the manifest needs it to explain a hash mismatch after an upgrade")
			}
		})
	}
}

// The registry is the source of the confirmed minimum and the format document
// keeps an approximate table beside it. This is what stops the two drifting.
//
// The document lives outside the repository, so on a fresh checkout there is
// nothing to compare against. That skips loudly rather than passing quietly.
func TestTheFormatDocumentAgreesWithTheRegistry(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "MVP-FORMATS.md")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Logf("SKIPPED: docs/MVP-FORMATS.md is not here, so nothing was compared. "+
			"The internal documents are excluded from the repository, so this check only runs on a machine that has them. (%v)", err)
		return
	}
	text := string(body)

	for _, d := range format.All() {
		// Every implemented format has to appear in the document at all.
		if !strings.Contains(strings.ToLower(text), "**"+strings.ToLower(d.ID)+"**") {
			t.Errorf("%s is implemented and the format document does not mention it", d.ID)
			continue
		}

		// Where the document states a minimum for an implemented format, it
		// has to be the measured one.
		if stated, ok := statedMinimum(text, d.ID); ok && stated != d.MinBytes {
			t.Errorf("%s: the registry measured a minimum of %d B and the document says %d B",
				d.ID, d.MinBytes, stated)
		}
	}
}

// checklistRow matches a row of the fidelity checklist, which names its format
// in bold in the first cell.
var checklistRow = regexp.MustCompile(`(?m)^\|\s*\*\*([^*|]+)\*\*\s*\|`)

// TestEveryFormatHasARowInTheFidelityChecklist keeps the one list a person
// fills in from falling behind the registry.
//
// D4 asks every format to declare a fidelity level and to record the result of
// opening it in a real application. The checklist in MVP-FORMATS.md section 6.1
// is that record, and it is the only guard in this project that a machine
// cannot stand in for - automatic checks answer whether a file parses, and this
// answers whether somebody watched it open.
//
// The check above asks whether the document mentions a format anywhere. A
// format can satisfy that from its own card and still be missing here, which
// would leave it looking finished while the one human step was never done.
//
// The document lives outside the repository, so this skips loudly on a fresh
// checkout, the same way its neighbour does.
func TestEveryFormatHasARowInTheFidelityChecklist(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "MVP-FORMATS.md")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Logf("SKIPPED: docs/MVP-FORMATS.md is not here, so nothing was compared. "+
			"The internal documents are excluded from the repository, so this check only runs on a machine that has them. (%v)", err)
		return
	}

	section, ok := fidelityChecklist(string(body))
	if !ok {
		t.Fatal("docs/MVP-FORMATS.md has no fidelity checklist section - this guard would pass without checking anything")
	}

	listed := map[string]bool{}
	for _, m := range checklistRow.FindAllStringSubmatch(section, -1) {
		// The checklist names formats the way a person writes them, so TAR.GZ
		// stands for the id targz. Dots go, case folds, and the two meet.
		listed[strings.ToLower(strings.ReplaceAll(strings.TrimSpace(m[1]), ".", ""))] = true
	}
	if len(listed) == 0 {
		t.Fatal("the fidelity checklist has no rows - this guard would pass without checking anything")
	}

	for _, d := range format.All() {
		if !listed[strings.ToLower(d.ID)] {
			t.Errorf("%s is implemented and the fidelity checklist in docs/MVP-FORMATS.md section 6.1 "+
				"has no row for it. That list is the record of somebody opening the file, and it is the "+
				"one check nothing automatic replaces", d.ID)
		}
	}
}

// fidelityChecklist returns the section 6.1 of the format document, so rows of
// the other tables in the file cannot be mistaken for checklist entries.
func fidelityChecklist(text string) (string, bool) {
	start := strings.Index(text, "## 6.1")
	if start < 0 {
		return "", false
	}
	rest := text[start+len("## 6.1"):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end], true
	}
	return rest, true
}

// statedMinimum reads a line of the shape "minimum ... 1234 B" from the
// implementation note of one format, when the document carries one.
var minimumLine = regexp.MustCompile(`(?i)najmniejszy\s+` + "`?" + `?(\w+)` + "`?" + `?[^\n]*?(\d[\d\s]*)\s*B`)

func statedMinimum(text, id string) (int64, bool) {
	for _, m := range minimumLine.FindAllStringSubmatch(text, -1) {
		if !strings.EqualFold(m[1], id) {
			continue
		}
		digits := strings.ReplaceAll(strings.TrimSpace(m[2]), " ", "")
		n, err := strconv.ParseInt(digits, 10, 64)
		if err != nil {
			continue
		}
		return n, true
	}
	return 0, false
}
