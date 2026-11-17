package text

import "sort"

// EnglishChoice is the English ChoiceName falls back to: the value itself,
// except for the values named lists. Exported for the guard that writes the
// words of the registries out for a translator.
//
// A value is what a recipe, the command line and the manifest write. A list on
// the screen names it in the language of the window (the owner's decision of
// 2026-10-05, docs/OKNO-PO-POLSKU-2026-10-05.md section 0), and English is one
// of those languages: test beside placeholder does not say what a picture
// shows, and a menu of reasons a file is refused should not read size_zero.
func EnglishChoice(of, value string) string {
	if name, ok := named[of][value]; ok {
		return name
	}
	return value
}

// named are the values whose English name is not the value: a word that says
// too little on its own (test, sweep, both), and a value written the way a
// recipe writes a key, with its words joined by _ or -. Read off every list on
// 2026-11-17 (docs/SIZE-FIXES-2026-11-16.md section 6.3).
//
// A word that reads by itself - portrait, semicolon, none - stays as it is, and
// so does a token somebody looks up or types elsewhere: utf-8, a4, sha256, 644,
// quoted-printable.
//
// Keyed by the recipe key of the setting the list belongs to, as ChoiceKey is:
// every format declaring one key shares one name for each of its values.
var named = map[string]map[string]string{
	"body": {
		"alternative": "plain text and HTML",
		"any-value":   "every kind of value",
		"html":        "HTML",
		"plain":       "plain text",
	},
	"content": {
		"sweep": "rising tone",
		"test":  "test pattern",
		"tone":  "steady tone",
	},
	"entry_format": {
		"apache-combined": "Apache combined",
		"apache-common":   "Apache common",
		"json-lines":      "JSON Lines",
	},
	"expected.reason": {
		"content_malformed":  "malformed content",
		"count_limit":        "file count limit",
		"dimensions_limit":   "dimensions limit",
		"encoding_invalid":   "invalid encoding",
		"extension_rule":     "extension rule",
		"filename_invalid":   "invalid file name",
		"filename_too_long":  "file name too long",
		"filename_traversal": "path outside the folder",
		"malware_signature":  "malware signature",
		"mime_mismatch":      "type does not match the content",
		"nesting_depth":      "nesting depth",
		"size_limit":         "size limit",
		"size_zero":          "empty file",
	},
	"formatting": {
		"record-per-line": "record per line",
	},
	"id_encoding": {
		"upper-hex": "upper-case hex",
	},
	"key_case": {
		"camel": "camelCase",
		"snake": "snake_case",
	},
	"methods": {
		"get":  "GET only",
		"read": "GET and HEAD",
	},
	"metric_kind": {
		"exponential-histogram": "exponential histogram",
	},
	"rotate": {
		"0":   "0 degrees",
		"90":  "90 degrees",
		"180": "180 degrees",
		"270": "270 degrees",
	},
	"status_mix": {
		"client-errors": "client errors",
		"server-errors": "server errors",
	},
	"text_encoding": {
		"both":     "unicode and code page",
		"codepage": "code page",
	},
	"time_order": {
		"in-order":     "in order",
		"out-of-order": "out of order",
	},
}

// NamedChoice is one value named by hand, for the guards: Of is the recipe key
// of the setting whose list holds it.
type NamedChoice struct{ Of, Value string }

// NamedChoices are the values EnglishChoice does not say as themselves, sorted,
// for the guard that asks each of them to still be a value something declares
// - a name left behind by a renamed value changes nothing and says nothing.
func NamedChoices() []NamedChoice {
	var out []NamedChoice
	for of, values := range named {
		for value := range values {
			out = append(out, NamedChoice{of, value})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Of != out[j].Of {
			return out[i].Of < out[j].Of
		}
		return out[i].Value < out[j].Value
	})
	return out
}
