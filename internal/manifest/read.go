package manifest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// How a manifest is read, a piece at a time.
//
// Until 2026-11-04 a manifest was read whole into memory and handed to
// encoding/json, under a ceiling of sixteen megabytes on the document - about
// twenty thousand files. A larger run wrote a manifest verify and cleanup
// refused to read, so the files of a run of six hundred thousand were files
// this tool would never remove, and the document read that way peaked at about
// four times its length (docs/MANIFEST-STREAMING-2026-11-04.md, sections 2 and
// 8). Now it is read one value at a time - the tool, the run, each entry -
// under a ceiling on one value rather than on the document, and what verify and
// cleanup keep of an entry is three fields of it. Encode writes the same
// document the same way round (encode.go).
//
// One reader for every caller. Load keeps every entry and ReadClaims three
// fields of the claimed ones, and both are this reader underneath, so they
// cannot come to disagree about which documents they refuse or in which words.

// MaxValueBytes is the most of a manifest held at once while it is read: one
// value - one entry, the run, the tool - and never the document.
//
// The sixteen megabytes the whole document was allowed before, so a hostile
// document cannot make the reader hold more than a legal one could. An entry
// this tool writes is a kilobyte or two, and the part of it a recipe chooses
// comes from a recipe that has a ceiling of a megabyte, so nothing it writes
// comes near. Measured with tools/probes/manifestread: a document of one value
// of 200 MB, read without this, peaked at 723 MB and was accepted, and with it
// was refused after 16 MB in 0.02 s - while an ordinary document of 80 MB read
// in the same time with it and without it (section 8 of the analysis).
const MaxValueBytes = 16 << 20

// readBuffer is how much of the file is read at a time underneath the decoder.
const readBuffer = 64 << 10

// errValueTooLarge is the reader under the decoder declining to hand over more
// of one value. It comes back out of the decoder as it went in.
var errValueTooLarge = errors.New("a value runs past the ceiling on one value")

// bounded hands the decoder the document up to a limit that is moved before
// each value. The decoder keeps what it was handed and has not used yet, so it
// never holds more than the limit past the place the value began.
type bounded struct {
	r     io.Reader
	read  int64
	limit int64
}

func (b *bounded) Read(p []byte) (int, error) {
	room := b.limit - b.read
	if room <= 0 {
		return 0, errValueTooLarge
	}
	if int64(len(p)) > room {
		p = p[:room]
	}
	n, err := b.r.Read(p)
	b.read += int64(n)
	return n, err
}

// ValueTooLargeError is a manifest holding one value past MaxValueBytes.
type ValueTooLargeError struct {
	Path string
	// At is about how far into the document the value begins.
	At int64
}

func (e *ValueTooLargeError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *ValueTooLargeError) Said() core.Said {
	return core.Says("manifest.ValueTooLarge",
		"%s holds a single value of more than %d B, starting about %d B in, and this tool reads at most that much of a manifest at once. "+
			"An entry of a manifest this tool wrote takes a kilobyte or two, so check that this is the manifest the run wrote",
		core.A("Path", e.Path), core.A("Limit", int64(MaxValueBytes)), core.A("At", e.At))
}

// reading is one manifest on its way through the reader.
type reading struct {
	ctx  context.Context
	path string
	in   *bounded
	dec  *json.Decoder
	// escaped is the first entry that leaves the directory. Kept rather than
	// returned, because a fault of the document further on, or a version this
	// build does not read, is the answer that comes first - see settled.
	escaped error
}

// decode reads the manifest r holds into m, handing each entry to each as it
// arrives rather than keeping it. path is only what the refusals call it.
//
// each runs before the document has been found sound - an entry may arrive
// before the version does, and a fault may follow it - so a caller keeps what
// it is handed and acts on none of it unless decode returns no error.
//
// The keys of the document may come in any order and in any case, as
// encoding/json has always read them, and the entries are checked one at a
// time as they pass, so reading a manifest whose files come first holds no
// more than reading one whose files come last.
func decode(ctx context.Context, r io.Reader, path string, m *Manifest, each func(File)) error {
	in := &bounded{r: r}
	rd := &reading{ctx: ctx, path: path, in: in, dec: json.NewDecoder(in)}
	if err := rd.document(m, each); err != nil {
		return err
	}
	return settled(path, m, rd.escaped)
}

// next moves the limit to one value past where the decoder stands. Called
// before every question put to the decoder, More among them - a question that
// finds the limit spent would answer for the limit rather than the document.
func (rd *reading) next() {
	rd.in.limit = rd.dec.InputOffset() + MaxValueBytes
}

