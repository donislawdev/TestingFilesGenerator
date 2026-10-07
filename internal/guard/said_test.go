package guard

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The engine's sentences in the window's language - docs/OKNO-PO-POLSKU-
// 2026-10-05.md, core/said.go for the sentence and text/said.go for the
// window. The English of every refusal and note lives at the call that says
// it, so these guards read it from there: a translator's copy that agrees with
// the code, every language complete and current, and every sentence written so
// that a translation can be made of it at all.

// saidDir is where every language keeps the engine's sentences.
var saidDir = filepath.Join(localeDir, text.SaidFolder)

// writeSaidCatalogue writes said/en.json out of the code.
const writeSaidCatalogue = "TFG_WRITE_SAID_CATALOGUE"

// saidCall is one sentence as the code says it.
type saidCall struct {
	id, one, other string
	plural         bool
	names          []string
	pkg, at        string
}

// saidID is what an id looks like: the package that says it, then a name. A
// package name may carry digits after its first letter, as mp4 does.
var saidID = regexp.MustCompile(`^[a-z][a-z0-9]*\.[A-Z][A-Za-z0-9]*$`)

// saidCalls is every core.Says and core.SaysN in the module, with what is
// wrong with how each one is written.
func saidCalls(t *testing.T) ([]saidCall, []string) {
	t.Helper()
	var calls []saidCall
	var wrong []string
	root := repoRoot(t)
	for _, p := range packages(t) {
		for _, path := range p.files {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			rel, _ := filepath.Rel(root, path)
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				fn := saysCalled(call, file.Name.Name)
				if fn == "" {
					return true
				}
				at := filepath.ToSlash(rel) + ":" + strconv.Itoa(fset.Position(call.Pos()).Line)
				c, problem := readSaid(call, fn, file.Name.Name)
				if problem != "" {
					wrong = append(wrong, at+": "+problem)
					return true
				}
				c.pkg, c.at = p.rel, at
				calls = append(calls, c)
				return true
			})
		}
	}
	return calls, wrong
}

// saysCalled is "Says" or "SaysN" when the call is one, and empty otherwise.
func saysCalled(call *ast.CallExpr, inPackage string) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok && x.Name == "core" && (fn.Sel.Name == "Says" || fn.Sel.Name == "SaysN") {
			return fn.Sel.Name
		}
	case *ast.Ident:
		if inPackage == "core" && (fn.Name == "Says" || fn.Name == "SaysN") {
			return fn.Name
		}
	}
	return ""
}

// readSaid is one call read into its parts, or what stops it being read.
func readSaid(call *ast.CallExpr, fn, inPackage string) (saidCall, string) {
	var c saidCall
	fixed := 2
	if fn == "SaysN" {
		fixed, c.plural = 3, true
	}
	if len(call.Args) < fixed || call.Ellipsis.IsValid() {
		return c, "the id and the layout are not all written out at the call"
	}
	var ok bool
	if c.id, ok = literalText(call.Args[0]); !ok {
		return c, "the id is not a literal, so no translator can find it"
	}
	if c.other, ok = literalText(call.Args[fixed-1]); !ok {
		return c, c.id + ": the layout is not a literal, so no translator can see it"
	}
	c.one = c.other
	if c.plural {
		if c.one, ok = literalText(call.Args[1]); !ok {
			return c, c.id + ": the layout for one is not a literal"
		}
	}
	for _, arg := range call.Args[fixed:] {
		name, ok := argName(arg, inPackage)
		if !ok {
			return c, c.id + ": a value is not written as core.A with a literal name"
		}
		c.names = append(c.names, name)
	}
	return c, ""
}

// argName is the name of one core.A at a call.
func argName(e ast.Expr, inPackage string) (string, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return "", false
	}
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		x, ok := fn.X.(*ast.Ident)
		if !ok || x.Name != "core" || fn.Sel.Name != "A" {
			return "", false
		}
	case *ast.Ident:
		if inPackage != "core" || fn.Name != "A" {
			return "", false
		}
	default:
		return "", false
	}
	return literalText(call.Args[0])
}

// literalText is a string literal, or literals joined with +, as its text.
func literalText(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(x.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		a, okA := literalText(x.X)
		b, okB := literalText(x.Y)
		return a + b, okA && okB
	case *ast.ParenExpr:
		return literalText(x.X)
	}
	return "", false
}

