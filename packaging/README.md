# Package sources: WinGet and Chocolatey

These are **templates**, not packages. Every `{{PLACEHOLDER}}` is filled by
`.github/scripts/build_packages.py` from the one place that owns the value: the
version from the release tag, the checksums from that release's
`verify-SHA256SUMS.txt`, the addresses from `go.mod` and `web/public/CNAME`, the
release date from `CHANGELOG.md`. The two values the renderer keeps a copy of,
the product name and the licence, are held to their Go originals by a guard.

    python .github/scripts/build_packages.py --tag v0.4.0 \
        --sums verify-SHA256SUMS.txt --out <a directory outside the repository>

## Four packages, two per feed

| | the window | the command line |
|---|---|---|
| WinGet | `DonislawDev.TestingFilesGenerator` | `DonislawDev.TestingFilesGenerator.CLI` |
| Chocolatey | `testing-files-generator` | `testing-files-generator-cli` |
| archive | `tfg-gui_<version>_windows_amd64.zip` | `tfg_<version>_windows_amd64.zip`, and `arm64` in WinGet |
| command | `tfg-gui` | `tfg` |

The window does not need the command line - it carries the same engine - so
each package stands alone. A build agent takes the command line without the
window, and one package waiting in moderation does not hold the other.

A template named `name.<kind>.ext.in` belongs to one kind of package only
(`window` or `cli`). One named `name.ext.in` belongs to every package.

## What each package has to get right

**WinGet.** `ArchiveBinariesDependOnPath: true`, in both. Without it WinGet reaches
the program through a symbolic link, and the window started that way looks for
its software renderer next to the link rather than in the `opengl` folder beside
the real file. With it, WinGet makes no link and puts the package's folder on
`PATH`. The command line would work either way, but without the field its shape
depends on the machine - a link where symbolic links are allowed, the folder on
`PATH` where they are not. WinGet adds no Start menu shortcut for a portable
package, and the window's description says so.

**Chocolatey.** The package downloads the release archive rather than carrying
it, so it holds no binaries and owes no `VERIFICATION.txt`, and the archive is the
same file, checksum and all, that the release page publishes. Each program is
unpacked into a folder of its own under `tools`. The window gets an empty
`tfg-gui.exe.gui` beside it, without which Chocolatey's shim waits for the window
to close and holds the terminal. It also gets a Start menu shortcut whose working
directory is `%USERPROFILE%`, because the window offers a `tfg-out` folder under
the directory it was started from, and the package folder is one an ordinary
account cannot write to. The uninstall removes that shortcut only when it points
into the package. The icon is a jsDelivr address pinned to the release tag:
moderation refuses `raw.githubusercontent.com` and `github.com/.../raw` alike,
and an icon on a branch would keep changing under an approved package.

**Neither package ends a running program.** `chocolateybeforemodify.ps1` says when
the program is still running from the package, and leaves closing it to the
person - a run in progress may be halfway through a set of files, and cutting it
would leave files with no manifest to say what they are.

## Submitting

Submitting is a person's step and stays one. Nothing here is wired into a
release: a package is published under the project's name to a feed somebody
else moderates.

1. The release is published and its `verify-SHA256SUMS.txt` is the file you
   render from.
2. Render, then `winget validate` both WinGet folders and `choco pack` both
   nuspecs.
3. Install, run and remove every package on a machine you can break.
4. Chocolatey: `choco push` with the maintainer's API key. WinGet: one pull
   request per package against `microsoft/winget-pkgs`, with the three files under
   `manifests/d/DonislawDev/TestingFilesGenerator/<version>/` and
   `manifests/d/DonislawDev/TestingFilesGenerator/CLI/<version>/`.

A published release asset is never replaced. Every package version names its
archive by address and checksum, so a replaced file breaks every install of it.