// token is the next token of the document, its error in the words of the
// manifest. An end of the input part way through is a document cut short.
func (rd *reading) token() (json.Token, error) {
	rd.next()
	tok, err := rd.dec.Token()
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return tok, rd.unreadable(err)
}

// value decodes the next value of the document into v.
func (rd *reading) value(v any) error {
	rd.next()
	return rd.unreadable(rd.dec.Decode(v))
}

// more says whether the object or list the decoder stands in goes on.
func (rd *reading) more() bool {
	rd.next()
	return rd.dec.More()
}

// unreadable is an error of the decoder in the words of a manifest. A fault of
// the document is a SchemaError, a value past the ceiling says so, and the
// rest - the disk failing, the context ending - is passed on as it came, so a
// failure to read is never called a fault of the document.
func (rd *reading) unreadable(err error) error {
	var syntax *json.SyntaxError
	var mistyped *json.UnmarshalTypeError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errValueTooLarge):
		return &ValueTooLargeError{Path: rd.path, At: rd.in.limit - MaxValueBytes}
	case errors.Is(err, io.ErrUnexpectedEOF):
		return &SchemaError{Path: rd.path, Detail: core.Says("manifest.ItEndsPartWay", "it ends part way through the document, so it was cut short")}
	case errors.As(err, &syntax), errors.As(err, &mistyped):
		return &SchemaError{Path: rd.path, Detail: core.SaidOf(err)}
	}
	return err
}

// document reads the one value a manifest is, and asks that nothing follows it.
func (rd *reading) document(m *Manifest, each func(File)) error {
	rd.next()
	tok, err := rd.dec.Token()
	if errors.Is(err, io.EOF) {
		return &SchemaError{Path: rd.path, Detail: core.Says("manifest.ItHoldsNoDocument", "it holds no JSON document at all")}
	}
	if err := rd.unreadable(err); err != nil {
		return err
	}
	switch tok {
	case nil:
		// null, which encoding/json reads into a manifest as nothing at all -
		// so it goes on to be refused for carrying no version, as it was.
	case json.Delim('{'):
		if err := rd.object(m, each); err != nil {
			return err
		}
	default:
		return &SchemaError{Path: rd.path, Detail: core.Says("manifest.ItIsNotAnObject", "it is a JSON %s rather than an object, so it is not a manifest this tool wrote", core.A("Kind", kindOf(tok)))}
	}
	return rd.end()
}

// end asks what follows the document, which may be white space and nothing
// else. Two documents in one file are not a manifest, and the decoder on its
// own stops where the first one closes.
func (rd *reading) end() error {
	rd.next()
	_, err := rd.dec.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	var syntax *json.SyntaxError
	if err == nil || errors.As(err, &syntax) {
		return &SchemaError{Path: rd.path, Detail: core.Says("manifest.SomethingFollowsIt", "something follows the end of the document, and a manifest is one JSON object with nothing after it but white space")}
	}
	return rd.unreadable(err)
}

// object reads the keys of the outermost object, the entries one at a time
// and every other key whole.
func (rd *reading) object(m *Manifest, each func(File)) error {
	seen := map[string]string{}
	for rd.more() {
		if err := rd.ctx.Err(); err != nil {
			return err
		}
		tok, err := rd.token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		if err := rd.once(seen, key); err != nil {
			return err
		}
		if strings.EqualFold(key, filesKey) {
			err = rd.entries(m, each)
		} else {
			err = rd.field(m, key)
		}
		if err != nil {
			return err
		}
	}
	_, err := rd.token()
	return err
}

// once remembers a key of the outermost object that is read into a field, and
// refuses it when the field was named before under any spelling - see twice. A
// key no field has is not remembered: it is read and let go, and a document of
// a million different keys would otherwise be a map with no ceiling.
func (rd *reading) once(seen map[string]string, key string) error {
	name, known := outermostKey(key)
	if !known {
		return nil
	}
	if first, twice := seen[name]; twice {
		return rd.twice(first, key)
	}
	seen[name] = key
	return nil
}

