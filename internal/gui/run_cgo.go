//go:build cgo

package gui

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/catalogue"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/icon"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// appID names this application to the desktop it runs on.
//
// Reverse domain form because that is what every desktop expects. Without an
// id the toolkit prints a complaint on every start and its preferences API
// refuses to work, which since 2026-08-25 is the API the window size and the
// output directory come back from - and even before that, a warning nobody can
// act on is noise that teaches people to ignore the log.
//
// What it costs, measured on 2026-08-05: starting the application creates a
// directory under the user's application data, and it does so with or without
// an id - app.New() makes one too. An id only decides whether that directory
// has our name on it or a derived one.
//
// The sentence that stood here until 2026-08-25 said no file was ever
// written into that directory, because nothing stored a preference. It had been
// untrue since 2026-08-05, the same day it was written. Measured rather than
// re-read: the directory holds preferences.json, 73 bytes, and in it
// fyne:fileDialogLastFolder and fyne:fileDialogViewLayout. The toolkit's folder
// picker writes them, so the Choose button put a state file on somebody's disk
// the day it arrived, and the note beside it said the opposite for twenty days.
// Left here rather than quietly corrected, because the interesting part is that
// a dependency can cross a line this project guards while a comment says it has
// not.
//
// So the decision docs/GUI.md section 4.1 was waiting for was in one sense
// already taken by something else. It has been taken properly since 2026-08-25:
// the owner asked for the output directory and the window size to be kept, and
// they are kept in that same file rather than in one of ours. See
// window.Remembered for what that is and is not, and for why nothing here ever
// deletes it - untouchable rule 7.
const appID = "dev.donislaw.tfg"

// desktop is a real window, told how to answer the one thing the screens
// cannot do for themselves.
//
// The toolkit's folder picker lives here rather than beside the screens for the
// same reason its app package does: it is the part that needs a real window,
// and keeping it out of internal/gui/window is what lets the whole screen tree
// build and render with no C compiler.
//
// It is the only place this build touches fyne.io/fyne/v2/dialog, and that
// import has a cost worth recording. The dialog package pulls in
// github.com/FyshOS/fancyfs, which decorates folder icons and is reached from
// exactly one line of it. Checked on 2026-08-05 before it was accepted: BSD-3,
// one way compatible with GPL-3.0 like the eleven other BSD-3 modules already
// here, 129 lines of code, written by the author of the toolkit itself. It was
// already named in our module graph because the toolkit requires it - what
// changed is that it is now downloaded, checksummed and compiled in.
type desktop struct {
	fyne.Window
	// software says the window is drawn by the renderer shipped beside it
	// rather than by the graphics driver - the fact the About screen states,
	// because a window drawn in software is slower and the person should
	// be able to read why.
	software bool
}

// SoftwareRendering reports whether this window draws through the software
// renderer shipped beside it. True in the second process after the driver
// refused, and in any process asked for it by the flag - and only when the
// renderer actually loaded, because a window that says it draws in software
// while the driver draws it would be saying something untrue.
func (d desktop) SoftwareRendering() bool { return d.software }

// Later fires on a clock and lands on the interface thread through the
// toolkit's queue, which is what fyne.Do is for. Calling it off stops the
// clock, and a function already on its way finds the state it checks
// before it acts - see busy.set.
func (d desktop) Later(after time.Duration, then func()) func() {
	timer := time.AfterFunc(after, func() { fyne.Do(then) })
	return func() { timer.Stop() }
}

// Remembered is the window size and output directory, kept by the toolkit in
// the file its folder picker already writes.
//
// A store of ours was the alternative and it was turned down for one measured
// reason: preferences.json exists on this machine and has since 2026-08-05 -
// see the note beside appID - so writing our own file would put a SECOND state
// file on somebody's disk to hold two values. Fyne writes it with os.Create,
// which truncates in place rather than replacing atomically, so a machine that
// dies mid write leaves a short file. That is survivable here and the toolkit
// already decides it: a file that will not parse is logged and the defaults are
// used, which for these two values means the window opens where it always did.
func (d desktop) Remembered() window.Remembered { return stored{fyne.CurrentApp().Preferences()} }

// stored puts names on the two things kept, so that no screen and no guard ever
// handles a preference key. The keys are here and nowhere else.
type stored struct{ prefs fyne.Preferences }

const (
	keyDirectory = "outputDirectory"
	keyWidth     = "windowWidth"
	keyHeight    = "windowHeight"
)

func (s stored) Directory() string          { return s.prefs.String(keyDirectory) }
func (s stored) RememberDirectory(d string) { s.prefs.SetString(keyDirectory, d) }

// Size is two numbers rather than one, because the toolkit's store holds
// scalars. Read back as a size so that everything above this line handles a
// size and not a pair.
func (s stored) Size() fyne.Size {
	return fyne.NewSize(float32(s.prefs.Float(keyWidth)), float32(s.prefs.Float(keyHeight)))
}

func (s stored) RememberSize(size fyne.Size) {
	s.prefs.SetFloat(keyWidth, float64(size.Width))
	s.prefs.SetFloat(keyHeight, float64(size.Height))
}

