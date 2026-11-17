package preset

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

const (
	pictureSizesID = "picture-sizes"

	// pictureSizesQuestion is announced by the preset AND written into the
	// header of an ejected recipe, so it is named once.
	pictureSizesQuestion = "Does my system take pictures at the sizes screens and platforms use?"

	// everyGroup is what the groups parameter says when it means all of them -
	// the word the formats of empty-and-minimal use for the same thing.
	everyGroup = "all"

	// defaultPictureFormat is JPEG because it is what a camera, a phone and
	// most of the web hand an upload form, and because its smallest pictures
	// stay small: the whole set at its defaults is about four megabytes, where
	// WebP, BMP and TIFF write their smallest picture uncompressed and come to
	// a hundred (measured 2026-11-17, docs/SIZE-FIXES-2026-11-16.md section 3.7).
	defaultPictureFormat = "jpg"

	screensGroup = "screens"
	avatarsGroup = "avatars"
	socialGroup  = "social"

	// dimensionsReason is the rule in play for every file of the set - the
	// size of the picture - rather than a verdict, the way a boundary set
	// expects the file under its limit to be accepted with size_limit.
	dimensionsReason = "dimensions_limit"
)

// pictureGroups is the order a set lays its groups out in, whatever order
// they were typed in, so two people asking for the same groups get the same
// recipe - the rule chosenFormats keeps for formats.
var pictureGroups = []string{screensGroup, avatarsGroup, socialGroup}

func init() {
	content := placeholder.Content()
	Register(Preset{
		ID:       pictureSizesID,
		Title:    "Picture sizes",
		Question: pictureSizesQuestion,

		Parameters: []format.Property{
			{
				Name: "groups", Kind: format.PropertyText,
				Shape:   "group names separated by commas, or all",
				Default: everyGroup,
				Detail: "Which sizes the set holds: screens for the sizes of screens, avatars for profile pictures " +
					"and social for covers, banners, posts and thumbnails. Leave it at all for every one of them.",
			},
			content,
		},
		Reads:        []string{"format"},
		ReadDefaults: map[string]string{"format": defaultPictureFormat},

		Requires: []string{"MVP"},
		Catches: []string{
			"a limit on the width or the height set with one shape in mind that turns away another, such as a profile background 1584 px wide",
			"a thumbnail cut or squashed because the code expects a landscape picture and a portrait one arrives",
			"a picture with an odd height, such as 1200 x 627, refused by a step that only takes even sizes",
			"a 4K picture that runs out of time or memory on its way to a thumbnail, although the file itself is small",
		},

		Says:   saidAboutThePictureSizes,
		Expand: expandPictureSizes,
	})
}

// pictureSize is one use of one size: a screen, or a place on a platform.
type pictureSize struct {
	group         string
	width, height int
	// use names what this size is, worded to follow "the size of" and without a
	// comma, so that the uses of one size can be joined into one sentence.
	use string
}

