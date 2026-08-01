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
