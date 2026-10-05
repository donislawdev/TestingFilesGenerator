package checksum

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The checksum file: the format GNU coreutils writes and reads, and the one
// "--tag" writes, which is the one BSD and perl's shasum write. Every rule
// below was measured on 2026-09-30 against coreutils 8.32, coreutils 9.7 and
// shasum rather than taken from memory - the memory was wrong about a byte
// order mark and about blank lines (docs/NARZEDZIA-SUMY-2026-09-29.md §15.2).

// sumsAlgorithms are the algorithms a checksum file is written with. crc32
// is not one: nothing reads a CRC32SUMS, so writing one would promise a check
// nobody can run.
var sumsAlgorithms = []string{"md5", "sha1", "sha256", "sha512"}

// sumsName is the name coreutils and release pages give the checksum file of
// an algorithm: SHA256SUMS for sha256.
func sumsName(algorithm string) string { return strings.ToUpper(algorithm) + "SUMS" }

// escaper is how coreutils 9 writes a name holding a backslash, a line break
// or a carriage return: each as two characters, on a line that starts with a
// backslash to say the name is written that way.
var escaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`)

// SumsLine is one entry, the way sha256sum writes it: the checksum, two
// spaces and the path, slash separated, on a line of its own.
//
// A carriage return is escaped the way coreutils 9 does it, and that has a
// cost which is named rather than hidden: coreutils 8.32 writes one as it is
// and refuses the escaped line, and perl's shasum does not read it either.
// The newest coreutils reads both ways and the other two read neither the same
// way, so the choice is the one the tool people have today makes. A carriage
// return in a file name is rare everywhere except in a folder of test files,
// which is what this program makes.
func SumsLine(sum, name string) string {
	escaped := escaper.Replace(name)
	if escaped == name {
		return sum + "  " + name + "\n"
	}
	return `\` + sum + "  " + escaped + "\n"
}

// The bounds on a checksum file this reads. It comes from somewhere else, so
// how much of this program's memory and time it may take is decided before a
// byte of it is read.
const (
	// sumsMostBytes is the largest checksum file read. A line of a sha256
	// and a path of sixty characters is about 130 bytes, so this is about half
	// a million files. How long a file this size takes to read is not measured.
	sumsMostBytes = 64 << 20
	// lineMostBytes is the longest line kept. The longest path Windows allows
	// is 32767 UTF-16 units, at most 98 KB of UTF-8, and escaping can double
	// it - so a longer line is not a line anybody wrote about a real file.
	lineMostBytes = 256 << 10
)

// Listed is one checksum line of a checksum file.
type Listed struct {
	// Line is where it stands, counted from one.
	Line      int
	Algorithm string
	// Sum is in lower case, whichever case the file used.
	Sum string
	// Path is as the file writes it, with the escaping undone.
	Path string
}

// LineRange is lines that follow one another, both ends counted from one.
type LineRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Sums is what a checksum file came to.
type Sums struct {
	Listed []Listed
	// NotSums are the lines that are not a checksum line - a blank line, a
	// comment, the armour of a signature, a checksum a digit short. Named
	// rather than failed, the owner's decision of 2026-09-30: sha256sum -c
	// passes over them, and a signed checksum file is full of them.
	//
	// As ranges, so lines one after another are one entry. A number a line
	// made a checksum file of 64 MiB of line breaks into 67 million numbers
	// and over two gigabytes before anything was shown (measured on
	// 2026-10-05, the review of #158). Now the most there can be is one more
	// than the checksum lines between them, which are kept anyway.
	NotSums []LineRange
	// Unknown are lines of an algorithm this tool does not work out, as
	// "line N: NAME" - a sha224, a BLAKE2b.
	Unknown []string
	// UnknownWords is Unknown as sentences, one for each, for a window.
	UnknownWords []core.Said `json:"-"`
}

// ParseSums reads a checksum file, line by line, never keeping more of a line
// than lineMostBytes.
func ParseSums(r io.Reader) (Sums, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	var out Sums
	for number := 1; ; number++ {
		line, tooLong, last, err := readLine(br)
		if err != nil {
			return Sums{}, err
		}
		if last {
			return out, nil
		}
		if number == 1 {
			// A byte order mark, which sha256sum refuses. Taken, because a
			// checksum line cannot start with one and an editor may add it.
			line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
		}
		out.add(number, line, tooLong)
	}
}

// add files one line under what it turned out to be.
func (s *Sums) add(number int, line []byte, tooLong bool) {
	if tooLong {
		s.notSum(number)
		return
	}
	entry, unknown, ok := readEntry(string(line))
	switch {
	case unknown != "":
		line := core.Says("checksum.UnknownLine", "line %d: %s", core.A("Line", number), core.A("Name", unknown))
		s.Unknown = append(s.Unknown, line.String())
		s.UnknownWords = append(s.UnknownWords, line)
	case ok:
		entry.Line = number
		s.Listed = append(s.Listed, entry)
	default:
		s.notSum(number)
	}
}

// notSum files a line that is not a checksum line, as the end of the range
// before it when it follows that range.
func (s *Sums) notSum(number int) {
	if last := len(s.NotSums) - 1; last >= 0 && s.NotSums[last].To == number-1 {
		s.NotSums[last].To = number
		return
	}
	s.NotSums = append(s.NotSums, LineRange{From: number, To: number})
}

// readLine is the next line without its line break. tooLong says it was longer
// than a line may be, and then the rest of it was read past rather than kept.
// last says there was no line left.
func readLine(br *bufio.Reader) (line []byte, tooLong, last bool, err error) {
	for {
		piece, readErr := br.ReadSlice('\n')
		line, tooLong = kept(line, piece, tooLong)
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, false, false, readErr
		}
		// A file that ends in a line break has nothing after it, and that is
		// the end rather than an empty last line.
		last = readErr != nil && len(line) == 0 && !tooLong
		return withoutLineBreak(line), tooLong, last, nil
	}
}

// kept is a line with one more piece of it read - or nothing, once the line
// is longer than a line may be, and for the rest of it.
func kept(line, piece []byte, tooLong bool) ([]byte, bool) {
	if tooLong || len(line)+len(piece) > lineMostBytes {
		return nil, true
	}
	return append(line, piece...), false
}

// withoutLineBreak takes the line break off the end of a line, and a carriage
// return before it - a file written on Windows, which coreutils 9 reads.
func withoutLineBreak(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte("\n"))
	return bytes.TrimSuffix(line, []byte("\r"))
}

// readEntry reads one line as a checksum line: GNU, "hex  path", or tagged,
// "SHA256 (path) = hex". unknown names an algorithm the line is written in
// that this tool does not work out.
func readEntry(text string) (entry Listed, unknown string, ok bool) {
	escaped := strings.HasPrefix(text, `\`)
	if escaped {
		text = text[1:]
	}
	algorithm, sum, path, unknown, ok := gnuEntry(text)
	if !ok && unknown == "" {
		algorithm, sum, path, unknown, ok = taggedEntry(text)
	}
	if !ok {
		return Listed{}, unknown, false
	}
	if escaped {
		if path, ok = unescaped(path); !ok {
			return Listed{}, "", false
		}
	}
	return Listed{Algorithm: algorithm, Sum: strings.ToLower(sum), Path: path}, "", true
}

// gnuEntry is "hex  path": the checksum, a space, a space or a star for the
// mode, and the path. One space alone is taken too - both coreutils do.
func gnuEntry(text string) (algorithm, sum, path, unknown string, ok bool) {
	digits := hexPrefix(text)
	if digits == 0 || digits == len(text) || text[digits] != ' ' {
		return "", "", "", "", false
	}
	path = text[digits+1:]
	if strings.HasPrefix(path, " ") || strings.HasPrefix(path, "*") {
		path = path[1:]
	}
	if path == "" {
		return "", "", "", "", false
	}
	sum = text[:digits]
	for _, name := range sumsAlgorithms {
		if find(name).digits == digits {
			return name, sum, path, "", true
		}
	}
	// A length another algorithm has is named for it. Any other length is a
	// checksum line that went wrong - a digit lost - and that is a line that
	// is not a checksum, which sha256sum calls "improperly formatted".
	if other, known := elsewhere[digits]; known {
		return "", "", "", other, false
	}
	return "", "", "", "", false
}

// elsewhere are the lengths of the algorithms coreutils has and this tool
// does not, so a line written with one is named for what it is.
var elsewhere = map[int]string{56: "sha224", 96: "sha384"}

// taggedEntry is "SHA256 (path) = hex". The path may hold ") = " itself, so
// the checksum is what follows the last one.
func taggedEntry(text string) (algorithm, sum, path, unknown string, ok bool) {
	open := strings.Index(text, " (")
	closing := strings.LastIndex(text, ") = ")
	if open <= 0 || closing < open+2 {
		return "", "", "", "", false
	}
	tag, path, sum := text[:open], text[open+2:closing], text[closing+4:]
	if path == "" || hexPrefix(sum) != len(sum) || sum == "" || !isTag(tag) {
		return "", "", "", "", false
	}
	algorithm = strings.ToLower(tag)
	a := find(algorithm)
	if a == nil || algorithm == "crc32" {
		return "", "", "", tag, false
	}
	if a.digits != len(sum) {
		return "", "", "", "", false
	}
	return algorithm, sum, path, "", true
}

// isTag is whether a word can be the name of an algorithm in a tagged line:
// letters, digits, and the dash and slash of names like SHA3-256 and
// SHA512/256.
func isTag(word string) bool {
	for _, r := range word {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '/') {
			return false
		}
	}
	return word != ""
}

// hexPrefix is how many characters at the start of a text are hexadecimal.
func hexPrefix(text string) int {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return i
		}
	}
	return len(text)
}

// unescaped undoes the escaping of a name: a backslash, a line break and a
// carriage return, each written as two characters. Anything else after a
// backslash is not a name coreutils wrote, and both versions refuse it.
func unescaped(name string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(name); i++ {
		if name[i] != '\\' {
			out.WriteByte(name[i])
			continue
		}
		if i+1 == len(name) {
			return "", false
		}
		i++
		switch name[i] {
		case '\\':
			out.WriteByte('\\')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		default:
			return "", false
		}
	}
	return out.String(), true
}

// readSums opens a checksum file without waiting on it, refuses one larger
// than a checksum file is allowed to be, and reads it. What was opened is
// handed back too, so the file can be told apart from the files it lists.
func readSums(path string) (Sums, os.FileInfo, error) {
	f, info, err := openRegular(path)
	if err != nil {
		return Sums{}, nil, err
	}
	defer func() { _ = f.Close() }()
	if info.Size() > sumsMostBytes {
		return Sums{}, nil, &SumsTooLargeError{Path: path, Bytes: info.Size()}
	}
	// Bounded again while reading, since a file can grow after it was asked.
	bounded := &io.LimitedReader{R: f, N: sumsMostBytes + 1}
	parsed, err := ParseSums(bounded)
	if err != nil {
		return Sums{}, nil, err
	}
	if bounded.N == 0 {
		return Sums{}, nil, &SumsTooLargeError{Path: path, Bytes: sumsMostBytes + 1}
	}
	if len(parsed.Listed) == 0 {
		return Sums{}, nil, &NoSumsError{Path: path}
	}
	return parsed, info, nil
}