// pictureSizes is every use the set knows, by group, in the order the files
// are laid out. A size that has several uses is listed once for each, beside
// the page it comes from, and becomes one file.
//
// Every number is from the page written beside it, read on 2026-11-17. A
// platform changes these when it redesigns a page, so the date belongs to the
// number: YouTube asked for a thumbnail of 1280 x 720 before it asked for
// 3840 x 2160. docs/SIZE-FIXES-2026-11-16.md section 7 has what each page says
// and the sizes left out.
var pictureSizes = []pictureSize{
	// ITU-R BT.1847-1 (2015), the 1 280 x 720 image format.
	{screensGroup, 1280, 720, "an HD screen"},
	// ITU-R BT.709-6 (2015): 1 920 samples a line and 1 080 lines.
	{screensGroup, 1920, 1080, "a Full HD screen"},
	// No television standard says 2560 x 1440. It is the sixth most common
	// computer screen, 3.79 per cent in October 2026:
	// gs.statcounter.com/screen-resolution-stats/desktop/worldwide
	{screensGroup, 2560, 1440, "a QHD screen"},
	// ITU-R BT.2020-2 (2015), the 3 840 x 2 160 system.
	{screensGroup, 3840, 2160, "a 4K Ultra HD screen"},

	// support.google.com/youtube/answer/10456525: a profile picture "renders
	// at 98 X 98 px".
	{avatarsGroup, 98, 98, "a YouTube channel picture as YouTube shows it"},
	// facebook.com/help/125379114252045: a page's profile picture shows at
	// 176 x 176 on a computer and 196 x 196 on a smartphone, and is best at
	// 320 x 320.
	{avatarsGroup, 176, 176, "a Facebook page picture as a computer shows it"},
	{avatarsGroup, 196, 196, "a Facebook page picture as a phone shows it"},
	{avatarsGroup, 320, 320, "a Facebook page picture as Facebook asks for it"},
	// help.x.com/en/managing-your-account/common-issues-when-uploading-profile-photo:
	// a profile photo of 400 x 400.
	{avatarsGroup, 400, 400, "an X profile photo"},
	// linkedin.com/help/linkedin/answer/a549049: a profile photo from 400 x 400
	// up to 7680 x 4320. a563309: a page logo of 400 x 400.
	{avatarsGroup, 400, 400, "the smallest LinkedIn profile photo"},
	{avatarsGroup, 400, 400, "a LinkedIn page logo"},

	// facebook.com/help/125379114252045: a page's cover of 851 x 315.
	{socialGroup, 851, 315, "a Facebook page cover"},
	// help.x.com, the page above: a header of 1500 x 500.
	{socialGroup, 1500, 500, "an X header"},
	// linkedin.com/help/linkedin/answer/a568217: a background of 1584 x 396.
	{socialGroup, 1584, 396, "a LinkedIn profile background"},
	// support.google.com/youtube/answer/10456525: a banner of at least
	// 2048 x 1152, and 2560 x 1440 recommended.
	{socialGroup, 2048, 1152, "the smallest YouTube channel banner"},
	{socialGroup, 2560, 1440, "a YouTube channel banner"},
	// developers.facebook.com/docs/sharing/webmasters/images: at least
	// 1200 x 630 for the picture of a shared link.
	{socialGroup, 1200, 630, "the picture of a link shared on Facebook"},
	// linkedin.com/help/linkedin/answer/a563309: 1.91:1, 1200 x 627.
	{socialGroup, 1200, 627, "a picture in a LinkedIn post"},
	// help.instagram.com/1631821640426723: 1080 wide and from 566 to 1440
	// high, 1.91:1 to 3:4.
	{socialGroup, 1080, 1080, "a square Instagram post"},
	{socialGroup, 1080, 566, "the widest Instagram post"},
	{socialGroup, 1080, 1440, "the tallest Instagram post"},
	// help.pinterest.com/en/business/article/pinterest-product-specs: 2:3 at
	// 1000 x 1500, and 1080 x 1920 to fill the screen.
	{socialGroup, 1000, 1500, "a Pinterest pin"},
	{socialGroup, 1080, 1920, "a Pinterest pin that fills the screen"},
	// support.google.com/youtube/answer/72431: 3840 x 2160 for videos and
	// 2160 x 3840 for Shorts.
	{socialGroup, 3840, 2160, "a YouTube video thumbnail"},
	{socialGroup, 2160, 3840, "a YouTube Shorts thumbnail"},
}

// groupsList is how the groups parameter is written.
var groupsList = commaList{
	preset:    pictureSizesID,
	param:     "groups",
	empty:     core.Says("preset.NoGroupsWereGivenSoThere", "no groups were given, so there is no size to make a picture of"),
	check:     checkPictureGroup,
	keep:      lower,
	duplicate: repeatedGroup,
}

func repeatedGroup(first string) core.Said {
	return core.Says("preset.ItIsTheSameGroupAs", "it is the same group as %q. Name every group once", core.A("First", first))
}

// checkPictureGroup lets the keyword through and refuses a name that is not
// a group, naming the ones there are.
func checkPictureGroup(item string) core.Said {
	if strings.EqualFold(item, everyGroup) || slices.Contains(pictureGroups, strings.ToLower(item)) {
		return core.Said{}
	}
	return core.Says("preset.ThereIsNoGroupCalledThat", "there is no group called that. The groups are: %s", core.A("Groups", strings.Join(pictureGroups, ", ")))
}

// chosenGroups is the groups a set covers, in the set's own order.
func chosenGroups(raw string) ([]string, error) {
	names, err := groupsList.parse(raw)
	if err != nil {
		return nil, err
	}
	if len(names) == 1 && names[0] == everyGroup {
		return pictureGroups, nil
	}
	if slices.Contains(names, everyGroup) {
		return nil, groupsList.refuse(everyGroup, core.Says("preset.AlreadyMeansEveryGroupSoIt", "%q already means every group, so it cannot stand beside a named one. Write it on its own, or name only the groups you want", core.A("EveryGroup", everyGroup)))
	}
	var out []string
	for _, g := range pictureGroups {
		if slices.Contains(names, g) {
			out = append(out, g)
		}
	}
	return out, nil
}

