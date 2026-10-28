package text

import "github.com/donislawdev/TestingFilesGenerator/internal/core"

// The words of the Preferences screen, out of screens.go on 2026-10-27 when
// the Memory section took that file past the ceiling on its length - the
// screen's sentences are one part of what the file held, and it is split by
// what the parts do.

// The Preferences screen: which language the window speaks, and what it keeps
// between runs. Nothing on it changes the files the window makes - a setting
// that did would be an input the recipe does not carry, and the command the
// window offers to show would make different files (docs/USTAWIENIA-2026-09-29.md
// section 2).
//
// "Preferences" and not "Settings" for the tab, because Settings is already the
// section of the form where a format's own settings stand, and two places
// called the same would read as one.
func TabPreferences() string { return say("TabPreferences", "Preferences") }

func SubtitlePreferences() string {
	return say("SubtitlePreferences", "How this window speaks, how much memory its runs may take, and what it keeps between runs. Nothing here changes the files it makes.")
}

func SectionLanguage() string { return say("SectionLanguage", "Language") }
func SectionMemory() string   { return say("SectionMemory", "Memory") }
func FieldMemoryWay() string  { return say("FieldMemoryWay", "Memory for a run") }
func FieldMemorySize() string { return say("FieldMemorySize", "Memory size") }
func HintMemorySize() string  { return say("HintMemorySize", "8gb") }
func MemoryWayAuto() string   { return say("MemoryWayAuto", "Automatic") }
func MemoryWayCustom() string { return say("MemoryWayCustom", "Custom") }
func MemoryWayOff() string    { return say("MemoryWayOff", "No limit") }

// PreferencesMemoryScope says what the memory setting is for, under it. The
// floor of the plan under Automatic is said from the constant that sets it,
// the way --help says it.
func PreferencesMemoryScope() string {
	return sayf("PreferencesMemoryScope", "How much memory one run may take, the plan and the files being written together. Pictures wait for room rather than all being held at once, and a plan larger than this is refused before anything is written. Under Automatic, a plan of up to {{.Floor}} is never refused. It applies from the next run, and nothing in the files changes.",
		map[string]any{"Floor": HumanBytes(core.MaxPlanBytes)})
}

// MemoryAutoIs is what Automatic comes to on this computer.
func MemoryAutoIs(size string) string {
	return sayf("MemoryAutoIs", "Runs may take {{.Size}}, half of the memory this computer gives the program.", map[string]any{"Size": size})
}

// MemoryAutoUnknown is Automatic on a computer that could not be asked.
func MemoryAutoUnknown(size string) string {
	return sayf("MemoryAutoUnknown", "The memory of this computer could not be read, so runs take {{.Size}}, the most every run took before this setting.", map[string]any{"Size": size})
}

func MemoryCustomIs() string {
	return say("MemoryCustomIs", "Runs may take the size above.")
}

func MemoryCustomEmpty() string {
	return say("MemoryCustomEmpty", "Write a size such as 8gb. Until there is one, runs use Automatic.")
}

func MemoryOffIs() string {
	return say("MemoryOffIs", "Runs take what they need. Pictures are not held back and the plan has no limit, so a very large run can be ended by the system without a word.")
}

// MemoryUnreadable is a kept setting that does not read as one.
func MemoryUnreadable(value string) string {
	return sayf("MemoryUnreadable", "The memory setting kept before, {{.Value}}, could not be read. Runs use Automatic until you choose again.", map[string]any{"Value": value})
}
func SectionRemembered() string { return say("SectionRemembered", "What this window remembers") }
func FieldLanguage() string     { return say("FieldLanguage", "Window language") }

// LanguageName is what the language of a catalogue is called in that language
// itself. Every catalogue carries it, and it is how that language is listed on
// the Preferences screen - so a person who cannot read the language the window
// speaks now can still find their own.
func LanguageName() string { return say("LanguageName", "English") }

// ChoiceSameAsSystem is the first entry of the language list, and what nobody
// having chosen means. It names the language that choice gives on this machine,
// because "the system" alone does not say whether that is a language this
// window carries.
func ChoiceSameAsSystem(lang string) string {
	return sayf("ChoiceSameAsSystem", "Same as the system ({{.Language}})", map[string]any{"Language": lang})
}

// PreferencesLanguageScope says what the language reaches, because a list of
// languages invites the question whether the files will be in it too.
func PreferencesLanguageScope() string {
	return say("PreferencesLanguageScope", "Only the window changes language. The files it makes, recipes, manifests and the command line do not.")
}

// PreferencesNextStart is said once a language other than the one on screen is
// chosen. A choice that changed nothing visible and said nothing would read as
// a list that does not work.
func PreferencesNextStart() string {
	return say("PreferencesNextStart", "The window speaks the chosen language from the next time it opens.")
}

func ButtonRestart() string { return say("ButtonRestart", "Restart now") }

// PreferencesRestartClears is beside the button, because the form on the other
// tabs is not kept across a restart and a person who typed a recipe there
// should know before pressing, not after.
func PreferencesRestartClears() string {
	return say("PreferencesRestartClears", "Restarting closes this window and opens it again. Anything typed on the other tabs is cleared.")
}

// PreferencesRestartBusy stands in for the button's use while files are being
// made: closing the window stops the run (docs/GUI.md G7), and a restart must
// not be a way to stop one without saying so.
func PreferencesRestartBusy() string {
	return say("PreferencesRestartBusy", "Files are being made. Restart once they are done.")
}

// PreferencesRestartFailed is the four parts of D6: what failed, why, and what
// still works - the choice is saved, so opening the window by hand gives it.
func PreferencesRestartFailed(reason string) string {
	return sayf("PreferencesRestartFailed", "The window could not start again: {{.Reason}}. The language chosen is saved. Open the program again to see it.", map[string]any{"Reason": reason})
}

// PreferencesMissing is said when a language chosen before is not in this
// version. Untouchable rule 6: the window does not quietly speak something
// else.
func PreferencesMissing(tag string) string {
	return sayf("PreferencesMissing", "The language chosen before, {{.Tag}}, is not in this version. The window follows the system until you choose again.", map[string]any{"Tag": tag})
}

// PreferencesKept says what is kept, in words, and that it stays on this
// computer. The folder picker's own memory is named as well: it writes into the
// same place, and a list of what is kept that left it out would be untrue.
func PreferencesKept() string {
	return say("PreferencesKept", "Between runs this window keeps four things: the output directory, the size of the window, and the language and the memory chosen above. The folder picker also keeps the last folder it showed. Nothing leaves this computer.")
}

func PreferencesKeptIn() string { return say("PreferencesKeptIn", "They are kept in this folder:") }

func ButtonForget() string { return say("ButtonForget", "Forget") }

// PreferencesForgetWhat is beside the button, so it says before the press what
// the press does and what it leaves alone.
func PreferencesForgetWhat() string {
	return say("PreferencesForgetWhat", "Forget clears the four things above. This window then keeps nothing when it closes, and the next one opens the way the first one did.")
}

func PreferencesForgotten() string {
	return say("PreferencesForgotten", "Forgotten. The next window opens the way the first one did.")
}
