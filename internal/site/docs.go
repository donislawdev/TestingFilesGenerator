// This file holds the documentation: the hub at /docs/ and the pages under it,
// most of them written in English alone.
//
// The owner decided on 2026-11-24 that the documentation is English only,
// because reference text in twenty two languages cannot be kept true. The hub
// stays in every language, since search engines know its addresses, and leads
// a reader to the English pages with a line in their own language about each
// (docs/DOCS-SITE-2026-11-24.md section 4.1).

package site

import (
	"fmt"
	"sort"
)

// docsKey is the key of the documentation hub, the page every page of the
// documentation sits under.
const docsKey = "docs"

// GuideText is what a translated hub says about one English page.
type GuideText struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// DocLink is one page as the hub and the menu of the documentation list it.
type DocLink struct {
	Key     string
	Title   string
	Text    string
	URL     string
	Current bool
	// English marks a link from a translated page to a page written in
	// English only, so a reader knows before following it.
	English bool
	// Anchors are the sections of the old single page of documentation that
	// this page took over, so an address with one of them still lands on it.
	Anchors []string
}

// oldDocsAnchors are the sections /docs/ had until 2026-11-24, when it was one
// page, and the page that took each over. Links with these fragments were
// published for three months, so the hub keeps every one as the id of the
// card that leads on.
var oldDocsAnchors = map[string]string{
	"commands":     "guide-commands",
	"one-file":     "guide-start",
	"damage":       "damage",
	"recipes":      "guide-recipes",
	"expectations": "guide-recipes",
	"manifest":     "guide-manifest",
	"presets":      "presets",
	"ci":           "guide-commands",
	"window":       "guide-window",
}

// anchorsOf is every old section that leads to the page with this key.
func anchorsOf(key string) []string {
	var out []string
	for anchor, to := range oldDocsAnchors {
		if to == key {
			out = append(out, anchor)
		}
	}
	sort.Strings(out)
	return out
}

// docLinks is every page under the hub, in the order the root language lists
// them, as a reader of one language sees them.
//
// A page the language has is its own. A page written in English alone is the
// root language's, described by the line this language wrote about it, and a
// language without that line is an error rather than an English title on a
// translated hub.
func (s Site) docLinks(lang Language, current Page) ([]DocLink, error) {
	root, ok := s.rootLanguage()
	if !ok {
		return nil, fmt.Errorf("no language is served at the root, so the documentation has no pages to list")
	}
	var out []DocLink
	for _, p := range root.Pages {
		if p.Parent != docsKey {
			continue
		}
		if own, ok := find(lang, p.Key); ok {
			out = append(out, DocLink{Key: own.Key, Title: own.Nav, Text: own.Description, URL: pageURL(lang, own),
				Current: own.Key == current.Key, Anchors: anchorsOf(own.Key)})
			continue
		}
		text := lang.Docs.guide(p.Key)
		if text.Title == "" || text.Text == "" {
			return nil, fmt.Errorf("the English page %q has no title or line written in %s for the documentation hub", p.Key, lang.Code)
		}
		out = append(out, DocLink{Key: p.Key, Title: text.Title, Text: text.Text, URL: pageURL(root, p),
			English: true, Anchors: anchorsOf(p.Key)})
	}
	return out, nil
}

// PagesOf is the pages with these keys in the language being rendered, as
// cards for the hub: the pages of the product the documentation leads to.
func (v view) PagesOf(keys ...string) ([]DocLink, error) {
	out := make([]DocLink, 0, len(keys))
	for _, key := range keys {
		p, ok := find(v.Lang, key)
		if !ok {
			return nil, fmt.Errorf("the %s page %q links to the page %q, which is not there", v.Lang.Code, v.Page.Key, key)
		}
		out = append(out, DocLink{Key: key, Title: p.Nav, Text: p.Description, URL: pageURL(v.Lang, p), Anchors: anchorsOf(key)})
	}
	return out, nil
}

// nearestIn is the page of another language a reader of an untranslated page
// is sent to: the closest page above it that the language has, or its home.
//
// The walk is bounded by the number of pages, so two pages naming each other
// as parent end it rather than loop.
func nearestIn(other, from Language, page Page) Page {
	key := page.Parent
	for range from.Pages {
		if key == "" {
			break
		}
		if mate, ok := find(other, key); ok {
			return mate
		}
		up, ok := find(from, key)
		if !ok {
			break
		}
		key = up.Parent
	}
	return Page{Key: "index"}
}

// checkUntranslated refuses a page marked untranslated anywhere but in the root
// language, and one that has a translation after all, since then the mark
// would hide the translation from search engines.
func checkUntranslated(langs []Language) error {
	all := everyPage(langs)
	languages := map[string]int{}
	for _, x := range all {
		languages[x.page.Key]++
	}
	for _, x := range all {
		switch {
		case !x.page.Untranslated:
		case x.lang.Dir != "":
			return fmt.Errorf("the %s page %q is marked untranslated, and only a page of the language served at the root can be", x.lang.Code, x.page.Key)
		case languages[x.page.Key] > 1:
			return fmt.Errorf("the page %q is marked untranslated and has a translation, which the mark would hide", x.page.Key)
		}
	}
	return nil
}

// placed is one page in one language.
type placed struct {
	lang Language
	page Page
}

// everyPage is every page of every language, as one list.
func everyPage(langs []Language) []placed {
	var out []placed
	for _, l := range langs {
		for _, p := range l.Pages {
			out = append(out, placed{lang: l, page: p})
		}
	}
	return out
}
