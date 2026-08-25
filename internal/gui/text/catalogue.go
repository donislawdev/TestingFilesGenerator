package text

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"

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
