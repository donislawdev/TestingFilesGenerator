package window

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Recipe is the screen that runs several batches in one go.
//
// It is the single batch screen's sibling rather than its replacement, and the
// difference between them is how many batches - not how advanced the person is.
// What it adds beyond a second batch is everything the engine can be asked for
// that one target's worth of form had no room for: a size range, a boundary, a
// class, a declared expectation, the files inside an archive.
//
// No file is read and none is written. The screen holds a recipe in memory and
// runs it, which is the owner's decision of 2026-08-12: parity does not need
// saving, because the threshold a guard asks about is a value found in the
// manifest of a run started from the window. Opening and saving are a separate
// piece of work with their own four states, and docs/GUI.md section 4.3 is
// marked deferred rather than deleted, because the measurement behind the YAML
// library choice will be needed the day it comes back.
//
// The screen holds no rule about what a value may be. It collects text, hands it
// to recipe.Compose, and lets Parse judge - so a size is "2mb" until the recipe
// package says otherwise, and the refusal that comes back is already addressed to
// the setting it is about. That is G1, and it is why this screen cannot come to
// accept something "tfg generate" refuses.
//
// Every batch is drawn in full, one under another, and it is worth saying what
// that costs because it was measured rather than guessed. tools/probes/formheight
// on 2026-08-18: one batch makes a form 1038 px tall in 794 px of room, so this
// screen scrolls from the start - 244 px over, where the preset screen is 225 -
// and each further batch adds about the height of the block again.
//
// The alternative was a list of batches with one of them open at a time, which
// keeps the height constant however many there are. It was rejected, and not on
// taste: a refusal names the batch it is about, and the whole point of the
// addressing work behind this screen is that the box carrying it is marked. A
// batch whose fields are not on the screen has no box to mark, so refusals about
// every batch but the open one would fall back to the foot of the form - which is
// the defect this screen was built to avoid, reintroduced to save scrolling.
//
// So the form is long on purpose. What is NOT settled is whether it should stay
// that way at ten batches, and nobody has looked at ten.
type Recipe struct {
	*runner

	host Host

	batches []*batch

	// batchBox and outBox are refilled together by rebuild, and they have to be:
	// the address of a setting carries the position of its batch, so the whole
	// registry is built again whenever the list changes. A section built once and
	// left alone would hold controls the registry had forgotten, which is a
	// control whose refusal has nowhere to go.
	batchBox *fyne.Container
	outBox   *fyne.Container

	// addBtn lives in the bar at the foot rather than in the form, because the
	// form scrolls and one batch is taller than the window (O112).
	addBtn *widget.Button

	outDir   *parts.Entry
	manifest *parts.Entry
	seed     *parts.Entry
	label    *parts.Toggle

	tips *parts.Tips
	body fyne.CanvasObject
}

// batch is one target as the screen holds it.
//
// The controls are built once and kept, which is what makes adding and removing
// batches safe: the form is registered and laid out again when the list changes,
// because an address carries its position, but the widgets are the same pointers
// - so whatever somebody had typed is still in them. A rebuild that made new
// widgets would empty the form without saying so.
type batch struct {
	formatPick *parts.Chooser
	id         *parts.Entry
	count      *parts.Entry
	size       *parts.Entry
	sizeRange  *parts.Entry
	boundary   *parts.Entry

	// sizeWay is which of the three ways of saying how big this batch uses, and
	// the two boxes it does not use are hidden rather than absent.
	//
	// Hidden rather than absent because everything a refusal needs is set up
	// when a field is registered: the address it is marked by, the star, the
	// count of bytes under it. Building only the chosen one would mean building
	// it again on every change of mind, and a rebuilt box is a box that has
	// forgotten what was typed into it.
	//
	// Only the chosen one is sent - see draft. That is what makes the state the
	// engine refuses unreachable rather than merely discouraged.
	sizeWay   *widget.RadioGroup
	sizeBoxes map[string]fyne.CanvasObject

	// folded is whether this batch is put away, and it is a plain bool on the
	// batch rather than state inside the panel because the panel is built again
	// whenever the list of batches changes. A fold that lived in the panel
	// would spring open every time somebody added a batch.
	folded bool
	fold   *parts.Folding

	// The two sections inside a batch, and whether each is put away. Both
	// arrive folded - see batchBlock - so these start true rather than false,
	// which is the one place in this struct where the zero value is not what is
	// wanted and newBatch says so.
	//
	// Kept on the batch for the reason above and for one more: the settings
	// section is rebuilt whenever the format changes, so a fold living in it
	// would spring open every time somebody looked at another format.
	settingsFolded bool
	notesFolded    bool
	settings       *parts.Folding
	notes          *parts.Folding
	name           *parts.Entry
	group          *parts.Entry
	expected       *parts.Chooser
	reason         *parts.Chooser

	// declared is what the chosen format takes, and props are the controls drawn
	// from it. Both are replaced when the format changes and reused across a
	// rebuild, for the reason above.
	declared []format.Property
	props    []parts.PropertyField

	contents []*content
}

