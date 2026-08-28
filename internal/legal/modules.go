// The reviewed list of modules, one entry for each module a binary of this
// project links. Seeded from THIRD-PARTY-NOTICES.md on 2026-08-28 and held to
// it by a guard from that day on, so the two cannot drift apart.
//
// There are no versions here, and that is the point. A version is a fact about
// the build, so it is read from the build - by `go list` when a document is
// generated, and by debug.ReadBuildInfo when a binary is asked about itself. A
// third written copy would be a third thing to keep true.

package legal

// modules is the list. Copyright is the line the module's own licence file
// carries. When it carries none, Note says so rather than leaving a blank that
// reads like an oversight.
var modules = []Module{
	{Path: "github.com/goccy/go-yaml", SPDX: "MIT", Copyright: "(c) 2019 Masaaki Goshima",
		Note: "Reads the recipe file. One of the two modules in the command line binary."},
	{Path: "golang.org/x/text", SPDX: "BSD-3-Clause", Copyright: "Copyright 2009 The Go Authors",
		Note: "Unicode normalisation, used to decide whether two file names are one name spelled two ways. One of the two modules in the command line binary."},
	{Path: "std", SPDX: "BSD-3-Clause", Copyright: "Copyright 2009 The Go Authors",
		Note: "The Go runtime and standard library, linked into every binary here. Not a module, so `go list -deps` never names it - the one entry this list carries that no build can report."},
	{Path: "fyne.io/fyne/v2", SPDX: "BSD-3-Clause", Copyright: "(C) 2018 Fyne.io developers (see AUTHORS)"},
	{Path: "fyne.io/systray", SPDX: "Apache-2.0", Copyright: "2014 Brave New Software Project, Inc."},
	{Path: "github.com/BurntSushi/toml", SPDX: "MIT", Copyright: "(c) 2013 TOML authors"},
	{Path: "github.com/FyshOS/fancyfs", SPDX: "BSD-3-Clause", Copyright: "(C) 2025 FyshOS developers (see AUTHORS)"},
	{Path: "github.com/anthonynsimon/bild", SPDX: "MIT", Copyright: "(c) 2021 Anthony Najjar Simon"},
	{Path: "github.com/clipperhouse/uax29/v2", SPDX: "MIT", Copyright: "(c) 2020 Matt Sherman"},
	{Path: "github.com/fsnotify/fsnotify", SPDX: "BSD-3-Clause", Copyright: "Copyright 2012 The Go Authors"},
	{Path: "github.com/fyne-io/image", SPDX: "BSD-3-Clause", Copyright: "(c) 2022, Fyne.io"},
	{Path: "github.com/fyne-io/oksvg", SPDX: "BSD-3-Clause", Copyright: "(c) 2018, Steven R Wiley"},
	{Path: "github.com/go-gl/gl", SPDX: "MIT", Copyright: "(c) 2014 Eric Woroshow"},
	{Path: "github.com/go-gl/glfw/v3.4/glfw", SPDX: "BSD-3-Clause", Copyright: "(c) 2012 The glfw3-go Authors. All rights reserved."},
	{Path: "github.com/go-text/render", SPDX: "BSD-3-Clause", Copyright: "2021 The go-text authors"},
	{Path: "github.com/go-text/typesetting", SPDX: "BSD-3-Clause", Copyright: "2021 The go-text authors"},
	{Path: "github.com/godbus/dbus/v5", SPDX: "BSD-2-Clause", Copyright: "(c) 2013, Georg Reinke (<guelfey at gmail dot com>), Google"},
	{Path: "github.com/jeandeaual/go-locale", SPDX: "MIT", Copyright: "(c) 2020 Alexis Jeandeau"},
	{Path: "github.com/jsummers/gobmp", SPDX: "MIT", Copyright: "(c) 2012-2015 Jason Summers"},
	{Path: "github.com/mattn/go-runewidth", SPDX: "MIT", Copyright: "(c) 2016 Yasuhiro Matsumoto"},
	{Path: "github.com/nfnt/resize", SPDX: "ISC", Copyright: "(c) 2012, Jan Schlicht"},
	{Path: "github.com/nicksnyder/go-i18n/v2", SPDX: "MIT", Copyright: "(c) 2014 Nick Snyder https://github.com/nicksnyder"},
	{Path: "github.com/rymdport/portal", SPDX: "Apache-2.0", Copyright: "",
		Note: "The licence file is the unmodified Apache template with the copyright placeholder left in, so there is no line to quote. Published by the rymdport project, and linked on Linux only."},
	{Path: "github.com/srwiley/oksvg", SPDX: "BSD-3-Clause", Copyright: "(c) 2018, Steven R Wiley"},
	{Path: "github.com/srwiley/rasterx", SPDX: "BSD-3-Clause", Copyright: "(c) 2018, Steven R Wiley"},
	{Path: "github.com/yuin/goldmark", SPDX: "MIT", Copyright: "(c) 2019 Yusuke Inuzuka"},
	{Path: "golang.org/x/image", SPDX: "BSD-3-Clause", Copyright: "2009 The Go Authors."},
	{Path: "golang.org/x/net", SPDX: "BSD-3-Clause", Copyright: "2009 The Go Authors."},
	{Path: "golang.org/x/sys", SPDX: "BSD-3-Clause", Copyright: "2009 The Go Authors."},
}
