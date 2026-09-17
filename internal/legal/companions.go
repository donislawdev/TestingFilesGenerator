package legal

// A Companion is somebody else's program shipped BESIDE a binary of this
// project rather than inside it: a file in the release archive, next to
// ours, loaded at run time.
//
// The third class of the registry, since 2026-09-17, and different from the
// other two in every way that matters to a guard. A module is reported by
// the build and a font by the compiler that embeds it, so both can be asked
// about. A companion is put in the archive by the release workflow, from a
// download that workflow makes, and nothing in the binary knows whether it
// is there - the program finds out by looking beside itself when it needs
// it. So what holds this entry honest is not a build report but two
// comparisons: with the file the workflow reads its download from
// (.github/mesa-dist-win), and with the archive the release publishes,
// which tools/release-check.py opens.
type Companion struct {
	// Name is what a person calls it.
	Name string

	// Binary is the command it accompanies - the name of the binary, not of
	// one platform's archive. The command line binary has no companion, and
	// a guard holds it to that.
	Binary string

	// Platform is the one release archive that carries it, as GOOS/GOARCH.
	// Nothing else does: on Linux the system has a software renderer of its
	// own, and macOS has never offered the toolkit less than it needs.
	Platform string

	// Files are the paths inside the archive, slash separated, in the order
	// the program loads them - and the size and SHA-256 of each, read from
	// the pinned archive so that what the release packs can be compared with
	// what was reviewed.
	Files []CompanionFile

	// SPDX is the licence expression of everything compiled into these
	// files, read from the files of the pinned versions rather than from
	// memory - see the Note for what each part is.
	SPDX string

	// Copyright is the line of the principal component. The rest are named in
	// the Note, because a renderer built from four projects has four.
	Copyright string

	// Source says exactly where the bytes come from: the project that
	// builds them, its release, the archive inside that release, the
	// archive's SHA-256, and the directory inside the archive the files are
	// taken from. The workflow reads the same version and sum from
	// .github/mesa-dist-win, and a guard holds the two equal.
	Source CompanionSource

	// Note is the sentence the fields above cannot carry: what the files do
	// here, and what was measured inside them.
	Note string
}

// A CompanionFile is one file of a companion as the pinned archive holds it.
type CompanionFile struct {
	Path   string // inside the release archive, slash separated
	Size   int64
	SHA256 string
}

// A CompanionSource is where a companion's bytes are downloaded from.
type CompanionSource struct {
	Project string // the project that builds and publishes the archive
	Version string // its release
	Archive string // the file inside that release
	SHA256  string // of that file, as downloaded
	URL     string // where that file is
	Inside  string // the directory inside the archive the files are taken from
}

// companions is the reviewed list.
//
// Everything below was read on 2026-09-17 from the files of the pinned
// versions: docs/license.rst and licenses/ of mesa-26.2.0.tar.xz from
// archive.mesa3d.org, buildinfo/msvc.txt of pal1000/mesa-dist-win at its
// 26.2.0 tag, llvm/LICENSE.TXT at llvmorg-22.1.8, and LICENSE of
// microsoft/DirectX-Headers at v1.619.5. What is inside the shipped file was
// measured in its bytes rather than taken from the build list: it names
// "Mesa 26.2.0" and "LLVM 22.1.8", carries the llvmpipe, d3d12, zink and
// softpipe drivers and the D3D12 headers, and holds neither zlib's nor
// zstd's code - the only mention of zlib is LLVM saying it was built without
// it. Its imports are system libraries only, so it needs no runtime
// installed.
//
// Two things in the Mesa tree carry a GPL identifier and neither is in
// these files: Linux kernel headers under include/drm-uapi and headers of
// the freedreno and svga drivers, none of which a Windows WGL build
// compiles. The c11 threads shim Mesa compiles on Windows is BSL-1.0.
var companions = []Companion{
	{
		Name:     "Mesa 3D, llvmpipe software renderer",
		Binary:   "tfg-gui",
		Platform: "windows/amd64",
		Files: []CompanionFile{
			{Path: "opengl/libgallium_wgl.dll", Size: 61734400, SHA256: "1a2e49cd5fdb1a857d98117ab04240d723b57da5dffe6d07f5386f42014557c1"},
			{Path: "opengl/opengl32.dll", Size: 139264, SHA256: "33b217ed7947b48684baa987914475898a2b4d7d64cce96b078216c67a633582"},
		},
		SPDX:      "MIT AND Apache-2.0 WITH LLVM-exception AND BSL-1.0",
		Copyright: "Copyright (C) 1999-2007 Brian Paul, Copyright (C) 2008 VMware, Inc.",
		Source: CompanionSource{
			Project: "pal1000/mesa-dist-win",
			Version: "26.2.0",
			Archive: "mesa3d-26.2.0-release-msvc.7z",
			SHA256:  "dcb2719ef346dab5b609fcb193a5f13cfc4b0502e3f4de1ad43d349477402f47",
			URL:     "https://github.com/pal1000/mesa-dist-win/releases/download/26.2.0/mesa3d-26.2.0-release-msvc.7z",
			Inside:  "x64/",
		},
		Note: "OpenGL in software, for a machine whose graphics driver offers no OpenGL 2.1 - a virtual machine without 3D acceleration, a remote desktop, a server. " +
			"Loaded only after the toolkit could not open a window, or when the window is started with the flag that asks for it, and never otherwise. " +
			"Mesa is MIT (core, the llvmpipe driver and the WGL frontend - the core's line is quoted, the driver is Copyright 2007 VMware, Inc.). " +
			"LLVM 22.1.8, compiled in for llvmpipe's code generation, is Apache-2.0 WITH LLVM-exception - its licence file states no copyright line under that licence, and the one it does carry, Copyright (c) 2003-2019 University of Illinois at Urbana-Champaign, belongs to the legacy licence the file also reproduces. " +
			"The C11 threads shim Mesa compiles on Windows is BSL-1.0, Copyright yohhoy 2012. " +
			"The Direct3D 12 headers, 1.619.5, are MIT, Copyright (c) Microsoft Corporation. " +
			"Built and published by pal1000/mesa-dist-win, whose own scripts are MIT, Copyright (c) 2017-2020 pal1000, and whose archive carries no licence file - which is why the texts are read from the sources of the pinned versions and reproduced in the notices.",
	},
}

// Companions returns the reviewed list of files shipped beside a binary.
func Companions() []Companion { return companions }
