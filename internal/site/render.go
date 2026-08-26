// This file turns content and facts into the files that get published.
//
// Nothing here writes to disk. Render hands back every file keyed by its path
// under the published root, which is what lets one guard either compare the
// result with what is committed or write it out.

package site

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// pageURL is the path a page is served at, always with a trailing slash so a
// directory and its index are one address rather than two.
func pageURL(l Language, p Page) string {
	parts := []string{}
	if l.Dir != "" {
		parts = append(parts, l.Dir)
	}
	if p.Slug != "" {
		parts = append(parts, p.Slug)
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/") + "/"
}

// pageFile is where that page is written under the published root.
func pageFile(l Language, p Page) string {
	u := strings.Trim(pageURL(l, p), "/")
	if u == "" {
		return "index.html"
	}
	return path.Join(u, "index.html")
}

// Render produces every file of the published site, keyed by its path under
// the published root.
//
// Nothing is written to disk. The caller decides whether to compare or to
// write, which is what lets one guard do both.
func (s Site) Render() (map[string][]byte, error) {
	if len(s.Languages) == 0 {
		return nil, fmt.Errorf("a site with no languages has no pages to render")
	}
	out := map[string][]byte{}

	// Every language is filled in with the facts before anything is rendered,
	// so a title may state a number the same way a table does.
	filled, err := s.filledLanguages()
	if err != nil {
		return nil, err
	}
	s.Languages = filled

	shell, err := os.ReadFile(filepath.Join(s.TemplateDir, "layout.html"))
	if err != nil {
		return nil, fmt.Errorf("reading the page shell: %w", err)
	}
	partials, err := os.ReadFile(filepath.Join(s.TemplateDir, "partials.html"))
	if err != nil {
		return nil, fmt.Errorf("reading the shared pieces: %w", err)
	}

	if err := s.renderPages(out, string(partials), string(shell)); err != nil {
		return nil, err
	}

	notFound, err := s.notFound(string(partials), string(shell))
	if err != nil {
		return nil, err
	}
	out["404.html"] = notFound

	social, err := s.socialCard()
	if err != nil {
		return nil, err
	}
	out["social.html"] = social

	out["sitemap.xml"] = s.sitemap()
	out["robots.txt"] = s.robots()
	out["CNAME"] = []byte(s.Host() + "\n")

	if err := s.copyDir(out); err != nil {
		return nil, err
	}
	if err := s.copyExtras(out); err != nil {
		return nil, err
	}
	return out, nil
}

// filledLanguages runs every language through the facts.
//
// Its own function because Render was crowding the size ceiling and the guard
// that noticed asks for a split rather than a bigger number.
func (s Site) filledLanguages() ([]Language, error) {
	out := make([]Language, len(s.Languages))
	for i, lang := range s.Languages {
		done, err := lang.expand(s.Facts)
		if err != nil {
			return nil, err
		}
		out[i] = done
	}
	return out, nil
}

// copyExtras publishes files that live elsewhere in the repository.
//
// The window screenshot and the application icon come this way rather than
// being duplicated under web, so the picture on the site is the picture the
// project already keeps and cannot drift from it.
func (s Site) copyExtras(out map[string][]byte) error {
	for at, from := range s.Extra {
		b, err := os.ReadFile(from)
		if err != nil {
			return fmt.Errorf("reading %s to publish it at %s: %w", from, at, err)
		}
		out[at] = b
	}
	return nil
}

// viewFor assembles what one page is rendered against.
func (s Site) viewFor(lang Language, page Page) (view, error) {
	v := view{
		Facts:     s.Facts,
		Page:      page,
		Lang:      lang,
		W:         lang.Words,
		Path:      pageURL(lang, page),
		Canonical: s.Origin() + pageURL(lang, page),
		IsHome:    page.Slug == "",
	}
	for _, item := range lang.Pages {
		v.Nav = append(v.Nav, NavItem{
			Label:   item.Nav,
			URL:     pageURL(lang, item),
			Current: item.Key == page.Key,
		})
	}
	for _, other := range s.Languages {
		mate, ok := find(other, page.Key)
		if !ok {
			return view{}, fmt.Errorf("the %s page %q has no %s translation, so its language link would lead nowhere", lang.Code, page.Key, other.Code)
		}
		v.Alternates = append(v.Alternates, Alternate{Lang: other.Code, URL: s.Origin() + pageURL(other, mate)})
		if other.Code != lang.Code {
			v.Switches = append(v.Switches, Switch{Name: other.Name, URL: pageURL(other, mate), Code: other.Code})
		}
	}
	if root, ok := s.rootLanguage(); ok {
		if mate, found := find(root, page.Key); found {
			v.Alternates = append(v.Alternates, Alternate{Lang: "x-default", URL: s.Origin() + pageURL(root, mate)})
		}
	}
	return v, nil
}

// notFound renders the page a visitor gets for an address that is not there.
//
// It is in the language served at the root, because the address that missed
// carries no reliable clue about who asked.
func (s Site) notFound(partials, shell string) ([]byte, error) {
	root, ok := s.rootLanguage()
	if !ok {
		return nil, fmt.Errorf("no language is served at the root, so there is nothing to answer an unknown address in")
	}
	page := Page{
		Key:         "notfound",
		Slug:        "404",
		Title:       root.Words["notFoundTitle"],
		Description: root.Words["notFoundLead"],
		Nav:         root.Words["notFoundTitle"],
	}
	v := view{
		Facts:  s.Facts,
		Page:   page,
		Lang:   root,
		W:      root.Words,
		Path:   "/404.html",
		IsHome: false,
	}
	for _, item := range root.Pages {
		v.Nav = append(v.Nav, NavItem{Label: item.Nav, URL: pageURL(root, item)})
	}
	const body = `<h1>{{ .Word "notFoundTitle" }}</h1>` +
		`<p class="lead">{{ .Word "notFoundLead" }}</p>` +
		`<p><a class="button primary" href="/">{{ .Word "notFoundBack" }}</a></p>`
	rendered, err := run("body:404", partials+body, v)
	if err != nil {
		return nil, err
	}
	v.Body = template.HTML(rendered) //nolint:gosec // built from the word list above
	whole, err := run("page:404", partials+shell, v)
	if err != nil {
		return nil, err
	}
	return []byte(whole), nil
}

// renderPages writes every page of every language.
//
// Its own function because Render keeps drifting towards the size ceiling as
// the site grows, and the guard that notices asks for a split rather than a
// bigger number. The body of a page is rendered first and then poured into the
// shell, so a content file can call the same shared pieces the shell can.
func (s Site) renderPages(out map[string][]byte, partials, shell string) error {
	for _, lang := range s.Languages {
		for _, page := range lang.Pages {
			v, err := s.viewFor(lang, page)
			if err != nil {
				return err
			}
			fragment, err := os.ReadFile(filepath.Join(s.ContentDir, lang.Code, page.Key+".html"))
			if err != nil {
				return fmt.Errorf("reading the %s text of the %s page: %w", lang.Code, page.Key, err)
			}
			body, err := run("body:"+lang.Code+":"+page.Key, partials+string(fragment), v)
			if err != nil {
				return err
			}
			v.Body = template.HTML(body) //nolint:gosec // our own content files, rendered by us

			whole, err := run("page:"+lang.Code+":"+page.Key, partials+shell, v)
			if err != nil {
				return err
			}
			out[pageFile(lang, page)] = []byte(whole)
		}
	}
	return nil
}

// socialCard renders the picture other sites show when this project is shared.
//
// It is a page rather than a drawing on purpose. The number of formats on the
// card comes from the registry the same way the tables do, so a card that
// disagrees with the tool is a red guard rather than an image nobody thought
// to redo. What it is not is part of the site: it carries noindex, it is in no
// sitemap and nothing links to it. It exists to be photographed at 1280 by
// 640, which is what GitHub asks for.
//
// English only, and that is a limitation rather than a decision waiting to be
// made: one repository has one social card, and the audience of a shared link
// is whoever the sharer is talking to.
func (s Site) socialCard() ([]byte, error) {
	root, ok := s.rootLanguage()
	if !ok {
		return nil, fmt.Errorf("no language is served at the root, so the social card has no words")
	}
	body, err := os.ReadFile(filepath.Join(s.TemplateDir, "social.html"))
	if err != nil {
		return nil, fmt.Errorf("reading the social card: %w", err)
	}
	whole, err := run("page:social", string(body), view{
		Facts: s.Facts,
		Lang:  root,
		W:     root.Words,
		Path:  "/social.html",
	})
	if err != nil {
		return nil, err
	}
	return []byte(whole), nil
}

// sitemap lists every page, each carrying the addresses of its translations.
//
// The alternates are inside the sitemap as well as in the pages because a
// crawler that reaches the sitemap first should not have to fetch a page to
// learn that another language exists.
func (s Site) sitemap() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, lang := range s.Languages {
		for _, page := range lang.Pages {
			b.WriteString("  <url>\n")
			fmt.Fprintf(&b, "    <loc>%s%s</loc>\n", s.Origin(), pageURL(lang, page))
			for _, other := range s.Languages {
				if mate, ok := find(other, page.Key); ok {
					fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=%q href=\"%s%s\"/>\n", other.Code, s.Origin(), pageURL(other, mate))
				}
			}
			if root, ok := s.rootLanguage(); ok {
				if mate, found := find(root, page.Key); found {
					fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s%s\"/>\n", s.Origin(), pageURL(root, mate))
				}
			}
			b.WriteString("  </url>\n")
		}
	}
	b.WriteString("</urlset>\n")
	return b.Bytes()
}

