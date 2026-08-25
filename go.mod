module github.com/donislawdev/TestingFilesGenerator

// The compiler takes part in producing bytes, so its version is part of the
// byte stability contract (D11). This line is a minimum - the exact version
// used for tests and releases is pinned in the CI workflow, and the guard
// test in internal/guard reports any drift it causes. See docs/STACK.md.
go 1.26.5

// What we actually build with, which is not the same statement as the line
// above. That one is a floor for anybody compiling this. This one says which
// toolchain produces our binaries, and Go fetches it rather than asking anyone
// to install it.
//
// Raised to 1.26.6 on 2026-08-13 because govulncheck reported five standard
// library vulnerabilities reachable from the window binary under 1.26.5 - among
// them net/url, crypto/tls and encoding/xml - all of them fixed in 1.26.6. The
// command line binary was clean. The byte stability guards were run under the
// new toolchain before this line moved and none of them shifted, so D11 holds
// and no major version is owed.
//
// Raised again to 1.26.7 on 2026-08-25, and the reason is different. That
// release is not a security one - it went out on 2026-08-19 with fixes to
// net/http, which only the window binary links and only through the toolkit.
// What was actually wrong is that the machine this is written on had already
// moved to 1.26.7, while this line and the workflow both said 1.26.6: a
// toolchain line is a floor, so the local build quietly used a compiler no CI
// job ever ran. For a project whose bytes are part of its contract, "green
// here" and "green there" have to mean the same compiler. The byte stability
// guards were run under 1.26.7 before this line moved and none of them
// shifted, so D11 holds and no major version is owed.
toolchain go1.26.7

require github.com/goccy/go-yaml v1.19.2

require golang.org/x/text v0.41.0

require github.com/FyshOS/fancyfs v0.0.1 // indirect

require (
	fyne.io/fyne/v2 v2.8.0
	fyne.io/systray v1.12.2 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/anthonynsimon/bild v0.14.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fredbi/uri v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fyne-io/gl-js v0.2.1-0.20260315212741-029c47fd27e8 // indirect
	github.com/fyne-io/glfw-js v0.4.0 // indirect
	github.com/fyne-io/image v0.1.1 // indirect
	github.com/fyne-io/oksvg v0.2.0 // indirect
	github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276 // indirect
	github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1.0.20260707082822-2a407d02d01a // indirect
	github.com/go-text/render v0.2.1 // indirect
	github.com/go-text/typesetting v0.3.4 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/hack-pad/go-indexeddb v0.3.2 // indirect
	github.com/hack-pad/safejs v0.1.0 // indirect
	github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade // indirect
	github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nicksnyder/go-i18n/v2 v2.5.1
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rymdport/portal v0.4.2 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/yuin/goldmark v1.8.2 // indirect
	golang.org/x/image v0.43.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
