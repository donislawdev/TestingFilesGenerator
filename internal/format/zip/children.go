package zip

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// What the archive holds, worked out before a byte is written.
//
// Split out of zip.go so that file stays under the crowding cap this project
// ratchets downwards. It is one subject: turning the groups a recipe declares
// into the children this archive will carry, each with its own derived seed.

// planChildren plans every file the archive will hold.
//
// The children are real files of another format, each valid on its own. The
// registry sits on the same layer as this package, so reaching for it needs
// nothing from the engine.
//
// Members are numbered across the whole archive rather than per group, so the
// seed of a member does not move when a group above it changes count. That is
// untouchable rule 2 applied one level down.
func planChildren(r format.Request, groups []format.Content, layout archive.Layout) ([]child, error) {
	var out []child
	index := 0
	// Numbering runs per format rather than per group, so two groups of the
	// same format do not both start at 0001 and collide inside the archive.
	numbered := map[string]int{}
	for _, g := range groups {
		desc, err := format.Get(g.Format)
		if err != nil {
			return nil, err
		}
		for i := 0; i < g.Count; i++ {
			childSeed := core.FileSeed(r.Seed, index)
			cp, err := desc.Generator.Plan(format.Request{
				Bytes: g.Bytes,
				Seed:  childSeed,
				Label: r.Label,
			})
			if err != nil {
				return nil, fmt.Errorf("zip: the %s file inside cannot be made: %w", g.Format, err)
			}
			numbered[g.Format]++
			out = append(out, child{
				name: layout.Path(fmt.Sprintf("%s_%04d%s", g.Format, numbered[g.Format], desc.Extension)),
				desc: desc,
				plan: cp,
			})
			index++
		}
	}
	return out, nil
}
