package guard

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/site"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The website is prose about the program, so its failure mode is not a crash.
// It is a page that is still up, still pretty, and no longer true. These
// guards exist for that one defect.
//
// Everything a visitor could check against the tool - the number of formats,
// the smallest file each can be, the exit codes, the presets, which systems
// have a binary, the version - is rendered from the program rather than typed
// into a page, and the whole published site is compared against what the
// program would produce right now. Adding a format and forgetting the site is
// therefore a red test rather than a page that lies until somebody notices.
//
// To take the new rendering after a deliberate change:
//
//	TFG_WRITE_SITE=1 go test ./internal/guard/ -run TestTheSiteSaysWhatTheToolSays
//
// then read the diff. It is the same shape as the screen references, and for
// the same reason: a picture or a page is easier to approve than to describe.

// siteOrigin is where the pages are served from. It is one string because
// every canonical link, every hreflang, the sitemap and the CNAME file are all
// derived from it, and two of them disagreeing would be a site that points
// search engines at an address it is not at.
const siteOrigin = "https://testingfilesgenerator.donislawdev.com"

// webRoot is the directory holding the site, source and published alike.
func webRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "web")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no web directory here: %v", err)
	}
	return root
}

// declaredDownloads is what the site tells a visitor to expect.
//
// It is declared rather than derived because the workflow states the command
// line targets literally and leaves the window architecture to whichever
// runner builds it. TestTheSiteOffersEveryBinaryTheReleaseBuilds checks this
// against the workflow and says in its own comment which half of it that
// proves.
func declaredDownloads() []site.Download {
	return []site.Download{
		{System: "Windows", CLI: []string{"amd64", "arm64"}, GUI: []string{"amd64"}},
		{System: "Linux", CLI: []string{"amd64", "arm64"}, GUI: []string{"amd64"}},
		{System: "macOS", CLI: []string{"arm64"}, GUI: []string{"arm64"}},
	}
}

// exitCodesInOrder is the frozen table of docs/CLI.md, read from the constants
// rather than copied. The order is the order a reader meets them.
func exitCodesInOrder() []int {
	return []int{
		cli.ExitOK,
		cli.ExitRuntime,
		cli.ExitUsage,
		cli.ExitRecipe,
		cli.ExitFormat,
		cli.ExitIO,
		cli.ExitSpace,
		cli.ExitVerify,
		cli.ExitPartial,
		cli.ExitInterrupted,
		cli.ExitTerminated,
	}
}

// factsFromTheProgram fills every number the pages may state.
func factsFromTheProgram() site.Facts {
	formats := make([]site.Format, 0, len(format.All()))
	for _, d := range format.All() {
		props := make([]site.Property, 0, len(d.Properties))
		for _, p := range d.Properties {
			props = append(props, site.Property{
				Name:    p.Name,
				Kind:    string(p.Kind),
				Min:     p.Min,
				Max:     p.Max,
				Unit:    p.Unit,
				Choices: append([]string(nil), p.Choices...),
			})
		}
		formats = append(formats, site.Format{
			ID:          d.ID,
			Extension:   d.Extension,
			Fidelity:    string(d.Fidelity),
			Determinism: string(d.Determinism),
			// The same number the command line prints, from the same call.
			// The structural floor and the accepted floor differ for formats
			// whose smallest file moves with the seed, and two surfaces
			// answering one question with two numbers is the defect this
			// whole file exists to stop.
			Smallest:   d.SmallestAccepted(format.Request{Label: true}),
			Padding:    d.Padding.Name,
			Oracle:     d.Oracle,
			Container:  d.Container,
			Properties: props,
		})
	}

	ids := make([]string, 0, len(preset.All()))
	for _, p := range preset.All() {
		ids = append(ids, p.ID)
	}

	return site.Facts{
		Version:   version.Version,
		Formats:   formats,
		ExitCodes: exitCodesInOrder(),
		Presets:   ids,
		Downloads: declaredDownloads(),
		// Fixed on purpose. See the comment on the field.
		Year: 2026,
		Repo: "https://github.com/donislawdev/TestingFilesGenerator",
		// The list of releases rather than whichever one is newest, since
		// 2026-09-03. "latest" skips a prerelease, so a page offering to
		// download a version it had just named would hand over a different one
		// the moment a release candidate existed - and nothing here compares
		// the two, because this guard compares the site against the CODE. The
		// button names no version now and leads to the list, which is true
		// whatever is published.
		Releases: "https://github.com/donislawdev/TestingFilesGenerator/releases",
		Issues:   "https://github.com/donislawdev/TestingFilesGenerator/issues",
		Support:  "https://donislawdev.com/support/",
		Origin:   siteOrigin,
	}
}