// twice is the refusal of a document that names one key two times.
//
// encoding/json keeps the second, so a document with two lists of files was
// acted on by its second list, and a reader going a piece at a time would read
// the first, check it and throw it away to do the same. This tool writes every
// key once, so a document with two was written or edited by something else,
// and which half of it was meant is a guess (decided by the owner on
// 2026-11-04, docs/MANIFEST-STREAMING-2026-11-04.md section 4.3). A key
// spelled another way is the same key, because encoding/json reads it as one.
func (rd *reading) twice(first, second string) error {
	detail := core.Says("manifest.ItHoldsTheKeyTwice", "it holds the key %q twice", core.A("Key", first))
	if first != second {
		detail = core.Says("manifest.ItHoldsTheKeyTwiceSpelled", "it holds the key %q twice, the second time spelled %q", core.A("Key", first), core.A("Second", second))
	}
	return &SchemaError{Path: rd.path, Detail: core.Says("manifest.TwiceBecause",
		"%s. This tool writes every key once, so the manifest was written or edited by something else, and which of the two it means is a guess. "+
			"Use the manifest the run actually wrote", core.A("Detail", detail))}
}

// field reads one key other than files into m, through encoding/json - so the
// key is matched to a field, and the value held to the type of the field,
// exactly the way a document read whole is. A key no field has is read and
// let go, as it always was.
func (rd *reading) field(m *Manifest, key string) error {
	var raw json.RawMessage
	if err := rd.value(&raw); err != nil {
		return err
	}
	name, err := json.Marshal(key)
	if err != nil {
		return err
	}
	doc := make([]byte, 0, len(name)+len(raw)+3)
	doc = append(append(append(append(append(doc, '{'), name...), ':'), raw...), '}')
	return rd.unreadable(json.Unmarshal(doc, m))
}

// entries reads the list of files an entry at a time, checking where each one
// lands as it passes and handing it on.
func (rd *reading) entries(m *Manifest, each func(File)) error {
	tok, err := rd.token()
	if err != nil {
		return err
	}
	switch tok {
	case nil:
		// null, which encoding/json reads as no list at all.
		return nil
	case json.Delim('['):
	default:
		return &SchemaError{Path: rd.path, Detail: core.Says("manifest.FilesIsNotAList", "files is a JSON %s rather than a list", core.A("Kind", kindOf(tok)))}
	}
	// Present and empty until something arrives, which is how encoding/json
	// leaves a list that was there - and how Encode tells [] from null.
	m.Files = []File{}
	for i := 1; rd.more(); i++ {
		// Asked once an entry, because a manifest of a million entries takes
		// seconds, and Ctrl+C in the middle of them has to be answered then
		// rather than at the end.
		if err := rd.ctx.Err(); err != nil {
			return err
		}
		var f File
		if err := rd.value(&f); err != nil {
			return err
		}
		if rd.escaped == nil {
			rd.escaped = entryEscapes(rd.path, i, f.Path)
		}
		each(f)
	}
	_, err = rd.token()
	return err
}

// kindOf is what a JSON token that opens or is a value is called in a refusal.
func kindOf(tok json.Token) string {
	switch tok {
	case json.Delim('['):
		return "list"
	case json.Delim('{'):
		return "object"
	}
	switch tok.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "true or false"
	}
	return "value"
}

// filesKey is the key the entries are under.
const filesKey = "files"

// outermostKeys is every key of the outermost object that encoding/json reads
// into a field of Manifest - taken from the type, so a field added to it is
// held to the rule about naming a key twice without anybody listing it here.
var outermostKeys = func() []string {
	var keys []string
	t := reflect.TypeFor[Manifest]()
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		keys = append(keys, name)
	}
	return keys
}()

// outermostKey is the field a key of the outermost object is read into, under
// the same rule encoding/json matches keys by, and whether there is one.
func outermostKey(key string) (string, bool) {
	for _, name := range outermostKeys {
		if strings.EqualFold(key, name) {
			return name, true
		}
	}
	return "", false
}

// ClaimedFile is one file a manifest says is on the disk, and all that verify and
// cleanup need of it.
type ClaimedFile struct {
	Path   string
	SHA256 string
	Bytes  int64
}

// Claims is what a manifest says is on the disk, in the order it lists the
// files, and the name of the instructions it names, which both commands treat
// as the run's own.
type Claims struct {
	Files        []ClaimedFile
	Instructions string
}

// claimed says whether an entry is a file the manifest says is on the disk.
//
// Two kinds of entry are deliberately not claimed. An entry marked failed
// describes a file that was never written - reporting it missing would be a
// false alarm about the one thing the manifest was honest about. And an entry
// that was not materialised was never meant to reach the disk at all.
//
// This is the single reading of the manifest. Both commands come through it,
// so neither can drift into claiming something the other does not.
func (f File) claimed() bool { return !f.Failed && f.Materialized }