// content is one entry of a contains list: what an archive holds.
type content struct {
	formatPick *parts.Chooser
	count      *parts.Entry
	size       *parts.Entry
}

// NewRecipe builds the screen with one empty batch on it.
//
// One rather than none, because a screen that opens empty makes the first thing
// anybody does a press of "Add a batch" to find out what the screen is. One is
// also what the single batch screen shows, so the two read as one tool.
func NewRecipe(host Host, links ...fyne.CanvasObject) *Recipe {
	r := &Recipe{runner: newRunner(), host: host, tips: parts.NewTips()}
	r.runner.settle = r.settle
	r.runner.openFolder = host.OpenFolder
	// A refusal about a size belongs on the box the switch is showing.
	r.runner.readdress = r.readdressSizeWay
	// A box inside a folded batch cannot be brought into view by scrolling, so
	// the fold is opened first - see runner.unfold.
	r.runner.unfold = r.openFoldHolding

	r.outDir = parts.NewEntry()
	r.outDir.SetText(startingDirectory())
	r.manifest = parts.NewEntry()
	r.manifest.SetPlaceHolder(engine.DefaultManifestName)
	r.seed = parts.NewEntry()
	// What happens if it is left alone, in the place a box says that. Counted
	// on 2026-08-23 off the stored widget trees: six boxes on this screen held
	// nothing at all, and four of them are settings a run refuses when they are
	// empty. This one and the class below are the two that are not - so they
	// were the only boxes on any screen where "you may leave this" and "you
	// must fill this in" looked the same. See the note on newBatch.
	r.seed.SetPlaceHolder(text.PlaceholderLeftEmpty(strconv.Itoa(recipe.DefaultSeed)))
	r.label = parts.NewToggle(text.FieldLabel(), nil)

	r.batchBox = parts.FieldColumn()
	r.outBox = parts.FieldColumn()
	r.batches = []*batch{r.newBatch()}

	// In the bar rather than in the list, so the one control that makes this
	// screen what it is does not depend on scrolling to reach - see rebuild.
	// It is disabled with the rest of the form while a run is going, because
	// adding a batch mid run would rebuild the form under the run.
	r.addBtn = widget.NewButton(text.ButtonAddBatch(), r.addBatch)
	r.runner.alsoDisabled = append(r.runner.alsoDisabled, r.addBtn)

	r.body = r.tips.Over(container.NewBorder(
		nil,
		parts.ActionBar(rail(append([]fyne.CanvasObject{donateButton(host), parts.Divider(), r.addBtn}, links...)...),
			r.actions(), r.progress(), r.problem.Object()),
		nil, nil,
		(r.keepScroll(container.NewVScroll(parts.Screen(text.HeadingRecipe(), r.batchBox, r.outBox)))),
	))

	// The format of the first batch has to be chosen for its declared settings
	// to exist, and choosing it is what fills them in. The same ordering the
	// single batch screen needs, and for the same reason.
	r.batches[0].formatPick.SetSelected(format.IDs()[0])
	r.rebuild()

	// Said last, once the box it reads exists.
	r.runner.destination = r.OutDir
	r.runner.sayDestination()
	return r
}

