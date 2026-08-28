# Security Policy

## Supported versions

Testing Files Generator has a single line of development. Security fixes are made
against the **latest release** and the `main` branch. Please confirm you can
reproduce a problem on the latest version before reporting it.

**Older releases receive nothing.** When a new version is published, the one before
it stops being supported that day: no security updates, no backports, no patched
builds. The supported version is whichever release is currently the latest, for as
long as it is the latest. There is no long-term support line and none is planned,
so the upgrade path for a security fix is always to move to the newest release.

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Use GitHub's private vulnerability reporting: open the
[Security tab](https://github.com/donislawdev/TestingFilesGenerator/security/advisories/new)
of this repository and choose **Report a vulnerability**. That keeps the report
private until a fix is available.

Please include:

- the version (`tfg version`) and your operating system,
- whether you used the command line (`tfg`) or the window (`tfg-gui`),
- a clear description and the smallest steps to reproduce, ideally one command
  line or one recipe,
- the impact you believe it has.

You can expect an initial response within 14 days or fewer. Once a fix is ready it
ships in the next release, and the advisory is published crediting the reporter
unless you prefer to stay anonymous.

## What this tool does, and what that means for scope

Testing Files Generator writes files. That is the whole product, so a report has to
be about it writing something other than what it was asked to.

**It makes no network connections of any kind.** No telemetry, no update check, no
cloud client, nothing downloaded while it runs. Measured rather than promised: the
command line binary does not link a network package at all. The window binary links
one because the graphical toolkit imports it, and nothing in this project calls it -
a guard refuses a network import inside our own window code, and the `Donate` button
hands a link to your browser rather than fetching anything.

**It never deletes anything it was not told about.** `tfg cleanup` removes exactly
the files a manifest lists and nothing else.

**It writes only inside the output directory.** A file name is a name rather than a
path, and the manifest name too. Paths that would leave the directory, including
through a symbolic link, are refused.

**The released binaries are not signed.** Signing is not set up, the release notes
say so, and your operating system will warn you. Verify a download against
`verify-SHA256SUMS.txt` from the same release.

### In scope

- A way to make the tool write or delete outside the directory it was given.
- A recipe or a manifest that makes it do something other than what it says.
- A crash that corrupts files that were already on disk.
- A way to make `verify` report a file as good when it does not match the manifest.

### Not in scope

- **The content of the files it generates.** Producing a file that some scanner
  dislikes is the purpose of a test file generator, not a vulnerability.
- **Filling a disk when asked to.** The tool refuses a run larger than the free
  space it can see, but a run that fits and then fills the disk did what you asked.
- **A file name your operating system will not store.** That is reported as a
  refusal, on purpose, and is a documented difference between systems.

## Secrets and permissions in this repository

**There are no repository secrets.** Measured on 2026-08-27: zero. Every workflow
runs on the per-job token GitHub issues for the run, and nothing else is stored
here.

**Access is scoped per workflow.** The main suite, the dependency review and the
release workflow all declare `contents: read` at the top, and the single job that
publishes a release raises `contents: write` on itself. The dependency review
could post a summary comment on a pull request, which would need
`pull-requests: write` - that scope is deliberately not granted, because a failing
check already says what the comment would.

The one workflow holding more is the one that publishes the website: `pages: write`
and `id-token: write`. It has a single job, and it runs only for a push to this
repository - a condition added on 2026-08-27 and held by a guard, because the event
it triggers on fires with this repository's permissions.

**Every action is pinned to a commit** rather than to a tag somebody else can
repoint, and a guard refuses any workflow that names one another way.

**Dependencies are checked on every pull request**, for known vulnerabilities and
for licences. A licence GitHub cannot identify blocks the pull request, because for
a GPL-3.0 project that ships a binary an unidentified licence is the one answer
nobody can act on.

## Code of conduct

Behaviour in this repository is covered by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
