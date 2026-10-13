package text

import (
	"sort"
	"strings"
)

// SettingLabel is what the name above a declared setting reads.
//
// A format declares its settings under the key a recipe writes - width,
// bit_depth, entry_size - and until 2026-08-20 the window put that key on the
// screen as the label. Two naming systems in one visual style: the names this
// window writes are capitalised and spaced, and the ones coming out of the
// registry were not, so half the labels on the screen looked written and half
// looked leaked.
//
// It changes only what is drawn. The key is what a refusal is matched against
// and what goes into a recipe, and neither of those goes through here - see
// SettingKey for where the key stays visible.
//
// Looked up by the key since 2026-09-29, because another language cannot be
// spelled out of an English key. The English is still spelled out of it - the
// registry declares no label - except for the few keys respelled lists.
func SettingLabel(key string) string { return lookup(LabelKey(key), EnglishLabel(key)) }

// EnglishLabel is the English SettingLabel falls back to, spelled out of the
// key - bit_depth reads "Bit depth". Exported for the guard that writes the
// words of the registries out for a translator.
func EnglishLabel(key string) string {
	if label, ok := respelled[key]; ok {
		return label
	}
	if key == "" {
		return key
	}
	spaced := strings.ReplaceAll(key, "_", " ")
	return strings.ToUpper(spaced[:1]) + spaced[1:]
}

// respelled are the keys whose spelling out is a word somebody meeting the
// window for the first time does not know. Inline is what MIME calls a picture
// the text shows (the owner's decision of 2026-10-14,
// docs/EML-2-2026-10-13.md section 8). The key is still one hover away.
//
// Keyed by the recipe key rather than declared on a format's setting, because
// a label belongs to the key: every format, preset and tool declaring one key
// shares one Label entry in the catalogue.
var respelled = map[string]string{
	"inline":        "Pictures",
	"inline_format": "Picture format",
	"inline_size":   "Picture size",
}

// RespelledKeys are the keys EnglishLabel does not spell out of themselves,
// for the guard that asks each of them to still be a declared setting - an
// entry for a key nothing declares any more changes nothing and says nothing.
func RespelledKeys() []string {
	keys := make([]string, 0, len(respelled))
	for k := range respelled {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