// Object is the screen, to put in the window.
func (r *Recipe) Object() fyne.CanvasObject { return r.body }

// FirstField is where the keyboard starts: the format of the first batch. There
// is always a first batch - the last one cannot be removed.
func (r *Recipe) FirstField() fyne.Focusable { return r.batches[0].formatPick }

// OutDir is where this screen would write, for the screen somebody moves to.
func (r *Recipe) OutDir() string { return r.outDir.Text }

// SetOutDir points this screen at a directory chosen on another one. The same
// reasoning as on the other two screens: both used to hold their own box, so
// they agreed until somebody changed one and then silently disagreed.
func (r *Recipe) SetOutDir(dir string) {
	if dir != "" {
		r.outDir.SetText(dir)
	}
}

// newBatch builds the controls of one batch, without placing them.
func (r *Recipe) newBatch() *batch {
	b := &batch{
		id:        parts.NewEntry(),
		count:     parts.NewEntry(),
		size:      parts.NewEntry(),
		sizeRange: parts.NewEntry(),
		boundary:  parts.NewEntry(),
		name:      parts.NewEntry(),
		group:     parts.NewEntry(),
		sizeWay:   newSizeWaySwitch(),
		// Both sections arrive put away. The owner's decision of 2026-08-25,
		// and the number under it is 248 px of form per screen (O98) for
		// settings a format works out on its own when nobody states them.
		settingsFolded: true,
		notesFolded:    true,
	}
	b.name.SetPlaceHolder(text.PlaceholderNameTemplate)
	// What happens if the box is left alone, in the place a box says that.
	//
	// The single batch screen arrives with "files" and "1" typed into the same
	// two settings and this screen arrives empty, which read as one of the two
	// being wrong. Neither is: a value typed in is a value stated, and on this
	// screen an unstated setting has to stay unstated, because that is what
	// makes the recipe leave the key out. The difference is deliberate and it
	// was written down only in the code, so from the screen it looked like an
	// inconsistency (O109). A placeholder says it without stating anything.
	// The id is not given one, and that is the difference between a setting
	// with a default and a setting without: a batch with no id is refused
	// rather than filled in, because an id is what anchors a batch's seed.
	b.count.SetPlaceHolder(text.PlaceholderLeftEmpty(strconv.Itoa(recipe.DefaultCount)))
	// The class is optional metadata, so it says so the same way. It stood
	// empty and silent beside the id above it, which is REFUSED when empty -
	// two boxes side by side, one you must fill in and one you need not,
	// drawn identically. The rule this closes is worth more than the two
	// fields: a box with a hint may be left alone, a box with nothing in it
	// may not, and TestABoxYouMayLeaveAloneSaysSo holds it from the registry.
	b.group.SetPlaceHolder(text.PlaceholderNotStated())

	// Nothing is filled in with a default, on either list. A list carrying a
	// value cannot say "I did not state this", and an expectation nobody stated
	// has to stay unstated - manifest rule MF5, because an invented expectation
	// produces false failures in somebody else's test run.
	b.expected = parts.NewChooser(recipe.Outcomes(), nil)
	b.expected.PlaceHolder = text.PlaceholderNotStated()
	b.reason = parts.NewChooser(recipe.Reasons(), nil)
	b.reason.PlaceHolder = text.PlaceholderNotStated()

	b.formatPick = parts.NewChooser(format.IDs(), func(id string) {
		b.formatPick.KindOf = parts.KindOfFile
		r.onFormatChosen(b, id)
	})
	return b
}

