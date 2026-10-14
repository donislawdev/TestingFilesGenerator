// Package picture is what the formats that carry pictures of another format
// share: the three settings, what each of them takes, how they are read, which
// formats are pictures, and what reaches the manifest.
//
// A mail message carries its pictures beside the HTML that shows them, and a
// page carries them inside itself (docs/HTML-3-2026-10-14.md). The owner's
// decision of 2026-10-14 is that the settings are one set of names with one
// meaning in both, so a test reads the same keys from either - and that is held
// here by there being one declaration rather than two kept alike.
package picture

import (
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// Setting names, public under rule 10. Inline is what MIME calls a picture
// the text shows, and what a page calls a picture written into it.
const (
	Count  = "inline"
	Format = "inline_format"
	Size   = "inline_size"
)

// DefaultFormat is what a picture is when nothing says - the format a mail
// program puts a pasted picture in, and a page saved as one file most often
// carries.
const DefaultFormat = "png"

// Settings is what the three settings ask for.
type Settings struct {
	Count  int
	Format string
	Size   int64
}

// Details are the three sentences a format says about its pictures - what the
// settings do in that format and what readers make of it, which differ between
// a message and a page while what each setting takes does not.
type Details struct {
	Count, Format, Size string
}

// Properties declares the three settings in a group, with the format's own
// sentences. As many pictures as an archive holds files, and the size of a
// file inside one when nothing says.
func Properties(group string, d Details) []format.Property {
	return []format.Property{
		{
			Name: Count, Kind: format.PropertyInt,
			Min: 0, Max: archive.MaxMembers,
			Default: "0",
			Detail:  d.Count,
			Group:   group,
		},
		{
			Name: Format, Kind: format.PropertyText,
			Shape:   "the id of a picture format, as tfg formats lists them",
			Default: DefaultFormat,
			Detail:  d.Format,
			Group:   group,
		},
		{
			Name: Size, Kind: format.PropertySize,
			Default: archive.DefaultSizeText,
			Detail:  d.Size,
			Group:   group,
		},
	}
}

// Parse is the three settings as numbers, from values already checked against
// their declarations and with the defaults filled in. A value that does not
// parse here passed a check that should have refused it, which is a defect
// rather than a mistake in the recipe.
//
// Whether the format is a picture is asked by Of, in Plan, where the registry
// is complete - this also runs while the formats are still registering, to
// work out the minimum.
func Parse(values map[string]string) (Settings, error) {
	n, err := strconv.Atoi(values[Count])
	if err != nil {
		return Settings{}, core.Defect(err)
	}
	size, err := core.ParseSize(values[Size])
	if err != nil {
		return Settings{}, core.Defect(err)
	}
	return Settings{Count: n, Format: values[Format], Size: size}, nil
}

// Read is the three settings of one format's request: each one checked against
// its declaration and its default when it is left out. owner is the format
// that refuses a value.
func Read(owner string, props map[string]string) (Settings, error) {
	values := map[string]string{}
	for _, p := range Properties("", Details{}) {
		v, ok := props[p.Name]
		if !ok || v == "" {
			v = p.Default
		} else if bad := p.Allows(v); !bad.IsZero() {
			return Settings{}, &format.PropertyValueError{Format: owner, Key: p.Name, Value: v, Reason: bad, Remedy: p.Instead()}
		}
		values[p.Name] = v
	}
	return Parse(values)
}

// Of is the descriptor of a format whose files are pictures. ok is false for a
// format of this build that is not one - the refusal of that is the caller's,
// in its own words - and err is the registry's refusal of an id it does not
// know.
func Of(id string) (desc format.Descriptor, ok bool, err error) {
	desc, err = format.Get(id)
	if err != nil {
		return format.Descriptor{}, false, err
	}
	return desc, strings.HasPrefix(desc.MediaType, "image/"), nil
}

// Formats are the formats of this build whose files are pictures, from the
// media type each declares rather than from a list kept here.
func Formats() []string {
	var out []string
	for _, d := range format.All() {
		if strings.HasPrefix(d.MediaType, "image/") {
			out = append(out, d.ID)
		}
	}
	return out
}

// Describe adds the pictures to what reaches the manifest - always the three
// settings, so a test reads the same keys whether there are any pictures or
// not, and how many bytes they are as files and as they travel.
func Describe(props map[string]any, s Settings, count int, raw, encoded int64) {
	props[Count] = count
	props[Format] = s.Format
	props[Size] = s.Size
	props["inline_bytes"] = raw
	props["inline_encoded_bytes"] = encoded
}
