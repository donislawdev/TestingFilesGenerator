// Package archivescan is the tool that looks into an archive and says whether
// unpacking it is safe - without unpacking it.
//
// What it looks for are the ways an archive hurts whoever unpacks it: an entry
// written outside the folder, a link written through, two entries landing on
// one file, a name a system cannot hold, an archive built to explode, contents
// that are not what the list of contents says. Everything is read, the list
// and the contents, and nothing is written - the contents go through a
// checksum and are thrown away.
//
// The analysis, the measurements behind the limits and what was left out:
// docs/SKANER-ARCHIWUM-2026-10-19.md.
package archivescan

import (
	"context"
	"errors"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// ID is the name of the tool, after "tfg tool".
const ID = "archive-scan"

// The names of what the tool declares, used by the request and by both
// surfaces.
const (
	InputArchive = "archive"
	SettingLimit = "unpacked_limit"
)

// The limits, fixed. The one a person sets is SettingLimit. Each is the limit
// of the archive given and every archive inside it together, not of each one,
// so an archive of a thousand archives holds no more than one archive may
// (docs/SKANER-W-GLAB-2026-10-19.md §2.1).
const (
	// MostEntries is where reading stops. A million names held to find two
	// that land on one file is a few hundred megabytes, and an archive with
	// more is a trap of its own for a file system.
	MostEntries = 1_000_000
	// MostPlaces and MostNameBytes bound what is held to compare names: the
	// files and folders the entries unpack to, and the bytes of their names.
	// An entry with a name of 64 KiB, a million times, would otherwise be
	// 64 GB held in memory to answer a question about a file.
	MostPlaces    = 2 * MostEntries
	MostNameBytes = 256 << 20
	// MostDepth is how many archives deep the tool looks. An archive can
	// hold itself, and that ends here rather than never.
	MostDepth = 16
	// MostInside is the most the archives inside hold in memory at once to
	// be read: a ZIP is read from its end, so it has to be held whole.
	MostInside = 256 << 20
	// MostListed, MostShownName and MostListedBytes bound the report: the
	// items of one note, the bytes of one name in it, and the bytes of every
	// name in it. Past them a finding is counted and not listed - a million
	// findings, or a name of 64 KiB, are no report anybody reads.
	MostListed      = 100
	MostShownName   = 1024
	MostListedBytes = 4 << 20
	// MostReports is how many reports of archives inside --json carries.
	MostReports = 1000
	// BombBytes and BombRatio are an archive built to explode: at least this
	// much when unpacked, and more than this many times its own size. Both,
	// because either alone is innocent - a file of zeros packs a thousand to
	// one, and a big archive unpacks big. Measured on 906 real archives
	// (docs/SKANER-ARCHIWUM-2026-10-19.md §2.2): 99 in 100 unpack to 7 times
	// their size or less, the largest to 73.
	BombBytes = 1 << 30
	BombRatio = 100
)

// The keys of the conclusions.
const (
	concludedSafe   = "safe"
	concludedUnsafe = "unsafe"
)

func init() {
	tool.Register(tool.Descriptor{
		ID:       ID,
		Question: "Is this archive safe to unpack?",
		Detail: "Reads a ZIP, TAR, TAR.GZ or gzip file and everything in it without unpacking anything, " +
			"and says whether unpacking it could write outside its folder, overwrite a file or fill the disk.",
		Inputs: []tool.Input{{
			Name: InputArchive, Kind: tool.File,
			Detail: "The archive to look into: ZIP (DOCX, XLSX and JAR are ZIP inside), TAR, TAR.GZ or gzip. It is only read.",
		}},
		Settings: []format.Property{{
			Name: SettingLimit, Kind: format.PropertySize,
			Detail: "The most the archive may unpack to, all its files together with those of the archives inside it - for a server or a disk that takes no more. " +
				"Left empty, only an archive built to explode is reported: 1 GB or more, and more than 100 times its own size.",
		}},
		Notes: declaredNotes(),
		Conclusions: []tool.Conclusion{
			{ID: concludedSafe, Says: "Safe to unpack: none of the traps this tool looks for is in it."},
			{ID: concludedUnsafe, Says: "Not safe to unpack: the lines above say why.", Fails: true},
		},
		Run: run,
	})
}

// run is the tool, on a request tool.Start has checked and defaulted.
func run(ctx context.Context, in tool.Request, progress tool.Progress) (tool.Result, error) {
	limit := int64(-1)
	if raw := in.Values[SettingLimit]; raw != "" {
		// Checked by the registry already, which is why the error is not.
		limit, _ = core.ParseSize(raw)
	}
	path := in.Inputs[InputArchive]
	f, before, err := core.OpenRegular(path)
	var not *core.NotRegularError
	if errors.As(err, &not) {
		return tool.Result{}, &NotAFileError{Path: path, Directory: not.Directory}
	}
	if err != nil {
		return tool.Result{}, err
	}
	defer func() { _ = f.Close() }()

	s := newScan(ctx, path, before.Size(), limit, progress)
	if err := s.look(f); err != nil {
		return tool.Result{}, err
	}
	s.overLimit()
	after, err := f.Stat()
	if err != nil {
		return tool.Result{}, err
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return tool.Result{}, &ChangedError{Path: path}
	}
	return s.result(), nil
}

// look tells what an archive is and reads it as that: the file given, or an
// archive inside it held in memory.
func (s *scan) look(r io.ReaderAt) error {
	head := make([]byte, headLength)
	n, err := r.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	head = head[:n]
	switch k, other := sniff(head); {
	case k == kindZip:
		s.kind = kindZip
		return s.zip(r)
	case k == kindTar:
		s.kind = kindTar
		return s.tar(io.NewSectionReader(r, 0, s.size))
	case k == kindGzip:
		return s.gzip(io.NewSectionReader(r, 0, s.size))
	case other != "":
		return &NotReadYetError{Path: s.path, Kind: other}
	}
	return &NotAnArchiveError{Path: s.path}
}