// pictureFile is one size of the set with every use it has in the groups
// chosen, under the first of those groups that names it.
type pictureFile struct {
	group         string
	width, height int
	uses          []string
}

func (f pictureFile) id() string { return fmt.Sprintf("%dx%d", f.width, f.height) }

// props are the settings that make the picture this size, and content only
// when it is the placeholder - the test pattern is every format's default and
// a recipe states nothing it does not need to.
func (f pictureFile) props(content string) map[string]string {
	props := map[string]string{
		"width":  strconv.Itoa(f.width),
		"height": strconv.Itoa(f.height),
	}
	if content == placeholder.ContentPlaceholder {
		props[placeholder.SettingContent] = content
	}
	return props
}

// filesIn merges the uses of the chosen groups into one file per size.
//
// The id of a file is its size and nothing else, so a size has the same seed
// and the same bytes whichever groups were chosen, and a set of avatars is the
// same files as the avatars of a set of everything.
func filesIn(groups []string) []pictureFile {
	var files []pictureFile
	at := map[string]int{}
	for _, s := range usesIn(groups) {
		f := pictureFile{group: s.group, width: s.width, height: s.height, uses: []string{s.use}}
		if i, seen := at[f.id()]; seen {
			files[i].uses = append(files[i].uses, s.use)
			continue
		}
		at[f.id()] = len(files)
		files = append(files, f)
	}
	return files
}

// usesIn is every use of the chosen groups, group by group in the set's order.
func usesIn(groups []string) []pictureSize {
	var out []pictureSize
	for _, g := range groups {
		out = append(out, slices.DeleteFunc(slices.Clone(pictureSizes), func(s pictureSize) bool { return s.group != g })...)
	}
	return out
}

// pictureSet is a set laid out for one format: the files it can draw and the
// sizes it cannot, with the format's own refusal of the first of those.
type pictureSet struct {
	desc    format.Descriptor
	content string
	files   []pictureFile
	left    []pictureFile
	// why is the refusal without the format in front of it, because every
	// sentence it goes into has already named the format.
	why any
}

// withoutTheFormat is a refusal of a setting as it reads after the format has
// been named, or the refusal itself when it is of another kind.
func withoutTheFormat(err error) any {
	var refused *format.PropertyValueError
	if errors.As(err, &refused) {
		return refused.Bare()
	}
	return err
}

// layOutPictures is the set the values ask for, or the refusal of them.
//
// A size the format cannot draw is left out before anything is planned, asked
// of the format's declaration - which is cheap, where planning a picture
// encodes it. An icon is at most 256 pixels a side, so a set of icons is three
// of the avatars, and the set says so rather than shrinking in silence (PR7).
func layOutPictures(args Args) (pictureSet, error) {
	groups, err := chosenGroups(args["groups"])
	if err != nil {
		return pictureSet{}, err
	}
	desc, err := pictureFormat(args["format"])
	if err != nil {
		return pictureSet{}, err
	}
	content := args[placeholder.SettingContent]
	if err := drawsContent(desc, content); err != nil {
		return pictureSet{}, err
	}
	set := pictureSet{desc: desc, content: content}
	for _, f := range filesIn(groups) {
		err := desc.CheckProperties(f.props(content))
		if err == nil {
			set.files = append(set.files, f)
			continue
		}
		if set.why == nil {
			set.why = withoutTheFormat(err)
		}
		set.left = append(set.left, f)
	}
	if len(set.files) == 0 {
		return pictureSet{}, &format.PropertyValueError{
			Format: pictureSizesID, Key: "groups", Value: strings.Join(groups, ","),
			Reason: core.Says("preset.NoSizeFitsTheFormat", "%s cannot draw any size of these groups. Its refusal of the first: %s", core.A("Format", desc.ID), core.A("Cause", set.why)),
			Remedy: core.Says("preset.NoSizeFitsTheFormatFix", "Choose other groups, or another picture format"),
		}
	}
	return set, nil
}

