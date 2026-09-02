package guard

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// What these defend. A setting with a closed set of values is refused in one
// voice, and the voice is the declaration's.
//
// Why they were written. Measured 2026-09-02 while closing O168, and then
// again while closing O171: three places in the tree built the sentence that
// says which words a setting takes. One was the registry, which builds it from
// the declared set. The other two were the archive package writing the same
// list out by hand, in the same order and the same phrasing, for compression
// and for entry_owner - and the second of those was added by the very change
// that closed O168, which is the clearest evidence available that this copies
// itself faster than anybody notices.
//
// Why identical copies are the dangerous kind. Nothing goes red while they
// agree, and they agreed. The day they stop agreeing is the day somebody adds
// a fifth compression level to the declaration: the registry lists five, tfg
// formats prints five, the window offers five, and the hand written line lists
// four. Nobody set out to write two answers - one line simply did not get
// edited, and no test in this repository could have said so.
//
// Why the user never saw it, and why that is not a reason to leave it. Measured
// 2026-09-02 on the built binary: --set compression=TURBO and --set
// entry_owner=USER are both refused by the REGISTRY, exit 4, in the registry's
// words. The hand written branches sit behind that check and no surface reaches
// them. They are there for a direct caller, which is what a guard is - so they
// can be reddened, which is the test this project applies before calling
// anything a defence.
//
// What they do NOT check. Whether the sentence reads well. Three tests can say
// it is built once, that a refusal quotes it, and that every word in it works.
// None of them can say it is the right sentence to put in front of a person.