// rebuild draws every batch and registers every field under its address.
//
// Called on every structural change, and it has to be: an address carries the
// position of its batch, so removing the first batch moves every setting of every
// batch after it. Registering the lot again is the only way that stays correct
// without a second place remembering which index a widget belongs to.
func (r *Recipe) rebuild() {
	r.fields.KeepFirst(0)
	r.batchBox.RemoveAll()
	r.outBox.RemoveAll()

	panels := make([]fyne.CanvasObject, 0, len(r.batches)+1)
	for i, b := range r.batches {
		panels = append(panels, r.batchBlock(i, b))
	}
	// Adding a batch is NOT in this list any more, as of 2026-08-19. It sat at
	// the end of it - under the output section before that - and one batch is
	// already taller than the window, so the only control that makes this
	// screen different from the other two was off the bottom of it when the
	// screen opened. It is in the bar at the foot now, which does not scroll
	// (O112).
	// Stacked rather than added straight to the box, so the space between two
	// batches is the space between two panels. Added one by one they sat as close
	// together as two fields inside one of them, and two panels a hair apart read
	// as one panel with a line across it - reported on 2026-08-19.
	r.batchBox.Add(parts.Stacked(panels...))
	// After the batches, so that Tab walks the screen in the order it is read.
	r.outBox.Add(r.outputSection())

	r.batchBox.Refresh()
	r.outBox.Refresh()
}

// batchBlock is one batch as it appears on the screen.
//
// The setting names and the shape of an address both come from the recipe
// package, so a misspelling here is a compile error rather than a refusal that
// silently lands at the foot of the form. What that still cannot prove is that
// the RIGHT name is registered for a given box - both sides can agree and both be
// wrong - and that is what TestEveryRefusalAboutABatchMarksThatBatchsBox is for.
func (r *Recipe) batchBlock(index int, b *batch) fyne.CanvasObject {
	at := func(setting string) string {
		return recipe.TargetAddress(index+1, setting)
	}
	add := func(setting, label, hint string, detail parts.Detail, control fyne.CanvasObject) fyne.CanvasObject {
		return r.fields.Add(at(setting), label, hint, detail, control)
	}
	// Before the fields, because Add reads it, and per batch because an address
	// carries the batch it belongs to - the id of a second batch is a different
	// name from the id of the first.
	//
	// The id, and the way of saying how big that the switch is on.
	//
	// All three ways are marked and only one of them is ever on the screen, so
	// what a person sees is one star on the box in front of them. Until
	// 2026-08-25 none of the three carried one, and that was right at the time:
	// they stood side by side, filling any one satisfied the run, and three
	// stars would have read as "fill all three". The switch makes the one that
	// is shown the one the run will not do without, which is exactly what a
	// star says.
	r.fields.Require(at(recipe.KeyID),
		at(recipe.KeySize), at(recipe.KeySizeRange), at(recipe.KeyBoundary))
	// Two of the three ways of saying how big hold ONE size, so those two say
	// what it comes to. A size range holds two numbers and a count under it
	// would be answering about half the box, so it is left out - see
	// Fields.InBytes.
	r.fields.InBytes(at(recipe.KeySize), at(recipe.KeyBoundary))

	// The block is titled by the section below, so nothing repeats it here. The
	// heading was drawn twice until it was looked at: parts.Section takes a title
	// and this put the same words inside it as well.
	//
	// Removing is offered from the second batch onwards. A screen with one batch
	// and a Remove button invites somebody to press it and be left with a form
	// that can produce nothing.
	// Removing moved into the head of the fold on 2026-08-25, beside copying,
	// so a batch that is put away is still a batch somebody can act on. It was
	// at the right edge of the first row before that, which is where O93 put
	// the buttons that start a run - left aligned under the title it read as a
	// field label rather than as something to press.
	var rows []fyne.CanvasObject

	rows = append(rows,
		add(recipe.KeyFormat, text.FieldFormat(), text.HintFormat(),
			r.tips.Say(text.DetailFormat()), b.formatPick),
		r.fields.Row(
			add(recipe.KeyID, text.FieldTargetID(), text.HintTargetID(),
				r.tips.Say(text.DetailTargetID()), b.id),
			add(recipe.KeyCount, text.FieldCount(), "", parts.NoDetail, parts.Numeric(b.count)),
		),
		// One way of saying how big, chosen from three, since 2026-08-25.
		//
		// They were three boxes side by side with a sentence above them saying
		// that only one might be filled in - O114, and a sentence because the
		// screen let somebody fill in two and learn it from a refusal. A switch
		// takes the state away rather than describing it, and the box it leaves
		// is a full row wide rather than a third of one.
		r.sizeWayFor(b, at, add),
		add(recipe.KeyName, text.FieldNameTemplate(), text.HintNameTemplate(),
			r.tips.Say(text.DetailNameTemplate()), b.name),
	)

	// A format that declares nothing has no settings section at all, so the
	// list is filtered rather than padded: a nil in a column is a gap where a
	// heading would be.
	for _, section := range []fyne.CanvasObject{
		r.declaredSettings(b, at), r.manifestNotes(b, add), r.contentsBlock(index, b),
	} {
		if section != nil {
			rows = append(rows, section)
		}
	}

	// Folded away, since 2026-08-25, and the number behind that is worth
	// carrying: one batch is a form 913 px tall in 849 px of room and each
	// further one adds 659 px, so ten batches is 6884 px - eight screens of
	// scrolling for something this screen exists to make easy.
	//
	// The two buttons stay in the head. A folded batch that could not be copied
	// or removed would be a batch somebody has to open to do the two things
	// they are most likely to want from a list of them - and copying is what
	// makes several batches quick to write in the first place, since batches
	// usually differ from each other in one setting.
	head := []fyne.CanvasObject{
		widget.NewButton(text.ButtonDuplicateBatch(), func() { r.duplicateBatch(index) }),
	}
	if len(r.batches) > 1 {
		head = append(head, widget.NewButton(text.ButtonRemoveBatch(), func() { r.removeBatch(index) }))
	}
	b.fold = parts.NewFolding(text.BatchHeading(index+1), head, rows...)
	r.wire(b.fold, &b.folded, b.summary)
	return b.fold.Object()
}

