package window

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What a finished run leaves behind, and the way to each of it.
//
// Its own type since 2026-09-23, when the manifest got a button beside the
// folder's and the runner went one method and one field past its ceilings.
// Those ceilings are ratchets, so the answer is to move state out and never
// to raise the number - and the seam was already there to be found: two
// buttons, two paths, one rule about when each is offered, and nothing in any
// of it about running anything.
//
// Both buttons are built hidden and shown when there is something behind
// them. A button leading to a file or a folder that need not exist yet is a
// button that does nothing, and this window has spent months getting rid of
// those. Both are taken away the moment the next run starts, so neither ever
// points at the results of the run before this one - which is worse than not
// being there, because somebody would open it, see the old files and believe
// them.
type offers struct {
	folderBtn   *parts.Button
	manifestBtn *parts.Button

	// openFolder and openFile are how this asks the desktop, held as
	// functions rather than reaching for the host: the runner is shared by
	// three screens and none of them owns the window. See Host.OpenFile for
	// why showing a directory and opening a file are two calls.
	openFolder func(string)
	openFile   func(string)

	// wroteInto and wroteManifest are where the run that just finished put
	// its files and its record. Kept rather than read back off the form,
	// because the boxes on the screen can be edited afterwards and a button
	// has to lead where the run ACTUALLY went.
	wroteInto     string
	wroteManifest string

	// relay redraws the bar when a button appears or goes, because the
	// toolkit does not lay the row out again by itself when a child of it is
	// shown or hidden. See busy.relay.
	relay func()
}

// newOffers builds both buttons, hidden, and wires each to what it leads to.
func newOffers(relay func()) *offers {
	o := &offers{relay: relay}
	o.folderBtn = parts.NewButton(parts.Secondary, text.ButtonOpenFolder(), func() {
		if o.wroteInto != "" && o.openFolder != nil {
			o.openFolder(o.wroteInto)
		}
	})
	o.folderBtn.Hide()
	o.manifestBtn = parts.NewButton(parts.Secondary, text.ButtonOpenManifest(), func() {
		if o.wroteManifest != "" && o.openFile != nil {
			o.openFile(o.wroteManifest)
		}
	})
	o.manifestBtn.Hide()
	return o
}

// through says which desktop these buttons reach. Called by every screen as
// it is built, because the host is what knows how to open anything.
func (o *offers) through(host Host) {
	o.openFolder = host.OpenFolder
	o.openFile = host.OpenFile
}

// theFolder shows the way to the files, once there are some.
//
// Asked of the RESULT rather than of the box on the screen: a run that wrote
// nothing has nothing to show, and a run that was stopped after three files
// has three files somebody may well want to look at. The manifest is what
// knows.
func (o *offers) theFolder(res *engine.Result) {
	if res == nil || res.Manifest == nil || len(res.Manifest.Files) == res.Failures {
		return
	}
	if o.wroteInto == "" {
		return
	}
	o.folderBtn.Show()
	o.relay()
}

// theManifest shows the way to the record, once there is one.
//
// Asked about the SAVING rather than about the run, which is the difference
// from the folder above: a run that wrote files and could not save its
// manifest has a folder worth opening and no record to open. The screen
// refuses about that in its own sentence, and a button pointing at the file
// that refusal is about would be the screen disagreeing with itself.
//
// The path is the one saving used rather than one worked out again here. The
// manifest's name is a field on the batch screen, so a second way of arriving
// at it is a second chance to name a different file.
func (o *offers) theManifest(path string) {
	if path == "" {
		return
	}
	o.wroteManifest = path
	o.manifestBtn.Show()
	o.relay()
}

// forget takes both offers away when the next run starts.
func (o *offers) forget() {
	o.wroteInto = ""
	o.folderBtn.Hide()
	o.wroteManifest = ""
	o.manifestBtn.Hide()
	o.relay()
}