func claimOf(f File) ClaimedFile {
	return ClaimedFile{Path: f.Path, SHA256: f.Hashes.SHA256, Bytes: f.Bytes}
}

// Claims is what this manifest says is on the disk - what ReadClaims gives,
// for a manifest already held whole.
func (m *Manifest) Claims() Claims {
	c := Claims{Instructions: m.Run.Instructions}
	for _, f := range m.Files {
		if f.claimed() {
			c.Files = append(c.Files, claimOf(f))
		}
	}
	return c
}

// ReadClaims reads what the manifest at path says is on the disk, for verify
// and cleanup, a piece at a time.
//
// most is about how much memory checking what it claims may take, from the
// memory setting, and nought is no limit. A manifest past it is refused once
// the whole document has been read and found sound, so the refusal can say
// how much it would have taken and a document with a fault of its own is
// refused for that fault - nothing is held of it past the limit in the
// meantime.
func ReadClaims(ctx context.Context, path string, most int64) (Claims, error) {
	var claims Claims
	var need int64
	files := 0
	instructions, err := EachClaim(ctx, path, func(c ClaimedFile) {
		files++
		need += checkingCost(c.Path, c.SHA256)
		if most > 0 && need > most {
			// The rest is counted and let go, so the refusal can say how
			// much it would take without the reader holding it.
			claims.Files = nil
			return
		}
		claims.Files = append(claims.Files, c)
	})
	if err != nil {
		return Claims{}, err
	}
	if most > 0 && need > most {
		return Claims{}, &TooMuchToHoldError{Path: path, Files: files, Need: need, Most: most}
	}
	claims.Instructions = instructions
	return claims, nil
}

// EachClaim reads the manifest at path a piece at a time and hands every file
// it claims to keep, holding none of them itself, and gives back the name of
// the instructions it names.
//
// keep runs before the document has been found sound - see decode - so a
// caller acts on nothing it was handed unless EachClaim returns no error. For
// a caller that keeps a few of the claims out of many, which is what the
// records of the other runs in a directory are to verify.
func EachClaim(ctx context.Context, path string, keep func(ClaimedFile)) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	var m Manifest
	err = decode(ctx, bufio.NewReaderSize(f, readBuffer), path, &m, func(e File) {
		if e.claimed() {
			keep(claimOf(e))
		}
	})
	if err != nil {
		return "", err
	}
	return m.Run.Instructions, nil
}

// checkingCost is about how much memory checking one claimed file takes in
// verify or cleanup: the claim held, and what the checking makes of it.
//
// The claim is a ClaimedFile and its two strings - measured at 134 B for a path of
// 17 characters, and 48 B and the two lengths come to 129 (section 8 of the
// analysis). The checking holds more of each, and that was measured too: in a
// container of 1 GB on 2026-11-04, a manifest of 600 000 files with paths of
// 16 characters, verify held at most 206 MB alive and cleanup --yes 282 MB -
// 343 and 470 B a file, 2.6 and 3.5 times what the claims take
// (tools/probes/manifestread/container.py). Four times covers both. The path
// is held about four times over as well - claimed, joined to the directory,
// folded for spelling, and in the list of what the directory holds - so a
// longer path is counted at about the rate it costs.
func checkingCost(path, sum string) int64 {
	return checkingFactor * (claimBytes + int64(len(path)) + int64(len(sum)))
}

// claimBytes is what a claim takes besides the bytes of its two strings.
const claimBytes = 48

// checkingFactor is how many times its claims checking a manifest takes.
const checkingFactor = 4

// TooMuchToHoldError is a manifest listing more files than verify or cleanup
// may hold under the memory setting.
type TooMuchToHoldError struct {
	Path  string
	Files int
	// Need is about how much checking the files takes, and Most what the
	// memory setting gives the command.
	Need, Most int64
}

func (e *TooMuchToHoldError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *TooMuchToHoldError) Said() core.Said {
	return core.Says("manifest.TooMuchToHold",
		"%s lists %s, and checking them takes about %s of memory, more than the %s the memory setting gives this command. "+
			"Give it more through the memory setting, a size such as 8gb or off for no limit, or split the run it describes",
		core.A("Path", e.Path), core.A("Files", core.SaysN("manifest.Files", "%d file", "%d files", core.A("Count", e.Files))),
		core.A("Need", core.Bytes(e.Need)), core.A("Most", core.Bytes(e.Most)))
}