// contentsBlock is the list of what an archive holds.
//
// Offered for every format rather than only for containers, and that is a
// deliberate choice with a named cost. Asking a format whether it is a container
// and hiding the block otherwise would put a rule about formats in the window,
// and the refusal for telling a plain file to contain something is one the engine
// already words and addresses. What it costs is a button on a TXT batch that
// leads to a refusal - which is why a button is all that shows until somebody
// presses it.
func (r *Recipe) contentsBlock(index int, b *batch) fyne.CanvasObject {
	addContents := widget.NewButton(text.ButtonAddContents(), func() {
		b.contents = append(b.contents, r.newContent())
		r.rebuild()
	})
	if len(b.contents) == 0 {
		return addContents
	}

	rows := []fyne.CanvasObject{parts.Heading(text.ContentsHeading())}
	for j, c := range b.contents {
		at := func(setting string) string {
			return recipe.ContentAddress(index+1, j+1, setting)
		}
		entry := j
		rows = append(rows, r.fields.Row(
			r.fields.Add(at(recipe.KeyFormat), text.FieldFormat(), "", parts.NoDetail, c.formatPick),
			r.fields.Add(at(recipe.KeyCount), text.FieldCount(), "", parts.NoDetail, parts.Numeric(c.count)),
			r.fields.Add(at(recipe.KeySize), text.FieldSize(), "", parts.NoDetail, parts.Numeric(c.size)),
			widget.NewButton(text.ButtonRemoveContents(), func() { r.removeContent(b, entry) }),
		))
	}
	rows = append(rows, addContents)
	return container.NewVBox(rows...)
}

func (r *Recipe) newContent() *content {
	return &content{
		formatPick: parts.NewChooser(format.IDs(), nil),
		count:      parts.NewEntry(),
		size:       parts.NewEntry(),
	}
}

