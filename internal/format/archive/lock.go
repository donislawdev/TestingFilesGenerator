package archive

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	// SHA-1 is not a choice here. WinZip AES derives its key with
	// PBKDF2-HMAC-SHA1 and signs the ciphertext with HMAC-SHA1, both named
	// in the specification, and an archive built with anything else is one
	// no archiver on earth opens. It is used for key derivation and for a
	// message code, never as a digest anybody trusts to be collision free.
	//nolint:gosec // G505: the format specifies SHA-1 and a different hash writes an unreadable archive
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Locking an archive, and the two settings that say so together.
//
// Everything here follows one measurement, written up in docs/MVP-FORMATS.md
// section 2.16, and the measurement is what makes the feature possible at all:
// the bytes a lock adds are a FIXED count per entry. A stream cipher does not
// change the length of what it encrypts, so only the salt, the verifier and
// the authentication code are added, and each is a constant. The size of the
// archive therefore stays an exact function of its input, and the planning
// phase can still state it without building it - which is the guard the whole
// container design rests on.
//
// Compression would not have that property, and that is the difference between
// this setting and the one nobody has built.

const (
	// The parts of a WinZip AE entry, read out of what 7-Zip writes rather
	// than out of a specification remembered. Salt length follows the key.
	pwvLen     = 2
	authLen    = 10
	iterations = 1000

	// aesExtraLen is the 0x9901 field: two bytes of id, two of length and
	// seven of body. It is written into the local header AND the central
	// directory, so an entry pays for it twice.
	aesExtraLen = 11
)

// Lock is what an archive is locked with, once both halves agree.
//
// The zero value is an open archive, which is what nearly every run wants.
type Lock struct {
	// Method is one of the encryption constants. NoEncryption or empty means
	// the archive is open.
	Method string
	// Password is written into the manifest exactly as it was typed. That is a
	// decision rather than an oversight: a fixture nobody can open is a fixture
	// nobody can test with, and the whole point of the file is that the test
	// knows the password.
	Password string
}

// On says whether anything is encrypted.
func (l Lock) On() bool { return l.Method != "" && l.Method != NoEncryption }

// keyLen is the AES key in bytes, and the salt is half of it. Zero for an
// archive that is not locked with AES.
func (l Lock) keyLen() int {
	switch l.Method {
	case AES128:
		return 16
	case AES192:
		return 24
	case AES256:
		return 32
	}
	return 0
}

func (l Lock) saltLen() int { return l.keyLen() / 2 }

// strength is what the 0x9901 field calls the key length.
func (l Lock) strength() byte {
	switch l.Method {
	case AES128:
		return 1
	case AES192:
		return 2
	case AES256:
		return 3
	}
	return 0
}

// EntryOverhead is the bytes this lock adds to one entry's DATA.
//
// Measured, and it agrees from two directions - the size of the whole file and
// the compressed size field in the local header:
//
//	AES-128     +20   (8 salt, 2 verifier, 10 authentication)
//	AES-192     +24
//	AES-256     +28
//
// The 0x9901 field is not counted here. It lives in the headers, and the
// headers are written for real during the counting pass, so the writer counts
// those eleven bytes twice over on its own.
func (l Lock) EntryOverhead() int64 {
	if !l.On() {
		return 0
	}
	return int64(l.saltLen() + pwvLen + authLen)
}

// Extra is the 0x9901 field an AES entry carries, and nil for anything else.
func (l Lock) Extra() []byte {
	if l.keyLen() == 0 {
		return nil
	}
	out := make([]byte, 0, aesExtraLen)
	out = binary.LittleEndian.AppendUint16(out, 0x9901)
	out = binary.LittleEndian.AppendUint16(out, 7)
	// AE-2, which carries no CRC of the plaintext. That is what makes an entry
	// writable in one streaming pass: nothing about the contents has to be
	// known before the first byte goes out. Measured on what 7-Zip writes.
	out = binary.LittleEndian.AppendUint16(out, 2)
	out = append(out, 'A', 'E')
	out = append(out, l.strength())
	// Stored underneath, because everything in these archives is stored.
	out = binary.LittleEndian.AppendUint16(out, 0)
	return out
}

