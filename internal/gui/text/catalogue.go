package text

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	// Window labels and tooltips, which are never HTML. The rule is about
	// rendering user content into a page, and this package renders neither.
	// nosemgrep: go.lang.security.audit.xss.import-text-template.import-text-template
	"text/template"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// The catalogue this build carries, as files rather than as code.
//
// Every entry in this package states its English on the spot, beside the reason
// it is worded that way, and that stays the one place English is written. The
// catalogue is where the OTHER languages live, and en.json is that same English
// written out again by tools/gen-locale.py so a translator has a complete file
// to copy rather than a Go package to read. A guard holds the two together, so
// the copy cannot drift into a second opinion.
//
// Compiled in rather than read from beside the binary. This tool ships as one
// file somebody downloads, it does no input or output it was not asked for, and
// a missing language file is a failure mode worth not having.
//
//go:embed locale
var builtIn embed.FS

// The library rather than the toolkit's wrapper around the same library, and
// the reason is measured rather than architectural.
//
// fyne.io/fyne/v2/lang offers exactly this and is already linked into the window
// binary, so it was the obvious choice and was built first. Then it was run on
// this machine, which is set to pl-PL: the toolkit ships its own Polish, so
// Polish is a language it KNOWS, and every message of ours missing from it was
// written to the log as "Translation failure". Measured on 2026-08-25 -
// thirty three lines from building ONE screen, and English on the screen the
// whole time. A fallback that is loud is not a fallback, and this project has
// already written down what a warning nobody can act on does: it teaches people
// to ignore the log.
//
// go-i18n answers a missing message with the default it was handed and says
// nothing, which is the behaviour a fallback has to have. It is not a new
// module either - the toolkit requires it, so it was already in the graph and
// already in the window binary. This build pins the version the toolkit
// resolved rather than the newest, because moving it would move a dependency of
// the toolkit and that is a question about byte stability rather than about
// translations. See docs/STACK.md.
var localiser *i18n.Localizer

// sayf is say for a sentence that has values in it.
//
// Named fields rather than the printf verbs the rest of this tool writes, and
// the reason is rule 6 rather than taste. A catalogue is DATA: it arrives as a
// file, nothing compiles it, and no test of ours stands between it and the
// window. A translator who writes %s where the code hands a number puts
// "%!d(string=...)" on somebody's screen at run time and nothing says a word. A
// named field cannot be the wrong type, and one that is missing renders as
// "<no value>" - wrong, but visible.
//
// Decided with the owner on 2026-08-26. The printf spelling was the cheaper of
// the two and was turned down: the generator already reads it, the call sites
// stay one line, and the failure it allows is silent and lands on a user.
//
// The fallback renders the English with the same template engine rather than
// returning it raw, because a sentence with {{.Directory}} still in it is not a
// fallback, it is a different defect.
func sayf(id, english string, data map[string]any) string {
	if localiser == nil {
		return fill(english, data)
	}
	out, err := localiser.Localize(&i18n.LocalizeConfig{
		MessageID:      id,
		DefaultMessage: &i18n.Message{ID: id, Other: english},
		TemplateData:   data,
	})
	if err != nil {
		return fill(english, data)
	}
	return out
}

// sayN is sayf where a count chooses the form of the sentence.
//
// English has two forms and Polish has three, and the library knows which rule
// belongs to which language - so this hands over both English forms and the
// number, and a translator's file may carry as many forms as their language
// needs. Writing "1 file" and "%d files" as two flat entries would have put a
// thing into the catalogue that is broken for Polish before anybody starts,
// with no way to fix it from a translation file.
//
// The count is always available to the sentence as Count, because a plural form
// that cannot say the number is not much use.
func sayN(id, one, other string, count int, data map[string]any) string {
	values := map[string]any{"Count": count}
	for k, v := range data {
		values[k] = v
	}
	if localiser == nil {
		if count == 1 {
			return fill(one, values)
		}
		return fill(other, values)
	}
	out, err := localiser.Localize(&i18n.LocalizeConfig{
		MessageID:      id,
		DefaultMessage: &i18n.Message{ID: id, One: one, Other: other},
		PluralCount:    count,
		TemplateData:   values,
	})
	if err != nil {
		if count == 1 {
			return fill(one, values)
		}
		return fill(other, values)
	}
	return out
}

// fill puts the values into a sentence without the catalogue.
//
// Both failures return the layout untouched rather than an empty string: a
// sentence with its placeholders showing is readable and says something is
// wrong, and nothing at all says the window is broken.
func fill(layout string, data map[string]any) string {
	t, err := template.New("").Parse(layout)
	if err != nil {
		return layout
	}
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		return layout
	}
	return out.String()
}

// say is what every entry in this package asks, and what makes the English
// beside it a default rather than the answer.
//
// The English is handed over on every call rather than kept in a file of its
// own, so there is one place a sentence is written and no way for a catalogue
// to disagree with the code about what it says in English.
func say(id, english string) string {
	if localiser == nil {
		return english
	}
	out, err := localiser.Localize(&i18n.LocalizeConfig{
		MessageID:      id,
		DefaultMessage: &i18n.Message{ID: id, Other: english},
	})
	if err != nil {
		return english
	}
	return out
}

// Load reads a catalogue and chooses the language to answer in.
//
// The filesystem is a parameter rather than the built in one, so a guard can
// put a language in and watch the window take it. A seam nothing has ever
// pushed a second language through is a seam that compiles.
//
// Languages are tried in the order given and English is always last, so a
// catalogue that carries half a language leaves English sentences rather than
// empty ones.
func Load(fsys fs.FS, dir string, prefer ...string) error {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		if _, err := bundle.LoadMessageFileFS(fsys, path.Join(dir, entry.Name())); err != nil {
			return err
		}
	}

	localiser = i18n.NewLocalizer(bundle, append(prefer, "en")...)
	return nil
}

// LoadBuiltIn is Load over the catalogue compiled into this program.
//
// It is called once, where the window is built. Changing language after that
// needs the screens built again - they are built once on purpose, so that a run
// in flight and everything typed into a form survive moving between tabs, and
// docs/GUI.md section 6 names that as its own piece of work rather than as
// something a text package can do.
//
// No language is preferred yet. Every message answers in English, which is the
// only language this build carries, and asking the machine which language it
// wants is the next step rather than this one - a preference nothing can honour
// is a setting that does nothing.
func LoadBuiltIn() error {
	return Load(builtIn, "locale")
}
