package recipe

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading the contains list: what an archive holds.
//
// Split out of target.go on 2026-08-25, when that file went past three quarters
// of the ceiling after the format check moved in. The ceiling is a ratchet, so
// the answer is a split and never a higher number.

// contentGroups reads the contains list.
//
// Every problem is collected rather than returned on the first one, because a
// contains list written by hand usually has more than one thing wrong with it
// and fixing a recipe one error per run is how people stop using a tool.
func contentGroups(p *problems, where spot, raw []map[string]scalar) []Content {
	if len(raw) == 0 {
		// An empty list is not the same as no list. It says "an archive
		// holding nothing", which is a legitimate thing to test, and the size
		// then comes from the container overhead alone.
		return []Content{}
	}

	out := make([]Content, 0, len(raw))
	for i, item := range raw {
		at := where.entry("contains", i)
		g := Content{Count: 1}

		// Sorted, because Go randomises map order and these keys become
		// problems in the report. Unsorted, the same broken recipe would print
		// its problems in a different order on two runs.
		for _, key := range sortedKeys(item) {
			switch key {
			case "format", "count", "size":
			default:
				p.add(at.key, core.Says("recipe.HasTheKey", "%s has the key %q", core.A("At", at), core.A("Key", key)),
					core.Says("recipe.AContainsEntryDescribesFilesWith", "a contains entry describes files with a format, a count and a size, and anything else would be dropped on the way"),
					core.Says("recipe.RemoveTheKeyOrMoveIt", "remove the key, or move it to the target itself"))
			}
		}

		if v, ok := item["format"]; ok {
			if s, ok := v.value(); ok && s != "" {
				g.Format = s
			} else {
				p.add(at.of("format"), core.Says("recipe.HasAFormatThatIsNot", "%s has a format that is not a name", core.A("At", at)),
					core.Says("recipe.AFormatIsTheIdOf", "a format is the id of a format this build supports"),
					core.Says("recipe.UseFormatPdfOrRunTfg", "use format: pdf, or run tfg formats to see the list"))
			}
		} else {
			p.add(at.of("format"), core.Says("recipe.HasNoFormat", "%s has no format", core.A("At", at)),
				core.Says("recipe.AContainerHoldsRealFilesSo", "a container holds real files, so each group says which format its files are"),
				core.Says("recipe.AddFormatPdf", "add format: pdf"))
		}

		if v, ok := item["count"]; ok {
			n, ok := v.number()
			switch {
			case !ok:
				p.add(at.of("count"), core.Says("recipe.HasSettingThatIsNotA", "%s has {a} {setting} that is not a whole number", core.A("At", at)),
					core.Says("recipe.SettingIsHowManyFilesOf", "{a} {setting} is how many files of this group the container holds"),
					core.Says("recipe.UseCount50", "use count: 50"))
			case n < 0:
				p.add(at.of("count"), core.Says("recipe.HasANegativeSetting", "%s has a negative {setting}", core.A("At", at)),
					core.Says("recipe.AContainerCannotHoldFewerThan", "a container cannot hold fewer than no files"),
					core.Says("recipe.UseCount0ForAGroup", "use count: 0 for a group that contributes nothing, or drop the entry"))
			// The same ceiling the count of a target gets, and for the same
			// reason - judged before the list is built, because building it is
			// the failure. It was missing here until 2026-08-20, when CodeQL
			// pointed at the conversion below and the measurement behind it
			// turned out to be real rather than theoretical: one entry asking
			// for 2^63-1 files took this process to 12.9 GB and had to be
			// killed. The target path had been refusing that number since
			// 2026-08-03 and this path never learned it.
			case n > core.MaxFilesPerRun:
				p.add(at.of("count"), core.Says("recipe.AsksForFiles", "%s asks for %d files", core.A("At", at), core.A("Value", n)),
					core.SaidOf(core.ErrTooManyFiles),
					core.Says("recipe.UseSettingOfOrLessOr", "use {a} {setting} of %d or less, or split the container across several entries", core.A("MaxFilesPerRun", core.MaxFilesPerRun)))
			default:
				g.Count = int(n)
			}
		}

		if v, ok := item["size"]; ok {
			s, ok := v.value()
			if !ok {
				p.add(at.of("size"), core.Says("recipe.HasSettingThatIsNeitherText", "%s has {a} {setting} that is neither text nor a number", core.A("At", at)),
					core.Says("recipe.SettingIsWrittenAs2mbOr", "{a} {setting} is written as 2mb or as a plain byte count"),
					core.Says("recipe.UseSize2mbOrSize2097152", "use size: 2mb or size: 2097152"))
			} else if n, err := core.ParseSize(s); err != nil {
				p.add(at.of("size"), core.Says("recipe.WhereError", "%s: %v", core.A("Where", at), core.A("Error", err)),
					core.Says("recipe.UnitsCountIn1024sSo10mb", "units count in 1024s, so 10mb is 10485760 bytes"),
					core.Says("recipe.UseSettingSuchAs2mb512kb", "use {a} {setting} such as 2mb, 512kb or a plain byte count"))
			} else {
				g.Bytes = n
			}
		} else {
			// The same rule as AR10 one level down. Without it the size of the
			// container could not be worked out before writing, which is the
			// whole reason contains counts as a way of declaring a size.
			p.add(at.of("size"), core.Says("recipe.HasNoSetting", "%s has no {setting}", core.A("At", at)),
				core.Says("recipe.TheSettingOfTheContainerFollows", "the {setting} of the container follows from the {setting} of what it holds, so every group states one"),
				core.Says("recipe.AddSize2mb", "add size: 2mb"))
		}

		out = append(out, g)
	}
	return out
}