// rememberThisSize writes down how big the window is now.
//
// It reads the CANVAS, which is the content area, and that is the same thing
// Resize is given - so what is written down and what is asked for next time are
// the same measurement. Measured on 2026-08-25 and worth knowing: the canvas
// reports the size that was ASKED for, which is not always the size the system
// gave. That only pulls apart when the program asks for something impossible,
// and a window a person dragged to a size is a window the system agreed to.
//
// A size with a nought in it is not written, so a window closed while minimised
// does not come back as nothing - see window.HowToOpen, which refuses the same
// shape on the way back in.
func (d desktop) rememberThisSize() {
	size := d.Canvas().Size()
	if !window.WorthRemembering(size) {
		return
	}
	d.Remembered().RememberSize(size)
}

func (d desktop) ChooseDirectory(chosen func(string)) {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		// Nothing chosen and nothing to say. Cancelling a picker is an
		// ordinary answer rather than a failure, and an error here is the
		// toolkit failing to read a directory - which the field the person
		// can still type into makes survivable.
		if err != nil || dir == nil {
			chosen("")
			return
		}
		chosen(dir.Path())
	}, d.Window)
}

// OpenLink hands an address to the desktop's own browser.
//
// The program makes no request. It parses the address and passes it to the
// toolkit, which passes it to the system - so nothing here fetches anything and
// nothing is sent. That is the line untouchable rule 8 draws, and the reason the
// Donate button is not a hole in it.
//
// net/url is a parser and not a network package, which the guard over these
// files agrees with: it refuses net, net/http, crypto/tls and their kin, and
// this is none of them.
//
// A refusal to open is swallowed on purpose. There is nothing useful to say to
// somebody whose desktop has no browser registered, no screen to say it on that
// would not be a modal about a button they pressed by curiosity, and no harm
// done - the address is in the About screen for anybody who wants to type it.
//
// That last clause was FALSE until 2026-09-07 and is the reason this paragraph
// is worth reading twice. text.SupportURL had exactly one use in the whole
// tree, as this button's destination, so it was on no screen at all and a
// person whose desktop did nothing had nowhere to go. The decision to stay
// quiet was sound and rested on a fallback that did not exist. The About screen
// carries the address now, in its Support section, which is what makes the
// sentence above true rather than merely reasonable.
func (d desktop) OpenLink(address string) {
	parsed, err := url.Parse(address)
	if err != nil {
		return
	}
	_ = fyne.CurrentApp().OpenURL(parsed)
}

// OpenFolder asks the desktop to show a directory.
//
// The address is BUILT as a URL rather than glued together, and that is a
// measurement rather than caution. tools/probes/fileuri on 2026-08-25: the
// toolkit's own storage.NewFileURI puts "file://" in front of a slashed path
// and escapes nothing, so "C:\a#b&c" comes out as file://C:/a#b&c - where
// everything after the hash is a URL fragment and the directory the person
// asked for is not the one that opens. A space is left raw too. Setting Path on
// a url.URL and letting String do the escaping is the difference between
// %23 and a silently truncated name.
//
// Absolute first, because a relative path has no meaning to another process and
// the box on the screen is allowed to hold one.
//
// A refusal is swallowed for the same reason OpenLink swallows one: there is
// nothing useful to say to somebody whose desktop has no file manager, and the
// path is on the screen for anybody who wants to copy it.
func (d desktop) OpenFolder(path string) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return
	}
	address := &url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	_ = fyne.CurrentApp().OpenURL(address)
}

