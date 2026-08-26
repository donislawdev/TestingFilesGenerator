// Package site renders the project website from content files and facts that
// come from the tool itself.
//
// It exists so the website cannot quietly disagree with the program. The
// numbers a visitor reads - how many formats there are, how small each one can
// be, what an exit code means, which systems have a binary - are not typed
// into a page. They arrive here as data and are rendered, and a guard compares
// what this produces against what is committed under web/public. A format
// added without the site being regenerated turns that guard red.
//
// The split between a number and the word beside it is the point of most of
// what follows. A range comes from the registry, because only the registry
// knows it. The noun after the range comes from the language file, because a
// Polish page saying "pixels" is a page that was translated halfway. Anything
// the registry names and no language explains is a failure here rather than a
// silently English phrase on a Polish page.
//
// This package imports nothing else from the project on purpose. The facts it
// needs live on four different layers, and reaching for them here would put a
// renderer above the command line in the layer map for no reason. The caller
// assembles them instead - see internal/guard/site_test.go, which is external
// to everything and already allowed to look anywhere.
//
// Design notes and the reasoning behind the site itself are in
// docs/WEBSITE-PLAN.md.
package site

import (
	"strings"
)

// Property is one setting a format understands.
//
// It carries the parts rather than a finished sentence, because the sentence
// differs by language and the parts do not.
type Property struct {
	Name    string
	Kind    string
	Min     int64
	Max     int64
	Unit    string
	Choices []string
}

// Format is one entry of the registry, flattened for display.
//
// Smallest is the number the tool would accept, not the structural floor. They
// differ for formats whose smallest file moves with the seed, and the command
// line prints the accepted one - so the site prints the same, or the two
// surfaces would answer one question with two numbers.
type Format struct {
	ID          string
	Extension   string
	Fidelity    string
	Determinism string
	Smallest    int64
	Padding     string
	Oracle      string
	Container   bool
	Properties  []Property
}

// Ending is one row of the frozen exit code table, with the meaning filled in
// from the language being rendered.
type Ending struct {
	Code    int
	Meaning string
}

// Preset is one ready made set of files, named by the question it answers.
//
// The identifier comes from the registry and the question from the language
// file, for the same reason the units do: the registry states its question in
// English, and a Polish page carrying it would be a page translated halfway.
type Preset struct {
	ID       string
	Question string
}

// Download says which architectures a system actually gets.
//
// Both lists are here because they differ, and a page that flattened them into
// "Windows, macOS, Linux" would be untrue for anybody on an Intel Mac or on
// Windows for ARM who wants the window. An empty GUI list means no window
// binary ships for that system.
type Download struct {
	System string
	CLI    []string
	GUI    []string
}

// Facts is everything the pages may state about the program.
//
// Every field is filled from the program, never from prose. Adding a field
// here means finding the place in the code that owns that fact, not writing
// the value down.
type Facts struct {
	Version   string
	Formats   []Format
	ExitCodes []int
	Presets   []string
	Downloads []Download

	// Year is fixed rather than taken from the clock. A footer that rendered
	// the current year would make the committed pages differ from freshly
	// rendered ones every first of January, and the guard would go red for a
	// reason nobody could find.
	Year int

	Repo     string
	Releases string
	Issues   string
	Support  string
	Origin   string
}

// FormatCount is how many formats this build has, for prose that needs it.
func (f Facts) FormatCount() int { return len(f.Formats) }

// Containers lists the formats that can hold other files.
func (f Facts) Containers() []Format {
	var out []Format
	for _, d := range f.Formats {
		if d.Container {
			out = append(out, d)
		}
	}
	return out
}

// WithProperties lists the formats that take settings of their own.
func (f Facts) WithProperties() []Format {
	var out []Format
	for _, d := range f.Formats {
		if len(d.Properties) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// QA is one question and its answer.
//
// The answer is plain text with no markup, and that is deliberate. The same
// pair is rendered twice - once for a reader and once as the structured data
// a search engine reads - and a single source is the only way the two cannot
// come to say different things. Code is an optional command shown under the
// answer, left out of the structured data because a search engine wants the
// sentence, not the shell line.
type QA struct {
	Q    string `json:"q"`
	A    string `json:"a"`
	Code string `json:"code,omitempty"`
}

// Page is one document, in one language.
//
// Key pairs a page with its translation. Two pages sharing a key are the same
// document in two languages, which is what the language switcher and the
// hreflang links are built from - so a missing translation is a missing key
// rather than a broken link.
type Page struct {
	Key         string `json:"key"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Nav         string `json:"nav"`
}

// Language is one whole version of the site.
//
// Dir is the path prefix. It is empty for the language served at the root,
// which is the one search engines are pointed at by x-default.
//
// Endings and Terms are the two places where a word has to exist for every
// value the program can produce. Endings is keyed by the exit code written out
// in decimal, Terms by the kind or unit exactly as the registry spells it.
type Language struct {
	Code    string            `json:"code"`
	Name    string            `json:"name"`
	Dir     string            `json:"dir"`
	Words   map[string]string `json:"words"`
	Endings map[string]string `json:"endings"`
	Terms   map[string]string `json:"terms"`
	Presets map[string]string `json:"presets"`
	Pages   []Page            `json:"pages"`
	Faq     []QA              `json:"faq"`
}

// Site is everything needed to render.
type Site struct {
	Facts     Facts
	Languages []Language

	// ContentDir holds one directory per language code, each with one HTML
	// fragment per page key. TemplateDir holds the shell and the partials.
	// AssetDir is copied through untouched.
	ContentDir  string
	TemplateDir string
	AssetDir    string

	// Extra are files copied in from elsewhere in the repository, keyed by
	// their path under the published root. The window screenshot and the
	// application icon come this way rather than being duplicated, so the
	// picture on the site is the picture the project already keeps.
	Extra map[string]string
}

// Alternate is one hreflang entry.
type Alternate struct {
	Lang string
	URL  string
}

// NavItem is one link in the header.
type NavItem struct {
	Label   string
	URL     string
	Current bool
}

// Switch is the link to this page in another language.
type Switch struct {
	Name string
	URL  string
	Code string
}

// Origin is the address the site is served from, without a trailing slash.
func (s Site) Origin() string { return strings.TrimSuffix(s.Facts.Origin, "/") }

// Host is the name alone, which is what the CNAME file beside the pages holds.
//
// It is derived rather than written down a second time. That file is what
// tells the host which address these pages answer at, and an address that
// disagreed with the one in the sitemap would break the site quietly - every
// canonical link would point somewhere the pages are not served.
func (s Site) Host() string {
	host := s.Origin()
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	return strings.TrimSuffix(host, "/")
}

// rootLanguage is the one served without a path prefix.
func (s Site) rootLanguage() (Language, bool) {
	for _, l := range s.Languages {
		if l.Dir == "" {
			return l, true
		}
	}
	return Language{}, false
}

// find looks up a page by the key that pairs translations.
func find(l Language, key string) (Page, bool) {
	for _, p := range l.Pages {
		if p.Key == key {
			return p, true
		}
	}
	return Page{}, false
}