// outputSection is the part every batch shares, registered as it is drawn so the
// registry and the screen cannot come apart.
func (r *Recipe) outputSection() fyne.CanvasObject {
	// Before the fields below, because Add reads it.
	//
	// This was briefly NOT required here, and the story is worth the four lines
	// because it is the guard doing its job twice. The star went on all three
	// screens on the assumption that they agree about an empty directory. They
	// did not, TestAStarIsOnEveryBoxTheRunWillNotDoWithout went red, and taking
	// the star off was the honest thing to do at that moment - the screen would
	// otherwise have been asking for work the run did not need.
	//
	// Then the disagreement itself was fixed rather than described, so the star
	// is true here now. See statedDirectory and O125.
	//
	// The manifest name and the seed both say what they fall back to, so this
	// section has one box that has to be answered.
	r.fields.Require(recipe.KeyOutputDir)
	return parts.Section(text.SectionOutput(),
		r.fields.Add(recipe.KeyOutputDir, text.FieldOutputDir(), text.HintOutputDir(),
			r.tips.Say(text.DetailOutputDir()), chooserFor(r.host, r.outDir)),
		r.fields.Row(
			r.fields.Add(recipe.KeyOutputManifest, text.FieldManifest(), text.HintManifest(),
				r.tips.Say(text.DetailManifest()), r.manifest),
			r.fields.Add(recipe.KeySeed, text.FieldSeed(), text.HintSeed(),
				r.tips.Say(text.DetailSeed()), parts.Numeric(r.seed)),
		),
		r.fields.AddToggle(recipe.KeyDefaultsLabel, text.FieldLabel(), "",
			r.tips.Say(text.DetailLabel()), r.label),
	)
}

// addBatch puts another batch at the end of the list.
func (r *Recipe) addBatch() {
	r.batches = append(r.batches, r.newBatch())
	// A new batch has no format until one is chosen, and its declared settings
	// come with that choice. Chosen here rather than left empty so that a batch
	// arrives looking like the one above it.
	r.batches[len(r.batches)-1].formatPick.SetSelected(format.IDs()[0])
	r.rebuild()
}

// removeBatch drops one batch. The last cannot go: a screen with no batches can
// produce nothing, and would answer a press with a refusal about a document
// rather than about anything anybody did.
// duplicateBatch copies one batch and puts the copy under it.
//
// Batches usually differ from each other in one setting - a size, a format, a
// count - so writing the second one from an empty form is typing the first one
// again. The copy is a new batch with the same values rather than a shared one,
// so changing it changes nothing else.
//
// The name is left empty on the copy, on purpose. Two targets with one id is a
// refusal the recipe reader already words and addresses, and handing somebody a
// form that is refused the moment they press a button would be a copy that has
// to be repaired before it can be used. Empty is refused too, and it is refused
// pointing at a box that is asking to be filled in.
func (r *Recipe) duplicateBatch(index int) {
	if index < 0 || index >= len(r.batches) {
		return
	}
	from := r.batches[index]
	to := r.newBatch()

	to.formatPick.SetSelected(from.formatPick.Selected)
	to.count.SetText(from.count.Text)
	to.size.SetText(from.size.Text)
	to.sizeRange.SetText(from.sizeRange.Text)
	to.boundary.SetText(from.boundary.Text)
	to.name.SetText(from.name.Text)
	to.group.SetText(from.group.Text)
	to.expected.SetSelected(from.expected.Selected)
	to.reason.SetSelected(from.reason.Selected)
	if from.sizeWay != nil && to.sizeWay != nil {
		to.sizeWay.SetSelected(from.sizeWay.Selected)
	}

	rest := append([]*batch{to}, r.batches[index+1:]...)
	r.batches = append(r.batches[:index+1], rest...)
	r.rebuild()
}

func (r *Recipe) removeBatch(index int) {
	if len(r.batches) <= 1 || index < 0 || index >= len(r.batches) {
		return
	}
	r.batches = append(r.batches[:index], r.batches[index+1:]...)
	r.rebuild()
}

func (r *Recipe) removeContent(b *batch, index int) {
	if index < 0 || index >= len(b.contents) {
		return
	}
	b.contents = append(b.contents[:index], b.contents[index+1:]...)
	r.rebuild()
}

// onFormatChosen replaces the declared settings of one batch.
func (r *Recipe) onFormatChosen(b *batch, id string) {
	d, err := format.Get(id)
	if err != nil {
		// The registry filled the list, so a press cannot land here. A build
		// where the two have come apart can, and saying so beats a batch with no
		// settings and no reason given.
		r.refuse(err)
		return
	}

	b.declared = d.Properties
	b.props = make([]parts.PropertyField, 0, len(d.Properties))
	for _, p := range d.Properties {
		b.props = append(b.props, parts.FromProperty(p))
	}
	r.rebuild()
}

