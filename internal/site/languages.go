// This file holds what a language has to look like before anything is
// rendered from it.
//
// A language file is written by hand, and the values in it end up in places
// nobody reads twice: the lang of a page, the hreflang of its alternates, the
// og:locale of a shared link, the address of every page and the line of the
// sitemap. A typo in any of them does not break a page. It makes a search
// engine quietly ignore one - an hreflang it cannot parse is dropped, and so
// is the page it was meant to connect. So the values are checked here, once,
// and the render stops and names the language instead.

package site

import (
	"fmt"
	"regexp"
)

var (
	// languageTag is a BCP 47 tag of the shapes this site uses: a language, an
	// optional script and an optional region, as in de, zh-Hans and pt-BR.
	languageTag = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z][a-z]{3})?(-[A-Z]{2})?$`)

	// openGraphLocale is language and region joined by an underscore, which is
	// the only form the Open Graph protocol defines.
	openGraphLocale = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}$`)
)

// checkLanguages refuses a set of languages that would put a wrong or a
// repeated value on a page.
//
// It asks about the languages as a set because most of what can go wrong is a
// repetition: two languages under one prefix write their pages over each other,
// and two with one tag give a page two alternates that disagree.
func checkLanguages(langs []Language) error {
	roots := 0
	claims := claimed{}
	for _, l := range langs {
		if err := checkLanguage(l); err != nil {
			return err
		}
		if l.Dir == "" {
			roots++
		}
		if err := claims.add(l); err != nil {
			return err
		}
	}
	if roots != 1 {
		return fmt.Errorf("%d languages are served at the root and exactly one has to be, because x-default points at it", roots)
	}
	return nil
}

// claimed remembers which language holds each value that has to be unique.
type claimed map[string]string

// add records the values of one language, or says which earlier language
// already holds one of them.
func (c claimed) add(l Language) error {
	// In a fixed order, so the same mistake always reads the same.
	for _, v := range [][2]string{{"code", l.Code}, {"locale", l.Locale}, {"directory", l.Dir}} {
		if v[0] == "directory" && v[1] == "" {
			continue
		}
		key := v[0] + " " + v[1]
		if other, taken := c[key]; taken {
			return fmt.Errorf("the %s %q is used by both %s and %s", v[0], v[1], other, l.Code)
		}
		c[key] = l.Code
	}
	return nil
}

// checkLanguage asks the questions that need one language only.
func checkLanguage(l Language) error {
	if !languageTag.MatchString(l.Code) {
		return fmt.Errorf("the language %q has a code that is not a BCP 47 tag such as de, zh-Hans or pt-BR", l.Code)
	}
	if !openGraphLocale.MatchString(l.Locale) {
		return fmt.Errorf("the %s language has the locale %q, and Open Graph wants language and region with an underscore, as in de_DE", l.Code, l.Locale)
	}
	if l.Name == "" {
		return fmt.Errorf("the %s language has no name, so the link that leads to it would be blank", l.Code)
	}
	if l.Dir != "" && !addressable.MatchString(l.Dir) {
		return fmt.Errorf("the %s language is served under %q, which cannot be part of an address - lower case letters, digits and single dashes can", l.Code, l.Dir)
	}
	seen := map[string]string{}
	for _, p := range l.Pages {
		if p.Slug != "" && !addressable.MatchString(p.Slug) {
			return fmt.Errorf("the %s page %q has the slug %q, which cannot be part of an address - lower case letters, digits and single dashes can", l.Code, p.Key, p.Slug)
		}
		if other, taken := seen[p.Slug]; taken {
			return fmt.Errorf("the %s pages %q and %q share the address %q, so one would be written over the other", l.Code, other, p.Key, p.Slug)
		}
		seen[p.Slug] = p.Key
	}
	return nil
}