// engineSentences is every sentence by id, failing on anything a translation
// could not be made of.
func engineSentences(t *testing.T) map[string]saidCall {
	t.Helper()
	calls, wrong := saidCalls(t)
	for _, w := range wrong {
		t.Error(w)
	}
	// The damage package alone says nine. Fewer means the walk found nothing,
	// and every guard below would compare nothing with nothing.
	if len(calls) < 9 {
		t.Fatalf("the code says %d sentence(s) through core.Says, so the walk did not find them", len(calls))
	}
	out := map[string]saidCall{}
	for _, c := range calls {
		if problem := laidOutForTranslation(c); problem != "" {
			t.Errorf("%s: %s: %s", c.at, c.id, problem)
		}
		if seen, twice := out[c.id]; twice && (seen.one != c.one || seen.other != c.other ||
			strings.Join(seen.names, ",") != strings.Join(c.names, ",")) {
			t.Errorf("%s and %s say %s two ways - one id is one sentence", seen.at, c.at, c.id)
		}
		out[c.id] = c
	}
	return out
}

// laidOutForTranslation is what is wrong with one sentence, or nothing.
func laidOutForTranslation(c saidCall) string {
	if !saidID.MatchString(c.id) {
		return "the id is not package.Name"
	}
	seen := map[string]bool{}
	for _, n := range c.names {
		if seen[n] || n == core.SettingField || n == "" {
			return "the value " + strconv.Quote(n) + " is named twice, empty, or by the name the setting slot keeps"
		}
		seen[n] = true
	}
	if c.plural && !seen[core.CountArg] {
		return "a sentence with two forms has no value named Count to choose between them"
	}
	for _, layout := range []string{c.one, c.other} {
		if strings.Contains(layout, "{{") {
			return "the layout holds {{, which a translation would read as a field"
		}
		directives := core.DirectivesOf(layout)
		// A count that only chooses the form - "holds" against "hold" - is the
		// last value and has no place in the layout.
		unsaid := c.plural && len(c.names) > 0 && c.names[len(c.names)-1] == core.CountArg && len(directives) == len(c.names)-1
		if len(directives) != len(c.names) && !unsaid {
			return "the layout takes " + strconv.Itoa(len(directives)) + " value(s) and is given " + strconv.Itoa(len(c.names))
		}
		wraps := 0
		for _, d := range directives {
			if strings.ContainsAny(d, "*[") {
				return "the layout takes a width or a position from a value: " + d
			}
			if strings.HasSuffix(d, "w") {
				wraps++
			}
		}
		// A refusal unwraps to the one error it wraps, the way fmt.Errorf
		// with one %w does. Two would be two refusals to a window that
		// spreads an error carrying several, and the sentence round them
		// would be lost (core.Refusal.Unwrap).
		if wraps > 1 {
			return "the layout wraps " + strconv.Itoa(wraps) + " errors with %w, and a sentence wraps one at most"
		}
	}
	return ""
}

// TestEverySentenceTheEngineSaysCanBeTranslated holds every core.Says to the
// shape a translation needs: a literal id and layout a guard can read, named
// values, as many values as the layout takes, and no width or position taken
// from a value - a translation fills named fields, and a value without one
// name and one place cannot be put into it.
func TestEverySentenceTheEngineSaysCanBeTranslated(t *testing.T) {
	engineSentences(t)
}