// ReadLock reads the two settings that say how an archive is locked, and
// refuses the two states where they disagree.
//
// The pair rule is forced by the window rather than chosen. A menu cannot be
// empty - it opens on its declared default and sends it - so encryption arrives
// as "none" on every run from a window, whether or not anybody looked at it. A
// box somebody types in arrives empty and is left out. So "password given,
// encryption none" cannot be told apart from "password given, encryption not
// considered", and guessing between them would either lock an archive nobody
// asked to lock or hand back an open one to somebody who asked for a locked
// one. The second is what 7-Zip does with a tar, silently, and it is the reason
// this refuses instead.
//
// Both refusals name both halves, the way a log setting that can do nothing for
// the chosen shape does.
func ReadLock(id string, props map[string]string) (Lock, error) {
	password := props[Password]
	method := props[Encryption]
	if method == "" {
		method = NoEncryption
	}

	locked := method != NoEncryption
	switch {
	case password == "" && !locked:
		return Lock{Method: NoEncryption}, nil
	case password != "" && !locked:
		return Lock{}, &format.PropertyValueError{
			Format: id, Key: Encryption, Value: NoEncryption,
			Reason: "a password was given and this says the archive is not locked, so one of the two is not what you meant",
			Remedy: "Set encryption to " + AES256 + " to lock the archive, or remove the password to leave it open.",
		}
	case password == "" && locked:
		return Lock{}, &format.PropertyValueError{
			Format: id, Key: Password, Value: "",
			Reason: "the archive is set to be locked with " + method + " and there is no password to lock it with",
			Remedy: "Give a password, or set encryption to " + NoEncryption + ".",
		}
	}
	return Lock{Method: method, Password: password}, nil
}

// NewEntryWriter wraps w so that everything written to it is encrypted, with
// the salt and the verifier going out first.
//
// Nothing is held. The salt is written, then each block is encrypted as it
// arrives, and Close appends the authentication code - so an entry of any size
// costs the same memory as an entry of one byte, which is the guard this
// package would otherwise break.
//
// The salt comes from the run seed and never from crypto/rand. A random salt
// would give two runs of one recipe different bytes, which is untouchable rule
// 3 - and the same recipe producing the same file is more of the product here
// than the encryption is.
func (l Lock) NewEntryWriter(w io.Writer, seed uint64, index int) (io.WriteCloser, error) {
	if l.keyLen() == 0 {
		return nil, fmt.Errorf("archive: %q is not an encryption this build can write", l.Method)
	}
	salt := l.saltFor(seed, index)
	material, err := pbkdf2.Key(sha1.New, l.Password, salt, iterations, l.keyLen()*2+pwvLen)
	if err != nil {
		return nil, fmt.Errorf("archive: the key could not be derived: %w", err)
	}
	block, err := aes.NewCipher(material[:l.keyLen()])
	if err != nil {
		return nil, fmt.Errorf("archive: the cipher could not be built: %w", err)
	}
	if _, err := w.Write(salt); err != nil {
		return nil, err
	}
	if _, err := w.Write(material[l.keyLen()*2:]); err != nil {
		return nil, err
	}
	return &entryWriter{
		out: w,
		ctr: newCounter(block),
		mac: hmac.New(sha1.New, material[l.keyLen():l.keyLen()*2]),
	}, nil
}

// saltFor is this entry's salt, derived from the run rather than drawn.
func (l Lock) saltFor(seed uint64, index int) []byte {
	rng := core.NewRand(core.FileSeed(seed, index))
	salt := make([]byte, l.saltLen())
	for i := range salt {
		salt[i] = byte(rng.Uint32())
	}
	return salt
}

// entryWriter encrypts on the way through and signs what it wrote.
type entryWriter struct {
	out io.Writer
	ctr *counter
	mac hash
}

// hash is the part of hash.Hash this uses. Named so the field above reads as
// what it is rather than as an io.Writer that happens to be a digest.
type hash interface {
	io.Writer
	Sum(b []byte) []byte
}

func (e *entryWriter) Write(p []byte) (int, error) {
	out := make([]byte, len(p))
	e.ctr.xor(out, p)
	if _, err := e.mac.Write(out); err != nil {
		return 0, err
	}
	if _, err := e.out.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close writes the authentication code, which is what makes a wrong password
// an error rather than a file of noise.
func (e *entryWriter) Close() error {
	_, err := e.out.Write(e.mac.Sum(nil)[:authLen])
	return err
}

// counter is the counter mode WinZip AES uses, which is not the one
// crypto/cipher provides: this one counts little endian from one, and Go's CTR
// counts big endian. Two lines of difference and a file no reader will open.
type counter struct {
	block cipher.Block
	n     uint64
	in    [aes.BlockSize]byte
	pad   [aes.BlockSize]byte
	used  int
}

func newCounter(b cipher.Block) *counter {
	return &counter{block: b, used: aes.BlockSize}
}

func (c *counter) xor(dst, src []byte) {
	for i := range src {
		if c.used == aes.BlockSize {
			c.next()
		}
		dst[i] = src[i] ^ c.pad[c.used]
		c.used++
	}
}

func (c *counter) next() {
	c.n++
	c.in = [aes.BlockSize]byte{}
	binary.LittleEndian.PutUint64(c.in[:8], c.n)
	c.block.Encrypt(c.pad[:], c.in[:])
	c.used = 0
}