// pictureFormat is the format the set is written in, which has to be a
// picture - the rule the engine takes smallest by, so the preset never lays
// out a set the engine refuses.
func pictureFormat(id string) (format.Descriptor, error) {
	if id == "" {
		id = defaultPictureFormat
	}
	desc, err := format.Get(id)
	if err != nil {
		return format.Descriptor{}, err
	}
	if !desc.IsPicture() {
		return format.Descriptor{}, &format.PropertyValueError{
			Format: pictureSizesID, Key: "format", Value: id,
			Reason: core.Says("preset.NotAPictureFormat", "%s files are not pictures, and every file of this set is one. The picture formats of this build: %s",
				core.A("Format", desc.ID), core.A("Formats", strings.Join(pictureFormatIDs(), ", "))),
			Remedy: core.Says("preset.NotAPictureFormatFix", "Choose one of the picture formats"),
		}
	}
	return desc, nil
}

// pictureFormatIDs is every picture format of this build, asked of the
// registry rather than listed here.
func pictureFormatIDs() []string {
	var out []string
	for _, d := range format.All() {
		if d.IsPicture() {
			out = append(out, d.ID)
		}
	}
	return out
}

// drawsContent refuses the placeholder for a format that does not declare
// one. The test pattern is what every picture format draws when nothing is
// said, so it needs no declaration.
func drawsContent(desc format.Descriptor, content string) error {
	if content != placeholder.ContentPlaceholder || declaresPlaceholder(desc) {
		return nil
	}
	var can []string
	for _, d := range format.All() {
		if declaresPlaceholder(d) {
			can = append(can, d.ID)
		}
	}
	return &format.PropertyValueError{
		Format: pictureSizesID, Key: placeholder.SettingContent, Value: content,
		Reason: core.Says("preset.CannotDrawAPlaceholder", "%s cannot draw a placeholder. The formats that can: %s",
			core.A("Format", desc.ID), core.A("Formats", strings.Join(can, ", "))),
		Remedy: core.Says("preset.CannotDrawAPlaceholderFix", "Choose one of those formats, or leave the content as a test pattern"),
	}
}

// declaresPlaceholder says the format offers the placeholder among the
// values of its content setting.
func declaresPlaceholder(d format.Descriptor) bool {
	for _, p := range d.Properties {
		if p.Name == placeholder.SettingContent && slices.Contains(p.Choices, placeholder.ContentPlaceholder) {
			return true
		}
	}
	return false
}

// draft is one file of the set as a target of the recipe.
func (s pictureSet) draft(f pictureFile) recipe.TargetDraft {
	return recipe.TargetDraft{
		ID: f.id(), Format: s.desc.ID, Count: "1",
		Size: core.SizeSmallest, Name: f.id() + s.desc.Extension, Group: f.group,
		Expected: "accept", ExpectedReason: dimensionsReason,
		Purpose: fmt.Sprintf("A picture of %d x %d written as %s, the size of %s. "+
			"Your system should take it. If it does not, check whether it limits the width, "+
			"the height or the size of a file below this picture.",
			f.width, f.height, s.desc.Name, joinUses(f.uses)),
		Properties: f.props(s.content),
	}
}

// joinUses says the uses of one size as one list: a, b and c.
func joinUses(uses []string) string {
	if len(uses) == 1 {
		return uses[0]
	}
	return strings.Join(uses[:len(uses)-1], ", ") + " and " + uses[len(uses)-1]
}

// saidAboutThePictureSizes says which sizes a format left out, and why.
func saidAboutThePictureSizes(args Args) []core.Said {
	set, err := layOutPictures(args)
	if err != nil || len(set.left) == 0 {
		// A refusal is Expand's to make, with a message that names the value.
		return nil
	}
	sizes := make([]string, 0, len(set.left))
	for _, f := range set.left {
		sizes = append(sizes, f.id())
	}
	return []core.Said{core.Says("preset.SizesLeftOut",
		"%s cannot draw %d of the sizes. Its refusal of the first: %s. The set leaves them out: %s",
		core.A("Format", set.desc.ID), core.A("Count", len(set.left)),
		core.A("Cause", set.why), core.A("Sizes", strings.Join(sizes, ", ")))}
}

func expandPictureSizes(args Args) ([]byte, error) {
	set, err := layOutPictures(args)
	if err != nil {
		return nil, err
	}
	targets := make([]recipe.TargetDraft, 0, len(set.files))
	for _, f := range set.files {
		targets = append(targets, set.draft(f))
	}
	return plan{preset: pictureSizesID, question: pictureSizesQuestion, targets: targets}.source()
}