// One place builds the sentence, and it is the registry.
//
// String literals rather than the file text, the same choice plurals_test.go
// makes and for the same reason: a comment explaining this rule has to be able
// to name the sentence, and reading raw bytes could not tell a comment from a
// message. The parser can.
func TestTheSentenceNamingWhichWordsASettingTakesIsWrittenInOnePlace(t *testing.T) {
	root := repoRoot(t)

	// The stem rather than the whole sentence. The registry writes it with a
	// colon and a trailing space, and a copy that dropped the colon would be
	// exactly as much of a copy.
	const sentence = "it takes one of"

	// The one place allowed to say it, written with forward slashes so this
	// reads the same on every system.
	const home = "internal/format/format.go"

	var found []string
	files := 0
	for _, dir := range []string{"internal", "cmd"} {
		start := filepath.Join(root, dir)
		err := filepath.WalkDir(start, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			// Guards quote the sentences they assert on, so a guard reading
			// guards would fail on its own evidence.
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files++

			fset := token.NewFileSet()
			parsed, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Errorf("parsing %s: %v", path, perr)
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)

			ast.Inspect(parsed, func(n ast.Node) bool {
				lit, isLit := n.(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					return true
				}
				if strings.Contains(lit.Value, sentence) {
					found = append(found, fmt.Sprintf("%s:%d", rel, fset.Position(lit.Pos()).Line))
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", start, err)
		}
	}

	// A walk that read nothing is a green test about nothing. This has happened
	// in this repository often enough to be written down as a trap.
	if files == 0 {
		t.Fatal("no source file was read, so this proved nothing")
	}

	switch {
	case len(found) == 0:
		t.Fatalf("nothing in the tree says %q, so either the registry stopped saying it "+
			"or this guard is looking for the wrong words. Both are worth knowing: "+
			"the sentence is what a person reads when a setting is refused.", sentence)
	case len(found) > 1:
		t.Errorf("%d places say %q and there may only be one, which is %s.\n"+
			"  %s\n"+
			"A format that wants this sentence asks its declaration for it - Property.Allows "+
			"builds it from the declared set, so a value added to the set reaches the sentence "+
			"on its own. A list written out by hand is a second answer to one question, and it "+
			"agrees with the first until the day somebody adds a value (O171).",
			len(found), sentence, home, strings.Join(found, "\n  "))
	case !strings.HasPrefix(found[0], home+":"):
		t.Errorf("the one place saying %q is %s, and it should be %s.\n"+
			"The sentence belongs where the declared set is read, so that every format "+
			"refuses in the same words.", sentence, found[0], home)
	}
}

// A refusal quotes the registry, whatever the wording is that day.
//
// This is the half that survives a rewording. The test above forbids the exact
// list coming back. This one forbids a differently worded copy, because a
// second sentence saying the same thing is the same defect wearing other words.
func TestAnArchiveRefusesAValueInTheWordsTheRegistryBuilds(t *testing.T) {
	t.Run(archive.Compression, func(t *testing.T) {
		const bad = "turbo"
		_, err := archive.ReadCompression("zip", format.Request{
			Properties: map[string]string{archive.Compression: bad},
		}, false)
		refusedInTheRegistrysWords(t, err, archive.Compression, bad)
	})

	t.Run(archive.EntryOwner, func(t *testing.T) {
		const bad = "nobody"
		_, err := archive.ReadOwnership("targz", map[string]string{archive.EntryOwner: bad})
		refusedInTheRegistrysWords(t, err, archive.EntryOwner, bad)
	})
}

// Every word the declaration offers is a word its reader takes.
//
// This is what the fix above newly depends on, so it is what the fix newly has
// to be guarded against. The sentence now comes from the declared set, which
// means a value added to the set is named as allowed by the refusal that
// refuses it - "compression cannot be turbo, it takes one of: best, default,
// fast, none, turbo" - and worse, a reader with no branch for it returns an
// EMPTY reason, because Allows says nothing is wrong with a value it allows.
//
// Asked of the declaration rather than of a list written here, so a fourth
// closed set added to the archive package is covered on the day it arrives.
func TestEveryWordTheArchiveDeclarationOffersIsOneItsReaderAccepts(t *testing.T) {
	readers := map[string]func(word string) error{
		archive.Compression: func(word string) error {
			_, err := archive.ReadCompression("zip", format.Request{
				Properties: map[string]string{archive.Compression: word},
			}, false)
			return err
		},
		archive.EntryOwner: func(word string) error {
			_, err := archive.ReadOwnership("targz", map[string]string{archive.EntryOwner: word})
			return err
		},
		archive.EntryMode: func(word string) error {
			_, err := archive.ReadOwnership("targz", map[string]string{archive.EntryMode: word})
			return err
		},
	}

	for key, read := range readers {
		t.Run(key, func(t *testing.T) {
			p := archive.Axes(key)[0]
			if p.Kind != format.PropertyChoice {
				t.Fatalf("%s is declared as %v rather than a closed set, so this test is "+
					"asking the wrong question about it", key, p.Kind)
			}
			// An empty set would make the loop below pass without running, which
			// is the shape of a green test that proves nothing.
			if len(p.Choices) == 0 {
				t.Fatalf("%s declares no values, so the loop over them proved nothing", key)
			}

			for _, word := range p.Choices {
				if err := read(word); err != nil {
					t.Errorf("the declaration offers %s=%q and the reader refuses it: %v\n"+
						"Both halves are read by a person: the window draws this word in a menu "+
						"and tfg formats prints it, so a word the reader will not take is a "+
						"setting that is offered and does not work.", key, word, err)
				}
			}
		})
	}
}

// refusedInTheRegistrysWords checks that err is the refusal that names a
// setting, and that its reason is the one the declaration builds.
func refusedInTheRegistrysWords(t *testing.T, err error, key, bad string) {
	t.Helper()

	p := archive.Axes(key)[0]

	// If the value this test picked has since been declared, the comparison
	// below would hold two empty strings against each other and pass. Naming
	// that rather than letting it happen: a test whose evidence has evaporated
	// reports success in exactly the same words as a test that worked.
	want := p.Allows(bad)
	if want == "" {
		t.Fatalf("%s=%q is a value the declaration now allows, so this test no longer has "+
			"a refusal to read. Pick a value the declaration does not contain.", key, bad)
	}

	if err == nil {
		t.Fatalf("%s=%q was accepted, and the declaration does not allow it", key, bad)
	}

	var value *format.PropertyValueError
	if !errors.As(err, &value) {
		t.Fatalf("%s=%q was refused as %T rather than as the refusal that names a setting, "+
			"so nothing downstream can say which field to point at: %v", key, bad, err, err)
	}

	if value.Reason != want {
		t.Errorf("%s=%q is refused in words of its own rather than the declaration's.\n"+
			"  refusal says:      %q\n"+
			"  declaration says:  %q\n"+
			"Ask the declaration - Property.Allows builds this from the declared set, so the "+
			"two cannot come apart. Written out by hand they agree until somebody adds a "+
			"value to the set and edits one of them (O171).",
			key, bad, value.Reason, want)
	}
}
