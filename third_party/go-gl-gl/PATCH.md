# A copy of github.com/go-gl/gl with one change

This directory is `github.com/go-gl/gl` at version
`v0.0.0-20260331235117-4566fea9a276`, module sum
`h1:IO5P06Pcj9K04d+l4nrf3c2U56+dAotIFG6u4P1wAHI=`, reduced to the two
packages this project imports, with one change to one of them. The `go.mod`
at the root of the repository points the module path here with a `replace`
directive, so every build takes the binding from this directory.

## What was changed

`v2.1/gl/procaddr.go` and `v2.1/gl/package.go` no longer carry the line
`#cgo !gles2,windows LDFLAGS: -lopengl32`, and the Windows branch of
`GlowGetProcAddress` in `procaddr.go` looks `wglGetProcAddress` up with
`GetProcAddress` after `LoadLibraryA("opengl32.dll")` instead of calling it
as an imported symbol. Nothing else differs from the published module. A
guard in `internal/guard`, `TestTheOpenGLBindingIsThePinnedModulePlusExactlyThePatch`,
downloads the pinned version, checks its sum against the one above and holds
every file in this copy to the published one plus exactly this change.

## Why

With the import, the executable names `opengl32.dll` in its import table and
the loader maps the system's copy before any code of ours runs. Windows hands
a library already mapped under a name to every later request for that name,
so a software renderer loaded from a path afterwards can never be the
`opengl32.dll` the toolkit finds. Measured on 2026-09-17: the window binary
built from the published module carries the import, the one built from this
copy carries none, and on a machine with a working driver the window opens
the same way from both.

Without the import, the first request for `opengl32.dll` is the one the
windowing library makes by name when it creates the context, and by then the
program has had its chance to load a renderer of its own choosing. That is
what lets the window fall back to a software renderer on a machine whose
driver offers no OpenGL, which is the reason this copy exists.

## What was left as it was

`v3.1/gles2` is carried unchanged. No release build links it, but the
toolkit imports it on Linux for arm64, and the test build for that platform
has to keep compiling.

The lookup by name in the patched function goes through the same search
order the windowing library already uses for the same file, so it opens no
door that was not open before.

## Licence

MIT, as published. The `LICENSE` file beside this one is the module's own,
carried unchanged.
