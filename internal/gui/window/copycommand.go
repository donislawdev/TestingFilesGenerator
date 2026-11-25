package window

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/commandline"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The Copy command button: the command that does what a screen would do, put on
// the clipboard to paste into a terminal, a script or a pipeline.
//
// What the command says is read off the same settle the Preview and Generate
// buttons read, so a form those buttons refuse is refused here under the same
// boxes and nothing reaches the clipboard. What the engine judges - the
// smallest size of a format, free space, a name already taken - the command
// judges when it runs, in the same words, the way it judges a recipe written by
// hand. Asking the engine here would mean planning every file on the thread that
// draws the window, and a set of a million files would freeze it for a copy.
// See docs, the analysis of 2026-11-25.

// copying is the button and what pressing it needs.
type copying struct {
	btn *parts.Button
	// line writes the screen's command, or refuses the way Generate would.
	line func() (*commandline.Line, error)
	// put is the clipboard, tell the status line, clear takes the marks of an
	// earlier refusal away and refuse puts one under its box.
	put    func(string)
	tell   func(string)
	clear  func()
	refuse func(error)
	// fields gives a setting the label its box shows, for the line that
	// names boxes.
	fields *parts.Fields
}

// newCopying builds the button. The screen hands over where its words go,
// because the runner of the three work screens and the Tools screen keep them
// in different places.
func newCopying(host Host, fields *parts.Fields, line func() (*commandline.Line, error)) *copying {
	c := &copying{line: line, put: host.Copy, fields: fields}
	c.btn = parts.NewButton(parts.Secondary, text.ButtonCopyCommand(), c.press).InTheBar()
	return c
}

// press writes the command for this computer and puts it on the clipboard.
func (c *copying) press() {
	c.clear()
	l, err := c.line()
	if err != nil {
		c.refuse(err)
		return
	}
	shell := commandline.ShellFor(runtime.GOOS)
	written, err := l.Write(commandline.Locate(thisSystem()), shell)
	if err != nil {
		c.refuse(unwritable(err))
		return
	}
	c.put(written.Text)
	c.tell(c.said(written, shell))
}

// said is what the status line says after a copy: that it happened and for
// which shell, then anything the person needs before they paste it.
func (c *copying) said(w commandline.Written, shell commandline.Shell) string {
	lines := []string{text.CommandCopiedTerminal()}
	if shell == commandline.PowerShell {
		lines[0] = text.CommandCopiedPowerShell()
	}
	if len(w.OlderPowerShell) > 0 {
		lines = append(lines, text.CommandNeedsPowerShell7(c.labelsOf(w.OlderPowerShell)))
	}
	if w.Program == commandline.Missing {
		lines = append(lines, text.CommandWithoutTfg())
	}
	return strings.Join(lines, " ")
}

// labelsOf is the labels of the boxes these settings came from, each once, in
// the order the line has them.
func (c *copying) labelsOf(settings []string) []string {
	var labels []string
	seen := map[string]bool{}
	for _, s := range settings {
		label := s
		if f := c.fields.Lookup(s); f != nil && f.Label != "" {
			label = f.Label
		}
		if !seen[label] {
			seen[label] = true
			labels = append(labels, label)
		}
	}
	return labels
}

// unwritable words a value the line cannot carry, under the box it came from.
func unwritable(err error) error {
	var u *commandline.Unwritable
	if !errors.As(err, &u) {
		return err
	}
	said := text.CommandControlCharacter()
	switch u.Reason {
	case commandline.ControlCharacter:
	case commandline.NotText:
		said = text.CommandNotText()
	case commandline.CommaInDamage:
		said = text.CommandCommaInDamage()
	case commandline.LeadingDash:
		said = text.CommandLeadingDash()
	}
	return saying(u.Setting, errors.New(said))
}

// thisSystem is what commandline.Locate reads of this computer.
//
// The path of the window with its links followed, because a window started
// through the link next to its bundle on macOS reports the link, and the
// folder that matters is the one the bundle is in.
func thisSystem() commandline.System {
	exe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
	}
	return commandline.System{GOOS: runtime.GOOS, Getenv: os.Getenv, Executable: exe, Stat: os.Stat}
}

// offerCopy makes the button for a screen run by a runner, and answers it for
// the screen to put on its rail. A function rather than a method, because the
// runner stands at its ceiling of methods.
//
// At the right end of the bar rather than in the row of the run buttons, so
// the row stays centred on Preview and Generate - see parts.ActionBarEnds.
// Held while a run owns the screen like Add a batch, because the form it
// reads is frozen then.
func offerCopy(r *runner, host Host, line func() (*commandline.Line, error)) *parts.Button {
	c := newCopying(host, r.fields, line)
	c.tell = func(said string) { showOn(r.status, said) }
	c.clear = r.clearProblems
	c.refuse = r.refuse
	r.busy.also = append(r.busy.also, c.btn)
	return c.btn
}

// The values the command line takes when a flag is not given. A line leaves
// out a flag the command would fill in the same way, so it reads like one a
// person would type. That these are still the command line's own is held by the
// guard that runs a copied line and compares the files - copycommand_test.go -
// because a default that moved would make the same line mean something else.
const (
	commandCount = "1"
	commandSeed  = "0"
)

