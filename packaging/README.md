# Package sources: WinGet, Chocolatey and the Windows installer

These are **templates**, not packages. Every `{{PLACEHOLDER}}` is filled by
`.github/scripts/build_packages.py` from the one place that owns the value: the
version from the release tag, the checksums from that release's
`verify-SHA256SUMS.txt`, the addresses from `go.mod` and `web/public/CNAME`, the
release date from `CHANGELOG.md`. The three values the renderer keeps a copy
of, the product name, the licence and the copyright line, are held to their Go
originals by a guard.

    python .github/scripts/build_packages.py --tag vX.Y.Z \
        --sums verify-SHA256SUMS.txt --out <a directory outside the repository>

The window's WinGet package installs the Windows installer first, so the
checksum file has to have the installer's line. A release published before the
installer existed renders for Chocolatey only, with `--only chocolatey`.

The renderer refuses, with a sentence that names the input and says what to do,
and never a traceback: a tag that is a release candidate or not a tag, a
checksum file of another release or missing an archive, a version the changelog
never dated, a checksum file that is missing, unreadable, not a regular file,
not UTF-8 or over a megabyte (a release's is under a kilobyte), a destination
inside the repository - compared as a resolved path, so a link or a junction
does not get round it - or one that already holds something or cannot be looked
into, and a value that would break the file it lands in. It renders into a
working folder beside the destination and renames it at the end, and a run that
fails removes the working folder and any folder it made to hold the
destination.

## Four packages, two per feed

| | the window | the command line |
|---|---|---|
| WinGet | `DonislawDev.TestingFilesGenerator` | `DonislawDev.TestingFilesGenerator.CLI` |
| Chocolatey | `testing-files-generator` | `testing-files-generator-cli` |
| archive | `tfg-gui_<version>_windows_amd64.zip`, and in WinGet the installer `tfg-setup_<version>_windows_amd64.msi` first | `tfg_<version>_windows_amd64.zip`, and `arm64` in WinGet |
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
`PATH` where they are not. One cost of the field is not
ours to fix. WinGet 1.29.380 left the package's folder on `PATH` after an
uninstall, for both of these packages, in machine scope on Windows Server 2025
and in user scope on Windows 11 - measured on 2026-09-25, the entry stays and
points at a folder that no longer exists.
[microsoft/winget-cli#6160](https://github.com/microsoft/winget-cli/issues/6160),
open, reports the same for another package that sets the field.

**The window in WinGet: the installer first, the archive second.** A portable
package gets no Start menu shortcut, and nothing in a manifest can give it one -
the schema has no field for it, and
[microsoft/winget-cli#2299](https://github.com/microsoft/winget-cli/issues/2299)
asking for one has been open since 2022. So the window's manifest lists two
installers, and the order is load-bearing. The Windows installer comes first:
recent clients prefer it over a portable package, and older ones take the first
installer that applies. It installs for the whole machine, with the shortcut,
and WinGet knows it again by its `UpgradeCode`. The archive comes second, with
no scope, for two kinds of install the installer cannot serve. One is
`--scope user`, without administrator rights. The other is an install made
before the package had the installer - WinGet upgrades only within the kind
that is installed, so without the archive that install could not be upgraded at
all, and with it, it is upgraded as it is. The description says how to get the
shortcut then. The fields of the archive stand in its own entry, not at the top
of the manifest, because the two installers are of two kinds. The command line's
manifest keeps one kind and its fields at the top.

**Chocolatey.** The package downloads the release archive rather than carrying
it, so it holds no binaries and owes no `VERIFICATION.txt`, and the archive is the
same file, checksum and all, that the release page publishes. Each program is
unpacked into a folder of its own under `tools`. The window gets an empty
`tfg-gui.exe.gui` beside it, without which Chocolatey's shim waits for the window
to close and holds the terminal. It also gets a Start menu shortcut whose working
directory is `%USERPROFILE%`, because the window offers a `tfg-out` folder under
the directory it was started from, and the package folder is one an ordinary
account cannot write to. The install leaves alone a shortcut under the same name
that starts a program outside the package - the Windows installer makes exactly
that one - or that points at no file, as a shortcut to a shell item does, and
replaces one whose target is gone. The uninstall removes the shortcut only when
it points into the package. The icon is a jsDelivr address pinned to the release tag:
moderation refuses `raw.githubusercontent.com` and `github.com/.../raw` alike,
and an icon on a branch would keep changing under an approved package.

**Neither package ends a running program** - a run in progress may be halfway
through a set of files, and cutting it would leave files with no manifest to say
what they are. What each feed does instead was measured on 2026-09-25, with the
program running - Chocolatey on Windows Server 2025, WinGet there in machine
scope and on Windows 11 in user scope:

- **Chocolatey goes ahead**, an upgrade and an uninstall alike, and reports
  success. It moves the package folder aside to `lib-bkp`, and the running copy
  keeps working from there - but it cannot delete that copy, so the copy stays:
  after an upgrade until the next Chocolatey operation on the package, after an
  uninstall for good. `chocolateybeforemodify.ps1` says so at that moment and
  names the folder to delete once the program is closed.
- **WinGet with the archive stops half way.** An upgrade fails with "Access is
  denied" on the program, having already deleted some of the other files, and
  the package works again once the upgrade runs with the program closed.
  Measured for the window and, on Windows 11, for the command line while a tfg
  command was running. A portable package carries no script, so the description
  is where this is said.
- **WinGet with the Windows installer goes ahead.** Measured on 2026-10-06 on
  Windows Server 2025, the installer of 0.4.0 over 0.3.0 while a tfg command was
  running: WinGet said "Restart your PC to finish installation." and exited 0,
  the run went on, and a new tfg was the new version at once - the restart only
  removes the old copy, as the installer section below says. Not measured with
  the window open, so the window's description still asks to close it first.

## The Windows installer

`msi/tfg-setup.wxs.in` is the source of `tfg-setup_<version>_windows_amd64.msi`,
a release asset of its own beside the zip archives. The window's WinGet package
installs it first. The Chocolatey packages and the command line's WinGet package
stay on the zips. `.github/scripts/build_msi.py` fills it and builds it with WiX 5.0.2,
from the two signed amd64 archives:

    python .github/scripts/build_msi.py --tag v0.5.0 --archives <folder> --out-dir <folder>

It is built by `sign_release.py`, after the card has signed the programs, and
signed the same way, with the product's name as the signature's description.
Windows shows that name when it asks an administrator to let the installer run,
and a string of digits without it. The signing script runs `build_msi.py` from the tree of the
tag, exported with `git archive`, so the installer is built from what was tagged
whatever the checkout stands on. `ci.yml` builds an unsigned one from the latest
release in every pull request and installs it on a Windows runner.

What it does, each line measured on Windows Server 2025 before it was written:

- **For the machine.** `Program Files\Testing Files Generator` with both programs,
  the software renderer in `opengl` and the three documents. The folder goes on
  the machine's `PATH` once, at the end, and comes off at uninstall. The software
  renderer stays one level down, because a library named `opengl32.dll` in a
  folder on `PATH` is one other programs would load.
- **The window in the Start menu,** started in the install folder. The window
  recognises its own folder and offers `tfg-out` in the home folder of whoever
  opened it. The shortcut cannot say that itself - Windows Installer expands a
  profile variable when it installs, in the installing account.
- **Nothing running is ended.** The Restart Manager is off. With it on, an
  upgrade while `tfg` ran waited thirty seconds, failed and closed the program
  anyway. With it off the file in use is set aside, the new version is in place
  at once and the upgrade answers 3010, a restart to remove the old copy. It has
  to be in the package from the first installer on, because an upgrade removes
  the old version under the OLD package's properties. Started with a double
  click, Windows Installer asks first. It names the open window and offers
  Cancel, Retry and Ignore. With the window closed before going on, the upgrade
  needs no restart.
- **One entry in Programs and Features.** A rebuild of the same version replaces
  the first build, and an older version is refused with a sentence.
- **No extension, no custom action, no dialogs of WiX's own**, so nothing but our
  files and Windows Installer's own tables goes into the package.

The `UpgradeCode` in `build_msi.py` is the product's identity for good. Every
machine finds the version it has through it, so a new one would leave the old
install in place beside the new one - a guard pins it. A release candidate, a tag
with a hyphen, gets no installer: Windows Installer reads only the three numbers
of a version. `build_msi.py` refuses one, and refuses two archives that hold
different copies of one file, an archive missing a program, a name that leads
out of the folder, and a WiX of another version.

## Submitting

Submitting is a person's step and stays one. Nothing here is wired into a
release: a package is published under the project's name to a feed somebody
else moderates.

1. The release is published and its `verify-SHA256SUMS.txt` is the file you
   render from, with the installer's line.
2. Render, then `winget validate` both WinGet folders and `choco pack` both
   nuspecs.
3. Install, run and remove every package on a machine you can break. A fresh
   WinGet install of the window takes the installer and puts the window in the
   Start menu, `--scope user` takes the archive, and an install of the archive
   is upgraded as the archive.
4. Chocolatey: `choco push` with the maintainer's API key. WinGet: one pull
   request per package against `microsoft/winget-pkgs`, with the three files under
   `manifests/d/DonislawDev/TestingFilesGenerator/<version>/` and
   `manifests/d/DonislawDev/TestingFilesGenerator/CLI/<version>/`.

A published release asset is never replaced. Every package version names its
archive by address and checksum, so a replaced file breaks every install of it.