// run opens a real window. The only file in this tree that reaches the app
// package, and therefore the only one that needs a C compiler.
func run(launch Launch, errOut io.Writer) int {
	// The software renderer, when asked for, before anything below asks the
	// driver for anything: the toolkit's first request for OpenGL by name has
	// to find it already mapped. A renderer that does not load is said out
	// loud and the window goes on to the toolkit - which on the machine this
	// exists for will refuse, and the refusal then says the renderer was
	// tried and why it could not be. On a machine with a driver it opens as
	// usual, drawn by the driver, and says so by not claiming otherwise.
	var loaded error
	software := false
	if launch.SoftwareGL {
		dir, err := executableDir()
		if err == nil {
			err = LoadSoftwareRenderer(dir)
		}
		loaded = err
		software = err == nil
		fmt.Fprintln(errOut, LoadingSentence(loaded))
	}

	// Said out loud rather than left to be inferred: everything that touches a
	// widget from the worker goes through fyne.Do, and a static guard checks
	// it, but the toolkit had no way to know that. Without this it printed
	// three lines at every start telling the person running it that this
	// application has not been migrated - a warning about our code, on their
	// terminal, that was not true.
	//
	// Set here rather than passed as a build tag, because a tag has to be
	// remembered at every build and this cannot be forgotten.
	app.SetMetadata(fyne.AppMetadata{
		ID:         appID,
		Name:       "Testing Files Generator",
		Version:    version.Version,
		Migrations: map[string]bool{"fyneDo": true},
	})

	// Asked before the window exists, because it is a setting for the whole
	// process rather than for one window, and Windows reads it when it builds
	// the menus. See darkmenus_windows.go for what it is and what it costs.
	PreferDarkMenus()

	// The catalogue, before the first word is asked for. A failure here is not
	// a reason to refuse to start: every message states its English on the spot
	// and answers with it when no catalogue is loaded, so the window opens in
	// English rather than not at all. It is said out loud rather than swallowed,
	// because a language silently not arriving is the shape of defect somebody
	// reports as "it ignores my system settings" a year later.
	if err := text.LoadBuiltIn(); err != nil {
		fmt.Fprintln(errOut, text.CatalogueNotLoaded(err))
	}

	// Where the toolkit says what went wrong when it cannot open its window.
	// It writes through the standard logger, whose stream a binary built for
	// the windows subsystem does not have, so the stream is kept as it was
	// and a copy is held for the sentence at the end of this function. The
	// copy comes first: a writer that stops at the first failure would stop
	// at a standard error that is not there. Our own code never logs, so on a
	// machine where the window opens the copy stays empty (O218).
	var said bytes.Buffer
	log.SetOutput(io.MultiWriter(&said, log.Writer()))

	a := app.NewWithID(appID)
	// The picture the desktop shows for this program, in the taskbar, in the
	// switcher and on the window itself - the toolkit says an application icon
	// is also the default icon for every window it opens, so this one line
	// covers all of them. Without it the toolkit falls back to its own logo,
	// which is how somebody ends up looking for our window under a name that
	// is not ours.
	a.SetIcon(fyne.NewStaticResource("chickpea.png", icon.PNG))
	// The palette of docs/UX.md sections 8.2 and 8.3, computed before the first
	// widget and describing nothing until it was installed here - O70. It
	// answers dark whatever the desktop is set to, by the owner's decision.
	a.Settings().SetTheme(parts.Theme())
	w := a.NewWindow(text.WindowTitle(version.Version))
	host := desktop{Window: w, software: software}
	// The window coming to the front is not the keyboard arriving, and only
	// the window can tell the two apart - see WindowReturning for the
	// measurement. The foreground hook runs just before the driver's call, so
	// this is where the control is told what is coming. The registration is
	// behind cgo like the remembering above, so a guard reads this line out
	// of the source and calls what it registers with a canvas of its own.
	a.Lifecycle().SetOnEnteredForeground(func() { WindowReturning(w.Canvas()) })
	// What a first start opens at, if nothing is remembered: nothing for the
	// catalogue, which opens at the ceiling, and what the screens want for
	// the ordinary window.
	var wanted fyne.Size
	if launch.Catalogue {
		// The hidden screen of GUI rule 4: every part in every state, for
		// whoever builds the window. No host, because nothing on it runs or
		// chooses a directory - and nothing about it is remembered either,
		// which is why the close callback below is registered for the
		// ordinary window alone. It opens at the remembered size all the same,
		// because that is the size this person's screen has room for.
		w.SetContent(catalogue.Screen())
	} else {
		wanted = window.Open(host)
	}

	// The size it was closed at, and whether to put it in the middle. The two
	// answers come together because they are one decision - a window bigger than
	// the screen it comes back on has its title bar off the top when it is
	// centred, and cannot then be moved or resized at all. Measured, both ways,
	// in window.HowToOpen.
	size, centre := window.HowToOpen(host.Remembered().Size(), wanted)
	w.Resize(size)
	if centre {
		w.CenterOnScreen()
	}
	// Written down as the window goes rather than as it is resized. SetOnClosed
	// runs after the close intercept, which is where the directory is kept, so
	// the two land together whichever way the window was shut.
	//
	// The ordinary window only. The catalogue is a developer's screen, 8900 px
	// of controls somebody drags tall to read, and until 2026-09-16 closing it
	// wrote that size down as the size the ordinary window opens at next time.
	// An outside review of the pull request named it. No guard reaches this
	// line - it is behind cgo, like the rest of the remembering - so it is
	// checked the way the rest was, by a run of the binary.
	if !launch.Catalogue {
		w.SetOnClosed(func() { host.rememberThisSize() })
	}
	// Shown, then run - and when the toolkit gave the window no window,
	// tried again in a second process with the software renderer, and only
	// then refused out loud. Not ShowAndRun, which is the same two calls
	// without the question between them, and which on a machine without
	// OpenGL runs a loop nothing will ever end (O218). The refusal goes to
	// standard error for whatever started this, and to a system dialog for
	// the person, because the window that would have carried it is what
	// failed - and it says what became of the renderer, when there is
	// something to say.
	return OpenOrRefuse(w, a.Run, SecondAttemptFor(launch, errOut).Try, func(why error) {
		sentence := text.WindowRefused(CauseFrom(said.String()))
		fmt.Fprintln(errOut, sentence)
		body := sentence
		if renderer := RendererSentence(why, loaded); renderer != "" {
			fmt.Fprintln(errOut, renderer)
			body += "\n\n" + renderer
		}
		sayInADialog(text.WindowRefusedTitle(), body)
	})
}