// saidReflection is said/en.json as the code writes it.
func saidReflection(t *testing.T) []byte {
	t.Helper()
	entries := map[string]map[string]string{}
	for id, c := range engineSentences(t) {
		other := core.NamedLayout(c.other, c.names)
		entry := map[string]string{"description": saidWhere(c), "hash": saidHash(c), "other": other}
		if c.plural {
			entry["one"] = core.NamedLayout(c.one, c.names)
		}
		entries[id] = entry
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// saidHash is what another language records of the English it translated.
func saidHash(c saidCall) string {
	if c.plural {
		return englishHash(core.NamedLayout(c.one, c.names) + "\n" + core.NamedLayout(c.other, c.names))
	}
	return englishHash(core.NamedLayout(c.other, c.names))
}

// saidWhere is the sentence a translator reads beside the English.
func saidWhere(c saidCall) string {
	where := "Said by " + c.pkg + " - a refusal under the box it is about or at the foot of the form, or a note after a run."
	if len(c.names) == 0 {
		return where
	}
	fields := make([]string, len(c.names))
	for i, n := range c.names {
		fields[i] = "{{." + n + "}}"
	}
	return where + " Carries " + strings.Join(fields, ", ") +
		", written the way the program writes them - keep each spelled exactly that way."
}

// TestTheEnglishCopyOfTheEngineSentencesSaysWhatTheCodeSays keeps the copy a
// translator works from equal to the code, and is where the hashes every other
// language is held to come from.
func TestTheEnglishCopyOfTheEngineSentencesSaysWhatTheCodeSays(t *testing.T) {
	want := saidReflection(t)
	path := filepath.Join(saidDir, "en.json")
	if os.Getenv(writeSaidCatalogue) == "1" {
		if err := os.MkdirAll(saidDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s, %d B", path, len(want))
		return
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("there is no %s: %v\nRun: %s=1 go test ./internal/guard/ -run ^TestTheEnglishCopyOfTheEngineSentencesSaysWhatTheCodeSays$",
			path, err, writeSaidCatalogue)
	}
	if !bytes.Equal(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n")), want) {
		t.Errorf("%s is not what the code says - a sentence was added, changed or removed.\n"+
			"Run: %s=1 go test ./internal/guard/ -run ^TestTheEnglishCopyOfTheEngineSentencesSaysWhatTheCodeSays$\n"+
			"and then bring every other language in %s up to it.", path, writeSaidCatalogue, saidDir)
	}
}

// TestEveryLanguageSaysEverythingTheEngineSays holds every language the window
// carries to every sentence of the engine: there, translated from the English
// the code says now, and carrying exactly the values the English carries - a
// translation without {{.Fix}} is well formed and drops the part of a refusal
// that says what to do (D6).
func TestEveryLanguageSaysEverythingTheEngineSays(t *testing.T) {
	sentences := engineSentences(t)
	files := catalogueFilesIn(t, saidDir)
	for tag := range catalogueFiles(t) {
		if _, has := files[tag]; !has {
			t.Errorf("the window speaks %s and %s has no %s.json, so the engine's refusals are English there",
				tag, saidDir, tag)
		}
	}
	if len(files) < 2 {
		t.Fatal("no language but English has the engine's sentences, so this guard asserts about nothing")
	}
	for tag, entries := range files {
		if tag == text.English {
			continue
		}
		languageSaysTheEngine(t, tag, entries, sentences)
	}
}

// languageSaysTheEngine holds one language to every sentence.
func languageSaysTheEngine(t *testing.T, tag string, entries map[string]map[string]string, sentences map[string]saidCall) {
	t.Helper()
	ids := make([]string, 0, len(sentences))
	for id := range sentences {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		c := sentences[id]
		entry, has := entries[id]
		switch {
		case !has || entry["other"] == "":
			t.Errorf("%s has no %s, so the window says it in English: %q", tag, id, core.NamedLayout(c.other, c.names))
		case entry["hash"] != saidHash(c):
			t.Errorf("%s translates %s from an English the code no longer says. It says now:\n  %q\n"+
				"Translate it again and record hash %s.", tag, id, core.NamedLayout(c.other, c.names), saidHash(c))
		default:
			for form, said := range entry {
				if form == "hash" || form == "description" {
					continue
				}
				if problem := fieldsOf(said, c, form); problem != "" {
					t.Errorf("%s %s (%s): %s\n  %q", tag, id, form, problem, said)
				}
			}
		}
	}
	for id := range entries {
		if _, known := sentences[id]; !known {
			t.Errorf("%s carries %s, which the code no longer says", tag, id)
		}
	}
}

// fieldFound is one {{.Name}} in a translation.
var fieldFound = regexp.MustCompile(`{{\s*\.([A-Za-z0-9]+)\s*}}`)

// fieldsOf is what is wrong with the fields one form of a translation uses.
func fieldsOf(said string, c saidCall, form string) string {
	want := map[string]bool{}
	for _, n := range c.names {
		want[n] = true
	}
	if strings.Contains(c.other, core.SettingSlot) {
		want[core.SettingField] = true
	}
	have := map[string]bool{}
	for _, m := range fieldFound.FindAllStringSubmatch(said, -1) {
		have[m[1]] = true
	}
	if strings.Contains(said, core.ArticleSlot) || strings.Contains(said, core.SettingSlot) {
		return "carries a slot of the English layout, which a translation writes as {{.Setting}}"
	}
	for n := range have {
		if !want[n] {
			return "uses {{." + n + "}}, which the sentence does not carry"
		}
	}
	for n := range want {
		// The number may be left out of every form when the English does not
		// print it, and of the form for one when it does - "one file".
		countOptional := n == core.CountArg && (form != "other" || len(core.DirectivesOf(c.other)) < len(c.names))
		if !have[n] && !countOptional {
			return "leaves out {{." + n + "}}, so part of what the English says is missing"
		}
	}
	values := map[string]any{}
	for n := range want {
		values[n] = "x"
	}
	tmpl, err := template.New("").Parse(said)
	if err != nil {
		return "is not a template: " + err.Error()
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, values); err != nil || strings.Contains(out.String(), "<no value>") {
		return "does not render with the values it is given"
	}
	return ""
}

// said is a sentence a guard makes up, for a refusal built by hand. Its
// English is the text itself.
func said(text string) core.Said { return core.Says("guard.Text", "%s", core.A("Text", text)) }

// enginePackages are the packages whose refusals and notes reach a person
// through both surfaces, and so are said as sentences.
var enginePackages = []string{"internal/core", "internal/engine", "internal/recipe", "internal/preset",
	"internal/damage", "internal/format", "internal/tool", "internal/manifest"}

// TestNoRefusalOfTheEngineIsWrittenAsBareText holds the engine to saying every
// error as a sentence: core.Says, or core.Defect around an error only a fault
// in the program can produce. An fmt.Errorf or an errors.New anywhere else is
// a refusal a window can only show in English.
//
// One exception, by its shape: the publishing calls wrap errors.ErrUnsupported
// so the caller can ask errors.Is and fall back. That error is a signal between
// two functions, and no person reads it.
func TestNoRefusalOfTheEngineIsWrittenAsBareText(t *testing.T) {
	root := repoRoot(t)
	found := 0
	for _, p := range packages(t) {
		if !insideAny(p.rel, enginePackages) {
			continue
		}
		for _, path := range p.files {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			found++
			for _, bare := range bareErrors(file) {
				rel, _ := filepath.Rel(root, path)
				if strings.HasPrefix(filepath.Base(path), "publish_") && wrapsUnsupported(bare) {
					continue
				}
				t.Errorf("%s:%d builds an error from bare text. Say it with core.Says (core.Refuse, core.RefuseAbout), "+
					"or wrap it in core.Defect when only a fault in the program can produce it",
					filepath.ToSlash(rel), fset.Position(bare.Pos()).Line)
			}
		}
	}
	if found < 50 {
		t.Fatalf("read %d file(s) of the engine, so the walk did not find it", found)
	}
}

func insideAny(rel string, roots []string) bool {
	rel = filepath.ToSlash(rel)
	for _, r := range roots {
		if rel == r || strings.HasPrefix(rel, r+"/") {
			return true
		}
	}
	return false
}

// bareErrors is every fmt.Errorf and errors.New in a file that is not the
// argument of a Defect.
func bareErrors(file *ast.File) []*ast.CallExpr {
	inDefect := map[*ast.CallExpr]bool{}
	var all []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if calledAs(call, "core", "Defect") || (file.Name.Name == "core" && isIdentCall(call, "Defect")) {
			for _, a := range call.Args {
				if inner, ok := a.(*ast.CallExpr); ok {
					inDefect[inner] = true
				}
			}
		}
		if calledAs(call, "fmt", "Errorf") || calledAs(call, "errors", "New") {
			all = append(all, call)
		}
		return true
	})
	var out []*ast.CallExpr
	for _, c := range all {
		if !inDefect[c] {
			out = append(out, c)
		}
	}
	return out
}

func calledAs(call *ast.CallExpr, pkg, fn string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg
}

func isIdentCall(call *ast.CallExpr, fn string) bool {
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == fn
}

// wrapsUnsupported is fmt.Errorf("%w: %w", errors.ErrUnsupported, ...).
func wrapsUnsupported(call *ast.CallExpr) bool {
	if len(call.Args) < 2 {
		return false
	}
	layout, ok := literalText(call.Args[0])
	sel, isSel := call.Args[1].(*ast.SelectorExpr)
	return ok && layout == "%w: %w" && isSel && sel.Sel.Name == "ErrUnsupported"
}
