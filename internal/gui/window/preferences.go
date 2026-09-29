package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Preferences is the screen of how the window speaks and what it keeps between
// runs - docs/USTAWIENIA-2026-09-29.md, the owner's decisions of that day.
//
// Nothing on it changes the files the window makes. A setting that did would
// be an input the recipe does not carry, and the command the window offers to
// show would then make different files from the ones on screen - so anything
// that shapes a file belongs in the form, not here.
//
// The language takes effect from the next start rather than at once. The
// words of every screen are asked for once, when the screens are built, and
// the screens are built once so that a run in flight and everything typed
// survive moving between tabs - so a language changed in place would need
// every screen rebuilt around its state. Restart now is the honest version of
// that: it closes the window the way a person closing it does, and opens a new
// one.
type Preferences struct {
	host Host
	// leave closes the window the way the close button does - runs stopped,
	// the folder written down - which Window.Close alone does not do.
	leave func()
	// busy says whether files are being made on any screen.
	busy func() bool

	pick    *parts.Chooser
	choices languageChoices
	// state is the line under the button: what a press would do, or why it
	// cannot be pressed now.
	state   *parts.QuietText
	restart *parts.Button

	forget    *parts.Button
	forgotten *parts.QuietText

	object fyne.CanvasObject
}

// NewPreferences builds the screen.
func NewPreferences(h Host, leave func(), busy func() bool) *Preferences {
	p := &Preferences{host: h, leave: leave, busy: busy}
	p.choices = choicesFor(text.Languages(), h.SystemLanguage())

	p.pick = parts.NewChooser(p.choices.options, nil)
	p.show(p.choices.optionFor(h.Remembered().Language()))
	p.restart = parts.NewButton(parts.Primary, text.ButtonRestart(), p.restartNow)
	p.state = parts.NewQuietText("")
	p.state.Wrapping = fyne.TextWrapWord

	p.forget = parts.NewButton(parts.Secondary, text.ButtonForget(), p.forgetNow)
	p.forgotten = parts.NewQuietText(text.PreferencesForgetWhat())
	p.forgotten.Wrapping = fyne.TextWrapWord
	p.refresh()

	fields := parts.NewFields()
	language := parts.Section(text.SectionLanguage(),
		parts.Grid(fields.Named(text.FieldLanguage(), parts.NoDetail, parts.Menu(p.pick))),
		parts.Prose(text.PreferencesLanguageScope()),
		container.NewHBox(p.restart),
		p.state,
	)
	remembered := parts.Section(text.SectionRemembered(),
		parts.Prose(text.PreferencesKept()),
		parts.Prose(text.PreferencesKeptIn()),
		parts.Prose(h.SettingsFolder()),
		// A row of ours, GapButtons apart - a bare box put the toolkit's 4 px
		// between the two and they read as one control - held to the left
		// edge the way the About screen holds its button.
		container.NewHBox(parts.ButtonRow(
			parts.NewButton(parts.Secondary, text.ButtonOpenFolder(), func() { h.OpenFolder(h.SettingsFolder()) }),
			p.forget)),
		p.forgotten,
	)
	page := parts.Screen(parts.Titled(text.TabPreferences(), text.SubtitlePreferences()), language, remembered)
	p.object = container.NewVScroll(page)
	return p
}

// Object is the screen, for the tab that holds it.
func (p *Preferences) Object() fyne.CanvasObject { return p.object }

// BusyChanged is told when files start or stop being made on any screen.
func (p *Preferences) BusyChanged() { p.refresh() }

// show puts an entry in the list without saving it. Opening the screen must
// not write anything, least of all when the saved language is one this build
// does not carry: the list then shows the system, and saving that would erase
// the person's choice without a press - untouchable rule 7 - and the sentence
// saying it was missing with it.
func (p *Preferences) show(option string) {
	p.pick.OnChanged = nil
	p.pick.SetSelected(option)
	p.pick.OnChanged = p.chose
}

// chose saves what was picked, at once: there is no Save button, because a
// choice that has to be confirmed twice is a choice that gets lost when the
// window is closed in between.
func (p *Preferences) chose(option string) {
	p.host.Remembered().RememberLanguage(p.choices.tagOf[option])
	p.refresh()
}

// restartNow closes this window the way a person does and asks for a new one
// once it has gone - see Host.RestartWhenClosed. Asked again at the press,
// because a run may have started between the last refresh and this.
func (p *Preferences) restartNow() {
	if p.busy() || !p.pending() {
		p.refresh()
		return
	}
	p.host.RestartWhenClosed()
	p.leave()
}

// forgetNow removes what the window keeps and says it did. The list goes back
// to the system's language with it, because that is what the window will speak
// next time now that nothing is chosen.
func (p *Preferences) forgetNow() {
	p.host.Remembered().Forget()
	p.show(p.choices.optionFor(""))
	p.forgotten.SetText(text.PreferencesForgotten())
	p.forget.Disable()
	p.refresh()
}

// pending says whether a restart would change the language on screen.
func (p *Preferences) pending() bool {
	return p.choices.speaks(p.host.Remembered().Language()) != text.Speaking()
}

// refresh sets the button and the line under it from the state as it is now.
func (p *Preferences) refresh() {
	saved := p.host.Remembered().Language()
	p.state.SetText(restartSentence(p.pending(), p.busy(), p.choices.missing(saved)))
	if p.pending() && !p.busy() {
		p.restart.Enable()
	} else {
		p.restart.Disable()
	}
}

// restartSentence is the line under Restart now. Never empty, so the screen
// does not move when it changes - GUI rule 3.
func restartSentence(pending, busy bool, missing string) string {
	switch {
	case pending && busy:
		return text.PreferencesRestartBusy()
	case pending:
		return text.PreferencesRestartClears()
	case missing != "":
		return text.PreferencesMissing(missing)
	default:
		return text.PreferencesNextStart()
	}
}

// languageChoices is the list of languages as the screen shows it, and what
// each entry of it is. Built from the one list of carried languages, so the
// words on screen and the tags saved cannot come apart - GUI rule 8.
type languageChoices struct {
	carried []text.Language
	system  string
	// options are the entries in order: the system first, then every
	// carried language by its own name.
	options []string
	// tagOf is what each entry saves: empty for the system.
	tagOf map[string]string
}

func choicesFor(carried []text.Language, system string) languageChoices {
	c := languageChoices{carried: carried, system: system, tagOf: map[string]string{}}
	first := text.ChoiceSameAsSystem(nameOf(carried, text.Resolve("", system, carried).Tag))
	c.options = append(c.options, first)
	c.tagOf[first] = ""
	for _, l := range carried {
		c.options = append(c.options, l.Name)
		c.tagOf[l.Name] = l.Tag
	}
	return c
}

// optionFor is the entry a saved value stands for. A value this build does
// not carry is shown as the system, because that is what the window speaks
// for it - and the line under the button says what was missing.
func (c languageChoices) optionFor(saved string) string {
	if saved == "" || c.missing(saved) != "" {
		return c.options[0]
	}
	return nameOf(c.carried, c.speaks(saved))
}

// speaks is the language the window opens in for a saved value.
func (c languageChoices) speaks(saved string) string {
	return text.Resolve(saved, c.system, c.carried).Tag
}

// missing is a saved value this build does not carry, or empty.
func (c languageChoices) missing(saved string) string {
	return text.Resolve(saved, c.system, c.carried).Missing
}

// nameOf is what a carried language calls itself.
func nameOf(carried []text.Language, tag string) string {
	for _, l := range carried {
		if l.Tag == tag {
			return l.Name
		}
	}
	return tag
}
