// The files that arrive inside somebody else's module and end up in a binary
// here. Every one of them was found by asking the compiler, not by reading
// source: `go list -deps -f '{{.EmbedFiles}}'` answers for a whole package, and
// the first version of this work read one file instead and missed a font.
//
// Measured 2026-08-27 on the pinned modules: fyne.io/fyne/v2/theme embeds 104
// files, fyne.io/fyne/v2/lang 15 and fyne.io/fyne/v2/internal/painter/gl 14.
// The command line binary embeds nothing from anybody.

package legal

// The copyright lines below are read from the font files themselves - the name
// table, records 0 and 13 - and not from memory or from the module. Where the
// font and the module disagree, both are recorded.
//
// Two of them are written with "(c)" although the font writes the sign. Every
// byte this project prints to a terminal is ASCII, and a guard says so.
var assets = []Asset{
	{
		Name:      "Noto Sans",
		Package:   "fyne.io/fyne/v2/theme",
		Files:     []string{"font/NotoSans-Regular.ttf", "font/NotoSans-Bold.ttf", "font/NotoSans-Italic.ttf", "font/NotoSans-BoldItalic.ttf"},
		SPDX:      "OFL-1.1",
		Copyright: "Copyright 2015 Google Inc. All Rights Reserved.",
		Note:      "The text of the window, in four styles. The licence requires its notice to travel with the font, which is why it is quoted in THIRD-PARTY-NOTICES.md in full.",
	},
	{
		Name:      "Inter",
		Package:   "fyne.io/fyne/v2/theme",
		Files:     []string{"font/InterSymbols-Regular.ttf"},
		SPDX:      "OFL-1.1",
		Copyright: "(c) 2020 The Inter Project Authors",
		Note:      "Symbols only. Inter is a trademark of Rasmus Andersson. Neither licence file declares a Reserved Font Name, so the identifier is OFL-1.1 rather than OFL-1.1-RFN - checked in the files, not assumed.",
	},
	{
		Name:      "DejaVu Sans Mono for Powerline",
		Package:   "fyne.io/fyne/v2/theme",
		Files:     []string{"font/DejaVuSansMono-Powerline.ttf"},
		SPDX:      "Bitstream-Vera",
		Copyright: "Copyright (c) 2003 by Bitstream, Inc. All Rights Reserved. DejaVu changes are in public domain",
		Note:      "The monospaced face. Glyphs imported from the Arev fonts are (c) Tavmjong Bah.",
	},
	{
		Name:      "EmojiOne Color",
		Package:   "fyne.io/fyne/v2/theme",
		Files:     []string{"font/EmojiOneColor.otf"},
		SPDX:      "MIT",
		Copyright: "Copyright 2016 Adobe Systems Incorporated",
		Note:      "Colour emoji. The module ships this file under MIT and the font's own metadata states CC-BY-4.0 with attribution required. Both can be true at once - font software and artwork are not the same work - so the notices record what each source says instead of choosing one.",
	},
	{
		Name:      "Fyne icon set",
		Package:   "fyne.io/fyne/v2/theme",
		Files:     []string{"icons/"},
		SPDX:      "BSD-3-Clause",
		Copyright: "(C) 2018 Fyne.io developers (see AUTHORS)",
		Note:      "Ninety-six drawings and one image, the toolkit's own. The module ships no separate licence for them and says nothing about where they came from, so they travel under its licence. Some path data matches the Material Design set, which is Apache-2.0 - either answer is a licence this project already accepts.",
	},
	{
		Name:      "Fyne translations",
		Package:   "fyne.io/fyne/v2/lang",
		Files:     []string{"translations/"},
		SPDX:      "BSD-3-Clause",
		Copyright: "(C) 2018 Fyne.io developers (see AUTHORS)",
		Note:      "Fifteen languages of toolkit wording. Ours are elsewhere and are our own.",
	},
	{
		Name:      "Fyne shaders",
		Package:   "fyne.io/fyne/v2/internal/painter/gl",
		Files:     []string{"shaders/"},
		SPDX:      "BSD-3-Clause",
		Copyright: "(C) 2018 Fyne.io developers (see AUTHORS)",
		Note:      "The programs that draw every rectangle and line on the screen.",
	},
}