// robots lets everything in and says where the sitemap is.
//
// Nothing here is hidden - the whole point of the site is to be found - so the
// file exists to carry the sitemap line rather than to keep anybody out.
func (s Site) robots() []byte {
	return []byte("User-agent: *\nAllow: /\n\nSitemap: " + s.Origin() + "/sitemap.xml\n")
}

// copyDir takes the static assets in as they are.
func (s Site) copyDir(out map[string][]byte) error {
	if s.AssetDir == "" {
		return nil
	}
	return filepath.WalkDir(s.AssetDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(s.AssetDir, p)
		if relErr != nil {
			return relErr
		}
		out[path.Join("assets", filepath.ToSlash(rel))] = b
		return nil
	})
}

// run executes one template and turns a failure into a sentence naming the
// page it came from.
func run(name, text string, v view) (string, error) {
	t, err := template.New(name).Funcs(funcs()).Parse(text)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", name, err)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, v); err != nil {
		return "", fmt.Errorf("rendering %s: %w", name, err)
	}
	return b.String(), nil
}

// funcs are the few helpers a content file may call.
//
// Deliberately small. Anything cleverer belongs in the data the caller
// assembles, where it can be checked, rather than in a template where it
// cannot.
func funcs() template.FuncMap {
	return template.FuncMap{
		"join":   strings.Join,
		"sorted": sortedStrings,
	}
}

// sortedStrings returns a sorted copy, so a template never sorts in place.
func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