// languagesOnDisk reads every language description under web/content.
func languagesOnDisk(t *testing.T) []site.Language {
	t.Helper()
	content := filepath.Join(webRoot(t), "content")
	entries, err := os.ReadDir(content)
	if err != nil {
		t.Fatalf("reading the content directory: %v", err)
	}
	var langs []site.Language
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(content, e.Name(), "site.json"))
		if err != nil {
			t.Fatalf("reading the %s language description: %v", e.Name(), err)
		}
		var l site.Language
		if err := json.Unmarshal(b, &l); err != nil {
			t.Fatalf("reading the %s language description: %v", e.Name(), err)
		}
		if l.Code != e.Name() {
			t.Fatalf("the language in %s calls itself %q, so a page of it would be written somewhere nobody looks", e.Name(), l.Code)
		}
		langs = append(langs, l)
	}
	// The language served at the root goes first, so it is the one an
	// alternate list names before the others and the one a reader of the
	// sitemap meets first.
	sort.SliceStable(langs, func(i, j int) bool { return langs[i].Dir == "" && langs[j].Dir != "" })
	return langs
}

// siteUnderTest is the whole site as the program would render it now.
func siteUnderTest(t *testing.T) site.Site {
	t.Helper()
	root := webRoot(t)
	return site.Site{
		Facts:       factsFromTheProgram(),
		Languages:   languagesOnDisk(t),
		ContentDir:  filepath.Join(root, "content"),
		TemplateDir: filepath.Join(root, "templates"),
		AssetDir:    filepath.Join(root, "assets"),
		Extra: map[string]string{
			"assets/window.png": filepath.Join(repoRoot(t), ".github", "window.png"),
			"assets/icon.png":   filepath.Join(repoRoot(t), "internal", "gui", "icon", "chickpea.png"),
		},
	}
}

func TestTheSiteSaysWhatTheToolSays(t *testing.T) {
	s := siteUnderTest(t)
	rendered, err := s.Render()
	if err != nil {
		t.Fatalf("rendering the site: %v", err)
	}
	published := filepath.Join(webRoot(t), "public")

	if os.Getenv("TFG_WRITE_SITE") == "1" {
		writePublished(t, published, rendered)
		t.Log("the published site has been rewritten - read the diff before committing it")
		return
	}

	onDisk := readPublished(t, published)

	for path, want := range rendered {
		got, ok := onDisk[path]
		if !ok {
			t.Errorf("the program renders %s and the published site does not have it - rerun with TFG_WRITE_SITE=1", path)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("%s on the site is not what the program renders now: %s\nrerun with TFG_WRITE_SITE=1 and read the diff", path, firstDifference(string(got), string(want)))
		}
	}
	for path := range onDisk {
		if _, ok := rendered[path]; !ok {
			t.Errorf("the published site has %s and the program no longer renders it - rerun with TFG_WRITE_SITE=1", path)
		}
	}
}

// firstDifference names the line that parted, because a whole page in a
// failure message is unreadable where the failure happens - on a runner with
// no working copy to diff against.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return "line " + strconv.Itoa(i+1) + "\n  published: " + strings.TrimSpace(g[i]) + "\n  rendered:  " + strings.TrimSpace(w[i])
		}
	}
	return "one is longer: " + strconv.Itoa(len(g)) + " lines published, " + strconv.Itoa(len(w)) + " rendered"
}

func readPublished(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatalf("reading the published site: %v", err)
	}
	return out
}