// unlessDefault is the value, or nothing when the command line would fill in
// the same one by itself.
func unlessDefault(value, byItself string) string {
	if value == byItself {
		return ""
	}
	return value
}

// absolute is a folder or file as a path from the root.
//
// The window reads a relative path from the folder it was started in, and a
// terminal reads it from wherever the person is - so the same words would name
// two places. Written whole, the line names the one the window would use.
func absolute(path string) string {
	if path == "" {
		return ""
	}
	whole, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return whole
}

// memoryWritten is --memory as this window would run, or nothing for auto,
// which is what the command line does without the flag.
func memoryWritten(kept Remembered) string {
	stated := kept.Memory()
	if m, err := core.ParseMemory(stated); err != nil || m == (core.Memory{}) {
		return ""
	}
	return stated
}

// damageWritten is the damage of a target as --damage takes it, or nothing.
// The window offers one damage at a time, so a chain longer than one is not
// something this screen can hold.
func damageWritten(chain damage.Chain) (string, error) {
	if len(chain) == 0 {
		return "", nil
	}
	return commandline.DamageValue(chain[0].ID, chain[0].Values)
}

// commandLine is the command for the Single batch screen: tfg generate with a
// flag for every box somebody stated.
func (g *Generate) commandLine() (*commandline.Line, error) {
	targets, opt, err := g.settle()
	if err != nil {
		return nil, err
	}
	t := targets[0]
	damaged, err := damageWritten(t.Damage)
	if err != nil {
		return nil, err
	}
	l := commandline.New(commandline.Generate).
		Flag(commandline.Format, t.Format, engine.SettingFormat).
		Flag(commandline.Size, g.size.Text, format.SettingSize).
		Flag(commandline.Count, unlessDefault(strconv.Itoa(len(t.Sizes)), commandCount), engine.SettingCount).
		Flag(commandline.ID, unlessDefault(t.ID, startingBatchName), engine.SettingID).
		Flag(commandline.NameTemplate, t.NameTmpl, engine.SettingName)
	stated := g.properties()
	for _, p := range g.props {
		if v, ok := stated[p.Name]; ok {
			l.Flag(commandline.Set, p.Name+"="+v, p.Name)
		}
	}
	return l.Flag(commandline.Damage, damaged, recipe.KeyDamage).
		Switch(commandline.Clean, !t.Label).
		Flag(commandline.Seed, unlessDefault(strconv.FormatInt(opt.Seed, 10), commandSeed), engine.SettingSeed).
		Flag(commandline.Out, absolute(opt.OutDir), engine.SettingOutDir).
		Switch(commandline.NumberIfTaken, opt.NumberIfTaken).
		Flag(commandline.Memory, memoryWritten(g.kept()), ""), nil
}

// commandLine is the command for the Presets screen: tfg generate --preset with
// a flag for every parameter somebody stated, which is how the command line
// takes them.
func (p *Preset) commandLine() (*commandline.Line, error) {
	_, opt, err := p.settle()
	if err != nil {
		return nil, err
	}
	l := commandline.New(commandline.Generate).Flag(commandline.Preset, p.pick.Selected, settingPreset)
	for _, f := range p.params {
		l.Flag(commandline.Named(f.Name), f.Value(), f.Name)
	}
	return l.Flag(commandline.Seed, unlessDefault(strconv.FormatInt(opt.Seed, 10), commandSeed), engine.SettingSeed).
		Flag(commandline.Out, absolute(opt.OutDir), engine.SettingOutDir).
		Switch(commandline.NumberIfTaken, opt.NumberIfTaken).
		Flag(commandline.Memory, memoryWritten(p.kept()), ""), nil
}

// toolCopying is the button on the Tools screen, which keeps its words and its
// refusals in places of its own rather than in a runner.
func toolCopying(host Host, t *Tools) *copying {
	c := newCopying(host, t.fields, func() (*commandline.Line, error) { return toolsCommandLine(t) })
	c.tell = func(said string) { showOn(t.status, said) }
	c.clear = func() {
		t.fields.ClearAll()
		t.fault.Clear()
	}
	// Every problem under its own box, the way a press of Run marks them.
	c.refuse = func(err error) {
		for _, one := range spread(err) {
			t.refuse(one)
		}
	}
	return c
}

// toolsCommandLine is the command for the Tools screen: tfg tool with the files it
// works on, whole, and a flag for every setting somebody stated.
//
// Checked the way Run checks it first, so a box Run would refuse is refused
// here and nothing reaches the clipboard.
//
// A function rather than a method, because Tools stands at the ceiling on the
// methods of a type.
func toolsCommandLine(t *Tools) (*commandline.Line, error) {
	d := t.chosen
	in := t.form.request()
	if bad := d.CheckEach(in); len(bad) > 0 {
		return nil, errors.Join(bad...)
	}
	l := commandline.New(commandline.Tool, d.ID)
	for _, input := range d.Inputs {
		l.Argument(absolute(in.Inputs[input.Name]), input.Name)
	}
	for _, f := range t.form.settings {
		l.Flag(commandline.Named(f.Name), f.Value(), f.Name)
	}
	return l, nil
}
