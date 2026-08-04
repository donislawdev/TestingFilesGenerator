# Third party notices

Testing Files Generator is Copyright (C) 2026 DonislawDev and is licensed under
the GNU General Public License, version 3. See [LICENSE](LICENSE).

A compiled `tfg` binary also contains code from the projects below. Their
licences require that their copyright notices travel with any copy, including a
binary one, so this file is part of every release. Each is compatible with
GPL-3.0.

If you build from source you are not distributing their code and this file is
informational. If you hand somebody a built binary, hand them this file too.

---

## Go standard library and runtime

Copyright 2009 The Go Authors.

Licensed under the BSD 3-Clause "New" or "Revised" License. Every Go program
links the runtime and parts of the standard library, so this notice applies to
`tfg` on every platform. The full text ships with any Go distribution, in the
`LICENSE` file at the root of `GOROOT`, and is the same text reproduced under
`golang.org/x/text` below.

Source: <https://go.googlesource.com/go>

---

## github.com/goccy/go-yaml

Version 1.19.2. Reads the recipe file.

```
MIT License

Copyright (c) 2019 Masaaki Goshima

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

Source: <https://github.com/goccy/go-yaml>

---

## golang.org/x/text

Version 0.40.0. Supplies Unicode normalisation, used to decide whether two file
names are one name spelled two ways.

```
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

`golang.org/x/text` also ships a `PATENTS` file granting a patent licence on
the same terms as the Go project. Its text is at
<https://go.googlesource.com/text/+/refs/heads/master/PATENTS>.

Source: <https://go.googlesource.com/text>

---

## Modules named in the build graph but not linked

`go list -m all` also names `golang.org/x/mod`, `golang.org/x/sync` and
`golang.org/x/tools`. `golang.org/x/text` requires them for the tooling that
generates its Unicode tables. No code from them reaches a `tfg` binary. They
carry the same BSD 3-Clause licence from the Go Authors as the text reproduced
above.

---

## Not used

`tfg` embeds no fonts, images, sound or other third party assets. The pixel
font that draws the label on a generated image is drawn in this repository, in
`internal/format/imagelabel/font.go`, so that a font licence is not a question
this project has to answer. The filler vocabulary is a short list of ordinary
English words written for this project.