// writePublished replaces the published site with what was rendered.
//
// It removes what is no longer rendered as well as writing what is. A page
// that was deleted from the content but left on disk would keep being served
// and keep being indexed, which is the same defect as a page that lies.
func writePublished(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for path := range readPublished(t, dir) {
		if _, keep := files[path]; !keep {
			if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
				t.Fatalf("removing %s: %v", path, err)
			}
		}
	}
	for path, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making room for %s: %v", path, err)
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
}

func TestTheSiteOffersEveryBinaryTheReleaseBuilds(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Skipf("no release workflow here: %v", err)
	}
	// Comments come out first, and the first version of this guard is the
	// reason. The workflow says "No darwin/amd64" in a comment explaining that
	// Intel Macs are deliberately unsupported, and a plain search for the
	// pattern read that sentence as a build target - so the guard reported a
	// download the release does not produce. A file that explains itself in
	// prose cannot be searched as if it were only declarations.
	workflow := withoutYamlComments(string(b))

	// The command line half is stated literally in the workflow, as a list of
	// GOOS/GOARCH pairs, so this half is proven rather than assumed.
	built := map[string]bool{}
	for _, m := range regexp.MustCompile(`(windows|linux|darwin)/(amd64|arm64)`).FindAllString(workflow, -1) {
		built[m] = true
	}
	if len(built) == 0 {
		t.Fatal("the release workflow names no build targets, so this guard is reading the wrong file")
	}

	goos := map[string]string{"Windows": "windows", "Linux": "linux", "macOS": "darwin"}
	offered := map[string]bool{}
	for _, d := range declaredDownloads() {
		for _, arch := range d.CLI {
			offered[goos[d.System]+"/"+arch] = true
		}
	}
	for target := range built {
		if !offered[target] {
			t.Errorf("the release builds %s and the site does not offer it, so somebody with that machine is told there is nothing for them", target)
		}
	}
	for target := range offered {
		if !built[target] {
			t.Errorf("the site offers %s and the release does not build it, so that download link leads to a file that is not there", target)
		}
	}

	// The window half is a weaker check and this says so rather than reading
	// as proof. The workflow builds the window on whatever architecture the
	// runner happens to be, so the architecture is a property of the runner
	// and is nowhere in the file. What is in the file is which systems get a
	// window at all, and that is what this compares.
	runners := map[string]string{"windows-latest": "Windows", "ubuntu-latest": "Linux", "macos-latest": "macOS"}
	withWindow := map[string]bool{}
	for runner, system := range runners {
		if strings.Contains(workflow, "- "+runner) {
			withWindow[system] = true
		}
	}
	for _, d := range declaredDownloads() {
		if len(d.GUI) > 0 && !withWindow[d.System] {
			t.Errorf("the site offers a window for %s and the release workflow builds none there", d.System)
		}
		if len(d.GUI) == 0 && withWindow[d.System] {
			t.Errorf("the release workflow builds a window on %s and the site does not offer it", d.System)
		}
	}
}

