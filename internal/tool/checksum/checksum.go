// Package checksum is the tool that works out the checksums of a file and
// compares one with a checksum somebody was given.
//
// Five algorithms, in the order a person meets them: md5, sha1, sha256,
// sha512 and crc32. sha256 alone unless asked, because the others cost time
// nobody asked to spend - measured 2026-09-29 on 512 MiB in memory, five runs:
// sha256 2064 MB/s on its own, all five through one writer 305 MB/s
// (docs/NARZEDZIA-SUMY-2026-09-29.md section 2).
//
// md5 and sha1 are here because files are still published with them, not
// because they protect anything. Telling a download apart from the file its
// page describes is a comparison, and a comparison with a broken hash is
// still a comparison. The description of the setting says which to trust.
package checksum

import (
	"context"
	//nolint:gosec // md5 is offered to compare with checksums published as md5, and the setting says it proves nothing
	"crypto/md5"
	//nolint:gosec // sha1 for the same reason as md5, and said the same way
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"hash/crc32"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// ID is the name of the tool, after "tfg tool".
const ID = "checksum"

// The names of what the tool declares, used by the request and by both
// surfaces.
const (
	InputFile        = "file"
	SettingAlgorithm = "algorithm"
	SettingExpect    = "expected"
)

// algorithm is one checksum this tool works out.
type algorithm struct {
	name string
	// digits is how long its checksum is in hexadecimal. Different for all
	// five, which is what lets a pasted checksum say which one it is.
	digits int
	make   func() hash.Hash
}

// algorithms is every checksum this tool knows, in the order it prints them.
var algorithms = []algorithm{
	{name: "md5", digits: 32, make: md5.New},
	{name: "sha1", digits: 40, make: sha1.New},
	{name: "sha256", digits: 64, make: sha256.New},
	{name: "sha512", digits: 128, make: sha512.New},
	// IEEE, which is the one ZIP, gzip and PNG carry. Not the one cksum prints:
	// that is another polynomial with the length folded in, and the setting
	// says so, because the two look alike and never agree.
	{name: "crc32", digits: 8, make: func() hash.Hash { return crc32.NewIEEE() }},
}

// Names is the algorithms this tool knows, in the order it prints them.
func Names() []string {
	out := make([]string, 0, len(algorithms))
	for _, a := range algorithms {
		out = append(out, a.name)
	}
	return out
}

func init() {
	tool.Register(tool.Descriptor{
		ID:       ID,
		Question: "Is this file the one it claims to be?",
		Detail: "Works out the checksum of a file, and compares it with one you were given - " +
			"from a download page, a release note or a colleague.",
		Inputs: []tool.Input{{
			Name: InputFile, Kind: tool.File,
			Detail: "The file to work out the checksums of. It is only read.",
		}},
		Settings: []format.Property{
			{
				Name: SettingAlgorithm, Kind: format.PropertyText, Default: "sha256",
				Shape: "algorithm names separated by commas, or all",
				Detail: "Which checksums to work out: md5, sha1, sha256, sha512 or crc32. " +
					"Only sha256 and sha512 still show that nobody changed the file on purpose. " +
					"crc32 is the one ZIP and PNG use, not the one cksum prints.",
			},
			{
				Name: SettingExpect, Kind: format.PropertyText, Long: true,
				Shape: "a checksum in hexadecimal",
				Detail: "The checksum the file should have. The algorithm is told from its length, " +
					"and worked out even when it is not chosen above.",
			},
		},
		Columns: []string{"Algorithm", "Checksum"},
		Run:     run,
	})
}

// Checksums is what --json prints about one file.
type Checksums struct {
	File      string            `json:"file"`
	Bytes     int64             `json:"bytes"`
	Checksums map[string]string `json:"checksums"`
	Expected  *Expected         `json:"expected,omitempty"`
}

// Expected is the comparison, when a checksum was given.
type Expected struct {
	Algorithm string `json:"algorithm"`
	Checksum  string `json:"checksum"`
	Matches   bool   `json:"matches"`
}

// run is the tool, on a request tool.Start has checked and defaulted.
func run(ctx context.Context, in tool.Request, progress tool.Progress) (tool.Result, error) {
	chosen, err := choose(in.Values[SettingAlgorithm])
	if err != nil {
		return tool.Result{}, err
	}
	expected, err := readExpected(in.Values[SettingExpect])
	if err != nil {
		return tool.Result{}, err
	}
	if expected != nil && !chosen[expected.Algorithm] {
		// Worked out although not chosen. The row shows it, so nothing
		// happens that the person cannot see.
		chosen[expected.Algorithm] = true
	}

	path := in.Inputs[InputFile]
	var done int64
	digests, opened, err := digest(ctx, path, chosen, nil, func(read, size int64) {
		done += read
		progress(done, size)
	})
	if err != nil {
		return tool.Result{}, err
	}

	out := Checksums{File: path, Bytes: opened.Size(), Checksums: digests, Expected: expected}
	result := tool.Result{Data: &out}
	for _, a := range algorithms {
		if sum, ok := digests[a.name]; ok {
			result.Rows = append(result.Rows, []string{a.name, sum})
		}
	}
	if expected != nil {
		expected.Matches = digests[expected.Algorithm] == expected.Checksum
		result.Verdict = tool.Verdict{
			Outcome: tool.Mismatch, About: expected.Algorithm,
			Wanted: expected.Checksum, Got: digests[expected.Algorithm],
		}
		if expected.Matches {
			result.Verdict.Outcome = tool.Match
		}
	}
	return result, nil
}

// choose reads the algorithm setting into the set to work out.
//
// Spelled exactly, the way a closed set of choices is (O168): "SHA256" is
// refused with the list rather than understood, so what a recipe or a script
// writes means one thing everywhere.
func choose(raw string) (map[string]bool, error) {
	chosen := map[string]bool{}
	if strings.TrimSpace(raw) == "all" {
		for _, a := range algorithms {
			chosen[a.name] = true
		}
		return chosen, nil
	}
	for _, word := range strings.Split(raw, ",") {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if find(word) == nil {
			return nil, &UnknownAlgorithmError{Name: word}
		}
		chosen[word] = true
	}
	if len(chosen) == 0 {
		return nil, &UnknownAlgorithmError{Name: raw}
	}
	return chosen, nil
}

// find is the algorithm of a name, or nil.
func find(name string) *algorithm {
	for i := range algorithms {
		if algorithms[i].name == name {
			return &algorithms[i]
		}
	}
	return nil
}

// readExpected reads a pasted checksum, or nothing when none was given.
//
// Either case is taken, because the same checksum is published both ways and
// neither is a spelling of something else - PowerShell prints capitals and
// sha256sum does not. Spaces at the ends are what a copy picks up.
func readExpected(raw string) (*Expected, error) {
	sum := strings.ToLower(strings.TrimSpace(raw))
	if sum == "" {
		return nil, nil
	}
	if _, err := hex.DecodeString(sum); err != nil || len(sum)%2 != 0 {
		return nil, &ExpectedError{Given: raw}
	}
	for _, a := range algorithms {
		if len(sum) == a.digits {
			return &Expected{Algorithm: a.name, Checksum: sum}, nil
		}
	}
	return nil, &ExpectedError{Given: raw, Digits: len(sum)}
}
