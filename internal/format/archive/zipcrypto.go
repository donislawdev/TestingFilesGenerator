package archive

import (
	"hash/crc32"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The old ZIP encryption, the one everything opens and nothing modern trusts.
//
// It is here for what it does to a reader rather than for what it protects.
// Measured on 2026-09-01: .NET's own ZipFile opens a ZipCrypto archive, reports
// the entry at its true length, hands back a stream and fills it with
// CIPHERTEXT - 'ea 59 89 8e' where the file holds 'AAAA' - and never says the
// entry was encrypted at all. An application built on that library processes
// noise and calls it data. AES fails loudly in the same library, which is the
// safer defect and the less interesting one.
//
// So this is the fixture that finds a real class of fault, and it is the one
// the presets have been waiting for: PRESETS.md section 4.12 names "an archive
// with a password waved through without warning" as a thing to catch.
//
// The cipher itself is not ours and was not taken on trust. It was written into
// a probe first, pointed at an archive 7-Zip produced, and asked to decrypt it -
// the check byte, the plaintext CRC and the bytes themselves all came back
// right. tools/probes/zipcrypto, and it stays runnable, because a keystream
// that is subtly wrong produces a file some readers still accept.

const (
	// The three keys PKWARE starts from and the multiplier it steps them with.
	// Constants of the scheme rather than choices of ours.
	key0Init   = 305419896
	key1Init   = 591751049
	key2Init   = 878082192
	multiplier = 134775813

	// zipCryptoHeader is the twelve bytes that precede an entry's data: eleven
	// that vary and one that lets a reader reject a wrong password without
	// decrypting anything else. It is the whole of what this scheme adds, which
	// is why the overhead is a constant.
	zipCryptoHeader = 12
)

var crcTable = crc32.MakeTable(crc32.IEEE)

// pkware is the stream cipher, keyed by a password and then by every byte of
// plaintext that passes through it.
//
// The plaintext updates the keys in both directions, which is why encrypting
// and decrypting are two functions here rather than one - a stream cipher that
// is its own inverse would not need the distinction, and this one is not.
type pkware struct{ k0, k1, k2 uint32 }

func newPKWARE(password string) *pkware {
	c := &pkware{k0: key0Init, k1: key1Init, k2: key2Init}
	for i := 0; i < len(password); i++ {
		c.update(password[i])
	}
	return c
}

func (c *pkware) update(p byte) {
	c.k0 = crcTable[(c.k0^uint32(p))&0xff] ^ (c.k0 >> 8)
	c.k1 += c.k0 & 0xff
	c.k1 = c.k1*multiplier + 1
	c.k2 = crcTable[(c.k2^(c.k1>>24))&0xff] ^ (c.k2 >> 8)
}

func (c *pkware) keyByte() byte {
	t := uint16(c.k2|2) & 0xffff
	return byte((t * (t ^ 1)) >> 8)
}

func (c *pkware) encrypt(p byte) byte {
	x := p ^ c.keyByte()
	c.update(p)
	return x
}

// zipCryptoWriter encrypts on the way through, header first.
type zipCryptoWriter struct {
	out io.Writer
	c   *pkware
}

// newZipCryptoWriter starts an entry, writing the twelve byte header before
// anything else.
//
// The eleven bytes that vary come from the run seed and never from crypto/rand,
// for the reason every other draw in this tool avoids it: two runs of one
// recipe have to give one file. The twelfth is the high byte of the plaintext
// CRC, and it is why this scheme needs the contents known before the first byte
// goes out - the whole reason a ZipCrypto entry is generated twice.
func (l Lock) newZipCryptoWriter(w io.Writer, seed uint64, index int, crc uint32) (io.WriteCloser, error) {
	c := newPKWARE(l.Password)
	rng := core.NewRand(core.FileSeed(seed, index))

	header := make([]byte, zipCryptoHeader)
	for i := range header[:zipCryptoHeader-1] {
		header[i] = byte(rng.Uint32())
	}
	header[zipCryptoHeader-1] = byte(crc >> 24)
	for i := range header {
		header[i] = c.encrypt(header[i])
	}
	if _, err := w.Write(header); err != nil {
		return nil, err
	}
	return &zipCryptoWriter{out: w, c: c}, nil
}

func (z *zipCryptoWriter) Write(p []byte) (int, error) {
	out := make([]byte, len(p))
	for i := range p {
		out[i] = z.c.encrypt(p[i])
	}
	if _, err := z.out.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close has nothing to finish. ZipCrypto signs nothing - the entry's own CRC is
// what a reader checks, after decrypting, if it checks at all.
func (z *zipCryptoWriter) Close() error { return nil }