// settle turns the screen into a run.
//
// It parses nothing and judges nothing. Every value crosses as the text somebody
// typed, Compose writes the document, and Parse decides whether it describes a
// run - so there is no rule here to keep in step with the recipe reader, and a
// refusal arrives already addressed to the box it came from.
func (r *Recipe) settle() ([]engine.Target, engine.Options, error) {
	var none engine.Options

	doc := recipe.Document{
		Seed:     r.seed.Text,
		OutDir:   r.outDir.Text,
		Manifest: r.manifest.Text,
		Label:    &r.label.Checked,
	}
	for _, b := range r.batches {
		doc.Targets = append(doc.Targets, b.draft())
	}

	src, err := recipe.Compose(doc)
	if err != nil {
		return nil, none, err
	}
	rec, err := recipe.Parse(src, text.HeadingRecipe())
	if err != nil {
		return nil, none, err
	}
	hash, err := recipe.Hash(src)
	if err != nil {
		return nil, none, err
	}

	targets := make([]engine.Target, 0, len(rec.Targets))
	for _, t := range rec.Targets {
		targets = append(targets, engineTarget(t))
	}
	return targets, engine.Options{
		OutDir:       statedDirectory(r.outDir.Text, rec.Output.Dir),
		Seed:         rec.Seed,
		Command:      "tfg-gui",
		ManifestName: manifestName(rec.Output.Manifest),
		RecipeHash:   hash,
	}, nil
}

// statedDirectory is where this screen would write, with an emptied box left
// empty rather than turned into the current directory.
//
// O125. The recipe reader answers "." for a recipe with no output.dir, and for
// a recipe FILE that is right - leaving the key out is how somebody says "put
// them here", and tfg generate has always meant that. On this screen it is
// wrong, and the difference is which question the emptiness answers: this box
// arrives filled in, so empty is not "I never said" but "I cleared it". Handing
// the reader's answer to the engine turned that into a silent run into whatever
// directory the window happened to be started from - which is the defect O102
// fixed for the other two screens on 2026-08-05, still open here because the
// three screens were assumed to agree and nobody had asked.
//
// Measured 2026-08-24 before the change: a recipe with no output section writes
// files_0001.txt and manifest.json beside the working directory, and the window
// took the same path. The other two screens hand the engine the empty string
// and it refuses. This makes the third do the same, so the engine judges all
// three alike rather than this one quietly not asking.
//
// The parsed value rather than the box for everything else, so a path that
// changed on its way through the document is the path that gets used.
func statedDirectory(typed, parsed string) string {
	if typed == "" {
		return ""
	}
	return parsed
}

// manifestName is what the run writes its record to, falling back to the name
// every other surface uses when nobody said.
func manifestName(stated string) string {
	if stated == "" {
		return engine.DefaultManifestName
	}
	return stated
}

// draft is one batch as text, ready to be composed into a document.
func (b *batch) draft() recipe.TargetDraft {
	props := map[string]string{}
	for _, f := range b.props {
		// An empty control means the setting was not stated, and a format that
		// works its own answer out has to be allowed to.
		if v := f.Value(); v != "" {
			props[f.Name] = v
		}
	}
	if len(props) == 0 {
		props = nil
	}

	var inside []recipe.ContentDraft
	for _, c := range b.contents {
		inside = append(inside, recipe.ContentDraft{
			Format: c.formatPick.Selected,
			Count:  c.count.Text,
			Size:   c.size.Text,
		})
	}

	return recipe.TargetDraft{
		ID:             b.id.Text,
		Format:         b.formatPick.Selected,
		Count:          b.count.Text,
		Size:           b.statedSize(recipe.KeySize),
		SizeRange:      b.statedSize(recipe.KeySizeRange),
		Boundary:       b.statedSize(recipe.KeyBoundary),
		Name:           b.name.Text,
		Group:          b.group.Text,
		Expected:       b.expected.Selected,
		ExpectedReason: b.reason.Selected,
		Properties:     props,
		Contains:       inside,
	}
}