// withoutYamlComments drops what a reader wrote for other readers.
//
// A hash starting a line, or following whitespace, begins a comment in YAML.
// That is not the whole of the grammar - a hash inside a quoted string is
// ordinary text - but the workflow this reads has none, and the alternative is
// a YAML parser in a guard whose question is about three lines of it.
func withoutYamlComments(in string) string {
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		for at, r := range line {
			if r != '#' {
				continue
			}
			if at == 0 || line[at-1] == ' ' || line[at-1] == '\t' {
				lines[i] = line[:at]
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func TestEveryLanguageDescribesEverythingTheProgramCanProduce(t *testing.T) {
	facts := factsFromTheProgram()
	for _, lang := range languagesOnDisk(t) {
		for _, code := range facts.ExitCodes {
			if _, ok := lang.Endings[strconv.Itoa(code)]; !ok {
				t.Errorf("exit code %d has no meaning in %s, so that row of the table would be blank", code, lang.Code)
			}
		}
		for _, id := range facts.Presets {
			if _, ok := lang.Presets[id]; !ok {
				t.Errorf("the preset %q has no question in %s", id, lang.Code)
			}
		}
		for _, f := range facts.Formats {
			// A format with no independent reader says "none" in the registry,
			// which is an identifier rather than a sentence. The page turns it
			// into words, and those words have to exist in every language or
			// the column reads as English on a Polish page.
			if f.Oracle == "none" {
				if _, ok := lang.Terms["oracleNone"]; !ok {
					t.Errorf("%s has no independent reader and %s has no words for that", f.ID, lang.Code)
				}
			}
			for _, p := range f.Properties {
				if _, ok := lang.Terms[p.Kind]; !ok {
					t.Errorf("%s.%s is a %q and %s has no word for that kind", f.ID, p.Name, p.Kind, lang.Code)
				}
				if p.Unit != "" {
					if _, ok := lang.Terms[p.Unit]; !ok {
						t.Errorf("%s.%s counts %q and %s has no word for it, so the page would read as half translated", f.ID, p.Name, p.Unit, lang.Code)
					}
				}
			}
		}
	}
}

func TestEveryPageExistsInEveryLanguage(t *testing.T) {
	langs := languagesOnDisk(t)
	if len(langs) < 2 {
		t.Skip("one language, so nothing can be missing from another")
	}
	keys := map[string]int{}
	for _, l := range langs {
		for _, p := range l.Pages {
			keys[p.Key]++
		}
	}
	for key, count := range keys {
		if count != len(langs) {
			t.Errorf("the page %q exists in %d of %d languages, so its language switch leads nowhere", key, count, len(langs))
		}
	}
	for _, l := range langs {
		for _, p := range l.Pages {
			body := filepath.Join(webRoot(t), "content", l.Code, p.Key+".html")
			if _, err := os.Stat(body); err != nil {
				t.Errorf("the %s page %q is listed and its text is missing: %v", l.Code, p.Key, err)
			}
			if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Description) == "" {
				t.Errorf("the %s page %q has no title or no description, and a search engine writes its own when we do not", l.Code, p.Key)
			}
		}
	}
}

// TestThePolishTextStaysOnThePolishPages holds the boundary the owner set when
// the site was allowed into this repository on 2026-08-26.
//
// D9 says text in the repository is English, and the criterion is the place
// rather than the reader. The site extends that rule to a second language and
// the extension is only as good as its border - so the border is machine
// checked here rather than remembered. Anything outside a pl directory that
// carries a character above ASCII is either a translation that leaked or an
// English page somebody typed a curly quote into, and both are defects.
func TestThePolishTextStaysOnThePolishPages(t *testing.T) {
	root := webRoot(t)
	text := map[string]bool{".html": true, ".json": true, ".css": true, ".xml": true, ".txt": true}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !text[strings.ToLower(filepath.Ext(p))] {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		slashed := filepath.ToSlash(rel)
		polish := strings.Contains(slashed, "/pl/") || strings.HasPrefix(slashed, "pl/")
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		for n, line := range strings.Split(string(b), "\n") {
			for col, r := range line {
				if r > 127 && !polish {
					t.Errorf("web/%s:%d:%d holds %q - only the Polish pages may carry it", slashed, n+1, col+1, r)
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the site: %v", err)
	}
}

// TestTheSiteFollowsThePunctuationRule applies rule 13 to the site.
//
// Fenced samples are left alone, the same way the README guard leaves licence
// text alone: a command or a snippet of YAML is quoted rather than written,
// and tidying somebody else's syntax would break it. Entities are skipped
// because the semicolon in &lt; is markup rather than punctuation.
func TestTheSiteFollowsThePunctuationRule(t *testing.T) {
	root := filepath.Join(webRoot(t), "content")
	pre := regexp.MustCompile(`(?s)<pre>.*?</pre>`)
	entity := regexp.MustCompile(`&[a-zA-Z]+;|&#[0-9]+;`)
	banned := map[rune]string{
		';': "a semicolon", '–': "an en dash", '—': "an em dash",
		'‘': "a curly quote", '’': "a curly quote",
		'“': "a curly quote", '”': "a curly quote", '…': "an ellipsis",
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".html" && ext != ".json" {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		body := entity.ReplaceAllString(pre.ReplaceAllString(string(b), ""), "")
		rel, _ := filepath.Rel(root, p)
		for n, line := range strings.Split(body, "\n") {
			for _, r := range line {
				if what, bad := banned[r]; bad {
					t.Errorf("web/content/%s:%d holds %s - see rule 13", filepath.ToSlash(rel), n+1, what)
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the content: %v", err)
	}
}

// TestEveryLinkOnTheSiteLeadsSomewhere follows every internal address.
//
// A dead link on a page whose whole job is to end in a download costs the
// download. Outside addresses are left alone on purpose - checking them would
// need the network, which this project does not do in a test.
func TestEveryLinkOnTheSiteLeadsSomewhere(t *testing.T) {
	s := siteUnderTest(t)
	rendered, err := s.Render()
	if err != nil {
		t.Fatalf("rendering the site: %v", err)
	}
	href := regexp.MustCompile(`(?:href|src)="(/[^"#]*)"`)
	for page, body := range rendered {
		if !strings.HasSuffix(page, ".html") {
			continue
		}
		for _, m := range href.FindAllStringSubmatch(string(body), -1) {
			target := strings.TrimPrefix(m[1], "/")
			if strings.HasSuffix(target, "/") || target == "" {
				target += "index.html"
			}
			if _, ok := rendered[target]; !ok {
				t.Errorf("%s links to /%s and nothing is published there", page, m[1])
			}
		}
	}
}

// TestTheSitemapNeedsNoSchemaButItsOwn asks that the sitemap carry nothing a
// reader would have to fetch a second schema to check.
//
// This guard exists because of a measurement rather than a preference. The
// sitemap schema lets an element from another namespace in, but only through a
// particle it insists on validating strictly - which tells a reader it must
// hold that other schema as well, and a reader without it turns down the whole
// file rather than the one element. We carried xhtml alternates in here until
// 2026-08-27 and paid exactly that. They are still on every page, in the head,
// which is where they are read from anyway.
//
// It asks the renderer rather than the published file because the renderer is
// where the answer is decided. TestTheSiteSaysWhatTheToolSays is what holds
// the published file to it.
func TestTheSitemapNeedsNoSchemaButItsOwn(t *testing.T) {
	const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

	s := siteUnderTest(t)
	rendered, err := s.Render()
	if err != nil {
		t.Fatalf("rendering the site: %v", err)
	}
	body, ok := rendered["sitemap.xml"]
	if !ok {
		t.Fatal("the program renders no sitemap.xml at all, so nothing here is being checked")
	}

	locations := 0
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("the sitemap does not parse, so no reader will take it: %v", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Space != sitemapNamespace {
			t.Errorf("the sitemap carries <%s> from %q - a reader holding only the sitemap schema has to find that one too, and turns the file down when it cannot", start.Name.Local, start.Name.Space)
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Space == "xmlns" {
				t.Errorf("the sitemap declares a second namespace %q as %q, and the only reason to declare one is to carry an element from it", attribute.Name.Local, attribute.Value)
			}
			if attribute.Name.Space == "" && attribute.Name.Local == "xmlns" && attribute.Value != sitemapNamespace {
				t.Errorf("the sitemap says it is %q, which is not the sitemap schema", attribute.Value)
			}
		}
		if start.Name.Local != "loc" {
			continue
		}
		locations++
		var address string
		if err := decoder.DecodeElement(&address, &start); err != nil {
			t.Fatalf("reading an address out of the sitemap: %v", err)
		}
		if !strings.HasPrefix(address, siteOrigin+"/") {
			t.Errorf("the sitemap names %q, which is not on the site it belongs to", address)
		}
	}

	pages := 0
	for _, language := range s.Languages {
		pages += len(language.Pages)
	}
	if locations != pages {
		t.Errorf("the site has %d pages and the sitemap names %d of them, so a crawler reading it is told about the wrong set", pages, locations)
	}
}
