"""The mutation entries. Each one breaks one behaviour on purpose and names
the guard that has to notice.

A green suite is not evidence. The evidence is the sentence "this test would be
red if I broke that", and the only way to get it is to break it on purpose.

An entry is five strings, in this order:

  - what breaks, in a sentence
  - the file, relative to the root of the repository
  - the exact text to find, which has to occur exactly once in that file
  - the text that takes its place
  - the name of the guard that has to go red

mutate.py next to this file runs them. The guard
TestEveryGuardIsEitherProvenByMutationOrListedAsNotProven reads this file too,
and holds every guard to being named here, proven by a probe, or written down
as not proven.
"""
MUTATIONS = [
    # --- the contributing guide is held to the same gates as the rest of the
    #     repository text (2026-09-07) ---
    (
        "the contributing guide tells somebody to build without the tag",
        "CONTRIBUTING.md",
        'go build -tags "$(cat .github/build-tags)" ./cmd/tfg',
        "go build ./cmd/tfg",
        "TestTheInstallInstructionsCarryTheBuildTags",
    ),
    (
        "the contributing guide names a Go version this module does not admit",
        "CONTRIBUTING.md",
        "Needs Go 1.27.3 or newer",
        "Needs Go 1.21 or newer",
        "TestTheBuildFloorIsThePinnedToolchainAndTheDocumentsSaySo",
    ),
    (
        "the contributing guide sends somebody to another project's issue tracker",
        "CONTRIBUTING.md",
        "https://github.com/donislawdev/TestingFilesGenerator/issues/new/choose",
        "https://github.com/donislawdev/QADataGenerator/issues/new/choose",
        "TestNothingThatDirectsAPersonPointsAtAnotherProject",
    ),
    (
        "the contributing guide picks up a semicolon",
        "CONTRIBUTING.md",
        "and nothing else for the command line tool.",
        "and nothing else for the command line tool;",
        "TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons",
    ),
    # --- the window says what the command line says about the manifest ceiling
    #     (2026-09-07, O184) ---
    (
        "the preview stops warning that the record will be too big to read back",
        "internal/gui/window/run.go",
        "r.say(append([]string{said}, manifestReachNote(res.Record)...)...)",
        "r.say(said)",
        "TestTheWindowSaysWhenItsManifestWillBeTooBigToReadBack",
    ),
    (
        "a finished run stops warning, so somebody who never pressed Preview is told nothing",
        "internal/gui/window/run.go",
        "said := append([]string{outcome}, manifestReachNote(saved.Size)...)",
        "said := []string{outcome}",
        "TestAFinishedRunInTheWindowSaysItsManifestIsTooBigToReadBack",
    ),
    (
        "the window warns about every run, which teaches people to skip the line",
        "internal/gui/window/runreport.go",
        "if !record.TooLarge {",
        "if false && !record.TooLarge {",
        "TestAnOrdinaryPreviewSaysNothingAboutTheManifestCeiling",
    ),
    (
        "the forecast of a run's record counts a hash one character short, so it drifts from what the run writes",
        "internal/engine/recordsize.go",
        "strings.Repeat(\"0\", 64)",
        "strings.Repeat(\"0\", 63)",
        "TestTheRecordARunIsToldBeforehandIsTheRecordItWrites",
    ),
    # --- verify says whose the other files are (2026-09-07, T8) ---
    (
        "the neighbour's files go back to being called extra",
        "internal/audit/audit.go",
        "if by, ok := neighbours.claimedBy[p]; ok {",
        "if by, ok := neighbours.claimedBy[p]; ok && false {",
        "TestVerifyNamesTheFilesOfAnotherRunRatherThanCallingThemExtra",
    ),
    (
        "the neighbour's record itself goes back to being called extra",
        "internal/audit/audit.go",
        "if neighbours.records[p] {",
        "if false && neighbours.records[p] {",
        "TestTheJSONReportCarriesTheOtherRunsFilesAndStillSaysItMatched",
    ),
    (
        "every file nobody claims is handed to the neighbour, so a polluted directory reads as a shared one",
        "internal/audit/audit.go",
        'return Extra, ""',
        'return AnotherRun, ""',
        "TestAStrayFileBesideAnotherRunsRecordIsStillExtra",
    ),
    (
        "the neighbour's files count as disagreements again, so a shared directory is permanently red",
        "internal/cli/verify.go",
        "if d.Kind != audit.AnotherRun {\n\t\t\tn++",
        "if d.Kind != audit.Extra {\n\t\t\tn++",
        "TestADirectorySharedWithAnotherRunStillMatchesThisManifest",
    ),
    (
        "nothing counts as a disagreement, so a missing file is called a match",
        "internal/cli/verify.go",
        "return n",
        "return 0",
        "TestARealDifferenceIsStillAMismatchInASharedDirectory",
    ),
    (
        "the attribution stops naming which run, so the reader is told somebody else's and not whose",
        "internal/audit/neighbours.go",
        "base := filepath.Base(rel)",
        'base := "a run"',
        "TestTheSentenceAboutAnotherRunNamesTheRecordThatClaimsTheFile",
    ),
    (
        "every unclaimed file is opened and parsed, which is the design that was measured out",
        "internal/audit/neighbours.go",
        'return strings.EqualFold(filepath.Ext(rel), ".json")',
        'return true || strings.EqualFold(filepath.Ext(rel), ".json")',
        "TestARecordUnderAnotherExtensionIsStillExtra",
    ),
    # --- one directory holds one run at a time (2026-09-07, T2) ---
    (
        "the check goes, so a second run walks into a directory another run is filling",
        "internal/engine/preflight.go",
        "if path := RunLockPath(opt.OutDir); exists(path) {",
        "if path := RunLockPath(opt.OutDir); false && exists(path) {",
        # Named at the dry run rather than at the real one, and the first
        # attempt got that wrong. The real run is protected twice - this check
        # and the claim itself - so removing one leaves the other, and the
        # guard for a refused run stayed green while the mutation was in. What
        # only this check covers is the preview: the claim comes after the dry
        # run has already returned, so without this a preview would promise a
        # run into a directory somebody else is filling.
        "TestADryRunNeitherTakesTheDirectoryNorIgnoresIt",
    ),
    (
        "the lock name carries the process id, so two runs never meet on it",
        "internal/engine/preflight.go",
        "return filepath.Join(outDir, core.RunLockName)",
        'return filepath.Join(outDir, fmt.Sprintf("%s%d", core.RunLockName, os.Getpid()))',
        "TestTheRunLockIsSpelledInOnePlace",
    ),
    (
        "verify calls our own run lock a file somebody else put there",
        "internal/audit/audit.go",
        "core.IsRunLockName(base)",
        "false && core.IsRunLockName(base)",
        "TestVerifyNamesTheRunLockAsOursRatherThanAsSomebodyElses",
    ),
    (
        "the refusal stops naming the file to remove, so a stale lock is a dead end",
        "internal/engine/errors.go",
        "core.A(\"Dir\", core.Shown(e.Dir)), core.A(\"Lock\", core.Shown(e.Path)))",
        "core.A(\"Dir\", core.Shown(e.Dir)), core.A(\"Lock\", \"\"))",
        "TestASecondRunIntoADirectoryARunIsHoldingIsRefusedBeforeItWritesAnything",
    ),
    # --- a preset renders a recipe that still means what it says (2026-09-06) ---
    (
        "the whitelist on a spread value goes, so a space reaches a file name",
        "internal/preset/sizeboundaries.go",
        'if bad := firstUnusable(piece); bad != "" {',
        'if bad := firstUnusable(piece); false && bad != "" {',
        "TestEveryPresetEndingHasTheCodeItShould",
    ),
    # --- two one word findings, and a third the guard for them found (2026-09-06) ---
    (
        "the window loads uxtheme by name again, through the ordinary search order",
        "internal/gui/darkmenus_windows.go",
        "uxtheme, err := syscall.LoadDLL(path)",
        "_ = path; uxtheme, err := syscall.LoadDLL(\"uxtheme.dll\")",
        "TestALibraryIsOnlyLoadedLazilyWhenTheSystemAlreadyHasIt",
    ),
    (
        "the text catalogue is loaded a second time, so the localiser is written twice",
        "internal/gui/run_cgo.go",
        "if err := text.LoadBuiltIn(speak); err != nil {",
        "if err := text.LoadBuiltIn(speak); err != nil || text.LoadBuiltIn(speak) != nil {",
        "TestTheTextCatalogueIsLoadedFromOnePlaceOnly",
    ),
    # --- the documents say what the tool does (2026-09-06) ---
    (
        "the security policy goes back to calling the binaries unsigned",
        "SECURITY.md",
        "**The Windows and macOS downloads are signed**",
        "**The released binaries are not signed.** Signing is not set up.**",
        "TestTheSecurityPolicySaysWhatTheReleaseDoesAboutSigning",
    ),
    (
        "the locking setting stops saying where the key comes from",
        "internal/format/archive/archive.go",
        "from the run seed, so the same recipe gives the same archive on any machine, and the password ",
        "with the password you gave, and the password ",
        "TestTheLockingSettingSaysItIsAFixtureRatherThanProtection",
    ),
    # --- a credential appears where it was meant to and nowhere else (2026-09-06) ---
    (
        "the recorded command repeats the password again",
        "internal/cli/cli.go",
        'return prefix + name + "=" + redactedValue',
        "return arg",
        "TestTheRecordedCommandDoesNotRepeatACredential",
    ),
    (
        "the archive password stops being declared a credential",
        "internal/format/archive/archive.go",
        "Secret: true,",
        "Secret: false,",
        "TestTheRecordAndTheRegistryAgreeOnWhatIsACredential",
    ),
    # --- the build tags reach every build, including the ones people type (2026-09-06) ---
    (
        "an analysis job stops passing the build tags, so it reads a build we do not ship",
        ".github/workflows/ci.yml",
        'go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -tags "$(cat .github/build-tags)" ./...',
        "go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...",
        "TestEveryWorkflowCommandThatBuildsUsPassesTheBuildTags",
    ),
    (
        "the refusal stops being part of any build, so an untagged build compiles again",
        "internal/format/avif/requiretag.go",
        "//go:build !noasm",
        "//go:build ignore",
        "TestABuildWithoutTheBuildTagsRefusesAndSaysWhy",
    ),
    (
        "the install instructions drop the tag again",
        "README.md",
        "go install -tags noasm github.com/donislawdev/TestingFilesGenerator/cmd/tfg@latest",
        "go install github.com/donislawdev/TestingFilesGenerator/cmd/tfg@latest",
        "TestTheInstallInstructionsCarryTheBuildTags",
    ),
    # --- a manifest too big to read back says so where a script reads (2026-09-06) ---
    (
        "the report stops saying the manifest cannot be read back",
        "internal/engine/engine.go",
        "m.Summary.TooLargeToReadBack = record.TooLarge",
        "_ = record.TooLarge",
        "TestAManifestTooBigToReadBackSaysSoInTheReportAScriptReads",
    ),
    (
        "every manifest carries the ceiling field, changing the bytes of all of them",
        "internal/manifest/manifest.go",
        'TooLargeToReadBack bool `json:"too_large_to_read_back,omitempty"`',
        'TooLargeToReadBack bool `json:"too_large_to_read_back"`',
        "TestAnOrdinaryRunSaysNothingAboutTheCeilingInItsReport",
    ),
    # --- a stopped run says so in our own words (2026-09-06) ---
    (
        "a cancelled run goes back to printing Go's own vocabulary",
        "internal/cli/errors.go",
        'return "stopped before it finished."',
        "return err.Error()",
        "TestAStoppedRunSaysSoInOurOwnWords",
    ),
    (
        "a run that ran out of time reads the same as one somebody cancelled",
        "internal/cli/errors.go",
        'return "stopped before it finished, because the time allowed for it ran out."',
        "return err.Error()",
        "TestARunThatRanOutOfTimeSaysThatRatherThanThatItWasCancelled",
    ),
    # --- a size refusal names the key the recipe carries (2026-09-06) ---
    (
        "a range target is addressed to a size key it does not have",
        "internal/engine/errors.go",
        "if setting == core.SettingSize && t != nil && t.Range.Used {",
        "if setting == core.SettingSize && t != nil && t.Range.Used && false {",
        "TestASizeRefusalIsAddressedToTheKeyTheRecipeCarries",
    ),
    (
        "every size refusal is addressed to size-range, including ordinary targets",
        "internal/engine/errors.go",
        "if setting == core.SettingSize && t != nil && t.Range.Used {",
        "if setting == core.SettingSize && t != nil {",
        "TestASizeRefusalIsAddressedToTheKeyTheRecipeCarries",
    ),
    # --- notes are grouped by what they say, not one line per file (2026-09-06) ---
    (
        "every file gets its own note line again, the way it did until 2026-09-06",
        "internal/manifest/manifest.go",
        '\tg := n.byDetail[detail]\n\tif g == nil {\n\t\tg = &noteGroup{}\n\t\tn.byDetail[detail] = g\n\t\tn.order = append(n.order, detail)\n\t}',
        '\tg := n.byDetail[detail+name]\n\tif g == nil {\n\t\tg = &noteGroup{}\n\t\tn.byDetail[detail+name] = g\n\t\tn.order = append(n.order, detail)\n\t}',
        "TestNotesAreGroupedByWhatTheySayRatherThanOneLinePerFile",
    ),
    (
        "a note about one file stops leading with that file's name",
        "internal/manifest/manifest.go",
        "\tif g.Count == 1 {\n",
        "\tif g.Count == 1 && false {\n",
        "TestANoteAboutOneFileStillNamesThatFileFirst",
    ),
    (
        "a grouped note stops saying how many files it is not naming",
        "internal/manifest/manifest.go",
        "\tif hidden := g.Count - len(g.First); hidden > 0 {",
        "\tif hidden := g.Count - len(g.First); hidden > 0 && false {",
        "TestNotesAreGroupedByWhatTheySayRatherThanOneLinePerFile",
    ),
    (
        "the unnamed remainder loses its plural",
        "internal/manifest/manifest.go",
        "core.A(\"Hidden\", core.SaysN(\"manifest.Files\", \"%d file\", \"%d files\", core.A(\"Count\", hidden))))",
        "core.A(\"Hidden\", core.Says(\"manifest.Files\", \"%d files\", core.A(\"Count\", hidden))))",
        "TestTheUnnamedRemainderCarriesTheRightNoun",
    ),
    # --- both leftover markers are recognised, and each is spelled once (2026-09-06) ---
    (
        "the reading side forgets the second marker, so a half written record is an extra again",
        "internal/audit/audit.go",
        # One line of the case rather than the whole of it: the list grew a
        # third marker on 2026-09-07 and moved onto three lines.
        "core.IsWritingName(base)",
        "core.IsWritingName(base) && false",
        "TestAHalfWrittenRecordIsNamedForWhatItIs",
    ),
    (
        "a half written record gets the sentence written for a half written generated file",
        "internal/audit/audit.go",
        "if core.IsWritingName(filepath.Base(d.Path)) {",
        "if core.IsWritingName(filepath.Base(d.Path)) && false {",
        "TestAHalfWrittenRecordIsNamedForWhatItIs",
    ),
    # --- the build floor is the compiler CI runs (2026-09-06) ---
    (
        "the README goes back to naming a floor the module no longer admits",
        "README.md",
        "**From source.** Needs Go 1.27.3 or newer, and nothing else:",
        "**From source.** Needs Go 1.26.5 or newer, and nothing else:",
        "TestTheBuildFloorIsThePinnedToolchainAndTheDocumentsSaySo",
    ),
    (
        "CI builds with a compiler older than the floor go.mod admits",
        ".github/workflows/ci.yml",
        'GO_VERSION: "1.27.3"',
        'GO_VERSION: "1.26.8"',
        "TestTheBuildFloorIsThePinnedToolchainAndTheDocumentsSaySo",
    ),
    # --- the write path claims every name it writes under (2026-09-06) ---
    (
        "the claim believes os.Stat, so a link pointing at nothing is refused in the system's words rather than as a name in use",
        "internal/core/createnew.go",
        "_, lookErr := os.Lstat(path)",
        "_, lookErr := os.Stat(path)",
        "TestCreateNewCreatesOnlyWhenTheNameIsFree",
    ),
    (
        "the claim stops being exclusive, so it writes through whatever holds the name",
        "internal/core/createnew.go",
        "os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm",
        "os.O_WRONLY|os.O_CREATE, perm",
        "TestCreateNewCreatesOnlyWhenTheNameIsFree",
    ),
    (
        "recipe fmt -w writes its temporary copy with a plain create again",
        "internal/core/replace.go",
        "f, err := CreateNew(path, mode)",
        "f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)",
        "TestAHeldTemporaryNameStopsTheWriteRatherThanGoingThroughIt",
    ),
    (
        "a writer creates its own file again, so the one claim stops being the only door",
        "internal/engine/parallel.go",
        "fh, err := core.CreateNew(tmp, 0o666)",
        "_ = core.CreateNew; fh, err := os.Create(tmp)",
        "TestEveryFileThisToolWritesIsCreatedThroughOneClaim",
    ),
    (
        "an Office package stops being read back by an independent library, so a slide that is not a slide passes",
        "internal/oracle/strict.py",
        '    if key == "pptx":',
        '    if False and key == "pptx":',
        "TestTheStructuralCheckerRefusesAPackageThatIsNotADocument",
    ),
    (
        "the structural checker stops asking whether an entry carries its sizes after the data",
        "internal/oracle/strict.py",
        "        if info.flag_bits & 0x08:",
        "        if False and info.flag_bits & 0x08:",
        "TestTheStructuralCheckerRefusesAPackageThatIsNotADocument",
    ),
    (
        "the structural checker stops holding the archive comment to the size a reader accepts",
        "internal/oracle/strict.py",
        "    if len(z.comment) > OPC_COMMENT_LIMIT:",
        "    if False and len(z.comment) > OPC_COMMENT_LIMIT:",
        "TestTheStructuralCheckerRefusesAPackageThatIsNotADocument",
    ),
    (
        "an Office package goes back to writing its sizes after the data instead of in the header",
        "internal/format/opc/opc.go",
        "entry, err := zw.CreateRaw(head)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif withPadding {",
        "entry, err := zw.CreateHeader(head)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif withPadding {",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a part of an Office package stops declaring its own checksum, so a reader refuses the whole file",
        "internal/format/opc/opc.go",
        "\tentry, err := zw.CreateRaw(head)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = entry.Write(payload)",
        "\tentry, err := zw.CreateHeader(head)\n\tif err != nil {\n\t\treturn err\n\t}\n\t_, err = entry.Write(payload)",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "the filler text of an Office document stops being a fixed width, so the floor moves with the seed",
        "internal/format/opc/opc.go",
        "\treturn Fixed(b.String(), LineWidth)",
        "\treturn b.String()",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the part carrying the label is compressed again, so its length stops being its bytes",
        "internal/format/docx/docx.go",
        "Body: document(paragraphs, seed, label), Store: true},",
        "Body: document(paragraphs, seed, label)},",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the padding part loses its content type, which the packaging specification requires",
        "internal/format/opc/opc.go",
        "\tb.WriteString(`<Default Extension=\"` + fillerExtension + `\" ContentType=\"application/octet-stream\"/>`)",
        "",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "the smallest Office file is worked out for one label state instead of both",
        "internal/format/xlsx/xlsx.go",
        'for _, label := range []string{"", core.Label("xlsx", 0, 0)} {',
        'for _, label := range []string{""} {',
        "TestSizeBelowTheMinimumIsRefused",
    ),
    (
        "a BMP states a file size that is not the size of the file",
        "internal/format/bmp/bmp.go",
        'binary.LittleEndian.PutUint32(head[2:6], uint32(total))',
        'binary.LittleEndian.PutUint32(head[2:6], uint32(total-1))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a BMP says its pixels start past where they really do",
        "internal/format/bmp/bmp.go",
        'binary.LittleEndian.PutUint32(head[10:14], uint32(headers+m.gap))',
        'binary.LittleEndian.PutUint32(head[10:14], uint32(headers+m.gap+1))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a BMP grows its picture one row past what the request pays for",
        "internal/format/bmp/bmp.go",
        'h := avail / int64(stride(width))',
        'h := avail/int64(stride(width)) + 1',
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a GIF cuts its comment into too few sub blocks, so one of them overflows its length byte",
        "internal/format/gif/gif.go",
        'blocks = (delta - emptyComment + subBlockCost - 1) / subBlockCost',
        'blocks = (delta - emptyComment) / subBlockCost',
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a GIF accepts a size no arrangement of comments can produce",
        "internal/format/gif/gif.go",
        'case delta < smallestCarryingComment:',
        'case delta < smallestCarryingComment && false:',
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a JPEG pads with an application segment instead of a comment, which every reader still accepts",
        "internal/format/jpg/jpg.go",
        'head := [4]byte{0xFF, 0xFE, 0, 0}',
        'head := [4]byte{0xFF, 0xEF, 0, 0}',
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a JPEG spreads its padding over the segments unevenly, so the file misses the size by a few bytes",
        "internal/format/jpg/jpg.go",
        'if int64(i) < extra {',
        'if int64(i) <= extra {',
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a JPEG takes its floor from one seed instead of the worst of them, so the minimum it prints walks",
        "internal/format/jpg/jpg.go",
        'for off := uint64(0); off < 256; off++ {',
        'for off := uint64(0); off < 1; off++ {',
        "TestTheMinimumThisToolPrintsStandsWhateverTheSeedIs",
    ),
    (
        "an icon embeds a bitmap whose header forgets that the height counts the mask too",
        "internal/format/ico/ico.go",
        'binary.LittleEndian.PutUint32(out[8:12], uint32(int32(h*2)))',
        'binary.LittleEndian.PutUint32(out[8:12], uint32(int32(h)))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "an icon points at its picture without counting the padding in front of it",
        "internal/format/ico/ico.go",
        'binary.LittleEndian.PutUint32(e[12:16], uint32(int64(preamble)+m.gap))',
        'binary.LittleEndian.PutUint32(e[12:16], uint32(preamble))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "an icon understates how long its picture is",
        "internal/format/ico/ico.go",
        'binary.LittleEndian.PutUint32(e[8:12], uint32(len(m.image)))',
        'binary.LittleEndian.PutUint32(e[8:12], uint32(len(m.image)-1))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "the generate screen registers the opening format's settings before the screen is built, so they end up registered twice",
        "internal/gui/window/generate.go",
        # Widened on 2026-09-09. The screen gained a second guard on the same
        # flag when the damage menu arrived, so the bare condition matched
        # twice and staleness.py called it ambiguous - a mutation that could
        # land in either of two places proves neither. The line under it is
        # what makes this the format one.
        'if !g.ready {\n\t\treturn\n\t}\n\tg.propBox.RemoveAll()',
        'if !g.ready && false {\n\t\treturn\n\t}\n\tg.propBox.RemoveAll()',
        "TestEveryRegisteredFieldCanBeMarkedAndUnmarked",
    ),
    (
        "the engine gains a capability nobody classified for the window",
        "internal/format/imagedim/imagedim.go",
        '\tSettingHeight = "height"',
        '\tSettingHeight = "depth"',
        "TestEveryEngineCapabilityIsClassifiedForBothSurfaces",
    ),
    (
        "a recipe key stops being counted as something the window has to reach",
        "internal/recipe/keys.go",
        'if tag == "" || tag == "-" {',
        'if tag == "" || tag == "-" || tag == "locale" {',
        "TestEveryEngineCapabilityIsClassifiedForBothSurfaces",
    ),
    (
        # The control that the real entry point still works on the machine that
        # built the binary. Aimed at the INTERPRETER lookup rather than at
        # either of the two paths to the script, and that choice is the whole
        # point: both paths resolve here, so breaking one leaves the other
        # answering and the guard stays green while proving nothing.
        "the structural checker looks for an interpreter nobody installed",
        "internal/oracle/oracle.go",
        'python, ok := inPath("python")()',
        'python, ok := inPath("python-not-here")()',
        "TestTheStructuralCheckerIsAvailableWhereItWasBuilt",
    ),
    (
        # The second way in to the checker script, which is the only one a
        # binary built somewhere else can use. Aimed at the name, so the walk
        # runs to the top of the disk and finds nothing - which is what the
        # container did for months while every structural check reported a skip.
        "the walk for the structural checker looks for a name that is not there",
        "internal/oracle/oracle.go",
        'filepath.Join(dir, "internal", "oracle", "strict.py")',
        'filepath.Join(dir, "internal", "oracle", "strict.pyc")',
        "TestTheStructuralCheckerIsFoundByABinaryBuiltSomewhereElse",
    ),
    (
        # The same guard from the other side: the walk stops after the directory
        # it starts in, so it never climbs to where the script actually is. This
        # is the state the code was in before the walk existed at all.
        "the walk for the structural checker stops where it starts",
        "internal/oracle/oracle.go",
        "if up == dir {",
        "if up == dir || true {",
        "TestTheStructuralCheckerIsFoundByABinaryBuiltSomewhereElse",
    ),
    (
        "the version in the file properties drifts from the one the program reports",
        "internal/version/version.go",
        'const Version = "0.5.0"',
        'const Version = "0.1.0"',
        "TestBothBinariesCarryTheirVersionAndAuthor",
    ),
    (
        "the undocumented ordinal for dark menus stops resolving",
        "internal/gui/darkmenus_windows.go",
        "uxthemeSetPreferredAppMode = 135",
        "uxthemeSetPreferredAppMode = 1350",
        "TestTheProgramAsksWindowsForDarkMenus",
    ),
    (
        "the Donate button leads nowhere",
        "internal/gui/window/open.go",
        "h.OpenLink(text.SupportURL)",
        'h.OpenLink("")',
        "TestTheDonateButtonOpensTheSupportPage",
    ),
    (
        "the window and the sponsor file stop agreeing where the money goes",
        "internal/gui/text/screens.go",
        'const SupportURL = "https://donislawdev.com/support/"',
        'const SupportURL = "https://donislawdev.com/donate/"',
        "TestTheSponsorFileAndTheWindowAgreeWhereTheMoneyGoes",
    ),
    (
        "a screen leaves the window and the list the guards walk does not notice",
        "internal/gui/window/open.go",
        "&parts.Tab{ID: tabRecipe, Text: text.TabRecipe(), Content: rec.Object()},",
        '&parts.Tab{ID: tabRecipe, Text: text.TabRecipe() + "!", Content: rec.Object()},',
        "TestEveryTabInTheWindowIsOnTheListGuardsWalk",
    ),
    (
        "every batch registers its boxes under the same address, so batch 2 marks batch 1",
        "internal/gui/window/recipe.go",
        "return recipe.TargetAddress(index+1, setting)",
        "return recipe.TargetAddress(1, setting)",
        "TestEveryRefusalAboutABatchMarksTheBoxOfThatBatch",
    ),
    (
        "removing a batch removes nothing, so the ones below it never renumber",
        "internal/gui/window/recipe.go",
        "r.batches = append(r.batches[:index], r.batches[index+1:]...)",
        "r.batches = append(r.batches[:index], r.batches[index:]...)",
        "TestAddingAndRemovingBatchesKeepsWhatWasTypedAndRenumbersTheRest",
    ),
    (
        "the last batch can be removed, leaving a form that can produce nothing",
        "internal/gui/window/recipe.go",
        "if len(r.batches) <= 1 && !r.base.carriesTheRun() {",
        "if len(r.batches) <= 1 && false {",
        "TestTheLastBatchCannotBeRemoved",
    ),
    (
        "the recipe screen stops offering a box for the class of a batch",
        "internal/gui/window/recipefolds.go",
        "add(recipe.KeyGroup, text.FieldGroup(), text.HintGroup(),",
        "add(recipe.KeyGroup+\"-gone\", text.FieldGroup(), text.HintGroup(),",
        "TestTheRecipeScreenDrawsABoxForEverySettingItOffers",
    ),
    (
        "a run from the recipe screen ignores the manifest name it was given",
        "internal/gui/window/recipe.go",
        "ManifestName: manifestName(rec.Output.Manifest),",
        "ManifestName: engine.DefaultManifestName,",
        "TestWhatIsTypedOnTheRecipeScreenIsWhatGetsWritten",
    ),
    (
        "composing stops refusing characters a document cannot carry",
        "internal/recipe/compose.go",
        "if err := refuseUnwritable(d); err != nil {",
        "if err := refuseUnwritable(d); err != nil && false {",
        "TestEveryCharacterIsEitherCarriedThroughAComposedRecipeOrRefused",
    ),
    (
        "the unwritable class loses DEL, which is as invisible as the rest",
        "internal/recipe/compose.go",
        "if r < 0x20 || r == 0x7F {",
        "if r < 0x20 {",
        "TestEveryCharacterIsEitherCarriedThroughAComposedRecipeOrRefused",
    ),
    (
        "a composed target quietly loses its group",
        "internal/recipe/compose.go",
        'add("group", t.Group)',
        'add("group", "")',
        "TestARecipeComposedFromTypedTextSurvivesAnythingTypedIntoIt",
    ),
    (
        "a composed target never carries an expectation, so a bad one is never refused",
        "internal/recipe/compose.go",
        "if e := expectationEntry(t); e != nil {",
        "if e := expectationEntry(t); e != nil && false {",
        "TestAnEmptyOrHalfFilledScreenComposesIntoARefusalThatNamesSomething",
    ),
    (
        "a refusal stops naming the setting and points at the target as a whole",
        "internal/recipe/errors.go",
        'return s.key + "." + setting',
        "return s.key",
        "TestEveryRecipeRefusalSaysWhichSettingItIsAbout",
    ),
    (
        "a refusal carries no address at all, as none of them did before 2026-08-18",
        "internal/recipe/errors.go",
        "Problem{What: what, Why: why, Fix: fix, At: at}",
        'Problem{What: what, Why: why, Fix: fix, At: ""}',
        "TestEveryRecipeRefusalSaysWhichSettingItIsAbout",
    ),
    (
        "the address of a target becomes its id, so the box for a missing id has none",
        "internal/recipe/errors.go",
        "\t\ts.says = core.Says(\"recipe.Target\", \"target %q\", core.A(\"Target\", id))\n",
        "\t\ts.says = core.Says(\"recipe.Target\", \"target %q\", core.A(\"Target\", id))\n\t\ts.key = id\n",
        "TestEveryRecipeRefusalSaysWhichSettingItIsAbout",
    ),
    (
        "a refused recipe stops opening into its problems, so all of it lands in one spot",
        "internal/recipe/errors.go",
        "func (e *ValidationError) Unwrap() []error {",
        "func (e *ValidationError) unwrapWithdrawn() []error {",
        "TestARefusedRecipeArrivesAsItsProblemsEachKnowingItsField",
    ),
    (
        "a recipe problem stops saying which setting it is about",
        "internal/recipe/errors.go",
        "func (p Problem) AboutSetting() string { return p.At }",
        'func (p Problem) AboutSetting() string { return "" }',
        "TestARefusedRecipeArrivesAsItsProblemsEachKnowingItsField",
    ),
    (
        "the machine readable report drops the address on the way out",
        "internal/cli/recipecmd.go",
        "Fix: p.Fix.String(), At: p.At})",
        "Fix: p.Fix.String()})",
        "TestTheMachineReadableReportCarriesTheAddressOfEachRefusal",
    ),
    (
        "the machine readable list goes back to carrying names alone",
        "internal/cli/formats.go",
        "Unit: p.Unit, Choices: p.Choices, Default: p.Default, Detail: p.Detail,",
        "Unit: \"\", Choices: nil, Default: \"\", Detail: \"\",",
        "TestTheMachineReadableListCarriesTheWholeDeclaration",
    ),
    (
        "a property value is no longer checked against its declaration",
        "internal/format/format.go",
        "if why := p.Allows(raw); !why.IsZero() {",
        "if why := p.Allows(raw); !why.IsZero() && false {",
        "TestABadPropertyValueIsTheCallersMistakeNotOurs",
    ),
    (
        "a value the format cannot take is reported as a bug in the tool",
        "internal/cli/errors.go",
        "var badValue *format.PropertyValueError",
        "var badValue *format.NotAContainerError",
        "TestABadPropertyValueIsTheCallersMistakeNotOurs",
    ),
    (
        "tfg formats ignores the format it was asked about, as it did until 2026-08-03",
        "internal/cli/formats.go",
        "if wanted != \"\" {",
        "if wanted != \"\" && false {",
        "TestFormatsAnswersAboutOneFormat",
    ),
    (
        "the declared range stops bounding the value",
        "internal/format/format.go",
        "if p.Min != 0 || p.Max != 0 {\n\t\tif n < p.Min || n > p.Max {",
        "if p.Min != 0 || p.Max != 0 {\n\t\tif (n < p.Min || n > p.Max) && false {",
        "TestTheRefusalComesFromTheDeclaration",
    ),
    (
        "a size starts counting in thousands like a boundary",
        "internal/core/size.go",
        'case "m", "mb", "mi", "mib":',
        'case "mi", "mib":',
        "TestASizeKeepsCountingIn1024s",
    ),
    (
        "every group is named by boundary role, not just a boundary set",
        "internal/engine/engine.go",
        "case t.BoundaryLimit > 0 && index < len(boundaryRoles):",
        "case index < len(boundaryRoles):",
        "TestAnOrdinaryGroupIsStillNumbered",
    ),
    (
        "a boundary set goes back to plain numbering",
        "internal/engine/engine.go",
        "case t.BoundaryLimit > 0 && index < len(boundaryRoles):",
        "case t.BoundaryLimit > 0 && index < len(boundaryRoles) && false:",
        "TestABoundarySetSaysWhichFileIsWhich",
    ),
    (
        "the boundary roles are put on in the wrong order",
        "internal/engine/engine.go",
        'var boundaryRoles = [3]string{"under_1b", "at_limit", "over_1b"}',
        'var boundaryRoles = [3]string{"at_limit", "under_1b", "over_1b"}',
        "TestABoundarySetSaysWhichFileIsWhich",
    ),
    (
        "a name template stops winning over the boundary roles",
        "internal/engine/engine.go",
        "	case tmpl != \"\":",
        "	case tmpl != \"\" && false:",
        "TestABoundarySetStillObeysANameTemplate",
    ),
    (
        "progress is reported only between files, so a big file stays silent",
        "internal/engine/parallel.go",
        "if c.report != nil {",
        "if c.report != nil && false {",
        "TestProgressArrivesWhileOneFileIsStillBeingWritten",
    ),
    (
        "the running total restarts at every file",
        "internal/engine/parallel.go",
        "\tg.bytesDone += bytes\n\tg.workDone += work\n\tg.say()\n}\n\n// finished",
        "\tg.bytesDone = bytes\n\tg.workDone += work\n\tg.say()\n}\n\n// finished",
        "TestProgressCountsTheWholeRunNotOneFile",
    ),
    (
        "the bar is drawn even when nothing is watching",
        "internal/cli/progress.go",
        "if err != nil || info.Mode()&os.ModeCharDevice == 0 {",
        "if err != nil || info.Mode()&os.ModeCharDevice == 0 && false {",
        "TestProgressStaysOffWhenNothingIsWatching",
    ),
    (
        "the run protects a manifest name other than the one it writes",
        # Aimed at manifestNameOf rather than at either check, and that is the
        # whole point. Two layers protect this name - the collision check in
        # preflight and the claim that follows it - and each covers the other,
        # so switching off either one on its own leaves the guard green and
        # proves nothing. Both ask this function what the name is, so it is the
        # one place a single mutation can reach both. Measured 2026-08-04:
        # disabling the collision check alone was NOT CAUGHT.
        "internal/engine/preflight.go",
        "	return opt.ManifestName\n}",
        '	return opt.ManifestName + ".other"\n}',
        "TestASecondRunNeverWritesOverAnEarlierManifest",
    ),
    (
        "a dry run skips the checks and promises a run that would be refused",
        "internal/engine/engine.go",
        "if err := preflight(ctx, files, opt, record); err != nil {",
        "if err := preflight(ctx, files, opt, record); err != nil && !opt.DryRun {",
        "TestADryRunReachesTheSameVerdictAsTheRealRun",
    ),
    (
        "a run bigger than the disk is allowed to start",
        "internal/engine/preflight.go",
        "if available < needed {",
        "if available < needed && false {",
        "TestADryRunSeesAFullDiskAndStillWritesNothing",
    ),
    (
        "asking for help is not recognised, so it falls through as a mistake",
        "internal/cli/cli.go",
        '\t\tcase "-h", "-help", "--help":\n\t\t\treturn true',
        '\t\tcase "-h", "-help", "--help":\n\t\t\treturn false',
        "TestAskingForHelpIsNotAMistake",
    ),
    (
        "the top level answers a request for help on the complaint channel",
        "internal/cli/commands.go",
        # io.Discard rather than errOut, which was the first version and came
        # back BROKEN: the help entry names its error writer "_", so errOut is
        # not a thing there and the mutant did not compile. Sending the answer
        # nowhere breaks the same promise - it is not on stdout - and builds.
        "\t\t\t\tusage(out)\n\t\t\t\treturn ExitOK",
        "\t\t\t\tusage(io.Discard)\n\t\t\t\treturn ExitOK",
        "TestTheTopLevelTellsAskingApartFromTyping",
    ),
    (
        "a mistyped command is called an option, as it was in the source project",
        "internal/cli/cli.go",
        'unknown command %q',
        'unknown option %q',
        "TestAMistypedCommandIsStillAMistake",
    ),
    (
        "a flag nobody wrote is treated as written",
        "internal/cli/generate.go",
        'if given["seed"] {',
        "if true {",
        "TestAFlagNobodyWroteDoesNotOverrideTheRecipe",
    ),
    (
        "a flag that was written is ignored",
        "internal/cli/generate.go",
        'if given["seed"] {',
        "if false {",
        "TestAFlagThatWasWrittenWinsAndIsRecorded",
    ),
    (
        "the formatter drops comments",
        "internal/recipe/canonical.go",
        "parser.ParseBytes(src, parser.ParseComments)",
        "parser.ParseBytes(src, 0)",
        "TestFormattingARecipeSettlesAndKeepsWhatAPersonWrote",
    ),
    (
        "the declared expectation is dropped on the way to the manifest",
        # Aimed at a line with no column alignment in it. A pattern inside an
        # aligned struct literal goes stale the moment gofmt moves a column,
        # and a stale pattern is reported as SKIP rather than passing quietly.
        "internal/engine/engine.go",
        "switch f.Target.Expected {",
        'switch "" {',
        "TestAnExpectationFromTheRecipeReachesTheManifestUnchanged",
    ),
    (
        "validation stops at the first problem",
        "internal/recipe/errors.go",
        "func (p *problems) add(at string, what, why, fix core.Said) {\n",
        "func (p *problems) add(at string, what, why, fix core.Said) {\n\tif len(p.list) > 0 {\n\t\treturn\n\t}\n",
        "TestAllTheProblemsInARecipeAreReportedAtOnce",
    ),
    (
        "an invalid recipe is generated from anyway",
        "internal/recipe/recipe.go",
        "if err := p.err(); err != nil {\n\t\treturn nil, err\n\t}",
        "if err := p.err(); err != nil {\n\t\t_ = err\n\t}",
        "TestARecipeThatFailsValidationWritesNothing",
    ),
    (
        "an unknown key is accepted in silence",
        "internal/recipe/recipe.go",
        "yaml.NodeToValue(body, raw, yaml.Strict())",
        "yaml.NodeToValue(body, raw)",
        "TestATypoInARecipeKeyIsRefused",
    ),
    (
        "a file holding two documents is run anyway",
        "internal/recipe/canonical.go",
        "if n := recipesIn(f); n > 1 {",
        "if n := recipesIn(f); n > 99 {",
        "TestAFileHoldingTwoRecipesIsRefusedRatherThanHalfRun",
    ),
    (
        # The regression this replaced: signal.NotifyContext does not say which
        # signal arrived, so every stop came out as Ctrl+C and the 143 in the
        # frozen table was unreachable. CI could not tell a timeout from a
        # person cancelling.
        # Untouchable rule 4. Padding octets to three digits made the line
        # length trivial to predict and produced addresses no real log holds -
        # and a leading zero is read as octal by some parsers. Caught by the
        # structural checker written to the format, not to our output.
        "log addresses go back to padded octets",
        "internal/format/logfile/address.go",
        "dst = strconv.AppendInt(dst, int64(a.parts[i]), 10)",
        "dst = strconv.AppendInt(append(dst, '0'), int64(a.parts[i]), 10)",
        "TestEveryLineOfALogIsAWholeEntry",
    ),
    (
        # A preview and a real run have to be distinguishable in the report,
        # or a script cannot tell whether anything was deleted.
        "the cleanup report stops saying whether it applied anything",
        "internal/cli/cleanup.go",
        "report := cleanupReport{Manifest: run.path, Directory: run.dir, Applied: true}",
        "report := cleanupReport{Manifest: run.path, Directory: run.dir, Applied: false}",
        "TestCleanupAndValidateReportAsJSON",
    ),
    (
        # Collapsing the problems into one string is exactly what the JSON
        # form exists to avoid - RC7 reports them all at once and a script
        # should not have to split a sentence back apart.
        "validate reports its problems as one blob again",
        "internal/cli/recipecmd.go",
        "for _, p := range invalid.Problems {",
        "for _, p := range invalid.Problems[:1] {",
        "TestCleanupAndValidateReportAsJSON",
    ),
    (
        "a stop by signal is reported as Ctrl+C again",
        "internal/cli/cli.go",
        "if s == syscall.SIGTERM {",
        "if s == syscall.SIGTERM && false {",
        "TestAStopBySignalIsToldApartFromCtrlC",
    ),
    (
        "two endings collapse onto one exit code",
        "internal/cli/cli.go",
        "ExitTerminated = 143",
        "ExitTerminated = 130",
        "TestTheExitCodeConstantsMatchTheFrozenTable",
    ),
    (
        # The whole point of MD over TXT is the structure. A document that
        # stopped emitting blocks would still be the right size and would
        # still be valid Markdown, so size and determinism see nothing.
        "markdown stops emitting structure and becomes plain prose",
        "internal/format/md/md.go",
        "buf = appendBlock(buf, rng)",
        "buf = buf[:mark]",
        "TestAMarkdownDocumentCarriesRealStructure",
    ),
    (
        # A log is read line by line. Cutting the last line to land the byte
        # count leaves a half entry - which looks like a real log mid rotation
        # and is exactly why it has to be caught rather than eyeballed.
        # The padded request path is where a line can stop being a whole entry
        # now. A newline inside it splits the closing entry into several lines,
        # most of which are not entries, and the total byte count does not move
        # at all. Measured 2026-08-01: the size and determinism guards both
        # stayed green while this was in.
        #
        # This replaced an earlier entry that cut the closing line short. That
        # one went stale when the record loop moved into core.FillRecords,
        # which refuses a record of the wrong length before it reaches the file
        # - so the defect stopped being a wrong file and became an error.
        "a newline inside the padded request path splits the closing log entry",
        "internal/format/logfile/shapes.go",
        'dst = append(dst, " /"...)',
        'dst = append(dst, " /\\n"...)',
        "TestEveryLineOfALogIsAWholeEntry",
    ),
    (
        # The address is no longer fixed width - it is written the way a real
        # log writes it - so the line length depends on which address came up.
        # Getting that arithmetic wrong misses the byte count on the closing
        # entry, which is what this catches.
        "the varying width of the address stops being counted",
        "internal/format/logfile/shapes.go",
        "int64(addr.length()) + tail.width(len(agent))",
        "int64(12) + tail.width(len(agent))",
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        # SVG is case sensitive, so a lowercase viewbox is simply not the
        # attribute - and the substitution is the SAME LENGTH, so the file does
        # not move by one byte. Measured: size and determinism both stayed
        # green, and the drawing still opens.
        #
        # The pattern moved on 2026-09-08, when the root element stopped being
        # a constant with 800 and 600 written into it and became a function of
        # the two dimensions. Same mutation, same guard, new place to make it.
        "the svg viewBox loses its capital letter and stops being an attribute",
        "internal/format/svgfile/svg.go",
        'viewBox="0 0 %d %d"',
        'viewbox="0 0 %d %d"',
        "TestAnSVGDrawingCarriesRealShapes",
    ),
    (
        # A list left open runs to the end of the document, where it collides
        # with the tag that closes the body. HTML readers recover from this by
        # design, which is exactly why it needs a guard written to the format
        # rather than a guard that asks whether a parser complained.
        "an html list is never closed, so the document stops being balanced",
        "internal/format/htmlfile/blocks.go",
        'return append(dst, m[ulClose]...)',
        'return dst',
        "TestAnHTMLDocumentIsBalancedAndCarriesBlocks",
    ),
    (
        # A bare ampersand in text. The tolerant reader swallows it without a
        # word and the size stays exactly right.
        "the html ampersand stops being escaped",
        "internal/format/htmlfile/blocks.go",
        'dst = append(dst, " &amp; "...)',
        'dst = append(dst, " & "...)',
        "TestAnHTMLDocumentIsBalancedAndCarriesBlocks",
    ),
    (
        # A run that produced nothing still writes a manifest, and somebody's
        # script reads it. Left uninitialised the file list renders as null
        # while every empty collection beside it renders as {}, so a reader
        # looping over the entries meets a value that is not a list. Found by
        # the Ctrl+C probe on 2026-08-01, which crashed on exactly that.
        "the manifest file list renders as null instead of an empty list",
        "internal/manifest/manifest.go",
        "Files: []File{},",
        "Files: nil,",
        "TestAManifestWithNoFilesRendersAnEmptyListNotNull",
    ),
    (
        # Untouchable rule 2: seeds are derived, never drawn from a shared
        # stream. This is the shape that rule exists to forbid - the file seed
        # starts depending on how many files the run has already planned, so
        # adding one target at the top shifts the bytes of everything below it
        # and every hash in somebody's CI goes red at once.
        #
        # The regression surface claimed this was proven by mutation and it was
        # not: no entry here named a locality test. Found 2026-08-01 by the
        # coverage guard, not by reading.
        "the file seed comes from a stream shared by the whole run",
        "internal/engine/plantarget.go",
        "core.FileSeed(targetSeed, idx)",
        "core.FileSeed(targetSeed, idx+len(pl.out))",
        # Reordering only. Checked one test at a time: the same substitution
        # leaves the other three locality guards green, because a target
        # appended at the end shifts nothing before it. Naming a test that
        # stays green would have reported this mutation as uncaught.
        "TestReorderingTargetsChangesNothing",
    ),
    (
        # Adding a target anywhere changes every other target, because the seed
        # starts depending on how many targets the run has.
        "the number of targets reaches the seed of each of them",
        "internal/engine/engine.go",
        # totalFiles was tried here first and proved nothing: it is added to
        # after this line, so the targets already planned keep the seed they
        # had and a target appended at the end changes nothing before it. The
        # count of targets is the value that reaches every one of them.
        "targetSeed := core.TargetSeed(opt.Seed, t.ID)",
        "targetSeed := core.TargetSeed(opt.Seed+int64(len(targets)), t.ID)",
        "TestAddingATargetLeavesTheOthersUntouched",
    ),
    (
        # Raising a count rewrites the files that were already there, which is
        # the failure somebody meets after generating a fixture set twice.
        "the file seed depends on how many files the target holds",
        "internal/engine/plantarget.go",
        "core.FileSeed(targetSeed, idx)",
        "core.FileSeed(targetSeed, idx+len(t.Sizes))",
        "TestRaisingTheCountLeavesTheEarlierFilesIdentical",
    ),
    (
        # The sentence an operating system hands back is opaque whatever
        # language it is in, and Go falls back to the language of the machine
        # when the English resource is missing. Ours replaces it and carries the
        # number, which is the part that means the same thing everywhere.
        "the system's own sentence reaches the terminal instead of ours",
        "internal/cli/errors.go",
        'osText != "" && strings.Contains(full, osText)',
        'osText != "" && strings.Contains(full, osText) && false',
        "TestASystemErrorReachesTheUserInOurWords",
    ),
    (
        # Pointing a command at a directory is the mistake somebody makes, and
        # left alone it surfaces as "Incorrect function." from Windows.
        "a command pointed at a directory falls through to the system error",
        "internal/cli/errors.go",
        "if err != nil || !info.IsDir() {",
        "if err != nil || !info.IsDir() || true {",
        "TestPointingACommandAtADirectorySaysSo",
    ),
    (
        # The other half of untouchable rule 2: the identity of a target is what
        # its seed is derived from. Drop it and two targets produce the same
        # files, so renaming one changes nothing when it has to change
        # everything of that target.
        "the target id stops reaching the seed, so every target draws the same",
        "internal/engine/engine.go",
        "targetSeed := core.TargetSeed(opt.Seed, t.ID)",
        "targetSeed := core.TargetSeed(opt.Seed, t.ID[:0])",
        "TestChangingATargetIdChangesThatTargetOnly",
    ),
    (
        # The shared rule for every record based format: whole records, then a
        # closing record built to the byte. Break the loop and the padding value
        # swallows the whole file - one row with a hundred kilobyte field. The
        # size stays exact, the run stays repeatable, and the file still parses.
        # Measured 2026-08-01: this is what topples the Python csv reader at its
        # default settings, which is the reader a tester has nearest.
        "the padding swallows the file instead of going through whole records",
        "internal/core/records.go",
        "if remaining-int64(len(buf)-mark) < shortest {",
        "if remaining-int64(len(buf)-mark) < shortest || true {",
        "TestEveryRowOfACSVHasTheSameColumns",
    ),
    (
        # The description column is quoted because it carries the separator.
        # Stop asking and it never gets quotes, so the separators inside it
        # become column separators - a ragged row, which is a defect this tool
        # offers deliberately elsewhere. Size and determinism both stay green,
        # measured. Since 2026-09-03 the question is asked in one place, so
        # this is where the answer is broken.
        "the csv description stops being quoted, so its separators split the row",
        "internal/format/csvfile/dialect.go",
        "return bytes.IndexByte(description, sep) >= 0",
        "return bytes.IndexByte(description, sep) >= 0 && false",
        "TestEveryRowOfACSVHasTheSameColumns",
    ),
    (
        # The written out rule. This is the sentence that actually went stale:
        # it never says the word "formats", so a rule asking for a number next
        # to that word would not have seen it. Measured while writing the
        # guard - the first version missed exactly this one.
        "the site spells out a format count instead of counting it",
        "web/content/en/site.json",
        "All {{ .Facts.FormatCount }} open in the software that owns them.",
        "All twenty open in the software that owns them.",
        "TestTheSiteNeverCountsTheFormatsInItsOwnWords",
    ),
    (
        # The digit rule, which is the shape of the older defect: a page title
        # that read "PDF, DOCX, PNG, ZIP and 16 more formats". A digit has to
        # stand next to the word, or "--size 2mb" would redden it.
        "the site types a format count as a number instead of counting it",
        "web/content/en/site.json",
        "It produces real files of {{ .Facts.FormatCount }} formats",
        "It produces real files of 26 formats",
        "TestTheSiteNeverCountsTheFormatsInItsOwnWords",
    ),
    (
        # YAML forbids a tab in indentation outright. Three spaces and a tab is
        # the same four bytes, so the size guard, the determinism guard and the
        # pinned hashes would all have to be read as saying nothing - which is
        # the whole shape this entry exists to catch. Verified by hand before
        # it was written down: the oracle and the structural check both redden.
        "yaml indents with a tab, which is the same four bytes and not YAML",
        "internal/format/yamlfile/yaml.go",
        'openName = "\\n    name: "',
        'openName = "\\n   \\tname: "',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # The record a builder throws away takes its number with it, and without
        # the decrement the ids skip. Measured by hand: python-toml ACCEPTS that
        # file and reports its records, so the reference tool is not what
        # catches this - the hand written structural check is.
        "a toml record number is not handed back, so the ids skip",
        "internal/format/tomlfile/toml.go",
        "func (r *records) Discard() { r.next-- }",
        "func (r *records) Discard() {}",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # The same defect on the other format, and pointed at the other layer on
        # purpose: one entry proving the structural check and one proving the
        # guard that reads the bytes directly. A thing protected twice does not
        # move when one layer is mutated, which is how NOT CAUGHT ends up being
        # a sentence about the guard rather than about the code.
        "a yaml record number is not handed back, so the ids skip",
        "internal/format/yamlfile/yaml.go",
        "func (r *records) Discard() { r.next-- }",
        "func (r *records) Discard() {}",
        "TestRecordNumbersRunFromOneWithoutAGap",
    ),
    (
        # TOML says a document is UTF-8 and both readers refuse one that opens
        # with a byte order mark. The label comment is where a mark would ride
        # in, and the size stays exact because the filler absorbs it.
        "a toml document opens with a byte order mark, which no reader accepts",
        "internal/format/tomlfile/toml.go",
        'line := "# " + core.Label("toml", r.Bytes, r.Seed) + "\\n"',
        'line := "\\ufeff# " + core.Label("toml", r.Bytes, r.Seed) + "\\n"',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # A JSON fixture exists so a parser meets every value type. Drop one and
        # the document is still valid JSON of exactly the right size - only a
        # guard that knows which types were promised notices.
        "a json record loses its null, so one value type never reaches a parser",
        "internal/format/jsonfile/json.go",
        "dst = append(dst, r.s.nullPart...)",
        "dst = append(dst, r.s.nullPart[:0]...)",
        "TestAJSONDocumentIsAnArrayOfRecordsWithEveryType",
    ),
    (
        # A bare ampersand is the classic way a generated document stops being
        # well formed while staying exactly the right size. Both the hand
        # written structural check and the strict decoder refuse it.
        "the xml ampersand stops being escaped, so the document is not well formed",
        "internal/format/xmlfile/xml.go",
        'dst = append(dst, "&amp;"...)',
        "dst = append(dst, '&')",
        "TestAnXMLDocumentIsWellFormedAndCarriesRecords",
    ),
    (
        "contains is ignored, so the archive holds the default instead",
        "internal/format/archive/archive.go",
        "if r.Contains != nil {\n\t\treturn fromContains(id, r, stated, door)",
        "if false {\n\t\treturn fromContains(id, r, stated, door)",
        "TestTheMembersOfAnArchiveAreRealFilesOfEveryFormatAsked",
    ),
    (
        # Two groups of one format both starting at 0001 means the second
        # replaces the first, and the archive quietly holds fewer files than
        # the recipe asked for.
        "members are numbered per group, so two groups of a format collide",
        "internal/format/zip/children.go",
        "numbered[g.Format]++",
        "numbered[g.Format] = i + 1",
        "TestTheMembersOfAnArchiveAreRealFilesOfEveryFormatAsked",
    ),
    (
        "a format that holds nothing accepts contains anyway",
        "internal/engine/engine.go",
        "if len(t.Contains) > 0 && desc.Container == nil {",
        "if false {",
        "TestContentsThatCannotBeHonouredAreRefusedWithTheRightCode",
    ),
    (
        # The size stated in the recipe has to win, or an archive comes out a
        # different size than the recipe says - the one promise this tool makes.
        "the contents override a size the recipe stated",
        "internal/recipe/target.go",
        "case rt.Size != nil:",
        "case rt.Size != nil && rt.Contains == nil:",
        "TestASizeStatedBesideContentsWinsAndPaddingClosesTheGap",
    ),
    (
        # An empty contains says "hold nothing" and no contains says nothing at
        # all. Collapsing the two turns a legal empty archive into a refusal.
        "an empty contains is read as no contains",
        "internal/format/archive/archive.go",
        "if r.Contains != nil {\n\t\treturn fromContains(id, r, stated, door)",
        "if len(r.Contains) > 0 {\n\t\treturn fromContains(id, r, stated, door)",
        "TestAnArchiveHoldingNothingIsLegal",
    ),
    (
        # Untouchable rule 3, D11. The neighbouring stdlib golden file catches
        # the ground moving under us. This catches us moving.
        "an archive timestamp shifts, so every ZIP anybody generated changes",
        "internal/format/zip/zip.go",
        "time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)",
        "time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        # The label is embedded in the file, so its wording is part of the
        # bytes. The guard pins txt with the label and without it, and only
        # the first of the two may notice this.
        "the self describing label is reworded",
        "internal/core/label.go",
        '"tfg - %s - %d B - seed %s"',
        '"tfg %s %d B seed %s"',
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        # Untouchable rule 7. If this one ever goes green, the tool deletes
        # files nobody listed, on somebody else's disk, with no undo.
        "cleanup walks the directory instead of the manifest",
        "internal/audit/cleanup.go",
        "claimed := Claimed(m)",
        "claimed := m.Files",
        "TestCleanupDoesNotRemoveAFileTheRunNeverPutOnTheDisk",
    ),
    (
        # The condition is kept and disabled rather than replaced by a
        # constant. Replacing it orphans the flag variable, the tree stops
        # compiling, and the run reads as caught while proving nothing - which
        # is exactly what the build check now refuses.
        "cleanup deletes without being asked",
        "internal/cli/cleanup.go",
        "if !*yes {",
        "if !*yes && false {",
        "TestCleanupWithoutYesRemovesNothing",
    ),
    (
        "cleanup removes a file whose content it does not recognise",
        "internal/audit/cleanup.go",
        "return c.Disposition == Ready || (force && c.Disposition == Changed)",
        "return true",
        "TestCleanupLeavesAChangedFileAloneUnlessForced",
    ),
    (
        # The direction that breaks a script rather than a disk: a file that
        # was already gone counted as a leftover makes the second run fail.
        "a file already gone counts as left behind",
        "internal/audit/cleanup.go",
        "Blocked: c.Disposition != Absent,",
        "Blocked: true,",
        "TestCleanupRunTwiceIsNotAnError",
    ),
    (
        # Removing a name is not destroying a file, and a hard link is the case
        # that tells the two apart. Emptying the file first leaves the name gone
        # either way, so every other cleanup test stays green while somebody's
        # data disappears through a second name the tool never looked at.
        "cleanup empties a file instead of only unlinking it",
        "internal/audit/cleanup.go",
        "if err := os.Remove(full); err != nil {",
        "if err := os.Truncate(full, 0); err != nil {",
        "TestCleanupUnlinksOurNameWithoutDestroyingWhatIsBehindAHardLink",
    ),
    (
        # Nothing read the history until 2026-08-04. Commit messages, branch
        # names and every version of every file are as public as the code, and
        # deleting a file does not take it back.
        "the scan for private content stops looking for an account path",
        "internal/guard/privatecontent_test.go",
        '{"a path inside somebody\'s Windows profile", regexp.MustCompile(`(?i)[A-Za-z]:[\\\\/]{1,2}Users[\\\\/]`)},',
        '{"a path inside somebody\'s Windows profile", regexp.MustCompile(`(?i)ZZnevermatchesZZ`)},',
        "TestTheScanForPrivateContentCanActuallyFindIt",
    ),
    (
        # The other direction. A scan that refuses everything is a scan
        # somebody switches off, and then it guards nothing at all.
        "the scan starts calling an ordinary install path private",
        "internal/guard/privatecontent_test.go",
        '"user": true, "you": true, "me": true, "name": true,',
        '"nobody": true, "you": true, "me": true, "name": true,',
        "TestTheScanForPrivateContentLeavesOrdinaryTextAlone",
    ),
    (
        # The scan has to reach the whole repository. Run from the directory
        # the test lives in, git ls-files answers about that directory alone -
        # measured 2026-08-04, it read a handful of files and called the tree
        # clean.
        "the file scan reads only the directory the test sits in",
        "internal/guard/privatecontent_test.go",
        "cmd.Dir = repoRoot(t)",
        "cmd.Dir = \".\"",
        "TestNoTrackedFileCarriesPrivateContent",
    ),
    (
        # Asking git for less leaves the scan looking at forty character hex
        # strings, in which it will never find anything - green, and blind.
        "the history is read without the subjects and bodies",
        "internal/guard/privatecontent_test.go",
        'gitOutput(t, "log", "--all", "--format=%H%n%s%n%b")',
        'gitOutput(t, "log", "--all", "--format=%H")',
        "TestNoCommitMessageCarriesPrivateContent",
    ),
    (
        "the ref scan is pointed at something that is not a list of names",
        "internal/guard/privatecontent_test.go",
        'gitOutput(t, "for-each-ref", "--format=%(refname)")',
        'gitOutput(t, "for-each-ref", "--format=%(objecttype)")',
        "TestNoBranchOrTagNameCarriesPrivateContent",
    ),
    (
        # The rule used to be about dashes and semicolons alone, and two things
        # walked past it: the red circle this project marks warnings with in its
        # own documents, which had reached a comment in internal/core, and a
        # typographic ellipsis in internal/guard. Both found 2026-08-04 by
        # pulling all 5532 comment lines out of the tree and looking for
        # anything above ASCII - a question no rule was asking.
        "a symbol from the project's own notes reaches a comment that ships",
        "internal/core/records.go",
        "// The cut lands on a character boundary, not on a byte.",
        "// The cut lands on a character boundary \U0001f534, not on a byte.",
        "TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons",
    ),
    (
        # Asking is not a mistake. The same class as --help ending with 2: the
        # exit code guard stays green either way, because it asks whether a
        # code is in the table and not whether it is the right one.
        "asking for the licence is treated as a mistyped command",
        "internal/cli/commands.go",
        'Aliases: []string{"--license", "--licence", "licence"},',
        'Aliases: []string{"--license", "--licence"},',
        "TestAskingForTheLicenceIsAnsweredAndEndsWithZero",
    ),
    (
        # The notice could shrink to the name of the licence and still look
        # like an answer. The sentence about generated files is the reason
        # somebody runs this rather than reading LICENSE.
        # Moved from internal/cli to internal/version on 2026-08-05, when the
        # window started showing the same notice. Two surfaces on one layer
        # cannot import each other, so the text went below both of them.
        "the licence notice stops saying that generated files are the user's",
        "internal/version/version.go",
        "The files you generate are yours.",
        "The files you generate are covered too.",
        "TestAskingForTheLicenceIsAnsweredAndEndsWithZero",
    ),
    (
        # A notice naming a document that does not ship reads as an answer and
        # is not one.
        "the notices file stops being part of the repository",
        "internal/guard/licence_test.go",
        '[]string{"LICENSE", "THIRD-PARTY-NOTICES.md"}',
        '[]string{"LICENSE", "NO-SUCH-NOTICES.md"}',
        "TestTheFileTheLicenceNoticePointsAtExists",
    ),
    (
        "the manifest goes even though a file it lists is still there",
        "internal/cli/cleanup.go",
        "if blocked > 0 {\n\t\tfor i, p := range run.record {",
        "if false {\n\t\tfor i, p := range run.record {",
        "TestCleanupRemovesTheManifestOnlyWhenAskedAndOnlyWhenEverythingElseWent",
    ),
    (
        "the manifest is removed without being asked for",
        "internal/cli/cleanup.go",
        "if *withManifest {",
        "if *withManifest || true {",
        "TestCleanupRemovesWhatTheManifestListsAndNothingElse",
    ),
    (
        "verify never looks at content, so an edited file passes",
        "internal/audit/audit.go",
        "if got.sum != f.Hashes.SHA256 {",
        "if false {",
        "TestVerifyCatchesAMissingAnExtraAndAChangedFile",
    ),
    (
        "verify ignores files nobody asked for",
        "internal/audit/audit.go",
        "diffs = append(diffs, Difference{Kind: kind, Path: p, Want: want})",
        "_, _ = kind, want",
        "TestVerifyCatchesAMissingAnExtraAndAChangedFile",
    ),
    (
        "verify counts a file that was never written as missing",
        # A failed entry describes a file the run reported as not produced.
        # Claiming it would turn the manifest's own honesty into a false alarm.
        "internal/audit/audit.go",
        "if f.Failed || !f.Materialized {",
        "if false {",
        "TestVerifyDoesNotGoLookingForFilesTheRunSaidItNeverWrote",
    ),
    (
        "verify treats the manifest itself as a file nobody asked for",
        "internal/audit/audit.go",
        "if seen[p] || filepath.Base(p) == skip || (m.Run.Instructions != \"\" && filepath.Base(p) == m.Run.Instructions) {",
        "if seen[p] || (m.Run.Instructions != \"\" && filepath.Base(p) == m.Run.Instructions) {",
        "TestVerifyDoesNotReportTheManifestAsAnExtraFile",
    ),
    (
        "a manifest from an unreadable schema is acted on anyway",
        "internal/manifest/manifest.go",
        "if got != want {",
        "if got != want && false {",
        "TestVerifyRefusesAManifestFromASchemaItCannotRead",
    ),
    (
        # Found on 2026-08-03, after the textual containment check was already
        # in place. "jn/VICTIM.txt" holds no climb, so the text passed - and a
        # junction called "jn" took cleanup --yes --force one directory up,
        # exit code 0. docs/SECURITY.md section 2.4 carried the rule already,
        # marked NOT VERIFIED, and it was right.
        "a link inside the directory is followed out of it again",
        "internal/audit/audit.go",
        "if b.Escapes(full) {",
        "if b.Escapes(full) && false {",
        "TestCleanupDoesNotFollowALinkOutOfTheDirectory",
    ),
    (
        # Named against the linked-directory guard rather than against an
        # escape guard, and the first attempt got that wrong. Resolving the
        # base only changes the answer when the output directory is itself a
        # link: without it, Rel compares a link against its target and every
        # ordinary file in that directory reads as an escape. The escape guards
        # use a plain directory, so they stayed green and the runner said so.
        #
        # Re-pointed on 2026-08-20 when the boundary stopped being resolved per
        # entry. The named test had to change with it, and the reason is worth
        # keeping: the quick reading now answers a plain file in a linked
        # directory without ever consulting the resolved boundary, so
        # TestADirectoryReachedThroughALinkStillWorks stayed green under this
        # mutation. The resolved boundary is only load bearing where the quick
        # reading declines - a redirection below a directory that is itself
        # reached through a link - and that is the shape the guard below builds.
        "the directory is compared as written instead of as it resolves",
        "internal/core/filename.go",
        "	r, err := resolveAsFarAsItExists(dir)",
        "	r, err := filepath.Abs(dir)",
        "TestALinkInsideTheDirectoryPointingBackInsideItIsNotAnEscape",
    ),
    (
        # Measured on Windows 2026-08-03: two targets asking for report.txt and
        # REPORT.TXT ended with exit 0, one file on the disk and two entries in
        # the manifest, and verify then failed on the tool's own output.
        "two names differing only in case stop counting as one",
        "internal/core/foldname.go",
        "return cases.Fold().String(norm.NFC.String(name))",
        "return norm.NFC.String(name) + cases.Fold().String(\"\")",
        "TestTwoFilesDifferingOnlyInCaseAreRefused",
    ),
    (
        # The other direction. Comparing without case is not comparing without
        # looking - report_a and REPORT_B are two files everywhere.
        "the collision check starts refusing names that are not the same",
        "internal/core/foldname.go",
        "return cases.Fold().String(norm.NFC.String(name))",
        "return cases.Fold().String(norm.NFC.String(name[:1]))",
        "TestNamesThatDifferByMoreThanCaseStillWork",
    ),
    (
        # Measured on macOS 26.5.1, APFS, arm64, 2026-08-04: two targets asking
        # for cafe.txt with the accent composed and decomposed ended with exit
        # 0, one file on the disk, two entries in the manifest, and verify then
        # failing on the tool's own output. The same shape as the case defect,
        # reached by spelling a letter the other way.
        #
        # The replacement keeps norm referenced. Dropping the call outright
        # orphans the import, Go refuses to build, and a mutation that does not
        # compile proves nothing - it reports BROKEN.
        "two spellings of one accented name stop counting as one",
        "internal/core/foldname.go",
        "return cases.Fold().String(norm.NFC.String(name))",
        "return cases.Fold().String(name) + norm.NFC.String(\"\")",
        "TestTwoNamesDifferingOnlyInUnicodeNormalizationAreRefused",
    ),
    (
        # The other direction. An accent is ordinary input - only a pair that
        # collides is a problem, and refusing every accented name would satisfy
        # the rule above while making the tool worse.
        "the collision check starts refusing accented names that do not collide",
        "internal/core/foldname.go",
        "return cases.Fold().String(norm.NFC.String(name))",
        "return cases.Fold().String(norm.NFC.String(name[:1]))",
        "TestAccentedNamesThatDoNotCollideStillWork",
    ),
    (
        # Measured on 2026-08-04: a recipe saved as cp1250 was accepted and the
        # file arrived with four replacement characters in its name.
        "a recipe in any encoding is read as best it can be",
        "internal/recipe/recipe.go",
        "if !utf8.Valid(src) {",
        "if !utf8.Valid(src) && false {",
        "TestARecipeThatIsNotUTF8IsRefused",
    ),
    (
        # And the other way: refusing everything would satisfy that guard.
        "the encoding check refuses files that are perfectly good UTF-8",
        "internal/recipe/recipe.go",
        "if !utf8.Valid(src) {",
        "if utf8.Valid(src) {",
        "TestARecipeInUTF8KeepsItsLetters",
    ),
    (
        # Measured on 2026-08-04: "a:b.txt" accepted on Linux, refused on
        # Windows, from one recipe - and "AB:c.txt" left an empty file called
        # AB behind while reporting the file as not produced.
        "a colon in a name is left to the local machine to interpret",
        "internal/engine/filename.go",
        'case strings.Contains(name, ":"):',
        'case strings.Contains(name, ":") && false:',
        "TestANameIsJudgedTheSameWayOnEverySystem",
    ),
    (
        "the manifest records a local time, so two machines describe one run differently",
        "internal/manifest/manifest.go",
        "GeneratedAt:     time.Now().UTC().Format(time.RFC3339),",
        "GeneratedAt:     time.Now().Format(time.RFC3339),",
        "TestTheRecordedTimeDoesNotDependOnTheLocalZone",
    ),
    (
        # O45. The rule binding width and height lived in the generator and in
        # a sentence of prose, so "tfg formats png" offered 20000 by 20000 and
        # the run refused the pair. A window builds its fields from the machine
        # readable list, which had nothing in it to build from.
        "the joint limit stops reaching anything that is not prose",
        "internal/cli/formats.go",
        "Label: string(d.Label), Properties: props, JointLimits: jointsOf(d),",
        "Label: string(d.Label), Properties: props,",
        "TestTheJointLimitReachesMachineOutput",
    ),
    (
        # Found on 2026-08-03 by asking for exactly what the tool printed.
        # pdf, wav and zip advertised a minimum they then refused, because the
        # registry number is the structural floor with no label and the label
        # is on unless it is turned off.
        "the minimum on screen goes back to a number the tool refuses",
        "internal/cli/formats.go",
        "\t\td.ID, d.Fidelity, d.Determinism, core.ExactBytes(smallestAccepted(d)))",
        "\t\td.ID, d.Fidelity, d.Determinism, core.ExactBytes(d.MinBytes))",
        "TestTheMinimumThisToolPrintsIsASizeItAccepts",
    ),
    (
        # The other direction. A number well above the real floor would satisfy
        # the guard above while telling somebody they cannot ask for a file
        # they can have.
        "the minimum on screen sits above the smallest size that works",
        "internal/format/format.go",
        "\t\tif _, err := d.Generator.Plan(r); err == nil {\n\t\t\treturn size\n\t\t}",
        "\t\tif _, err := d.Generator.Plan(r); err == nil {\n\t\t\treturn size + 1\n\t\t}",
        "TestTheMinimumThisToolPrintsIsTheSmallestItAccepts",
    ),
    (
        # Found on 2026-08-03 with tools/probes/fidelity-sweep.py. The
        # reference tool only ever saw one size per format - MinBytes plus
        # 300 KB - so SVG at exactly its minimum rendered to a single flat
        # colour and every guard stayed green. Dropping the minimum back puts
        # that file back in reach.
        "the smallest drawing a format will make paints nothing again",
        "internal/format/svgfile/svg.go",
        "return int64(len(declaration)+len(rootOpen(w, h))) + s.Shortest() + 1",
        "return int64(len(declaration)+len(rootOpen(w, h))) + s.Shortest()",
        "TestEveryFormatSurvivesItsReferenceToolAtItsSmallest",
    ),
    # Not listed: weakening the shape counter in TestAnSVGDrawingCarriesRealShapes
    # back to counting an empty text element. The runner reported NOT CAUGHT and
    # was right - every size that guard now walks holds real text, so the
    # weakening changes no verdict. The input that would expose it was 193 B,
    # and that size stopped existing with the fix above. Same shape as O25: the
    # runner proving a repair rather than finding a hole.
    (
        # Found by audit on 2026-08-03. "--size 1e5" quietly meant 100000 bytes
        # while a recipe refuses "1_000" for the opposite reason, and the
        # difference fell out of ParseFloat being permissive rather than out of
        # a decision.
        "a size goes back to accepting any spelling ParseFloat takes",
        "internal/core/size.go",
        "if !plainDecimal(digits) {",
        "if !plainDecimal(digits) && false {",
        "TestASizeIsWrittenTheWayAPersonWritesIt",
    ),
    (
        "the recorded command splits an argument that holds a space",
        "internal/cli/cli.go",
        "out = append(out, quoteArg(withoutSecret(a, afterSet, secrets)))",
        "out = append(out, withoutSecret(a, afterSet, secrets))",
        "TestTheRecordedCommandCanBeRunAgain",
    ),
    (
        # The temporary file is created with os.Create, which truncates, and
        # the collision check only ever looked at the name a file ends up with.
        "the name a file is written through is not checked for a collision",
        "internal/engine/preflight.go",
        "if path := tempPathFor(opt.OutDir, f.Name); isTaken(taken, path, tempNameFor(f.Name)) {",
        "if path := tempPathFor(opt.OutDir, f.Name); isTaken(taken, path, tempNameFor(f.Name)) && false {",
        "TestAFileUnderTheTemporaryNameIsNotWrittenOver",
    ),
    (
        # "tfg generate --json | head" is the ordinary way this happens, and
        # the error was dropped on the floor.
        "a manifest that never reached standard output is reported as a success",
        "internal/cli/generate.go",
        "if _, err := out.Write(buf.Bytes()); err != nil {",
        "if _, err := out.Write(buf.Bytes()); err != nil && false {",
        "TestAManifestThatCannotReachStandardOutputIsNotASuccess",
    ),
    (
        "a CI job loses its ceiling and can hold a runner for six hours",
        ".github/workflows/ci.yml",
        "    timeout-minutes: 30",
        "    # timeout-minutes: 30",
        "TestEveryCIJobHasACeilingOnItsTime",
    ),
    (
        "nothing builds the window binary, so it can stop compiling in silence",
        ".github/workflows/ci.yml",
        # Both branches, because either one alone still builds the window and
        # the property really does survive. Measured on 2026-08-20: mutating
        # only the Windows line reported NOT CAUGHT, and the guard was right -
        # the else branch was still building it.
        '            go build -tags \"$(cat .github/build-tags)\" -ldflags="$(cat .github/gui-ldflags)" ./cmd/tfg-gui\n          else\n            go build -tags \"$(cat .github/build-tags)\" ./cmd/tfg-gui',
        '            go build -tags \"$(cat .github/build-tags)\" -ldflags="$(cat .github/gui-ldflags)" ./cmd/tfg\n          else\n            go build -tags \"$(cat .github/build-tags)\" ./cmd/tfg',
        "TestTheWindowBinaryIsBuiltSomewhere",
    ),
    (
        # The job's -run pattern no longer names the test. go test with a
        # pattern matching nothing exits 0 saying "no tests to run", the matrix
        # skips the guard by its variable, and the import table is read by
        # nobody - which is the whole premise of the software renderer (O218).
        "the job that reads the import table misspells the test it exists to run",
        ".github/workflows/ci.yml",
        "-run '^TestTheWindowBinaryDoesNotImportOpenGLAtLoadTime$'",
        "-run '^TestTheWindowBinaryDoesNotImportOpenGLAtLoadTim$'",
        "TestTheImportTableGuardIsRunByTheJobThatNamesIt",
    ),
    (
        "the test matrix no longer asks the import table guard to skip, so the cold cgo build is back in it",
        ".github/workflows/ci.yml",
        '      TFG_IMPORT_TABLE_JOB: "1"',
        '      TFG_IMPORT_TABLE_JOBS: "1"',
        "TestTheImportTableGuardIsRunByTheJobThatNamesIt",
    ),
    (
        # A step that no longer reads its log for the PASS line is green when
        # the guard skips on a runner without gcc and when the pattern matches
        # no test - both measured to exit 0.
        "the import table job no longer reads its log for the PASS line of the one test it runs",
        ".github/workflows/ci.yml",
        "          grep -q -- '--- PASS: TestTheWindowBinaryDoesNotImportOpenGLAtLoadTime' import-table.log",
        "          true",
        "TestTheImportTableGuardIsRunByTheJobThatNamesIt",
    ),
    (
        # The wording reverted to the finding it used to be. The guard beside
        # it reads the expected text from the function, so only this one goes
        # red - which is what the outside review of #117 asked for.
        "the line before the second start states the missing driver as a fact again",
        "internal/gui/text/text.go",
        "The first attempt to open a window did not succeed - usually a graphics driver without OpenGL 2.1. Starting again",
        "The graphics driver on this computer offers no OpenGL 2.1. Starting again",
        "TestTheSecondStartSpeaksAsANoteNotAFinding",
    ),
    (
        "the window binary is linked as a console program, so Windows opens a black terminal beside it",
        ".github/gui-ldflags",
        "-H windowsgui",
        "-s",
        "TestTheWindowBinaryStartsWithoutAConsole",
    ),
    (
        # A box that is not showing keeps what was typed into it, so that
        # changing your mind twice does not cost the value. Sending it anyway is
        # the whole risk of that: the run refuses a batch stating two sizes, and
        # the refusal names a box nobody can see. O114 through the back door.
        "a way of stating a size is sent even when the switch is not on it",
        "internal/gui/window/recipesize.go",
        "	if b.chosenSizeKey() != key {",
        "	if false && b.chosenSizeKey() != key {",
        "TestOnlyTheChosenWayOfStatingASizeReachesTheRun",
    ),
    (
        "the room is worked out and never reaches the list, so shortening it does nothing at all",
        "internal/gui/parts/openlist.go",
        "if l.room > 0 && height > l.room {",
        "if l.room > 0 && height > l.room && false {",
        "TestAnOpenListIsCutToTheRoomThereIs",
    ),
    (
        "an open list goes under the box at full height again, straight through the bottom of the window",
        "internal/gui/parts/ring.go",
        "\tif rows <= below || below >= listOpensDownwardFrom*listRowHeight() || below >= above {",
        "\tif rows <= below || true {",
        "TestAnOpenListStaysInsideTheWindow",
    ),
    (
        "adding a batch goes back into the scrolling form, where one batch is taller than the window",
        "internal/gui/window/recipe.go",
        # Repointed 2026-10-14, when GitHub joined Donate in the bar.
        "r.footer(rail(append(append(outwardButtons(host), parts.Divider(), r.addBtn), links...)...)),",
        "r.footer(rail(append(outwardButtons(host), links...)...)),",
        "TestAddingABatchIsReachableWithoutScrolling",
    ),
    (
        # What this proves and what it does not, because the difference matters
        # here. It proves the guard is live: with a mapping accepted as a single
        # value, a wrong shape stops being refused as a recipe problem and gets
        # judged as content instead, and the guard says so.
        #
        # It does NOT reproduce the original defect, and no compiling mutation
        # can. That defect was the parser answering for a typed field, so
        # recreating it means changing a field back from scalar to string - and
        # that does not compile, which the runner reports as BROKEN and which
        # proves nothing. Measured on 2026-08-20 rather than assumed.
        "a mapping passes as a single value, so a wrong shape is judged as content",
        "internal/recipe/scalar.go",
        'if !s.single() {\n\t\treturn "", false',
        'if !s.single() && false {\n\t\treturn "", false',
        "TestNoRecipeKeyAnswersWithTheWordsOfTheParser",
    ),
    (
        "the trailer filter swallows the whole message, so an address in a body goes unread",
        "internal/guard/privatecontent_test.go",
        "if identityTrailer.MatchString(line) {",
        "if identityTrailer.MatchString(line) || true {",
        "TestAnAddressInTheTextOfAMessageIsStillFound",
    ),
    (
        "an en dash reaches README, which is exempt from the ASCII rule but not from rule 13",
        "README.md",
        "part this replaces",
        "part this replaces – ",
        "TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons",
    ),
    (
        "a batch can be added while a run is writing, which rebuilds the form the run is going through",
        "internal/gui/window/recipe.go",
        "r.runner.busy.also = append(r.runner.busy.also, r.addBtn)",
        "r.runner.busy.also = append(r.runner.busy.also)",
        "TestAddingABatchIsOutOfUseWhileARunIsGoing",
    ),
    (
        "the licence notice points only at files, so somebody with just the binary is sent nowhere",
        "internal/version/version.go",
        "https://www.gnu.org/licenses/gpl-3.0.html, and in the LICENSE file if you have",
        "the LICENSE file, if you have",
        "TestTheLicenceNoticePointsSomewhereThatAlwaysExists",
    ),
    (
        "the batch screen stops saying what an empty count box will do",
        "internal/gui/window/recipe.go",
        "b.count.SetPlaceHolder(text.PlaceholderLeftEmpty(strconv.Itoa(recipe.DefaultCount)))",
        "b.count.SetPlaceHolder(text.PlaceholderLeftEmpty(strconv.Itoa(recipe.DefaultCount + 1)))",
        "TestTheBatchScreenSaysWhatAnEmptyBoxWillDo",
    ),
    (
        "the form stays editable while a run writes, so changing a setting means nothing anybody can predict",
        "internal/gui/window/runbusy.go",
        "\tb.worn = true\n\tb.fields.Freeze(true)",
        "\tb.worn = true\n\tb.fields.Freeze(false)",
        "TestTheFormCannotBeEditedWhileARunIsGoing",
    ),
    (
        "a refusal leaves the form where it was, so a marked box off the screen reads as a dead button",
        "internal/gui/window/runrefuse.go",
        "if field := r.fields.Lookup(first); field != nil {",
        "if field := r.fields.Lookup(first); field != nil && false {",
        "TestARefusalBringsTheBoxItIsAboutIntoView",
    ),
    (
        "a refusal placed entirely on the boxes says nothing where the button was pressed",
        "internal/gui/window/runrefuse.go",
        "if len(loose) == 0 {",
        "if len(loose) == 0 && false {",
        "TestARefusalBringsTheBoxItIsAboutIntoView",
    ),
    (
        "looking inside a control stops at the wrapper, so freezing and revealing silently do nothing",
        "internal/gui/parts/reveal.go",
        "for _, child := range box.Objects {",
        "for _, child := range box.Objects[:0] {",
        "TestTheFormCannotBeEditedWhileARunIsGoing",
    ),
    (
        "a menu spells its values out in prose as well as offering them, on a screen that already does not fit",
        "internal/gui/parts/property.go",
        "if p.Kind == format.PropertyChoice {",
        "if p.Kind == format.PropertyChoice && false {",
        "TestTheWindowDrawsAFieldForEveryDeclaredProperty",
    ),
    (
        "the window offers the directory it was started from, so a double clicked run fills somebody's Downloads",
        "internal/gui/window/offered.go",
        "return filepath.Join(working, OutputFolderName)",
        "return filepath.Join(working)",
        "TestTheWindowOffersAFolderOfItsOwnToWriteInto",
    ),
    (
        "the window compares its own directory by text, so a double click reached under another spelling offers the program folder",
        "internal/gui/window/offered.go",
        "return os.SameFile(ia, ib)",
        "return os.SameFile(ia, ib) && a == b",
        "TestTheWindowOffersTheHomeDirectoryWhenStartedWhereItShouldNotWrite",
    ),
    (
        "the window forgets the root of a disk, so macOS started from Finder offers a read only /tfg-out",
        "internal/gui/window/offered.go",
        "return isRoot(dir) || sameDirectory(dir, program)",
        "return sameDirectory(dir, program)",
        "TestTheWindowOffersTheHomeDirectoryWhenStartedWhereItShouldNotWrite",
    ),
    (
        "without a home directory the window makes up a path instead of offering the one it always did",
        "internal/gui/window/offered.go",
        'if home != "" && notMeantForWriting(working, program) {',
        "if notMeantForWriting(working, program) {",
        "TestTheWindowOffersTheHomeDirectoryWhenStartedWhereItShouldNotWrite",
    ),
    (
        "any remembered folder under the program directory counts as left by the old offer, so a chosen one is thrown away",
        "internal/gui/window/offered.go",
        "if filepath.Base(remembered) != OutputFolderName {",
        "if false {",
        "TestAFolderTheOldOfferLeftBehindIsNotOfferedAgain",
    ),
    (
        "the window hands the screens the folder the old offer left behind, so /tfg-out comes back at every start",
        "internal/gui/window/open.go",
        'if last == "" || LeftByTheOldOffer(last, programDirectory()) {',
        'if last == "" {',
        "TestTheWindowDoesNotOfferTheFolderTheOldOfferLeftBehind",
    ),
    (
        "the window stops saying where the files will go, so the only field that decides it is off the screen",
        "internal/gui/window/runline.go",
        "\treturn text.RunLine(int(sum.files), sum.totalText(), sum.formats, l.where(dir))",
        "\treturn text.RunLine(int(sum.files), sum.totalText(), sum.formats, \"\")",
        "TestWhereTheFilesGoIsSaidOutsideTheScrollingPart",
    ),
    (
        "a run's messages take their room only once there is a run, so the form jumps when one starts",
        "internal/gui/window/run.go",
        "return parts.WithRoomForARun(container.NewVBox(bar, parts.Flush(status), problem.Object()))",
        "return container.NewVBox(bar, parts.Flush(status), problem.Object())",
        "TestTheFormDoesNotMoveWhenARunStarts",
    ),
    # REMOVED 2026-08-20. It took the slimming wrapper off the track and was
    # reported NOT CAUGHT by the full run, correctly: the track became a control
    # of ours that carries its own height, so the wrapper had stopped doing
    # anything and taking it off broke nothing. The wrapper is gone with it.
    # A mutation that cannot break the program is not proof of a guard.
    (
        "the room kept for a run's messages holds no line at all, so the bar still grows",
        "internal/gui/parts/parts.go",
        'NewProgress(), widget.NewLabel("Ag")',
        "NewProgress()",
        "TestTheRoomKeptForARunHoldsTheBarAndALine",
    ),
    (
        "the workflow writes the linker flags out by hand, so the release build can drift from it",
        ".github/workflows/ci.yml",
        '            go build -tags \"$(cat .github/build-tags)\" -ldflags="$(cat .github/gui-ldflags)" ./cmd/tfg-gui',
        "            go build -tags \"$(cat .github/build-tags)\" ./cmd/tfg-gui  # was: with the flags",
        "TestTheWorkflowBuildsTheWindowWithTheFlagsTheRepositoryDeclares",
    ),
    (
        # The formatter writes through a temporary name so that a process
        # ending mid write does not leave a recipe at half its length.
        "settling a recipe leaves the temporary file beside it",
        "internal/core/replace.go",
        "if err := os.Rename(tmp, path); err != nil {",
        "if err := error(nil); err != nil {",
        "TestSettlingARecipeLeavesNothingBesideIt",
    ),
    (
        # Found by audit on 2026-08-03. "--count -5" was reported as "asks for
        # 0 files", because anything below one built an empty list and the
        # planner then described the list rather than the request.
        "a count below one is described by the empty list it produces",
        "internal/cli/generate.go",
        'if given["count"] && g.count < 1 {',
        'if given["count"] && g.count < 1 && false {',
        "TestACountBelowOneIsReportedWithTheNumberThatWasWritten",
    ),
    (
        # Two messages for one fault, the first saying "there is nothing at
        # that path" about a path with something at it.
        "pointing the output at a file falls through to the system message",
        "internal/engine/preflight.go",
        "if info, err := os.Stat(opt.OutDir); err == nil && !info.IsDir() {",
        "if info, err := os.Stat(opt.OutDir); err == nil && !info.IsDir() && false {",
        "TestOutPointingAtAFileSaysSoOnce",
    ),
    (
        # The same class closed for declared ranges on 2026-08-03, surviving one
        # layer deeper: the declaration bounds each side on its own and cannot
        # express a limit on the two multiplied, so this ended with 1.
        # This one targets the sentence rather than the type. Wrapping the
        # error to change its class does not compile - the struct literal
        # cannot be closed by a swap of one line - and the runner reported
        # BROKEN, which proves nothing. The exit code half of that guard rides
        # on the classify mutation listed further up, which turns every
        # PropertyValueError back into a RUNTIME ending.
        "the refusal stops saying which limit the two settings ran into",
        "internal/format/png/png.go",
        'Unit: "megapixels", Per: 1_000_000,',
        'Unit: "", Per: 1_000_000,',
        "TestAPictureTooLargeToHoldIsTheCallersRequest",
    ),
    (
        "the format list stops admitting the limit on two settings at once",
        "internal/cli/formats.go",
        "for _, j := range d.JointLimits {\n\t\tfmt.Fprintf(out,",
        "for _, j := range d.JointLimits[:0] {\n\t\tfmt.Fprintf(out,",
        "TestTheFormatListAdmitsTheLimitOnTwoSettingsAtOnce",
    ),
    (
        # The command line could not state a reason at all, so a run driven by
        # flags could never fill the category the closed list exists to count.
        "the reason a flag states never reaches the manifest",
        "internal/cli/generate.go",
        "ExpectedReason: g.expectedReason,",
        'ExpectedReason: "",',
        "TestTheCommandLineCanStateWhyAnOutcomeIsExpected",
    ),
    (
        "the command line stops checking the reason against the closed list",
        "internal/cli/generate.go",
        'if g.expectedReason != "" && !recipe.KnownReason(g.expectedReason) {',
        'if g.expectedReason != "" && !recipe.KnownReason(g.expectedReason) && false {',
        "TestAReasonNobodyRecognisesIsRefusedOnBothSurfaces",
    ),
    (
        "a reason with no outcome is carried into the manifest describing nothing",
        "internal/cli/generate.go",
        'if g.expectedReason != "" && g.expected == "" {',
        'if g.expectedReason != "" && g.expected == "" && false {',
        "TestAReasonWithNoOutcomeIsRefused",
    ),
    (
        # Found by fuzzing on 2026-08-03, by a target written that day. The
        # printer in the dependency is not faithful for everything the parser
        # takes: "*0000000 : 000" prints as something that does not parse. With
        # this check gone, "recipe fmt -w" replaces the user's file with it and
        # reports success.
        "the formatter hands back a shape it never read back",
        "internal/recipe/canonical.go",
        "again, err := oneDocument(withoutBOM(out), name)",
        "again, err := oneDocument(withoutBOM(src), name)",
        "TestTheFormatterNeverWritesAFileItCannotReadBack",
    ),
    (
        # The other direction. A check that refuses everything would satisfy
        # the guard above and stop the formatter working at all.
        "the formatter refuses every file it is given",
        "internal/recipe/canonical.go",
        "if again.String() != string(out) {",
        "if again.String() == string(out) {",
        "TestAnOrdinaryRecipeStillSettlesAndStaysSettled",
    ),
    (
        # Found by tools/probes/hard-kill-probe.py on 2026-08-03. Three runs
        # killed with taskkill /F left one of these every time, and verify
        # called it "extra" - a word that says nothing about a file this tool
        # wrote itself, in a directory cleanup will never touch.
        "our own leftover is reported as a file we know nothing about",
        "internal/audit/audit.go",
        # Same reason as the entry about the second marker: the case list moved
        # onto three lines on 2026-09-07, so this takes the first of them.
        "core.IsPartialName(base)",
        "core.IsPartialName(base) && false",
        "TestALeftoverFromAKilledRunIsNamedForWhatItIs",
    ),
    (
        # The other direction. Calling everything a leftover would satisfy the
        # guard above and tell somebody their own file was ours.
        "somebody else's file is described as our leftover",
        "internal/audit/audit.go",
        'return Extra, ""',
        'return Leftover, ""',
        "TestAFileNobodyAskedForIsStillReportedAsExtra",
    ),
    (
        # The junction is the case the symbolic link guard could not see.
        # EvalSymlinks returns a junction unchanged and reports no error, so the
        # comparison said "inside" while the filesystem went elsewhere - and the
        # suite was green while the escape still worked. Measured 2026-08-03.
        "an unresolved reparse point is walked through rather than refused",
        "internal/core/filename.go",
        "if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {",
        "if fi.Mode()&os.ModeSymlink != 0 {",
        "TestCleanupDoesNotFollowAJunctionOutOfTheDirectory",
    ),
    (
        "verify walks through a junction instead of refusing it",
        "internal/core/filename.go",
        "return crossesUnresolvedLink(b.resolved, rel)",
        "return false",
        "TestVerifyDoesNotFollowAJunctionOutOfTheDirectory",
    ),
    (
        "verify reports a clean directory instead of refusing an entry that leaves it",
        "internal/audit/audit.go",
        "p, err := resolved(b, f)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}",
        "p, err := resolved(b, f)\n\t\tif err != nil {\n\t\t\treturn full, nil\n\t\t}",
        "TestVerifyDoesNotFollowALinkOutOfTheDirectory",
    ),
    (
        # The quick reading added on 2026-08-20 can only ever answer "inside".
        # That is the safety property the whole change rests on: it cannot
        # wrongly refuse, only wrongly accept - so this is the mutation that
        # matters, and it makes it accept a junction.
        "the quick reading accepts a redirection below the directory",
        "internal/core/filename.go",
        "&& !crossesUnresolvedLink(b.resolved, rel) {",
        "&& true {",
        "TestTheDirectoryMayBeNamedWithARelativePath",
    ),
    (
        # The other half of the same shortcut, and the one that nearly shipped.
        # A link inside the directory pointing at another file inside it has
        # left nothing, and the tool accepted it before. Answering from the
        # walk alone would turn a contained directory into exit 5.
        "a redirection below the directory is refused without asking where it points",
        "internal/core/filename.go",
        "	return b.escapesTheThoroughWay(full)",
        "	return true",
        "TestALinkInsideTheDirectoryPointingBackInsideItIsNotAnEscape",
    ),
    (
        # No verdict changes here, only the cost - filepath.Rel refuses a
        # relative path against an absolute one, so comparing against the name
        # as typed sends every "./out" run down the expensive reading with the
        # right answer. Nothing behavioural can see it, which is why the guard
        # reads the source.
        # The reserve is a floor and the scroll is the ceiling. Without the
        # scroll the box grows with the message again, which is what took 19 px
        # of form per extra line until 2026-08-20. A ceiling in reserving.MinSize
        # was tried alongside and taken out: it changed no measurement and could
        # not be reddened, because a scroll asks for almost nothing anyway.
        # The order stopped being cosmetic on the day the room became a ceiling.
        # With the message scrolling inside a fixed height, the first line is
        # the only one certain to be seen - and a finished run was showing "no
        # limit was given..." with "7 files written." out of sight below it.
        #
        # The first version of the guard checked the first line for the word
        # "file" and stayed green under this mutation, because the note talks
        # about files too. It matches the tail of the preview's own sentence now.
        "a note about a default is put above the sentence somebody pressed the button for",
        "internal/gui/window/run.go",
        "append(append([]string{}, lines...), r.notes...)",
        "append(append([]string{}, r.notes...), lines...)",
        "TestWhatARunSaysComesBeforeWhatSettlingSaid",
    ),
    (
        "a run's messages push the form instead of scrolling inside their own room",
        "internal/gui/parts/parts.go",
        "	inside := container.NewVScroll(content)",
        "	inside := content",
        "TestTheFormDoesNotMoveWhenARunStarts",
    ),
    (
        # The vertical scale, added 2026-08-20 after the design audit measured
        # sixteen consecutive gaps down one card, all of them 20 to 27 px. A
        # form that spaces "one field" and "a new group" the same reads as a
        # wall of text. Since 2026-09-22 the name stands over its box again,
        # so the mutation is the one a stacked form has: the name as far from
        # its box as the next field is, and nothing says which box it names.
        "a field's name stands as far from its box as the next field does",
        "internal/gui/parts/fieldstack.go",
        "\treturn container.New(fieldStack{column{gap: GapLabel}}, pieces...)",
        "\treturn container.New(fieldStack{column{gap: GapField}}, pieces...)",
        "TestANameStandsOverItsBoxOnOneEdgeWithEveryOther",
    ),
    (
        # The defect exactly as it stood: one screen handed Screen a box of
        # panels instead of the panels, so the section spacing landed around
        # the box. 7 px here against 23 px on the preset screen.
        "one screen wraps its panels, so the space between them is its own",
        "internal/gui/window/generate.go",
        "\t\t\tg.settingsSection()...,",
        "\t\t\tcontainer.NewVBox(g.settingsSection()...),",
        "TestTwoSectionsAreTheSameDistanceApartOnEveryScreen",
    ),
    (
        "the boundary between two sections is drawn no wider than the one between two fields",
        "internal/gui/parts/parts.go",
        "return Column(GapSection, panels...)",
        "return Column(GapField, panels...)",
        "TestTheGapBetweenSectionsIsWiderThanTheGapBetweenFields",
    ),
    (
        # Alignment, added 2026-08-20. Six pixels is the worst of the three
        # distances a title can stand at: aligned reads as deliberate, a real
        # indent reads as deliberate, six pixels reads as nobody having looked.
        "the screen title stands on the panel's edge instead of on its content",
        "internal/gui/parts/parts.go",
        "Stacked(append([]fyne.CanvasObject{Indented(head)}, sections...)...))",
        "Stacked(append([]fyne.CanvasObject{head}, sections...)...))",
        "TestEverythingAPersonReadsStartsOnOneLeftEdge",
    ),
    (
        # The report panel loses the padding every other panel has, so what a
        # run says starts at the panel's edge while the boxes above it start
        # at its content. Two left edges a few pixels apart in one column.
        #
        # Repointed on 2026-09-08, in the evening. The version above aimed at
        # the rebuilt window and went away with it when gui/new-look was
        # deleted - the guard it named does not exist on main, and neither
        # does raisedSurface. The guard that DOES exist asks the same
        # question of the same code, so the entry moves rather than dies.
        "the action bar's column loses its indent and speaks from the panel edge",
        "internal/gui/parts/parts.go",
        "\tcolumn := container.New(readableWidth{}, Indented(inner))",
        "\tcolumn := container.New(readableWidth{}, inner)",
        "TestTheActionBarSpeaksOnTheSameEdgeAsTheForm",
    ),
    (
        # The success and warning colours were in both palettes from the day the
        # palette was written and reached the screen for the first time on
        # 2026-08-20. Until then "3 files written." was drawn in exactly the
        # grey the neutral line beside it uses.
        "a finished run goes back to being drawn like any other sentence",
        "internal/gui/window/run.go",
        "	r.toneOfOutcome(res, runErr)",
        "	_ = res",
        "TestAFinishedRunIsColouredLikeAFinishedRun",
    ),
    (
        "the colour of one run's outcome is left on the line for the next one",
        "internal/gui/window/run.go",
        "	status.Importance = widget.MediumImportance\n"
        "	status.SetText(said)",
        "	status.SetText(said)",
        "TestTheColourOfAnOutcomeDoesNotOutliveIt",
    ),
    (
        # An enum set on a widget proves the field was set. This asks whether
        # setting it changes the drawing at all.
        "the success colour is the ordinary text colour, so saying so draws nothing",
        "internal/gui/parts/theme.go",
        "theme.ColorNameSuccess:   hex(0x42, 0xC2, 0x60),",
        "theme.ColorNameSuccess:   hex(0xE6, 0xE6, 0xE8),",
        "TestTheOutcomeColoursAreThreeDifferentColours",
    ),
    (
        # The toolkit draws the groove in the accent colour at half alpha, so a
        # bar at nothing was a solid blue bar. Measured 2026-08-20: the empty
        # track stood against the panel at 2.80 while the fill stood against
        # the track at 2.49 - louder empty than full.
        "the groove goes back to being drawn in the colour of the fill",
        "internal/gui/parts/progress.go",
        "r.track.FillColor = PaletteColour(theme.ColorNameInputBackground, theme.VariantDark)",
        "r.track.FillColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)",
        "TestTheFilledPartOfAProgressBarIsLouderThanTheEmptyPart",
    ),
    (
        "the fill is drawn the whole width whatever the value is",
        "internal/gui/parts/progress.go",
        "	width := size.Width * share",
        "	width := size.Width",
        "TestAProgressBarDrawsWhatItWasToldAndNotMore",
    ),
    (
        # Disabled and placeholder were one colour, so a run in flight drew the
        # value somebody chose in the same grey as the hint in an empty box.
        "a switched off value goes back to being drawn like a hint",
        "internal/gui/parts/theme.go",
        "theme.ColorNameDisabled:  hex(0xC7, 0xC7, 0xCC),",
        "theme.ColorNameDisabled:  hex(0xA2, 0xA2, 0xA9),",
        "TestAValueSwitchedOffIsItsOwnStepBetweenAValueAndAHint",
    ),
    (
        # Which side of the hint the step falls on is the decision, not the
        # value: a disabled value is content somebody typed.
        "the light palette puts a switched off value further away than a hint",
        "internal/gui/parts/theme.go",
        "theme.ColorNameDisabled:  hex(0x3D, 0x3D, 0x42),",
        "theme.ColorNameDisabled:  hex(0x8A, 0x8A, 0x8A),",
        "TestASwitchedOffValueIsEasierToReadThanAHint",
    ),
    (
        # A box is a promise about what goes in it. Measured 2026-08-20: the
        # width box of a BMP was 806 px for a number from 1 to 20000, on a
        # screen where "how many" was 140.
        #
        # The replacement is Sized(ColumnWidth) rather than the obvious
        # "return control", and that is O131. Dropping the wrapper used to
        # give the box the whole column, so it read as the plain way to put
        # the defect back. Measured 2026-08-26 off a diagnostic inside the
        # guard package: it now lays every numeric box out at 0 or -3 px,
        # because the row stopped stretching an unsized child somewhere
        # after 2026-08-20. That is the opposite failure from the one this
        # guard asks about, so the guard stayed green honestly - and the
        # entry beside this one already proves the too-narrow direction.
        "a box for a number goes back to running the width of the form",
        "internal/gui/parts/property.go",
        "	return Sized(fyne.Max(NumericWidth, roomFor(leftAlone(p, ofATool))), control)",
        "	return Sized(ColumnWidth, control)",
        "TestABoxForANumberIsNotAsWideAsTheForm",
    ),
    (
        # The same defect the other way up, and it happened on the first try:
        # at the plain number width, "worked out from the size" was cut off
        # mid-word.
        "a box for a number is too narrow to show its own placeholder",
        "internal/gui/parts/property.go",
        "	return Sized(fyne.Max(NumericWidth, roomFor(leftAlone(p, ofATool))), control)",
        "	return Numeric(control)",
        "TestABoxIsWideEnoughForItsOwnPlaceholder",
    ),
    (
        # Two screens draw a field from the same declaration through different
        # code, and only one of them got the width on the first try.
        "the batch screen draws a number in one column of the grid, too narrow for the words it shows when left alone",
        "internal/gui/window/recipefolds.go",
        "			parts.ShapedFor(b.declared[i], f.Control))",
        "			f.Control)",
        "TestABoxForANumberIsNotAsWideAsTheFormOnTheBatchScreen",
    ),
    (
        # Two naming systems in one visual style: Format, Size, How many on one
        # side and settings for bmp, width, bit_depth on the other, same weight
        # at the same size.
        "a declared setting wears its recipe key as its label again",
        "internal/gui/parts/property.go",
        "		field := into.Add(p.Name, text.SettingLabel(p.Name), PropertyDetail(owner, p),",
        "		field := into.Add(p.Name, p.Name, PropertyDetail(owner, p),",
        "TestNoSettingWearsItsRecipeKeyAsALabel",
    ),
    (
        # What makes the label change a translation rather than a loss: the key
        # is the only spelling that works in a recipe.
        "the recipe key of a setting is nowhere on the screen",
        "internal/gui/parts/property.go",
        "			tips.Say(text.WrittenAs(owner, p.Name)), shapedFor(p, f.Control, text.IsTool(owner)))",
        "			NoDetail, shapedFor(p, f.Control, text.IsTool(owner)))",
        "TestTheRecipeKeyOfASettingIsStillReachable",
    ),
    (
        # A refusal has four parts, so it is a sentence. Confined to one column
        # of a row it wrapped onto four lines with half the panel empty beside
        # it, measured 2026-08-20.
        "a refusal goes back into the column its field sits in",
        "internal/gui/parts/table.go",
        "	return Column(GapTight, append([]fyne.CanvasObject{Row(bodies...)}, areas...)...)",
        "	return Row(objects...)",
        "TestARefusalIsAsWideAsTheFormRatherThanItsColumn",
    ),
    (
        # The engine names a setting by its recipe key, which is the only name
        # the command line has. This window calls that box something else.
        "a refusal names the box by the key a recipe writes",
        "internal/gui/parts/fields.go",
        "	f.Say(inTheWordsOnScreen(f, err))",
        "	f.Say(err.Error())",
        "TestARefusalNamesTheBoxTheWayTheScreenNamesIt",
    ),
    (
        # The switch stops taking the other two off the screen, so three ways of
        # stating one size stand side by side again - which is the arrangement
        # O114 was written about, and the sentence that warned about it is gone.
        "the switch leaves every way of stating a size on the screen at once",
        "internal/gui/window/recipesize.go",
        "		box.Hide()",
        "		box.Show()",
        "TestEveryWayOfStatingASizeIsOfferedAndOnlyOneIsOnTheScreen",
    ),
    (
        # The recipe reader words a batch with no size as "target 1 has no size"
        # however it was meant to be stated, so with the switch on a range that
        # refusal is addressed to a box nobody can see. The form then refuses to
        # run and marks nothing, which reads as a button that did not work.
        "a refusal about a size lands on the box the switch is not showing",
        "internal/gui/window/recipe.go",
        "	r.runner.readdress = r.readdressSizeWay",
        "	r.runner.readdress = nil",
        "TestAStarIsOnEveryBoxTheRunWillNotDoWithout",
    ),
    (
        # Measured 2026-08-20: the selected tab stood at 7.71 against the page
        # and the three nobody was on stood at 13.36, so the chosen one was the
        # quietest label in the strip. The strip is ours since 2026-09-15 and
        # the word draws its own colour, so the defect in its new shape is the
        # chosen word drawn as quietly as the rest.
        "the chosen word on the strip goes as quiet as the rest",
        "internal/gui/parts/tabs.go",
        "\tif r.word.chosen || r.word.hovered {\n\t\tr.text.Color",
        "\tif r.word.hovered {\n\t\tr.text.Color",
        "TestTheTabSomebodyIsOnIsTheOneThatStandsOut",
    ),
    (
        # The card and its first field were both called Preset - the same word
        # twice inside 40 px, in two ranks of type.
        "a section goes back to being named after the field inside it",
        "internal/gui/text/screens.go",
        'func SectionPreset() string        { return say("SectionPreset", "The question") }',
        'func SectionPreset() string        { return say("SectionPreset", "Preset") }',
        "TestNoSectionIsNamedAfterAFieldInsideIt",
    ),
    (
        # Deleting the title also satisfies the guard above, which is why the
        # other half is guarded too.
        "the preset card loses the name that says what it asks",
        "internal/gui/window/preset.go",
        # Repointed 2026-09-23, when every section of a work screen started
        # to fold (window/sections.go).
        "p.sections.section(sectionPreset, text.SectionPreset(),",
        'p.sections.section(sectionPreset, "",',
        "TestThePresetCardIsNamedForWhatItAsks",
    ),
    (
        # The two least related controls in the window stood side by side in one
        # style at the left of the bar, so proximity said they were a pair.
        "Add a batch goes back to standing as a pair with the button before it",
        "internal/gui/window/recipe.go",
        # Repointed 2026-10-14, when GitHub came to stand between the two.
        "append(outwardButtons(host), parts.Divider(), r.addBtn)",
        "append(outwardButtons(host), r.addBtn)",
        "TestAddABatchIsNotDrawnAsAPairWithTheButtonBeforeIt",
    ),
    (
        # A table kept by hand goes stale in silence - O80 names two of those
        # already. The twenty-first format joins this one on the day it is
        # registered or the suite goes red.
        "a registered format is left out of the table that says what kind of file it is",
        "internal/gui/parts/filekind.go",
        '"svg":     kindPicture,',
        "",
        "TestEveryRegisteredFormatHasAKind",
    ),
    (
        # The toolkit's file icons are one sheet of paper with different creases
        # at the size a row draws them, so six rows carried one icon. An icon
        # that looks like information and is not is worse than none.
        "two kinds of file go back to being drawn with one picture",
        "internal/gui/parts/filekind.go",
        "		return theme.MediaPhotoIcon()",
        "		return theme.ListIcon()",
        "TestTwoKindsOfFileAreNotDrawnWithOnePicture",
    ),
    (
        # The inset that put the bar's words on the form's left edge was applied
        # on all four sides, so it went in twice vertically and took the bar
        # from 134 px to 146 on a screen where the form is already too tall.
        "the action bar is inset twice, so it costs the form twelve pixels for nothing",
        "internal/gui/parts/parts.go",
        "\tcolumn := container.New(readableWidth{}, Indented(inner))",
        "\tcolumn := container.New(readableWidth{}, Padded(InsetBar, Indented(inner)))",
        "TestTheActionBarCostsItsContentPlusOneInset",
    ),
    (
        # The reserve is why the form does not jump when Generate is pressed,
        # and taking it out is the cheapest way to make a bar shorter. The
        # replacement keeps the name in use so the package still compiles.
        "the room kept for a run is not kept at all",
        "internal/gui/parts/parts.go",
        "func WithRoomForARun(content fyne.CanvasObject) fyne.CanvasObject {",
        "func WithRoomForARun(content fyne.CanvasObject) fyne.CanvasObject { return content }\n"
        "func unusedRoomForARun(content fyne.CanvasObject) fyne.CanvasObject {",
        "TestTheActionBarStillReservesRoomForARun",
    ),
    (
        # Two boxes for a number stacked cost a row each and leave two thirds
        # of the panel empty. Pairing them took the batch screen from 1259 px
        # to 1173 on 2026-08-20.
        # Since 2026-09-22 a name stands over its box and there is no column,
        # so the way a declared setting leaves the edge is its whole block
        # being indented under the fields above it.
        "the block of declared settings is indented under the fields above it",
        "internal/gui/parts/parts.go",
        "\treturn Column(GapField, children...)",
        "\treturn Indented(Column(GapField, children...)).(*fyne.Container)",
        "TestADeclaredSettingStandsOnTheSameEdgeAsTheFieldsAboveIt",
    ),
    (
        "the quick comparison uses the directory as typed rather than its absolute spelling",
        "internal/core/filename.go",
        "filepath.Rel(b.abs,",
        "filepath.Rel(b.named,",
        "TestTheCheapComparisonUsesTheAbsoluteSpellingOfTheDirectory",
    ),
    (
        # Same class: right answers, cost back. Resolving the boundary inside
        # the loop is what observation O117 measured at two thirds of a verify
        # on Windows. The mutation keeps "boundary" in use so the package still
        # compiles - a mutation that does not build proves nothing.
        "the boundary is worked out again for every entry",
        "internal/audit/audit.go",
        "		p, err := resolved(b, f)",
        "		p, err := resolved(core.NewBoundary(b.Dir()), f)",
        "TestTheBoundaryIsWorkedOutOncePerPassAndNotOncePerEntry",
    ),
    (
        # The other direction, and it matters more here than usual: people keep
        # fixtures on redirected paths, so refusing every link would break an
        # ordinary setup. Two things had to be right for this to pass - the
        # walk resolves its root, and the manifest claim survives Go's O_EXCL
        # lying on a reparse point.
        "the walk stops resolving the directory it was pointed at",
        "internal/audit/walk.go",
        "if resolved, err := filepath.EvalSymlinks(dir); err == nil {",
        "if resolved, err := filepath.EvalSymlinks(dir); err != nil {",
        "TestADirectoryReachedThroughALinkStillWorks",
    ),
    (
        # Found by audit on 2026-08-03. "--name trailing." finished with exit 0,
        # the file landed as "trailing", and verify then reported the tool's own
        # output as wrong.
        "a name the system stores differently is accepted again",
        "internal/engine/filename.go",
        'case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):',
        'case (strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ")) && false:',
        "TestANameTheSystemWouldNotStoreVerbatimIsRefused",
    ),
    (
        # The other direction. Refusing every name holding a dot would satisfy
        # the guard above and refuse invoice.txt.
        "the name rule spreads from the last character to the whole name",
        "internal/engine/filename.go",
        'strings.HasSuffix(name, ".") ||',
        'strings.Contains(name, ".") ||',
        "TestAnOrdinaryNameWithADotOrASpaceStillWorks",
    ),
    (
        "a manifest of any size is read into memory",
        "internal/manifest/manifest.go",
        "if int64(len(raw)) <= MaxBytes {",
        "if len(raw) >= 0 {",
        "TestAManifestTooLargeToReadIsRefusedBeforeItIsRead",
    ),
    (
        # Found by audit on 2026-08-03. Measuring an archive meant building it,
        # and building it meant running every generator inside, two or three
        # times over - so --dry-run on a 256 MB archive cost 960 ms against a
        # 56 ms baseline, and one with large declared contents never returned.
        #
        # This mutation restores that path, so it is one of the slower entries
        # here. It has to run to completion to be caught.
        #
        # Recorded honestly: the size guards go red on it as well, because
        # generating the contents while also adding them arithmetically counts
        # them twice. So this proves the cost guard runs, not that it is the
        # only thing standing between us and the defect. What it alone would
        # catch is a future change that keeps the sizes right and adds a second
        # full pass - and no single swap expresses that.
        "measuring an archive generates everything inside it again",
        "internal/format/zip/zip.go",
        "\t\tif withContents {\n\t\t\tif err := c.desc.Generator.Write(ctx, entry, c.plan); err != nil {",
        "\t\tif true {\n\t\t\tif err := c.desc.Generator.Write(ctx, entry, c.plan); err != nil {",
        "TestPlanningAContainerDoesNotGenerateWhatItHolds",
    ),
    (
        # Found by audit on 2026-08-03. Before the ceiling existed this reached
        # make([]int64) and panicked with a Go stack trace under exit code 2.
        "the flag path stops asking whether the count can be planned",
        "internal/cli/generate.go",
        "if int64(g.count) > core.MaxFilesPerRun {",
        "if int64(g.count) > core.MaxFilesPerRun && false {",
        "TestAnImpossibleFileCountIsRefusedRatherThanAllocated",
    ),
    (
        # The recipe reader took the worse of the two failures: it grew the list
        # one entry at a time and reached a 13 GB allocation.
        "the recipe reader stops asking whether the count can be planned",
        "internal/recipe/target.go",
        # Carries the line under it since 2026-08-20. The container entry gained
        # the same ceiling that day, so this pattern stopped being unique - and
        # an ambiguous pattern is how a mutation quietly starts proving a
        # different guard than the one it names. The two are told apart by which
        # address they build: `where` for the target, `at` for the entry.
        "case n > core.MaxFilesPerRun:\n\t\t\tp.add(where.of(\"count\")",
        "case n > core.MaxFilesPerRun && false:\n\t\t\tp.add(where.of(\"count\")",
        "TestAnImpossibleFileCountInARecipeIsRefused",
    ),
    (
        # Expected verdict: CAPPED, for the same reason its sibling above is -
        # the mutation removes the refusal, so the reader does the thing the
        # refusal existed to prevent and allocates until the runner stops it.
        # Measured on 2026-08-20 with try-one.py: stopped at 4.1 GB. Without the
        # mutation the same recipe is refused in milliseconds, and that
        # difference is the proof this entry carries.
        "the container entry stops asking whether its count can be planned",
        "internal/recipe/contains.go",
        "case n > core.MaxFilesPerRun:\n\t\t\t\tp.add(at.of(\"count\")",
        "case n > core.MaxFilesPerRun && false:\n\t\t\t\tp.add(at.of(\"count\")",
        "TestAnImpossibleFileCountInsideAContainerIsRefused",
    ),
    (
        "the ceiling is applied per target, so writing the number out in pieces gets past it",
        "internal/engine/engine.go",
        "totalFiles += len(t.Sizes)",
        "totalFiles = len(t.Sizes)",
        "TestTheCeilingCountsTheWholeRunNotOneTarget",
    ),
    (
        # The other direction. A ceiling low enough to refuse ordinary work
        # would satisfy every guard above and break the tool for everybody.
        "the ceiling drops to something a real run would meet",
        "internal/core/limits.go",
        "const MaxFilesPerRun = 1_000_000",
        "const MaxFilesPerRun = 10",
        "TestAnOrdinaryFileCountStillWorks",
    ),
    (
        # This one never crashed, which is why it is the dangerous one: the sum
        # wrapped negative, the free space check found the disk big enough, and
        # the run started writing.
        "the byte total may wrap again and satisfy the free space check",
        "internal/core/limits.go",
        "if total > math.MaxInt64-size {",
        "if total > math.MaxInt64-size && false {",
        "TestATotalThatDoesNotFitIsRefused",
    ),
    (
        "a boundary set with no room above the limit wraps instead of being refused",
        "internal/core/size.go",
        "if limit == math.MaxInt64 {",
        "if limit == math.MaxInt64 && false {",
        "TestABoundaryThatCannotHoldItsOwnSetIsRefused",
    ),
    (
        # Found by audit on 2026-08-03. Before the check existed, an entry with
        # the path "../VICTIM.txt" and cleanup --yes --force removed a file
        # above the output directory and ended with exit code 0.
        "a manifest entry may climb out of the directory again",
        "internal/core/filename.go",
        'strings.HasPrefix(cleaned, "../"):',
        'strings.HasPrefix(cleaned, "../") && false:',
        "TestTheTextualContainmentCheckCatchesEveryWrittenClimb",
    ),
    (
        "the manifest is read without asking where its entries point",
        "internal/manifest/manifest.go",
        "if err := checkPaths(path, &m); err != nil {",
        "if err := checkPaths(path, &m); err != nil && false {",
        "TestLoadingAManifestRefusesAnEntryThatClimbsOut",
    ),
    (
        # Measured on Windows before the fix: an absolute path came out as
        # "unreachable" because filepath.Join mangled it, which is the right
        # outcome reached by accident and only while the volume differs.
        "an absolute path in a manifest is resolved rather than refused",
        "internal/core/filename.go",
        "if hasVolumeName(unified) {",
        "if hasVolumeName(unified) && false {",
        "TestAManifestPathIsNeverAbsolute",
    ),
    (
        # The other direction. Refusing every path with a separator would
        # satisfy the three guards above and quietly break grouped output,
        # which nobody would notice until somebody used it.
        "the containment check starts refusing paths that stay inside",
        "internal/core/filename.go",
        'case cleaned == ".":',
        'case cleaned == ".", strings.Contains(cleaned, "/"):',
        "TestAManifestPathMayStillNameASubdirectory",
    ),
    (
        # The other direction of the door in manifest.Load, so shutting it on
        # everything is not a way to pass.
        "the door refuses a manifest naming a file in a subdirectory",
        "internal/manifest/manifest.go",
        "problem := core.ContainmentProblem(f.Path)",
        'problem := core.ContainmentProblem("../" + f.Path)',
        "TestLoadingAManifestAcceptsAnEntryInASubdirectory",
    ),
    (
        # And of the textual check itself. The prefix is emptied rather than
        # shortened to a dot: none of the ordinary names in that guard begins
        # with one, so shortening it refused nothing and proved nothing.
        "the textual check starts refusing paths that stay inside",
        "internal/core/filename.go",
        'case cleaned == "..", strings.HasPrefix(cleaned, "../"):',
        'case cleaned == "..", strings.HasPrefix(cleaned, ""):',
        "TestTheTextualContainmentCheckLetsOrdinaryPathsThrough",
    ),
    (
        "a path before the flags stops being accepted",
        "internal/cli/cli.go",
        "case leading != \"\" && fs.NArg() == 0:",
        "case false:",
        "TestAPathIsAcceptedBeforeOrAfterTheFlags",
    ),
    (
        "a byte order mark is left in, so the first key reads as a typo",
        "internal/recipe/canonical.go",
        "for strings.HasPrefix(out, bom) {",
        "for strings.HasPrefix(out, bom) && false {",
        "TestARecipeCarryingAByteOrderMarkIsRead",
    ),
    (
        # The other direction. Trimming everywhere rather than at the front
        # would eat a mark a recipe legitimately carries inside a value, which
        # the guard checks with a mark in a file name.
        "the mark is stripped wherever it appears, not just at the front",
        "internal/recipe/canonical.go",
        "out = strings.TrimPrefix(out, bom)",
        'out = strings.ReplaceAll(out, bom, "")',
        "TestARecipeCarryingAByteOrderMarkIsRead",
    ),
    (
        # The regression this replaced: counting raw documents makes a document
        # out of a comment before a leading "---", so one recipe reads as two
        # and a file somebody had every reason to write is turned away.
        "documents are counted raw again, so one recipe reads as two",
        "internal/recipe/canonical.go",
        "if d.Body == nil || d.Body.Type() == ast.CommentType {",
        "if (d.Body == nil || d.Body.Type() == ast.CommentType) && false {",
        "TestOneRecipeIsAcceptedWhateverSeparatorsSurroundIt",
    ),
    (
        # The other direction: counting nothing at all would accept two
        # recipes and produce half the fixtures, which is what the rule exists
        # to stop. One mutation per direction, because a counter can fail
        # either way and the two are caught by different guards.
        "every document is discounted, so two recipes slip through",
        "internal/recipe/canonical.go",
        "\t\tn++",
        "\t\tn += 0",
        "TestAFileHoldingTwoRecipesIsRefusedRatherThanHalfRun",
    ),
    (
        # The sharp one: the reader keeps its check, only the formatter loses
        # it. That is the exact shape of the bug this replaced - fmt laid out a
        # two recipe file and ended 0, and --check then called it clean.
        "the formatter stops checking and drifts away from the reader",
        "internal/recipe/canonical.go",
        "f, err := oneDocument(withoutBOM(src), name)",
        "f, err := parser.ParseBytes(withoutBOM(src), parser.ParseComments)",
        "TestTheFormatterRefusesTheSameFileTheReaderRefuses",
    ),
    (
        "a boundary gives three identical sizes instead of consecutive ones",
        "internal/core/size.go",
        "return []int64{limit - 1, limit, limit + 1}, nil",
        "return []int64{limit, limit, limit}, nil",
        "TestABoundaryGivesThreeConsecutiveSizesAroundTheLimit",
    ),
    (
        "a boundary beside a size is accepted",
        "internal/recipe/target.go",
        "case rt.Boundary != nil && rt.Size != nil:",
        "case rt.Boundary != nil && rt.Size != nil && false:",
        "TestABoundaryBesideASizeOrACountIsRefused",
    ),
    (
        "a boundary set never reaches below the limit",
        # The guard puts a boundary where WAV cannot deliver the lower edge -
        # once below its minimum, once in the unreachable gap just above it. A
        # set that quietly started at the limit would produce three valid files
        # and the test would never learn that its lower edge was gone.
        "internal/core/size.go",
        "return []int64{limit - 1, limit, limit + 1}, nil",
        "return []int64{limit, limit, limit + 1}, nil",
        "TestABoundaryTheFormatCannotDeliverIsRefusedBeforeAnyFile",
    ),
    (
        "a name carrying a path is accepted, so files land outside the run",
        "internal/engine/filename.go",
        "case strings.ContainsAny(name, `/\\`):",
        "case false:",
        "TestANameThatIsAPathIsRefusedAndNothingEscapes",
    ),
    (
        "the manifest name is not checked, so a manifest lands elsewhere",
        "internal/engine/engine.go",
        "if err := checkFileName(SettingOutputManifest, core.Says(\"engine.TheManifest\", \"the manifest\"), opt.ManifestName); err != nil {",
        "if err := error(nil); err != nil {",
        "TestAManifestNameThatIsAPathIsRefused",
    ),
    (
        "an unknown placeholder is left in the file name",
        "internal/engine/engine.go",
        'if strings.Contains(name, "{") {',
        "if false {",
        "TestAnUnknownPlaceholderInANameIsRefused",
    ),
    (
        "the declared reason is dropped on the way to the manifest",
        "internal/engine/engine.go",
        "Reason:     f.Target.ExpectedReason,",
        'Reason:     "",',
        "TestTheDeclaredReasonReachesTheManifest",
    ),
    (
        "a reason off the closed list is accepted",
        "internal/recipe/target.go",
        "} else if !reasons[s] {",
        "} else if false {",
        "TestAReasonOffTheClosedListIsRefused",
    ),
    (
        "recipe fmt --check passes whatever it is given",
        "internal/cli/recipecmd.go",
        "if *check {\n\t\tif settled {",
        "if *check {\n\t\tif true {",
        "TestFormattingCommandSettlesAndLeavesASettledFileAlone",
    ),
    (
        "the canonical form gains a stray blank line",
        "internal/recipe/canonical.go",
        "out := []byte(f.String())",
        'out := []byte(f.String() + "\\n")',
        "TestTheCanonicalFormOfARecipeHasNotDrifted",
    ),
    # The three below all leave the size exact and the run at code 0, which is
    # the point. Every one of them shipped in a file the owner looked at and
    # accepted, and no guard in the suite had anything to say.
    (
        "the record that did not fit keeps its number",
        "internal/core/records.go",
        "rec.Discard()",
        "_ = rec",
        "TestRecordNumbersRunFromOneWithoutAGap",
    ),
    (
        "the closing label shares a baseline with the identity label",
        "internal/format/svgfile/svg.go",
        "`<text x=\"16\" y=\"%d\" font-size=\"13\" fill=\"#333333\">`, h-34",
        "`<text x=\"16\" y=\"%d\" font-size=\"13\" fill=\"#333333\">`, h-16",
        "TestNothingIsPaintedOverTheSVGLabel",
    ),
    (
        "shapes reach the bottom edge and cover the label",
        "internal/format/svgfile/svg.go",
        "\tif d := h - TextBand; d > 0 {",
        "\tif d := h - 0; d > 0 {",
        "TestNothingIsPaintedOverTheSVGLabel",
    ),
    # Removed 2026-08-02: "the vocabulary stops being ASCII and the filler cuts
    # a character in half". It was caught before core.AppendFiller learned to
    # cut on a character boundary, and it is NOT CAUGHT after - which is the
    # runner proving the fix rather than a hole. Breaking the guard now needs
    # two changes at once, a non ASCII vocabulary and a byte cut, and that is
    # not one substitution. It moved to provenByProbe with both measurements.
    (
        # Set takes a color.Color interface, so every pixel boxes a colour onto
        # the heap. This is what the code did until 2026-08-02: 786443 objects
        # for one 16 MiB image, while the byte budget beside it stayed green
        # because the boxes are small and short lived.
        "a colour is boxed onto the heap for every pixel",
        "internal/format/png/png.go",
        "img.SetRGBA(x, y, color.RGBA{",
        "img.Set(x, y, color.RGBA{",
        "TestNoGeneratorHoldsTheWholeFileInMemory",
    ),
    (
        "a goroutine appears outside the files that declare concurrency",
        "internal/core/records.go",
        "shortest := rec.Shortest()",
        "shortest := rec.Shortest()\n\tgo func() {}()",
        "TestConcurrencyStaysWhereItWasPutOnPurpose",
    ),
    (
        # Rule 13 asks for one dash and no semicolons in the prose that ships.
        # Nothing held it until 2026-08-02, and the tree passed on the day the
        # guard arrived - which is the state a ratchet wants to start from.
        "a semicolon reaches the prose the user reads",
        "internal/cli/cli.go",
        "// Exit codes are a frozen contract.",
        "// Exit codes are a frozen contract; changing one is breaking.",
        "TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons",
    ),
    (
        # Deliberately outside internal/cli and cmd/tfg. The ASCII guard covers
        # the dash there already, so a mutation in those two would re-prove
        # ground that was held, not the ground this guard added.
        "an em dash reaches a comment the ASCII guard never looks at",
        "internal/core/records.go",
        "// A truncated last record is the worst kind of defect this tool can ship.",
        "// A truncated last record — the worst defect this tool can ship.",
        "TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons",
    ),
    (
        # Base 0 asks Go to infer the base from the prefix, which is exactly the
        # rule YAML applied and exactly the defect this type was written to
        # remove: 010 becomes 8 again. Sizes stay right and the exit code stays
        # zero, so nothing but the fidelity guard notices.
        "a padded number is read in the base its prefix suggests",
        "internal/recipe/scalar.go",
        "n, err := strconv.ParseInt(s.text, 10, 64)",
        "n, err := strconv.ParseInt(s.text, 0, 64)",
        "TestANumberInARecipeMeansWhatItLooksLike",
    ),
    (
        # Guessing instead of refusing. A spelling we cannot read becomes zero
        # and the run goes ahead, which is the same silence in the other
        # direction - seed 0x10 would quietly become seed 0.
        "a spelling we cannot read is guessed at instead of refused",
        "internal/recipe/scalar.go",
        "if err != nil {",
        "if err != nil && false {",
        "TestASpellingThatOnlyYAMLCallsANumberIsRefused",
    ),
    (
        # Every file of a target then draws the same size. That still hits the size
        # to the byte and still repeats across runs, so the exact size guard and
        # the determinism guard both stay green while the feature is useless.
        "every file of a range draws the same size",
        "internal/engine/drawsizes.go",
        "r := core.NewRand(core.SizeSeed(targetSeed, i))",
        "r := core.NewRand(core.SizeSeed(targetSeed, 0))",
        "TestARangeIsRepeatableAndStaysInsideItsEnds",
    ),
    (
        # Sizes that depend on how many files were asked for. This is rule 2
        # broken, and it stays invisible until somebody raises a count and every
        # hash below it moves for no reason they can see.
        "the size of a file depends on how many files were asked for",
        "internal/engine/drawsizes.go",
        "r := core.NewRand(core.SizeSeed(targetSeed, i))",
        "r := core.NewRand(core.SizeSeed(targetSeed, i+len(t.Sizes)))",
        "TestRaisingTheCountOfARangeLeavesTheEarlierFilesAlone",
    ),
    (
        # Judged at the wrong end of the range. A range whose low end the format
        # cannot deliver then passes planning, and whether the run works at all
        # depends on what the seed happened to draw.
        "a range is judged at its high end instead of its low end",
        "internal/engine/drawsizes.go",
        "\t\tfirst.Bytes = t.Range.Min",
        "\t\tfirst.Bytes = t.Range.Max",
        "TestARangeIsJudgedByItsLowEndRatherThanByWhatWasDrawn",
    ),
    (
        # Two ways of stating a size then stop being an error and one of them wins
        # in silence. The recipe still produces files of an exact size, so only a
        # guard that asks about the pairing itself notices.
        "two size declarations in one target stop being an error",
        "internal/recipe/target.go",
        "case rt.SizeRange != nil && rt.Size != nil:",
        "case rt.SizeRange != nil && rt.Size != nil && false:",
        "TestTwoSizeDeclarationsInOneTargetAreRefused",
    ),
    (
        # Every archive of the target comes out at the low end of its range. The
        # sizes are still exact, still inside the range and still repeatable, so
        # the only thing that gives it away is that nothing varies.
        "a container range collapses onto its low end",
        "internal/engine/drawsizes.go",
        "span := uint64(t.Range.Max - t.Range.Min)",
        "span := uint64(0)",
        "TestAContainerTakesARangeAndStillHoldsItsContents",
    ),
    (
        # The flag stops drawing and gives every file the low end instead. Both
        # surfaces still work on their own, and they quietly disagree - which is
        # the failure that makes a copied command line untrustworthy.
        "the range flag stops meaning what the range key means",
        "internal/cli/generate.go",
        "lo, hi, err := core.ParseSizeRange(g.sizeRange)",
        "hi, lo, err := core.ParseSizeRange(g.sizeRange)",
        "TestTheRangeFlagAndTheRangeKeyAgree",
    ),
    (
        # Three files at the limit rather than one either side of it. The count is
        # right, the sizes are exact and the whole point of a boundary set is
        # gone.
        "a boundary set from the flag is three files at the limit",
        "internal/cli/generate.go",
        "sizes, err := core.BoundarySizes(limit)",
        "sizes, err := core.BoundarySizes(limit + 1)",
        "TestTheBoundaryFlagGivesTheThreeSizesAroundTheLimit",
    ),

    # TAR.GZ. Its size comes from arithmetic rather than from measuring the
    # structure the way every other format here does, so most of these break the
    # arithmetic in a different place. Each one leaves the tree compiling and
    # the file looking ordinary.
    (
        # The stream closes with an empty stored block that costs five bytes.
        # Drop it from the sum and every archive is five bytes longer than the
        # plan said.
        "the closing stored block is counted twice",
        "internal/format/targz/size.go",
        "return f.base + f.perBlock*blocks + tarLen",
        "return f.base + f.perBlock*(blocks+1) + tarLen",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        # Tar pads every entry up to a whole 512 byte block. Stop rounding and
        # the arithmetic is right only for entries that happen to be a multiple
        # of 512, which the default shape is.
        "the tar block padding stops being counted",
        "internal/format/targz/size.go",
        "return n + tarBlock - r",
        "return n",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        # The comment costs its length plus the byte that ends it.
        #
        # It moved on 2026-09-01, and where it moved FROM is the interesting
        # half. Until that day padding rode in the comment, so this off by one
        # was absorbed: the solve built a comment one byte longer and the
        # archive still came out exact, leaving only the floor to shift - which
        # is what the minimum guard watches, and it was aimed there for that
        # reason, measured 2026-08-04.
        #
        # Padding rides in the extra field now and the comment carries the
        # label alone, so nothing absorbs it and the byte lands on the size.
        # The full run called this a HOLE, correctly: the named guard stayed
        # green. Measured by hand the same day - three guards go red on it now,
        # and the arithmetic guard is the one that owns the fact.
        "the comment terminator stops being counted",
        "internal/format/targz/size.go",
        "return int64(len(comment)) + 1",
        "return int64(len(comment))",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        # Compression makes the output size unknowable without compressing, so
        # the plan and the file part company. This is the decision the format
        # rests on, mutated.
        "the archive starts compressing, so its planned size stops being real",
        "internal/format/targz/size.go",
        "gzip.NewWriterLevel(w, m.squeeze.Level)",
        "gzip.NewWriterLevel(w, gzip.BestSpeed)",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        # One byte above the floor is unreachable without a label, because Go
        # will not emit a zero length comment. Accept it and a file comes out a
        # byte short of what was ordered.
        "the size one byte above the floor stops being refused",
        "internal/format/targz/size.go",
        "\t\tcomment, withExtra, extra, ok := place(label, want)\n\t\tif !ok {",
        "\t\tcomment, withExtra, extra, ok := place(label, want)\n\t\tif !ok && false {",
        "TestTarGzHasExactlyOneGapAboveItsFloor",
    ),
    (
        # Padding never leaves the comment, so a 300 kB archive carries a 300 kB
        # gzip comment. The size is still exact and 7-Zip refuses the whole file
        # - measured 2026-08-04, the ceiling is 65 535 B. This is the mutation
        # that has to be caught by a reference tool rather than by a size.
        "padding above the comment limit stays in the comment",
        "internal/format/targz/size.go",
        "if want <= maxCost() {",
        "if want <= maxCost() || true {",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # Drop a format from the structural checker and the reference tool test
        # simply skips that layer, in silence. That is why the list is declared
        # and compared rather than trusted.
        "a format loses its structural check without anything saying so",
        "internal/oracle/oracle.go",
        'case "png", "wav", "pdf", "zip", "targz", "log", "csv", "json", "xml", "svg", "html",',
        'case "png", "wav", "pdf", "zip", "log", "csv", "json", "xml", "svg", "html",',
        "TestTheStructuralCheckerCoversEveryFormatItShould",
    ),
    (
        # When contains decides the size, the label states a measurement of the
        # archive the label is inside, so the two settle against each other.
        # Stop at the first round and the size is whatever the first guess was.
        #
        # Aimed first at the starting value of that loop, by adding one to it,
        # and that stayed GREEN - measured 2026-08-04. The loop converges from
        # any starting point, so it corrects the mutation. Worth knowing about
        # the design rather than about the guard.
        "an archive sized by its contents stops settling with its own label",
        "internal/format/targz/targz.go",
        "if measured == size {",
        "if measured == size || true {",
        "TestATarGzDeclaringItsContentsNeedsNoSizeAndTheDryRunIsExact",
    ),
    (
        # Members are numbered per format so two groups of the same format do
        # not both start at 0001. Freeze the number and they collide inside the
        # archive - tar takes two entries under one name without complaint, so
        # nothing but a name check notices.
        "two members of an archive are written under one name",
        "internal/format/targz/targz.go",
        'name: prefix + fmt.Sprintf("%s_%04d%s", g.Format, numbered[g.Format], desc.Extension),',
        'name: prefix + fmt.Sprintf("%s_%04d%s", g.Format, 1, desc.Extension),',
        "TestTheMembersOfATarGzAreRealFilesOfEveryFormatAsked",
    ),
    (
        # A format losing the only case that pins its bytes. Until 2026-08-04
        # nothing noticed: the neighbouring check compares how many cases there
        # are against how many golden values there are, and both counts stay
        # right when a case changes which format it produces.
        #
        # Aimed at a format with ONE case. This aimed at md until 2026-09-16,
        # and md had grown two more cases (its encodings, 2026-09-07), so
        # turning one of them into txt left md pinned by the other two and
        # the guard green - a mutation of one of two layers moves nothing.
        # pdf has one case, measured by counting Format: fields in the file.
        "a format quietly stops having its bytes pinned anywhere",
        "internal/guard/generatorbytes_test.go",
        "\"png_64kib\": {ID: \"g\", Format: \"png\",",
        "\"png_64kib\": {ID: \"g\", Format: \"txt\",",
        "TestEveryFormatHasAGoldenValue",
    ),
    (
        # The checklist writes format names the way a person does, so TAR.GZ has
        # to meet the id targz. Stop folding the dot away and the one format
        # whose name carries one looks missing - which is the half of this guard
        # that does the work, since every other name matches already.
        "the fidelity checklist stops recognising a name with a dot in it",
        "internal/guard/registry_test.go",
        'listed[strings.ToLower(strings.ReplaceAll(strings.TrimSpace(m[1]), ".", ""))] = true',
        "listed[strings.ToLower(strings.TrimSpace(m[1]))] = true",
        "TestEveryFormatHasARowInTheFidelityChecklist",
    ),
    (
        # Reading the wrong part of the document. The format file is full of
        # tables whose first cell is a bold name, so a guard pointed at the
        # wrong section would happily find rows and check nothing.
        "the fidelity checklist is looked for in the wrong section",
        "internal/guard/registry_test.go",
        'start := strings.Index(text, "## 6.1")',
        'start := strings.Index(text, "## 7.")',
        "TestEveryFormatHasARowInTheFidelityChecklist",
    ),
    (
        # The real defect, put back. The minimum table called itself indicative
        # and nothing read it, so it said images start at 30 to 70 B while PNG
        # refused anything under 73 - outside its own stated range, with a green
        # suite. Wrong number in the table is the first of the two shapes.
        "the minimum table states a number the registry does not refuse at",
        "docs/MVP-FORMATS.md",
        "| **PNG** | **74 B**",
        "| **PNG** | **70 B**",
        "TestTheMinimumTableCarriesTheMeasuredNumberForEveryBuiltFormat",
    ),
    (
        # And the second shape, which is how the table drifted in the first
        # place: a built format has no row of its own and hides inside a
        # grouped range. ZIP shared a cell with 7Z and a span of 22 to 35 B
        # while the registry refused under 156.
        "a built format loses its own row in the minimum table",
        "docs/MVP-FORMATS.md",
        "| Archiwa | **ZIP** | **156 B**",
        "| Archiwa | ZIP · 7Z | 22-35 B |\n| | unused | **156 B**",
        "TestTheMinimumTableCarriesTheMeasuredNumberForEveryBuiltFormat",
    ),
    (
        # The real defect, put back. ZIP declared entry_size as 8kb and used
        # 4096, so tfg formats printed one answer and generating without the
        # setting gave the other. Both numbers were written by hand in two
        # places and nothing compared them - the declaration is only read for
        # printing. This is that drift, reintroduced by hand.
        "a container goes back to ignoring the default it advertises",
        "internal/format/archive/archive.go",
        "sizeProperty(id, r.Properties, keys.Size, defaultSize)",
        "sizeProperty(id, r.Properties, keys.Size, 4096)",
        "TestADeclaredDefaultIsTheOneTheFormatUses",
    ),
    (
        # The other half: a guard that compares nothing passes. Skipping every
        # declared default leaves the loop with no work, and without the count
        # at the end this would be a green test that looked at zero formats.
        "the default check skips every property it was written for",
        "internal/guard/declareddefaults_test.go",
        'if p.Default == "" {',
        'if p.Default == "" || true {',
        "TestADeclaredDefaultIsTheOneTheFormatUses",
    ),
    (
        # The class a file belongs to stops reaching the manifest. Nothing
        # about the file changes, so every other guard stays green and the
        # assertion somebody wanted to write about a whole class has nothing
        # to stand on.
        "the group of a file never arrives in the manifest",
        "internal/engine/engine.go",
        "Group:         f.Target.Group,",
        'Group:         "",',
        "TestAGroupInTheRecipeReachesTheManifest",
    ),
    (
        # Written out as an empty string rather than left out. A consumer then
        # cannot tell "no class" from "a class called nothing", and the
        # difference only shows in the raw bytes somebody's script reads.
        "a file with no group says it has one called nothing",
        "internal/manifest/manifest.go",
        'Group string `json:"group,omitempty"`',
        'Group string `json:"group"`',
        "TestAGroupInTheRecipeReachesTheManifest",
    ),
    (
        # The drift that was found on 2026-08-04, put back. validate used to
        # build its own engine targets and left this out, so it planned the
        # numbered names for a boundary set instead of under_1b, at_limit
        # and over_1b - and refused recipes generate accepts.
        "validate goes back to imagining the wrong names for a boundary set",
        "internal/cli/generate.go",
        "BoundaryLimit:    t.BoundaryLimit,",
        "BoundaryLimit:    0,",
        "TestValidateAgreesWithGenerateAboutBoundaryNames",
    ),
    (
        # The other half of that pair. Teaching validate to stop imagining
        # names must not turn into teaching it to accept everything, so the
        # reverse case needs its own break: the collision check itself, seen
        # from the command that exists to run it before a commit.
        "two targets heading for one name stop being a collision",
        "internal/engine/names.go",
        "if !clash {",
        "if !clash || true {",
        "TestValidateStillCatchesARealBoundaryCollision",
    ),
    (
        # A preset expanding into something this build will not read. It is
        # the same bytes that eject prints and a run consumes, so a preset
        # that produces an unreadable recipe is one nobody can run and nobody
        # can edit either.
        "a preset expands into a recipe the parser refuses",
        "internal/preset/limitset.go",
        "\t\t\tFormat: s.desc.ID,\n",
        '\t\t\tFormat: s.desc.ID + "x",\n',
        "TestEveryPresetExpandsIntoARecipeThisBuildAccepts",
    ),
    (
        # The flip moved off the limit. Seven files still appear, all of them
        # the right size, and the one statement the preset exists to make is
        # wrong - the file ON the limit is the one people argue about.
        "the file on the limit is expected to be rejected",
        "internal/preset/limitset.go",
        "accept: true})",
        "accept: false})",
        "TestSizeBoundariesPutsTheFlipExactlyAtTheLimit",
    ),
    (
        # PR7 and the rule against silence. A limit too small for the files
        # below it produces part of a set instead of a refusal, and the part
        # that fits is never the part the run was about.
        "a set that cannot be completed is delivered in part",
        "internal/preset/limitset.go",
        "if one.size >= floor {",
        "if one.size >= floor || true {",
        "TestSizeBoundariesRefusesASetItCannotCompleteRatherThanPartOfIt",
    ),
    (
        # The budget somebody plans disk space around. A figure typed beside the
        # code was wrong by a factor of three and a half for three days, so this
        # one is counted from the plan - and an off by one in the counting puts
        # it right back to being a number nothing checks.
        "the budget counts all but one of the files it would write",
        "internal/cli/preset.go",
        "Bytes: engine.TotalBytes(planned), Formats: formats,",
        "Bytes: engine.TotalBytes(planned[:len(planned)-1]), Formats: formats,",
        "TestTheBudgetShownIsTheBudgetWritten",
    ),
    (
        # PR5, in the form that cannot be a promise: eject and a run consume the
        # same bytes. Expanding on the declared defaults instead of what the
        # caller gave makes eject print a recipe for a different set than the one
        # the same command line would produce.
        "eject expands on the defaults rather than on what was given",
        "internal/cli/presetcmd.go",
        "expanded, err := preset.Expand(id, givenPresetArgs(fs, p))",
        "expanded, err := preset.Expand(id, nil)",
        "TestEjectingAPresetAndRunningItGivesTheSameRunBackDefaultsIncluded",
    ),
    (
        # Untouchable rule 5. A limit we invented and a limit somebody measured
        # produce files that look identical and mean opposite things, and the
        # manifest is the only place that outlives the sentence saying which.
        "the manifest stops saying which numbers were ours",
        "internal/cli/preset.go",
        "Defaulted:  e.Defaulted,",
        "Defaulted:  nil,",
        "TestTheManifestSaysWhichNumbersWereOursRatherThanTheCallers",
    ),
    (
        # "tfg preset eject x > my.yaml" has to give a file that is a recipe.
        # Prose on the data channel puts a sentence about our placeholder limit
        # inside a file somebody is about to commit.
        "the note about an invented limit lands inside the ejected recipe",
        "internal/cli/presetcmd.go",
        # Anchored on the write that follows, because the run path prints the
        # same note and the pattern alone would match both.
        "sayNotes(expanded.Notes(), errOut)\n\tif to.set {",
        "sayNotes(expanded.Notes(), out)\n\tif to.set {",
        "TestEjectPutsTheRecipeOnStandardOutputAndTheRestBesideIt",
    ),
    (
        # CLI.md section 6: a parameter clashing with a global flag is a mistake
        # in the preset, caught at startup. The flag package answers a name
        # registered twice with a panic, so this one has to be caught before it
        # ships rather than reported by somebody hitting it.
        "a preset declares a parameter that is already a global flag",
        "internal/preset/sizeboundaries.go",
        'Name: "spread", Kind: format.PropertyText,',
        'Name: "seed", Kind: format.PropertyText,',
        "TestNoPresetDeclaresAParameterThatIsAlreadyAFlag",
    ),
    (
        # A preset id nobody registered is a typo in the invocation. Without its
        # own line it falls through to RUNTIME, which tells CI this program has a
        # bug rather than that somebody mistyped a name.
        "a preset nobody registered is reported as a fault of the tool",
        "internal/cli/errors.go",
        "var unknownPreset *preset.UnknownPresetError",
        "var unknownPreset *preset.ImpossibleError",
        "TestEveryPresetEndingHasTheCodeItShould",
    ),
    (
        # The message listing the ways to say what to produce named two of them
        # for as long as there were two. A preset is the third, and it is the one
        # somebody reaching for this tool first would want.
        "the message about producing nothing goes back to naming two ways out",
        "internal/cli/generate.go",
        "name a recipe file, or pick a preset with --preset.",
        "or name a recipe file.",
        "TestNothingToProduceNamesAllThreeWaysOut",
    ),
    (
        # Which flags exist depends on which preset was named, so the scan that
        # finds it has to handle every spelling the flag package accepts. Missing
        # one turns a documented invocation into "flag provided but not defined".
        "the preset written with an equals sign is not found before parsing",
        "internal/cli/preset.go",
        "\t\tif hasValue {\n\t\t\treturn value\n\t\t}",
        "\t\tif hasValue && false {\n\t\t\treturn value\n\t\t}",
        "TestReadingThePresetOutOfTheArgumentsHandlesBothSpellings",
    ),
    (
        # A parameter that parses and never reaches the plan is the worst shape
        # of this: the run succeeds, the files are the right size, and the set is
        # built around a number nobody asked for.
        "a parameter the caller gave never reaches the expansion",
        "internal/cli/preset.go",
        'if names[f.Name] && f.Value.String() != "" {',
        'if names[f.Name] && f.Value.String() != "" && false {',
        "TestAPresetParameterBecomesAFlagAndAnUnknownOneDoesNot",
    ),
    (
        # The window renders and shows nothing - a screen built from parts that
        # draw no content. It passes every structural check, exactly as SVG did
        # at its minimum size, which is why both are guarded by counting
        # colours rather than by parsing.
        # Aimed at the composition rather than at one label, and that is a
        # measurement rather than a preference. Emptying a single label leaves
        # the colour count at 188, identical to the full screen, because the
        # number counts the anti-aliasing palette of the font rather than the
        # amount of text. Dropping every section takes it to 1.
        "every screen renders as an empty rectangle",
        "internal/gui/window/open.go",
        # Moved here on 2026-08-11, and the move is worth the note. It emptied
        # parts.Screen until then, which stopped proving anything the moment the
        # actions were pinned outside the scrolling part: an empty form still
        # draws a bar with three buttons on it, so the guard stayed green on a
        # screen with no form at all. The runner reported that the same day.
        #
        # Emptying what the window is handed is the question the guard actually
        # asks - does anything get drawn - and it survives the layout moving
        # again, because it does not name a layout.
        "return NewGenerate(h).Object()",
        "return container.NewVBox()",
        "TestTheWindowActuallyDrawsSomething",
    ),
    (
        # The sentence somebody using only the window has no other way to read:
        # the licence covers the tool and not the files it makes. It reached
        # the terminal from 2026-08-04 and reached the window from today.
        "the licence screen stops saying who owns the generated files",
        "internal/version/version.go",
        "The files you generate are yours.",
        "The files you generate are covered too.",
        "TestTheAboutScreenSaysWhoOwnsTheGeneratedFiles",
    ),
    (
        # Two surfaces, one constant. They sit on the same layer and cannot
        # import each other, which is the shape in which a second copy gets
        # written and nobody compares the two again.
        "the window quotes its own copy of the licence instead of the shared one",
        "internal/gui/window/about.go",
        # Repointed 2026-08-11, when the notice moved into a card like every
        # other block on every other screen.
        # And again 2026-09-23, when the window joined the notice's lines into
        # paragraphs so it wraps to the window.
        "parts.Prose(paragraphs(text.Licence(version.LicenceNotice)))",
        "parts.Prose(\"Released under the GNU General Public License, version 3.\")",
        "TestTheWindowAndTheCommandQuoteTheSameLicence",
    ),
    (
        # D16 and untouchable rule 8. The toolkit brings net/http and the
        # command line must not inherit it - a build for a server carrying a
        # socket cannot demonstrate the promise to anybody auditing it.
        # Breaks the subject rather than the judge. Shortening the guard's own
        # list of network packages changes nothing while the binary links none
        # of them - it proves the list, not the binary. This puts a socket in
        # the command line, which is the thing that must never happen.
        "the command line binary gains a socket, as it would from one careless import",
        "cmd/tfg/main.go",
        '\t"context"\n\t"os"',
        '\t"context"\n\t_ "net/http"\n\t"os"',
        "TestTheCommandLineBinaryCannotReachTheNetwork",
    ),
    (
        # Same shape one layer up: the toolkit's own imports are outside our
        # control and written down as such, ours are not. A window that fetches
        # something itself is the case this watches, and it would be invisible
        # to the scan beside it, which excludes the whole gui tree.
        "our own window code starts reaching for the network",
        "internal/gui/window/about.go",
        '\t"fyne.io/fyne/v2"\n',
        '\t"fyne.io/fyne/v2"\n\t_ "net/http"\n',
        "TestOurOwnWindowCodeDoesNotReachTheNetwork",
    ),
    (
        # The first window that does work rather than only saying what the tool
        # is. Everything below breaks the subject rather than the judge: the
        # guards read the widget tree a person is shown, so shortening a list
        # inside a guard would leave the window offering exactly what it offers
        # and prove nothing.
        # G6, and the one thing a window does better than a command line rather
        # than merely as well. --dry-run has to be known about and remembered.
        "Generate comes before Preview, so the cost arrives after the writing",
        "internal/gui/window/runactions.go",
        "r.previewBtn, r.generateBtn,",
        "r.generateBtn, r.previewBtn,",
        "TestPreviewComesBeforeGenerate",
    ),
    (
        # A preview that writes is not a preview. The whole value of the button
        # is that the answer costs nothing, on a machine where a preset can run
        # to several gigabytes.
        "the preview writes the files it was only supposed to count",
        "internal/gui/window/run.go",
        "\topt.DryRun = true\n",
        "\topt.DryRun = false\n",
        "TestPreviewSaysTheCostAndWritesNothing",
    ),
    (
        # The menu is filled from the registry so a fourteenth format joins it
        # on the day it is registered. Dropping one leaves twelve formats a
        # person can pick and thirteen the engine has, which is D1 breaking
        # quietly - the window simply offers less.
        "the window offers fewer formats than the registry holds",
        "internal/gui/window/generate.go",
        "\tids := format.IDs()",
        "\tids := format.IDs()[1:]",
        "TestTheWindowOffersEveryFormatTheRegistryHas",
    ),
    (
        # What a setting takes is the sentence "tfg formats" prints, read from
        # the declaration. Losing it leaves a box with a name over it and no way
        # to know a page count stops at five thousand.
        "a generated field stops saying what the setting takes",
        "internal/gui/parts/property.go",
        "\tdetail := allowedOnAScreen(p)",
        '\tdetail := ""',
        "TestTheWindowDrawsAFieldForEveryDeclaredProperty",
    ),
    (
        # G1. A window that words a refusal itself is a second copy of the
        # rules, and the copy is the one that drifts - after which the window
        # accepts or refuses something the command line does not, and nobody
        # compares the two.
        "the window words a size refusal itself instead of asking the engine",
        "internal/gui/window/generate.go",
        "\tbytesWanted, err := core.ParseSize(g.size.Text)\n\tif err != nil {\n\t\tbad = append(bad, saying(format.SettingSize, err))\n\t}",
        # errors.New rather than fmt.Errorf, because this file does not import
        # fmt and the mutation reported BROKEN for two runs - a mutation that
        # does not compile proves nothing about the guard it names.
        '\tbytesWanted, err := core.ParseSize(g.size.Text)\n\tif err != nil {\n\t\tbad = append(bad, errors.New("that is not a size this window accepts"))\n\t}',
        "TestTheWindowAsksTheEngineWhetherASizeIsGood",
    ),
    (
        # G9 as a property of what arrives rather than of the wording. A
        # refusal in this tool has four parts and an area that shortens it
        # delivers one of them, which is the failure mode the rule was written
        # against.
        "the error area shortens the refusal it was given",
        "internal/gui/parts/field.go",
        "\ta.label.SetText(text)",
        "\ta.label.SetText(text[:min(len(text), 40)])",
        "TestTheWindowShowsTheWholeRefusal",
    ),
    (
        # A box that is typed into and dropped on the way to the engine looks
        # exactly like one that works, and the parity list would say the window
        # can set a format property while the window sets nothing.
        "the generated property fields stop reaching the run",
        "internal/gui/window/generate.go",
        "\t\tProperties: g.properties(),",
        "\t\tProperties: nil,",
        "TestWhatIsTypedOnTheScreenIsWhatGetsWritten",
    ),
    (
        # A surface that writes files and forgets the record leaves them with
        # nothing able to remove them. The command line learned this on
        # 2026-08-03 and the window inherits the rule rather than the memory.
        "the window writes the files and never records them",
        "internal/gui/window/runreport.go",
        "\tif opt.DryRun || res == nil || !res.Started {",
        "\tif opt.DryRun || res == nil || res.Started {",
        "TestGeneratingFromTheWindowWritesTheFilesAndTheManifest",
    ),
    (
        # G7, and the half that is easy to leave out. Cancelling and closing at
        # once ends the process while the engine is still winding down, so the
        # manifest describing what did get written never lands - and that
        # manifest is the only thing cleanup can work from. Measured on
        # 2026-08-05: red three times out of three, with no manifest at all.
        "closing the window cancels the run without waiting for it",
        "internal/gui/window/run.go",
        "\t\tcancel()\n\t\t<-done\n\t}\n\n\tgo func() {\n\t\tplanned, planErr := engine.PlanContext(ctx, targets, opt)\n\t\tif planErr != nil {\n\t\t\t// A refusal",
        "\t\tcancel()\n\t\t_ = done\n\t}\n\n\tgo func() {\n\t\tplanned, planErr := engine.PlanContext(ctx, targets, opt)\n\t\tif planErr != nil {\n\t\t\t// A refusal",
        "TestClosingTheWindowDuringARunStopsItAndWaitsForIt",
    ),
    (
        # Found by running the built window and looking at it, on 2026-08-05.
        # The format menu was empty while every guard was green, because this
        # test package pulls the registrations in for its own use - so the
        # registry was full everywhere a test could see and empty in the binary
        # somebody would be handed. No test that imports the code can catch
        # that, which is why the guard asks the compiler instead.
        "the window binary stops carrying the format registrations",
        "internal/gui/window/generate.go",
        '\t_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"\n',
        "",
        "TestBothBinariesCarryTheFormatRegistrations",
    ),
    (
        # The preset menu is the registry's, so a second preset joins it on the
        # day it is registered. This is the drift O59 already caught once, from
        # the other side: a preset was added and the parity guard stayed green
        # because nothing looked for one.
        "the window offers fewer presets than the registry holds",
        "internal/gui/window/preset.go",
        "\tids := preset.IDs()",
        "\tids := preset.IDs()[:0]",
        "TestTheWindowOffersEveryPresetThereIs",
    ),
    (
        # The question is the whole reason a preset is not a recipe with a name.
        # A list of ids says nothing about which one answers what somebody came
        # to ask, and the title is the id spelled nicely.
        "the preset screen shows the name instead of the question it closes",
        "internal/gui/window/preset.go",
        "p.about.Add(parts.Prose(text.PresetQuestion(chosen.ID, chosen.Question)))",
        "p.about.Add(parts.Prose(text.PresetQuestion(chosen.ID, chosen.Title)))",
        "TestTheWindowDrawsAFieldForEveryPresetParameter",
    ),
    (
        # Turning a parsed recipe into engine targets is written twice, because
        # the two surfaces cannot share it. Exactly this conversion drifted
        # before: validate and generate each had one and validate had lost
        # BoundaryLimit. The guard does not compare the two lists of fields - it
        # runs one preset from both surfaces and compares the bytes.
        "the window drops one field on the way from recipe to engine",
        "internal/gui/window/preset.go",
        "\t\tLabel:            t.Label,",
        "\t\tLabel:            false,",
        "TestThePresetScreenAndTheCommandLineProduceTheSameRun",
    ),
    (
        # Untouchable rule 5. A limit belongs to somebody else's system, and a
        # set built around a number we invented carries expectations that read
        # exactly like a set built around the real one. The run says so out loud
        # or it is lying by omission.
        "the window stops saying which number it invented",
        "internal/gui/window/preset.go",
        "\tp.notes = notes\n",
        "\tp.notes = notes[:0]\n",
        "TestThePresetScreenSaysWhichNumbersWereOurs",
    ),
    (
        # A dot is legible in a terminal, where you typed your way into the
        # directory, and opaque in a window started from a desktop. This is the
        # one part of this tool that writes into other people's directories.
        #
        # It mutates the PRESET screen although the guard reads both, and that
        # is measured rather than chosen. Open ends with showGenerate(), which
        # carries pre.OutDir() into the generate screen, so the generate
        # screen's own starting value is overwritten before anything can look
        # at it - mutating it there changed nothing and the guard stayed green
        # through a run that reported it as a hole. The carry arrived with the
        # window telling one answer about where the files go, so a real change
        # quietly emptied a mutation that still matched its pattern and still
        # compiled. Staleness cannot see this: the text is all still there.
        # And it moved BACK on 2026-08-12, for the same reason in the other
        # direction. Selecting the preset tab carries the generate screen's
        # answer over - open.go, OnSelected - so the preset screen's own
        # starting value is overwritten before the guard reads it, and this
        # entry reported NOT CAUGHT while the behaviour was perfectly intact.
        # Twice now the same mutation has been emptied by a change somewhere
        # else, both times invisible to staleness because the text still
        # matches, and both times found only by running it.
        #
        # The generate screen is where it means something: its value is the one
        # that gets carried, so a dot there is a dot on both screens.
        "the window goes back to offering a dot as the output directory",
        "internal/gui/window/generate.go",
        "\tg.outDir = entry(startingDirectory(), \"\")",
        "\tg.outDir = entry(\".\", \"\")",
        "TestBothScreensSayWhereTheFilesWillGo",
    ),
    (
        # "text" is a word where a description should be. Seen on screen as
        # "text, default 1B,1kb,1mb" for a setting that wants a list of sizes -
        # the declaration knew that and had nowhere to put it.
        "a text setting goes back to describing itself as text",
        "internal/format/format.go",
        "\t\twhat = p.Shape",
        '\t\twhat = "text"',
        "TestNoTextSettingDescribesItselfAsText",
    ),
    (
        # The entries have commas inside them, so run together they read as
        # three times as many findings as there are.
        "what a preset finds is run together into one sentence",
        "internal/gui/window/preset.go",
        # One label instead of a list. Written without strings.Join on purpose:
        # preset.go does not import strings, and a mutation that does not build
        # is reported as BROKEN and proves nothing at all.
        "\t\tp.about.Add(parts.Bullets(text.PresetCatches(chosen.ID, chosen.Catches)))",
        "\t\tp.about.Add(parts.Note(chosen.Catches[0]))",
        "TestWhatAPresetFindsIsShownAsSeparateLines",
    ),
    (
        # The real defect, put back: a word reaching a person from outside the
        # package that holds the words. This is the state the whole window was
        # in until 2026-08-10, when about forty labels and messages sat as
        # literals across six files and nobody could count them.
        "a label goes back to being a literal in the window",
        "internal/gui/window/open.go",
        '&parts.Tab{ID: tabPresets, Text: text.TabPresets(), Content: pre.Object()},',
        '&parts.Tab{ID: tabPresets, Text: "Presets", Content: pre.Object()},',
        "TestTheWindowSaysNothingItDoesNotSayFromTheTextPackage",
    ),
    (
        # The other half, and the one the guard missed on its first run. Glueing
        # a value onto a literal hides the literal from a check that reads
        # arguments alone - and it is also the shape that translates worst,
        # because languages disagree about where a name goes in a phrase.
        "a sentence goes back to being glued together in the window",
        "internal/gui/window/preset.go",
        "parts.Subheading(text.PresetCatchesHeading())",
        'parts.Subheading("Typically" + " finds:")',
        "TestTheWindowSaysNothingItDoesNotSayFromTheTextPackage",
    ),
    (
        # The real defect, put back. Each screen used to hold its own output
        # box, both starting at the working directory - so they agreed until
        # somebody changed one and then silently disagreed. Reported from use
        # on 2026-08-11: files went to a chosen directory, the preset screen
        # was opened, and its run wrote somewhere else.
        "the chosen directory is carried backwards, to the screen somebody just left",
        "internal/gui/window/open.go",
        "to.SetOutDir(from.OutDir())",
        "from.SetOutDir(to.OutDir())",
        "TestBothScreensAgreeWhereTheFilesGo",
    ),
    (
        # And the way back, because carrying it one way leaves the other
        # direction exactly as confusing as it was.
        "the window forgets which screen somebody came from, so it carries nothing",
        "internal/gui/window/open.go",
        "showing = item.ID",
        "_ = item.ID",
        "TestBothScreensAgreeWhereTheFilesGo",
    ),
    (
        # A control that is always there and never usable is a question the
        # screen keeps asking and answering itself. Reported from looking at the
        # window on 2026-08-11. This takes away the hiding at the END of a run,
        # which the constructor's hiding covers up until the first run finishes.
        "cancel sits on the screen with nothing to cancel",
        "internal/gui/window/runbusy.go",
        "\tb.cancel.Disable()\n\tb.cancel.Hide()\n\tb.bar.Hide()",
        "\tb.cancel.Disable()\n\tb.bar.Hide()",
        "TestCancelIsOnlyThereWhenThereIsSomethingToCancel",
    ),
    (
        # The way between screens used to be a button in the row of actions
        # under the last field, so changing screen meant scrolling past the
        # whole form. Reported from use 2026-08-11. Tabs added while the buttons
        # stay is the likely half done move, and it looks fixed from the top.
        "the way between screens goes back to a button at the foot",
        "internal/gui/window/open.go",
        "\tgen := NewGenerate(h)\n\tpre := NewPreset(h)",
        "\tgen := NewGenerate(h, parts.NewButton(parts.Secondary, text.TabPresets(), func() {}))\n\tpre := NewPreset(h)",
        "TestMovingBetweenScreensIsTabsAndNotButtons",
    ),
    (
        # The focus colour painted rather than blended, which is what the
        # window shipped until 2026-08-12: the toolkit fills a whole menu with
        # this name and writes the chosen value across it, so an opaque value
        # put the format's name on solid blue at 2.28:1 against a threshold of
        # 4.5. Reported from the screen as a field that stays lit after the
        # choice.
        #
        # This entry used to lower the colour instead, back to the 1.16:1 the
        # window had before the palette was installed. The threshold it was
        # aimed at moved on 2026-08-12: the state is carried by a line now, so
        # what this colour has to meet is the one a reader needs THROUGH it.
        "the focus colour is painted rather than blended",
        "internal/gui/parts/theme.go",
        # The spacing here is whatever gofmt last decided, and it realigns the
        # whole map when the longest key changes - which it did the day hover
        # became an overlay. Staleness names it in a second when that happens.
        "theme.ColorNameFocus: overlay(0x00, 0xA5, 0xF0, 0x66),",
        "theme.ColorNameFocus: hex(0x00, 0xA5, 0xF0),",
        "TestThePaletteMeetsTheContrastItWasComputedFor",
    ),
    (
        # A field's name in the value's ink: two names, one colour, and the
        # form has one ink where the owner asked for two.
        "a field's name is inked like the value under it",
        "internal/gui/parts/theme.go",
        "ColorNameLabel:        hex(0xD3, 0xD3, 0xD7),",
        "ColorNameLabel:        hex(0xE6, 0xE6, 0xE8),",
        "TestThePaletteMeetsTheContrastItWasComputedFor",
    ),
    (
        # The program is dark only by the owner's decision, and the toolkit
        # hands the system's variant to every colour lookup - so branching on
        # it is one plausible looking edit away, and it would put a screen
        # nobody has ever seen on somebody else's desktop.
        "the window follows the desktop's light setting again",
        "internal/gui/parts/theme.go",
        "\tif c, ok := darkColours[name]; ok {\n\t\treturn c\n\t}\n\treturn o.Theme.Color(name, theme.VariantDark)",
        "\tpalette := darkColours\n\tif variant == theme.VariantLight {\n\t\tpalette = lightColours\n\t}\n\tif c, ok := palette[name]; ok {\n\t\treturn c\n\t}\n\treturn o.Theme.Color(name, variant)",
        "TestTheWindowKeepsOneLookWhateverTheDesktopSays",
    ),
    (
        # The light palette is kept measured although it is not installed, so
        # the numbers are there the day it is wanted. This is what proves they
        # are still real numbers rather than a copy of the dark ones.
        "the light palette stops being a palette of its own",
        "internal/gui/parts/theme.go",
        "palette := darkColours\n\tif variant == theme.VariantLight {\n\t\tpalette = lightColours\n\t}",
        "palette := darkColours",
        "TestTheLightAndDarkPalettesAreNotTheSame",
    ),
    (
        # The sweep found twelve of these by hand and the guard found three
        # more the same minute, in files nobody had thought to look in. The
        # defect is the dashes rather than which word follows them, so the
        # guard asks about the syntax and this puts one back. O79.
        "a message below the surfaces goes back to flag spelling",
        "internal/format/wav/wav.go",
        ', or drop the label"',
        ', or drop the label with --clean"',
        "TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling",
    ),
    (
        # The preset screen names its settings from the preset's own parameters
        # rather than from the recipe keys the engine uses, so the two screens
        # share the mechanism and not the vocabulary. This was the half left
        # undone when the generate screen's refusals were moved. O73.
        "a preset refusal stops saying which parameter it is about",
        "internal/preset/limitset.go",
        "\t\t\tSetting: s.setting,\n\t\t\tDetail:  what,",
        "\t\t\tSetting: \"\",\n\t\t\tDetail:  what,",
        "TestARefusalAboutAPresetSettingAppearsUnderIt",
    ),
    (
        # Without the setting on the error the window has nothing to place the
        # message by, and it falls back to the foot of the form - which is
        # where it sat, 748 px below the field it named. O73.
        "a refusal stops saying which setting it is about",
        "internal/engine/engine.go",
        "&RecipeError{Setting: SettingCount,\n\t\t\tDetail: core.Says(\"engine.NoFiles\",",
        "&RecipeError{\n\t\t\tDetail: core.Says(\"engine.NoFiles\",",
        "TestARefusalAboutOneSettingAppearsUnderIt",
    ),
    (
        # Moving a message is easy to do badly: showing it in both places looks
        # fixed in a screenshot of the top of the screen and is two copies of
        # one refusal.
        "the refusal is shown at the foot as well as beside the field",
        "internal/gui/window/runrefuse.go",
        # Repointed 2026-09-23, when placing a refusal moved out of the loop
        # into placed() and the loop lost a level.
        "\t\t\t\tfirst = where\n\t\t\t}\n\t\t\tcontinue\n\t\t}",
        "\t\t\t\tfirst = where\n\t\t\t}\n\t\t}",
        "TestARefusalAboutOneSettingAppearsUnderIt",
    ),
    (
        # A VBox hands its children the whole width it was given, which is the
        # whole window. Measured at 3862 px: every box was 3848 to 3854 px, so
        # the seed field holding "0" was nearly four thousand pixels wide. O72.
        "the form stretches to the window again",
        "internal/gui/parts/parts.go",
        "\twidth := fyne.Min(size.Width, ColumnWidth)",
        "\twidth := size.Width",
        "TestTheFormDoesNotRunToTheEdgeOfTheWindow",
    ),
    (
        # A change of look that the structural guards pass. Measured with
        # try-one on 2026-08-17: TestABoxForANumberIsTheWidthOfANumber STAYED
        # GREEN, because it compares the box against the constant rather than
        # against a number - widen the constant and it widens what it allows.
        # The stored picture is the only thing that notices.
        "the numeric fields get wider",
        "internal/gui/parts/tokens.go",
        "\tNumericWidth = 140",
        "\tNumericWidth = 260",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        # Everything else about this writer is a refusal, and a writer that
        # refuses correctly while putting the wrong bytes down would pass all of
        # them. This is the plain case, broken plainly.
        "the replacement writes something other than what was asked for",
        "internal/core/replace.go",
        "\tif _, err := f.Write(content); err != nil {",
        "\tif _, err := f.Write(content[:0]); err != nil {",
        "TestReplacingPutsTheContentThatWasAskedFor",
    ),
    (
        # The real defect, put back. A rename asks for permission on the
        # DIRECTORY rather than on the file, so read only stops a replacement on
        # Windows and does not stop it on Linux - measured on both with
        # tools/probes/atomic-replace. Refusing everywhere is the same rule this
        # project applies to file names that only work on one system.
        "a read only file stops being protected",
        "internal/core/replace.go",
        "\tif mode&0o200 == 0 {",
        "\tif mode&0o200 == 0 && false {",
        "TestAReadOnlyFileIsLeftAlone",
    ),
    (
        # The half written copy sits in somebody's repository, because a rename
        # across volumes is not one operation and it has to be beside the target.
        # The version this replaced cleaned up after a failed rename and not
        # after a failed write.
        "a failed replacement leaves its half written copy behind",
        "internal/core/replace.go",
        "\tif err := os.Rename(tmp, path); err != nil {\n\t\t_ = os.Remove(tmp)",
        "\tif err := os.Rename(tmp, path); err != nil {",
        "TestReplacingLeavesNoHalfWrittenCopyBehind",
    ),
    (
        # The arrangement the guard above rests on. A test that sets up a failure
        # it never triggers reports a clean directory because nothing happened in
        # it, which is the same shape as a guard that never reaches the code.
        "replacing onto a non empty directory stops failing",
        "internal/core/replace.go",
        "\tif err := os.Rename(tmp, path); err != nil {\n\t\t_ = os.Remove(tmp)\n\t\treturn &ReplaceError{Path: path, Err: err}\n\t}",
        "\tif err := os.Rename(tmp, path); err != nil {\n\t\t_ = os.Remove(tmp)\n\t\t_ = err\n\t}",
        "TestTheRenameFailureThatGuardIsBuiltOnReallyHappens",
    ),
    (
        # The real defect, put back. Of the four fields on those two rows the
        # size was the only one with nowhere for its refusal to land, and it is
        # the one most likely to get one - every format has a minimum. The
        # message appeared at the foot of the form, 900 px under the box.
        "a refusal about the size goes back to the foot of the form",
        "internal/format/format.go",
        "func (e *BelowMinimumError) AboutSetting() string { return SettingSize }",
        'func (e *BelowMinimumError) AboutSetting() string { return "" }',
        "TestARefusalAboutTheSizeAppearsUnderIt",
    ),
    (
        # The real defect, put back. The quiet look paints no fill and no
        # border at rest, so the way to stop a run arrives as bare text between
        # two filled buttons and reads as a disabled label.
        "Cancel goes back to the quiet look, which paints no surface",
        "internal/gui/window/run.go",
        "\tcancel := parts.NewButton(parts.Secondary, text.ButtonCancel(), r.onCancel)",
        "\tcancel := parts.NewButton(parts.Quiet, text.ButtonCancel(), r.onCancel)",
        "TestCancelIsDrawnAsAButton",
    ),
    (
        # The label switch stands under its name in the column of names since
        # 2026-09-15. Registered with no name it goes back to a bare square with
        # nothing to read beside it - the O72 defect the grid answers.
        "the label switch is registered with no name",
        "internal/gui/parts/fields.go",
        "return s.Add(setting, name, hint, detail, check)",
        'return s.Add(setting, "", hint, detail, check)',
        "TestTheLabelSwitchStandsUnderItsName",
    ),
    (
        # The focus ring drawn for the pointer as well as the keyboard, which is
        # what the toolkit did and what O207 is about: a mark meaning "the
        # keyboard is here" shown to somebody using the mouse.
        "the button draws its ring for the pointer too",
        "internal/gui/parts/button.go",
        "\tb.hovered = true",
        "\tb.hovered = true\n\tb.marked = true",
        "TestAButtonDrawsItsRingForTheKeyboardNotThePointer",
    ),
    (
        # The press forgotten only in MouseUp, which never comes back when the
        # pointer slides off before releasing - so the button stays drawn as
        # pressed for good (the Fyne guide, section 3.27).
        "the button keeps a press after the pointer leaves",
        "internal/gui/parts/button.go",
        "\tb.hovered, b.pressed = false, false",
        "\tb.hovered = false",
        "TestAButtonForgetsAPressWhenThePointerLeaves",
    ),
    (
        # The defect put back exactly as it was written on 2026-09-15: the
        # desktop driver delivers one press of space as the key AND the
        # character, so a button answering both is pressed twice. Green in
        # every guard that sent one call, red in the one that sends both.
        "the button presses on the space character as well as the space key",
        "internal/gui/parts/button.go",
        "func (b *Button) TypedRune(rune) {}",
        "func (b *Button) TypedRune(r rune) {\n\tif r == ' ' {\n\t\tb.Tapped(nil)\n\t}\n}",
        "TestOnePressOfSpacePressesAButtonOnce",
    ),
    (
        # The switch stops taking the keyboard on a press, which is where it
        # stood from 2026-09-15 to 2026-09-16: the driver unfocuses whatever
        # had the keyboard and the switch left it nowhere, so the next Space
        # flipped nothing and the next Tab started from the top. Found by an
        # outside review of #103.
        "a press on the switch stops taking the keyboard",
        "internal/gui/parts/toggle.go",
        "\tt.from.Take(t)\n\tt.SetChecked(!t.Checked)",
        "\tt.SetChecked(!t.Checked)",
        "TestAPressMovesTheKeyboardWithoutDrawingItsMark",
    ),
    (
        # The mark drawn for a press: the blue disc of 2026-08-18, in the new
        # switch's shape. FocusGained stops asking who brought the keyboard.
        "a press on the switch draws the keyboard mark",
        "internal/gui/parts/toggle.go",
        "\tif !t.from.Draws() {\n\t\treturn\n\t}\n\tt.mark()",
        "\tt.mark()",
        "TestAPressMovesTheKeyboardWithoutDrawingItsMark",
    ),
    (
        # The defect as it was written: one press of space is the key and the
        # character, and a switch answering both flips twice, back to where
        # it started.
        "the switch flips on the space character as well as the key",
        "internal/gui/parts/toggle.go",
        "func (t *Toggle) TypedRune(rune) {}",
        "func (t *Toggle) TypedRune(r rune) {\n\tif r == ' ' {\n\t\tt.Tapped(nil)\n\t}\n}",
        "TestOnePressOfSpaceFlipsASwitchOnce",
    ),
    (
        # The same drift on the segmented switch: a press chooses and leaves
        # the keyboard nowhere.
        "a press on a segment stops taking the keyboard",
        "internal/gui/parts/segments.go",
        "\ts.from.Take(s)\n\tif at := s.segmentAt(event.Position.X); at >= 0 {",
        "\tif at := s.segmentAt(event.Position.X); at >= 0 {",
        "TestAPressOnASegmentedSwitchPutsTheKeyboardOnItQuietly",
    ),
    (
        # A frozen switch that kept the keyboard moves its choice on the
        # arrows under a form drawn as frozen - the focus manager asks
        # Disabled only when it moves the focus.
        "a frozen segmented switch answers the arrows again",
        "internal/gui/parts/segments.go",
        "\tif event == nil || s.Disabled() || len(s.Options) == 0 {",
        "\tif event == nil || len(s.Options) == 0 {",
        "TestAFrozenControlAnswersNoKey",
    ),
    (
        # And the menu, through the toolkit's own Select.TypedKey, which asks
        # nothing about Disabled before moving the value on Left and Right.
        "a frozen menu answers the arrows again",
        "internal/gui/parts/ring.go",
        "\tif event == nil || c.Disabled() {\n\t\treturn\n\t}\n\t// The keyboard has been used",
        "\t// The keyboard has been used",
        "TestAFrozenControlAnswersNoKey",
    ),
    (
        # Back to the space bar alone, which is what the toolkit answers - so
        # Enter on a focused button does nothing, the silence this window
        # refuses everywhere else.
        "the button answers the space bar and not Enter",
        "internal/gui/parts/button.go",
        "\tcase fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:",
        "\tcase fyne.KeySpace:",
        "TestAButtonIsPressedByEnterAsWellAsSpace",
    ),
    (
        # A switch of fixed choices that takes a value it does not hold, which
        # is a box pretending to be a switch - a copy of a batch could land it
        # on a way of stating a size that is not one of the three.
        "the segmented switch takes a value it does not offer",
        "internal/gui/parts/segments.go",
        "\tif value == s.Selected || s.indexOf(value) < 0 {",
        "\tif value == s.Selected {",
        "TestASegmentedSwitchIgnoresAValueItDoesNotHold",
    ),
    (
        # The right arrow stops moving the choice, so the switch is a row the
        # keyboard can land on and not step through - which is the whole of what
        # a segmented control is over a label.
        "the segmented switch stops moving on the right arrow",
        "internal/gui/parts/segments.go",
        "\t\t\ts.SetSelected(s.Options[at+1])",
        "\t\t\ts.SetSelected(s.Options[at])",
        "TestASegmentedSwitchMovesTheChoiceWithTheArrows",
    ),
    (
        # Half of every field's explanation lives behind this button since
        # 2026-08-12. Without the button those sentences are written, shipped
        # and unreadable - which is worse than the wall of text they replaced,
        # because the wall could at least be read.
        # The button is added by the name cell of a field row since the form
        # became a grid (2026-09-14). The line this entry used to break lived
        # in withDetail, which nothing had called since that day - the full
        # run of 2026-09-16 found the entry green and the function dead.
        "the longer explanation loses the button that opens it",
        "internal/gui/parts/required.go",
        "\t\trow = append(row, newDetailButton(detail))",
        "\t\t_ = newDetailButton(detail)",
        "TestTheLongerExplanationOpensWhenAsked",
    ),
    (
        # Reported on 2026-08-12: it opened on a click, and people point at a
        # small letter i and wait. The toolkit has no tooltip - measured, there
        # is no such identifier anywhere in v2.8.0 - so hovering is three
        # methods answered by hand, and a hook answered but never wired looks
        # from outside exactly like one that works.
        "the explanation stops opening when the pointer arrives",
        "internal/gui/parts/detail.go",
        "func (b *DetailButton) MouseIn(e *desktop.MouseEvent) {\n"
        "\tb.Button.MouseIn(e)\n"
        "\tb.show()",
        "func (b *DetailButton) MouseIn(e *desktop.MouseEvent) {\n"
        "\tb.Button.MouseIn(e)",
        "TestTheLongerExplanationOpensWhenAsked",
    ),
    (
        # The real defect, put back. widget.PopUp wraps itself in an overlay
        # container that covers the canvas and declares itself Hoverable with
        # empty methods, so opening the explanation stole the hover that was
        # keeping it open - close, uncover, reopen, at the speed of mouse
        # movement. Reported by somebody moving a mouse, invisible to the
        # guard that opens it and reads it.
        # Through the canvas's own overlay stack rather than widget.PopUp:
        # detail.go stopped importing widget when DetailButton moved onto
        # our own Button (2026-09-15), and a mutant naming a package the file
        # does not import is BROKEN, which proves nothing.
        "the explanation goes back to the overlay layer",
        "internal/gui/parts/detail.go",
        "\tt.sheet.Add(box)\n\tt.sheet.Refresh()\n\treturn box",
        "\tdriver.CanvasForObject(near).Overlays().Add(box)\n"
        "\treturn box",
        "TestTheExplanationNeverUsesTheOverlayLayer",
    ),
    (
        # O217, the half nothing behavioural can see. The sheet gains a box and
        # says nothing, so the box waits for a repaint from anywhere - which
        # the pointer's path supplies and the keyboard's never does. The test
        # driver's canvas ignores Refresh, so this is held by reading the
        # code: a container field that is added to or removed from is
        # refreshed in the same function.
        "the sheet takes a box off and never says so",
        "internal/gui/parts/detail.go",
        "\tt.sheet.Remove(box)\n\tt.sheet.Refresh()",
        "\tt.sheet.Remove(box)",
        "TestABoxThatGainsOrLosesAPieceSaysSo",
    ),
    (
        # The real defect of 2026-09-16, put back (O217). r.button is the
        # Button embedded in DetailButton, and the tree holds the DetailButton,
        # so the driver has no canvas filed under this value: nothing is marked
        # dirty and the explanation put on the sheet is never painted. The
        # test driver answers CanvasForObject for any object, so only the
        # reading of the renderer sees it.
        "a renderer asks the canvas about the widget instead of its pieces",
        "internal/gui/parts/button.go",
        "\tredraw(r.bg, r.ring, r.label, r.icon)\n\tr.Layout(r.button.Size())",
        "\tcanvas.Refresh(r.button)\n\tr.Layout(r.button.Size())",
        "TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget",
    ),
    (
        # And the other silence: a face that changes and tells nobody at all.
        "a renderer changes its face and tells the canvas nothing",
        "internal/gui/parts/progress.go",
        "\tr.Layout(r.bar.Size())\n\tredraw(r.track, r.fill)",
        "\tr.Layout(r.bar.Size())",
        "TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget",
    ),
    (
        # The quieter silence an outside review of #106 pointed at: three
        # pieces named and the fourth left out. The tick is only ever shown or
        # hidden, and Show on a canvas image sets Hidden and stops (fyne
        # v2.8.1 canvas/base.go), so a tick shown and not named stays unpainted
        # until something else repaints. Passed the guard until 2026-09-16,
        # which counted the arguments of redraw and asked nothing of them.
        "a renderer names three of the four pieces it changes",
        "internal/gui/parts/toggle.go",
        "\tredraw(r.halo, r.ring, r.square, r.tick)",
        "\tredraw(r.halo, r.ring, r.square)",
        "TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget",
    ),
    (
        # The same, one method away: the fill's colour is set in applyColours,
        # not in Refresh, so a reader stopping at Refresh reads nothing about
        # it. The guard follows the renderer into its own methods.
        "a renderer colours a piece in a helper and never names it",
        "internal/gui/parts/progress.go",
        "\tr.Layout(r.bar.Size())\n\tredraw(r.track, r.fill)",
        "\tr.Layout(r.bar.Size())\n\tredraw(r.track)",
        "TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget",
    ),
    (
        # The refresh is there and stands BEFORE the change, so it repaints
        # the sheet with the box still on it and the removal is as silent as
        # no refresh at all. Any refresh anywhere in the method satisfied the
        # guard until 2026-09-16 - the second thing the review of #106 named.
        "the sheet says so first and takes the box off after",
        "internal/gui/parts/detail.go",
        "\tt.sheet.Remove(box)\n\tt.sheet.Refresh()",
        "\tt.sheet.Refresh()\n\tt.sheet.Remove(box)",
        "TestABoxThatGainsOrLosesAPieceSaysSo",
    ),
    (
        # A button built hidden opens its explanation to a call and to nobody
        # with a pointer. Caught only since the guard moves the pointer through
        # the canvas (2026-09-16): the call it made before reached a hidden
        # button as readily as a drawn one.
        "the explanation button is built hidden",
        "internal/gui/parts/detail.go",
        "\tb.OnTapped = b.toggle\n\treturn b",
        "\tb.OnTapped = b.toggle\n\tb.Hide()\n\treturn b",
        "TestTheLongerExplanationOpensWhenAsked",
    ),
    (
        # A tooltip that opens and never closes is worse than one that never
        # opens, because it covers the field underneath it.
        "the explanation stays on screen after the pointer leaves",
        "internal/gui/parts/detail.go",
        "func (b *DetailButton) MouseOut() {\n"
        "\tb.Button.MouseOut()\n"
        "\tb.hide()",
        "func (b *DetailButton) MouseOut() {\n"
        "\tb.Button.MouseOut()",
        "TestTheLongerExplanationOpensWhenAsked",
    ),
    (
        # The other half: the button is there and opens nothing. A control that
        # does nothing is the version somebody would ship without noticing,
        # because the screenshot looks identical.
        "the button beside a field opens nothing",
        "internal/gui/parts/detail.go",
        "\tt.sheet.Add(box)\n\tt.sheet.Refresh()\n\treturn box",
        "\treturn box",
        "TestTheLongerExplanationOpensWhenAsked",
    ),
    (
        # The real defect, put back. Capping the column stopped a field at
        # 820 px, which is still a paragraph's width offered for the digit 0.
        # Measured before the fix: the seed was 397 px and the count 399.
        "a box for a number loses its floor, so in the narrowest window it is drawn narrower than a number",
        "internal/gui/parts/parts.go",
        "\treturn Sized(NumericWidth, control)",
        "\treturn control",
        "TestABoxForANumberIsTheWidthOfANumber",
    ),
    (
        # The bar's surface runs the window and what stands on it does not,
        # which is one line of layout and looks like an accident. Without it a
        # refusal about a field 822 px in ran to 1099.
        "what the run says stops lining up with the form",
        "internal/gui/parts/parts.go",
        "\tcolumn := container.New(readableWidth{}, Indented(inner))",
        "\tcolumn := Indented(inner)",
        "TestTheRunSpeaksInsideTheSameColumnAsTheForm",
    ),
    (
        # The real defect, put back. A card fills itself with the page colour -
        # the toolkit's card.go line 44 - so three sections rendered at exactly
        # the page colour, 0.00 L* apart, and the grouping they were introduced
        # for did not exist. Every structural guard stayed green through it.
        "a section goes back to a card and stops drawing a surface",
        "internal/gui/parts/parts.go",
        # Repointed 2026-09-23, when a section's content became a Grid.
        "\treturn container.NewStack(panelSurface(), Padded(Inset, Column(GapField, sectionTitle(title), Grid(content...))))",
        '\treturn widget.NewCard(title, "", container.NewVBox(content...))',
        "TestASectionDrawsItsOwnSurface",
    ),
    (
        # The same defect reached the other way: the surface is still drawn, and
        # painted with the colour of the thing behind it. That is what a card
        # was doing, and it is the version a palette edit could reintroduce
        # without touching a line of layout.
        "the panel is painted with the page colour",
        "internal/gui/parts/parts.go",
        "rect := canvas.NewRectangle(PaletteColour(ColorNamePanel, theme.VariantDark))",
        "rect := canvas.NewRectangle(PaletteColour(theme.ColorNameBackground, theme.VariantDark))",
        "TestASectionDrawsItsOwnSurface",
    ),
    (
        # The mistake that was nearly made on the way in. docs/UX.md section 8.2
        # records a surface at #2E2E30 and reaching for it is the obvious move -
        # but that value is what an input box is painted with, so the panel
        # would have swallowed every field standing on it. Grouped screen,
        # invisible controls.
        "the panel takes the colour an input box already uses",
        "internal/gui/parts/theme.go",
        "ColorNamePanel:           hex(0x2B, 0x2B, 0x2F),",
        "ColorNamePanel:           hex(0x39, 0x39, 0x3F),",
        "TestEachSurfaceIsToldFromTheOneUnderIt",
    ),
    (
        # The real defect, put back. The set named its files after the DISTANCE
        # from the limit and never after the limit, so "at_limit.pdf" from a
        # 10mb run and from a 5mb run are one name for two different claims.
        # Reported from use on 2026-08-11. The full suite stayed green when the
        # names changed, which is why the guard had to be written.
        "the limit disappears from the names of a boundary set",
        "internal/preset/limitset.go",
        "\t\t\tName:     s.limitText + \"_\" + one.id + s.desc.Extension,\n",
        "\t\t\tName:     one.id + s.desc.Extension,\n",
        "TestEveryBoundaryFileNamesTheLimitItWasBuiltAround",
    ),
    (
        # A button that asks and drops the answer looks exactly like one that
        # works. The picker itself needs a real window, so what is provable is
        # the wiring - and the wiring is the part that goes wrong.
        "the chosen directory never reaches the field",
        "internal/gui/window/open.go",
        "				box.SetText(path)",
        "				_ = path",
        "TestBrowsingForADirectoryPutsItInTheField",
    ),
    (
        # Cancelling a picker answers with nothing, and a field emptied by
        # cancelling sends the run at a directory nobody named.
        "cancelling the picker empties the directory field",
        "internal/gui/window/open.go",
        "			if path != \"\" {",
        "			if path == path {",
        "TestCancellingTheDirectoryPickerLeavesTheFieldAlone",
    ),
    (
        # The notices used to ask one direction only: is every listed module
        # still built. So a module that ARRIVED went unlisted and unnoticed,
        # which is the direction that matters - their licences require the
        # notice to travel with the code that ships. Found on 2026-08-05 when
        # the folder picker brought one in.
        "a module ships in the window with its notice removed",
        "THIRD-PARTY-NOTICES.md",
        "| `github.com/FyshOS/fancyfs` | v0.0.1 | BSD-3-Clause | (C) 2025 FyshOS developers (see AUTHORS) |\n",
        "",
        "TestTheNoticesFileNamesTheVersionsThatAreActuallyBuilt",
    ),
    (
        # The ceiling counts code and not explanation, and until 2026-08-05
        # nothing proved the exclusion still worked. It would not have shown up
        # as a wrong number - it would have shown up as a function suddenly over
        # the ceiling for having been explained, which reads like a real
        # finding. Breaks the measure rather than the guard reading it.
        "the size ceiling starts counting comments as code",
        "internal/guard/codeshape_test.go",
        "\t\tif comments[line] {\n\t\t\tcontinue\n\t\t}",
        "\t\tif comments[line] && false {\n\t\t\tcontinue\n\t\t}",
        "TestTheSizeCeilingCountsCodeAndNotExplanation",
    ),
    (
        # The classification is the subject here, so the classification is
        # what gets broken. Until 2026-08-05 the UTF-8 check walked a list
        # written by hand and nothing asked whether the list still covered
        # the registry - so a format added later would simply not have been
        # checked, with the suite green and covering less.
        "a registered format is on neither the text nor the binary list",
        "internal/guard/textformats_test.go",
        "\"webm\", \"webp\", \"xlsx\", \"zip\"}",
        "\"webm\", \"webp\", \"xlsx\"}",
        "TestEveryFormatIsClassifiedAsTextOrBinary",
    ),
    (
        # Found by coverage rather than by reading: onCancel came back at
        # nought per cent while every guard around it was green. The button was
        # on the screen and in the brief, and nothing pressed it.
        "the Cancel button stops reaching the run",
        "internal/gui/window/run.go",
        "func (r *runner) Stop() {\n\tif r.stop != nil {",
        "func (r *runner) Stop() {\n\tif r.stop != nil && false {",
        "TestCancelStopsTheRun",
    ),
    (
        # O63. Fyne 2.8 warns at every start that changes have to go through
        # fyne.Do and says the next major release stops tolerating it. Measured
        # on 2026-08-05: under the test driver fyne.Do runs on the calling
        # goroutine and a widget touched from a worker does not complain, so
        # neither -race nor a rendering test can see this. Static reading is
        # what is left.
        "the window is told the run finished straight from the worker",
        "internal/gui/window/run.go",
        "\t\tfyne.Do(func() { r.runFinished(res, runErr, saveErr, room, saved) })",
        "\t\tr.runFinished(res, runErr, saveErr, room, saved)",
        "TestTheWindowIsOnlyTouchedFromTheInterfaceThread",
    ),
    (
        # The owner decided on 2026-08-05 that the window opens on the work.
        # A notice shown at every start is one nobody reads twice, and what the
        # rule actually asks is that the licence can be reached.
        # Which tab is first is what "opens on the work" means since the screens
        # became tabs on 2026-08-11. Putting the notice first is the same defect
        # the owner decided against on 2026-08-05, in its new shape.
        "the window goes back to opening on the licence notice",
        "internal/gui/window/open.go",
        "&parts.Tab{ID: tabGenerate, Text: text.TabOneTarget(), Content: gen.Object()},",
        "&parts.Tab{ID: tabAbout, Text: text.TabAbout(), Content: About(h)},",
        "TestTheWindowOpensOnTheGenerateScreen",
    ),
    (
        # The other half of the same decision. Moving the notice off the opening
        # screen is only safe while it can still be got to, and since it became
        # a tab on 2026-08-11 the way to lose it is to drop the tab - which also
        # takes away the way back, because a tab was both.
        "the licence tab is removed, so the notice becomes unreachable",
        "internal/gui/window/open.go",
        "&parts.Tab{ID: tabAbout, Text: text.TabAbout(), Content: About(h)},",
        "&parts.Tab{ID: tabAbout, Text: text.TabAbout() + \"!\", Content: About(h)},",
        "TestTheLicenceIsStillReachableFromTheOpeningScreen",
    ),
    (
        # A version typed once beside the code rather than derived from it.
        # Three of them went stale within hours of being written, when raising
        # x/image to clear four vulnerabilities moved it, x/net and x/sys.
        # Their licences ask the notice to travel with the code that ships, so
        # the number has to come from the build.
        # Breaks the subject rather than the judge: the number in the file goes
        # back to what it was before x/image was raised. Disabling the guard's
        # own comparison would leave it green, since the versions agree today,
        # and would prove nothing about whether drift is caught.
        "the notices file goes back to naming a version that is no longer built",
        "THIRD-PARTY-NOTICES.md",
        "| `golang.org/x/net` | v0.60.0 |",
        "| `golang.org/x/net` | v0.35.0 |",
        "TestTheNoticesFileNamesTheVersionsThatAreActuallyBuilt",
    ),
    (
        # The real defect, put back, and it is the one the guard above told a
        # person to fix by hand: "change the counts in the paragraph". Nobody
        # did. The paragraph said twelve BSD 3-Clause and nine MIT while the
        # table held thirteen and ten, and two internal documents copied the
        # wrong pair onwards. This file is the one artefact here that exists to
        # satisfy somebody else's licence, so a reader who counts the rows and
        # finds the sentence disagreeing cannot tell which half is wrong.
        "the notices count a licence differently from the table beneath them",
        "THIRD-PARTY-NOTICES.md",
        "thirteen BSD 3-Clause, nine MIT",
        "twelve BSD 3-Clause, eight MIT",
        "TestTheNoticesCountsAgreeWithTheTableTheyDescribe",
    ),
    (
        # Found by fuzzing 2026-08-05. The text of a distance becomes the id of
        # a target and the name of a file, and a carriage return reaches the
        # recipe source raw - the size parser trims the ends and that one sits
        # in the middle. The document then does not parse at all.
        "a distance carrying a character a name cannot is let through",
        "internal/preset/sizeboundaries.go",
        'if bad := firstUnusable(piece); bad != "" {',
        'if bad := firstUnusable(piece); bad != "" && false {',
        "TestEveryPresetEndingHasTheCodeItShould",
    ),
    (
        # The engine writes one sentence and both surfaces show it word for
        # word, so a spelling only one of them takes sends the other's reader
        # translating. Seen on a screenshot of the refused preset screen on
        # 2026-08-11: it said "Raise --limit", and the window labels that field
        # "limit" and has no flags on it at all. O79.
        "the refusal goes back to naming a command line flag",
        "internal/preset/limitset.go",
        '"Raise the {setting} above %d B, narrow the spread,',
        '"Raise --limit above %d B, narrow the spread,',
        "TestSizeBoundariesRefusesASetItCannotCompleteRatherThanPartOfIt",
    ),
    (
        # Two equal distances make two steps of the set that are the same file
        # twice, and they collide on the id built from the distance. Compared as
        # bytes, so 1024 and 1kb are caught as well as 1B and 1b.
        "the same distance twice builds a set holding that step twice",
        "internal/preset/build.go",
        "if first, repeated := seen[key]; repeated {",
        "if first, repeated := seen[key]; repeated && false {",
        "TestEveryPresetEndingHasTheCodeItShould",
    ),
    (
        # A bad spread used to be a plain error, which falls through the
        # classifier to RUNTIME - so a mistyped flag value told CI this program
        # had a bug. The same class as "--set width=abc", fixed two days before
        # for the same reason.
        "a mistyped distance is reported as a fault of the tool again",
        "internal/preset/build.go",
        "\treturn &format.PropertyValueError{\n\t\tFormat: l.preset, Key: l.param, Value: value, Reason: reason,\n\t}\n",
        "\treturn fmt.Errorf(\"%s: %s cannot be %q - %s\", l.preset, l.param, value, reason)\n",
        "TestEveryPresetEndingHasTheCodeItShould",
    ),
    (
        # The state before 2026-08-12, put back. A refused value was answered
        # with a paragraph and the box it was typed into looked like every
        # other one, so on a form of eight settings the reader finds the
        # sentence before they can find the field.
        "a refused box goes back to looking like every other box",
        "internal/gui/parts/field.go",
        "\ta.box.Show()\n\ta.mark(true)",
        "\ta.box.Show()",
        "TestTheBoxARefusalIsAboutIsMarked",
    ),
    (
        # The half nothing on the screen looks wrong for: a mark left behind
        # after the value was fixed describes a state that is no longer true,
        # and the next press is refused by a screen already showing a refusal.
        "the mark stays on a box whose value was fixed",
        "internal/gui/parts/field.go",
        "\ta.mark(false)\n}",
        "\ta.mark(true)\n}",
        "TestTheMarkGoesWhenTheValueIsFixed",
    ),
    (
        # The wiring between the sentence and the edge, cut. The area still
        # says what happened and the box says nothing, which is the shape this
        # started as and looks correct at every call site.
        "the refusal is no longer wired to the box it is about",
        "internal/gui/parts/field.go",
        "\tarea := NewErrorArea()\n\tarea.edge = ring\n\tpieces := ",
        "\tarea := NewErrorArea()\n\t_ = ring\n\tpieces := ",
        "TestTheBoxARefusalIsAboutIsMarkedOnThePresetScreenToo",
    ),
    (
        # The keyboard arrives and nothing says so. The toolkit's own mark for
        # a menu is a fill nobody can read a value through, which is why this
        # line exists at all - so losing it leaves focus invisible rather than
        # merely quieter.
        "a menu stops saying when the keyboard is in it",
        "internal/gui/parts/ring.go",
        "c.ring.Focus(true)",
        "c.ring.Focus(false)",
        "TestTheMenuTheKeyboardIsInDrawsALine",
    ),
    (
        # The order between the two states, swapped. A refused box with the
        # keyboard in it draws the focus colour, so the one mark that says the
        # run will not start is the one that disappears when somebody goes to
        # fix it.
        "focus outranks a refusal, so a refused box loses its mark when used",
        "internal/gui/parts/ring.go",
        "\tcase r.refused:",
        "\tcase r.focused:",
        "TestARefusedBoxStaysRedWhileTheKeyboardIsInIt",
    ),
    (
        # Registration stops putting a closed set in order, so the order is
        # whatever the declaration was typed in - which is where the pdf paper
        # sizes were a4, a3, a5, letter, legal.
        "a closed set is registered in whatever order it was written",
        "internal/format/registry.go",
        "\t\tSortChoices(d.Properties[i].Choices)",
        "\t\t_ = i",
        "TestEveryClosedSetIsRegisteredInOrder",
    ),
    (
        # Numbers sorted as words. The WAV bit depths come out 16, 24, 32, 8,
        # which is alphabetical and is the reason this is not a plain sort.
        "numbers in a closed set are sorted as words",
        "internal/format/registry.go",
        "if numeric(choices) {",
        "if numeric(choices) && false {",
        "TestAClosedSetOfNumbersIsInNumericOrder",
    ),
    (
        # The registry hands its ids back in map order, which is deliberately
        # random in Go. Both menus are built straight from this.
        "the list of formats comes back in whatever order the map likes",
        "internal/format/registry.go",
        "\tsort.Strings(out)\n\treturn out",
        "\treturn out",
        "TestTheFormatAndPresetMenusAreInOrder",
    ),
    (
        # The state as it shipped: the preset screen draws the parameters a
        # preset declares and not the global flags it supplies a value for, so
        # size-boundaries can only ever be built as PDFs from the window.
        "the preset screen stops offering the global settings a preset reads",
        "internal/gui/window/preset.go",
        "settings = append(settings, chosen.Globals()...)",
        "_ = chosen.Globals()",
        "TestThePresetScreenCanBuildTheSetInAnyFormat",
    ),
    (
        # Everything typed on the preset screen is dropped on the way to the
        # expansion, so the window runs the preset at its declared defaults
        # while the command line runs what was asked for. The drift D1 is about,
        # in the one place the two surfaces cannot share code.
        "what was typed on the preset screen never reaches the expansion",
        "internal/gui/window/preset.go",
        "p.last.of(p.host, p.pick.Selected, p.given())",
        "p.last.of(p.host, p.pick.Selected, nil)",
        "TestChoosingTheFormatGivesTheSameSetOnBothSurfaces",
    ),
    (
        # The preview goes back to saying how many and how big without saying
        # what. On the preset screen that is the one fact nothing else on the
        # screen carries, because a preset states its own targets.
        "the preview stops saying what kind of files it would write",
        "internal/gui/window/runsummary.go",
        "		formats:   formatsOf(planned),",
        "		formats:   nil,",
        "TestThePreviewSaysWhatKindOfFilesItWouldWrite",
    ),
    (
        # A box built outside the registry, which is the state the whole screen
        # was in for the format menu, the seed and every setting a format
        # declares. It looks completely ordinary at the call site - that is why
        # the answer had to become "there is no other way to build one".
        "a field is built without registering it, so nothing can mark it",
        "internal/gui/window/generate.go",
        "add(engine.SettingSeed, text.FieldSeed(),",
        'add("", text.FieldSeed(),',
        "TestEveryBoxOnAScreenIsOneThatCanBeMarked",
    ),
    (
        # The registry knows every field and loses the message on the way. From
        # outside this is indistinguishable from the wiring being right, which
        # is the shape a window ends up in when somebody fixes the lookup and
        # not the delivery.
        "a marked field is found and never told",
        "internal/gui/parts/fields.go",
        # Repointed 2026-09-23, when a refusal started offering its own fix.
        "\tf.Say(inTheWordsOnScreen(f, err))\n\tif label, apply := fixFor(f, err)",
        "\t_ = f\n\tif label, apply := fixFor(f, err)",
        "TestEveryRegisteredFieldCanBeMarkedAndUnmarked",
    ),
    (
        # The fields a format declares are drawn and not registered, so the
        # boxes that exist for exactly as long as one format is chosen are the
        # ones a refusal cannot reach. Fourteen of them across thirteen formats.
        "the settings a format declares are drawn outside the registry",
        "internal/gui/parts/property.go",
        "field := into.Add(p.Name, text.SettingLabel(p.Name), PropertyDetail(owner, p),",
        'field := into.Add("", text.SettingLabel(p.Name), PropertyDetail(owner, p),',
        "TestEveryFormatsOwnSettingsCanBeMarked",
    ),
    (
        # The colour the open list had until 2026-08-12: the input colour to the
        # byte, over a panel 3.8 L* away, with no border. Reported from a
        # screenshot as a list where everything runs together.
        "the menu's surface drops to the colour of an input box, so the rung of the ladder above a button's face is missing",
        "internal/gui/parts/theme.go",
        "theme.ColorNameMenuBackground: hex(0x57, 0x57, 0x5F),",
        "theme.ColorNameMenuBackground: hex(0x39, 0x39, 0x3F),",
        "TestTheLadderClimbsInEvenSteps",
    ),
    (
        # The toolkit's own behaviour, put back: a list of thirteen formats
        # opens with nothing saying which one is in the box, and the box is
        # behind the list.
        "the open list stops marking the value that is chosen",
        "internal/gui/parts/listcontents.go",
        "func (c *listContents) isChosen(value string) bool { return value == c.chosen }",
        "func (c *listContents) isChosen(value string) bool { return false }",
        "TestTheOpenListMarksTheValueThatIsChosen",
    ),
    (
        # The state this guard was in until 2026-08-13: it asked the compiler
        # about the system it happened to be running on. A module built on Linux
        # and nowhere else was therefore invisible on Windows, which is exactly
        # what happened to github.com/rymdport/portal for a week - listed
        # nowhere, shipped in the Linux window, and the guard green.
        "the notices are compared against one system instead of every one",
        "internal/guard/notices_test.go",
        'for _, goos := range []string{"windows", "linux", "darwin"} {',
        'for _, goos := range []string{"windows"} {',
        "TestTheNoticesFileNamesTheVersionsThatAreActuallyBuilt",
    ),
    (
        # A word a person reads, built outside the one package that holds them.
        # The title of the window itself was exactly this until 2026-08-13 and
        # the guard could not see it, because it worked from a list of the calls
        # that show text and nobody had put the toolkit's NewWindow on that
        # list. This puts a literal somewhere no such list would ever name.
        "a sentence for a person is built outside the text package again",
        "internal/gui/window/run.go",
        "text.NotAWholeNumber(field, value)",
        '"that is not a whole number"',
        "TestTheWindowSaysNothingItDoesNotSayFromTheTextPackage",
    ),
    (
        # The real defect, put back: a counted noun with brackets round the
        # plural. Fourteen lines across seven files wrote this until
        # 2026-08-13, while the window had stopped a day earlier - so one run
        # was described two ways and the owner read the worse one off a screen.
        # Chosen in an archive rather than in the command line because this
        # guard is about the whole tree: a mutation inside internal/cli would
        # pass on a guard that only watched internal/cli, and that was the
        # narrower guard worth refusing.
        "a counted noun goes back to dodging the plural",
        "internal/format/zip/zip.go",
        "core.SaysN(\"format.GroupOf\", \"%d %s file of %d B\", \"%d %s files of %d B\",",
        "core.SaysN(\"format.GroupOf\", \"%d %s file(s) of %d B\", \"%d %s file(s) of %d B\",",
        "TestNoMessageDodgesThePluralWithBrackets",
    ),
    (
        # The real defect, put back: the picture is drawn, committed, embedded
        # and linked, and nobody gives it to the toolkit - so the desktop shows
        # the toolkit's own logo and every check that reads the file on disk is
        # green. The bytes stay referenced here, so this compiles and the
        # linking half of the guard still passes. Only the half that asks
        # whether anybody handed it over can see it.
        "the icon is shipped and never given to the toolkit",
        "internal/gui/run_cgo.go",
        'a.SetIcon(fyne.NewStaticResource("chickpea.png", icon.PNG))',
        "_ = icon.PNG",
        "TestTheWindowShipsTheIconWeDrew",
    ),
    (
        # The real defect, put back in the shape it would really arrive in: the
        # row of buttons goes into a container that lays nothing out, so every
        # button keeps its text, keeps Visible, keeps its handler - and gets no
        # size. Every test in this package that presses by calling OnTapped
        # stays green through that, because a handler does not care how big
        # anything is. A press through the canvas has nothing to land on.
        "the action bar stops laying its buttons out",
        # Repointed 2026-09-23, when the row became parts.ButtonRow: its layout
        # stops placing anything.
        "internal/gui/parts/buttonrow.go",
        "\t\to.Resize(min)\n\t\to.Move(fyne.NewPos(x, (size.Height-min.Height)/2))\n",
        "\t\t_ = x\n",
        "TestEveryButtonAPersonCanSeeIsReallyPressable",
    ),
    (
        # The pointer takes the keyboard and says so again, which is the state
        # reported from the screen twice - a menu left painted blue after a
        # value was chosen, with nothing left to press that would take it off.
        "a press with the mouse draws the keyboard mark again",
        "internal/gui/parts/ring.go",
        "\tif !c.from.Draws() {\n\t\treturn\n\t}\n\tc.mark()",
        "\tc.mark()",
        "TestAPressMovesTheKeyboardWithoutDrawingItsMark",
    ),
    (
        # The other direction: nothing ever draws the mark, which looks tidy on
        # every screenshot and breaks UX9 - where the keyboard is has to be
        # visible or the rest of that rule buys nothing.
        "the keyboard arrives and nothing on the control says so",
        "internal/gui/parts/ring.go",
        "func (c *Chooser) mark() {\n\tc.marked = true",
        "func (c *Chooser) mark() {\n\tif true {\n\t\treturn\n\t}\n\tc.marked = true",
        "TestTheKeyboardDrawsItsMarkWhenTheKeyboardIsWhatArrived",
    ),
    (
        # Somebody opens a list with the mouse, then drives it with the arrows,
        # and from that moment they need to see which control is listening.
        "reaching for the keyboard no longer turns the mark on",
        "internal/gui/parts/ring.go",
        "\tif !c.marked {\n\t\tc.mark()\n\t}\n\tswitch event.Name {",
        "\tswitch event.Name {",
        "TestReachingForTheKeyboardTurnsTheMarkOn",
    ),
    (
        # The switch built from the toolkit's own type keeps the blue disc on
        # every press. This is the second place switches come from and the one
        # nobody would remember, because no format declares a bool setting yet.
        #
        # The menu a declared choice produces put back as the toolkit's own,
        # which fills itself in the focus colour when pressed and never takes it
        # off. This is O131: property.go does not import widget, so the way to
        # name the toolkit's control without a second change is the Select our
        # Chooser embeds - &NewChooser(...).Select is a *widget.Select without
        # naming its package. The guard that asks whether every menu is ours
        # reads the type FromProperty returns and goes red.
        #
        # Since 2026-09-15 the switch this used to swap is our own with no Check
        # to embed, so the handle moved from the bool field to the choice field
        # - the guard proves both, by asking FromProperty for each kind.
        "a declared menu is the toolkit's own again",
        "internal/gui/parts/property.go",
        "Control: sel,\n\t\tValue:   func() string { return sel.Selected },",
        "Control: &sel.Select,\n\t\tValue:   func() string { return sel.Selected },",
        "TestEverySwitchAndMenuOnScreenKnowsWhoFocusedIt",
    ),
    (
        # Back to one refusal per press however many boxes are wrong, which is
        # what was reported: settle giving up at the first bad value.
        "the form gives up at the first bad box again",
        "internal/gui/window/generate.go",
        "\t\tbad = append(bad, saying(format.SettingSize, err))",
        "\t\treturn nil, none, saying(format.SettingSize, err)",
        "TestARefusalAboutTwoBoxesMarksBothOfThem",
    ),
    (
        # A box says nothing about what is in it until a button is pressed,
        # which is the state this window was in until 2026-08-18.
        "typing no longer checks what was typed",
        "internal/gui/window/run.go",
        "\tr.fields.WhenTypedIn(r.recheck)",
        "\t_ = r.recheck",
        "TestABadValueMarksItsBoxWithNothingPressed",
    ),
    (
        # Every refusal on the screen wiped at each keystroke, so a size the
        # FORMAT refused goes unmarked the moment anything else is typed - and
        # it is still refused. Found by the stored tree rather than by reading.
        "a keystroke wipes what every other box was told",
        "internal/gui/window/runrefuse.go",
        "\tr.fields.Clear(setting)\n\tif r.fields.Blank(setting) {",
        "\tr.fields.ClearAll()\n\tif r.fields.Blank(setting) {",
        "TestTypingInOneBoxLeavesAnotherBoxesRefusalAlone",
    ),
    (
        # A box emptied to retype it turns red immediately, which is the
        # commonest thing anybody does in a form.
        "an empty box is called wrong while it is being retyped",
        "internal/gui/window/runrefuse.go",
        "\tif r.fields.Blank(setting) {\n\t\treturn\n\t}",
        "\tif false {\n\t\treturn\n\t}",
        "TestAnEmptyBoxIsNotCalledWrongUntilSomethingIsPressed",
    ),
    (
        # Repointed on 2026-09-08, in the evening. The version above belonged
        # to gui/new-look, where the buttons sat at the right hand end - the
        # branch is gone and so is that guard. On main they are centred, on
        # the owner's decision of 2026-08-19, and centring takes a spacer at
        # BOTH ends: one greedy spacer can only push the group to an end.
        # Taking one away compiles, keeps the layout import used, and moves
        # the pair off centre where nothing but the guard can see it.
        "the run buttons are laid from the left edge rather than centred",
        # Repointed 2026-09-23: the two spacers became parts.ButtonRow, which
        # centres by working out where the row starts. Repointed 2026-09-24:
        # the row gives way to the right of centre when the rail needs it.
        "internal/gui/parts/buttonrow.go",
        "\tx := fyne.Max((size.Width-b.MinSize(objects).Width)/2, b.clearLeft)\n",
        "\tx := fyne.Max(float32(0)*size.Width, b.clearLeft)\n",
        "TestTheButtonsThatRunSomethingSitInTheMiddle",
    ),
    (
        # The state the toolkit ships: widget.Select.TypedRune is left blank on
        # purpose, so a letter at a closed list did nothing at all.
        "a letter typed at a shut list does nothing again",
        "internal/gui/parts/ring.go",
        "if strings.HasPrefix(strings.ToLower(c.shown(c.Options[at])), want) {\n\t\t\tc.SetSelectedIndex(at)",
        "if strings.HasPrefix(strings.ToLower(c.shown(c.Options[at])), want) {\n\t\t\t_ = at",
        "TestALetterTypedAtTheShutListMovesToThatValue",
    ),
    (
        # The same letter twice stops walking, so the second of two values
        # sharing a letter cannot be reached at all.
        "the letter jump sticks on the first value that matches",
        "internal/gui/parts/ring.go",
        "\tfrom := c.SelectedIndex()",
        "\tfrom := -1",
        "TestALetterTypedAtTheShutListMovesToThatValue",
    ),
    (
        # PopUpMenu.TypedRune does nothing, which is where this started.
        "a letter typed at the open list does nothing again",
        "internal/gui/parts/openlist.go",
        "\t\tif e := l.entries[at]; e.kind == entryValue && strings.HasPrefix(strings.ToLower(e.text), want) {\n\t\t\tl.moveTo(at)",
        "\t\tif e := l.entries[at]; e.kind == entryValue && strings.HasPrefix(strings.ToLower(e.text), want) {\n\t\t\t_ = at",
        "TestALetterTypedAtTheOpenListMovesTheKeyboard",
    ),
    (
        # A press paints a bar across the row the arrows start from, which says
        # the keyboard is here to somebody who has just used a mouse.
        "opening with a press paints the keyboard's row again",
        "internal/gui/parts/openlist.go",
        "\t\t\tl.moveTo(i)\n\t\t\tl.shown = false",
        "\t\t\tl.moveTo(i)\n\t\t\tl.shown = true",
        "TestOpeningAListWithAPressDrawsNoKeyboardBar",
    ),
    (
        # The ceiling goes and the list is as tall as it likes - 476 px at
        # thirteen formats, 672 at the twenty-four there are, which is more
        # than half of the reference window and all of a 650 px one.
        "the open list loses its ceiling and covers the form",
        "internal/gui/parts/ring.go",
        "\tif rows > ceiling {\n",
        "\tif rows > ceiling && false {\n",
        "TestTheOpenListCoversHalfTheWindowAndGrowsWithIt",
    ),
    (
        # The ceiling stops following the window and is worked out for one
        # window of 800 px whatever the window is - fourteen rows everywhere,
        # which is the count of eight back under another name. Caught by the
        # half that asks two canvases, and by the taller one leaving more than
        # a row of its half unused.
        "the open list's ceiling is the same in every window again",
        "internal/gui/parts/ring.go",
        "ceiling := wholeRows(canvasHeight*listShare - head)",
        "ceiling := wholeRows(800*listShare - head)",
        "TestTheOpenListCoversHalfTheWindowAndGrowsWithIt",
    ),
    (
        # Escape closes the list and leaves the keyboard on a control that is no
        # longer on the screen, after which Tab starts from nowhere visible.
        "Escape closes the list without giving the keyboard back",
        "internal/gui/parts/ring.go",
        "\tif byKeyboard {\n\t\tsurface.Focus(c)",
        "\tif byKeyboard {\n\t\tsurface.Focus(nil)",
        "TestEscapeClosesTheListAndGivesTheKeyboardBack",
    ),
    (
        # The room between rows comes back, which is the whole reason this
        # control exists: widget.List spaced rows by the form's own padding.
        # Since 2026-09-23 the rows are ours (rowStack), laid out one row apart.
        "the rows are spaced by the form's padding again",
        "internal/gui/parts/rowview.go",
        "\t\to.Move(fyne.NewPos(0, float32(at)*row))",
        "\t\to.Move(fyne.NewPos(0, float32(at)*(row+rowGap)))",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        # The rows go back under a theme override - a new scope and a fresh
        # set of fonts at every opening, 158 MB for ten (GUI-MEMORY 4e).
        "the open list draws its rows under a theme override again",
        "internal/gui/parts/openlist.go",
        "\trows := l.view.scroll\n",
        "\trows := container.NewThemeOverride(l.view.scroll, Theme())\n",
        "TestNoScreenStandsInAThemeOverride",
    ),
    (
        # Arrowing down past the room leaves the keyboard on a row nobody can
        # see. No stored screen scrolls the list, so no picture would notice.
        "the keyboard walks below the open list out of sight",
        "internal/gui/parts/rowview.go",
        "\t\tv.scroll.Offset.Y = top + row - v.scroll.Size().Height",
        "\t\tv.scroll.Offset.Y += 0",
        "TestTheRowTheKeyboardIsOnIsAlwaysInSight",
    ),
    (
        # An emptied filter in a list whose box holds no value finds nothing
        # to start on, and the rows of what it had narrowed to stay drawn.
        "an emptied filter leaves the narrowed rows drawn when nothing is chosen",
        "internal/gui/parts/openlist.go",
        "\t// narrowed to still drawn.\n\tl.view.show(len(l.entries), l.fill)",
        "\t// narrowed to still drawn.\n\t_ = l.view",
        "TestEmptyingTheFilterOfAListWithNothingChosenDrawsEveryRowAgain",
    ),
    (
        # Arrowing up to the first value of a kind stops showing the heading
        # that says which kind it was.
        "the first value of a kind comes into sight without its heading",
        "internal/gui/parts/openlist.go",
        "\t\tl.view.bringIntoView(at - 1)\n",
        "\t\t_ = at\n",
        "TestTheRowTheKeyboardIsOnIsAlwaysInSight",
    ),
    (
        # A spelling that can mean two things stops meaning the one this tool
        # means everywhere else, which is the whole of the rule the owner set
        # on 2026-08-18.
        "a boundary spelled in units stops counting in 1024s",
        "internal/core/size.go",
        "func ParseBoundary(s string) (int64, error) {\n\treturn ParseSize(s)",
        "func ParseBoundary(s string) (int64, error) {\n\treturn ParseSize(s + \"x\")",
        "TestABoundarySpelledInUnitsIsTakenAsTheOneWeUseEverywhere",
    ),
    (
        # The run stops saying which number it built around. This is what makes
        # accepting an ambiguous spelling safe: the rare reader whose system
        # meant the other number sees it in a dry run, before a byte is written.
        "a boundary run stops saying the number it built around",
        "internal/cli/generate.go",
        "\t\tfmt.Fprintf(errOut, \"boundary %q around %s:\\n\", t.ID, core.ExactBytes(t.BoundaryLimit))",
        "\t\tfmt.Fprintf(errOut, \"boundary %q:\\n\", t.ID)",
        "TestABoundaryRunSaysTheNumberItBuiltAround",
    ),
    (
        # The recipe key and the flag stop reading a spelling the same way,
        # which is a recipe and a command line disagreeing about one word.
        "the recipe boundary key reads units its own way",
        "internal/recipe/target.go",
        "\tlimit, err := core.ParseBoundary(text)",
        "\tlimit, err := core.ParseBoundary(text + \"b\")",
        "TestTheRecipeBoundaryKeyReadsUnitsLikeTheFlag",
    ),
    (
        # Nudged up until the rounding it absorbs is bigger than the smallest
        # real change this window can make, which is the quiet way a tolerance
        # turns a comparison into a formality.
        "the arm64 blend tolerance grows until it can hide a change somebody meant to see",
        "internal/guard/screenpixels_test.go",
        "const armBlendTolerance = 6",
        "const armBlendTolerance = 12",
        "TestTheBlendToleranceStaysBetweenTheRoundingAndTheSmallestRealChange",
    ),
    (
        # And the other way, which is the half a one sided check would miss: a
        # tolerance under the measured rounding makes arm64 red on every run for
        # a difference nobody can see.
        "the arm64 blend tolerance drops below the rounding it was measured to absorb",
        "internal/guard/screenpixels_test.go",
        "const armBlendTolerance = 6",
        "const armBlendTolerance = 2",
        "TestTheBlendToleranceStaysBetweenTheRoundingAndTheSmallestRealChange",
    ),
    (
        # The tolerance moves to amd64, which was measured at exactly zero - so
        # it would give away accuracy this guard actually has, on the
        # architecture that never needed it.
        "the blend tolerance lands on the architecture that was measured not to need it",
        "internal/guard/screenpixels_test.go",
        "\tif runtime.GOARCH == \"arm64\" {\n\t\treturn armBlendTolerance\n\t}",
        "\tif runtime.GOARCH != \"arm64\" {\n\t\treturn armBlendTolerance\n\t}",
        "TestTheBlendToleranceStaysBetweenTheRoundingAndTheSmallestRealChange",
    ),
    (
        # Back to one line of evidence, which is how O115 stayed open for three
        # days: the runner has no file to diff, so the first differing line was
        # the whole of what anybody could see.
        "a failing screen names only the first line that moved",
        "internal/guard/screenpixels_test.go",
        "\t\tif total <= cap {",
        "\t\tif total <= 1 {",
        "TestAFailingScreenNamesEveryLineThatMovedNotJustTheFirst",
    ),
    (
        # The report stops at its cap and says nothing about stopping, so a
        # reader counts the lines it shows and believes that is all of them.
        "the capped diff stops without saying anything was left out",
        "internal/guard/screenpixels_test.go",
        "\tif total > cap {",
        "\tif false && total > cap {",
        "TestAFailingScreenNamesEveryLineThatMovedNotJustTheFirst",
    ),
    (
        # Skipped on every system again, which is the state this was in until
        # 2026-08-23 - a bramka that runs nowhere and reports green.
        "the screens go back to being skipped on the whole of CI",
        "internal/guard/screenpixels_test.go",
        "\treturn goos != \"darwin\"",
        "\treturn false",
        "TestTheScreensAreComparedOnCIEverywhereButMacOS",
    ),
    (
        # Compared on macOS as well, where Fyne models the system setting for
        # showing scroll bars and nine screens legitimately differ - so the
        # comparison would be red for a reason that is not a defect.
        "the screens are compared on macOS, where the system hides scroll bars",
        "internal/guard/screenpixels_test.go",
        "\treturn goos != \"darwin\"",
        "\treturn true",
        "TestTheScreensAreComparedOnCIEverywhereButMacOS",
    ),
    (
        # Back to asking the stop handle whether a run is going. The handle is
        # assigned once and never returns to nil, so the live check died after
        # the first Generate and a bad value said nothing until a button was
        # pressed.
        "the live check asks the stop handle whether a run is going",
        "internal/gui/window/runrefuse.go",
        "\tif r.busy.occupied {\n\t\treturn\n\t}",
        "\tif r.stop != nil {\n\t\treturn\n\t}",
        "TestTypingIsStillCheckedAfterARunHasFinished",
    ),
    (
        # The disk work goes back onto the interface thread, which is a window
        # that stops drawing for as long as somebody else's disk takes - no bar,
        # no way out, and both buttons still looking pressable.
        "a preview reads the disk on the interface thread again",
        "internal/gui/window/run.go",
        "\tgo func() {\n\t\tplanned, planErr := engine.PlanContext(ctx, targets, opt)\n\t\tif planErr != nil {\n\t\t\t// Do rather",
        "\tfunc() {\n\t\tplanned, planErr := engine.PlanContext(ctx, targets, opt)\n\t\tif planErr != nil {\n\t\t\t// Do rather",
        "TestAPreviewDoesItsDiskWorkOffTheInterfaceThread",
    ),
    (
        # Nothing says the screen is occupied, so the form stays editable and
        # both buttons stay pressable while a preview is reading the disk.
        "a preview never says the screen is busy",
        "internal/gui/window/run.go",
        "\tr.busy.set(true, busyFace{stoppable: true})\n\tr.say(text.WorkingOutTheCost())",
        "\tr.say(text.WorkingOutTheCost())",
        "TestAPreviewDoesItsDiskWorkOffTheInterfaceThread",
    ),
    (
        # The screen is never handed back, so a finished preview leaves the form
        # frozen and the only way out is to close the window.
        "a preview never gives the screen back",
        "internal/gui/window/run.go",
        "\tr.busy.set(false, busyFace{})\n\tif runErr != nil {",
        "\tif runErr != nil {",
        "TestAPreviewGivesTheScreenBackWhenItIsDone",
    ),
    (
        # The screen is marked busy before the settings have been judged, so a
        # preview that is refused straight away leaves the form frozen with a
        # refusal on it and nothing to press.
        "a preview freezes the screen before it knows it is refused",
        "internal/gui/window/run.go",
        "\tr.clearProblems()\n\ttargets, opt, err := r.settle()\n\tif err != nil {\n\t\tr.refuse(err)\n\t\treturn\n\t}\n\topt.DryRun = true",
        "\tr.clearProblems()\n\tr.busy.set(true, busyFace{})\n\ttargets, opt, err := r.settle()\n\tif err != nil {\n\t\tr.refuse(err)\n\t\treturn\n\t}\n\topt.DryRun = true",
        "TestARefusedPreviewLeavesTheScreenUsable",
    ),
    (
        # The count stops following the box, so it keeps saying what the value
        # WAS. A stale number beside a box is worse than no number: the reader
        # has no way to tell it is stale.
        "the count of bytes stops following what is typed in the box",
        "internal/gui/parts/fields.go",
        "func(value string, into *ByteCount) { into.show(value) })",
        "func(value string, into *ByteCount) {})",
        "TestABoxHoldingASizeSaysWhatItComesTo",
    ),
    (
        # A count appears on every box whose value happens to parse as a size,
        # which is the version this was deliberately not built as - "how many"
        # holds 1 and would grow a caption reading "1 B".
        "a count of bytes appears on boxes that do not hold a size",
        "internal/gui/parts/fields.go",
        "	if !s.inBytes[setting] {\n		return nil\n	}",
        "	if false {\n		return nil\n	}",
        "TestOnlyABoxHoldingASizeCarriesACount",
    ),
    (
        # Back to handing the engine the recipe reader's answer for an absent
        # output directory, which is the current one - so clearing the box on
        # the batch screen writes beside the window without a word. O125, and
        # the same defect O102 fixed for the other two screens.
        "an emptied output directory on the batch screen means the current one again",
        "internal/gui/window/recipe.go",
        "	if typed == \"\" {\n		return \"\"\n	}\n	return parsed",
        "	return parsed",
        "TestClearingTheOutputDirectoryRefusesTheRunOnEveryScreen",
    ),
    (
        # A declared setting that holds a size stops saying what it comes to.
        # This is the call the preset screen only started making on 2026-08-24 -
        # before that it drew its parameters with a loop of its own and this had
        # to be written twice.
        "a declared setting that holds a size stops carrying its count",
        "internal/gui/parts/property.go",
        "		if p.Kind == format.PropertySize {\n			into.InBytes(p.Name)\n		}",
        "		if false && p.Kind == format.PropertySize {\n			into.InBytes(p.Name)\n		}",
        "TestADeclaredSizeSaysWhatItComesToOnEveryScreenThatDrawsOne",
    ),
    (
        # Back to the toolkit's default, which is 11 of 255 off a fill chosen
        # for somebody else's palette - and it is the only mark left saying
        # where a value goes.
        "the mark round a box to type in goes back to the toolkit's own",
        "internal/gui/parts/theme.go",
        "theme.ColorNameInputBorder:     hex(0x65, 0x65, 0x6D),",
        "theme.ColorNameInputBorder:     hex(0x39, 0x39, 0x3A),",
        "TestABoxToTypeInWearsAMarkThatCanBeSeen",
    ),
    (
        # The star stops being drawn while every screen still declares which
        # fields it cannot run without. Nothing else on any screen looks at
        # this mark, so without a guard the whole of it could go and the form
        # would read as if it never had one.
        "the star saying a field must be filled in stops being drawn",
        "internal/gui/parts/required.go",
        "\tif required {\n\t\trow = append(row, newRequiredMark())",
        "\tif required && false {\n\t\trow = append(row, newRequiredMark())",
        "TestTheStarsDrawnAreTheOnesTheScreenDeclared",
    ),
    (
        # A setting the run refuses when it is empty stops being marked. This
        # is the drift the declaration exists to risk, so it is the one worth
        # proving the engine half catches: the box goes on refusing and the
        # only way to find out is to press a button.
        "a box the run will not do without stops carrying its star",
        "internal/gui/window/generate.go",
        "\tg.fields.Require(format.SettingSize, engine.SettingCount, engine.SettingID,",
        "\tg.fields.Require(engine.SettingCount, engine.SettingID,",
        "TestAStarIsOnEveryBoxTheRunWillNotDoWithout",
    ),
    (
        # And the other way round, which is the half a guard reading only the
        # declaration would never see: a star on a field nobody has to fill in
        # asks for work the tool does not need.
        "a star appears on a box the run is happy to leave empty",
        "internal/gui/window/generate.go",
        "\t\tengine.SettingSeed, engine.SettingOutDir)",
        "\t\tengine.SettingSeed, engine.SettingOutDir, engine.SettingName)",
        "TestAStarIsOnEveryBoxTheRunWillNotDoWithout",
    ),
    (
        # A slot is machinery rather than a word. A refusal that carries one and
        # is never asked to render it puts "{setting}" in front of somebody, and
        # from the side that writes the sentence that is invisible - it compiles,
        # it passes vet and it reads correctly in the source.
        "a refusal keeps its slot instead of putting a name in it",
        "internal/recipe/errors.go",
        "	return core.InTheWordsOf(",
        "	return (func(m, n string) string { return m })(",
        "TestNoRefusalEverShowsASlotToAPerson",
    ),
    (
        # An empty name is what a field with no label asks for. Answering it with
        # the sentence untouched leaves the slots standing, which is the one
        # thing a slot must never do.
        "a refusal with nobody to name it leaves the slot showing",
        "internal/recipe/errors.go",
        "	if name == \"\" {\n		name = core.LastSettingSegment(p.At)\n	}",
        "	if false && name == \"\" {\n		name = core.LastSettingSegment(p.At)\n	}",
        "TestNoRefusalEverShowsASlotToAPerson",
    ),
    (
        # The article rule answers from the first written letter, which is right
        # for every name either surface uses and is not the whole of English.
        # Losing a vowel gives "a id", which reads as a typo rather than as a
        # mechanism coming apart.
        "the article in front of a setting stops matching the name after it",
        "internal/core/setting.go",
        "	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':",
        "	case 'a', 'e', 'o', 'u', 'A', 'E', 'O', 'U':",
        "TestEveryNameARefusalCanBeGivenTakesTheArticleThisRuleGivesIt",
    ),
    (
        # Every word became a lookup, and from inside the build that is
        # invisible: the English is still written beside each entry and answers
        # when no catalogue is loaded. A seam that stops asking looks exactly
        # right in English and can never be translated.
        "the window stops asking the catalogue and answers in English forever",
        "internal/gui/text/catalogue.go",
        "\t\treturn pseudoOf(english)\n\t}\n\tif localiser == nil {",
        "\t\treturn pseudoOf(english)\n\t}\n\tif true || localiser == nil {",
        "TestTheWindowSpeaksWhateverTheCatalogueSays",
    ),
    (
        # English is written in the code and copied into en.json for translators.
        # Two places holding one sentence, and only this guard between them - a
        # message changed on one side leaves a translator working from the other.
        "the English in the code walks away from the English a translator is given",
        "internal/gui/text/screens.go",
        'func FieldSize() string         { return say("FieldSize", "Size") }',
        'func FieldSize() string         { return say("FieldSize", "Size of each file") }',
        "TestTheEnglishCatalogueSaysWhatTheCodeSays",
    ),
    (
        # D17 is a flat hyphen and no semicolons, and PRODUCT.md puts translation
        # files in its scope. The punctuation guard covers neither internal/gui
        # nor any JSON, so on the day the first catalogue appeared the rule lost
        # its guard exactly where the document declares it applies.
        "a translation takes a dash D17 does not allow",
        "internal/gui/text/locale/en.json",
        "The tool is free and stays free - this pays",
        "The tool is free and stays free – this pays",
        "TestEveryTranslationObeysThePunctuationRule",
    ),
    (
        # A folded batch with no line under its name is a column of headings
        # somebody opens one by one to find the one they meant, which is a worse
        # screen than a long one.
        "a folded batch says nothing about what is in it",
        "internal/gui/window/recipefolds.go",
        "			fold.Say(say())",
        "			fold.Say(\"\")",
        "TestAFoldedBatchSaysWhatIsInIt",
    ),
    (
        # The whole reason putting batches away was rejected on 2026-08-18: a
        # batch that is not on the screen has no box to mark, so the form
        # refuses to run and marks nothing anybody can see.
        "a refusal leaves the batch it is about folded shut",
        "internal/gui/window/recipe.go",
        "	r.runner.unfold = r.openFoldHolding",
        "	r.runner.unfold = nil",
        "TestARefusalOpensTheBatchItIsAbout",
    ),
    (
        # Copying is what makes several batches quick to write, and batches
        # usually differ in one setting. A copy that arrives empty is a form
        # somebody types out again.
        "copying a batch copies nothing but the batch",
        "internal/gui/window/recipe.go",
        "	to.count.SetText(from.count.Text)",
        "	to.count.SetText(\"\")",
        "TestACopiedBatchCarriesEverythingButItsName",
    ),
    (
        # The switch is built with the batch rather than with the panel that
        # draws it, so a copy can read it. Built later, a copy took the values
        # of the other two ways and then showed the first one.
        "a copied batch forgets which way it states its size",
        "internal/gui/window/recipe.go",
        "	if from.sizeWay != nil && to.sizeWay != nil {",
        "	if false && from.sizeWay != nil && to.sizeWay != nil {",
        "TestACopiedBatchCarriesEverythingButItsName",
    ),
(
    # A refusal about a box two floors down has to open both. Opening only the
    # batch is the defect of 2026-08-18 one floor lower: the screen refuses to
    # run and marks a box nobody can see.
    "a refusal opens the batch but not the section inside it",
    "internal/gui/window/recipefolds.go",
    "for _, fold := range []*parts.Folding{b.fold, b.settings, b.notes.fold} {",
    "for _, fold := range []*parts.Folding{b.fold} {",
    "TestARefusalOpensTheSectionTheBoxIsIn",
),
(
    # The section is named for where its settings go. One that carried nothing
    # to the manifest would be a fold hiding boxes with no effect anywhere.
    "the kind of case stops reaching the manifest",
    "internal/gui/window/recipe.go",
    "Group:          b.notes.group.Text,",
    "Group:          \"\",",
    "TestNothingInTheManifestNotesChangesAByte",
),
(
    # A section that arrives folded and swallows what somebody typed is a
    # setting they cannot see and did not remove.
    "a folded section says nothing about the notes in it",
    "internal/gui/window/recipefolds.go",
    "return text.FoldedSummary(n.group.Text, n.purpose.Text, outcomeName(n.expected.Selected), reasonName(n.reason.Selected))",
    "return \"\"",
    "TestASectionPutAwayStillSaysWhatIsInIt",
),
(
    # The owner's decision of 2026-08-25: both sections arrive put away, which
    # is 248 px of form per screen.
    "the settings of a batch arrive open instead of folded",
    "internal/gui/window/recipe.go",
    "settingsFolded: true,",
    "settingsFolded: false,",
    "TestTheSectionsInsideABatchArriveFolded",
),
(
    # Until 2026-08-25 the declaration's verdict arrived a layer up, where a
    # target is an entry in a list rather than targets[2] - so a mistyped width
    # had no address in validate --json and nothing for a form to mark.
    "the recipe stops asking the format about the values it was given",
    "internal/recipe/target.go",
    "\tif len(stated) == 0 {\n\t\treturn\n\t}",
    "\tif len(stated) >= 0 {\n\t\treturn\n\t}",
    "TestEveryRecipeRefusalSaysWhichSettingItIsAbout",
),
(
    # A refusal with an empty reason prints as a bare dash, which is what the
    # first version of the property check did on 2026-08-25 and what nothing
    # caught until this guard asked every case rather than one recipe.
    "a refusal about a property loses the reason behind it",
    "internal/recipe/target.go",
    "what, why, instead := unknown.Parts()\n\t\t\tp.add(at, whereWhat(where, what), why, instead)",
    "what, _, instead := unknown.Parts()\n\t\t\tp.add(at, whereWhat(where, what), core.Said{}, instead)",
    "TestEveryRecipeRefusalSaysWhichSettingItIsAbout",
),
(
    # A box drops a shortcut it does not know without a word, so a shortcut
    # registered anywhere else never arrives while somebody is typing - which
    # is exactly when they would press Ctrl+Enter.
    "a box swallows the window's own shortcuts instead of passing them on",
    "internal/gui/parts/entry.go",
    "if _, ours := shortcut.(*desktop.CustomShortcut); ours && e.onOurs != nil {",
    "if _, ours := shortcut.(*desktop.CustomShortcut); ours && e.onOurs != nil && false {",
    "TestTheKeyboardStartsARunFromInsideABox",
),
(
    # Ctrl+P is a separate entry in the table from Ctrl+Enter, so breaking one
    # says nothing about the other.
    "the preview shortcut is not wired to anything",
    "internal/gui/window/open.go",
    "\t\tkeyboardScreen.PressPreview)",
    "\t\tfunc(keyboardScreen) {})",
    "TestThePreviewShortcutAsksWithoutWriting",
),
(
    # Owner's decision of 2026-08-25: the keyboard starts on the first field of
    # the screen being looked at, so a person can type without reaching for the
    # mouse first.
    "the keyboard starts nowhere when a screen opens",
    "internal/gui/window/open.go",
    "\t\tparts.FocusQuietly(h.Canvas(), first)",
    "\t\t_ = first",
    "TestTheKeyboardStartsOnTheFirstFieldOfTheScreen",
),
(
    # A finished run offers the folder it wrote into - point 20 of the rebuild
    # and the last live part of O85.
    "a finished run does not offer the folder it wrote into",
    "internal/gui/window/runoffers.go",
    "\to.folderBtn.Show()",
    "\to.folderBtn.Hide()",
    "TestAFinishedRunOffersTheFolderItWroteInto",
),
(
    # The offer belongs to one run. Left standing it points at the results of
    # the run before this one, and somebody would open it and believe them.
    "the folder offer keeps pointing at the run before this one",
    "internal/gui/window/run.go",
    "\tr.offer.wroteInto = opt.OutDir",
    "\t_ = opt.OutDir",
    "TestTheFolderOfferGoesAwayWhenTheNextRunStarts",
),
(
    # Escape has to be harmless while nothing is running. Broken the other way
    # round - by making it start a run instead of stopping one - because a
    # guard about the ABSENCE of an effect cannot be reddened by removing one.
    "Escape starts a run instead of stopping one",
    "internal/gui/window/open.go",
    "wire(&desktop.CustomShortcut{KeyName: fyne.KeyEscape}, keyboardScreen.PressCancel)",
    "wire(&desktop.CustomShortcut{KeyName: fyne.KeyEscape}, keyboardScreen.PressGenerate)",
    "TestEscapeIsOnlyLiveWhileThereIsSomethingToStop",
),
    (
        "a menu goes back to taking the whole column, so it is the same shape as a box to type in",
        "internal/gui/parts/field.go",
        "	if menu, is := control.(*Chooser); is {\n		return Menu(menu)\n	}",
        "	if menu, is := control.(*Chooser); is && false {\n		return Menu(menu)\n	}",
        "TestAMenuIsNotDrawnAsWideAsABoxToTypeIn",
    ),
    (
        "a menu draws no line when nobody is using it, so it has no edge of its own again",
        "internal/gui/parts/ring.go",
        "	case r.resting != nil:",
        "	case r.resting != nil && false:",
        "TestTheMenuTheKeyboardIsInDrawsALine",
    ),
    (
        "the directory the window was last told to use is not offered when it opens",
        "internal/gui/window/open.go",
        "	if last == \"\" || LeftByTheOldOffer(last, programDirectory()) {",
        "	if last == \"\" || LeftByTheOldOffer(last, programDirectory()) || true {",
        "TestTheWindowOffersTheDirectoryItWasLastToldToUse",
    ),
    (
        "closing the window forgets where the files were going",
        "internal/gui/window/open.go",
        "		if screen, ok := working[*showing]; ok {\n			h.Remembered().RememberDirectory(screen.OutDir())\n		}",
        "		if screen, ok := working[*showing]; ok && false {\n			h.Remembered().RememberDirectory(screen.OutDir())\n		}",
        "TestClosingTheWindowRemembersWhereTheFilesWereGoing",
    ),
    (
        "a first start offers the working directory itself instead of a folder of ours",
        "internal/gui/window/offered.go",
        "	return filepath.Join(working, OutputFolderName)",
        "	return filepath.Join(working, \"\")",
        "TestAFirstStartOffersTheFolderItAlwaysDid",
    ),
    (
        "a remembered window size is centred anyway, which puts a big one out of reach",
        "internal/gui/window/opening.go",
        "	return remembered, false",
        "	return remembered, true",
        "TestARememberedSizeIsUsedAndNotRecentred",
    ),
    (
        "a first start is not centred, so it lands wherever the system chose",
        "internal/gui/window/opening.go",
        "	return size, true",
        "	return size, false",
        "TestAFirstStartOpensAsTallAsTheScreensWantInTheMiddle",
    ),
    (
        "a size with a nought in it counts as a size, so a minimised window comes back as nothing",
        "internal/gui/window/remembered.go",
        "	return size.Width > 0 && size.Height > 0",
        "	return size.Width >= 0 && size.Height >= 0",
        "TestASizeWithANoughtInItIsNotWorthRemembering",
    ),
    (
        "a doc comment opens by naming a different exported declaration of the same package",
        "internal/core/size.go",
        "// BoundarySizes turns a limit into the three sizes a boundary set means: one",
        "// ParseSizeRange turns a limit into the three sizes a boundary set means: one",
        "TestNoDocCommentOpensByNamingADifferentDeclaration",
    ),
    (
        "the shape this guard was written for: a paragraph about Plan left sitting above an unexported helper",
        "internal/engine/engine.go",
        "// settleTarget checks everything that has to be true about one target before",
        "// Plan checks everything that has to be true about one target before",
        "TestNoDocCommentOpensByNamingADifferentDeclaration",
    ),
    (
        "the rule about characters the host cannot store stops being asked",
        "internal/engine/filename.go",
        "\tcase firstForbidden(name) != 0:",
        "\tcase false && firstForbidden(name) != 0:",
        "TestANameTheHostCannotStoreIsRefusedBeforeAnythingIsWritten",
    ),
    (
        "only the control character half of the name rule stops being asked",
        "internal/engine/filename.go",
        "\t\tif r < 0x20 || strings.ContainsRune(forbiddenChars, r) {",
        "\t\tif r < 0x20 && false || strings.ContainsRune(forbiddenChars, r) {",
        "TestANameTheHostCannotStoreIsRefusedBeforeAnythingIsWritten",
    ),
    (
        "the name rule grows to refuse a character the host stores perfectly well",
        "internal/engine/filename.go",
        'const forbiddenChars = `<>"|?*`',
        'const forbiddenChars = `<>"|?*-`',
        "TestAnOrdinaryNameIsStillAccepted",
    ),
    (
        "progress stops being serialised, so two reports run at once",
        "internal/engine/parallel.go",
        "\tg.report(Progress{",
        "\tgo g.report(Progress{",
        "TestNoTwoProgressReportsArriveAtOnce",
    ),
    (
        "a writer tops the bar up by the whole file instead of what it had left to say",
        "internal/engine/parallel.go",
        "\t\tp.gate.finished(plan.Bytes-p.reported, plan.Bytes+plan.Work-p.worked)",
        "\t\tp.gate.finished(plan.Bytes, plan.Bytes+plan.Work-p.worked)",
        "TestProgressOverSeveralWritersStillReachesTheEndAndNeverGoesBack",
    ),
    (
        "a stopped run goes back to recording a prefix, so a file that finished is left unnamed",
        "internal/engine/engine.go",
        "\t\tcase r.ok:",
        "\t\tcase r.ok && stopped == nil:",
        "TestARunStoppedPartWayNamesEveryFileThatFinished",
    ),
    (
        "the null device stops being refused, so a run records a file that went nowhere",
        "internal/engine/filename.go",
        '\tcase strings.EqualFold(name, "nul"):',
        '\tcase false && strings.EqualFold(name, "nul"):',
        "TestANameTheHostCannotStoreIsRefusedBeforeAnythingIsWritten",
    ),
    (
        "the device rule grows into the folklore version, refusing nul.txt as well",
        "internal/engine/filename.go",
        '\tcase strings.EqualFold(name, "nul"):',
        '\tcase strings.HasPrefix(strings.ToLower(name), "nul"):',
        "TestAnOrdinaryNameIsStillAccepted",
    ),
    (
        "a release grows a second heading for one kind of change",
        "CHANGELOG.md",
        "## [0.2.0] - 2026-08-28\n\n### Breaking",
        "## [0.2.0] - 2026-08-28\n\n### Breaking\n\n### Breaking",
        "TestTheChangelogHasOneHeadingPerKindOfChange",
    ),
    (
        "a kind of change nobody can filter on is invented",
        "CHANGELOG.md",
        "## [0.2.0] - 2026-08-28\n\n### Breaking",
        "## [0.2.0] - 2026-08-28\n\n### Broken",
        "TestTheChangelogHasOneHeadingPerKindOfChange",
    ),
    (
        "the ceiling refusal stops naming the batch that caused it",
        "internal/engine/engine.go",
        "\t\t\t\tSetting: core.TargetAddress(i+1, SettingCount),",
        '\t\t\t\tSetting: "",',
        "TestARefusalFromTheEngineAboutTheWholeRunMarksTheBatchThatCausedIt",
    ),
    (
        "the ceiling refusal names the batch before the one that caused it",
        "internal/engine/engine.go",
        "\t\t\t\tSetting: core.TargetAddress(i+1, SettingCount),",
        "\t\t\t\tSetting: core.TargetAddress(i, SettingCount),",
        "TestARefusalFromTheEngineAboutTheWholeRunMarksTheBatchThatCausedIt",
    ),
    (
        "the report stops carrying the address of a refusal from below the reader",
        "internal/cli/recipecmd.go",
        "entry := validateProblem{What: err.Error(), At: addressOf(err)}",
        "entry := validateProblem{What: err.Error()}",
        "TestEveryAddressTheReportCarriesSaysWhereItIs",
    ),
    (
        "an address that names no target starts being cut like one",
        "internal/core/setting.go",
        '\treturn strings.HasPrefix(address, KeyTargets+"[")',
        "\treturn true",
        "TestARefusalAboutTheRunItselfKeepsItsWholeAddress",
    ),
    (
        "a refusal from below stops carrying the position of its target",
        "internal/engine/errors.go",
        "\treturn &addressedError{err: err, at: core.TargetAddress(position, setting)}",
        "\treturn err",
        "TestOneRefusalMarksItsBoxOnBothScreens",
    ),
    (
        "the single batch screen stops dropping the position it is handed",
        "internal/gui/window/runrefuse.go",
        "\treturn core.LastSettingSegment(address)",
        "\treturn address",
        "TestOneRefusalMarksItsBoxOnBothScreens",
    ),
    (
        "the manifest name goes back to calling itself a file name",
        "internal/engine/engine.go",
        "if err := checkFileName(SettingOutputManifest, core.Says(\"engine.TheManifest\", \"the manifest\"), opt.ManifestName); err != nil {",
        "if err := checkFileName(SettingName, core.Says(\"engine.TheManifest\", \"the manifest\"), opt.ManifestName); err != nil {",
        "TestARefusalAboutTheManifestNameMarksTheManifestBox",
    ),
    (
        "validate stops looking at the name of the manifest",
        "internal/cli/recipecmd.go",
        "\t\tManifestName: rec.Output.Manifest,",
        "",
        "TestValidateRefusesWhatGenerateWouldRefuse",
    ),
    (
        "a refusal from below stops handing out its reason",
        "internal/format/format.go",
        "func (e *BelowMinimumError) Why() string { return e.Reason.String() }",
        'func (e *BelowMinimumError) Why() string { return "" }',
        "TestARefusalFromBelowTheReaderArrivesInItsThreeParts",
    ),
    (
        "the report stops asking a refusal for its parts",
        "internal/cli/recipecmd.go",
        "\tif errors.As(err, &parted) {",
        "\tif false && errors.As(err, &parted) {",
        "TestARefusalFromBelowTheReaderArrivesInItsThreeParts",
    ),
    (
        "the parts a refusal hands out come apart from the sentence it prints",
        "internal/format/container.go",
        "func (e *NotAContainerError) Instead() string { return e.instead().String() }",
        'func (e *NotAContainerError) Instead() string { return "Change the format" }',
        "TestARefusalFromBelowTheReaderArrivesInItsThreeParts",
    ),
    (
        "a paragraph comes adrift onto an unexported declaration below it",
        "internal/cli/verify.go",
        "// verify compares a directory against a manifest an earlier run wrote.",
        "// describeError compares a directory against a manifest an earlier run wrote.",
        "TestNoDocCommentOpensByNamingADifferentDeclaration",
    ),
    (
        "the engine stops joining the remedy into the sentence it prints",
        "internal/engine/errors.go",
        "case !e.Because.IsZero() && !e.Remedy.IsZero():",
        "case !e.Because.IsZero() && !e.Remedy.IsZero() && false:",
        "TestARefusalFromBelowTheReaderArrivesInItsThreeParts",
    ),
    (
        "a refusal of the engine stops saying why the rule is there",
        "internal/engine/filename.go",
        "Because: core.Says(\"engine.NameForbiddenWhy\", \"Windows refuses that character in a file name, so the file is not written there at all. It is refused on every system so that a recipe means one thing everywhere\"),",
        "Because: core.Said{},",
        "TestARefusalFromBelowTheReaderArrivesInItsThreeParts",
    ),
    (
        "the manifest stops holding its own name, so a target can take it",
        "internal/engine/engine.go",
        "names[collisionKey(manifestNameOf(opt))] = nameOwner{",
        'names[collisionKey("")] = nameOwner{',
        "TestNoTargetCanTakeTheNameTheManifestNeeds",
    ),
    (
        "only a manifest somebody named is held, so the default one is open again",
        "internal/engine/engine.go",
        "names[collisionKey(manifestNameOf(opt))] = nameOwner{",
        "names[collisionKey(opt.ManifestName)] = nameOwner{",
        "TestNoTargetCanTakeTheNameTheManifestNeeds",
    ),
    (
        "the manifest collision goes back to reading like two targets, with no address",
        "internal/engine/engine.go",
        "name: manifestNameOf(opt), manifest: true}",
        "name: manifestNameOf(opt), manifest: false}",
        "TestNoTargetCanTakeTheNameTheManifestNeeds",
    ),
    (
        "verify goes back to comparing a manifest path as it was written",
        "internal/audit/audit.go",
        "seen[comparablePath(f.Path)] = true",
        "seen[f.Path] = true",
        "TestVerifyMatchesAPathSpelledTheLongWayRound",
    ),
    (
        "the copy replacing somebody else's file stops reaching the disk",
        "internal/core/replace.go",
        "if err := f.Sync(); err != nil {",
        "if err := error(nil); err != nil {",
        "TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst",
    ),
    (
        "a report that could not be written goes back to being a success",
        "internal/cli/cli.go",
        "if err := enc.Encode(v); err != nil {",
        "if err := enc.Encode(v); err != nil && false {",
        "TestAReportThatCouldNotBeWrittenWholeIsNotASuccess",
    ),
    (
        "the exit code of a broken report stops being an IO failure",
        "internal/cli/cli.go",
        "if code == ExitOK {",
        "if code == ExitOK && false {",
        "TestAReportThatCouldNotBeWrittenWholeIsNotASuccess",
    ),
    (
        "a recipe past the ceiling goes back to being a fault of the tool",
        "internal/cli/errors.go",
        "if errors.As(err, &recipeTooLarge) {",
        "if errors.As(err, &recipeTooLarge) && false {",
        "TestARecipeTooLargeToBeWrittenByHandIsRefused",
    ),
    (
        "the recipe reader stops refusing what it could not read whole",
        "internal/cli/recipecmd.go",
        "if int64(len(src)) <= recipe.MaxBytes {",
        "if len(src) >= 0 {",
        "TestARecipeTooLargeToBeWrittenByHandIsRefused",
    ),
    (
        "a boundary limit stops being refused at the low end",
        "internal/core/size.go",
        "if limit < 1 {",
        "if limit < 1 && false {",
        "TestABoundaryLimitIsRefusedAtBothEnds",
    ),
    (
        "the low end of a boundary is refused only at zero, not below it",
        "internal/core/size.go",
        "if limit < 1 {",
        "if limit == 0 {",
        "TestABoundaryLimitIsRefusedAtBothEnds",
    ),
    (
        "the walk of the directory stops asking whether it was cancelled",
        "internal/audit/walk.go",
        "\t\tif err := ctx.Err(); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif d.IsDir() {",
        "\t\tif d.IsDir() {",
        "TestACancelledVerifyStopsInTheWalkToo",
    ),
    (
        "the refusal for too many files names a number the rule does not use",
        "internal/core/limits.go",
        "A(\"Most\", MaxFilesPerRun))",
        "A(\"Most\", MaxFilesPerRun+1))",
        "TestTheRefusalForTooManyFilesNamesTheCeilingItEnforces",
    ),
    (
        "the buffered write is never flushed, so the file on disk is short",
        "internal/engine/parallel.go",
        "\tif writeErr == nil {\n\t\twriteErr = buffered.Flush()\n\t}\n",
        "",
        "TestTheFileOnTheDiskIsTheSizeThatWasOrdered",
    ),
    (
        "a second file introduces the package as well",
        "internal/cli/cleanup.go",
        "// Part of package cli. See cli.go.",
        "// Package cli is the command line.",
        "TestNoPackageIntroducesItselfTwice",
    ),
    (
        # Both sentences describing a setting come from the declaration, and
        # for size they used to be written out separately - so what a person
        # read depended on whether they had already typed something wrong. O64.
        "a size setting goes back to being described two ways",
        "internal/format/format.go",
        "\t\twhat = sizePhrase\n",
        "\t\twhat = \"a size written the way any size is, such as 2mb or a plain byte count\"\n",
        "TestASettingIsDescribedTheSameWayEmptyOrRefused",
    ),
    (
        # The way out this refusal used to offer is one the window refuses:
        # leaving the directory out is exactly what the person just did. O125.
        "the empty directory refusal offers leaving it out again",
        "internal/engine/engine.go",
        "core.Says(\"engine.NoOutDirFix\", \"Name a directory, for example ./fixtures\")",
        "core.Says(\"engine.NoOutDirFix\", \"Name a directory, for example ./fixtures, or leave it out to use the current one\")",
        "TestTheEmptyDirectoryRefusalDoesNotOfferAWayOutThatIsRefused",
    ),
    (
        # A recipe naming both spellings writes two files on Windows and ONE on
        # macOS - measured on a Mac Mini, see engine/names.go and
        # tools/probes/apfs-case.py. That is the split D10 exists to close.
        #
        # 🔴 The replacement carries "_ = cases.Fold" and that line is the whole
        # point of it. The faithful revert is the lowercasing that was here
        # before, and on its own it leaves the cases import unused, so the tree
        # stops compiling and the runner answers BROKEN - a verdict that proves
        # nothing about a guard that is fine. That is O131 exactly, walked into
        # again on the day it was fixed. The blank assignment keeps the import
        # alive so the behaviour is the only thing that changes.
        "two names that are one file on macOS go back to being two names here",
        "internal/core/foldname.go",
        "func FoldName(name string) string {\n\treturn cases.Fold().String(norm.NFC.String(name))\n}",
        "func FoldName(name string) string {\n\t_ = cases.Fold\n\treturn norm.NFC.String(name)\n}",
        "TestTwoNamesThatFoldTogetherAreRefused",
    ),
    (
        # strings.EqualFold folds SIMPLY, which puts the long s in the same
        # orbit as s - so "maſs.txt" against "mass.txt" was answered with "they
        # differ only in case", about two letters that are plainly not one
        # letter in two sizes. A guard caught it on 2026-08-26, the day the
        # sentence was written.
        "two different letters are called a difference of case again",
        "internal/engine/names.go",
        "case strings.ToLower(norm.NFC.String(owner.name)) == strings.ToLower(norm.NFC.String(name)):",
        "case strings.EqualFold(owner.name, name):",
        "TestTwoNamesThatFoldTogetherAreRefused",
    ),
    (
        # gen-locale.py writes the catalogue from the calls it can see, so a
        # sentence that states itself is simply absent from en.json - and a
        # translator copying that file is handed an incomplete one with nothing
        # to say so. Eighteen sentences were in that state until 2026-08-26.
        # O130.
        "a sentence in the window stops going through the catalogue",
        "internal/gui/text/text.go",
        "separator + say(\"PreviewNothingWritten\", \"nothing written yet\")",
        "separator + \"nothing written yet\"",
        "TestEverySentenceInTheTextPackageGoesThroughTheCatalogue",
    ),
    (
        # A plural hands over BOTH English forms, so a translation file may
        # carry as many as its language needs - Polish has three. Sending one
        # sentence with the number in it instead would put a thing into the
        # catalogue that is broken for Polish before anybody starts, and it
        # would look complete.
        "a plural stops offering both English forms to a translator",
        "internal/gui/text/text.go",
        "return sayN(\"Files\", \"1 file\", \"{{.Count}} files\", n, nil)",
        "return sayf(\"Files\", \"{{.Count}} files\", map[string]any{\"Count\": n})",
        "TestTheEnglishCatalogueSaysWhatTheCodeSays",
    ),
    (
        "the site stops giving each language its own address, so two languages write to one page",
        "internal/site/render.go",
        'if l.Dir != "" {',
        'if false && l.Dir != "" {',
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the download table stops offering the Windows build for ARM that the release actually produces",
        "internal/guard/site_test.go",
        '{System: "Windows", CLI: []string{"amd64", "arm64"}, GUI: []string{"amd64"}},',
        '{System: "Windows", CLI: []string{"amd64"}, GUI: []string{"amd64"}},',
        "TestTheSiteOffersEveryBinaryTheReleaseBuilds",
    ),
    (
        "a unit the registry counts in loses its Polish word, so that column would read as English on a Polish page",
        "web/content/pl/site.json",
        '"pixels": "pikseli",',
        '"pixelsGone": "pikseli",',
        "TestEveryLanguageDescribesEverythingTheProgramCanProduce",
    ),
    (
        "a page loses its twin in the other language, so the language switch on it would lead nowhere",
        "web/content/pl/site.json",
        '"key": "faq",',
        '"key": "faq-orphan",',
        "TestEveryPageExistsInEveryLanguage",
    ),
    (
        "Polish leaks onto an English page, which is the border the site rule stands on",
        "web/content/en/site.json",
        '"navLabel": "Main",',
        '"navLabel": "Główna",',
        "TestTranslatedTextStaysOnTheTranslatedPages",
    ),
    (
        "an English page picks up a semicolon, which rule 13 does not allow anywhere a user reads",
        "web/content/en/index.html",
        "Command line and desktop window,",
        "Command line and desktop window;",
        "TestTheSiteFollowsThePunctuationRule",
    ),
    (
        "a link on the busiest page points at a page that is not published",
        "web/content/en/index.html",
        '<a href="/docs/">documentation</a>',
        '<a href="/documentation/">documentation</a>',
        "TestEveryLinkOnTheSiteLeadsSomewhere",
    ),
    (
        "the WAV ceiling goes away, so a file past four gigabytes states a wrapped length",
        "internal/format/wav/wav.go",
        "\tif r.Bytes > maxFileBytes {",
        "\tif false && r.Bytes > maxFileBytes {",
        "TestAFormatRefusesASizeItCannotDescribe",
    ),
    (
        "a refusal about a size that is too large goes back to saying the file is too small",
        "internal/format/format.go",
        "core.Says(\"format.AboveMaximum\", \"%s cannot be larger than",
        "core.Says(\"format.AboveMaximum\", \"%s cannot be smaller than",
        "TestARefusalAboutTooLargeSaysLargerNotSmaller",
    ),
    (
        "one container declares a shared setting its own way instead of the shared way",
        "internal/format/zip/zip.go",
        "\t\tProperties: archive.Axes(archive.Entries, archive.EntryFormat, archive.EntrySize,\n\t\t\tarchive.Compression, archive.Depth, archive.DirectoryEntries,\n\t\t\tarchive.Password, archive.Encryption),",
        '\t\tProperties: append(archive.Axes(archive.Entries, archive.EntryFormat,\n\t\t\tarchive.Compression, archive.Depth, archive.DirectoryEntries,\n\t\t\tarchive.Password, archive.Encryption), ''format.Property{Name: archive.EntrySize, Kind: format.PropertySize, Default: "4kb"}),',
        "TestEveryContainerDeclaresTheSharedAxesAsTheyAreDeclaredOnce",
    ),
    (
        "a picture format counts its width in a unit of its own",
        "internal/format/webp/webp.go",
        '\t\t\timagedim.Width(imagedim.Side{Largest: maxDimension,\n\t\t\t\tDetail: "How wide the picture is. Left out, the picture is sized to fill the bytes you asked for."}),',
        '\t\t\tformat.Property{Name: imagedim.SettingWidth, Kind: format.PropertyInt,\n\t\t\t\tMin: imagedim.Smallest, Max: maxDimension, Unit: "px",\n\t\t\t\tDetail: "How wide the picture is. Left out, the picture is sized to fill the bytes you asked for."},',
        "TestOneSettingNameMeansOneKindOfSetting",
    ),
    (
        "a tabular format stops offering more columns than a spreadsheet accepts",
        "internal/format/xlsx/xlsx.go",
        "\tmaxColumns = 32768",
        "\tmaxColumns = 64",
        "TestAFormatWithColumnsReachesPastWhatASpreadsheetAccepts",
    ),
    (
        "the backstop under the picture formats stops checking the range",
        "internal/format/imagedim/imagedim.go",
        "\tif n < Smallest || n > largest {",
        "\tif (n < Smallest || n > largest) && false {",
        "TestThePictureSideBackstopRefusesWhatTheDeclarationWould",
    ),
    (
        "the backstop starts changing the side it was given",
        "internal/format/imagedim/imagedim.go",
        "\treturn n, nil",
        "\treturn n + 1, nil",
        "TestASideInsideTheRangeIsTakenAsWritten",
    ),
    (
        "a text format declares the encoding setting its own way instead of the shared way",
        "internal/format/md/md.go",
        "\t\tProperties:       textenc.Axes(textenc.Setting, textenc.SettingBOM),",
        "\t\tProperties: []format.Property{\n\t\t\t{Name: textenc.Setting, Kind: format.PropertyChoice,\n\t\t\t\tChoices: []string{textenc.UTF16LE, textenc.UTF8}, Default: textenc.UTF8,\n\t\t\t\tDetail: \"Which encoding the characters are written in.\"},\n\t\t\ttextenc.Axes(textenc.SettingBOM)[0],\n\t\t},",
        "TestEveryTextFormatDeclaresTheEncodingSettingsAsTheyAreDeclaredOnce",
    ),
    (
        "zero-head stops zeroing the bytes it says it zeroes",
        "internal/damage/zerohead.go",
        "\t\tz.buf[i] = 0",
        "\t\tz.buf[i] = z.buf[i]",
        "TestZeroHeadZeroesTheBytesItSaysAndNoOthers",
    ),
    (
        "a damage claims it changed something whatever the bytes were",
        "internal/damage/zerohead.go",
        "\t\tif b != 0 {",
        "\t\tif b != 0 || true {",
        "TestADamageThatMovedNothingSaysSo",
    ),
    (
        "the damage floor stops following the settings and answers with the default",
        "internal/damage/zerohead.go",
        "\t\tFloor: func(v Values) int64 { return int64(headBytes(v)) },",
        "\t\tFloor: func(v Values) int64 { return defaultHead },",
        "TestTheFloorFollowsThePairOfSettings",
    ),
    (
        "a chain applies its damages in the order it was built rather than listed",
        "internal/damage/apply.go",
        "\tfor i := len(c) - 1; i >= 0; i-- {",
        "\tfor i := 0; i < len(c); i++ {",
        "TestAChainAppliesItsDamagesInTheOrderItListsThem",
    ),
    (
        "a chain stops noticing that one of its damages was idle",
        "internal/damage/apply.go",
        "\t\tif i < len(c) && !s.Touched() {",
        "\t\tif i < len(c) && !s.Touched() && false {",
        "TestADamageThatWouldChangeNothingStopsTheFile",
    ),
    (
        "the writer stops putting damage in the path and writes a whole file",
        "internal/engine/parallel.go",
        "\tif f.Damaged() {\n\t\tdamaged, opened, err := f.Target.Damage.Open(counter)",
        "\tif f.Damaged() && false {\n\t\tdamaged, opened, err := f.Target.Damage.Open(counter)",
        "TestADamagedFileIsRefusedByAJudge",
    ),
    (
        "the manifest stops recording what was broken about a file",
        "internal/engine/damage.go",
        "\tif !f.Damaged() {\n\t\treturn nil\n\t}",
        "\tif !f.Damaged() || true {\n\t\treturn nil\n\t}",
        "TestTheManifestOfADamagedRunSaysSoAndExpectsRejection",
    ),
    (
        "a damaged file goes back to having no expectation declared for it",
        "internal/engine/engine.go",
        "\t\tif f.Damaged() {\n\t\t\treturn manifest.Expected{\n\t\t\t\tOutcome:    manifest.OutcomeReject,",
        "\t\tif f.Damaged() && false {\n\t\t\treturn manifest.Expected{\n\t\t\t\tOutcome:    manifest.OutcomeReject,",
        "TestTheManifestOfADamagedRunSaysSoAndExpectsRejection",
    ),
    (
        "planning stops refusing a file smaller than the damage it was given",
        "internal/engine/damage.go",
        "\t\tif size < floor {",
        "\t\tif size < floor && false {",
        "TestAFileSmallerThanItsDamageIsRefusedBeforeAnythingIsWritten",
    ),
    (
        # The help is rendered from the declaration now, so the way to make it
        # lie is to stop rendering one entry - not to edit a string.
        "a listed command stops being printed, so the help hides something the tool answers to",
        "internal/cli/commands.go",
        "\t\tif c.Summary == \"\" {\n\t\t\tcontinue\n\t\t}\n\t\tpad :=",
        "\t\tif c.Summary == \"\" || c.Verb == \"damage\" {\n\t\t\tcontinue\n\t\t}\n\t\tpad :=",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        # Take the summary OFF a real command rather than putting an empty
        # one on help, which was the first version of this entry and changed
        # nothing - that field was already empty. What this guard is for is a
        # verb that works and nobody is told about, so that is what to build.
        "a working command loses its summary, so it runs and nobody is told it exists",
        "internal/cli/commands.go",
        "			Verb: \"formats\", Summary: \"list the formats this build supports\",",
        "			Verb: \"formats\", Summary: \"\",",
        "TestACommandTheHelpDoesNotListOnlyPrintsTheHelp",
    ),
    (
        "the dispatch stops answering to a verb the help still lists",
        "internal/cli/commands.go",
        "\t\t\tVerb: \"formats\", Summary: \"list the formats this build supports\",",
        "\t\t\tVerb: \"formatz\", Shown: \"formats\", Summary: \"list the formats this build supports\",",
        "TestEveryCommandTheHelpListsIsOneTheToolAnswersTo",
    ),
    (
        "the README flag table loses a flag generate really takes",
        "README.md",
        "| `--damage <name>` |",
        "| `--damaged <name>` |",
        "TestTheReadmeFlagTableNamesEveryFlagGenerateTakes",
    ),
    (
        "the English page describes a command differently from the program",
        "web/content/en/site.json",
        '"formats": "list the formats this build supports",',
        '"formats": "list the formats supported",',
        "TestEveryLanguageDescribesEverythingTheProgramCanProduce",
    ),
    (
        "the one rule about damage and accept stops firing, so both surfaces go quiet at once",
        "internal/damage/refusals.go",
        "\tif len(c) == 0 || expected != RuledOutExpectation {",
        "\tif len(c) == 0 || expected != RuledOutExpectation || true {",
        "TestDamageBesideAnExpectationOfAcceptIsRefused",
    ),
    (
        # The engine half on its own. This is the mutation that reproduces
        # O199 exactly: the recipe keeps refusing and the command line stops,
        # which is the state 0.3.0 shipped in.
        "the engine stops asking, so the command line writes a damaged file it calls acceptable",
        "internal/engine/damage.go",
        "\treturn checkDamageExpectation(t)",
        "\treturn nil",
        "TestTheCommandLineRefusesDamageBesideAcceptToo",
    ),
    (
        "the refusal comes back as a format problem, telling a script to try another format",
        "internal/cli/errors.go",
        "\t\treturn ExitUsage, true\n\t}\n\tif code, ok := classifyFormat(err); ok {",
        "\t\treturn ExitFormat, true\n\t}\n\tif code, ok := classifyFormat(err); ok {",
        "TestTheCommandLineRefusesDamageBesideAcceptToo",
    ),
    (
        "the word damage rules out drifts from the one the manifest knows",
        "internal/damage/refusals.go",
        'const RuledOutExpectation = "accept"',
        'const RuledOutExpectation = "accepted"',
        "TestTheOutcomeDamageRulesOutIsTheOneTheManifestKnows",
    ),
    (
        # The site half, aimed at the arithmetic rather than at the refusal
        # beside it. Two earlier versions of this entry came back NOT CAUGHT
        # and both were statements about the ENTRY: the first named the guard
        # that reads the word lists, which never renders a page, and the second
        # broke the "no summary" branch - a branch nothing can reach while
        # every command has a summary, so the mutant compiled and proved
        # nothing. Losing a summary is reachable only by taking one out of the
        # language file, which is the entry below this one.
        #
        # The padding is content: the block sits in a <pre>, so a column one
        # space narrower is a page that differs from the published one.
        "the command column loses a space, so the block no longer lines up the way the help does",
        "internal/site/view.go",
        "\t\t\tPad:     strings.Repeat(\" \", widest-len(name)+2),",
        "\t\t\tPad:     strings.Repeat(\" \", widest-len(name)+1),",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the site stops listing a command the program prints, which is the defect this pair was built for",
        "web/content/en/site.json",
        '"damage": "list the ways this build can break a file on purpose",',
        '"damaged": "list the ways this build can break a file on purpose",',
        "TestEveryLanguageDescribesEverythingTheProgramCanProduce",
    ),
    (
        "the signing script goes back to the drive that vanishes when pwsh starts it",
        ".github/scripts/sign_release.py",
        "        \"foreach ($where in 'CurrentUser', 'LocalMachine') { \"",
        "        \"foreach ($where in 'CurrentUser') { \" \"# was Cert:\\CurrentUser \"",
        "TestTheSigningScriptDoesNotDependOnWhichShellStartedIt",
    ),
    (
        "the signing script hides what PowerShell said whenever the code is zero",
        ".github/scripts/sign_release.py",
        "    if out.stderr.strip():",
        "    if out.stderr.strip() and False:",
        "TestTheSigningScriptDoesNotDependOnWhichShellStartedIt",
    ),
    (
        # Instead(), not Error(). The first version of this entry mutated Error
        # and came back NOT CAUGHT - which was a statement about the MUTATION
        # rather than about the code: a recipe reports a refusal through What,
        # Why and Instead, and Error is only what the one-target flag path
        # prints. The names a person needs live in Instead on that road.
        "a refusal about an unknown damage stops naming the ones there are",
        "internal/damage/refusals.go",
        "return core.Says(\"damage.UnknownInstead\", \"use one of: %s\", core.A(\"Known\", strings.Join(e.Known, \", \")))",
        "return core.Says(\"damage.UnknownInsteadRemove\", \"remove the damage line\")",
        "TestADamageNobodyRegisteredIsRefusedWithTheNamesThereAre",
    ),
    (
        "a damage setting stops being checked against its declaration",
        "internal/damage/apply.go",
        "why := p.Allows(raw)\n\tif why.IsZero() {",
        "why := p.Allows(raw)\n\tif why.IsZero() || true {",
        "TestADamageSettingOutsideItsDeclarationIsRefused",
    ),
    (
        "a file nobody damaged is treated as damaged",
        "internal/engine/engine.go",
        "\treturn f.Target != nil && len(f.Target.Damage) > 0",
        "\treturn f.Target != nil && len(f.Target.Damage) >= 0",
        "TestARunWithNoDamageIsUnchanged",
    ),
    (
        "the damage floor becomes a wall and refuses the size it says it takes",
        "internal/engine/damage.go",
        "\t\tif size < floor {",
        "\t\tif size <= floor {",
        "TestASizeTheDamageCanTakeIsPlanned",
    ),
    (
        "the window stops drawing the fields a damage declares",
        "internal/gui/window/generate.go",
        "\tfields, objects := parts.DeclaredFields(text.DamageOwner(d.ID), d.Parameters, g.fields, g.tips)",
        "\tfields, objects := parts.DeclaredFields(text.DamageOwner(d.ID), nil, g.fields, g.tips)",
        "TestTheWindowDrawsAFieldForEveryDamageParameter",
    ),
    (
        "the damage menu stops offering what the registry holds",
        "internal/gui/window/generate.go",
        "\tg.damage.pick = parts.NewChooser(append([]string{text.DamageNone()}, damage.Names()...),",
        "\tg.damage.pick = parts.NewChooser(append([]string{text.DamageNone()}, []string{}...),",
        "TestTheWindowOffersEveryDamageThereIs",
    ),
    (
        "the window draws the damage menu and drops what was chosen",
        "internal/gui/window/generate.go",
        "\t\tDamage:     g.chosenDamage(),",
        "\t\tDamage:     nil,",
        "TestARunFromTheWindowIsReallyDamaged",
    ),
    (
        "a picture format lets its side start at zero instead of at one pixel",
        "internal/format/bmp/bmp.go",
        "\t\t\timagedim.Width(imagedim.Side{Largest: maxDimension,\n\t\t\t\tDetail: \"How wide the picture is. Left out, the picture is sized to fill the bytes you asked for.\"}),",
        "\t\t\tformat.Property{Name: imagedim.SettingWidth, Kind: format.PropertyInt,\n\t\t\t\tMin: 0, Max: maxDimension, Unit: imagedim.Unit,\n\t\t\t\tDetail: \"How wide the picture is. Left out, the picture is sized to fill the bytes you asked for.\"},",
        "TestEveryPictureFormatDeclaresItsSidesAsTheImageDimensionPackageWould",
    ),
    (
        "a deliberate homonym stops being one and the excuse stays behind",
        "internal/format/logfile/log.go",
        '\t\t\tName: "entry_format", Kind: format.PropertyChoice,',
        '\t\t\tName: "entry_format", Kind: format.PropertyText,',
        "TestEveryDeliberateHomonymStillNamesTwoMeanings",
    ),
    (
        "a log stops terminating its lines, while still hitting the size to the byte",
        "internal/format/logfile/options.go",
        '\t\teol:        "\\n",',
        '\t\teol:        " ",',
        "TestEveryLogShapeWritesWholeLinesAtTheRightSize",
    ),
    (
        "the counter mode counts the way Go counts, not the way WinZip does",
        "internal/format/archive/lock.go",
        "binary.LittleEndian.PutUint64(c.in[:8], c.n)",
        "binary.BigEndian.PutUint64(c.in[:8], c.n)",
        "TestARealArchiverOpensALockedArchiveAndRefusesTheWrongPassword",
    ),
    (
        "a lock is charged one byte an entry more than it costs",
        "internal/format/archive/lock.go",
        "return int64(l.saltLen() + pwvLen + authLen)",
        "return int64(l.saltLen() + pwvLen + authLen + 1)",
        "TestALockedArchiveStillHitsTheSizeToTheByte",
    ),
    (
        "the authentication code is remembered as nine bytes rather than ten",
        "internal/format/archive/lock.go",
        "\tauthLen    = 10\n",
        "\tauthLen    = 9\n",
        "TestTheCostOfALockIsTheMeasuredCost",
    ),
    (
        "a password with the lock switched off is accepted, and the archive comes out open",
        "internal/format/archive/lock.go",
        "\tcase password != \"\" && !locked:\n",
        "\tcase false && password != \"\" && !locked:\n",
        "TestTheTwoHalvesOfALockHaveToAgree",
    ),
    (
        "the salt is derived from the entry alone, so two seeds give one file",
        "internal/format/archive/lock.go",
        "\trng := core.NewRand(core.FileSeed(seed, index))\n",
        "\trng := core.NewRand(uint64(index))\n",
        "TestALockedArchiveIsTheSameFileForTheSameSeed",
    ),
    (
        "a format stops saying why it cannot take a setting, so the refusal reads as a missing feature",
        "internal/format/refusals.go",
        "\t\tif u.Name == key {\n",
        "\t\tif false && u.Name == key {\n",
        "TestAFormatThatCannotLockSaysSoInItsOwnWords",
    ),
    (
        "the manifest stops recording the password, so nothing can open the file it describes",
        "internal/format/zip/zip.go",
        "\tif m.lock.On() {\n\t\tp.Properties[archive.Encryption] = m.lock.Method\n",
        "\tif false && m.lock.On() {\n\t\tp.Properties[archive.Encryption] = m.lock.Method\n",
        "TestTheManifestCarriesThePasswordSoATestCanOpenTheFile",
    ),
    (
        "the mode stops reaching the tar header, so every entry is 644 whatever was asked for",
        "internal/format/targz/size.go",
        "\t\tMode:     e.own.Mode,\n",
        "\t\tMode:     0o644,\n",
        "TestWhatAnArchiveSaysAboutAFileIsWhatAReaderFinds",
    ),
    (
        "the default mode stops being the one this format has always written",
        "internal/format/archive/ownership.go",
        "\town := Ownership{Mode: 0o644, Stated: Stated{Mode: \"644\", Owner: OwnerUnset}}\n",
        "\town := Ownership{Mode: 0o600, Stated: Stated{Mode: \"600\", Owner: OwnerUnset}}\n",
        "TestTheDefaultOwnershipIsWhatWasAlwaysWritten",
    ),
    (
        "a lock nothing implements is offered again, so the tool advertises what it fails on",
        "internal/format/archive/archive.go",
        "\t\tChoices: []string{AES128, AES192, AES256, NoEncryption, ZipCrypto},\n",
        '\t\tChoices: []string{AES128, AES192, AES256, NoEncryption, ZipCrypto, "aes-512"},\n',
        "TestEveryLockTheRegistryOffersCanActuallyBeWritten",
    ),
    (
        "padding goes back into the gzip comment, where Go stops reading at 511 bytes",
        "internal/format/targz/size.go",
        "\t\treturn label, true, padding - 2, true\n",
        '\t\treturn label + strings.Repeat(" ", int(padding)), false, 0, true\n',
        "TestEveryArchiveThisToolWritesCanBeReadByTheStandardLibrary",
    ),
    (
        "every locked entry declares the AES method, including the one that has to stay stored",
        "internal/format/archive/lock.go",
        "\tif l.keyLen() == 0 {\n\t\treturn zipStore\n\t}\n\treturn winZipAES\n",
        "\tif l.keyLen() == 0 && false {\n\t\treturn zipStore\n\t}\n\treturn winZipAES\n",
        "TestALockedEntryDeclaresTheShapeItsSchemeRequires",
    ),
    (
        "a container stops holding contains to the ceiling entries already has",
        "internal/format/archive/archive.go",
        "\tif asked > maxEntries {",
        "\tif false && asked > maxEntries {",
        "TestBothWaysOfAskingForEntriesShareOneCeiling",
    ),
    (
        "the plan memory ceiling stops refusing, so a plan grows until the machine says no",
        "internal/engine/planmemory.go",
        "\tif used <= p.ceiling {",
        "\tif true || used <= p.ceiling {",
        "TestAPlanTooBigToHoldIsRefusedWhileItIsBuilt",
    ),
    (
        "the plan ceiling stops taking readings, so nothing is ever measured",
        "internal/engine/planmemory.go",
        "\tif p.seen < p.nextAt {",
        "\tif true || p.seen < p.nextAt {",
        "TestAPlanTooBigToHoldIsRefusedWhileItIsBuilt",
    ),
    (
        "a run stops saying that its manifest will be too big for verify and cleanup to read",
        "internal/cli/record.go",
        "if !record.TooLarge {",
        "if true || !record.TooLarge {",
        "TestARunSaysWhenItsManifestWillBeTooBigToReadBack",
    ),
    (
        "the counted length of a manifest leaves out the line that closes its list of files",
        "internal/manifest/size.go",
        "between*int64(n-1) + closing",
        "between*int64(n-1)",
        "TestTheManifestSizeIsTheLengthOfTheManifest",
    ),
    (
        "the size-boundaries preset stops refusing a limit with no number above it",
        "internal/preset/limitset.go",
        "if s.limit > math.MaxInt64-far.bytes {",
        "if false && s.limit > math.MaxInt64-far.bytes {",
        "TestThePresetAnswersALimitTooLargeTheSameWayTheFlagDoes",
    ),
    (
        "the plan ceiling drops to a size an ordinary run cannot stay under",
        "internal/core/limits.go",
        "const MaxPlanBytes = 2 << 30",
        "const MaxPlanBytes = 2 << 10",
        "TestAnOrdinaryRunIsNotRefusedByThePlanCeiling",
    ),
    (
        "the reason given for a plan refusal names a ceiling other than the one in force",
        "internal/core/limits.go",
        "A(\"Ceiling\", Bytes(MaxPlanBytes)))",
        "A(\"Ceiling\", Bytes(MaxPlanBytes/2)))",
        "TestThePlanCeilingIsTheOneCoreStates",
    ),
    (
        "a container starts refusing exactly the number of entries it says it holds",
        "internal/format/archive/archive.go",
        "\tif asked > maxEntries {",
        "\tif asked >= maxEntries {",
        "TestTheEntryCeilingStillAcceptsTheNumberBelowIt",
    ),
    (
        "every run starts warning that its manifest is too big to read back",
        "internal/cli/record.go",
        "if !record.TooLarge {",
        "if !record.TooLarge && false {",
        "TestAnOrdinaryRunSaysNothingAboutTheManifestCeiling",
    ),
    (
        "free space too large to count wraps into a negative number again",
        "internal/core/diskspace.go",
        "\tif blocks > uint64(math.MaxInt64)/blockSize {",
        "\tif false && blocks > uint64(math.MaxInt64)/blockSize {",
        "TestFreeSpaceTooLargeToCountIsNotANegativeNumber",
    ),
    (
        "a block size the system never reported divides by nought instead of being answered",
        "internal/core/diskspace.go",
        "\tif blockSize == 0 {",
        "\tif false && blockSize == 0 {",
        "TestFreeSpaceTooLargeToCountIsNotANegativeNumber",
    ),
    (
        "a filler with nothing to add goes back to spinning without end",
        "internal/core/records.go",
        "\tif !wordsHaveBytes(words) {",
        "\tif false && !wordsHaveBytes(words) {",
        "TestAFillerWithNothingToAddRefusesRatherThanSpinning",
    ),
    (
        "a filler refuses every vocabulary, including the ones that pad perfectly well",
        "internal/core/records.go",
        "\tif !wordsHaveBytes(words) {",
        "\tif !wordsHaveBytes(words) || true {",
        "TestAFillerWithOneRealWordAmongEmptyOnesStillPadsToTheByte",
    ),
    (
        "the ceiling a caller does not name stops being the real one, so ordinary short runs are refused",
        "internal/engine/planmemory.go",
        "\t\tceiling = core.MaxPlanBytes",
        "\t\tceiling = 1",
        "TestAShortOrdinaryRunIsNotRefusedByThePlanCeiling",
    ),
    (
        "the plan ceiling goes back to ignoring the first sixty three files of a run",
        "internal/engine/planmemory.go",
        "nextAt:         1,",
        "nextAt:         64,",
        "TestARunTooShortToHaveBeenWeighedIsWeighed",
    ),
    (
        "a crash in a generator stops being one file's failure and takes the run",
        "internal/engine/crash.go",
        "err = &GeneratorCrashError{Format: f.Desc.ID, While: crashWriting, Name: f.Name, Value: v}",
        "_ = v",
        "TestAGeneratorThatCrashesWhileWritingCostsOneFile",
    ),
    (
        "the engine calls a generator directly again, so a crash while planning ends the process",
        "internal/engine/plantarget.go",
        "planWithoutCrashing(ctx, desc, format.Request{",
        "desc.Generator.Plan(format.Request{",
        "TestEveryCallIntoAGeneratorGoesThroughTheCrashWrapper",
    ),
    (
        "zip stops refusing an archive whose size its own arithmetic cannot predict",
        "internal/format/zip/zip.go",
        "\tif total <= zip32Ceiling {",
        "\tif true || total <= zip32Ceiling {",
        "TestAContainerRefusesASizeItsArithmeticCannotSee",
    ),
    (
        "an Office package stops refusing a size past what a zip32 archive describes",
        "internal/format/opc/opc.go",
        "\tif want > Zip32Ceiling {",
        "\tif false && want > Zip32Ceiling {",
        "TestAContainerRefusesASizeItsArithmeticCannotSee",
    ),
    (
        "the forecast of the record and the preflight both stop noticing that they were asked to stop",
        "internal/engine/engine.go",
        "record, err := opt.forecast(ctx, files)\n\tif err != nil {\n\t\treturn res, err\n\t}\n\tres.Record = record\n\tif err := preflight(ctx, files, opt, record); err != nil {",
        "record, err := opt.forecast(context.Background(), files)\n\tif err != nil {\n\t\treturn res, err\n\t}\n\tres.Record = record\n\tif err := preflight(context.Background(), files, opt, record); err != nil {",
        "TestWorkingOutTheCostCanBeStopped",
    ),
    (
        "a cleanup report goes back to counting only blocked files as kept",
        "internal/cli/cleanup.go",
        "\treport.Removed, report.Kept = removed, len(outcomes)-removed",
        "\treport.Removed, report.Kept = removed, blocked",
        "TestACleanupReportAddsUp",
    ),
    (
        "a boundary that could not be resolved answers that everything stays inside it",
        "internal/core/filename.go",
        "\tif !b.known {\n\t\treturn true\n\t}",
        "\tif !b.known {\n\t\treturn false\n\t}",
        "TestABoundaryThatCannotTellRefuses",
    ),
    (
        "a boundary starts calling its own files escapes, so every ordinary run is refused",
        "internal/core/filename.go",
        "!crossesUnresolvedLink(b.resolved, rel) {\n\t\treturn false\n\t}",
        "!crossesUnresolvedLink(b.resolved, rel) {\n\t\treturn true\n\t}",
        "TestABoundaryThatKnowsStillLetsItsOwnFilesThrough",
    ),
    (
        "a run whose manifest could not be saved stops saying that it left files nothing can remove",
        "internal/cli/record.go",
        "\t\tif n := len(res.Manifest.Files); n > 0 {",
        "\t\tif n := len(res.Manifest.Files); false && n > 0 {",
        "TestARunThatCannotSaveItsManifestSaysWhatItLeftBehind",
    ),
    (
        "a package that exists for the tests is linked into the shipped binaries",
        "internal/cli/cli.go",
        "\t_ \"github.com/donislawdev/TestingFilesGenerator/internal/format/all\"",
        "\t_ \"github.com/donislawdev/TestingFilesGenerator/internal/format/all\"\n\t_ \"github.com/donislawdev/TestingFilesGenerator/internal/site\"",
        "TestThePackagesOnlyTestsUseStayOutOfBothBinaries",
    ),
    (
        "where the manifest lands stops falling back to the default name, so a caller that leaves it blank aims at the output directory itself",
        "internal/engine/preflight.go",
        "func manifestNameOf(opt Options) string {\n\tif opt.ManifestName == \"\" {",
        "func manifestNameOf(opt Options) string {\n\tif false && opt.ManifestName == \"\" {",
        "TestWhereTheManifestLandsIsOneAnswer",
    ),
    (
        "a preview marks the screen busy without marking it stoppable, so work that can be cancelled offers no way to cancel it",
        "internal/gui/window/run.go",
        "\tr.busy.set(true, busyFace{stoppable: true})\n\tr.say(text.WorkingOutTheCost())",
        "\tr.busy.set(true, busyFace{})\n\tr.say(text.WorkingOutTheCost())",
        "TestAPreviewOffersTheWayOut",
    ),
    (
        "a format declares a minimum nobody could ever meet, quietly, so it refuses every size ever asked of it",
        "internal/format/png/png.go",
        "\treturn body + iendSize\n}",
        "\treturn body + 1<<62\n}",
        "TestNoFormatDeclaresAMinimumNobodyCouldMeet",
    ),
    (
        "the manifest stops saying which Go built the run, so a hash that moved after a rebuild cannot be told from one that moved because the recipe changed",
        "internal/manifest/manifest.go",
        "\t\t\tGo:         runtime.Version(),",
        "\t\t\tGo:         runtime.Version()[:0],",
        "TestTheManifestNamesTheToolchainThatBuiltTheRun",
    ),
    (
        "the manifest entry stops naming the target the file came from, so counting files per target means parsing names again",
        "internal/engine/engine.go",
        "\t\tTargetID:      f.Target.ID,",
        "\t\tTargetID:      f.Target.Group,",
        "TestTheManifestSaysWhichTargetEachFileCameFrom",
    ),
    (
        "the summary stops breaking a run down by target, so the cheap answer and the entries disagree",
        "internal/manifest/manifest.go",
        "\tif f.TargetID != \"\" {",
        "\tif false && f.TargetID != \"\" {",
        "TestTheManifestSaysWhichTargetEachFileCameFrom",
    ),
    (
        "a generator allocates its write buffer once per chunk instead of once per file, which leaves the bytes identical",
        "internal/format/txt/txt.go",
        "\tbuf := make([]byte, 0, chunkSize+16)\n\tcol := 0\n\tfor remaining > 0 {",
        "\tcol := 0\n\tfor remaining > 0 {\n\t\tbuf := make([]byte, 0, chunkSize+16)",
        "TestNoGeneratorHoldsTheWholeFileInMemory",
    ),
    (
        "the recipe reader takes the bytes of a node instead of the node, so goccy renders every value through its document formatter",
        "internal/recipe/scalar.go",
        "func (s *scalar) UnmarshalYAML(n ast.Node) error {\n\tb := []byte(n.String())",
        "func (s *scalar) UnmarshalYAML(b []byte) error {\n\tvar _ ast.Node",
        "TestReadingARecipeDoesNotCostTheSquareOfItsSize",
    ),
    (
        "verify stops telling one of our own files under another spelling apart from somebody else's file",
        "internal/audit/audit.go",
        "return Respelled, claimedAs",
        "return Extra, claimedAs",
        "TestOneFileUnderTwoSpellingsIsNamedAndNotDeleted",
    ),
    (
        "cleanup stops checking how the directory really spells a name, so a filesystem that ignores case lets it delete a name the manifest does not list",
        "internal/audit/cleanup.go",
        "\t\tif actual, known := stored.differing(full); known {",
        "\t\tif actual, known := stored.differing(full); known && false {",
        "TestOneFileUnderTwoSpellingsIsNamedAndNotDeleted",
    ),
    (
        "the sitemap declares a second namespace again, which is what makes a strict reader go looking for a schema it has not got",
        "internal/site/render.go",
        'schemas/sitemap/0.9">',
        'schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">',
        "TestTheSitemapNeedsNoSchemaButItsOwn",
    ),
    (
        "the sitemap names pages by path rather than by address, so it points a crawler at nothing",
        "internal/site/render.go",
        "s.Origin(), pageURL(lang, page))",
        '"", pageURL(lang, page))',
        "TestTheSitemapNeedsNoSchemaButItsOwn",
    ),
    (
        "a menu with a declared default opens on nothing, so a fresh screen shows a blank where the format has an answer",
        "internal/gui/parts/property.go",
        "sel.SetSelected(p.Default)",
        'sel.SetSelected("")',
        "TestAMenuOpensOnItsDeclaredDefault",
    ),
    (
        "a menu of every format draws no file kind pictures, so one list of formats looks unlike the others",
        "internal/gui/parts/ring.go",
        "if IsEveryFormat(options) {",
        "if IsEveryFormat(options) && false {",
        "TestEveryMenuOfferingEveryFormatDrawsTheKindPictures",
    ),
    (
        # Aimed at the row rather than at the box since 2026-09-01. The box
        # half - turning off the widening in ring.go - stopped reddening this
        # guard, and measuring says the branch is unreachable for every menu
        # this window has today: the same tightest slot is reported with it on
        # and off. That is a question about a defence nobody can turn red,
        # recorded as an observation rather than answered here.
        #
        # This one breaks what the guard names instead: the room a row gives
        # its words. Four values come out cut off and the guard names each.
        "a row gives its words less room than it promised, so values are cut off in the open list",
        "internal/gui/parts/listrow.go",
        "room := size.Width - left - right\n",
        "room := size.Width - left - right - rowGutter*3\n",
        "TestEveryValueInAMenuFitsInTheListItOpens",
    ),
    (
        # O220 put back: the tick in front of the words and its column kept
        # whether or not anything is ticked, so the words of every list stand
        # 36 px right of the word in the box. The guard reads the positions
        # off the canvas, box against row.
        "the tick goes back in front of the words and pushes them off the box's word",
        "internal/gui/parts/listrow.go",
        "\tr.tick.Move(fyne.NewPos(size.Width-rowGutter-icon, (size.Height-icon)/2))\n"
        "\tright += icon + rowGap\n",
        "\tr.tick.Move(fyne.NewPos(rowGutter, (size.Height-icon)/2))\n"
        "\tleft += icon + rowGap\n",
        "TestTheWordsInAnOpenListStartWhereTheWordInTheBoxDoes",
    ),
    (
        # O223 put back: the switch goes on the form and into no list, so
        # Freeze never reaches it and a press on another way of stating a
        # size rebuilds the boxes under a form drawn as frozen. Measured on
        # a render of the batch screen mid run before the fix.
        "the unlabelled switch is put on the form and registered nowhere",
        "internal/gui/parts/fields.go",
        "\t\ts.bare = append(s.bare, bareControl{control: d, after: len(s.list)})",
        "\t\t_ = d",
        "TestTheSizeWaySwitchFreezesWithTheRestOfTheForm",
    ),
    (
        # The other half of O223: a frozen switch drops the chosen fill, so
        # a form frozen for a run no longer says which way it is running with.
        "a frozen switch forgets which way is chosen",
        "internal/gui/parts/segments.go",
        "\t\tif chosen {\n\t\t\tfill = PaletteColour(theme.ColorNameSelection, dark)\n\t\t}\n\tcase chosen:",
        "\tcase chosen:",
        "TestAFrozenSwitchStillShowsWhichWayIsChosen",
    ),
    (
        # O219 put back, first geometry: the fill laid at the switch's own
        # edge, under the border's stroke, so its corner stands out past the
        # border's arc.
        "the chosen fill reaches under the border again",
        "internal/gui/parts/segments.go",
        "\t\tr.fills[i].Move(fyne.NewPos(x+edgeWidth, edgeWidth))",
        "\t\tr.fills[i].Move(fyne.NewPos(x, 0))",
        "TestTheChosenSegmentStaysInsideTheBorderAndNoRuleTouchesIt",
    ),
    (
        # O219 put back, second geometry: a hairline against the chosen
        # segment's edge.
        "a rule is drawn against the chosen segment",
        "internal/gui/parts/segments.go",
        "\t\tif chosen != i && chosen != i+1 {",
        "\t\tif true {",
        "TestTheChosenSegmentStaysInsideTheBorderAndNoRuleTouchesIt",
    ),
    (
        "changing the format hides the archive contents somebody typed, so a refusal names a box nobody can see",
        "internal/gui/window/recipe.go",
        "if len(b.contents) == 0 {",
        "if len(b.contents) == 0 || !holds {",
        "TestContentsAlreadyTypedStayOnScreenWhenTheFormatChanges",
    ),
    (
        "the format menu stops drawing the kind of each format",
        "internal/gui/parts/filekind.go",
        "func KindOfFile(id string) fyne.Resource {\n\tswitch fileKinds[id] {",
        "func KindOfFile(id string) fyne.Resource {\n\tswitch kindUnknown {",
        "TestTheFormatMenuDrawsThePictureOfEachKind",
    ),
    (
        "a menu reorders the choices the declaration gave it",
        "internal/gui/parts/property.go",
        "NewChooser(p.Choices, nil)",
        "NewChooser(append([]string{p.Default}, p.Choices...), nil)",
        "TestAMenuOffersItsChoicesInTheDeclaredOrder",
    ),
    (
        "a batch of any format offers to put files inside it, which leads nowhere but a refusal",
        "internal/gui/window/recipe.go",
        "holds = d.Container != nil",
        "holds = d.Container != nil || true",
        "TestOnlyAFormatThatHoldsFilesOffersToPutFilesInIt",
    ),
    (
        "the depth metric stops counting a closure as a level, so the ceiling "
        "standing on it measures something shallower than what a person reads "
        "indented - and nothing else in the tree would notice, because a "
        "metric that reports LESS depth never trips the ceiling",
        "internal/guard/codeshape_test.go",
        "\t\tcase *ast.FuncLit:\n\t\t\twalk(node.Body, depth+1)",
        "\t\tcase *ast.FuncLit:\n\t\t\twalk(node.Body, depth)",
        "TestTheNestingMetricMeasuresWhatAPersonSeesIndented",
    ),
    (
        "a linter is dropped from the configuration to turn a red build green, "
        "and the gate that vanished cannot fail to announce itself",
        ".golangci.yml",
        "    - errorlint\n",
        "",
        "TestTheLinterSetOnlyEverGrows",
    ),
    (
        "the linter set goes back to inheriting whatever the tool ships with, "
        "so the written down list stops describing the whole set",
        ".golangci.yml",
        "  default: none\n",
        "  default: standard\n",
        "TestTheLinterSetOnlyEverGrows",
    ),
    (
        "staticcheck is switched on under golangci-lint as well, which quietly "
        "swaps a gate that honours //lint:ignore for one that does not",
        ".golangci.yml",
        "    - ineffassign\n",
        "    - ineffassign\n    - staticcheck\n",
        "TestTheLinterSetOnlyEverGrows",
    ),
    (
        "CI stops verifying the linter configuration, so a key the schema "
        "refuses is ignored in silence",
        ".github/workflows/ci.yml",
        "golangci-lint@v2.15.0 config verify",
        "golangci-lint@v2.15.0 version",
        "TestTheWorkflowRunsTheLinterAndVerifiesItsConfiguration",
    ),
    (
        "CI reads the linter configuration and never runs it against the tree",
        ".github/workflows/ci.yml",
        'golangci-lint@v2.15.0 run --build-tags "$(cat .github/build-tags)" ./...',
        "golangci-lint@v2.15.0 help",
        "TestTheWorkflowRunsTheLinterAndVerifiesItsConfiguration",
    ),
    (
        "the function ceiling is left one line above the longest function, the "
        "same silent headroom one axis over",
        "internal/guard/codeshape_test.go",
        "\tlongestFunction = 75\n",
        "\tlongestFunction = 76\n",
        "TestTheCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the crowd cap is left one above the count, so the next function to "
        "reach the band arrives without a word",
        "internal/guard/crowding_test.go",
        "\tcrowdedFunctions = 9\n",
        "\tcrowdedFunctions = 10\n",
        "TestTheCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the crowding band stops being a number of lines and goes back to a "
        "share of the ceiling, so lowering a ceiling reshapes the band under it",
        "internal/guard/crowding_test.go",
        "func crowding(n, band int) bool {\n\treturn n >= band\n}",
        "func crowding(n, band int) bool {\n\treturn n*100 >= band*75\n}",
        "TestTheCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the ceiling on guard files is left one line above the biggest one, so "
        "the half of the tree that grows fastest gets headroom nobody granted",
        "internal/guard/testshape_test.go",
        "\tlongestTestFile     = 726\n",
        "\tlongestTestFile     = 727\n",
        "TestTheGuardCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the depth ceiling for guards is left above the deepest one",
        "internal/guard/testshape_test.go",
        "\tdeepestTestNesting  = 7\n",
        "\tdeepestTestNesting  = 8\n",
        "TestTheGuardCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the crowd cap for guards is left one above the count, so the next long "
        "guard joins the band without a word",
        "internal/guard/testshape_test.go",
        "\tcrowdedTestFunctions = 1\n",
        "\tcrowdedTestFunctions = 2\n",
        "TestTheGuardCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the scan over guard files reads nothing, which satisfies every ceiling "
        "ever set and looks exactly like a guard that works",
        "internal/guard/testshape_test.go",
        "\t\tfor _, path := range p.tests {",
        "\t\tfor _, path := range p.tests[:0] {",
        "TestNoGuardHasGrownPastWhatAPersonCanFollow",
    ),
    (
        "the method ceiling is left one above the widest type, which grants "
        "headroom nobody decided to grant on the axis a file split cannot answer",
        "internal/guard/typeshape_test.go",
        "\tmostMethods = 25\n",
        "\tmostMethods = 26\n",
        "TestTheTypeCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the field ceiling is left one above the widest type, so a type may take "
        "on one more piece of state than anybody agreed to",
        "internal/guard/typeshape_test.go",
        "\tmostFields  = 21\n",
        "\tmostFields  = 22\n",
        "TestTheTypeCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the cap on types crowding the method ceiling is left one above the "
        "count, so the next type to reach the band arrives without a word",
        "internal/guard/typeshape_test.go",
        "\tcrowdedMethodTypes = 3\n",
        "\tcrowdedMethodTypes = 4\n",
        "TestTheTypeCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the cap on types crowding the field ceiling is left one above the count",
        "internal/guard/typeshape_test.go",
        "\tcrowdedFieldTypes  = 4\n",
        "\tcrowdedFieldTypes  = 5\n",
        "TestTheTypeCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the type scan reads nothing, which satisfies both ceilings perfectly "
        "and looks exactly like a scan that works",
        "internal/guard/typeshape_test.go",
        "\t\tfor _, path := range p.files {",
        "\t\tfor _, path := range p.files[:0] {",
        "TestNoTypeHasGrownPastWhatAPersonCanHoldInTheirHead",
    ),
    (
        "the method band drops far enough that ordinary types are inside it, "
        "which is what the cap on the band exists to notice",
        "internal/guard/typeshape_test.go",
        "\tcrowdingMethods = 21\n",
        "\tcrowdingMethods = 11\n",
        "TestNoSecondTypeIsCreepingUpOnTheTypeCeilings",
    ),
    (
        "the field band drops far enough that ordinary types are inside it",
        "internal/guard/typeshape_test.go",
        "\tcrowdingFields  = 17\n",
        "\tcrowdingFields  = 9\n",
        "TestNoSecondTypeIsCreepingUpOnTheTypeCeilings",
    ),
    (
        "a debug line is left in a library below the surface, printed with the "
        "builtin that needs no import - it fails no run, trips no exit code and "
        "turns a --json report into text no parser accepts",
        "internal/core/size.go",
        "func ParseSize(s string) (int64, error) {\n\traw := strings.TrimSpace(s)",
        "func ParseSize(s string) (int64, error) {\n\tprintln(\"parsing\", s)\n\traw := strings.TrimSpace(s)",
        "TestNothingBelowASurfaceReachesTheConsoleDirectly",
    ),
    (
        "the engine says something on stdout on the SUCCESS path, where the "
        "guard that a failed run prints nothing cannot see it",
        "internal/engine/engine.go",
        "func Run(ctx context.Context, files []PlannedFile, opt Options) (*Result, error) {\n\tm := newManifest(opt)",
        "func Run(ctx context.Context, files []PlannedFile, opt Options) (*Result, error) {\n\tfmt.Println(\"running\")\n\tm := newManifest(opt)",
        "TestNothingBelowASurfaceReachesTheConsoleDirectly",
    ),
    (
        "the console scan reads no packages at all, which finds no offender and "
        "looks exactly like a scan that works",
        "internal/guard/consolereach_test.go",
        "\tfor _, p := range packages(t) {",
        "\tfor _, p := range packages(t)[:0] {",
        "TestNothingBelowASurfaceReachesTheConsoleDirectly",
    ),
    (
        "the support address comes off the About screen, which turns the "
        "considered silence in OpenLink back into a dead end - the button does "
        "nothing and there is nowhere left to read the address",
        "internal/gui/window/about.go",
        # Repointed 2026-09-23, when Donate moved into this card, and on
        # 2026-10-14 into linkCard, which draws this card and Source code.
        "\t\tparts.Prose(address))",
        "\t\t)",
        "TestTheSupportAddressIsReadableOnAScreen",
    ),
    (
        "the support address is folded into the sentence beside it, so it is on "
        "the screen and no longer a line somebody can select in one gesture",
        "internal/gui/window/about.go",
        # Repointed 2026-09-23, when Donate moved into this card, and on
        # 2026-10-14 into linkCard, which draws this card and Source code.
        "\t\tparts.Prose(address))",
        "\t\tparts.Prose(detail+\" \"+address))",
        "TestTheSupportAddressStandsOnItsOwnLine",
    ),
    (
        "the branching ceiling is left above the most branching function, so a "
        "function can climb toward it without anything saying so",
        "internal/guard/branching_test.go",
        "\tworstComplexity = 22 ",
        "\tworstComplexity = 23 ",
        "TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the argument ceiling is left above the widest signature",
        "internal/guard/branching_test.go",
        "\tmostArguments = 7 ",
        "\tmostArguments = 8 ",
        "TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the depth crowd count is left above the measurement, which is the one "
        "that matters most here - a function sits one level under a ceiling "
        "that was already exact, and no guard could see it",
        "internal/guard/branching_test.go",
        "\tcrowdedDepthFunctions = 49\n",
        "\tcrowdedDepthFunctions = 50\n",
        "TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the branching count stops counting short circuit operators, so a "
        "function full of && reads as simple",
        "internal/guard/ratchet_test.go",
        "\t\t\tif v.Op.String() == \"&&\" || v.Op.String() == \"||\" {",
        "\t\t\tif v.Op.String() == \"&&\" && v.Op.String() == \"||\" {",
        "TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "counting arguments stops counting the names sharing one type, so "
        "func(a, b, c int) reads as one argument",
        "internal/guard/ratchet_test.go",
        "\t\tn += len(f.Names)",
        "\t\tn++",
        "TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber",
    ),
    (
        "the cap on branching functions is pushed BELOW what the tree measures, "
        "which is how this guard reports a tree that has crossed it",
        "internal/guard/branching_test.go",
        "\tcrowdedComplexity = 4\n",
        "\tcrowdedComplexity = 3\n",
        "TestNothingBranchesOrTakesMoreThanWhatAPersonCanFollow",
    ),
    (
        "the cap on wide signatures is pushed below the measurement, one axis over",
        "internal/guard/branching_test.go",
        "\tcrowdedArguments = 3\n",
        "\tcrowdedArguments = 2\n",
        "TestNothingBranchesOrTakesMoreThanWhatAPersonCanFollow",
    ),
    (
        "an action goes back to a tag somebody else can repoint, which is the "
        "supply chain question these workflows answer nowhere else",
        ".github/workflows/pages.yml",
        "actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346",
        "actions/deploy-pages@v5",
        "TestEveryActionIsPinnedToACommitRatherThanATag",
    ),
    (
        "the rule stops asking whether a reference is hexadecimal, so forty "
        "characters of anything would read as a commit",
        "internal/guard/actionpinning_test.go",
        "if !digit && !lower {",
        "if !digit && !lower && false {",
        "TestEveryActionIsPinnedToACommitRatherThanATag",
    ),
    (
        "a licence GitHub could not determine passes, which is exactly the half "
        "the action already declines to fail on",
        ".github/scripts/dependency_gate.py",
        'return "unknown", licence',
        'return "ok", licence',
        "TestTheDependencyGateRefusesALicenceThisProjectCannotDistribute",
    ),
    (
        "the vulnerability half is loosened so only a critical one stops a pull request",
        ".github/workflows/dependency-review.yml",
        "fail-on-severity: moderate",
        "fail-on-severity: critical",
        "TestCIAsksTheDependencyGateOnEveryPullRequest",
    ),
    (
        "a gate that never read its answer reports that it found no problems",
        ".github/scripts/dependency_gate.py",
        '(args.review, exc), file=sys.stderr)\n        return 2',
        '(args.review, exc), file=sys.stderr)\n        return 0',
        "TestTheDependencyGateFailsClosedWhenItCannotReadTheAnswer",
    ),
    (
        "the gate blocks only what the scanner's own severity flag can express, "
        "so a registry rule marked HIGH or CRITICAL passes unseen",
        ".github/scripts/semgrep_gate.py",
        'BLOCKING = ("ERROR", "HIGH", "CRITICAL")',
        'BLOCKING = ("ERROR",)',
        "TestTheSemgrepGateBlocksTheSeveritiesTheFlagCannotExpress",
    ),
    (
        "a rule that failed to run counts as a rule that found nothing",
        ".github/scripts/semgrep_gate.py",
        'str(e.get("level", "")).lower() == "error"',
        'str(e.get("level", "")).lower() != "error"',
        "TestASemgrepScanErrorIsNotACleanScan",
    ),
    (
        "a scan that read no file at all reports a clean tree",
        ".github/scripts/semgrep_gate.py",
        "if not scanned:",
        "if not scanned and False:",
        "TestTheSemgrepGateFailsClosedWhenTheScanDidNotHappen",
    ),
    (
        "the scanner starts sending telemetry about this repository's own source",
        ".github/workflows/ci.yml",
        "--metrics=off",
        "--metrics=on",
        "TestCIRunsSemgrepAndDecidesFromTheReportRatherThanTheExitCode",
    ),
    (
        "the publish job stops asking whose repository the run came from, so a fork's "
        "pull request can reach the job that deploys the website",
        ".github/workflows/pages.yml",
        "github.event.workflow_run.head_repository.full_name == github.repository",
        "github.event.workflow_run.head_branch == 'main'",
        "TestTheSiteIsPublishedOnlyForAPushToThisRepository",
    ),
    (
        "the push trigger loses its branch filter, so every branch push runs the whole "
        "suite a second time beside the pull request that already runs it",
        ".github/workflows/ci.yml",
        "  push:\n    branches: [main]\n",
        "  push:\n",
        "TestTheSuiteRunsOncePerChange",
    ),
    (
        "the pull request trigger is dropped, which stops the doubling by leaving a "
        "fork's pull request unchecked and the required contexts never reporting",
        ".github/workflows/ci.yml",
        "    branches: [main]\n  pull_request:\n",
        "    branches: [main]\n",
        "TestTheSuiteRunsOncePerChange",
    ),
    (
        "the race detector goes back to asking a push-only field, so every pull request "
        "counts as touched and it runs on all of them again",
        ".github/workflows/ci.yml",
        'before="${{ github.event.pull_request.base.sha || github.event.before }}"',
        'before="${{ github.event.before }}"',
        "TestTheRaceDetectorAsksAboutWhatThePullRequestChanges",
    ),
    (
        "the private reporting link still names the project these files were copied "
        "from, so a vulnerability report goes to somebody else's repository",
        ".github/ISSUE_TEMPLATE/config.yml",
        "url: https://github.com/donislawdev/TestingFilesGenerator/security/advisories/new",
        "url: https://github.com/donislawdev/BeanNetworkTester/security/advisories/new",
        "TestNothingThatDirectsAPersonPointsAtAnotherProject",
    ),
    (
        "two fields of one issue form share an id, which makes GitHub refuse the whole form",
        ".github/ISSUE_TEMPLATE/question.yml",
        "    id: question\n",
        "    id: extra\n",
        "TestTheIssueFormsAreShapedTheWayGitHubNeeds",
    ),
    (
        "the dependency review takes back a write scope on pull requests, for a comment "
        "that a red check already makes",
        ".github/workflows/dependency-review.yml",
        "  contents: read\n",
        "  contents: read\n  pull-requests: write\n",
        "TestTheDependencyReviewAsksForNoWriteScope",
    ),
    (
        # The defect O150 records, put back: a file ships inside somebody
        # else's module, under a licence of its own, and no list here knows
        # about it. Seven fonts and ninety-seven images did exactly that for as
        # long as the window existed, and every check passed the whole time,
        # because every check asked about modules.
        "a font ships with nothing in the registry accounting for it",
        "internal/legal/assets.go",
        '"font/EmojiOneColor.otf"',
        '"font/EmojiOneColour.otf"',
        "TestEveryFileEmbeddedFromSomebodyElseIsAccountedFor",
    ),
    (
        # The registry is code and the notices are what travels with a
        # download. A name in one and not the other means the person holding
        # the binary cannot read what they were given.
        "the notices stop naming something the registry says we ship",
        "THIRD-PARTY-NOTICES.md",
        "| Fyne shaders, 14 programs that draw the screen |",
        "| Shaders, 14 programs that draw the screen |",
        "TestEveryEmbeddedAssetIsNamedInTheNotices",
    ),
    (
        # A module arrives and nobody reads its licence. The notices guard
        # catches the same arrival against the document. This one catches it
        # against the list an SBOM will be generated from, which is the copy a
        # machine reads.
        "a linked module has no entry in the licence registry",
        "internal/legal/modules.go",
        '{Path: "github.com/yuin/goldmark", SPDX: "MIT"',
        '{Path: "github.com/yuin/goldmark-gone", SPDX: "MIT"',
        "TestEveryModuleTheBinariesLinkHasAReviewedLicence",
    ),
    (
        # An empty field reads like an answer. One module here genuinely has no
        # copyright line and says so in a note, so a blank has to be the
        # deliberate kind rather than the forgotten kind.
        "an entry loses its copyright line and says nothing about why",
        "internal/legal/modules.go",
        'SPDX: "MIT", Copyright: "(c) 2019 Yusuke Inuzuka"',
        'SPDX: "MIT", Copyright: ""',
        "TestNoEntryInTheRegistryIsSilentAboutItsLicence",
    ),
    (
        # Two written copies of one fact are safe only while something compares
        # them. Without this the code could publish one licence in an SBOM
        # while the notices in the same archive stated another.
        "the registry and the notices disagree about one licence",
        "internal/legal/modules.go",
        '{Path: "github.com/mattn/go-runewidth", SPDX: "MIT"',
        '{Path: "github.com/mattn/go-runewidth", SPDX: "MIT-0"',
        "TestTheRegistryAndTheNoticesAgreeOnEveryLicence",
    ),
    (
        # The real defect found on 2026-08-28, put back. The notices said
        # golang.org/x/text was 0.40.0 while every binary linked 0.41.0, and
        # the guard was green because it compared table rows only - and the two
        # modules of the command line binary are named by a section instead. A
        # section carries the whole licence text, so it was the stronger notice
        # with the weaker check.
        "a module's own section states a version nobody builds",
        "THIRD-PARTY-NOTICES.md",
        # The number here follows the build: 0.40 became 0.41 on 2026-08-28
        # and 0.42 on 2026-09-22 (#112), 0.43 on 2026-10-19, and each time this
        # pattern went stale with it - a SKIP that reads like a proof, which
        # staleness.py names.
        "Version 0.43.0. Supplies Unicode normalisation",
        "Version 0.42.0. Supplies Unicode normalisation",
        "TestTheNoticesFileNamesTheVersionsThatAreActuallyBuilt",
    ),
    (
        # The window stops showing the list while the command still prints it.
        # Two surfaces, two different answers about what the program is made
        # of - and which one a person reads depends on which file they
        # downloaded.
        "the about screen stops naming what the window carries",
        "internal/gui/window/about.go",
        "items := legal.Reviewed()",
        "items := legal.Reviewed()[:0]",
        "TestTheWindowShowsTheSameLicencesAsTheCommandDoes",
    ),
    (
        # An asset entry names a module that does not bring it. Nothing breaks
        # loudly: the entry simply stops matching the build, so the font drops
        # off the list the command prints and nobody is told.
        "an asset entry names the wrong module",
        "internal/legal/assets.go",
        "\t\tPackage:   \"fyne.io/fyne/v2/lang\",\n\t\tModule:    \"fyne.io/fyne/v2\",",
        "\t\tPackage:   \"fyne.io/fyne/v2/lang\",\n\t\tModule:    \"fyne.io/fyne/v3\",",
        "TestEveryAssetEntryNamesTheModuleItComesFrom",
    ),
    (
        # The command goes back to pointing at a file instead of answering.
        # That was the state until 2026-08-28, and it is useless to the person
        # the notice is for: somebody holding one downloaded binary has no
        # THIRD-PARTY-NOTICES.md anywhere near it.
        #
        # Sliced to nothing rather than replaced by nil, because nil leaves the
        # import unused and a mutation that stops the tree compiling proves
        # nothing. It went BROKEN exactly that way on the first attempt.
        "the licence command stops listing what the binary carries",
        "internal/cli/commands.go",
        "printCarried(out, legal.CarriedHere())",
        "printCarried(out, legal.CarriedHere()[:0])",
        "TestTheLicenceCommandListsWhatTheBinaryLinks",
    ),
    (
        # The document goes back to describing modules only - which is what
        # every list in this project did until 2026-08-28, and why seven fonts
        # and ninety-seven drawings shipped named nowhere.
        "the bill of materials stops naming the files a module brings",
        "internal/legal/spdx.go",
        "\treturn append(items, embeddedItems(linkedSet(binary.Modules), binary.Packages)...)",
        "\treturn items",
        "TestTheSBOMNamesEverythingTheBinariesCarry",
    ),
    (
        # Everything is attached to everything: the document says the command
        # line binary carries fonts, which it does not - it has no toolkit in
        # it at all. A bill of materials that describes a file nobody ships is
        # worse than none, because a compliance process acts on it.
        "the document says every binary carries every font",
        "internal/legal/carried.go",
        "\treturn linkedModules[asset.Module]\n}",
        "\treturn true\n}",
        "TestTheSBOMKeepsTheTwoBinariesApart",
    ),
    (
        # A field SPDX requires goes empty. Nothing fails loudly - the document
        # still parses as JSON, and a reader takes the gap for an answer.
        "the document loses a field SPDX requires",
        "internal/legal/spdx.go",
        "\t\tDataLicense:       \"CC0-1.0\",",
        "\t\tDataLicense:       \"\",",
        "TestTheSBOMCarriesTheFieldsSPDXRequires",
    ),
    (
        # The order stops being settled, so two renderings of one build differ.
        # A release attaches this file and an attestation is made about its
        # bytes, so a document that moves on its own is one nobody can verify
        # against anything.
        "the order of components stops being settled",
        "internal/legal/spdx.go",
        "\tsort.Strings(paths)",
        "\tsort.Strings(paths[:0])",
        "TestTheSBOMIsTheSameBytesForTheSameInputs",
    ),
    (
        # The gate finds an unaccounted component and passes anyway. This is
        # the exact shape of a gate that reads like a gate and is not one.
        "the bill of materials gate stops refusing what it found",
        ".github/scripts/sbom_gate.py",
        "    if unaccounted:",
        "    if unaccounted and False:",
        "TestTheBillOfMaterialsGateRefusesWhatTheRegistryDoesNotKnow",
    ),
    (
        # A scanner that fell over is treated as a clean tree. Measured in this
        # repository with a different scanner: four errors, exit code zero and
        # a file that was not JSON.
        "an empty scan is treated as a clean scan",
        ".github/scripts/sbom_gate.py",
        "        return unusable(\"the scan names no artifact at all, so it scanned nothing\")",
        "        return 0",
        "TestAnUnusableScanIsNotACleanScan",
    ),
    (
        # The workflow stops running the gate. The script stays in the tree, the
        # guards over it stay green, and nothing asks the question on any change
        # - which is how a probe in this project sat unrun for a day.
        "the workflow stops running the bill of materials gate",
        ".github/workflows/ci.yml",
        "        run: python .github/scripts/sbom_gate.py scan.json ours.spdx.json",
        "        run: echo the gate is not run here",
        "TestTheWorkflowScansBothBinariesAndReadsTheReport",
    ),
    (
        # The permission goes away and nothing fails loudly: the step cannot
        # mint a token, the release goes out, and every other job stays green -
        # while what it published carries no statement anybody can check.
        "the release loses the permission an attestation needs",
        ".github/workflows/release.yml",
        "      id-token: write",
        "      id-token: read",
        "TestTheReleaseAsksForWhatAnAttestationNeeds",
    ),
    (
        # The notes stop telling anybody how to check. A statement nobody knows
        # about is a statement nobody checks, which is the same as not making
        # it - except that it looks like diligence.
        "the release notes stop saying how to check a download",
        ".github/workflows/release.yml",
        "            echo \"gh attestation verify <the file you downloaded> -R ${GITHUB_REPOSITORY}\"",
        "            echo \"nothing to check here\"",
        "TestTheReleaseNotesSayHowToCheckWhatWasDownloaded",
    ),
    (
        # The statement stops being about a named list and becomes about
        # whatever is in a directory, which is how a statement ends up covering
        # files nobody published and missing ones they did.
        "the build attests a glob instead of the list of what it made",
        ".github/workflows/release.yml",
        "          subject-checksums: incoming/build.sha256",
        "          subject-path: incoming/*",
        "TestTheReleaseAttestsWhatItBuilt",
    ),
    (
        # The statement stays in the attestation store and never reaches the
        # machine that signs - so the signing script has nothing to check the
        # build against, and signing something you did not check is the whole
        # attack this ritual is shaped around.
        "the build's statement stops travelling with the build",
        ".github/workflows/release.yml",
        "        run: cp \"${{ steps.provenance.outputs.bundle-path }}\" incoming/build.provenance.sigstore.json",
        "        run: echo the statement stays in the store",
        "TestTheReleaseMakesItsDocumentAndHandsItOver",
    ),
    (
        # The defect the four phase split exists to prevent: unsigned Windows
        # executables on a release page, replaced later by signed ones. Even a
        # quarter of an hour is long enough for somebody to download one.
        "the build publishes archives that have not been signed yet",
        ".github/workflows/release.yml",
        "          gh release create \"${GITHUB_REF_NAME}\" \"${flags[@]}\"\n",
        "          gh release create \"${GITHUB_REF_NAME}\" \"${flags[@]}\" incoming/*\n",
        "TestTheReleaseHandsTheBuildOverInsteadOfPublishingIt",
    ),
    (
        # The question stops naming a workflow and asks about whatever happened
        # to report. A workflow that never ran then reads exactly like one that
        # passed, which is the failure this check exists for.
        "the release asks about any run instead of about CI",
        ".github/workflows/release.yml",
        "gh run list --workflow=CI --commit=",
        "gh run list --commit=",
        "TestTheReleaseChecksTheTreeAndTheChangelogFirst",
    ),
    (
        # The real bug, put back. Measured on this machine on 2026-08-28:
        # selecting the certificate by the display name "Code Signing" finds
        # nothing on a Polish Windows, which calls it "Podpisywanie kodu" - so
        # the script reports a missing card while the card is plugged in and
        # working, and the person holding it starts debugging the reader.
        "the signing script looks for the certificate by name instead of by OID",
        ".github/scripts/sign_release.py",
        "CODE_SIGNING_OID = \"1.3.6.1.5.5.7.3.3\"",
        "CODE_SIGNING_OID = \"Code Signing\"",
        "TestTheSigningScriptRefusesBeforeItSigns",
    ),
    (
        # The workflow trusts the digest it was handed instead of fetching what
        # the release holds. Everything it then attests is a statement about
        # bytes it never saw, which is the difference between an attestation
        # and a rumour.
        "the attestation workflow stops checking that the checksums describe the files it fetched",
        ".github/workflows/attest-release.yml",
        "          sha256sum -c verify-SHA256SUMS.txt",
        "          sha256sum verify-SHA256SUMS.txt",
        "TestTheAttestationWorkflowDoesNotClaimToHaveBuiltAnything",
    ),
    (
        # The pin becomes a name. A name is what a second certificate on the
        # same machine also has, and the release page looks identical either
        # way - which is exactly the accident the pin exists to catch.
        "the certificate is pinned by a name instead of by its bytes",
        "internal/legal/codesign.go",
        "const CodeSigningSHA256 = \"47b79ad3cfa53ef846cad03a59148f8c981d0b1196891e48b8c8d7982b10c148\"",
        "const CodeSigningSHA256 = \"the code signing certificate on the card\"",
        "TestThePinnedCertificateIsADigestWithADate",
    ),
    (
        "the window binary reaches macOS as a bare binary, which cannot be stapled",
        ".github/workflows/release.yml",
        '              "${work}" "tfg-gui" "com.donislawdev.tfg-gui" "${version}"',
        '              "${work}" "tfg-gui" "com.donislawdev.tfg" "${version}"',
        "TestTheReleaseWrapsTheMacBinariesInBundles",
    ),
    (
        "a copy of the binary can sit beside the bundle and be rejected by Gatekeeper",
        ".github/scripts/make_app_bundle.sh",
        'if [ ! -L "${work}/${binary}" ]; then',
        'if [ ! -e "${work}/${binary}" ]; then',
        "TestTheLinkBesideTheBundleCannotBeACopy",
    ),
    (
        "the Apple certificate has no expiry that can be read as a date",
        "internal/legal/codesign.go",
        'const AppleDeveloperIDExpires = "2029-10-27"',
        'const AppleDeveloperIDExpires = "soon"',
        "TestThePinnedAppleCertificateIsADigestWithADate",
    ),
    (
        "notarisation that came back anything at all counts as success",
        ".github/scripts/sign_macos.sh",
        'grep -q "status: Accepted"',
        'grep -q "status:"',
        "TestTheMacSigningScriptRefusesOnBothSidesOfSigning",
    ),
    (
        "the release is signed on the card only, and macOS goes out unsigned",
        ".github/scripts/sign_release.py",
        "sign_macos(args.tag, work, args.macos_host, args.dry_run)",
        "pass  # sign_macos removed",
        "TestTheReleaseWillNotBeSignedOnOneMachineOnly",
    ),
    (
        "the release notes go on telling people the macOS binaries are unsigned",
        ".github/workflows/release.yml",
        'echo "**Linux binaries are not signed.** Linux says nothing about that, but you may"',
        'echo "**macOS and Linux binaries are not signed yet.** Linux says nothing, but you may"',
        "TestTheReleaseNotesDoNotCallTheMacBinariesUnsigned",
    ),
    (
        "the published release is never read on a Mac, so nobody checks the ticket",
        ".github/workflows/verify-release.yml",
        'if spctl --status 2>/dev/null | grep -q "assessments enabled"; then',
        'if spctl --status 2>/dev/null | grep -q "assessments"; then',
        "TestThePublishedReleaseIsCheckedOnAMacAsWell",
    ),
    (
        "the files a person checks a download with lose the prefix that sorts them last",
        ".github/scripts/sign_release.py",
        'AUX_PREFIX = "verify-"',
        'AUX_PREFIX = "aaa-"',
        "TestTheFilesYouCheckADownloadWithSortToTheEnd",
    ),
    (
        "the statement about the signed bytes lands in the middle of the archives",
        ".github/workflows/attest-release.yml",
        'name="verify-tfg_${TAG#v}.sbom.sigstore.json"',
        'name="tfg_${TAG#v}.sbom.sigstore.json"',
        "TestEveryFileYouCheckADownloadWithCarriesThePrefix",
    ),
    (
        "the workflow that verifies a release can also write to it",
        ".github/workflows/verify-release.yml",
        "    runs-on: windows-latest\n    timeout-minutes: 20\n    permissions:\n      contents: read",
        "    runs-on: windows-latest\n    timeout-minutes: 20\n    permissions:\n      contents: write",
        "TestThePublishedReleaseIsCheckedTheWayAUserWould",
    ),
    (
        "a menu stops being held to a floor, so it shrinks to the length of its own values and cuts off the placeholder the toolkit puts in it",
        "internal/gui/parts/ring.go",
        "\treturn fyne.Max(NumericWidth, box)",
        "\treturn box",
        "TestAMenuIsWideEnoughForTheWordsTheToolkitPutsInIt",
    ),
    (
        "the floor a menu is held to drops below the narrowest box beside it, so one setting is drawn shorter on one screen than on another",
        "internal/gui/parts/ring.go",
        "fyne.Max(NumericWidth, box)",
        "fyne.Max(NumericWidth/2, box)",
        "TestNoMenuIsNarrowerThanTheBoxesItStandsBeside",
    ),
    (
        "a button standing in a row of fields goes back to taking the column, so it is drawn as a panel with a word in the middle",
        "internal/gui/parts/parts.go",
        "\treturn container.NewVBox(layout.NewSpacer(), container.NewHBox(o), layout.NewSpacer())",
        "\treturn container.NewVBox(layout.NewSpacer(), o, layout.NewSpacer())",
        "TestAButtonInARowOfFieldsIsTheSizeOfAButton",
    ),
    (
        "the macOS bundle stops carrying an icon, so the Finder and the Dock draw the program as a blank page",
        ".github/scripts/make_app_bundle.sh",
        'cp "${icon}" "${app}/Contents/Resources/icon.icns"',
        'true "${icon}" "${app}/Contents/Resources"',
        "TestTheMacBundleCarriesOurIcon",
    ),
    (
        "the macOS signing script goes back to changing directory, so the paths it was handed stop meaning anything",
        ".github/scripts/sign_macos.sh",
        '  codesign -d --extract-certificates="$tmp/cert" "$bundle" >/dev/null 2>&1',
        '  ( cd "$tmp" && codesign -d --extract-certificates=cert "$bundle" >/dev/null 2>&1 )',
        "TestTheMacSigningScriptDoesNotDependOnWhereItIsRunFrom",
    ),
    (
        "the attesting half goes back to its own list of what a release holds, which stops agreeing with the checksums",
        ".github/workflows/attest-release.yml",
        "          gh release download \"$TAG\" --pattern 'verify-SHA256SUMS.txt'",
        "          gh release download \"$TAG\" --pattern '*.zip' --pattern '*.tar.gz' --pattern 'verify-SHA256SUMS.txt'",
        "TestTheAttestingHalfFetchesWhatTheChecksumsName",
    ),
    (
        "one workflow goes back to promising a predicate type that the statement does not carry",
        ".github/workflows/release.yml",
        'echo "   --predicate-type https://spdx.dev/Document/v2.3"',
        'echo "   --predicate-type https://spdx.dev/Document"',
        "TestEveryWorkflowNamesTheSamePredicateType",
    ),
    (
        "a TIFF directory says the pixels are a different length than the geometry gives",
        "internal/format/tiff/tiff.go",
        'entry(tagStripByteCounts, typeLong, 1, uint32(pixelBytes(m.width, m.height)))',
        'entry(tagStripByteCounts, typeLong, 1, uint32(pixelBytes(m.width, m.height)/2))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a TIFF writes two directory entries out of ascending tag order",
        "internal/format/tiff/tiff.go",
        "\tshort(tagImageWidth, uint16(m.width))\n\tshort(tagImageLength, uint16(m.height))",
        "\tshort(tagImageLength, uint16(m.height))\n\tshort(tagImageWidth, uint16(m.width))",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a TIFF points StripOffsets at the padding instead of the pixels",
        "internal/format/tiff/tiff.go",
        'entry(tagStripOffsets, typeLong, 1, uint32(pixelsAt(m)))',
        'entry(tagStripOffsets, typeLong, 1, uint32(header))',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a TIFF declares grayscale while writing three samples per pixel",
        "internal/format/tiff/tiff.go",
        '\tshort(tagPhotometric, 2) // RGB',
        '\tshort(tagPhotometric, 1) // RGB',
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a TIFF writes its colour channels in the wrong order",
        "internal/format/tiff/tiff.go",
        "\t\trow[x*3] = uint8((x + off) % 256)\n\t\trow[x*3+1] = uint8((y + off) % 256)\n\t\trow[x*3+2] = uint8((x + y + off) % 256)",
        "\t\trow[x*3] = uint8((x + y + off) % 256)\n\t\trow[x*3+1] = uint8((y + off) % 256)\n\t\trow[x*3+2] = uint8((x + off) % 256)",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a TIFF picture stops being grown to fill the request",
        "internal/format/tiff/tiff.go",
        "\t\treturn w, fill(r.Bytes, w), nil\n\t\tdefault:",
        "\t\treturn w, minDimension, nil\n\t\tdefault:",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a GIF goes back to writing one frame by default",
        "internal/format/gif/gif.go",
        "\tdefaultFrames = 3",
        "\tdefaultFrames = 1",
        "TestAGeneratedGifReallyCarriesAnAnimation",
    ),
    (
        "a GIF puts the marker of every frame in the same place",
        "internal/format/gif/animation.go",
        "\tx = (width - side) * i / frames",
        "\tx = (width - side) * i / (frames * 64)",
        "TestAGeneratedGifReallyCarriesAnAnimation",
    ),
    (
        "a GIF stops asking for a global colour table, so every frame carries its own",
        "internal/format/gif/animation.go",
        "\t\tConfig: image.Config{ColorModel: base.Palette, Width: m.width, Height: m.height},",
        "\t\tConfig: image.Config{Width: m.width, Height: m.height},",
        "TestAGeneratedGifReallyCarriesAnAnimation",
    ),
    (
        "a GIF animation plays once instead of looping",
        "internal/format/gif/animation.go",
        "\t\tLoopCount: 0,",
        "\t\tLoopCount: -1,",
        "TestAGeneratedGifReallyCarriesAnAnimation",
    ),
    (
        "a still GIF is quietly animated too, so there is no way back to the older bytes",
        "internal/format/gif/animation.go",
        "\tif m.frames <= 1 {",
        "\tif m.frames <= 0 {",
        "TestAStillGifIsStillAvailableAndSaysSo",
    ),
    (
        "a WebP writes its colour channels in the wrong order",
        "internal/format/webp/vp8l.go",
        "\t\t\tw.write(reverseByte(px[x*3+1]), 8)\n\t\t\tw.write(reverseByte(px[x*3]), 8)",
        "\t\t\tw.write(reverseByte(px[x*3]), 8)\n\t\t\tw.write(reverseByte(px[x*3+1]), 8)",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a WebP miscounts its own header, so the chunk promises a length the stream does not reach",
        "internal/format/webp/vp8l.go",
        "\treturn (headerBits+7)/8 + pixels*bitsPerPixel/8",
        "\treturn (headerBits+7)/8 + pixels*bitsPerPixel/8 + 1",
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a WebP drops the odd byte after the payload, so half of all sizes stop being reachable",
        "internal/format/webp/webp.go",
        "\t\tm.tail = (delta - chunkHeader - 4) % 2",
        "\t\tm.tail = (delta - chunkHeader - 4) * 0",
        "TestEveryFormatHitsTheOrderedSizeExactly",
    ),
    (
        "a WebP declares a green code with no room for the length prefixes",
        "internal/format/webp/vp8l.go",
        "\twriteFlatCode(w, greenAlphabet)",
        "\twriteFlatCode(w, literals)",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a WebP lies about how long its RIFF payload is",
        "internal/format/webp/webp.go",
        "\t\t{num: uint32(4 + body)},",
        "\t\t{num: uint32(body)},",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "the AVIF ladder claims a picture fits a file far too small to hold it, so the size it promised cannot be met",
        "internal/format/avif/avif.go",
        "\t{640, 480, 16389}, {320, 240, 6792}, {256, 256, 4890}, {160, 120, 3392},",
        "\t{640, 480, 311}, {320, 240, 6792}, {256, 256, 4890}, {160, 120, 3392},",
        "TestAvifDrawsAPictureThatGrowsWithTheFileAndAlwaysFits",
    ),
    (
        "planning codes the picture again, so previewing a large run costs what the run costs",
        "internal/format/avif/avif.go",
        "\treturn memo{width: pick.width, height: pick.height, quality: q, seed: r.Seed, label: label}, nil",
        "\treturn coded(memo{width: pick.width, height: pick.height, quality: q, seed: r.Seed, label: label})",
        "TestAvifPlanningDoesNotCodeThePicture",
    ),
    (
        "the build tags stop naming the tag that keeps the encoder inside its buffers",
        ".github/build-tags",
        "noasm",
        "keepthecrash",
        "TestTheBuildTagsFileNamesTheTagThatKeepsTheEncoderInsideItsBuffers",
    ),
    (
        "a workflow command compiles this project without the build tags, so CI stops testing what the release ships",
        ".github/workflows/ci.yml",
        "        run: go vet -tags \"$(cat .github/build-tags)\" ./...",
        "        run: go vet ./...",
        "TestEveryWorkflowCommandThatBuildsUsPassesTheBuildTags",
    ),
    (
        "the window opens on a format picked by position rather than on the first one the registry hands it",
        "internal/gui/window/generate.go",
        "\t\tg.formatPick.SetSelected(ids[0])",
        "\t\tg.formatPick.SetSelected(ids[len(ids)-1])",
        "TestTheFormatAScreenOpensOnIsTheFirstOneRegistered",
    ),
    (
        "a log setting arriving at its default counts as one somebody asked for, which puts every shape but the default out of reach from a window",
        "internal/format/logfile/options.go",
        "\tif !ok || v == def {",
        "\tif !ok {",
        "TestEveryLogShapeIsReachableWithEverySettingSent",
    ),
    (
        "the syslog process id is counted as four digits always, so one closing entry in ten is a byte short",
        "internal/format/logfile/shapes.go",
        "pidWidth(pid)+len(\"]: \")",
        "4+len(\"]: \")",
        "TestEveryLogShapeHitsTheSizeAcrossSeeds",
    ),
    (
        "a menu sitting on its declared default counts as chosen, so the folded line lists every setting a format declares",
        "internal/gui/parts/property.go",
        "return sel.Selected != \"\" && sel.Selected != p.Default",
        "return sel.Selected != \"\"",
        "TestTheFoldedLineNamesWhatWasChosenAndNotTheDefaults",
    ),
    (
        "the log clock stops advancing, so every entry happens at the same instant again",
        "internal/format/logfile/clock.go",
        "c.at = c.at.Add(c.step)",
        "c.at = c.at.Add(0)",
        "TestALogClockAdvancesByOneTickPerEntry",
    ),
    (
        "the log clock is not put back for the entry built only to be measured, so the file skips a tick",
        "internal/format/logfile/clock.go",
        "func (c *clock) back() { c.at = c.at.Add(-c.step) }",
        "func (c *clock) back() { c.at = c.at.Add(0) }",
        "TestALogClockAdvancesByOneTickPerEntry",
    ),
    (
        "timestamps=fixed starts advancing too, so the way back from the breaking change stops being one",
        "internal/format/logfile/clock.go",
        "if o.advancing {",
        "if true {",
        "TestALogWithFixedTimestampsHoldsOneInstant",
    ),
    (
        "a log setting that can do nothing for the chosen shape is accepted in silence instead of refused",
        "internal/format/logfile/options.go",
        "\tif s.web {\n\t\treturn nil\n\t}",
        "\tif true {\n\t\treturn nil\n\t}",
        "TestALogRefusesASettingThatWouldChangeNothing",
    ),
    (
        "the JSON log label goes back to a hash comment, the one line no JSON reader will take",
        "internal/format/logfile/shapes.go",
        "shortest: shortestJSONLine, label: jsonLabel}",
        "shortest: shortestJSONLine, label: hashLabel}",
        "TestTheLogLabelIsALineItsOwnReaderAccepts",
    ),
    (
        "a one item list of log settings spends a draw anyway, which shifts every entry and breaks the pinned way back",
        "internal/format/logfile/address.go",
        "\tif len(xs) == 1 {",
        "\tif len(xs) == 0 {",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the JPEG XL ladder claims a picture fits a file far too small to hold it, so the size it promised cannot be met",
        "internal/format/jxl/jxl.go",
        "	{640, 480, 22762}, {320, 240, 7949}, {256, 256, 6485}, {160, 120, 3542},",
        "	{640, 480, 147}, {320, 240, 7949}, {256, 256, 6485}, {160, 120, 3542},",
        "TestJxlDrawsAPictureThatGrowsWithTheFileAndAlwaysFits",
    ),
    (
        "a JPEG XL rung declares a ceiling below what that rung actually codes to",
        "internal/format/jxl/jxl.go",
        "	{80, 60, 1267}, {40, 30, 731}, {20, 15, 452}, {8, 6, 222},",
        "	{80, 60, 100}, {40, 30, 731}, {20, 15, 452}, {8, 6, 222},",
        "TestEveryJxlRungStillFitsInsideItsDeclaredCeiling",
    ),
    (
        "JPEG XL planning codes the picture again, so previewing a large run costs what the run costs",
        "internal/format/jxl/jxl.go",
        "	return memo{width: pick.width, height: pick.height, quality: q, seed: r.Seed, label: label}, nil",
        "	return coded(memo{width: pick.width, height: pick.height, quality: q, seed: r.Seed, label: label})",
        "TestJxlPlanningDoesNotCodeThePicture",
    ),
    (
        "a named JPEG XL quality trusts a ceiling table measured at a different one, so the picture does not fit the file",
        "internal/format/jxl/jxl.go",
        "	if _, named := r.Properties[\"quality\"]; named && q != defaultQuality {",
        "	if _, named := r.Properties[\"quality\"]; named && q == defaultQuality {",
        "TestJxlStillFitsWhenTheQualityIsNotTheOneTheCeilingsWereMeasuredAt",
    ),
    (
        "the JPEG XL file type box stops naming the compatible brand, which libjxl refuses and the Go decoder does not",
        "internal/format/jxl/jxl.go",
        "	'j', 'x', 'l', ' ', // compatible brand",
        "	'n', 'o', 'n', 'e', // compatible brand",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "the JPEG XL codestream goes into a partial codestream box without the index one needs, so no reader finds a picture",
        "internal/format/jxl/jxl.go",
        "	if err := isobmff.WriteBoxHeader(w, \"jxlc\", int64(len(coded))); err != nil {",
        "	if err := isobmff.WriteBoxHeader(w, \"jxlp\", int64(len(coded))); err != nil {",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        "a named quality trusts a ceiling table measured at a different one, so the picture does not fit the file",
        "internal/format/avif/avif.go",
        "\tif _, named := r.Properties[\"quality\"]; named && q != defaultQuality {",
        "\tif _, named := r.Properties[\"quality\"]; named && q == defaultQuality {",
        "TestAvifStillFitsWhenTheQualityIsNotTheOneTheCeilingsWereMeasuredAt",
    ),
    (
        "the deepest nesting an archive offers goes past what a tar header can hold",
        "internal/format/archive/layout.go",
        "maxDepth = 50",
        "maxDepth = 200",
        "TestTheDeepestPathEveryFormatCanMakeStillFitsAUstarHeader",
    ),
    (
        "the ceiling on nesting stops being one tar actually enforces",
        "internal/format/archive/layout.go",
        "return maxDepth }",
        "return maxDepth * 10 }",
        "TestAPathOneStepPastTheCeilingIsRefusedByTar",
    ),
    (
        "an archive nobody asked to nest starts out nested, moving the bytes of every archive ever written",
        "internal/format/archive/layout.go",
        "defaultDepth = 0",
        "defaultDepth = 1",
        "TestAnArchiveNobodyAskedToNestIsStillFlat",
    ),
    (
        "asking for directory entries in a flat archive stops being refused and quietly does nothing",
        "internal/format/archive/layout.go",
        "if dirs && depth == 0 {",
        "if dirs && depth == 0 && false {",
        "TestAskingForDirectoryEntriesInAFlatArchiveNamesBothSettings",
    ),
    (
        "the directories an archive names come innermost first, so a reader meets a child before its parent",
        "internal/format/archive/layout.go",
        "Layout{Depth: i}.Path(\"\")",
        "Layout{Depth: l.Depth + 1 - i}.Path(\"\")",
        "TestTheDirectoriesAnArchiveNamesComeOutermostFirst",
    ),
    (
        "a switch on the window is drawn and then ignored, so the setting never reaches the file",
        "internal/gui/parts/property.go",
        "Value:   func() string { return strconv.FormatBool(check.Checked) },",
        "Value:   func() string { return strconv.FormatBool(check.Checked && false) },",
        "TestASwitchOnTheWindowReachesTheFileItDescribes",
    ),
    (
        "a compressed archive stops giving back what the compressor freed, so it comes out short",
        "internal/format/zip/filler.go",
        "size := m.fillerSize + freed()",
        "size := m.fillerSize + freed()*0",
        "TestACompressedArchiveStillHitsTheSizeToTheByte",
    ),
    (
        "asking for compression quietly stores everything and only grows the padding",
        "internal/format/targz/size.go",
        "gzip.NewWriterLevel(w, m.squeeze.Level)",
        "gzip.NewWriterLevel(w, m.squeeze.Level*0)",
        "TestAskingForCompressionActuallyCompresses",
    ),
    (
        "compression stops being refused when the size is left to the contents, so a length is planned that only compressing could give",
        "internal/format/archive/compression.go",
        "if r.SizeFromContents {",
        "if r.SizeFromContents && false {",
        "TestCompressionWithASizeFromTheContentsIsRefusedNamingBoth",
    ),
    (
        "compression stops being refused beside a password, so a locked entry would have to be held in memory whole",
        "internal/format/archive/compression.go",
        "if locked {",
        "if locked && false {",
        "TestCompressionWithAPasswordIsRefusedNamingBoth",
    ),
    (
        "an archive nobody asked to compress starts compressing, moving the bytes of every archive ever written",
        "internal/format/archive/compression.go",
        "return Squeeze{Name: CompressNone, Level: flate.NoCompression}, nil",
        "return Squeeze{Name: CompressDefault, Level: levels[CompressDefault]}, nil",
        "TestAnArchiveNobodyAskedToCompressIsStored",
    ),
    (
        "the plain log draws from every severity this format knows rather than from the mix that was chosen",
        "internal/format/logfile/shapes.go",
        "func appendPlain(dst []byte, st *state, want int64) []byte {\n\tlevel := pick(st.rng, st.opt.levels)",
        "func appendPlain(dst []byte, st *state, want int64) []byte {\n\tlevel := pick(st.rng, levelSets[\"realistic\"])",
        "TestTheLogSeverityMixIsDrawnAndSetsTheMinimum",
    ),
    (
        "the JSON log line draws from every severity rather than from the mix that was chosen",
        "internal/format/logfile/shapes.go",
        "func appendJSONLine(dst []byte, st *state, want int64) []byte {\n\tlevel := pick(st.rng, st.opt.levels)",
        "func appendJSONLine(dst []byte, st *state, want int64) []byte {\n\tlevel := pick(st.rng, levelSets[\"realistic\"])",
        "TestTheLogSeverityMixIsDrawnAndSetsTheMinimum",
    ),
    (
        "the shortest log entry leaves room for the longest severity of ALL of them, so a quiet log announces a floor it can beat",
        "internal/format/logfile/address.go",
        "func longestLevel(o options) int  { return longest(o.levels) }",
        "func longestLevel(o options) int  { return longest(levelSets[\"realistic\"]) }",
        "TestTheLogSeverityMixIsDrawnAndSetsTheMinimum",
    ),
    (
        "a severity mix asked for beside a shape that carries no severity is accepted and then does nothing",
        "internal/format/logfile/options.go",
        "chosen && !o.shape.levelled",
        "chosen && !o.shape.levelled && false",
        "TestALogRefusesASettingThatWouldChangeNothing",
    ),
    (
        "the padding drops a comma whatever the dialect, so a semicolon table never exercises its own quoting",
        "internal/format/csvfile/csv.go",
        "both := string(sep) + \" \"",
        "both := \", \"",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        "the fields of a row are separated by a comma whatever the dialect says",
        "internal/format/csvfile/csv.go",
        "sep := r.dia.sep",
        "sep := byte(',')",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        "a row ends with a newline whatever the dialect says, so crlf never reaches the file",
        "internal/format/csvfile/csv.go",
        "return append(dst, r.dia.eol...)",
        "return append(dst, \"\\n\"...)",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        "the header row is written even when it was turned off",
        "internal/format/csvfile/csv.go",
        "if m.dia.header {",
        "if m.dia.header || true {",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        "the row width counts one byte of ending whatever the dialect, so a crlf table announces a floor it cannot meet",
        "internal/format/csvfile/csv.go",
        "total := int64(d.columns-1) /* separators */ + int64(len(d.eol)) +",
        "total := int64(d.columns-1) /* separators */ + int64(1) +",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        # The floor pays for the quotes only when every field carries them.
        # Charge nothing and quote_style all announces a floor twelve bytes
        # below the shortest row it can write, so the closing row overshoots
        # the length it was handed.
        "the floor is worked out as if no field were ever quoted",
        "internal/format/csvfile/dialect.go",
        "return 2 * columns",
        "return 0 * columns",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        # A header is a row of fields like any other. Leave it bare under
        # quote_style all and the file is still the right size and still
        # parses - only the one row disagrees with the setting that wrote it.
        "the header is left bare while every row below it is quoted",
        "internal/format/csvfile/dialect.go",
        "if d.quotes.everyField {",
        "if d.quotes.everyField && false {",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        # quote_style none has to change the CONTENT, not only the punctuation.
        # Keep dropping separators into an unquoted description and every row
        # of a none table is ragged, at exactly the right size.
        "a description under quote_style none keeps carrying separators it has no quotes for",
        "internal/format/csvfile/csv.go",
        "if carries && i%3 == 0 {",
        "if (carries || true) && i%3 == 0 {",
        "TestEveryCSVDialectIsWellFormed",
    ),
    (
        # The same defect in the padding rather than in the phrase, which is a
        # different function and a different guard - the closing row is the
        # only one the filler writes.
        "the padding under quote_style none keeps carrying separators it has no quotes for",
        "internal/format/csvfile/csv.go",
        "if carries && i%4 == 0 {",
        "if (carries || true) && i%4 == 0 {",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        # The closing row under minimal decides between quoted and bare by
        # MEASURING the description it built. Skip the measurement and it is
        # never quoted, so the padding stops carrying the separator at all -
        # right size, valid CSV, and the quoted path is never exercised again.
        "the closing row never asks whether its description earned its quotes",
        "internal/format/csvfile/csv.go",
        "if q.separatorsInDescription && room >= 2 {",
        "if q.separatorsInDescription && room >= 2 && false {",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        # The checker is TOLD the width and stops comparing it, so it becomes a
        # rubber stamp on the one axis where every value produces a well formed
        # file. A nine column table called eight would sail through.
        "the structural checker accepts whatever width it is told, so it judges nothing",
        "internal/oracle/strict.py",
        "if wanted is not None and columns != wanted:",
        "if wanted is not None and columns != wanted and False:",
        "TestEveryCSVWidthIsWellFormed",
    ),
    (
        # The six columns this format has always written come out in a
        # different order. Same count, same widths, same size, and every
        # reader still parses it - somebody's pinned hash is simply wrong now.
        "the six original columns come out in a different order",
        "internal/format/csvfile/dialect.go",
        'var baseColumns = []string{"id", "name", "email", "amount", "created"}',
        'var baseColumns = []string{"name", "id", "email", "amount", "created"}',
        "TestTheDefaultCSVStillHasItsSixOriginalColumns",
    ),
    (
        # The width is stored and the writer ignores it: every table comes out
        # six columns wide whatever was ordered. Right size, parses everywhere,
        # every row agreeing with every other - only a guard counting the
        # fields against the number asked for sees it.
        "the table is six columns wide whatever width was ordered",
        "internal/format/csvfile/csv.go",
        "for at := 0; at < r.dia.columns-1; at++ {",
        "for at := 0; at < defaultColumns-1; at++ {",
        "TestTheCSVColumnCountIsInTheFileAndMovesTheFloor",
    ),
    (
        # The header keeps the width and the rows lose it, which is the shape
        # where a reader reports a ragged table rather than a narrow one.
        "the header names a different number of columns than the rows carry",
        "internal/format/csvfile/dialect.go",
        "func columnNamesFor(columns int) []string {",
        "func columnNamesFor(columns int) []string {\n\tcolumns++",
        "TestTheCSVColumnCountIsInTheFileAndMovesTheFloor",
    ),
    (
        # The floor is worked out for six columns and handed to every width, so
        # a wide table announces a minimum it cannot meet and a narrow one
        # refuses sizes it could have written.
        "the floor is worked out for one width and handed to the rest",
        "internal/format/csvfile/csv.go",
        "for at := 0; at < d.columns-1; at++ {",
        "for at := 0; at < defaultColumns-1; at++ {",
        "TestTheCSVColumnCountIsInTheFileAndMovesTheFloor",
    ),
    (
        # The description stops being last. The row is the right length and the
        # padding still reaches the byte - it just lands in the middle of the
        # record, where a stretched field is not what the format promised.
        "the description is no longer the last column",
        "internal/format/csvfile/dialect.go",
        'return append(out, "description")',
        'return append([]string{"description"}, out...)',
        "TestTheCSVColumnCountIsInTheFileAndMovesTheFloor",
    ),
    (
        # The manifest reports the count this format started with rather than
        # the one the run used, so a script reading the manifest is told the
        # wrong width about a perfectly correct file.
        "the manifest names six columns whatever width was written",
        "internal/format/csvfile/csv.go",
        '"columns":     d.columns,',
        '"columns":     defaultColumns,',
        "TestTheCSVManifestRecordsTheDialectAsFacts",
    ),
    (
        # The exact shape that cost two red runs: a whole tree test run with no
        # timeout of its own, so Go's ten minutes a package decides instead of
        # the twenty the job declares. The run then PANICS out of whichever test
        # was running, which reads as a broken test rather than a busy runner.
        "a workflow runs the whole suite without stating a timeout",
        ".github/workflows/ci.yml",
        './... -count=1 -timeout 18m\n\n      - name: build the command line binary',
        './... -count=1\n\n      - name: build the command line binary',
        "TestEveryWholeTreeTestRunStatesItsOwnTimeout",
    ),
    (
        # A setting renamed in the registry and not in the README is exactly the
        # drift that left log saying "none" while it carried eight settings, and
        # zip listing three of its eight. The page still reads perfectly - it is
        # simply describing a build that no longer exists.
        "a setting is renamed in the registry and the README keeps the old name",
        "internal/format/csvfile/dialect.go",
        "Name: QuoteStyle, Kind: format.PropertyChoice,",
        "Name: QuoteStyle + \"_v2\", Kind: format.PropertyChoice,",
        "TestTheReadmeSettingsTableAgreesWithTheRegistry",
    ),
    (
        # The other half: a format the registry has and the first page a visitor
        # reads does not. jxl arrived this way on 2026-08-31 and sat unlisted.
        "a format is registered under a name the README's own list does not carry",
        "internal/format/jxl/jxl.go",
        'ID:          "jxl",',
        'ID:          "jxl2",',
        "TestTheReadmeListsEveryFormatItShips",
    ),
    (
        # The manifest is the half a test suite reads. Write the default
        # whatever was asked for and a run under all or none is described as
        # a run under minimal, with the file itself perfectly correct.
        "the manifest names the default quote style whatever was asked for",
        "internal/format/csvfile/csv.go",
        '"quote_style": d.quotes.id,',
        '"quote_style": defaultDialect().quotes.id,',
        "TestTheCSVManifestRecordsTheDialectAsFacts",
    ),
    (
        "the floor pays for a header that is not there",
        "internal/format/csvfile/dialect.go",
        "if !d.header {\n\t\treturn 0\n\t}",
        "if !d.header && false {\n\t\treturn 0\n\t}",
        "TestTheCSVDialectIsInTheFileAndMovesTheFloor",
    ),
    (
        "the manifest names the separator with the recipe's word instead of the character that is in the file",
        "internal/format/csvfile/csv.go",
        "\"separator\":   string(d.sep),",
        "\"separator\":   d.delimiterID,",
        "TestTheCSVManifestRecordsTheDialectAsFacts",
    ),
    (
        "the structural checker splits on a comma whatever dialect it was told, so it judges the wrong table",
        "internal/oracle/strict.py",
        "elif ch == sep:",
        "elif ch == \",\":",
        "TestEveryCSVDialectIsWellFormed",
    ),
    (
        "the gzip framing is measured a byte per block out, so every tar.gz misses its planned length",
        "internal/format/targz/framing.go",
        "f := framing{perBlock: two - one}",
        "f := framing{perBlock: two - one + 1}",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        "the gzip framing keeps the block cost in its base, so the arithmetic is out by five on every archive",
        "internal/format/targz/framing.go",
        "f.base = one - f.perBlock",
        "f.base = one",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        "the check that the framing model is a line asks about the wrong point, so it refuses a build that is fine",
        "internal/format/targz/framing.go",
        "if want := f.base + 3*f.perBlock; three != want {",
        "if want := f.base + 2*f.perBlock; three != want {",
        "TestTarGzSizeArithmeticSurvivesEveryShape",
    ),
    (
        "the compressed archive gives back only the overshoot, so the walk steps across a 512 byte staircase and never lands",
        "internal/format/targz/compress.go",
        "next := filler - (2 - deficit) - tarBlock",
        "next := filler - (2 - deficit)",
        "TestACompressedArchiveStillHitsTheSizeToTheByte",
    ),
    (
        # The pattern names no version. It named 1.176.0 until 2026-09-16 and
        # went stale on the very first Dependabot bump of that file (#102),
        # which is the bump this file exists to receive - an entry that goes
        # stale on the event it guards against would go stale every week.
        # ">=" keeps the line a valid requirement and stops being a pin, which
        # is what the guard asks about.
        "the semgrep pin loses its version, so somebody else's release becomes a red build here",
        ".github/requirements-semgrep.txt",
        "\nsemgrep==",
        "\nsemgrep>=",
        "TestCIRunsSemgrepAndDecidesFromTheReportRatherThanTheExitCode",
    ),
    (
        "the registry folds a choice again, so a value the declaration does not spell reaches the generator",
        "internal/format/format.go",
        "if raw == c {",
        "if strings.EqualFold(raw, c) {",
        "TestAValueTheDeclarationDoesNotSpellIsRefusedByTheRegistry",
    ),
    (
        "the registry folds a boolean again, so TRUE is somebody else's problem three layers down",
        "internal/format/format.go",
        "\tswitch raw {",
        "\tswitch strings.ToLower(raw) {",
        "TestAValueTheDeclarationDoesNotSpellIsRefusedByTheRegistry",
    ),
    (
        "an owner the archive does not know falls through to no owner at all, the way it did before",
        "internal/format/archive/ownership.go",
        "\tdefault:",
        "\tcase \"a value no declaration contains\":",
        "TestAnOwnerTheArchiveDoesNotKnowIsRefusedRatherThanDropped",
    ),
    (
        "the manifest reports the owner the archive would have had anyway, so asking for root leaves no trace",
        "internal/format/targz/targz.go",
        "archive.EntryOwner: m.own.Stated.Owner,",
        "archive.EntryOwner: archive.OwnerUnset,",
        "TestTheArchiveManifestSaysWhoOwnsTheEntries",
    ),
    (
        "the manifest reports the default mode whatever was asked for",
        "internal/format/targz/targz.go",
        "archive.EntryMode:  m.own.Stated.Mode,",
        "archive.EntryMode:  \"644\",",
        "TestTheArchiveManifestSaysWhoOwnsTheEntries",
    ),
    (
        "the archive package goes back to writing out the list of compression words by hand",
        "internal/format/archive/compression.go",
        "Reason: axes[Compression].Allows(raw),",
        "Reason: core.Says(\"archive.CompressionWords\", \"it takes one of: best, default, fast, none\"),",
        "TestTheSentenceNamingWhichWordsASettingTakesIsWrittenInOnePlace",
    ),
    (
        "an archive refuses an owner in words of its own rather than the declaration's",
        "internal/format/archive/ownership.go",
        "Reason: axes[EntryOwner].Allows(raw),",
        "Reason: core.Says(\"archive.OwnerWords\", \"it is not an owner this build knows\"),",
        "TestAnArchiveRefusesAValueInTheWordsTheRegistryBuilds",
    ),
    (
        "the reader stops recognising an owner the declaration still offers",
        "internal/format/archive/ownership.go",
        "case OwnerUser:",
        'case OwnerUser + "-gone":',
        "TestEveryWordTheArchiveDeclarationOffersIsOneItsReaderAccepts",
    ),
    (
        "a check of a published release goes back to stopping the job, so the checks behind it are never asked",
        ".github/workflows/verify-release.yml",
        "        id: latest\n        continue-on-error: true",
        "        id: latest",
        "TestNoReleaseCheckCanHideTheOnesBehindIt",
    ),
    (
        "the release verdict reads conclusion, which continue-on-error rewrites to success, so it passes whatever happened",
        ".github/workflows/verify-release.yml",
        "steps.latest.outcome",
        "steps.latest.conclusion",
        "TestNoReleaseCheckCanHideTheOnesBehindIt",
    ),
    (
        "the release verdict stops running after a red check, so the job ends silently on the first failure",
        ".github/workflows/verify-release.yml",
        "        if: always()",
        "        if: success()",
        "TestNoReleaseCheckCanHideTheOnesBehindIt",
    ),
    (
        "the check of a published release holds the page the owner writes by hand to the list again",
        ".github/workflows/verify-release.yml",
        '          latest="$(gh api',
        '          bash .github/scripts/note_says.sh notes.md\n          latest="$(gh api',
        "TestWhatAReleaseNoteHasToSayIsDecidedInOnePlace",
    ),
    (
        "the release build stops holding the note it generates to what a release note has to say",
        ".github/workflows/release.yml",
        "bash .github/scripts/note_says.sh notes.md",
        "true",
        "TestWhatAReleaseNoteHasToSayIsDecidedInOnePlace",
    ),
    (
        "the generated release note names a predicate type the list does not ask for, so the published page tells people the wrong command",
        ".github/workflows/release.yml",
        'echo "   --predicate-type https://spdx.dev/Document/v2.3"',
        'echo "   --predicate-type https://spdx.dev/Document"',
        "TestTheGeneratedReleaseNoteSaysEverythingTheListAsksFor",
    ),
    (
        "a comment in the repository is written in Polish, which D9 forbids and no character based check can see",
        "internal/format/archive/compression.go",
        "\ts := Squeeze{Name: raw, Level: level}",
        "\t// Tutaj jest wynik, ktory oddajemy dalej.\n\ts := Squeeze{Name: raw, Level: level}",
        "TestNoCommentInTheRepositoryIsWrittenInPolish",
    ),
    (
        "the social card changes wording and the committed picture of it stays as it was",
        "web/templates/social.html",
        "one YAML file, same bytes in every CI run",
        "one YAML file, identical bytes in every CI run",
        "TestTheSocialPictureShowsTheCardAsItIsNow",
    ),
    (
        "generate goes back to planning with a context nobody can finish, so Ctrl+C waits for the whole plan",
        "internal/cli/generate.go",
        "engine.PlanContext(ctx, targets, opt)",
        "engine.Plan(targets, opt)",
        "TestPlanningStopsBeforeItWalksToTheFarTarget",
    ),
    (
        "validate goes back to planning with a context nobody can finish",
        "internal/cli/recipecmd.go",
        "engine.PlanContext(ctx, targets, planningOptions(rec))",
        "engine.Plan(targets, planningOptions(rec))",
        "TestPlanningFromTheCommandLineCanBeStopped",
    ),
    (
        "preset show goes back to planning its budget with a context nobody can finish",
        "internal/cli/preset.go",
        "engine.PlanContext(ctx, targets, engine.Options{",
        "engine.Plan(targets, engine.Options{",
        "TestPlanningFromTheCommandLineCanBeStopped",
    ),
    (
        "a stopped validate falls through to the machine readable report and calls the recipe invalid",
        "internal/cli/recipecmd.go",
        "stopped := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)",
        "stopped := false && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))",
        "TestAStoppedValidateDoesNotCallTheRecipeInvalid",
    ),
    (
        "a zip ordered at exactly its bare size takes the early return even when it is squeezed, so nothing gives the freed bytes back",
        "internal/format/zip/zip.go",
        "case target == bare && !m.squeeze.On():",
        "case target == bare:",
        "TestACompressedZipIsEitherWrittenAtItsSizeOrRefusedInWords",
    ),
    (
        "a filler that would have to be shorter than nothing writes nothing instead of saying the size is impossible",
        "internal/format/zip/filler.go",
        "if size < 0 {",
        "if false && size < 0 {",
        "TestACompressedZipIsEitherWrittenAtItsSizeOrRefusedInWords",
    ),
    (
        "a recipe stops being checked for how deeply it nests, so 40 kB can ask for as much heap as the machine has",
        "internal/recipe/recipe.go",
        "if depth := nestingDepth(src); depth > MaxNestingDepth {",
        "if depth := nestingDepth(src); false && depth > MaxNestingDepth {",
        "TestAHostileRecipeCannotHangTheReader",
    ),
    (
        "the depth scan counts brackets again and stops seeing block lists",
        "internal/recipe/limits.go",
        "if d := flow + len(dashes); d > deepest {",
        "if d := flow; d > deepest {",
        "TestAHostileRecipeCannotHangTheReader",
    ),
    (
        "a dash further right no longer opens a sequence inside the last one",
        "internal/recipe/limits.go",
        "if len(open) == 0 || open[len(open)-1] < column {",
        "if false && (len(open) == 0 || open[len(open)-1] < column) {",
        "TestAHostileRecipeCannotHangTheReader",
    ),
    (
        "the depth walk stops counting the collections that close, so ordinary nesting reads as unbounded",
        "internal/recipe/limits.go",
        "case token.SequenceEndType, token.MappingEndType:",
        "case token.SequenceEndType:",
        "TestAHostileRecipeCannotHangTheReader",
    ),
    (
        "regenerating goes back to rewriting every stored screen",
        "internal/guard/screenpixels_test.go",
        "return !v.accepts",
        "return !v.accepts || true",
        "TestRegeneratingAScreenLeavesTheUnchangedOnesAlone",
    ),
    (
        "regenerating stops writing anything at all",
        "internal/guard/screenpixels_test.go",
        "return !v.accepts",
        "return !v.accepts && false",
        "TestRegeneratingAScreenLeavesTheUnchangedOnesAlone",
    ),
    (
        "a stored picture showing something else is accepted anyway",
        "internal/guard/screenpixels_test.go",
        "v.accepts = v.worst <= pixelTolerance()",
        "v.accepts = v.worst <= pixelTolerance() || true",
        "TestRegeneratingAScreenLeavesTheUnchangedOnesAlone",
    ),
    (
        "a stored widget tree is taken as standing whatever it says",
        "internal/guard/screenpixels_test.go",
        "return err == nil && want == markup",
        "return err == nil && want == markup || true",
        "TestRegeneratingAScreenLeavesTheUnchangedOnesAlone",
    ),
    (
        "a changelog link definition drifts away from the section it names",
        "CHANGELOG.md",
        "[0.3.0]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.2.0...v0.3.0",
        "[0.3.0-typo]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.2.0...v0.3.0",
        "TestEveryChangelogVersionHasItsLinkDefinition",
    ),
    (
        "Unreleased is left comparing from the release before the newest one",
        "CHANGELOG.md",
        "[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.5.0...HEAD",
        "[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.4.0...HEAD",
        "TestEveryChangelogVersionHasItsLinkDefinition",
    ),
    (
        # The plainest way around every import based guard here: curl needs no
        # networking package of its own.
        "the command line binary gains the ability to start a program",
        "cmd/tfg/main.go",
        '\t"context"\n\t"os"',
        '\t"context"\n\t_ "os/exec"\n\t"os"',
        "TestTheCommandLineBinaryCannotStartAProcess",
    ),
    (
        # The library load is wanted and documented. Swapping which library is
        # the whole of what a telemetry addition would need to change.
        "the dark menu call loads a network library instead of the theme one",
        "internal/gui/darkmenus_windows.go",
        'systemLibraryPath("uxtheme.dll")',
        'systemLibraryPath("wininet.dll")',
        "TestNothingShippedHoldsAnUndeclaredWayOut",
    ),
    (
        # An address INSIDE a longer literal, which is the shape the first
        # version of this guard read straight past.
        "an endpoint replaces a namespace inside a longer string",
        "internal/format/svgfile/svg.go",
        "http://www.w3.org/2000/svg",
        "https://telemetry.example/v1",
        "TestNothingShippedHoldsAnUndeclaredWayOut",
    ),
    (
        # The other direction: a permission whose code is gone. This is how an
        # allowlist rots into a rubber stamp.
        "a registered address leaves the code and the permission stays behind",
        "internal/legal/modules.go",
        "https://github.com/nicksnyder",
        "https://example.invalid/moved",
        "TestEveryTelemetryExceptionStillNamesLiveCode",
    ),
    (
        "the scanner stops seeing a socket call",
        "internal/guard/notelemetry_test.go",
        "if socketCalls[name] {",
        "if socketCalls[name] && false {",
        "TestTheTelemetryScannerRejectsCodeItMustReject",
    ),
    (
        "the scanner stops seeing a spawned process",
        "internal/guard/notelemetry_test.go",
        "if spawnCalls[name] {",
        "if spawnCalls[name] && false {",
        "TestTheTelemetryScannerRejectsCodeItMustReject",
    ),
    (
        # The bug this file actually had, put back. Reading only position zero
        # is exactly "the literal must START with the scheme", which shipped
        # past a thirteen case canary because every case put the URL on its own.
        "the address scan goes back to reading only the start of a literal",
        "internal/guard/notelemetry_test.go",
        "for i := 0; i < len(text); i++ {",
        "for i := 0; i < 1; i++ {",
        "TestTheTelemetryScannerRejectsCodeItMustReject",
    ),
    (
        # A scanner that refuses everything satisfies the canary and guards
        # nothing, so the negative half needs its own break.
        "the address scan treats every position in every string as an address",
        "internal/guard/notelemetry_test.go",
        "if !strings.HasPrefix(rest, scheme) {",
        "if !strings.HasPrefix(rest, scheme) && false {",
        "TestTheTelemetryScannerLeavesOrdinaryCodeAlone",
    ),
    (
        # The registry is per file on purpose. An address permitted everywhere
        # once it is permitted anywhere would let the next one in beside it.
        "the registry stops caring which file an address was registered for",
        "internal/guard/notelemetry_test.go",
        'if _, ok := allowedURLs[rel+"|"+url]; ok {',
        "if _, ok := allowedURLs[url]; ok {",
        "TestARegisteredAddressInItsOwnFileIsAllowed",
    ),
    (
        # A platform the release ships and this does not is a fix somebody
        # cannot have, and it looks exactly like a build that did not run.
        "the build on demand quietly stops offering a command line platform",
        ".github/workflows/dev-build.yml",
        "            windows/amd64 windows/arm64 \\",
        "            windows/amd64 \\",
        "TestTheBuildOnDemandOffersEveryPlatformTheReleaseDoes",
    ),
    (
        "the build on demand builds the window for a different system than the release",
        ".github/workflows/dev-build.yml",
        "          - macos-latest",
        "          - macos-14",
        "TestTheBuildOnDemandOffersEveryPlatformTheReleaseDoes",
    ),
    (
        # One careless line is all it takes for an unsigned binary to reach a
        # release page that a person is supposed to sign for.
        "the build on demand gains permission to write",
        ".github/workflows/dev-build.yml",
        "  contents: read",
        "  contents: write",
        "TestTheBuildOnDemandCannotPublishAndSaysItIsUnofficial",
    ),
    (
        # The window archives lose the note while the command line ones keep
        # it, which is the shape a single found-it check would pass.
        "one of the two jobs stops writing the note that says this is not a release",
        ".github/workflows/dev-build.yml",
        '            .github/scripts/unofficial_note.sh "${work}/UNOFFICIAL-BUILD.txt" "${short}"',
        "            true",
        "TestTheBuildOnDemandCannotPublishAndSaysItIsUnofficial",
    ),
    (
        # The note stops saying the one thing that explains why the reader's
        # own system is refusing to open the file.
        "the note stops saying the binaries are unsigned",
        ".github/scripts/unofficial_note.sh",
        "It is NOT signed.",
        "It is ready to use.",
        "TestTheUnofficialNoteSaysWhatTheArchiveIsNot",
    ),
    (
        "a window binary with no window in it exits zero, so a script is told the window opened",
        "internal/gui/run_nocgo.go",
        "\treturn 1",
        "\treturn 0",
        "TestABuildWithNoWindowInItSaysSoAndKeepsStandardOutputEmpty",
    ),
    (
        "a window binary with no window in it stops saying why, leaving an exit code and nothing to read",
        "internal/gui/run_nocgo.go",
        "fmt.Fprintln(errOut, text.NoWindowInThisBuild)",
        "fmt.Fprintln(io.Discard, text.NoWindowInThisBuild)",
        "TestABuildWithNoWindowInItSaysSoAndKeepsStandardOutputEmpty",
    ),
    (
        "the window entry point stops handing back what opening the window answered, so a build with no window in it reports success",
        "internal/gui/gui.go",
        "\treturn run(ReadLaunch(args), errOut)",
        "\treturn 0",
        "TestABuildWithNoWindowInItSaysSoAndKeepsStandardOutputEmpty",
    ),
    (
        # Verified by hand on 2026-09-05 before this entry was written: with the
        # reversal in place the guard names files_0119.txt as offered out of
        # place, and every other cleanup guard stays green - they ask what was
        # removed, not in what order it was offered.
        "the parallel pass answers in completion order rather than in the order the manifest lists",
        "internal/audit/parallel.go",
        "out[i] = one(i, scratch)",
        "out[len(out)-1-i] = one(i, scratch)",
        "TestCleanupOffersTheFilesInTheOrderTheManifestListsThem",
    ),
    (
        # The workflow claimed the two lists could not drift because the map
        # guard reddens first. It does - but only while the file is missing
        # from the map, and adding it there is what that guard tells you to do.
        # So this is the drift the claim did not cover.
        "a file that declares concurrency is left off the list the race detector is run for",
        ".github/workflows/ci.yml",
        " internal/engine/parallel.go internal/tool/tool.go ",
        " internal/tool/tool.go ",
        "TestTheRaceDetectorIsRunForEveryFileThatDeclaresConcurrency",
    ),
    (
        # Verified by hand on 2026-09-05 before this entry was written: with the
        # old stat question back, the guard reports that the run went ahead over
        # a name somebody else's link was holding - which is also the proof that
        # the old behaviour really did replace it.
        "a name held by a link pointing nowhere stops being a name that is taken",
        "internal/engine/preflight.go",
        "\tif taken == nil {\n\t\treturn exists(path)\n\t}\n\t_, ok := taken[name]\n\treturn ok",
        "\treturn exists(path)",
        "TestANameTakenByALinkPointingNowhereIsStillTaken",
    ),
    (
        # A directory that refuses a LISTING needs an ACL on Windows, so the
        # guard skips there and runs on the other two systems in the matrix.
        # The mutation has to be tried on one of those.
        "a directory that cannot be listed is read as an empty one, so a run writes over what is inside",
        "internal/engine/preflight.go",
        "\tentries, err := os.ReadDir(dir)\n\tif errors.Is(err, fs.ErrNotExist) {\n\t\treturn map[string]struct{}{}\n\t}\n\tif err != nil {\n\t\treturn nil\n\t}",
        "\tentries, err := os.ReadDir(dir)\n\tif errors.Is(err, fs.ErrNotExist) {\n\t\treturn map[string]struct{}{}\n\t}\n\tif err != nil {\n\t\treturn map[string]struct{}{}\n\t}",
        "TestADirectoryThatCannotBeListedIsStillAskedAboutEveryName",
    ),
    (
        # The metric the performance report asked for. Measured 2026-09-05: with
        # it in place all four of the older plan ceiling guards stay green, and
        # only this one notices - which is why it was written.
        "the plan ceiling asks about the last collection rather than about what has been allocated",
        "internal/engine/planmemory.go",
        "\"/gc/heap/allocs:bytes\"",
        "\"/gc/heap/live:bytes\"",
        "TestThePlanCeilingAsksAboutAllocationRatherThanTheLastCollection",
    ),
    (
        "a format goes back to keeping its own copy of the bulk filler loop",
        "internal/format/png/png.go",
        "\t\tchunk := buf[:size]\n\t\tcore.FillRandomBE(chunk, rng)\n",
        "\t\tchunk := buf[:size]\n\t\tfor i := range chunk {\n\t\t\tchunk[i] = byte(rng.Uint64())\n\t\t}\n",
        "TestBulkRandomBytesComeFromOnePlace",
    ),
    (
        "the standing exception letting one format draw a single value outlives the code it was granted for",
        "internal/format/opc/opc.go",
        "rng.UintN(uint(len(Words)))",
        "rng.IntN(len(Words))",
        "TestBulkRandomBytesComeFromOnePlace",
    ),
    (
        "the shared filler stops writing the last group shorter than eight bytes, so every odd sized file loses its tail",
        "internal/core/fill.go",
        "\tif i < len(b) {\n\t\tvar eight [8]byte\n\t\tbinary.BigEndian.PutUint64(eight[:], rng.Uint64())",
        "\tif false && i < len(b) {\n\t\tvar eight [8]byte\n\t\tbinary.BigEndian.PutUint64(eight[:], rng.Uint64())",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the cycle the steady tone is worked out over is off by one, so the pitch drifts a frame every time it wraps",
        "internal/format/wav/signal.go",
        "\tperiod := int64(rate) / gcd(int64(base), int64(rate))\n",
        "\tperiod := int64(rate)/gcd(int64(base), int64(rate)) + 1\n",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "planning goes back to coding the picture it throws away, so a preview of a large run costs what the run costs",
        "internal/format/png/png.go",
        "\tif r.Bytes >= ladderCeiling+iendSize+chunkOverhead &&\n",
        "\tif false && r.Bytes >= ladderCeiling+iendSize+chunkOverhead &&\n",
        "TestPlanningAPictureDoesNotCodeIt",
    ),
    (
        "the same for the animated format, where planning also stopped coding the picture",
        "internal/format/gif/gif.go",
        "\tif r.Bytes >= ladderCeiling+trailerSize+smallestCarryingComment {\n",
        "\tif false && r.Bytes >= ladderCeiling+trailerSize+smallestCarryingComment {\n",
        "TestPlanningAPictureDoesNotCodeIt",
    ),
    (
        "the ceiling planning trusts drops below what the top rung really codes to, so planning accepts sizes the writer cannot deliver",
        "internal/format/png/png.go",
        "const ladderCeiling = 16384\n",
        "const ladderCeiling = 4096\n",
        "TestTheLadderCeilingIsAboveEveryPictureTheTopRungMakes",
    ),
    (
        "the same ceiling drops for the animated format, where sixty frames code to far more than one",
        "internal/format/gif/gif.go",
        "const ladderCeiling = 98304\n",
        "const ladderCeiling = 32768\n",
        "TestTheLadderCeilingIsAboveEveryPictureTheTopRungMakes",
    ),
    (
        "the directory chain loses the padding that keeps a level two digits wide, so every entry inside a deep archive is named something else",
        "internal/format/archive/layout.go",
        "\t\tif i < 10 {\n\t\t\tb.WriteByte('0')\n\t\t}\n",
        "\t\tif false && i < 10 {\n\t\t\tb.WriteByte('0')\n\t\t}\n",
        "TestTheDeepestPathEveryFormatCanMakeStillFitsAUstarHeader",
    ),
    (
        "the reused buffer of a locked entry is handed on at its full length instead of the length written, so the archive runs past its size",
        "internal/format/archive/lock.go",
        "\tout := e.buf[:len(p)]\n",
        "\tout := e.buf\n",
        "TestALockedArchiveStillHitsTheSizeToTheByte",
    ),
    (
        "the same for the older scheme, where the reused buffer is also handed on whole",
        "internal/format/archive/zipcrypto.go",
        "\tout := z.buf[:len(p)]\n",
        "\tout := z.buf\n",
        "TestALockedArchiveStillHitsTheSizeToTheByte",
    ),
    (
        "the document the recipe is decoded from stops skipping one that holds nothing but a comment, so an ordinary leading separator is refused again",
        "internal/recipe/recipe.go",
        "\t\tif d.Body == nil || d.Body.Type() == ast.CommentType {\n\t\t\tcontinue\n\t\t}\n\t\treturn d.Body\n",
        "\t\tif d.Body == nil {\n\t\t\tcontinue\n\t\t}\n\t\treturn d.Body\n",
        "TestOneRecipeIsAcceptedWhateverSeparatorsSurroundIt",
    ),
    (
        # Half of all sizes are unreachable in a two byte encoding, and the
        # promise is that the tool refuses rather than rounds. Let every size
        # through and a UTF-16 file comes out an odd number of bytes long -
        # which three readers reject and three others repair in silence.
        "a wide encoding accepts an odd size instead of refusing it",
        "internal/format/textenc/textenc.go",
        "if (fileBytes-int64(len(c.Preamble())))%c.width() == 0 {",
        "if (fileBytes-int64(len(c.Preamble())))%c.width() == 0 || true {",
        "TestAWideEncodingRefusesAnOddSizeAndNamesOneItCanWrite",
    ),
    (
        # The content budget is file bytes divided by the width of a character.
        # Divide by one and the file comes out twice as long as it was ordered.
        "the content budget forgets that a character costs two bytes",
        "internal/format/textenc/textenc.go",
        "return (fileBytes - int64(len(c.Preamble()))) / c.width()",
        "return fileBytes - int64(len(c.Preamble()))",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        # A mark that says the wrong byte order is worse than no mark: a reader
        # believes it and decodes every character backwards.
        "the little endian mark is written the other way round",
        "internal/format/textenc/textenc.go",
        "return []byte{0xFF, 0xFE}",
        "return []byte{0xFE, 0xFF}",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        # Swap the order the units go out in and the file still DECODES - every
        # pair of bytes is a valid character, 0x74 0x00 reads as U+7400 - so
        # only a check that looks at what the characters ARE catches this.
        "a big endian file is written little endian, and still decodes",
        "internal/format/textenc/textenc.go",
        "return append(dst, byte(u>>8), byte(u))",
        "return append(dst, byte(u), byte(u>>8))",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        # The encoder holds the tail of a character split across two writes.
        # Our own generators never split one, so this is the defence that only
        # a guard written for it can redden.
        "a character split across two writes loses its tail",
        "internal/format/textenc/textenc.go",
        "w.part = append(w.part[:0], p...)",
        "w.part = w.part[:0]",
        "TestACharacterSplitAcrossTwoWritesSurvives",
    ),
    (
        # The note has to say what the label COSTS in this encoding. Report the
        # length of the string instead and it is out by a factor of two.
        "the note about a missing label reports its cost in the wrong encoding",
        "internal/format/txt/txt.go",
        "core.A(\"Length\", codec.Cost(int64(len(line)))), core.A(\"Bytes\", r.Bytes)),",
        "core.A(\"Length\", int64(len(line))), core.A(\"Bytes\", r.Bytes)),",
        "TestALabelThatWillNotFitSaysWhatItWouldCost",
    ),
    (
        # Whether the label fits is a question about characters, not about file
        # bytes. Ask it in file bytes and a UTF-16 file writes a label twice
        # the size of the room it has.
        "the label is measured against the file size rather than what fits in it",
        "internal/format/md/md.go",
        "if int64(len(line)) <= m.source {",
        "if int64(len(line)) <= r.Bytes {",
        "TestALabelThatWillNotFitSaysWhatItWouldCost",
    ),
    (
        # Saying nothing and saying utf-8 out loud have to be the same file. Give
        # the default a mark and every hash in the golden file moves, which is a
        # breaking change dressed as a new setting.
        "the default encoding starts writing a byte order mark",
        "internal/format/textenc/textenc.go",
        "func Default() Codec { return Codec{name: UTF8} }",
        "func Default() Codec { return Codec{name: UTF8, bom: true} }",
        "TestTheDefaultEncodingIsTheBytesTheseFormatsAlwaysWrote",
    ),
    (
        # The content budget is file bytes divided by the width of a character,
        # which is only the same as "how much prose fits" while the vocabulary
        # is ASCII. One accented word is the day that stops being true.
        "the filler vocabulary stops being ASCII, and the width arithmetic stops holding",
        "internal/format/txt/txt.go",
        '"account", "after", "again", "amount", "answer", "before", "between",',
        '"acc\u00f3unt", "after", "again", "amount", "answer", "before", "between",',
        "TestTheTextGeneratorsWriteOnlyASCII",
    ),
    (
        # Look the value up and then ignore what came back, keeping every
        # symbol in use so the tree still builds. Every layout comes out as
        # one record per line, at the right size, parsing everywhere - the
        # file is simply not the one that was ordered.
        "the json layout is read and then ignored",
        "internal/format/jsonfile/style.go",
        "\ts, known := styles[v]",
        "\ts, known := styles[RecordPerLine]",
        "TestAJSONDocumentIsLaidOutTheWayItWasOrdered",
    ),
    (
        # A minified document that ends in a newline is not minified. It still
        # parses, it is still the right size, and the one thing somebody asked
        # for is gone.
        "a minified document ends with a newline after all",
        "internal/format/jsonfile/style.go",
        "\ttailLast: `\"}]`,",
        "\ttailLast: \"\\\"}]\\n\",",
        "TestAJSONDocumentIsLaidOutTheWayItWasOrdered",
    ),
    (
        # The floor is worked out for the DEFAULT layout rather than the one
        # asked for, so an indented document is offered a size it cannot hold.
        "the json minimum is worked out for the wrong layout",
        "internal/format/jsonfile/json.go",
        "\tr := records{s: s}",
        "\tr := records{s: defaultStyle()}",
        "TestEachJSONLayoutAnswersForItsOwnMinimum",
    ),
    (
        # Saying nothing has to be the layout this format always wrote. Move
        # the default and every hash anybody recorded moves with it.
        "the default json layout stops being the one it always was",
        "internal/format/jsonfile/style.go",
        "func defaultStyle() style { return recordPerLine }",
        "func defaultStyle() style { return minified }",
        "TestTheDefaultLayoutIsTheBytesJSONAlwaysWrote",
    ),
    (
        # The checker stops caring how many lines a record per line document
        # has, so it blesses a minified file told it is one. Every case that
        # hands it the RIGHT layout still passes - only the canary handing it
        # the wrong one can see this.
        "the json checker stops counting lines, and blesses a layout the file is not",
        "internal/oracle/strict.py",
        "        if newlines != len(doc) + 2:",
        "        if newlines != len(doc) + 2 and False:",
        "TestTheJSONCheckerRefusesALayoutTheFileIsNot",
    ),
    (
        # The clamp that keeps an argument to IntN positive stops clamping.
        # At 800 by 600 nothing changes, because every argument is already
        # positive - which is exactly why this needed a guard: the defect is
        # invisible at the only size this format could produce until now.
        # Measured 2026-09-08 by running it: IntN(0) and IntN(-30) panic with
        # "invalid argument to IntN".
        "an SVG canvas smaller than the margins its shapes assume",
        "internal/format/svgfile/svg.go",
        "\tif n < 1 {",
        "\tif n < 1 && false {",
        "TestASvgCanvasSmallerThanItsOwnMarginsStillDraws",
    ),
    (
        # The clamp that keeps a shape's lower edge out of the strip the label
        # sits in stops clamping. Inert at 800 by 600 - the widest shape lands
        # two units short of the strip - so only a small canvas can see it.
        # This is the defect the strip was added for in the first place: about
        # one shape in ten landed on the label, 11.0% and 10.3% measured.
        "a shape reaches past the drawing into the label strip",
        "internal/format/svgfile/svg.go",
        "func fit(extent, room int) int { return min(extent, room) }",
        "func fit(extent, room int) int { _ = room; return extent }",
        "TestNoSvgShapeReachesIntoTheStripTheLabelSitsIn",
    ),
    (
        # The root element goes back to announcing 800 by 600 whatever was
        # ordered. The file is still well formed, still the exact size asked
        # for, and still opens - it simply lies about how big the drawing is,
        # which is the whole of what this setting does.
        "the root element reports the default size whatever was asked for",
        "internal/format/svgfile/svg.go",
        "func rootOpen(w, h int) string {",
        "func rootOpen(w, h int) string {\n\tw, h = defaultWidth, defaultHeight",
        "TestASvgCanvasSmallerThanItsOwnMarginsStillDraws",
    ),
    (
        # A drawing too short to show the label claims to carry one anyway.
        # Nothing about the bytes changes shape, the size stays exact, and the
        # manifest quietly says label_embedded on a file where the label is
        # painted off the canvas. Rule 6, in the form that is hardest to see.
        "a drawing too short for the label claims to carry one",
        "internal/format/svgfile/svg.go",
        "func labelFits(h int) bool { return h > TextBand }",
        "func labelFits(h int) bool { return h > TextBand || true }",
        "TestASvgTooShortForTheLabelSaysSo",
    ),
    (
        # Leaving a dimension out stops meaning the same as writing its
        # declared default. Two journeys through the parser, the registry and
        # the generator, allowed to disagree by accident - and every recipe
        # written before this setting existed takes the first one.
        "silence about the SVG canvas stops meaning the declared default",
        "internal/format/imagedim/imagedim.go",
        "\t\treturn fallback, nil",
        "\t\treturn fallback + 1, nil",
        "TestSayingTheSvgCanvasOutLoudChangesNoByte",
    ),
    # --- XML carries an encoding, and its declaration has to agree with its
    #     bytes (2026-09-08, docs/XML-ENCODING-2026-09-08.md) ---
    (
        "the XML declaration goes back to naming one encoding whatever the bytes are",
        "internal/format/xmlfile/xml.go",
        "\treturn declarationUTF16",
        "\treturn declarationUTF8",
        "TestTheXMLDeclarationNamesTheEncodingTheBytesAreIn",
    ),
    (
        "XML stops requiring a byte order mark in a wide encoding",
        "internal/format/xmlfile/xml.go",
        "\tif c.Name() == textenc.UTF8 || c.HasBOM() {",
        "\tif true {",
        "TestXMLRefusesAWideEncodingWithoutAMark",
    ),
    (
        "the smallest XML file stops depending on the encoding it is written in",
        "internal/format/xmlfile/xml.go",
        "\treturn c.Cost(source) + int64(len(c.Preamble()))",
        "\treturn source",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        "the byte order mark is planned for XML and never written",
        "internal/format/xmlfile/xml.go",
        "core.WriteAll(w, m.codec.Preamble())",
        "core.WriteAll(w, nil)",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        "XML fills its document in file bytes rather than in characters",
        "internal/format/xmlfile/xml.go",
        "m.source-int64(len(head))",
        "p.Bytes-int64(len(head))",
        "TestATextFileIsTheEncodingItDeclares",
    ),
    (
        "the XML label note reports what the comment costs in UTF-8",
        "internal/format/xmlfile/xml.go",
        "core.A(\"Length\", codec.Cost(int64(len(line))))),",
        "core.A(\"Length\", int64(len(line)))),",
        "TestALabelThatWillNotFitSaysWhatItWouldCost",
    ),
    (
        "the structural check stops comparing the declaration with the bytes",
        "internal/oracle/strict.py",
        "    if opening.group(1) != want:",
        "    if opening.group(1) != want and False:",
        "TestTheStructuralCheckRefusesADeclarationThatDisagreesWithTheBytes",
    ),
    # --- a drawn size is snapped to one the format can write, and the move is
    #     reported (2026-09-08, O190) ---
    (
        "a drawn size goes back to being used whether the format can write it or not",
        "internal/engine/drawsizes.go",
        "\t\tt.Sizes[i] = got",
        "\t\tt.Sizes[i] = want",
        "TestASizeTheFormatCannotWriteIsMovedInsideTheRange",
    ),
    (
        "a size is moved to fit the format and nothing records that it was",
        "internal/engine/drawsizes.go",
        "\t\tt.SizeMoved[i] = got != want",
        "\t\tt.SizeMoved[i] = false",
        "TestAMovedSizeIsReported",
    ),
    (
        "the note about a moved size is attached to every file of a range",
        "internal/engine/plantarget.go",
        "\t\tif idx < len(t.SizeMoved) && t.SizeMoved[idx] {",
        "\t\tif idx < len(t.Sizes) {",
        "TestAMovedSizeIsReported",
    ),
    (
        "a range starting under what the format can do is no longer refused",
        "internal/engine/drawsizes.go",
        "\tif t.Range.Min < floor {",
        "\tif t.Range.Min < floor && false {",
        "TestARangeIsJudgedByItsLowEndRatherThanByWhatWasDrawn",
    ),
    (
        "snapping walks past the top of the range instead of stopping at it",
        "internal/engine/drawsizes.go",
        "\tfor at := from; at <= limit; {",
        "\tfor at := from; ; {",
        "TestARangeHoldingNoWritableSizeIsRefusedBeforeAnyFile",
    ),
    (
        "snapping starts from somewhere that depends on how many files were asked for",
        "internal/engine/drawsizes.go",
        "\t\tgot, err := firstWritable(desc, req, want, t.Range.Max)",
        "\t\tgot, err := firstWritable(desc, req, want+int64(len(t.Sizes)), t.Range.Max)",
        "TestSnappingStillLeavesTheEarlierFilesAlone",
    ),
    # --- HTML is a whole page or only the blocks that sit inside one
    #     (2026-09-08, docs/HTML-STRUCTURE-2026-09-08.md) ---
    # Repointed 2026-10-14, when html.go was split into settings.go, page.go,
    # blocks.go, catalogue.go and characters.go - the defects are the same.
    (
        "a fragment gets the skeleton of a whole page in front of it",
        "internal/format/htmlfile/page.go",
        '\tif s.shape == structureDocument {\n\t\tp = m[headOpen]',
        '\tif true {\n\t\tp = m[headOpen]',
        "TestAFragmentCarriesNoSkeletonAndAPageCarriesOne",
    ),
    (
        "a fragment still closes a body and a document it does not have",
        "internal/format/htmlfile/content.go",
        "\tif s.shape != structureDocument {\n\t\treturn \"\"\n",
        "\tif false {\n\t\treturn \"\"\n",
        "TestAFragmentCarriesNoSkeletonAndAPageCarriesOne",
    ),
    (
        "every shape of HTML inherits the floor of a whole page",
        "internal/format/htmlfile/html.go",
        "\treturn int64(len(s.prologue())) + c.openingBytes()",
        "\treturn int64(len(s.spelling()[headOpen]+s.spelling()[headClose])) + c.openingBytes()",
        "TestEachHTMLShapeAnswersForItsOwnMinimum",
    ),
    (
        "a fragment is given a title, which belongs to a head it has not got",
        "internal/format/htmlfile/html.go",
        "\thead, extra := plain, int64(0)",
        "\thead, extra := \"<title>\"+label+\"</title>\"+plain, int64(0)",
        "TestTheLabelIsVisibleInBothHTMLShapes",
    ),
    (
        "an HTML recipe that says nothing gets a fragment instead of a page",
        "internal/format/htmlfile/settings.go",
        "choice(props, settingStructure, structureDocument, structureFragment)",
        "choice(props, settingStructure, structureFragment, structureDocument)",
        "TestTheDefaultHTMLShapeIsTheBytesItAlwaysWrote",
    ),
    (
        "the structural check stops being told which HTML shape to expect",
        "internal/oracle/strict.py",
        '    shape = settings.get("structure", "document")',
        '    shape = "document"',
        "TestTheStructuralCheckIsToldWhichHTMLShapeToExpect",
    ),
    # --- HTML nesting, characters, elements and bom (2026-10-14,
    #     docs/HTML-2-2026-10-14.md) ---
    (
        "the floor of an html page carries a byte of text nobody needed",
        "internal/format/htmlfile/html.go",
        "\treturn int64(len(s.prologue())) + c.openingBytes() + c.linksBytes() + c.shortestRecord() + c.closingBytes()",
        "\treturn int64(len(s.prologue())) + c.openingBytes() + c.linksBytes() + c.shortestRecord() + c.closingBytes() + 1",
        "TestEveryHTMLSettingAnswersForItsOwnMinimum",
    ),
    (
        "the floor of an html page forgets the levels it has to close",
        "internal/format/htmlfile/html.go",
        "\treturn int64(len(s.prologue())) + c.openingBytes() + c.linksBytes() + c.shortestRecord() + c.closingBytes()",
        "\treturn int64(len(s.prologue())) + c.openingBytes() + c.linksBytes() + c.shortestRecord() + int64(len(c.documentClose()))",
        "TestEveryHTMLSettingAnswersForItsOwnMinimum",
    ),
    (
        "html nesting opens half the levels it was asked for",
        "internal/format/htmlfile/content.go",
        "\tfor k := 0; k < s.nesting && out.err == nil; k++ {",
        "\tfor k := 0; k < s.nesting/2 && out.err == nil; k++ {",
        "TestNestingPutsTheContentThatManyLevelsDeep",
    ),
    (
        "the html label moves inside the levels, where a reader drops it",
        "internal/format/htmlfile/html.go",
        "\tif err := core.WriteAll(w, []byte(head)); err != nil {\n\t\treturn err\n\t}\n\tif err := c.writeOpening(ctx, w, in); err != nil {\n\t\treturn err\n\t}\n",
        "\tif err := c.writeOpening(ctx, w, in); err != nil {\n\t\treturn err\n\t}\n\tif err := core.WriteAll(w, []byte(head)); err != nil {\n\t\treturn err\n\t}\n",
        "TestNestingPutsTheContentThatManyLevelsDeep",
    ),
    (
        "the manifest records no html nesting whatever was asked",
        "internal/format/htmlfile/settings.go",
        "\t\tsettingNesting:     s.nesting,",
        "\t\tsettingNesting:     0,",
        "TestEveryHTMLSettingIsRecordedInTheManifest",
    ),
    (
        "an html recipe that says nothing gets a level of nesting",
        "internal/format/htmlfile/settings.go",
        "\tif !ok || v == \"\" {\n\t\treturn 0, nil",
        "\tif !ok || v == \"\" {\n\t\treturn 1, nil",
        "TestTheDefaultHTMLSettingsAreTheBytesItAlwaysWrote",
    ),
    (
        "the structural check stops counting the html levels",
        "internal/oracle/strict.py",
        "    if opened != want or closed != want:",
        "    if False:",
        "TestTheStructuralCheckIsToldEveryHTMLSetting",
    ),
    (
        "the catalogue of every element loses search",
        "internal/format/htmlfile/catalogue.go",
        "<search><form>",
        "<div><form>",
        "TestEveryElementOfTheStandardIsInTheCatalogue",
    ),
    (
        "a fragment of every element leaves eight out without a word",
        "internal/format/htmlfile/html.go",
        "\tif s.every && s.shape == structureFragment {",
        "\tif false {",
        "TestEveryElementOfTheStandardIsInTheCatalogue",
    ),
    (
        "the catalogue of every element fetches its picture from a network",
        "internal/format/htmlfile/catalogue.go",
        "\tsmallPicture = \"data:image/png;base64,",
        "\tsmallPicture = \"https://example.com/p.png#",
        "TestEveryElementOfTheStandardIsInTheCatalogue",
    ),
    (
        "the arabic paragraph loses its direction",
        "internal/format/htmlfile/characters.go",
        "`<p lang=\"ar\" dir=\"rtl\">`",
        "`<p lang=\"ar\">`",
        "TestInternationalCarriesEveryScriptAndASCIICarriesNone",
    ),
    (
        "an ascii html page carries the paragraphs in other scripts",
        "internal/format/htmlfile/settings.go",
        "\t\ts.international = v == charactersInternational",
        "\t\ts.international = v != \"\"",
        "TestInternationalCarriesEveryScriptAndASCIICarriesNone",
    ),
    (
        "the html byte order mark lands where it was not ordered",
        "internal/format/htmlfile/page.go",
        "\tif s.bom {",
        "\tif !s.bom {",
        "TestTheHTMLByteOrderMarkIsThereOnlyWhenOrdered",
    ),
    (
        "the command line names a damage setting and stops saying what it may be",
        "internal/cli/damagecmd.go",
        "\t\tfmt.Fprintf(out, \"  %-14s %s\\n\", p.Name, p.Allowed())",
        "\t\tfmt.Fprintf(out, \"  %-14s %s\\n\", p.Name, \"\")",
        "TestTheCommandLineSaysWhatEveryDamageTakes",
    ),
    (
        "the smallest file a damage announces stops following its declaration",
        "internal/cli/damagecmd.go",
        "\treturn d.Floor(d.Defaults())",
        "\treturn 1",
        "TestTheSmallestFileTheDamageCommandAnnouncesIsOneARunAccepts",
    ),
    (
        "the machine readable damage list keeps the names and drops the settings",
        "internal/cli/damagecmd.go",
        "\t\tSmallestFileBytes: defaultFloor(d), Parameters: params,",
        "\t\tSmallestFileBytes: defaultFloor(d), Parameters: params[:0],",
        "TestTheMachineReadableDamageListCarriesTheWholeDeclaration",
    ),
    (
        "a command joins the dispatch and nobody adds it to the list watching --help",
        "internal/cli/commands.go",
        "\t\t\tVerb: \"formats\", Summary:",
        "\t\t\tVerb: \"listing\", Summary:",
        "TestEveryCommandTakingItsOwnArgumentsIsWatchedForHelp",
    ),
    (
        "the format list answers about the first of two names instead of refusing",
        "internal/cli/formats.go",
        "\twanted, ok := atMostOneName(leading, fs, errOut)\n\tif !ok {\n\t\treturn ExitUsage\n\t}",
        "\twanted, ok := atMostOneName(leading, fs, errOut)\n\tif !ok && false {\n\t\treturn ExitUsage\n\t}",
        "TestAListingCommandRefusesANameItCannotAnswerAbout",
    ),
    (
        "the preset list swallows a name it has no use for",
        "internal/cli/presetcmd.go",
        "\tif extra := fs.Args(); len(extra) > 0 {",
        "\tif extra := fs.Args(); len(extra) > 0 && false {",
        "TestAListingCommandRefusesANameItCannotAnswerAbout",
    ),
    (
        "a package below both surfaces writes a setting in command line spelling",
        "internal/damage/refusals.go",
        "core.Says(\"damage.UnknownInsteadRemove\", \"remove the damage line\")",
        "core.Says(\"damage.UnknownInsteadRemove\", \"remove --damage from this target\")",
        "TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling",
    ),
    # The two entries about the column of names (a name dropped from the
    # hand written list, a screen never told the column's width) went with
    # the column on 2026-09-22: a name stands over its box, and there is no
    # list and no width to drop anything from.
    (
        # The other half of the stacked form: the box indented under its name
        # reads as a child of the name rather than as the value it holds.
        "a field's box is indented under its name",
        "internal/gui/parts/field.go",
        "\tbody := FieldStack(pieces...)",
        "\tbody := FieldStack(pieces[0], Padded(GapLabel, pieces[1]))",
        "TestANameStandsOverItsBoxOnOneEdgeWithEveryOther",
    ),
    # --- the line at the foot of the form (2026-09-14, step 3 of the rework) ---
    (
        # The count on the line stops being the count on the form. Off by one
        # rather than gone, so a guard that only asked whether the line says
        # SOMETHING would stay green.
        "the line counts one file more than the form asks for",
        "internal/gui/window/runline.go",
        "\treturn text.RunLine(int(sum.files), sum.totalText(), sum.formats, l.where(dir))",
        "\treturn text.RunLine(int(sum.files)+1, sum.totalText(), sum.formats, l.where(dir))",
        "TestTheLineSaysWhatTheFormComesTo",
    ),
    (
        # A menu stops reporting its change, which is what every menu did
        # until this step: the line goes on naming the format chosen when the
        # screen was built. The box and the switch still report, so nothing
        # typed would show it.
        "a menu stops telling the runner it changed, so the line names the format from before",
        "internal/gui/parts/fields.go",
        "\t\tcase *Chooser:\n\t\t\tchainOnce(&it.OnChanged, &it.reports, setting, report)\n",
        "\t\tcase *Chooser:\n\t\t\tchainOnce(&it.OnChanged, &it.reports, setting, func(string, string) {})\n",
        "TestTheLineFollowsTheMenu",
    ),
    (
        # A form that does not settle keeps the line of the last one that
        # did, so a count stands on the bar for a run that will be refused.
        "the line keeps a stale summary when the form stops settling",
        "internal/gui/window/run.go",
        "\t\tshowOn(r.status, r.line.fallback(dir))\n\t\treturn\n",
        "\t\t_ = dir\n\t\treturn\n",
        "TestTheLineFallsBackToTheDestinationWhenTheFormDoesNotSettle",
    ),
    (
        # A range is added up as if its sizes were stated, so the line says
        # one number about files whose sizes have not been drawn.
        "a range is summed as though its sizes were already drawn",
        "internal/gui/window/runsummary.go",
        "\t\ts.lower(drawn)\n",
        "\t\ts.lower(stated)\n",
        "TestTheLineSaysBetweenForARange",
    ),
    (
        # The preview stops saying that nothing exists yet, so an exact line
        # reads as a report of files that were written.
        "the preview's line stops saying that nothing was written",
        "internal/gui/window/run.go",
        "\tsaid := r.line.said(exactly(planned), opt.OutDir) + text.AndNothingWrittenYet()",
        "\tsaid := r.line.said(exactly(planned), opt.OutDir)",
        "TestAPreviewMakesTheLineExact",
    ),
    (
        # The other half of the same guard: the disk is asked and the answer
        # thrown away, so the destination never carries the room left.
        "the preview measures the disk and tells the line nothing",
        "internal/gui/window/run.go",
        "\tif room.known {\n\t\tr.line.measured(opt.OutDir, room.free)\n\t}\n\tsaid :=",
        "\tif room.known && false {\n\t\tr.line.measured(opt.OutDir, room.free)\n\t}\n\tsaid :=",
        "TestAPreviewMakesTheLineExact",
    ),
    (
        # A change of the form stops putting the summary back, so what a
        # press said about a form that no longer exists stays on the line -
        # the one way flag this step removed, back again.
        "a change of the form leaves what a press said standing",
        "internal/gui/window/runrefuse.go",
        "\tlineFrom(r, targets, opt, err)\n\t// Only this box, in both directions.",
        "\t_, _ = targets, opt\n\t// Only this box, in both directions.",
        "TestAChangeOfTheFormPutsTheLineBackOverWhatAPressSaid",
    ),
    (
        # Step 4 of the window rework, 2026-09-15. The strip goes through the
        # same column and indent as a screen's title, so the two share an edge
        # by construction - and this is the construction taken away: the strip
        # on the column's edge, the title sixteen pixels in.
        "the strip stands on the column's edge rather than on the words' edge",
        "internal/gui/parts/tabs.go",
        "container.New(readableWidth{}, Indented(strip))",
        "container.New(readableWidth{}, strip)",
        "TestTheStripStandsOnTheEdgeTheScreenReadsFrom",
    ),
    (
        # The one vocabulary rule the other way up: the old title back at the
        # top and the tab's word under it as the explanation.
        "a screen's title and the sentence under it swap places",
        "internal/gui/window/generate.go",
        "parts.Titled(text.TabOneTarget(), text.SubtitleGenerate())",
        "parts.Titled(text.SubtitleGenerate(), text.TabOneTarget())",
        "TestAScreenIsTitledWithTheWordOnItsTab",
    ),
    (
        # Somebody who chose a screen with Enter is using the keyboard, so the
        # first field is marked for them. Placed quietly, the way a press
        # places it, they would have no idea where the next key goes.
        "a choice made from the keyboard places the keyboard quietly",
        "internal/gui/window/open.go",
        "\t\tfocusFirst(item.ID, byKeyboard)",
        "\t\tfocusFirst(item.ID, false)",
        "TestEnterOnAWordChoosesItsScreenAndHandsTheKeyboardOnVisibly",
    ),
    (
        # The toolkit's strip could not be worked from the keyboard at all,
        # and this is that state put back for the key that matters most.
        "Enter on a word of the strip chooses nothing",
        "internal/gui/parts/tabs.go",
        "\tcase fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:\n\t\tw.strip.choose(w.index, true)",
        "\tcase fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:\n\t\t_ = w.index",
        "TestEnterOnAWordChoosesItsScreenAndHandsTheKeyboardOnVisibly",
    ),
    (
        # Automatic activation, which the web also offers for a row of tabs
        # and which does not fit a strip whose choice moves the keyboard away:
        # every arrow would leave the strip.
        "the right arrow chooses a screen instead of moving along the strip",
        "internal/gui/parts/tabs.go",
        "\tcase fyne.KeyRight:\n\t\tw.strip.focusWord(w.index + 1)",
        "\tcase fyne.KeyRight:\n\t\tw.strip.choose(w.index+1, true)",
        "TestTheArrowsMoveTheKeyboardAlongTheStripWithoutChoosing",
    ),
    (
        # The same defect the menu and the switch had, in the strip: a press
        # takes the keyboard loudly, and on the chosen word nothing moves it
        # on, so the mark stays.
        "a press on a word of the strip draws the keyboard mark",
        "internal/gui/parts/tabs.go",
        "\t\tw.from.Quietly(func() { c.Focus(w) })",
        "\t\tc.Focus(w)",
        "TestAPressOnAWordChoosesWithoutDrawingTheKeyboardMark",
    ),
    (
        # O205 in the strip: the event arrives, the flag is set, and the
        # picture does not change. Only a guard reading the drawn colour sees
        # it.
        "the pointer over a word of the strip changes nothing on the screen",
        "internal/gui/parts/tabs.go",
        "\tif r.word.chosen || r.word.hovered {\n\t\tr.text.Color",
        "\tif r.word.chosen {\n\t\tr.text.Color",
        "TestAWordUnderThePointerIsDrawnAtFullStrength",
    ),
    (
        # The strip names one screen and shows another. The toolkit's tabs
        # did the showing inside their renderer, and ours does it in choose -
        # so this is the half of choose that a guard reading the strip alone
        # would never miss.
        "the chosen screen is named and not shown",
        "internal/gui/parts/tabs.go",
        "\tt.current = i\n\tt.items[i].Content.Show()",
        "\tt.current = i",
        "TestEnterOnAWordChoosesItsScreenAndHandsTheKeyboardOnVisibly",
    ),
    (
        # Every screen shown at once under the first word. The strip is what
        # hides the three not chosen, and the guard asks the tree rather than
        # the strip which screens are visible.
        "every screen is shown at once when the window opens",
        "internal/gui/parts/tabs.go",
        "\t\t} else {\n\t\t\ttab.Content.Hide()",
        "\t\t} else {\n\t\t\ttab.Content.Show()",
        "TestTheWindowOpensOnTheGenerateScreen",
    ),
    (
        # The window is set in Inter, and this is the theme quietly handing
        # the regular weight back to the toolkit. Every screen would still
        # draw, in Noto Sans, and nothing but a width would say so.
        "the regular weight falls back to the toolkit's face",
        "internal/gui/parts/typeface.go",
        "\t}\n\treturn interRegular\n}",
        "\t}\n\treturn o.Theme.Font(style)\n}",
        "TestTheWindowIsSetInInter",
    ),
    (
        "the bold weight falls back to the toolkit's face",
        "internal/gui/parts/typeface.go",
        "\tcase style.Bold:\n\t\treturn interBold",
        "\tcase style.Bold:\n\t\treturn o.Theme.Font(style)",
        "TestTheWindowIsSetInInter",
    ),
    (
        # Inter ships here in two weights and no italic. An italic answered
        # with the upright is a style silently drawn as another one.
        "an italic is answered with the upright Inter",
        "internal/gui/parts/typeface.go",
        "\tcase style.Monospace || style.Italic || style.Symbol:",
        "\tcase style.Monospace || style.Symbol:",
        "TestTheStylesTheWindowNeverDrawsKeepTheToolkitsFace",
    ),
    (
        # The package that embeds the font stops saying so, and the running
        # binary has no other way to learn it: both binaries are one module.
        # Announced under the wrong name rather than not at all: dropping the
        # call leaves the import unused and the tree does not compile, which
        # proves nothing (trap 4 in CLAUDE.md).
        "the font package announces itself under a name the registry does not know",
        "internal/gui/font/carry.go",
        "\tlegal.Carrying(packagePath)",
        "\tlegal.Carrying(packagePath + \"/typeface\")",
        "TestEveryPackageOfOursThatEmbedsSomebodyElsesBytesAnnouncesItself",
    ),
    (
        # The command line binary takes the window's font along. Nothing
        # structural stops an import like this, and tfg license would start
        # naming a font in a binary the notices say has none.
        "the command line binary links the window's font",
        "cmd/tfg/main.go",
        '\t"github.com/donislawdev/TestingFilesGenerator/internal/cli"',
        '\t"github.com/donislawdev/TestingFilesGenerator/internal/cli"\n\t_ "github.com/donislawdev/TestingFilesGenerator/internal/gui/font"',
        "TestTheCommandLineLinksNoBytesOfSomebodyElsesThatWeEmbed",
    ),
    (
        # The registry misspells this project's module, so every entry of our
        # own is answered as somebody else's and drops off every list.
        "the registry spells this project's module wrong",
        "internal/legal/legal.go",
        'const ourModule = "github.com/donislawdev/TestingFilesGenerator"',
        'const ourModule = "github.com/donislawdev/TestingFileGenerator"',
        "TestEveryAssetOfOurOwnIsSpelledAsGoListSpellsIt",
    ),
    (
        # Half of the font ships with no entry accounting for it. Until
        # 2026-09-15 this guard skipped our own module and would have passed.
        "a font file of ours ships with nothing in the registry accounting for it",
        "internal/legal/assets.go",
        '"Inter-Regular.ttf", "Inter-Bold.ttf"',
        '"Inter-Regular.ttf"',
        "TestEveryFileEmbeddedFromSomebodyElseIsAccountedFor",
    ),
    (
        # A file of our own work vanishes from the list of our own work, and
        # the guard has to say whose it is rather than assume.
        "our own icon is on neither list",
        "internal/guard/embeddedassets_test.go",
        '\t"github.com/donislawdev/TestingFilesGenerator/internal/gui/icon chickpea.png": true,',
        '\t// "github.com/donislawdev/TestingFilesGenerator/internal/gui/icon chickpea.png": true,',
        "TestEveryFileEmbeddedFromSomebodyElseIsAccountedFor",
    ),
    (
        # An entry of ours is answered by module after all, and both binaries
        # are one module - so the command line binary carries a font.
        "an entry of ours is attached to every binary",
        "internal/legal/carried.go",
        "\t\treturn ours[asset.Package]\n\t}",
        "\t\treturn true\n\t}",
        "TestTheSBOMKeepsTheTwoBinariesApart",
    ),
    (
        # A part the catalogue stops vouching for: the star beside a name is
        # drawn as a state of Fields, and the registry no longer says so.
        # An exemption that outlived its file. The stale check below it in
        # the guard asks the registry the same question; until 2026-09-16
        # nothing asked ownWork, so a key for a file that had gone would have
        # exempted whatever landed at that path next - named by an outside
        # review of #103.
        "an ownWork exemption names a file no build embeds",
        "internal/guard/embeddedassets_test.go",
        "var ownWork = map[string]bool{\n",
        "var ownWork = map[string]bool{\n\t\"github.com/donislawdev/TestingFilesGenerator/internal/gui/icon nothing.png\": true,\n",
        "TestEveryFileEmbeddedFromSomebodyElseIsAccountedFor",
    ),
    (
        "a type of parts is on no list of the catalogue",
        "internal/gui/catalogue/fields.go",
        'Covers: []string{"FieldSaying", "CellSaying", "FieldStack", "RequiredMark", "Table", "ToggleSaying"}',
        'Covers: []string{"FieldSaying", "CellSaying", "FieldStack", "Table", "ToggleSaying"}',
        "TestTheCatalogueCoversEveryPartAndNothingElse",
    ),
    # --- a picture is read by its text chunks, and they are still read (2026-09-16) ---
    (
        # The text chunk stops being read, so an address stamped into a
        # picture leaves unread - the narrowing turned into "pictures are
        # never read".
        "a picture's text chunk is no longer read for private content",
        "internal/guard/privatepng_test.go",
        "\t\treturn []string{string(bytes.ReplaceAll(data, []byte{0}, []byte{' '}))}, nil",
        '\t\treturn []string{""}, nil',
        "TestAnAddressInAPictureTextChunkIsStillFound",
    ),
    # --- a picture is read whole or refused (2026-09-16, CodeRabbit on #105) ---
    (
        # The read stops AT the room instead of one byte past it: a stream
        # longer than the room comes back exactly the room long, fits the
        # budget, and the rest leaves unread - the silent cut the review named.
        "a text chunk longer than the room is cut there rather than refused",
        "internal/guard/privatepng_test.go",
        "\t\tb, err = io.ReadAll(io.LimitReader(r, int64(room)+1))",
        "\t\tb, err = io.ReadAll(io.LimitReader(r, int64(room)))",
        "TestAPictureThisGuardCannotReadWholeIsRefused",
    ),
    (
        # The room is never spent, so the budget is per chunk again: two
        # chunks that each fit are read together past it.
        "the text budget is per chunk rather than per picture",
        "internal/guard/privatepng_test.go",
        "\t\tp.room -= len(piece)",
        "\t\tp.room -= 0",
        "TestAPictureThisGuardCannotReadWholeIsRefused",
    ),
    (
        # A stream that will not inflate - a wrong checksum, say - is read to
        # its end and handed over as if it were whole, which is what it did
        # until 2026-09-16 with "" and is the same silence either way.
        "compressed text that will not inflate is read as it stands rather than refused",
        "internal/guard/privatepng_test.go",
        '\t\treturn "", fmt.Errorf("has compressed text that will not inflate: %w", err)',
        "\t\treturn string(b), nil",
        "TestAPictureThisGuardCannotReadWholeIsRefused",
    ),
    (
        # A chunk running past the end of the file ends the reading quietly,
        # the way it did until 2026-09-16.
        "a chunk running past the end of the file ends the reading rather than refusing it",
        "internal/guard/privatepng_test.go",
        '\t\t\treturn "", fmt.Errorf("has a %s chunk of %d bytes that runs past the end of the file", kind, length)',
        "\t\t\tbreak",
        "TestAPictureThisGuardCannotReadWholeIsRefused",
    ),
    (
        # Whatever follows IEND is nobody's business again.
        "bytes after IEND are ignored rather than refused",
        "internal/guard/privatepng_test.go",
        "\t\t\tif len(rest) > 0 {",
        "\t\t\tif len(rest) > 0 && false {",
        "TestAPictureThisGuardCannotReadWholeIsRefused",
    ),
    # --- a heading laid out to nothing is refused, a hidden one is not measured (2026-09-16) ---
    (
        # The skip comes back: a heading the layout collapsed to nothing is
        # stepped over, and the guard passes on the strength of the others.
        "a heading laid out to nothing is stepped over rather than refused",
        "internal/guard/oneline_test.go",
        "\tif width = o.Size().Width; width == 0 {",
        "\tif width = o.Size().Width; width == 0 && false {",
        "TestAHeadingLaidOutToNothingIsRefusedAndAHiddenOneIsNotMeasured",
    ),
    (
        # Hiddenness stops counting: a heading nobody can see is refused for
        # the room it never had, which would make every folded section red.
        "a heading under something hidden is refused for the room it never had",
        "internal/guard/oneline_test.go",
        "\tif buried[o] {",
        "\tif buried[o] && false {",
        "TestAHeadingLaidOutToNothingIsRefusedAndAHiddenOneIsNotMeasured",
    ),
    # --- a title short of room ends in an ellipsis, and none of ours is (2026-09-16, O214) ---
    (
        # The cut comes off: a long title runs past its panel again, the way
        # canvas words did.
        "a title short of room runs past its panel instead of ending in an ellipsis",
        "internal/gui/parts/parts.go",
        "	label.Truncation = fyne.TextTruncateEllipsis",
        "	label.Truncation = fyne.TextTruncateOff",
        "TestATitleTooLongForItsRoomEndsInAnEllipsisInsideIt",
    ),
    (
        # A title wraps onto a second line instead - a sentence's answer,
        # which a name does not get.
        "a title short of room wraps onto a second line",
        "internal/gui/parts/parts.go",
        "	label.Wrapping = fyne.TextWrapOff",
        "	label.Wrapping = fyne.TextWrapWord",
        "TestATitleTooLongForItsRoomEndsInAnEllipsisInsideIt",
    ),
    (
        # A word of ours grows past its room: every screen's title is drawn
        # ten times over, so every one of them is cut on a shipped screen.
        "a screen's title is longer than the panel it heads",
        "internal/gui/parts/parts.go",
        "	return oneLine(text, theme.SizeNameSubHeadingText)",
        "	return oneLine(text+text+text+text+text+text+text+text+text+text, theme.SizeNameSubHeadingText)",
        "TestEveryTitleAndBlockNameFitsWhereItStands",
    ),
    # --- a first start opens as tall as the screens want (2026-09-16, O202) ---
    (
        # The want is ignored: a first start opens at the ceiling again, which
        # is the number measured once in August and typed in.
        "a first start ignores what the screens want and opens at the ceiling",
        "internal/gui/window/opening.go",
        "		size.Height = fyne.Min(wanted.Height, LargestOpening.Height)",
        "		size.Height = LargestOpening.Height",
        "TestAFirstStartOpensAsTallAsTheScreensWantInTheMiddle",
    ),
    (
        # The ceiling comes off: a want taller than somebody's screen opens
        # taller than it, with its bottom out of reach.
        "a first start opens taller than the ceiling when the screens want more",
        "internal/gui/window/opening.go",
        "		size.Height = fyne.Min(wanted.Height, LargestOpening.Height)",
        "		size.Height = wanted.Height",
        "TestAFirstStartOpensAsTallAsTheScreensWantInTheMiddle",
    ),
    (
        # The strip above the screens is left out of the want, so the first
        # screen scrolls from the first frame by the height of the tabs. The
        # call stays, multiplied by nought, so the import stays used - taking
        # it out broke the build and proved nothing (2026-09-21).
        "the first opening forgets the strip above the screens",
        "internal/gui/window/opening.go",
        "parts.AboveTheScreens(tabbed)+first.Unscrolled()+2*theme.Padding())",
        "parts.AboveTheScreens(tabbed)*0+first.Unscrolled()+2*theme.Padding())",
        "TestTheFirstOpeningShowsTheFirstScreenWholeAndNoMore",
    ),
    (
        # The canvas padding is forgotten, which is the eight pixels the first
        # version of this was short by - measured, not imagined.
        "the first opening forgets the padding the canvas keeps round the content",
        "internal/gui/window/opening.go",
        "parts.AboveTheScreens(tabbed)+first.Unscrolled()+2*theme.Padding())",
        "parts.AboveTheScreens(tabbed)+first.Unscrolled()+theme.Padding()-theme.Padding())",
        "TestTheFirstOpeningShowsTheFirstScreenWholeAndNoMore",
    ),
    (
        # A hundred pixels of slack under the first form: the band of nothing
        # O202 is about, back under a different number - and since 2026-09-21
        # the band the owner saw under the single batch screen, when the
        # height came from the tallest screen instead of the first.
        "the first opening is a hundred pixels taller than the first screen wants",
        "internal/gui/window/opening.go",
        "parts.AboveTheScreens(tabbed)+first.Unscrolled()+2*theme.Padding())",
        "parts.AboveTheScreens(tabbed)+first.Unscrolled()+2*theme.Padding()+100)",
        "TestTheFirstOpeningShowsTheFirstScreenWholeAndNoMore",
    ),
    # --- a table names its columns once, over the first row (2026-09-16) ---
    (
        # The header stops naming anything: every column gets the blank a
        # non field gets, so the table has rows and no names over them.
        "a table's header draws a blank over every column",
        "internal/gui/parts/table.go",
        "heads = append(heads, headingRow(f.Label, f.detail, t.fields.required[f.Setting]))",
        "heads = append(heads, Clear())",
        "TestATableNamesItsColumnsOnceOverTheFirstRow",
    ),
    (
        # The names are there and stand over the wrong columns: a blank in
        # front shifts every name one column to the right of its cells.
        "a table's names stand one column to the right of their cells",
        "internal/gui/parts/table.go",
        "return Row(heads...)",
        "return Row(append([]fyne.CanvasObject{Clear()}, heads...)...)",
        "TestATableNamesItsColumnsOnceOverTheFirstRow",
    ),
    (
        # A menu is made without the window's placeholder, so the toolkit puts
        # its own in when the menu is first drawn - the second width the two
        # row table showed (152 px over 140), and an English word in a window
        # speaking another language. Until 2026-10-06 this measured the
        # placeholder in menuWidth, which stopped proving anything once every
        # menu was made with a placeholder that drawing does not change.
        "a menu is made without the window's placeholder",
        "internal/gui/parts/ring.go",
        "\tc.PlaceHolder = text.PlaceholderChooseOne()\n",
        "",
        "TestAMenuIsOneWidthBeforeAndAfterItIsDrawn",
    ),
    (
        # Every row's cells slide one column to the right of the names: the
        # header stands where it stood and the first row does not.
        "a table's rows stand one column to the right of the names",
        "internal/gui/parts/table.go",
        "return Column(GapTight, append([]fyne.CanvasObject{Row(bodies...)}, areas...)...)",
        "return Column(GapTight, append([]fyne.CanvasObject{Row(append([]fyne.CanvasObject{Clear()}, bodies...)...)}, areas...)...)",
        "TestATableNamesItsColumnsOnceOverTheFirstRow",
    ),
    (
        # The other direction: the catalogue vouches for a part nobody has.
        "the catalogue covers a name parts does not export",
        "internal/gui/catalogue/words.go",
        'Covers: []string{"Divider", "ActionBar"}',
        'Covers: []string{"Dividers", "ActionBar"}',
        "TestTheCatalogueCoversEveryPartAndNothingElse",
    ),
    (
        # A fold is built open, so a "closed" state that forgets to close it
        # is a caption over the open one's picture - the defect the first
        # render of this catalogue had, put back.
        "two states of one entry draw the same picture",
        "internal/gui/catalogue/fields.go",
        "\t\t{\"closed\", func() fyne.CanvasObject {\n\t\t\tf := parts.NewFolding(\"Notes for the manifest\", nil, parts.Prose(\"inside the fold\"))\n\t\t\tf.Set(false)",
        "\t\t{\"closed\", func() fyne.CanvasObject {\n\t\t\tf := parts.NewFolding(\"Notes for the manifest\", nil, parts.Prose(\"inside the fold\"))\n\t\t\tf.Set(true)",
        "TestEveryCatalogueStateDrawsDifferently",
    ),
    (
        # The screen skips the first entry of the registry and looks complete.
        "the catalogue screen leaves an entry out",
        "internal/gui/catalogue/screen.go",
        "\tfor _, e := range entries {",
        "\tfor _, e := range entries[1:] {",
        "TestTheCatalogueScreenShowsTheWholeRegistry",
    ),
    (
        # The build with no window in it answers the catalogue flag with a
        # zero, which is a script being told the window opened.
        "a build with no window exits zero when asked for the catalogue",
        "internal/gui/run_nocgo.go",
        "func run(_ Launch, errOut io.Writer) int {\n\tfmt.Fprintln(errOut, text.NoWindowInThisBuild)\n\treturn 1\n}",
        "func run(launch Launch, errOut io.Writer) int {\n\tif launch.Catalogue {\n\t\treturn 0\n\t}\n\tfmt.Fprintln(errOut, text.NoWindowInThisBuild)\n\treturn 1\n}",
        "TestABuildWithNoWindowInItSaysSoAndKeepsStandardOutputEmpty",
    ),
    (
        # The catalogue drops off the list of stored scenes, and every other
        # scene goes on passing.
        # Renamed rather than removed, because removing the line leaves the
        # import unused and the tree does not compile, which proves nothing.
        "the catalogue is no longer a stored scene",
        "internal/guard/screenpixels_test.go",
        "\t\t{name: \"catalogue\", page: catalogue.Page},",
        "\t\t{name: \"parts\", page: catalogue.Page},",
        "TestTheCatalogueIsAStoredScene",
    ),
    (
        # O218 put back at the seam: the toolkit gave the window no window
        # and the loop is run anyway - on the guest that is the process that
        # lives forever with nothing on screen.
        "a window with no native window is run instead of refused",
        "internal/gui/opening.go",
        "\tif !Shown(w) {",
        "\tif !Shown(w) && false {",
        "TestAWindowTheToolkitGaveNoWindowIsRefusedAndNeverRun",
    ),
    (
        # The state read upside down: a zero handle passes as a window and a
        # real one is refused.
        "a zero native handle counts as a window",
        "internal/gui/opening.go",
        "\treturn handle != 0\n}",
        "\treturn handle == 0\n}",
        "TestAWindowTheToolkitGaveNoWindowIsRefusedAndNeverRun",
    ),
    (
        # Cannot tell taken as failed: a driver answering with a context this
        # code does not read would refuse a window that opened.
        "an unknown native context is refused",
        "internal/gui/opening.go",
        "\tif !answered {\n\t\treturn true\n\t}",
        "\tif !answered {\n\t\treturn false\n\t}",
        "TestAWindowTheToolkitGaveNoWindowIsRefusedAndNeverRun",
    ),
    (
        # The toolkit's own sentence about the driver dropped on the way to
        # the person - the refusal still stands, but says nothing about why.
        "the cause line is found and thrown away",
        "internal/gui/opening.go",
        "\t\t\treturn strings.TrimSpace(after)",
        "\t\t\treturn strings.TrimSpace(after)[:0]",
        "TestTheToolkitsOwnReasonReachesTheSentenceAndItsAbsenceDoesNotBreakIt",
    ),
    (
        # The sentence with a cause stops quoting it.
        "the refusal stops quoting what the toolkit said",
        "internal/gui/text/text.go",
        "The toolkit said: {{.Cause}}. ",
        "The toolkit said: . ",
        "TestTheToolkitsOwnReasonReachesTheSentenceAndItsAbsenceDoesNotBreakIt",
    ),
    (
        # ShowAndRun put back in front of the seam: the loop nothing ends
        # runs before the question is ever asked.
        "the window binary shows and runs before the seam",
        "internal/gui/run_cgo.go",
        "\tcode := OpenOrRefuse(w, a.Run, SecondAttemptFor(launch, errOut).Try, func(why error) {",
        "\tw.ShowAndRun()\n\tcode := OpenOrRefuse(w, a.Run, SecondAttemptFor(launch, errOut).Try, func(why error) {",
        "TestTheWindowBinaryOpensThroughTheRefusalSeam",
    ),
    (
        # The copy of the toolkit's log put second: a multi-writer stops at
        # the first writer that fails, and a windows-subsystem binary's
        # standard error is one, so the cause never reaches the copy.
        "the log copy is written after the stream that is not there",
        "internal/gui/run_cgo.go",
        "\tlog.SetOutput(io.MultiWriter(&said, log.Writer()))",
        "\tlog.SetOutput(io.MultiWriter(log.Writer(), &said))",
        "TestTheWindowBinaryOpensThroughTheRefusalSeam",
    ),
    (
        # An outside review of the pull request: the copy of the log goes
        # into SOME buffer, first, and the one the cause is read from stays
        # empty - the guard asked only for an address.
        "the log is copied into a buffer nobody reads",
        "internal/gui/run_cgo.go",
        "\tvar said bytes.Buffer\n\tlog.SetOutput(io.MultiWriter(&said, log.Writer()))",
        "\tvar said, other bytes.Buffer\n\tlog.SetOutput(io.MultiWriter(&other, log.Writer()))",
        "TestTheWindowBinaryOpensThroughTheRefusalSeam",
    ),
    (
        # A second change in the copy of the OpenGL binding, in a file the
        # compiler never reads - chosen so the entry costs seconds. Any
        # change in a file it does read recompiles the binding, the toolkit
        # and the guard binary, measured at 3 min 56 s, which the ceiling
        # would cap. The licence file is the copy's own, carried unchanged.
        "the copy of the binding carries a change its note does not describe",
        "third_party/go-gl-gl/LICENSE",
        "Copyright (c) 2014 Eric Woroshow",
        "Copyright (c) 2014 Eric Woroshow and others",
        "TestTheOpenGLBindingIsThePinnedModulePlusExactlyThePatch",
    ),
    (
        # The sum the note pins for the published version, changed by one
        # character. go.sum stopped carrying this module's sum when it was
        # replaced, so the note is where the pin lives now.
        "the note pins a different sum than the published module has",
        "third_party/go-gl-gl/PATCH.md",
        "`h1:IO5P06Pcj9K04d+l4nrf3c2U56+dAotIFG6u4P1wAHI=`",
        "`h1:IO5P06Pcj9K04d+l4nrf3c2U56+dAotIFG6u4P1wAHJ=`",
        "TestTheOpenGLBindingIsThePinnedModulePlusExactlyThePatch",
    ),
    (
        # The walk stops leaving a nested module alone: every guard reading
        # the shape of this module's code would be handed 2.3 MB of somebody
        # else's generated code.
        "a nested module is walked as a package of this one",
        "internal/guard/packages_test.go",
        "\t\t\t\treturn filepath.SkipDir\n\t\t\t}\n\t\t}",
        "\t\t\t\treturn nil\n\t\t\t}\n\t\t}",
        "TestANestedModuleIsNotAPackageOfThisModule",
    ),
    (
        # The second process starts a third: the flag it was started with
        # no longer says "this is the attempt", so a guest where the
        # renderer does not help either starts processes until something
        # else stops it.
        "the second process starts a third",
        "internal/gui/software.go",
        "\tif a.Launch.SoftwareGL {\n\t\treturn 0, alreadySoftware{}",
        "\tif a.Launch.SoftwareGL && false {\n\t\treturn 0, alreadySoftware{}",
        "TestTheSecondAttemptIsTakenOnlyWhereTheRendererCanBe",
    ),
    (
        # The flag dropped from the second process's arguments: it starts as
        # an ordinary window, the driver refuses again, and it starts a third.
        "the second process is started without the flag",
        "internal/gui/software.go",
        "code, err := a.Start(append(append([]string{}, a.Launch.Args...), SoftwareFlag))",
        "code, err := a.Start(append([]string{}, a.Launch.Args...))",
        "TestTheSecondAttemptIsTakenOnlyWhereTheRendererCanBe",
    ),
    (
        # A system the renderer does not ship for starts a second process
        # anyway, looking for a file nobody promised there.
        "the attempt is taken on a system nothing ships for",
        "internal/gui/software.go",
        "\tif a.GOOS != \"windows\" {\n\t\treturn 0, notShipped{a.GOOS}",
        "\tif a.GOOS != \"windows\" && false {\n\t\treturn 0, notShipped{a.GOOS}",
        "TestTheSecondAttemptIsTakenOnlyWhereTheRendererCanBe",
    ),
    (
        # The refusal loses the sentence about the missing file, which is
        # the one that tells the person what to put back.
        "the refusal says nothing about the file that is missing",
        "internal/gui/software.go",
        "\tcase errors.As(why, &beside):\n\t\treturn text.RendererNotBeside(beside.path)",
        "\tcase errors.As(why, &beside) && false:\n\t\treturn text.RendererNotBeside(beside.path)",
        "TestTheSecondAttemptIsTakenOnlyWhereTheRendererCanBe",
    ),
    (
        # The loader before the renderer: the loader imports the renderer
        # by name and finds nothing mapped, measured 2026-09-17.
        "the renderer's files are named loader first",
        "internal/gui/software.go",
        "\t\tfilepath.Join(exeDir, softwareDir, softwareRenderer),\n\t\tfilepath.Join(exeDir, softwareDir, softwareLoader),",
        "\t\tfilepath.Join(exeDir, softwareDir, softwareLoader),\n\t\tfilepath.Join(exeDir, softwareDir, softwareRenderer),",
        "TestTheRendererIsLoadedRendererFirstFromBesideTheProgram",
    ),
    (
        # The driver left unnamed: the renderer picks one itself and the
        # process dies in under three seconds on a machine with a card.
        # Windows only, like the file - the runner runs there.
        "the driver variable is set to nothing",
        "internal/gui/software_windows.go",
        "\tif err := os.Setenv(softwareDriver, softwareLLVMPipe); err != nil {",
        "\tif err := os.Setenv(softwareDriver, softwareLLVMPipe[:0]); err != nil {",
        "TestTheRendererIsLoadedRendererFirstFromBesideTheProgram",
    ),
    (
        # The seam throws the second process's exit code away.
        "the second process's exit code is replaced by one",
        "internal/gui/opening.go",
        "\t\tif why == nil {\n\t\t\treturn code\n\t\t}",
        "\t\tif why == nil {\n\t\t\treturn 1 + 0*code\n\t\t}",
        "TestAWindowTheToolkitGaveNoWindowIsRefusedAndNeverRun",
    ),
    (
        # The About screen stops saying the window is drawn in software.
        "the about screen says nothing about the software renderer",
        "internal/gui/window/about.go",
        "\tif h.SoftwareRendering() {",
        "\tif h.SoftwareRendering() && false {",
        "TestTheAboutScreenSaysWhenTheWindowIsDrawnInSoftware",
    ),
    (
        # The real window stops taking the second attempt: a guest with no
        # driver gets the refusal and never the renderer. Behind cgo, so
        # the guard reads the source.
        "the real window never takes the second attempt",
        "internal/gui/run_cgo.go",
        "SecondAttemptFor(launch, errOut).Try, func(why error) {",
        "func() (int, error) { return 0, alreadySoftware{} }, func(why error) {",
        "TestTheWindowBinaryOpensThroughTheRefusalSeam",
    ),
    (
        # The flag loads nothing.
        "a window asked for the renderer never loads it",
        "internal/gui/run_cgo.go",
        "\t\t\terr = LoadSoftwareRenderer(dir)",
        "\t\t\terr = notShipped{dir}",
        "TestTheWindowBinaryOpensThroughTheRefusalSeam",
    ),
    (
        # A spawn through the package the guard did not read until
        # 2026-09-17. The canary carries the case, and this entry proves
        # the canary reaches it: with os taken back out, the scanner
        # passes the canary's os.StartProcess and the canary says so.
        "os.StartProcess walks past the telemetry scanner again",
        "internal/guard/notelemetry_test.go",
        "\t\"os\": true,\n}",
        "\t\"os\": false,\n}",
        "TestTheTelemetryScannerRejectsCodeItMustReject",
    ),
    (
        # The spawn allowance forgives every file, not the one named: the
        # registry is read by one file's name whatever file is being read.
        # Until the second outside review of #109 this entry set the flag
        # for every file, which no longer opens anything - the operation is
        # matched by call and source, so a file that is not again.go would
        # still have to make again.go's exact spawn to be forgiven. This is
        # that file.
        "any file may start a process",
        "internal/guard/notelemetry_test.go",
        "\tspawn, spawns := spawnsAllowed[rel]",
        "\tspawn, spawns := spawnsAllowed[\"internal/gui/again.go\"]",
        "TestARegisteredFileIsForgivenOneOperationAndNoMore",
    ),
    (
        # The flag read and thrown away: a window asked for the renderer
        # opens as an ordinary one.
        "the software flag is read as nothing",
        "internal/gui/gui.go",
        "\t\tcase SoftwareFlag:\n\t\t\tlaunch.SoftwareGL = true",
        "\t\tcase SoftwareFlag:\n\t\t\tlaunch.SoftwareGL = false",
        "TestTheLaunchLineIsReadAndKeptWhole",
    ),
    (
        # One licence of the companion loses its text in the notices - the
        # heading the guard looks for is gone, and with it the text.
        "the notices stop reproducing one licence the renderer ships under",
        "THIRD-PARTY-NOTICES.md",
        "### BSL-1.0\n",
        "### Boost\n",
        "TestTheNoticesNameWhatShipsBesideTheWindow",
    ),
    (
        # The registry names a file the program does not load, so the
        # archive would be packed with the wrong name.
        "the registry names a file the window never loads",
        "internal/legal/companions.go",
        "{Path: \"opengl/libgallium_wgl.dll\", Size: 61868032,",
        "{Path: \"opengl/libgallium.dll\", Size: 61868032,",
        "TestTheRegistryNamesTheFilesTheWindowLoads",
    ),
    (
        # The bill of materials says the window CONTAINS the renderer,
        # which it does not - the archive carries it beside.
        "the SBOM says the window contains the renderer",
        "internal/legal/spdx.go",
        "\t\tRelationshipType:   \"DEPENDS_ON\",",
        "\t\tRelationshipType:   \"CONTAINS\",",
        "TestTheSBOMShipsTheCompanionBesideTheWindowOnly",
    ),
    (
        # The companion joins the window's list as ordinary code: it lands
        # under the wrong heading and the count of what ships beside is nought.
        "the companion is listed as code compiled in",
        "internal/legal/carried.go",
        "SPDX: c.SPDX, Copyright: c.Copyright, Beside: true})",
        "SPDX: c.SPDX, Copyright: c.Copyright, Beside: false})",
        "TestTheWindowListsTheCompanionAndTheCommandLineDoesNot",
    ),
    (
        # The pin the workflow downloads from moves without the registry the
        # notices are rendered from: the archive would carry one release and
        # the notices describe another.
        "the renderer pin names a release the registry does not",
        ".github/mesa-dist-win",
        "version=26.2.4",
        "version=26.2.5",
        "TestTheRendererPinAgreesWithTheRegistry",
    ),
    (
        # The archive's sum no longer checked before unpacking: whatever was
        # downloaded goes beside the program.
        "the fetch script unpacks a download it never summed",
        ".github/scripts/fetch_software_renderer.sh",
        '(cd "${fetched}" && echo "${sha256}  ${archive}" | sha256sum -c -)',
        '(cd "${fetched}" && echo "${sha256}  ${archive}" > /dev/null)',
        "TestTheFetchScriptChecksTheSumBeforeItUnpacks",
    ),
    (
        # The release packs the window without the renderer beside it.
        "the release forgets the renderer",
        ".github/workflows/release.yml",
        '            .github/scripts/fetch_software_renderer.sh "${work}"',
        '            true # .github/scripts/fetch_software_renderer.sh "${work}"',
        "TestEveryWindowBuildFetchesTheRendererBeforeItPacks",
    ),
    (
        # The libraries beside the program go out unsigned.
        "the signing skips the libraries",
        ".github/scripts/sign_release.py",
        '    signed = programs + [n for n in inside if n.endswith(".dll")]',
        '    signed = programs + [n for n in inside if n.endswith(".dylib")]',
        "TestTheSigningRepacksWholeAndSignsTheLibraries",
    ),
    (
        # The published archives are checked for the program's signature and
        # nobody asks the libraries.
        "the verification stops asking the libraries who signed them",
        ".github/workflows/verify-release.yml",
        "-Include *.exe, *.dll)",
        "-Include *.exe)",
        "TestThePublishedWindowArchiveIsCheckedForTheRenderer",
    ),
    (
        # The flag on a system nothing ships for names the system as if it
        # were the error: "could not be loaded: linux" - the first version.
        "the flag on a system nothing ships for reports the system as an error",
        "internal/gui/software.go",
        "\tcase errors.As(loaded, &shipsNot):\n\t\treturn text.RendererNotShipped()",
        "\tcase errors.As(loaded, &shipsNot) && false:\n\t\treturn text.RendererNotShipped()",
        "TestAWindowAskedForTheRendererSaysWhatBecameOfIt",
    ),
    (
        # The exception for the copy of the binding widened to the directory
        # above it: every future copy would pass the licence gate unread.
        "the dependency gate forgives every directory under third_party",
        ".github/scripts/dependency_gate.py",
        '    "./third_party/go-gl-gl": "a copy of github.com/go-gl/gl, MIT, held to the published module plus one patch by a guard",',
        '    "./third_party/go-gl-gl": "a copy of github.com/go-gl/gl, MIT, held to the published module plus one patch by a guard",\n    "./third_party/something-else": "",',
        "TestTheDependencyGateRefusesALicenceThisProjectCannotDistribute",
    ),
    (
        # The spawn allowance forgives every spawn in the registered file
        # again, not the one its entry was written for.
        "a registered file is forgiven every spawn",
        "internal/guard/notelemetry_test.go",
        "\t\t\tspawns = false\n\t\t\tcontinue",
        "\t\t\tcontinue",
        "TestARegisteredFileIsForgivenOneOperationAndNoMore",
    ),
    (
        # The release-download exemption widens from the fetch script to
        # every file, and a download link in SECURITY.md is forgiven.
        "any file may link to somebody else's release download",
        "internal/guard/community_test.go",
        "\t\tif rel == rendererFetchScript && download && companionProjects()[project] {",
        "\t\tif download && companionProjects()[project] {",
        "TestAReleaseDownloadIsForgivenInTheFetchScriptAlone",
    ),
    (
        # The exemption goes back to matching the script's NAME, so a second
        # script of that name anywhere under .github is forgiven with it -
        # the second outside review of #109.
        "a script of the same name anywhere may link to a release download",
        "internal/guard/community_test.go",
        "\t\tif rel == rendererFetchScript && download && companionProjects()[project] {",
        "\t\tif strings.HasSuffix(rel, \"/fetch_software_renderer.sh\") && download && companionProjects()[project] {",
        "TestAReleaseDownloadIsForgivenInTheFetchScriptAlone",
    ),
    (
        # The fetch script may download anybody's release, not the project
        # the companion registry names.
        "the fetch script may download any project's release",
        "internal/guard/community_test.go",
        "\t\tif rel == rendererFetchScript && download && companionProjects()[project] {",
        "\t\tif rel == rendererFetchScript && download {",
        "TestAReleaseDownloadIsForgivenInTheFetchScriptAlone",
    ),
    (
        # A file that cannot be read is called missing, and the person is
        # told to put back a folder that is there.
        "a renderer file that cannot be read is reported as missing",
        "internal/gui/software.go",
        "\t\tif errors.Is(err, fs.ErrNotExist) {\n\t\t\treturn path, nil\n\t\t}\n\t\treturn \"\", err",
        "\t\tif errors.Is(err, fs.ErrNotExist) || true {\n\t\t\treturn path, nil\n\t\t}\n\t\treturn \"\", err",
        "TestARendererFileThatCannotBeReadIsNotReportedAsMissing",
    ),
    # --- the second outside review of #109 (2026-09-17, afternoon) ---
    (
        # The walk moves into a docstring, which the first reader left in:
        # the text is in the file and the repack no longer walks.
        "the repack's walk survives only as a docstring",
        ".github/scripts/sign_release.py",
        "    for base, _dirs, names in os.walk(work):",
        "    \"\"\"for base, _dirs, names in os.walk(work):\"\"\"\n    for base, _dirs, names in os.scandir(work):",
        "TestTheSigningRepacksWholeAndSignsTheLibraries",
    ),
    (
        # The count moves into a short string literal on a line of its own.
        "the repack's count survives only inside a string literal",
        ".github/scripts/sign_release.py",
        "    if repacked != inside:",
        "    note = \"if repacked != inside:\"\n    if repacked is None:",
        "TestTheSigningRepacksWholeAndSignsTheLibraries",
    ),
    (
        # The certificate check commented out AFTER code on the same line,
        # which a reader of whole comment lines does not see.
        "the certificate check survives only as a trailing comment",
        ".github/scripts/sign_release.py",
        "        actual = certificate_of(target)",
        "        actual = pin  # actual = certificate_of(target)",
        "TestTheSigningRepacksWholeAndSignsTheLibraries",
    ),
    (
        # The pin constant names another file while three error messages go
        # on naming codesign.go: the text is in the script four times, and a
        # guard asking whether it is in the script at all stayed green.
        "the pin constant names another file and the messages keep the old name",
        ".github/scripts/sign_release.py",
        "PIN_FILE = os.path.join(\"internal\", \"legal\", \"codesign.go\")",
        "PIN_FILE = os.path.join(\"internal\", \"legal\", \"codesign.txt\")",
        "TestTheSigningScriptRefusesBeforeItSigns",
    ),
    (
        # The notes go back to naming the usual cause as the condition, which
        # is not the test the program makes.
        "the release notes say the renderer is used when the driver offers no OpenGL 2.1",
        ".github/workflows/release.yml",
        "            echo \"in \\`opengl\\` next to the program. The window loads it only after its first attempt\"",
        "            echo \"in \\`opengl\\` next to the program. The window loads it only when the graphics driver\"",
        "TestThePublishedWindowArchiveIsCheckedForTheRenderer",
    ),
    (
        # The allowance forgives any call handed the approved argument.
        "a registered file is forgiven any call handed the approved argument",
        "internal/guard/approvedoperation_test.go",
        "\treturn f.call == op.call && f.from == op.from",
        "\treturn f.from == op.from",
        "TestARegisteredFileIsForgivenOneOperationAndNoMore",
    ),
    (
        # The allowance forgives the approved call handed anything.
        "a registered file is forgiven the approved call handed anything",
        "internal/guard/approvedoperation_test.go",
        "\treturn f.call == op.call && f.from == op.from",
        "\treturn f.call == op.call",
        "TestARegisteredFileIsForgivenOneOperationAndNoMore",
    ),
    (
        # A name bound twice on the way to the call is followed to its
        # first binding, and the second is what runs.
        "an argument rebound on the way to the call is followed to its first binding",
        "internal/guard/approvedoperation_test.go",
        "\tif len(sources) != 1 {",
        "\tif len(sources) == 0 {",
        "TestARegisteredFileIsForgivenOneOperationAndNoMore",
    ),
    (
        # The reader of the signing script stops dropping docstrings, and an
        # operation named in one counts as the operation again.
        "the signing script reader keeps docstrings",
        "internal/guard/mesarelease_test.go",
        "\t\tcase strings.HasPrefix(rest, `\"\"\"`) || strings.HasPrefix(rest, `'''`):",
        "\t\tcase false:",
        "TestTheSigningScriptReaderKeepsCodeAndDropsProse",
    ),
    (
        # The reader keeps a comment that follows code on the same line.
        "the signing script reader keeps a trailing comment",
        "internal/guard/mesarelease_test.go",
        "\t\tcase rest[0] == '#':",
        "\t\tcase rest[0] == '#' && false:",
        "TestTheSigningScriptReaderKeepsCodeAndDropsProse",
    ),
    (
        # The window starts a program that is not the one os.Executable
        # answered: the registered file keeps its one spawn and the guard
        # has to see that it is not the approved one.
        "the second process is not the program os.Executable answered",
        "internal/gui/again.go",
        "\tcmd := exec.Command(exe, args...)",
        "\tcmd := exec.Command(exe[:0]+os.Args[0], args...)",
        "TestNothingShippedHoldsAnUndeclaredWayOut",
    ),
    (
        # The renderer is loaded by a name worked out in place rather than
        # by the path SoftwareFiles answers - the search order again, in the
        # registered file, as its one computed load.
        "the renderer is loaded by a name rather than by the path SoftwareFiles answers",
        "internal/gui/software_windows.go",
        "\t\tif _, err := syscall.LoadDLL(path); err != nil {",
        "\t\tif _, err := syscall.LoadDLL(softwareLoader + path[:0]); err != nil {",
        "TestALibraryIsOnlyLoadedLazilyWhenTheSystemAlreadyHasIt",
    ),
    # --- O221: the whole head row of a fold is one target (2026-09-17) ---
    (
        # A press on the row does nothing - which is what a press on the
        # title did until this change, with the arrow the only target.
        "a press on the head row leaves the fold as it was",
        "internal/gui/parts/foldhead.go",
        "\th.fold.Set(!h.fold.open)\n}",
        "\th.fold.Set(h.fold.open)\n}",
        "TestTheWholeHeadRowOfAFoldOpensAndShutsIt",
    ),
    (
        # The row answers the space character as well as the space key, so
        # one press of the space bar opens and shuts - the shape one press
        # of Space on Add batch had on 2026-09-16.
        "the head row answers the space bar twice",
        "internal/gui/parts/foldhead.go",
        "func (h *FoldHead) TypedRune(rune) {}",
        "func (h *FoldHead) TypedRune(rune) { h.Tapped(nil) }",
        "TestTheKeyboardOpensAndShutsAFoldOncePerPress",
    ),
    (
        # The keyboard arrives and the row draws nothing to say so.
        "the head row holds the keyboard without a mark",
        "internal/gui/parts/foldhead.go",
        "\th.marked = true\n\th.Refresh()\n}\n\nfunc (h *FoldHead) FocusLost() {",
        "\th.marked = false\n\th.Refresh()\n}\n\nfunc (h *FoldHead) FocusLost() {",
        "TestTheKeyboardOpensAndShutsAFoldOncePerPress",
    ),
    (
        # The fill under the pointer is never drawn: hovered is set and not
        # painted, which looks exactly like painted to every guard that
        # reads the flag.
        "the head row under the pointer draws no fill",
        "internal/gui/parts/foldhead.go",
        "\tif r.head.hovered {\n\t\tr.back.FillColor",
        "\tif r.head.hovered && false {\n\t\tr.back.FillColor",
        "TestTheHeadRowDrawsItsStatesAndTheArrowFollows",
    ),
    (
        # The ring for the keyboard is never drawn.
        "the head row holding the keyboard draws no ring",
        "internal/gui/parts/foldhead.go",
        "\tif r.head.marked {\n\t\tr.ring.StrokeWidth = ringWidth",
        "\tif r.head.marked && false {\n\t\tr.ring.StrokeWidth = ringWidth",
        "TestTheHeadRowDrawsItsStatesAndTheArrowFollows",
    ),
    (
        # The arrow stops following the row: inked the same under the pointer
        # as at rest.
        "the arrow does not brighten with the row under the pointer",
        "internal/gui/parts/foldhead.go",
        "\tif h.hovered {\n\t\tink = theme.ColorNameForeground",
        "\tif h.hovered && false {\n\t\tink = theme.ColorNameForeground",
        "TestTheHeadRowDrawsItsStatesAndTheArrowFollows",
    ),
    (
        # The fill under the pointer runs the width of the row again - the
        # owner's "enormous" hover of 2026-09-21.
        "the head's fill under the pointer is as wide as the row rather than as the words",
        "internal/gui/parts/foldhead.go",
        "\tif r.head.under != nil {",
        "\tif r.head.under != nil && false {",
        "TestTheHeadRowDrawsItsStatesAndTheArrowFollows",
    ),
    (
        # The arrow points down whatever the fold is doing.
        "the arrow points the same way open and shut",
        "internal/gui/parts/foldhead.go",
        "\tif !h.fold.open {\n\t\ticon = theme.MenuExpandIcon()",
        "\tif !h.fold.open && false {\n\t\ticon = theme.MenuExpandIcon()",
        "TestTheHeadRowDrawsItsStatesAndTheArrowFollows",
    ),
    (
        # The row no longer overhangs the column, so the room it keeps inside
        # its box moves the title off the edge its fields stand on.
        "the head row stops overhanging and the title moves off the edge",
        "internal/gui/parts/folding.go",
        "container.New(overhang{by: TabInset}, row)",
        "container.New(overhang{by: 0}, row)",
        "TestTheHeadRowOverhangsTheColumnAndTheTitleDoesNot",
    ),
    # --- after the outside review of #110 (2026-09-17) ---
    (
        # The row stops drawing the ring on the first key after a press - the
        # state it shipped in on the PR's first commit, found by asking the
        # row's siblings rather than the review.
        "the head row draws no ring on the first key after a press",
        "internal/gui/parts/foldhead.go",
        "\tif !h.marked {\n\t\th.marked = true\n\t\th.Refresh()\n\t}\n\tswitch event.Name {",
        "\tswitch event.Name {",
        "TestAKeyAfterAPressOnTheHeadRowDrawsTheRing",
    ),
    (
        # The one place a press takes the keyboard stops being quiet, so every
        # control of the family draws its mark for the mouse again - the
        # 2026-08-12 menu painted blue, for all of them at once.
        "a press takes the keyboard with the mark drawn, for every control",
        "internal/gui/parts/focus.go",
        "\tp.Quietly(func() { canvas.Focus(control) })",
        "\tcanvas.Focus(control)",
        "TestAPressMovesTheKeyboardWithoutDrawingItsMark",
    ),
    (
        # The head row of a fold is measured on its own width again - the
        # width the HBox hands it is the width of its own words, so this half
        # of the guard was whole by construction until 2026-09-17.
        "a fold's head row is measured on its own width rather than the room it has",
        "internal/guard/oneline_test.go",
        "row.MinSize().Width, row",
        "row.Size().Width, row",
        "TestAFoldsHeadRowIsMeasuredAsAWholeAgainstItsWidth",
    ),
    (
        # The pairing check goes: a head the walk could not pair with its
        # words is stepped over, and a fold built another way is a fold this
        # guard is silently not measuring.
        "a fold's head with no words beside it is stepped over rather than refused",
        "internal/guard/oneline_test.go",
        "\tif heads != rows {",
        "\tif heads != rows && false {",
        "TestAFoldsHeadRowIsMeasuredAsAWholeAgainstItsWidth",
    ),
    # --- the owner's nine points from the running window (2026-09-21) ---
    (
        # Point 1 put back: the explanation on the panel's surface, which over
        # a panel is a sentence with no edge anywhere.
        "the explanation stands on the panel colour again",
        "internal/gui/parts/detail.go",
        "\tbox := container.NewStack(append(floatingCard(), Padded(Inset, Prose(detail)))...)",
        "\tbox := container.NewStack(panelSurface(), Padded(Inset, Prose(detail)))",
        "TestTheExplanationFloatsOnTheSurfaceAnOpenListDoes",
    ),
    (
        # Point 6 put back: the tick a step inside the square on every side,
        # which drew it 7 px across in a square of 20.
        "the tick of a switch keeps a margin inside its square again",
        "internal/gui/parts/toggle.go",
        "\tr.tick.Resize(square)\n\tr.tick.Move(at)",
        "\tr.tick.Resize(fyne.NewSquareSize(markSide - GapInline*2))\n\tr.tick.Move(at.Add(fyne.NewPos(GapInline, GapInline)))",
        "TestTheTickOfASwitchFillsItsSquare",
    ),
    (
        # Point 2 put back: upward as soon as the list does not fit below and
        # there is more room above - the preset screen's format list over the
        # question the preset asks.
        "a list turns upward as soon as there is more room above the box",
        "internal/gui/parts/ring.go",
        "\tif rows <= below || below >= listOpensDownwardFrom*listRowHeight() || below >= above {",
        "\tif rows <= below || below >= above {",
        "TestAListOpensDownwardWheneverAFewRowsFitUnderTheBox",
    ),
    (
        # The cut stops landing on a row's edge: the room below is taken to
        # the pixel, and the last row of the list is cut through the middle.
        "a list cut to the room below ends through the middle of a row",
        "internal/gui/parts/ring.go",
        "\tbelow := wholeRows(canvasHeight - (boxTop + boxHeight) - listEdgeGap - head)",
        "\tbelow := fyne.Max(0, canvasHeight-(boxTop+boxHeight)-listEdgeGap-head)",
        "TestAListOpensDownwardWheneverAFewRowsFitUnderTheBox",
    ),
    (
        # Point 4 put back: the window coming to the front marks whatever
        # holds the keyboard - the one blue menu the owner saw.
        "the window coming back draws the mark whether or not it was there",
        "internal/gui/parts/focus.go",
        "\t\tp.returning = false\n\t\treturn p.wasMarked",
        "\t\tp.returning = false\n\t\treturn true",
        "TestTheWindowComingBackDrawsOnlyTheMarkThatWasThere",
    ),
    (
        # The return is never forgotten, so the keyboard arriving plainly
        # afterwards answers with the state of the last loss.
        "the window's return is never forgotten and mutes the next arrival",
        "internal/gui/parts/focus.go",
        "\t\tp.returning = false\n\t\treturn p.wasMarked",
        "\t\treturn p.wasMarked",
        "TestTheWindowComingBackDrawsOnlyTheMarkThatWasThere",
    ),
    (
        # Point 5 put back: a text box as wide as the column.
        "a box for a short piece of text takes the whole column again",
        "internal/gui/parts/tokens.go",
        # Repointed 2026-09-23, when a short text took one column of the grid.
        "\tTextWidth = NumericWidth\n",
        "\tTextWidth = ColumnWidth\n",
        "TestOnlyAPathTakesTheWholeRow",
    ),
    (
        # One box forgets to ask for the width: the batch name on the first
        # screen takes the row, the way every text box did until today.
        "the batch name box on the single batch screen takes the row",
        "internal/gui/window/generate.go",
        # Repointed 2026-09-23: in the grid an unsized box takes one column,
        # so taking the wrapper off no longer widens it. Marked Wide it takes
        # the row, which is the defect.
        "\t\t\tadd(engine.SettingID, text.FieldTargetID(), text.HintTargetID(), g.tips.Say(text.DetailTargetID()), parts.Text(g.id)),",
        "\t\t\tparts.Wide(add(engine.SettingID, text.FieldTargetID(), text.HintTargetID(), g.tips.Say(text.DetailTargetID()), parts.Text(g.id))),",
        "TestOnlyAPathTakesTheWholeRow",
    ),
    (
        # A declared text setting - an archive's password - takes the row.
        "a declared free text setting takes the whole row",
        "internal/gui/parts/property.go",
        # Repointed 2026-09-23: unsized, a box takes one column of the grid,
        # so the defect is a width as wide as the form.
        "\tcase p.Kind == format.PropertyText:\n\t\treturn Sized(fyne.Max(TextWidth, roomFor(leftAlone(p, ofATool))), control)\n",
        "\tcase p.Kind == format.PropertyText:\n\t\treturn Sized(ColumnWidth, control)\n",
        "TestOnlyAPathTakesTheWholeRow",
    ),
    (
        # Point 8 put back: no fill at rest, an outline round nothing.
        # Since 2026-09-22 the face is the button's own surface. The way back
        # to the owner's report is the field's colour, which the guard asks
        # about by name and by distance.
        "a secondary button wears the field's surface again",
        "internal/gui/parts/button.go",
        "\t\tf.fill = PaletteColour(theme.ColorNameButton, dark)",
        "\t\tf.fill = PaletteColour(theme.ColorNameInputBackground, dark)",
        "TestASecondaryButtonWearsAFaceAtRest",
    ),
    (
        # Point 9 put back: the face at once, which is the flash.
        "the busy face goes on the moment work starts",
        "internal/gui/window/runbusy.go",
        "\tb.callOff = b.later(BusyFaceAfter, func() {",
        "\tb.callOff = b.later(0, func() {",
        "TestTheBusyFaceWaitsForWorkThatLasts",
    ),
    (
        # The clock is bypassed: the face is worn at once and the clock is
        # asked for nothing.
        "the busy face is worn without asking the clock",
        "internal/gui/window/runbusy.go",
        "\tb.callOff = b.later(BusyFaceAfter, func() {\n\t\t// The same work still going, and not already worn: a face asked for\n\t\t// by earlier work is a face for a screen that has moved on, and a\n\t\t// face put on twice would be taken off once.\n\t\tif b.epoch == mine && b.occupied && !b.worn {\n\t\t\tb.wear(face)\n\t\t}\n\t})",
        "\t_ = mine\n\tb.wear(face)\n\tb.callOff = func() {}",
        "TestTheBusyFaceWaitsForWorkThatLasts",
    ),
    (
        # A second press inside the first moment is refused by nothing: the
        # button is still live and the state is not asked.
        "a second Preview inside the first moment starts a second preview",
        "internal/gui/window/run.go",
        "\t// The same refusal as onGenerate, for the same moment.\n\tif r.busy.occupied {\n\t\treturn\n\t}\n",
        "",
        "TestTheBusyFaceWaitsForWorkThatLasts",
    ),
    (
        # Work ending does not call the face off, so a clock firing later
        # dresses an idle screen.
        "work ending leaves the clock running",
        "internal/gui/window/runbusy.go",
        "\tif b.callOff != nil {\n\t\tb.callOff()\n\t\tb.callOff = nil\n\t}",
        "\tb.callOff = nil",
        "TestWorkOverBeforeTheClockWearsNoFace",
    ),
    (
        # The row is left to the toolkit when the face comes off: Preview and
        # Generate keep the positions they had with Cancel beside them.
        "the row of buttons is not laid out again when the face comes off",
        "internal/gui/window/runbusy.go",
        "\tb.bar.Hide()\n\tb.relay()",
        "\tb.bar.Hide()",
        "TestTheButtonsStandWhereTheyStoodAfterAPreview",
    ),
    (
        # And when it goes on: Cancel is shown and never laid out.
        "the row of buttons is not laid out again when the face goes on",
        "internal/gui/window/runbusy.go",
        "\tif face.progressing {\n\t\tb.bar.Show()\n\t}\n\tb.relay()",
        "\tif face.progressing {\n\t\tb.bar.Show()\n\t}",
        "TestTheButtonsStandWhereTheyStoodAfterAPreview",
    ),
    # --- after the outside review of #115 (2026-09-21, evening) ---
    (
        # The epoch check is made always true, so a face queued by earlier
        # work dresses the work that came after it - the review's finding.
        "a face asked for by earlier work dresses the later work",
        "internal/gui/window/runbusy.go",
        "\t\tif b.epoch == mine && b.occupied && !b.worn {",
        "\t\tif b.epoch >= mine && b.occupied && !b.worn {",
        "TestAFaceAskedForByEarlierWorkNeverDressesLaterWork",
    ),
    (
        # The hook stops telling anything: a focused menu is marked on the
        # window's return again.
        "the foreground hook tells the focused control nothing",
        "internal/gui/returning.go",
        "\t\treturning.WindowReturning()",
        "\t\t_ = returning",
        "TestTheForegroundHookTellsTheFocusedControlTheWindowIsBack",
    ),
    (
        # The registration stays and its body stops calling the hook, so the
        # window binary tells nothing on its return - read out of the
        # source, because the line is behind cgo.
        "the foreground hook is not registered by the window binary",
        "internal/gui/run_cgo.go",
        "\ta.Lifecycle().SetOnEnteredForeground(func() { WindowReturning(w.Canvas()) })",
        "\ta.Lifecycle().SetOnEnteredForeground(func() { _ = w.Canvas() })",
        "TestTheWindowBinaryRegistersTheForegroundHook",
    ),
    (
        # The defect put back exactly: the hook's file imports parts for the
        # interface, and the no-cgo half of the window package reaches the
        # toolkit's widgets - red on macOS alone until this guard.
        "the hook's file imports parts and drags the widgets into the no-cgo build",
        "internal/gui/returning.go",
        "import \"fyne.io/fyne/v2\"\n",
        "import (\n\t\"fyne.io/fyne/v2\"\n\n\t\"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts\"\n)\n\nvar _ parts.Returnable\n",
        "TestTheWindowPackageOutsideCgoReachesNoWidget",
    ),
    (
        "a checkout with a with block of its own keeps the token again, which "
        "is the shape a line based search would read wrong",
        ".github/workflows/pages.yml",
        "        with:\n          persist-credentials: false\n          # The commit CI passed on",
        "        with:\n          # The commit CI passed on",
        "TestEveryCheckoutTurnsItsTokenOff",
    ),
    (
        "the rule reads a boolean true as off, so the self test on the predicate "
        "is what has to catch it, not the tree",
        "internal/guard/checkoutcredentials_test.go",
        "\tcase bool:\n\t\treturn !v\n",
        "\tcase bool:\n\t\treturn true\n",
        "TestEveryCheckoutTurnsItsTokenOff",
    ),
    (
        "the YAML walk stops recognising a checkout, so it judges none of them, "
        "and only the second reading of the text can say so",
        "internal/guard/checkoutcredentials_test.go",
        'if !strings.HasPrefix(step.Uses, "actions/checkout@") {',
        'if !strings.HasPrefix(step.Uses, "actions/checkout@@") {',
        "TestEveryCheckoutTurnsItsTokenOff",
    ),
    (
        "the preset expands into a seed of its own, which a recipe building on it "
        "could not tell from the file's",
        "internal/preset/build.go",
        "\tbody, err := recipe.Compose(recipe.Document{Targets: p.targets})\n",
        "\tbody, err := recipe.Compose(recipe.Document{Targets: p.targets, Seed: \"1\"})\n",
        "TestAPresetExpandsToTargetsAndNothingElse",
    ),
    (
        "the rule stops seeing a seed on the preset's side, so the self test on "
        "the predicate is what has to catch it",
        "internal/recipe/extends.go",
        "\tif raw.Seed != nil {\n",
        "\tif false {\n",
        "TestAPresetExpandsToTargetsAndNothingElse",
    ),
    (
        "the file's own targets go before the preset's, leaving every byte in "
        "place and the manifest in another order",
        "internal/recipe/extends.go",
        "\tmerged.Targets = append(append([]rawTarget{}, from.Targets...), own.Targets...)\n",
        "\tmerged.Targets = append(append([]rawTarget{}, own.Targets...), from.Targets...)\n",
        "TestARecipeBuildingOnAPresetGivesTheBytesOfTheEjectedOneWithItsTargetsAppended",
    ),
    (
        "a run from a file that builds on a preset stops recording which preset, "
        "so the manifest reads like one from a file that stands alone",
        "internal/cli/generate.go",
        "\topt.Preset = record(read.Expansion)\n",
        "\topt.Preset = nil\n",
        "TestARecipeBuildingOnAPresetGivesTheBytesOfTheEjectedOneWithItsTargetsAppended",
    ),
    (
        "the batch screen draws the section and drops it on the way to the "
        "recipe, so the switch changes nothing the run can see",
        "internal/gui/window/recipe.go",
        "\tdoc.Extends, doc.With = r.base.draft()\n",
        "\t_, _ = r.base.draft()\n",
        "TestARecipeBuiltOnAPresetFromTheWindowGivesTheBytesTheFileGives",
    ),
    (
        "a refusal about a parameter the preset does not take arrives addressed "
        "to the preset line rather than to the parameter's, so a form marks the "
        "wrong box",
        "internal/preset/read.go",
        '\t\tproblem.At = recipe.KeyWith + "." + unknownParameter.Name\n',
        "\t\tproblem.At = recipe.KeyExtends\n",
        "TestEveryRefusalAboutThePresetSideOfARecipeNamesItsLine",
    ),
    (
        "an id the preset already uses is refused with the words for two targets "
        "of the file, which point at a line the file does not have",
        "internal/recipe/recipe.go",
        "\tif ofThePreset {\n",
        "\tif ofThePreset && false {\n",
        "TestEveryRefusalAboutThePresetSideOfARecipeNamesItsLine",
    ),
    (
        "extends stops asking for its scheme, so a file name is handed to the "
        "preset registry and refused as an unknown preset instead",
        "internal/recipe/extends.go",
        '\tif id == value || id == "" {\n',
        "\tif false {\n",
        "TestEveryRefusalAboutThePresetSideOfARecipeNamesItsLine",
    ),
    (
        "the door guard stops excusing the command line's preset path, and if "
        "the walk reaches that file it has to say so",
        "internal/guard/extends_test.go",
        '\t\tfilepath.Join("internal", "cli", "preset.go"):           true,\n',
        '\t\tfilepath.Join("internal", "cli", "preset.go"):           false,\n',
        "TestEveryRecipeFileIsReadThroughTheDoorThatKnowsPresets",
    ),
    (
        "the switch stops letting the last batch go, so a preset's set cannot be "
        "run alone from the screen - the shape an outside review of #119 found",
        "internal/gui/window/recipebase.go",
        "func (b *base) carriesTheRun() bool { return b.on.Checked }\n",
        "func (b *base) carriesTheRun() bool { return false }\n",
        "TestTheBatchScreenCanRunAPresetsSetAlone",
    ),
    (
        "a file of extends alone handed to the plain reader collects a second "
        "refusal about asking for no files, contradicting the README",
        "internal/recipe/recipe.go",
        "\"read the file with \\\"tfg generate\\\" or \\\"tfg validate\\\", which do\"))\n\t\treturn rec\n",
        "\"read the file with \\\"tfg generate\\\" or \\\"tfg validate\\\", which do\"))\n",
        "TestEveryRefusalAboutThePresetSideOfARecipeNamesItsLine",
    ),
    (
        "the refusal about a joint limit goes back to rounding both counts, so "
        "a request just over the limit prints as the limit itself",
        "internal/format/jointlimit.go",
        "\tif j.per() > 1 && got/j.per() != j.Max/j.per() {\n",
        "\tif j.per() > 1 {\n",
        "TestNoJointLimitRefusalPrintsTheRequestAndTheLimitAsOneNumber",
    ),
    (
        "the limit in a joint refusal loses its unit again, so it is a bare "
        "number taking its noun from four words earlier. Aimed at the guard "
        "that reads the SENTENCE - the one checking the declarations stayed "
        "green on it, because a declaration with a Base can still be printed "
        "without one",
        "internal/format/jointlimit.go",
        "core.Says(\"format.ExactlyOf\", \"%s %s\", core.A(\"Number\", core.Exactly(j.Max)), core.A(\"Unit\", core.UnitTerm(j.Base)))",
        "core.Says(\"format.ExactlyOf\", \"%s\", core.A(\"Number\", core.Exactly(j.Max)))",
        "TestNoJointLimitRefusalPrintsTheRequestAndTheLimitAsOneNumber",
    ),
    (
        "a rule binding two settings stops saying what it counts, so its exact "
        "refusal ends without a noun",
        "internal/format/png/png.go",
        '\t\t\tUnit: "megapixels", Per: 1_000_000, Base: "pixels",\n',
        '\t\t\tUnit: "megapixels", Per: 1_000_000,\n',
        "TestEveryJointLimitSaysWhatItCounts",
    ),
    (
        "no preset says which one the window opens on, so it falls back to the "
        "first id in alphabetical order",
        "internal/preset/emptyandminimal.go",
        "\t\tLanding: true,\n",
        "\t\tLanding: false,\n",
        "TestExactlyOnePresetOpensTheWindow",
    ),
    (
        "the presets screen goes back to the line it had until 2026-09-22 - "
        "opening on the FIRST id, which is the regression somebody would "
        "actually write. The behavioural guard cannot see it while the declared "
        "preset is also first alphabetically, which is why this is aimed at the "
        "one reading the source",
        "internal/gui/window/preset.go",
        "\tif landing := preset.Landing(); landing != \"\" {\n\t\tp.pick.SetSelected(landing)\n\t}\n",
        "\tif len(ids) > 0 {\n\t\tp.pick.SetSelected(ids[0])\n\t}\n",
        "TestNoScreenChoosesAPresetByItsPlaceInTheList",
    ),
    (
        "the batch screen goes back to opening on the FIRST id, the same "
        "regression one screen over",
        "internal/gui/window/recipebase.go",
        "\tif landing := preset.Landing(); landing != \"\" {\n\t\tb.pick.SetSelected(landing)\n\t}\n",
        "\tif len(ids) > 0 {\n\t\tb.pick.SetSelected(ids[0])\n\t}\n",
        "TestNoScreenChoosesAPresetByItsPlaceInTheList",
    ),
    (
        "a screen opens on a preset nobody declared - the LAST in the list. "
        "Kept beside the two above because it is the half the behavioural guard "
        "does see, and losing it would leave the screens proven only by a read "
        "of their source",
        "internal/gui/window/preset.go",
        "\t\tp.pick.SetSelected(landing)\n",
        "\t\tp.pick.SetSelected(preset.IDs()[len(preset.IDs())-1])\n",
        "TestBothScreensOpenOnTheDeclaredPreset",
    ),
    (
        "the formats are taken in the order they were typed, so one selection "
        "written two ways makes two recipes and two recipe_hash values",
        "internal/preset/emptyandminimal.go",
        "\tfor _, id := range format.IDs() {\n\t\tif !wanted[id] {\n\t\t\tcontinue\n\t\t}\n",
        "\tfor _, id := range ids {\n\t\tif false && wanted[id] {\n\t\t\tcontinue\n\t\t}\n",
        "TestTheMinimalSetIsTheSameWhateverOrderTheFormatsAreNamedIn",
    ),
    (
        "one format is laid out twice, which the counts of formats and of empty "
        "files cannot see because both of them are counts of a map",
        "internal/preset/emptyandminimal.go",
        "\t\t\tminimal = append(minimal, minimalFile{desc: d, size: floor})\n\t\t\tcontinue\n",
        "\t\t\tminimal = append(minimal, minimalFile{desc: d, size: floor}, minimalFile{desc: d, size: floor})\n\t\t\tcontinue\n",
        "TestTheMinimalSetSitsOnEveryFormatsFloor",
    ),
    (
        "a format is registered under the word that means every format, so the "
        "formats parameter can never reach it",
        "internal/format/txt/txt.go",
        '\t\tID:          "txt",\n',
        '\t\tID:          "all",\n',
        "TestNoFormatIsCalledByTheWordThatMeansAllOfThem",
    ),
    (
        "the minimal set asks for one byte more than the floor, so every file "
        "still generates and verifies and none of them is a minimum any more",
        "internal/preset/emptyandminimal.go",
        "\t\t\tminimal = append(minimal, minimalFile{desc: d, size: floor})\n",
        "\t\t\tminimal = append(minimal, minimalFile{desc: d, size: floor + 1})\n",
        "TestTheMinimalSetSitsOnEveryFormatsFloor",
    ),
    (
        "a format that reaches nought bytes stops bringing its one byte file, "
        "so two formats quietly leave the positive control",
        "internal/preset/emptyandminimal.go",
        "\t\tminimal = append(minimal, minimalFile{desc: d, size: 1})\n",
        "\t\t_ = d\n",
        "TestTheMinimalSetSitsOnEveryFormatsFloor",
    ),
    (
        "an empty file claims to be accepted rather than left to the policy of "
        "whoever reads it",
        "internal/preset/emptyandminimal.go",
        '\t\t\tExpected: "unspecified", ExpectedReason: "size_zero",\n',
        '\t\t\tExpected: "accept",\n',
        "TestTheMinimalSetSitsOnEveryFormatsFloor",
    ),
    (
        "a set built with no empty half goes quiet about it, so a preset named "
        "after both halves ships one without a word",
        "internal/preset/emptyandminimal.go",
        "\tfor _, d := range descs {\n\t\tif smallest(d) == 0 {\n\t\t\treturn nil\n\t\t}\n\t}\n",
        "\tif len(descs) >= 0 {\n\t\treturn nil\n\t}\n",
        "TestASetWithNoEmptyHalfSaysSoAndOneWithBothStaysQuiet",
    ),
    (
        "two spellings of one item are compared before either is normalised, "
        "so PNG and png become two targets of one id",
        "internal/preset/build.go",
        "\t\titem := typed\n\t\tif l.keep != nil {\n\t\t\titem = l.keep(typed)\n\t\t}\n\t\tkey := item\n",
        "\t\titem := typed\n\t\tkey := item\n\t\tif l.keep != nil {\n\t\t\titem = l.keep(typed)\n\t\t}\n",
        "TestAListParameterRefusesTwoSpellingsOfOneItem",
    ),
    (
        "a composed recipe goes back to the marshaller's flat sequences, so a "
        "target pasted in from the documents no longer lines up",
        "internal/recipe/compose.go",
        "\treturn yaml.MarshalWithOptions(doc, yaml.IndentSequence(true))\n",
        "\treturn yaml.Marshal(doc)\n",
        "TestAComposedRecipeIsWrittenTheWayAPersonWritesOne",
    ),
    (
        "every value goes back to being quoted, so a composed recipe writes "
        'size: "1024" where a person writes size: 1024',
        "internal/recipe/compose.go",
        "\t\t\tentry = append(entry, yaml.MapItem{Key: key, Value: bareNumber(value)})\n",
        "\t\t\tentry = append(entry, yaml.MapItem{Key: key, Value: value})\n",
        "TestAComposedRecipeIsWrittenTheWayAPersonWritesOne",
    ),
    (
        "a name made of digits is turned into a number, so a file called 123 "
        "loses its name. Not 007 - bareNumber refuses a leading zero anyway, "
        "which is how this entry sat green and proved nothing once",
        "internal/recipe/compose.go",
        '\tadd("name", t.Name)\n',
        '\taddNumber("name", t.Name)\n',
        "TestAComposedRecipeIsWrittenTheWayAPersonWritesOne",
    ),
    (
        "the window sends the formats box nowhere, so the screen builds the "
        "whole set whatever was typed into it",
        "internal/gui/window/preset.go",
        "\t\tif v := f.Value(); v != \"\" {\n",
        "\t\tif v := f.Value(); v == \"\" {\n",
        "TestThePresetScreenBuildsTheMinimalSetTheCommandLineBuilds",
    ),
    (
        "a group a parameter emptied goes quiet, so a set of seven groups "
        "ships as six and nothing in the run says which one is missing",
        "internal/preset/uploadvalidation.go",
        "\tif s.bulk == 0 {\n\t\tout = append(out, core.Says(\"preset.BulkNought\", \"bulk is nought, so this set holds no mass upload.\"))\n\t}\n",
        "\t_ = s.bulk\n",
        "TestAGroupAPresetLeavesOutIsSaidOutLoud",
    ),
    (
        "a list of formats refused stops naming what this build does have, so "
        "somebody who typed heic has nowhere to find out what it takes",
        "internal/preset/build.go",
        "\"this build has no format called that. It has: %s\", core.A(\"Join\", strings.Join(format.IDs(), \", \")))",
        "\"this build has no format called that\")",
        "TestAListOfFormatsRefusedNamesWhatThisBuildHas",
    ),
    (
        "the file at the limit is renamed, so every manifest written from this "
        "preset carries a recipe_hash that no longer matches the one it had",
        "internal/preset/limitset.go",
        '\tout = append(out, step{id: "at_limit", size: s.limit, accept: true})\n',
        '\tout = append(out, step{id: "at_the_limit", size: s.limit, accept: true})\n',
        "TestEjectingAPresetGivesTheBytesItAlwaysGave",
    ),
    (
        "the dialect group stops varying the settings that are switches, so a "
        "table with no header row quietly leaves the set",
        "internal/preset/tabularimport.go",
        "\t\tif p.Kind == format.PropertyChoice || p.Kind == format.PropertyBool {\n",
        "\t\tif p.Kind == format.PropertyChoice {\n",
        "TestEveryDialectTheTableDeclaresIsInTheSet",
    ),
    (
        "a type allowed and denied at once builds a set holding one file to "
        "accept and another to turn away, both of them that type",
        "internal/preset/uploadset.go",
        "\tif err := listsAgree(s.allowed, s.denied); err != nil {\n\t\treturn s, err\n\t}\n",
        "\t_ = listsAgree\n",
        "TestAnExtensionCannotBeAllowedAndDeniedAtOnce",
    ),
    (
        "a denied file is named from the format id rather than from the "
        "extension the registry declares, so targz becomes denied.targz",
        "internal/preset/uploadfiles.go",
        "\t\t\tname: \"denied\" + entry.extension(), size: sampleFor(entry.desc),\n",
        "\t\t\tname: \"denied.\" + entry.ext, size: sampleFor(entry.desc),\n",
        "TestADeniedFileIsNamedWithTheExtensionTheRegistryDeclares",
    ),
    (
        "the table preset promises a range of rows the spreadsheet does not "
        "take, so the field and the refusal describe a set the run will not build",
        "internal/preset/tabularimport.go",
        "\tsheetRowsMax    = 200000\n",
        "\tsheetRowsMax    = 100000\n",
        "TestTheTabularPresetTakesTheRangesTheSheetDeclares",
    ),
    (
        "a combination the format refuses stops being left out of the encoding "
        "set, so the whole set is refused over a cell nobody asked for",
        "internal/preset/textencoding.go",
        "\tif errors.As(err, &bad) {\n",
        "\tif false && errors.As(err, &bad) {\n",
        "TestTheEncodingSetHoldsEveryCombinationThisBuildCanWrite",
    ),
    # --- the window says what a finished run left, and offers the way to it
    #     (2026-09-23, the first contact analysis) ---
    (
        "the window stops naming the manifest a finished run wrote",
        "internal/gui/window/run.go",
        "outcome := text.SaidWithManifest(outcomeText(res, runErr), manifestNameOf(saved.Manifest))",
        "outcome := outcomeText(res, runErr)",
        "TestAFinishedRunNamesTheManifestAndOffersToOpenIt",
    ),
    (
        "a control with no setting behind it goes on the form with no name over it",
        "internal/gui/parts/fields.go",
        # Repointed 2026-09-23, when a named control became a cell of the grid.
        "\treturn cellOf(FieldStack(headingRow(label, detail, false), control))\n",
        "\treturn cellOf(control)\n",
        "TestEveryControlOnTheFormStandsUnderAName",
    ),
    (
        "the batch screen opens with no size, so Generate refuses on a form nobody typed into",
        "internal/gui/window/recipe.go",
        "\tr.batches[0].size.SetText(startingSize)\n",
        '\tr.batches[0].size.SetText("")\n',
        "TestEveryWorkScreenOpensReadyToRun",
    ),
    (
        "a batch added to the screen arrives carrying the first one's name",
        "internal/gui/window/recipe.go",
        "\tr.batches[len(r.batches)-1].formatPick.SetSelected(format.IDs()[0])\n",
        "\tr.batches[len(r.batches)-1].formatPick.SetSelected(format.IDs()[0])\n\tr.batches[len(r.batches)-1].id.SetText(startingBatchName)\n",
        "TestOnlyTheFirstBatchArrivesFilledIn",
    ),
    (
        "the About screen stops saying what to do with the program",
        "internal/gui/window/about.go",
        "\t\tparts.Section(text.SectionHowToUse(), parts.Bullets(text.HowToUseSteps())),\n",
        "",
        "TestTheAboutScreenSaysHowToUseTheProgram",
    ),
    # --- the palette comes from a rule, and the window can show it (#124,
    #     written down 2026-09-23 - the three guards landed with their
    #     mutations run by hand and never entered here, so the gate that asks
    #     whether every guard is proven was red on this machine and green in
    #     CI, where tools/ is not cloned) ---
    (
        "the panel is four bytes off the rung the rule gives it",
        "internal/gui/parts/theme.go",
        "ColorNamePanel:           hex(0x2B, 0x2B, 0x2F),",
        "ColorNamePanel:           hex(0x2F, 0x2F, 0x33),",
        "TestThePaletteIsTheOneTheRuleProduces",
    ),
    (
        "the menu drops below the face of a button, so the thing that floats is under it",
        "internal/gui/parts/theme.go",
        "theme.ColorNameMenuBackground: hex(0x57, 0x57, 0x5F),",
        "theme.ColorNameMenuBackground: hex(0x44, 0x44, 0x4A),",
        "TestTheLadderClimbsInEvenSteps",
    ),
    (
        "the preview offers the manifest of the run it did not make",
        "internal/gui/window/run.go",
        "\tsaid := r.line.said(exactly(planned), opt.OutDir) + text.AndNothingWrittenYet()\n",
        "\tr.offer.manifest.show(engine.ManifestPath(opt))\n\tsaid := r.line.said(exactly(planned), opt.OutDir) + text.AndNothingWrittenYet()\n",
        "TestAPreviewNamesNoManifestAndOffersNone",
    ),
    (
        "a colour joins the palette with no place in the order the page reads it by",
        "internal/gui/parts/theme.go",
        "\tdarkColours = map[fyne.ThemeColorName]color.Color{\n",
        "\tdarkColours = map[fyne.ThemeColorName]color.Color{\n\t\ttheme.ColorNameScrollBar: hex(0x48, 0x48, 0x4E),\n",
        "TestThePalettePageShowsEveryColourAndHowToReadIt",
    ),
    (
        "the steps drop one of the outcomes the manifest can record",
        "internal/gui/text/screens.go",
        "accept it, reject it or sanitize it - or records the outcome as unspecified",
        "accept it or reject it - or records the outcome as unspecified",
        "TestTheStepsNameEveryOutcomeTheManifestCanRecord",
    ),
    # --- the form in columns, groups that say what they are, and a refusal
    #     that offers its own fix (2026-09-23, the layout prototype) ---
    (
        "every field takes a whole row of the form again",
        "internal/gui/parts/grid.go",
        "func spanOf(o fyne.CanvasObject, column float32) int {\n",
        "func spanOf(o fyne.CanvasObject, column float32) int {\n\treturn GridColumns\n",
        "TestShortFieldsShareARowAndTakeOneColumnEach",
    ),
    (
        "the button under a size refusal says the smallest size and puts nothing in the box",
        "internal/gui/parts/fixes.go",
        "func() { box.SetText(value) }",
        "func() { _, _ = box, value }",
        "TestASizeBelowTheMinimumOffersTheSmallestSize",
    ),
    # --- the list of formats grouped by kind, with a filter (2026-09-23,
    #     docs/FORMAT-MENU-2026-09-23.md) ---
    (
        "a heading counts every format the filter kept rather than the ones under it",
        "internal/gui/parts/narrow.go",
        "text.ListHeadingCount(h, len(groups[h]))",
        "text.ListHeadingCount(h, len(kept))",
        "TestTheFormatListStandsUnderAHeadingForEachKind",
    ),
    (
        "a heading is matched anywhere in a word, so one letter keeps nearly every kind",
        "internal/gui/parts/narrow.go",
        "\t\tif strings.HasPrefix(word, want) {\n",
        "\t\tif strings.Contains(word, want) {\n",
        "TestTypingIntoTheFormatListNarrowsItAndLandsOnWhatStartsWithIt",
    ),
    (
        "the arrows stop on a heading as if it were a value",
        "internal/gui/parts/narrow.go",
        "\t\tif entries[at].kind == entryValue {\n\t\t\treturn at\n",
        "\t\tif entries[at].kind != entryNotice {\n\t\t\treturn at\n",
        "TestTheArrowsInTheFormatListStepOverTheHeadings",
    ),
    (
        "a letter typed at the shut format menu walks the values again instead of opening the filter",
        "internal/gui/parts/ring.go",
        "\tif c.Filtered {\n\t\tc.Tapped(nil)\n",
        "\tif false && c.Filtered {\n\t\tc.Tapped(nil)\n",
        "TestTypingAtTheShutFormatMenuOpensItsFilter",
    ),
    (
        "nothing of a value that matched the filter is drawn in bold",
        "internal/gui/parts/narrow.go",
        "e.from, e.to = at, at+len(want)",
        "e.from, e.to = at, at",
        "TestThePartOfAValueThatMatchedIsDrawnInBold",
    ),
    (
        "the shut format menu draws no picture of its value",
        "internal/gui/parts/menulook.go",
        "\tm.kind.Resource = m.menu.KindOf(m.menu.Selected)\n",
        "\tm.kind.Resource = nil\n",
        "TestTheShutFormatMenuDrawsThePictureOfItsValue",
    ),
    # --- after the review of #126 (2026-09-23, docs/REVIEW-126-2026-09-23.md) ---
    (
        "the smallest-size button comes back under a limit, where it leads to the same refusal",
        "internal/gui/parts/fixes.go",
        "parsed != nil || held != below.Requested {",
        "parsed != nil || held < 0 {",
        "TestALimitBelowTheMinimumOffersNoButtonThatLeadsBackToItself",
    ),
    (
        "the bar may be narrowed until the rail covers the centred buttons again",
        "internal/gui/parts/parts.go",
        "size.Width = fyne.Max(size.Width, need)",
        "size.Width = fyne.Max(size.Width, need*0)",
        # Repointed 2026-09-24: on a screen the form decides the smallest
        # window now, so only the bar laid out alone reaches this.
        "TestABarAtItsNarrowestHoldsTheRailAndTheButtonsSideBySide",
    ),
    (
        "the picture called recipe-refused is taken of a recipe with nothing wrong in it again",
        "internal/guard/screenpixels_test.go",
        "\t\t{name: \"recipe-refused\", tab: text.TabRecipe(), set: func(t *testing.T, s scene) {\n\t\t\temptyTheFirstBatch(t, s.tab)\n",
        "\t\t{name: \"recipe-refused\", tab: text.TabRecipe(), set: func(t *testing.T, s scene) {\n",
        "TestEveryPictureNamedAfterARefusalShowsOne",
    ),
    (
        "a refusal about a manifest already in the directory goes back to the one line strip at the foot",
        "internal/gui/window/runoffers.go",
        "\t\treturn filepath.Dir(collision.Path)\n",
        "\t\treturn filepath.Dir(collision.Path)[:0]\n",
        "TestARefusalAboutWhatIsInTheDirectoryStandsUnderItAndOpensIt",
    ),
    (
        "a refusal is laid inside its field's column again",
        "internal/gui/parts/grid.go",
        "\tarea.Resize(fyne.NewSize(f.areaWidth, area.MinSize().Height))\n",
        "\tarea.Resize(fyne.NewSize(size.Width, area.MinSize().Height))\n",
        "TestARefusalStandsUnderItsRowAcrossIt",
    ),
    (
        "a menu's arrow goes back to the colour of text",
        "internal/gui/parts/menulook.go",
        "theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNamePrimary)",
        "theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNameForeground)",
        "TestAMenuShowsItsArrowInTheAccent",
    ),
    (
        "a refusal about a field in a folded section leaves the section shut",
        "internal/gui/window/runrefuse.go",
        "\t\t\tr.sections.openHolding(field.Control)\n",
        "",
        "TestAFoldedSectionOpensForARefusalAboutItsField",
    ),
    (
        "the Space that opens the format list is typed into its filter as an invisible space",
        "internal/gui/parts/filterbox.go",
        "\tif f.Text == \"\" && unicode.IsSpace(r) {\n",
        "\tif false && f.Text == \"\" && unicode.IsSpace(r) {\n",
        "TestTheSpaceThatOpensTheFormatListIsNotTypedIntoItsFilter",
    ),
    (
        "a control registered again wraps its callback again and reports under every address it had",
        "internal/gui/parts/fields.go",
        "\tif !to.point(target) {\n\t\treturn\n\t}\n",
        "\tto.point(target)\n",
        "TestAControlRegisteredAgainReportsOnceUnderItsLatestAddress",
    ),
    (
        "a size box counts into the caption it was first drawn with, not the one on the screen",
        "internal/gui/parts/fields.go",
        "func(value string, into *ByteCount) { into.show(value) })",
        "func(value string, into *ByteCount) { count.show(value) })",
        "TestAControlRegisteredAgainReportsOnceUnderItsLatestAddress",
    ),
    (
        "a control registered again keeps reporting under the address it had first",
        "internal/gui/parts/fields.go",
        "\tw.target = target\n",
        "\tif !w.wired {\n\t\tw.target = target\n\t}\n",
        "TestAControlRegisteredAgainReportsOnceUnderItsLatestAddress",
    ),
    (
        "the batch id box is also registered as the file names, one control under two addresses",
        "internal/gui/window/recipe.go",
        "r.tips.Say(text.DetailNameTemplate()), parts.Text(b.name)),",
        "r.tips.Say(text.DetailNameTemplate()), parts.Text(b.id)),",
        "TestNoControlIsRegisteredUnderTwoAddressesAtOnce",
    ),
    # --- the same work done again leaves the window as it was (2026-09-25,
    #     docs/GUI-LEAK-TEST-2026-09-25.md section 7): the guard asks about the
    #     class, so each of the three counts is shown to go red on a defect
    #     of its own ---
    (
        "a control registered again wraps its callback again, so the same change reads the form more every round",
        "internal/gui/parts/fields.go",
        "\tif !to.point(target) {\n\t\treturn\n\t}\n",
        "\tto.point(target)\n",
        "TestTheSameWorkDoneAgainLeavesTheWindowAsItWas",
    ),
    (
        "the batch screen lays itself out again on top of the old panels instead of in their place",
        "internal/gui/window/recipe.go",
        "\tr.baseBox.RemoveAll()\n\tr.batchBox.RemoveAll()\n",
        "\tr.baseBox.RemoveAll()\n",
        "TestTheSameWorkDoneAgainLeavesTheWindowAsItWas",
    ),
    (
        "the worker of a run stays behind after reporting, waiting for a cancel nobody sends",
        "internal/gui/window/run.go",
        "\t\tclose(done)\n\t}()\n}\n\n// runFinished",
        "\t\tclose(done)\n\t\t<-ctx.Done()\n\t}()\n}\n\n// runFinished",
        "TestTheSameWorkDoneAgainLeavesTheWindowAsItWas",
    ),
    # --- the instructions beside the manifest (2026-09-25,
    #     docs/PRESET-INSTRUCTIONS-2026-09-25.md) - one break per rule ---
    (
        "the file name preset stops passing its purposes on, so fifty files say nothing about themselves",
        "internal/preset/filenamehandling.go",
        "\t\t\texpected: expected, reason: reason, purpose: c.purpose,\n",
        "\t\t\texpected: expected, reason: reason,\n",
        "TestEveryFileOfEveryPresetSaysWhatItIsFor",
    ),
    (
        "a name reaches the instructions without its invisible characters escaped",
        "internal/manifest/instructions.go",
        "\"- %s - %s, %s.\", codeSpan(core.Shown(f.Name)),",
        "\"- %s - %s, %s.\", codeSpan(f.Name),",
        "TestTheInstructionsShowEveryNameAsItIsAndNothingElse",
    ),
    (
        "a name holding a backtick is fenced by one backtick and read as markup",
        "internal/manifest/instructions.go",
        "\tfence := strings.Repeat(\"`\", longest+1)\n",
        "\tfence := strings.Repeat(\"`\", 1+0*longest)\n",
        "TestTheInstructionsShowEveryNameAsItIsAndNothingElse",
    ),
    (
        "a purpose written across two lines breaks the list it stands in",
        "internal/manifest/instructions.go",
        "\treturn core.Shown(strings.Join(strings.Fields(s), \" \"))\n",
        "\treturn core.Shown(s)\n",
        "TestTheInstructionsShowEveryNameAsItIsAndNothingElse",
    ),
    (
        "a file that failed is folded in with the files of its target that were written",
        "internal/manifest/instructions.go",
        "\treturn a.TargetID != \"\" && a.TargetID == b.TargetID && whole(a) && whole(b)\n",
        "\treturn a.TargetID != \"\" && a.TargetID == b.TargetID && (whole(a) || !whole(b))\n",
        "TestTheInstructionsFoldARunOfOneTargetIntoOneEntry",
    ),
    (
        "verify reports the instructions beside the manifest as a file nobody asked for",
        "internal/audit/audit.go",
        "\t\tif seen[p] || filepath.Base(p) == skip || (m.Run.Instructions != \"\" && filepath.Base(p) == m.Run.Instructions) {\n",
        "\t\tif seen[p] || filepath.Base(p) == skip {\n",
        "TestTheInstructionsGoWhereTheManifestGoes",
    ),
    (
        "cleanup --with-manifest takes the manifest and leaves its instructions behind",
        "internal/cli/cleanup.go",
        "\treturn append(record, full), \"\"\n",
        "\treturn record, \"\"\n",
        "TestTheInstructionsGoWhereTheManifestGoes",
    ),
    (
        "a run nobody explained writes instructions that can only say nothing",
        "internal/manifest/instructions.go",
        "\t\tif f.Purpose != \"\" {\n\t\t\treturn true\n",
        "\t\tif f.Purpose != \"\" || true {\n\t\t\treturn true\n",
        "TestARunWithNoPurposeWritesNoInstructions",
    ),
    (
        "instructions an earlier run left are not asked about before the first file",
        "internal/engine/preflight.go",
        "\tif explainedFiles(files) {\n",
        "\tif false && explainedFiles(files) {\n",
        "TestTheNameOfTheInstructionsIsHeldBeforeAnyFile",
    ),
    (
        "a target may take the name of the instructions beside the manifest",
        "internal/engine/engine.go",
        "\t\tpl.names[collisionKey(name)] = nameOwner{name: name, instructions: true}\n",
        "\t\t_ = nameOwner{name: name, instructions: true}\n",
        "TestTheNameOfTheInstructionsIsHeldBeforeAnyFile",
    ),
    (
        "a manifest naming its instructions outside the directory is read and acted on",
        "internal/manifest/manifest.go",
        "\tif name := m.Run.Instructions; name != \"\" && !isInstructionsName(name) {\n",
        "\tif name := m.Run.Instructions; false && name != \"\" && !isInstructionsName(name) {\n",
        "TestAManifestCannotSendCleanupAfterAFileOfSomebodyElses",
    ),
    (
        "instructions that could not be written are named in the manifest all the same",
        "internal/engine/record.go",
        "\t\t\trec.Missed = &InstructionsError{Path: path, Err: err}\n\t\t} else {\n",
        "\t\t\t_ = err\n\t\t\trec.Instructions = path\n\t\t\tm.Run.Instructions = filepath.Base(path)\n\t\t} else {\n",
        "TestInstructionsThatCannotBeWrittenAreSaidAndNotPretended",
    ),
    (
        "the window writes instructions and offers no way to open them",
        "internal/gui/window/run.go",
        "\tr.offer.instructions.show(saved.Instructions)\n",
        "",
        "TestTheWindowOpensTheInstructionsItsRunWrote",
    ),
    (
        "the smallest size of every format is worked out again at every expansion of the minimal set",
        "internal/format/registry.go",
        "\tsmallestMu.Unlock()\n\tif found {\n\t\treturn known\n",
        "\tsmallestMu.Unlock()\n\tif found && false {\n\t\treturn known\n",
        "TestTheMinimalSetIsWorkedOutOnceAndNotAtEveryExpansion",
    ),
    (
        "a sentence drawn ink tight goes back under a theme override, a scope and a font set of its own",
        "internal/gui/parts/words.go",
        "\treturn container.New(inkTightLayout{}, o)\n",
        "\treturn container.NewThemeOverride(o, Theme())\n",
        "TestNoScreenStandsInAThemeOverride",
    ),
    (
        "the live check reads the form a second time to mark the box",
        "internal/gui/window/runrefuse.go",
        "\tfor _, one := range spread(err) {\n\t\tvar about interface{ AboutSetting() string }\n",
        "\t_, _, err = r.settle()\n\tfor _, one := range spread(err) {\n\t\tvar about interface{ AboutSetting() string }\n",
        "TestOneChangeOfABoxReadsTheFormOnce",
    ),
    (
        "the live check stops putting what the form comes to on the line",
        "internal/gui/window/runrefuse.go",
        "\tlineFrom(r, targets, opt, err)\n\t// Only this box",
        "\t_, _ = targets, opt\n\t// Only this box",
        "TestTheLineSaysWhatTheFormComesTo",
    ),
    (
        "the batch screen reads its form without telling the host",
        "internal/gui/window/recipe.go",
        "\tr.runner.settle = watchedSettle(host, r.runner, r.settle)\n",
        "\tr.runner.settle = r.settle\n",
        "TestOneChangeOfABoxReadsTheFormOnce",
    ),
    (
        "the preset screen expands the preset whatever it remembers",
        "internal/gui/window/preset.go",
        "\tif l.held && l.id == id && maps.Equal(l.given, given) {\n",
        "\tif false && l.held && l.id == id && maps.Equal(l.given, given) {\n",
        "TestTypingWhatAPresetIsNotGivenDoesNotExpandItAgain",
    ),
    (
        "the preset screen keeps its expansion whatever the preset is given",
        "internal/gui/window/preset.go",
        "\tif l.held && l.id == id && maps.Equal(l.given, given) {\n",
        "\tif l.held && l.id == id {\n",
        "TestAChangedPresetValueIsExpandedAgain",
    ),
    (
        "the preset screen keeps its expansion whichever preset is chosen",
        "internal/gui/window/preset.go",
        "\tif l.held && l.id == id && maps.Equal(l.given, given) {\n",
        "\tif l.held && maps.Equal(l.given, given) {\n",
        "TestAChangedPresetValueIsExpandedAgain",
    ),
    (
        "the preset screen drops the notes of an expansion it remembered",
        "internal/gui/window/preset.go",
        "\t\treturn l.got, l.notes, l.err\n",
        "\t\treturn l.got, nil, l.err\n",
        "TestThePresetScreenSaysWhichNumbersWereOurs",
    ),
    (
        "the upload set works out a format's smallest size for every file about a name",
        "internal/preset/build.go",
        "if floor := format.SmallestWithLabel(desc); floor > sample {",
        "if floor := desc.SmallestAccepted(format.Request{Label: true}); floor > sample {",
        "TestANewUploadLimitDoesNotAskTheSameQuestionsAgain",
    ),
    (
        "the upload set plans the same picture under every name it holds",
        "internal/preset/uploadset.go",
        "\tif asked[key] {\n\t\treturn false\n\t}\n",
        "\tif false && asked[key] {\n\t\treturn false\n\t}\n",
        "TestANewUploadLimitDoesNotAskTheSameQuestionsAgain",
    ),
    (
        "a file asked at its floor has the floor worked out every time it is asked",
        "internal/preset/build.go",
        "return format.SmallestRemembered(f.desc, format.Request{Label: true, Properties: f.props})",
        "return f.desc.SmallestAccepted(format.Request{Label: true, Properties: f.props})",
        "TestANewRowCountWorksTheSheetsSmallestSizeOutOnce",
    ),
    (
        "the key of a request leaves out its seed",
        "internal/format/registry.go",
        "r.SizeFromContents, r.Label, r.Seed)",
        "r.SizeFromContents, r.Label, uint64(0))",
        "TestEveryFieldOfARequestIsInItsKey",
    ),
    (
        "the key of a request stops quoting the values of its settings",
        "internal/format/registry.go",
        "\t\tfmt.Fprintf(&b, \" %q=%q\", name, r.Properties[name])\n",
        "\t\tfmt.Fprintf(&b, \" %s=%s\", name, r.Properties[name])\n",
        "TestEveryFieldOfARequestIsInItsKey",
    ),
    (
        "a request with contents is keyed as if it had none",
        "internal/format/registry.go",
        "\tif len(r.Contains) > 0 {\n\t\treturn \"\", false\n\t}\n",
        "\tif false && len(r.Contains) > 0 {\n\t\treturn \"\", false\n\t}\n",
        "TestEveryFieldOfARequestIsInItsKey",
    ),
    (
        "laying the batch screen out says the line again, so a menu or a switch reads the form twice",
        "internal/gui/window/recipe.go",
        "\t// reading expanded it (docs/GUI-MEMORY-2026-09-23.md section 4j).\n}\n",
        "\t// reading expanded it (docs/GUI-MEMORY-2026-09-23.md section 4j).\n\tr.runner.refreshLine()\n}\n",
        "TestAChangeThatLaysTheBatchScreenOutAgainReadsTheFormOnce",
    ),
    (
        "a press that lays the batch screen out leaves the line describing the old form",
        "internal/gui/window/recipe.go",
        "func afterAPress(r *Recipe) {\n\tr.rebuild()\n\tr.runner.refreshLine()\n}\n",
        "func afterAPress(r *Recipe) {\n\tr.rebuild()\n}\n",
        "TestAChangeThatLaysTheBatchScreenOutAgainReadsTheFormOnce",
    ),
    (
        "the base switched off with no batch left brings the batch back through the press, which says the line a second time",
        "internal/gui/window/recipebase.go",
        "\t\t\tnewBatchAtTheEnd(r)\n\t\t}\n\t\tr.rebuild()\n",
        "\t\t\tr.addBatch()\n\t\t}\n\t\tr.rebuild()\n",
        "TestAChangeThatLaysTheBatchScreenOutAgainReadsTheFormOnce",
    ),
    (
        "something done while a release waits does not call it off, so memory goes back in the middle of work",
        "internal/gui/window/tidy.go",
        "func (t *tidy) touch() {\n\tt.Stop()\n\tt.after(",
        "func (t *tidy) touch() {\n\tt.after(",
        "TestSomethingDoneDuringTheWaitStartsTheQuietOver",
    ),
    (
        "memory is given back while a run owns the window",
        "internal/gui/window/tidy.go",
        "func (t *tidy) release() {\n\tif t.busy() {\n\t\tt.touch()\n\t\treturn\n\t}\n",
        "func (t *tidy) release() {\n",
        "TestNoMemoryIsGivenBackWhileWorkIsGoing",
    ),
    (
        "the quiet ends before the toolkit's renderers expire",
        "internal/gui/window/tidy.go",
        "const tidyAfterQuiet = 70 * time.Second",
        "const tidyAfterQuiet = 50 * time.Second",
        "TestTheWindowGivesMemoryBackOnceItHasBeenLeftAlone",
    ),
    (
        "memory goes back sooner than the toolkit's next clean after the frame",
        "internal/gui/window/tidy.go",
        "const tidyAfterFrame = 12 * time.Second",
        "const tidyAfterFrame = 2 * time.Second",
        "TestTheWindowGivesMemoryBackOnceItHasBeenLeftAlone",
    ),
    (
        "memory is given back on the window's thread, which once took 2491 ms",
        "internal/gui/window/tidy.go",
        "\tgo debug.FreeOSMemory()\n",
        "\tdebug.FreeOSMemory()\n",
        "TestConcurrencyStaysWhereItWasPutOnPurpose",
    ),
    (
        "a reading of the form does not tell the wait for quiet, so the window never gives memory back",
        "internal/gui/window/open.go",
        "\t\tif r.touched != nil {\n\t\t\tr.touched()\n\t\t}\n",
        "",
        "TestTheWindowGivesMemoryBackOnceItHasBeenLeftAlone",
    ),
    (
        "the concurrency scan stops seeing a timer's callback",
        "internal/guard/concurrency_test.go",
        "\tcase id.Name == \"time\" && node.Sel.Name == \"AfterFunc\":\n",
        "\tcase false && id.Name == \"time\" && node.Sel.Name == \"AfterFunc\":\n",
        "TestTheConcurrencyScanSeesEachKindItLooksFor",
    ),
    (
        "the window's clock stands outside the list of concurrent files",
        "internal/guard/concurrency_test.go",
        "\t\"internal/gui/run_cgo.go\": ",
        "\t\"internal/gui/run_cgo_elsewhere.go\": ",
        "TestConcurrencyStaysWhereItWasPutOnPurpose",
    ),
    (
        "a wait called off on its way is not asked whether it still belongs, so the quiet somebody broke gives memory back",
        "internal/gui/window/tidy.go",
        "\t\tif t.wait != mine {\n",
        "\t\tif t.wait < mine {\n",
        "TestAWaitCalledOffOnItsWayDoesNothingWhenItArrives",
    ),
    (
        "calling the wait off leaves a call already on its way thinking it is still the one waited for",
        "internal/gui/window/tidy.go",
        "func (t *tidy) Stop() {\n\tt.wait++\n",
        "func (t *tidy) Stop() {\n",
        "TestAWaitCalledOffOnItsWayDoesNothingWhenItArrives",
    ),
    (
        "a declared concurrent file that is gone is not reported",
        "internal/guard/concurrency_test.go",
        "filepath.FromSlash(rel))); err != nil {\n",
        "filepath.FromSlash(rel))); err != nil && false {\n",
        "TestADeclarationWhoseFileIsGoneIsReported",
    ),
    (
        "the formats table puts the identifier where the name goes",
        "internal/cli/formats.go",
        "d.ID, named, d.Name, d.Fidelity",
        "d.ID, named, d.ID, d.Fidelity",
        "TestEveryFormatIsNamedWhereverTheCommandLineListsIt",
    ),
    (
        "the card for one format stops giving its name",
        "internal/cli/formats.go",
        "\tfmt.Fprintf(out, \"  name       %s\\n\", d.Name)\n",
        "\t_ = d.Name\n",
        "TestEveryFormatIsNamedWhereverTheCommandLineListsIt",
    ),
    (
        "the machine readable list leaves the name out",
        "internal/cli/formats.go",
        "\t\tName:     d.Name,\n",
        "\t\tName:     \"\",\n",
        "TestEveryFormatIsNamedWhereverTheCommandLineListsIt",
    ),
    (
        "a format is registered with no name",
        "internal/format/jxl/jxl.go",
        "\t\tName:        \"JPEG XL\",\n",
        "\t\tName:        \"\",\n",
        "TestEveryFormatDeclaresTheFullSet",
    ),
    (
        "a format name carries a semicolon the prose rule refuses",
        "internal/format/targz/targz.go",
        "\t\tName:        \"tar + gzip\",\n",
        "\t\tName:        \"tar; gzip\",\n",
        "TestEveryFormatDeclaresTheFullSet",
    ),
    (
        "a format name carries a letter the command line cannot print",
        "internal/format/bmp/bmp.go",
        "\t\tName:        \"Windows Bitmap\",\n",
        "\t\tName:        \"Windows Bitmäp\",\n",
        "TestEveryFormatDeclaresTheFullSet",
    ),
    (
        "a format name starts with a space",
        "internal/format/webp/webp.go",
        "\t\tName:        \"WebP\",\n",
        "\t\tName:        \" WebP\",\n",
        "TestEveryFormatDeclaresTheFullSet",
    ),
    (
        "the open list of formats stops naming its values",
        "internal/gui/parts/ring.go",
        "\t\tlist.NameEach(c.NameOf)\n",
        "\t\t_ = c.NameOf\n",
        "TestTheFormatListNamesEveryFormatBesideItsIdentifier",
    ),
    (
        "each name starts right after its own value, so the names are no column",
        "internal/gui/parts/listrow.go",
        "\tat := r.row.column + listNameGap\n",
        "\tat := r.value().width() + listNameGap\n",
        "TestTheFormatListNamesEveryFormatBesideItsIdentifier",
    ),
    (
        "a name is drawn in the ink of the value beside it",
        "internal/gui/parts/listrow.go",
        "\tr.name.whole.Color = PaletteColour(ColorNameLabel, theme.VariantDark)\n",
        "\tr.name.whole.Color = Theme().Color(theme.ColorNameForeground, theme.VariantDark)\n",
        "TestTheFormatListNamesEveryFormatBesideItsIdentifier",
    ),
    (
        "the list of formats opens at the width of its box again",
        "internal/gui/parts/ring.go",
        "ColumnForList(surface.Size().Width, at.X, c.Size().Width, listWidth(c, c.Size().Width))",
        "ColumnForList(surface.Size().Width, at.X, c.Size().Width, c.Size().Width)",
        "TestTheFormatListIsWiderThanItsBoxAndCutsNoName",
    ),
    (
        "the width of the list leaves the names out, so they are cut off",
        "internal/gui/parts/ring.go",
        "RowWidthFor(column+listNameGap+name, c.KindOf != nil)",
        "RowWidthFor(column+0*name, c.KindOf != nil)",
        "TestTheFormatListIsWiderThanItsBoxAndCutsNoName",
    ),
    (
        "a list wider than the room beside its box runs past the window's edge",
        "internal/gui/parts/ring.go",
        "\t\tleft = fyne.Max(listEdgeGap, end-width)\n",
        "\t\tleft = boxLeft\n",
        "TestAListWiderThanTheRoomBesideItsBoxMovesLeft",
    ),
    (
        "a list no wider than its box is moved off it near the edge",
        "internal/gui/parts/ring.go",
        "width > boxWidth && left+width > end {",
        "left+width > end {",
        "TestAListWiderThanTheRoomBesideItsBoxMovesLeft",
    ),
    (
        "a list wider than the whole window is not cut to it",
        "internal/gui/parts/ring.go",
        "width > room && width > boxWidth {",
        "false && width > room && width > boxWidth {",
        "TestAListWiderThanTheRoomBesideItsBoxMovesLeft",
    ),
    (
        "the filter stops reading the names",
        "internal/gui/parts/narrow.go",
        "(by.nameOf != nil && aWordStartsWith(by.nameOf(v), want))",
        "(by.nameOf != nil && false)",
        "TestTheFilterFindsAFormatByAWordOfItsName",
    ),
    (
        "words part only at spaces again, so a word after a bracket or a hyphen is not found",
        "internal/gui/parts/narrow.go",
        "func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }",
        "func wordRune(r rune) bool { return !unicode.IsSpace(r) }",
        "TestTheFilterFindsAFormatByAWordOfItsName",
    ),
    (
        "every letter of a name counts as the start of a word",
        "internal/gui/parts/narrow.go",
        "; at > 0 && wordRune(before) {",
        "; false && at > 0 && wordRune(before) {",
        "TestTheFilterFindsAFormatByAWordOfItsName",
    ),
    (
        "the sentence saying nothing matched is drawn as a heading again",
        "internal/gui/parts/listrow.go",
        "\tcase r.row.notice:\n",
        "\tcase false && r.row.notice:\n",
        "TestTheListSaysWhatToDoWhenNothingMatches",
    ),
    (
        "a quiet button switched off goes back to the brighter disabled ink",
        "internal/gui/parts/button.go",
        "\t\t\tborder.ink = theme.ColorNameInputBorder\n",
        "\t\t\tborder.ink = theme.ColorNameDisabled\n",
        "TestAControlSwitchedOffIsNeverBrighterThanAtRest",
    ),
    (
        "a switch switched off goes back to the bright disabled edge",
        "internal/gui/parts/toggle.go",
        "\t\tr.square.StrokeColor = PaletteColour(theme.ColorNameSeparator, dark)\n",
        "\t\tr.square.StrokeColor = PaletteColour(theme.ColorNameDisabled, dark)\n",
        "TestAControlSwitchedOffIsNeverBrighterThanAtRest",
    ),
    (
        "a box switched off keeps the toolkit's bright border",
        "internal/gui/parts/entry.go",
        "\tif !q.entry.Disabled() {\n",
        "\tif true {\n",
        "TestAControlSwitchedOffIsNeverBrighterThanAtRest",
    ),
    (
        "a ticked switch that is off hides its tick again",
        "internal/gui/parts/toggle.go",
        "\tcase off && r.toggle.Checked:\n",
        "\tcase false && off && r.toggle.Checked:\n",
        "TestASwitchThatIsOffStillShowsItsValue",
    ),
    (
        "a button's edge is stroked across its bounds again, a pixel taller than the box beside it",
        "internal/gui/parts/button.go",
        "\tif edge > 0 {\n",
        "\tif false && edge > 0 {\n",
        "TestAButtonBesideABoxIsDrawnAsTallAsTheBox",
    ),
    (
        "the heart follows the words' ink instead of red",
        "internal/gui/parts/heart.go",
        "\tb.iconInk = theme.ColorNameError\n",
        "\tb.iconInk = theme.ColorNamePlaceHolder\n",
        "TestDonateDrawsARedHeartThatGoesOutWhenOff",
    ),
    (
        "a button switched off keeps its own inks",
        "internal/gui/parts/button.go",
        "\tif r.state() != stateDisabled {\n",
        "\tif true {\n",
        "TestDonateDrawsARedHeartThatGoesOutWhenOff",
    ),
    (
        "the rail stands on the form's edge by its invisible box rather than by its words",
        "internal/gui/parts/parts.go",
        "hungOut{by: BarButtonInsetX}",
        "hungOut{by: 0}",
        "TestTheBarsRailStandsOnTheFormsLeftEdge",
    ),
    (
        "the run buttons stay centred however narrow, running under the rail",
        "internal/gui/parts/parts.go",
        "\tkeep.clearLeft = room\n",
        "\tkeep.clearLeft = 0\n",
        "TestTheBarsButtonsClearItsRailInTheNarrowestWindow",
    ),
    (
        "a group's rail goes back to the primary colour",
        "internal/gui/parts/folding.go",
        "\trail := canvas.NewRectangle(PaletteColour(ColorNameLabel, theme.VariantDark))\n",
        "\trail := canvas.NewRectangle(PaletteColour(theme.ColorNamePrimary, theme.VariantDark))\n",
        "TestAGroupOfSettingsIsFramedWithTheOneNeutralRail",
    ),
    (
        "a refusal starts at its own field's edge again, and two of them step down",
        "internal/gui/parts/grid.go",
        "\t\tcell.areaLeft = -x\n",
        "\t\tcell.areaLeft = 0 * x\n",
        "TestARefusalStandsUnderItsRowAcrossIt",
    ),
    (
        "a list opened downward keeps the height of the whole list while it is filtered",
        "internal/gui/parts/ring.go",
        "\t\tlist.resize = pop.Resize\n",
        "\t\t_ = pop.Resize\n",
        "TestAListOpenedDownwardShrinksToWhatTheFilterLeft",
    ),
    (
        "a list opened upward shrinks too, moving its filter under the hands",
        "internal/gui/parts/ring.go",
        "\tif top >= at.Y+c.Size().Height {\n",
        "\tif true {\n",
        "TestAListOpenedUpwardKeepsOneHeight",
    ),
    (
        "the card an open list floats on loses its edge",
        "internal/gui/parts/parts.go",
        "\tface.StrokeWidth = edgeWidth\n",
        "\tface.StrokeWidth = 0\n",
        "TestAnOpenListIsToldFromTheFormBehindIt",
    ),
    (
        "the card an explanation floats on casts no shade",
        "internal/gui/parts/parts.go",
        "\treturn []fyne.CanvasObject{container.New(shifted{dy: TipShadowDrop}, shade), face}\n",
        "\treturn []fyne.CanvasObject{face}\n",
        "TestTheExplanationFloatsOnTheSurfaceAnOpenListDoes",
    ),
    (
        "a rail that grows in place leaves the run buttons under it again",
        "internal/gui/parts/parts.go",
        "\tif moved {\n\t\trow.Layout.Layout(row.Objects, row.Size())",
        "\tif moved && false {\n\t\trow.Layout.Layout(row.Objects, row.Size())",
        "TestARailThatGrowsInPlaceMovesTheRunButtonsOnWithIt",
    ),
    (
        "the palette's sentence for menuBackground runs past its section again",
        "internal/gui/catalogue/palette.go",
        "\"the toolkit's own menus - the one a right press opens in a box. Our lists float on a card\"",
        "\"what the toolkit's own menus float on - the one a box opens on a right press. This window's lists float on a card since 2026-09-24\"",
        "TestNoWordsInTheCatalogueRunPastTheEdgeOfItsSections",
    ),
    (
        "a long name's temporary form is the plain join again, too long for any file system",
        "internal/core/sibling.go",
        "if len(name)+len(suffix) <= MaxNameBytes {",
        "if true {",
        "TestANameAsLongAsEverySystemStoresIsWritten",
    ),
    (
        "two long names that begin alike share one temporary name again",
        "internal/core/sibling.go",
        "sum := sha256.Sum256([]byte(name))",
        "sum := sha256.Sum256(nil)",
        "TestTwoLongNamesThatBeginAlikeGetTwoSiblings",
    ),
    (
        "a long name is cut part way through a character again",
        "internal/core/sibling.go",
        "for n > 0 && !utf8.RuneStart(s[n]) {",
        "for n > 0 && !utf8.RuneStart(s[n]) && false {",
        "TestTwoLongNamesThatBeginAlikeGetTwoSiblings",
    ),
    (
        "the marker of a shortened leftover is no longer last, so verify calls it extra",
        "internal/core/sibling.go",
        "return cutToWholeCharacters(name, MaxNameBytes-len(suffix)-len(tag)) + tag + suffix",
        "return cutToWholeCharacters(name, MaxNameBytes-len(suffix)-len(tag)) + suffix + tag",
        "TestALeftoverUnderAShortenedNameIsNamedForWhatItIs",
    ),
    (
        "the engine joins the temporary name the plain way again",
        "internal/engine/preflight.go",
        'return core.SiblingName(name, fmt.Sprintf("%s%d", core.PartialMarker, os.Getpid()))',
        'return name + fmt.Sprintf("%s%d", core.PartialMarker, os.Getpid())',
        "TestANameAsLongAsEverySystemStoresIsWritten",
    ),
    (
        "a name longer than every system stores is written again where it can be",
        "internal/engine/filename.go",
        "case len(name) > core.MaxNameBytes:",
        "case len(name) > core.MaxNameBytes && false:",
        "TestANameLongerThanEverySystemStoresIsRefusedBeforeAnythingIsWritten",
    ),
    (
        "a name exactly as long as every system stores is refused a byte too early",
        "internal/engine/filename.go",
        "case len(name) > core.MaxNameBytes:",
        "case len(name) >= core.MaxNameBytes:",
        "TestANameLongerThanEverySystemStoresIsRefusedBeforeAnythingIsWritten",
    ),
    (
        "formatting a recipe in place joins its temporary name the plain way again",
        "internal/core/replace.go",
        "tmp := SiblingPath(path, writingSuffix)",
        "tmp := path + writingSuffix",
        "TestARecipeNamedAsLongAsEverySystemStoresCanBeReplacedInPlace",
    ),
    (
        # O243, measured 2026-09-24: TrimSpace took an ideographic space off
        # the front of a name and the file was written under another name.
        "a recipe value loses Unicode white space at its ends again",
        "internal/recipe/scalar.go",
        "\tt := strings.Trim(string(b), yamlBlank)",
        "\tt := strings.TrimSpace(string(b))",
        "TestAUnicodeSpaceAtEitherEndOfANameIsKept",
    ),
    (
        # O244, measured 2026-09-24: an ejected recipe carried a right to
        # left override and a line separator raw, and PyYAML refused it.
        "a composed value holding a character nobody can see is written raw again",
        "internal/recipe/compose.go",
        "\treturn escapedText(value)",
        "\treturn value",
        "TestACharacterNobodyCanSeeIsWrittenAsAnEscapeAndReadBackAsItself",
    ),
    (
        # The quotes alone are not the repair. Single quotes carry the
        # character raw, which is the document PyYAML refused.
        "the quoted form keeps the character raw inside the quotes",
        "internal/recipe/compose.go",
        "\treturn []byte(strconv.Quote(string(e))), nil",
        "\treturn []byte(\"'\" + string(e) + \"'\"), nil",
        "TestACharacterNobodyCanSeeIsWrittenAsAnEscapeAndReadBackAsItself",
    ),
    (
        # O241, measured 2026-09-24: verify drew "photo", a right to left
        # override, "gpj.txt" as phototxt.jpg.
        "the primitive writes a character nobody can see as it is",
        "internal/core/unseen.go",
        "\t\tcase !strconv.IsPrint(r):\n",
        "\t\tcase false:\n",
        "TestVerifyShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "verify prints a difference without showing its names",
        "internal/audit/audit.go",
        "\td.Path, d.Want, d.Got = core.Shown(d.Path), core.Shown(d.Want), core.Shown(d.Got)\n",
        "",
        "TestVerifyShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "cleanup previews a removal without showing the name",
        "internal/cli/cleanup.go",
        "\"  remove %s\\n\", core.Shown(c.Path))",
        "\"  remove %s\\n\", c.Path)",
        "TestCleanupShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "cleanup previews a file it keeps without showing the name",
        "internal/cli/cleanup.go",
        "\"  keep   %s - %s\\n\", core.Shown(c.Path), skipNote(c, force))",
        "\"  keep   %s - %s\\n\", c.Path, skipNote(c, force))",
        "TestCleanupShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "cleanup reports a file it kept without showing the name",
        "internal/cli/cleanup.go",
        "\"kept %s - %s\\n\", core.Shown(o.Path), core.Shown(o.Reason))",
        "\"kept %s - %s\\n\", o.Path, core.Shown(o.Reason))",
        "TestCleanupShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "a note about one file prints its name raw",
        "internal/manifest/manifest.go",
        "core.A(\"File\", core.Shown(g.First[0]))",
        "core.A(\"File\", g.First[0])",
        "TestANoteAboutAFileShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "a note about several files prints their names raw",
        "internal/manifest/manifest.go",
        "named := strings.Join(core.ShownEach(g.First), \", \")",
        "named := strings.Join(g.First, \", \")",
        "TestANoteAboutAFileShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "a refusal of two names differing in case prints them raw",
        "internal/engine/names.go",
        "which differ only in case. Most filesystems treat those as one file, so one would be written over the other and the manifest would describe both\",\n\t\t\tcore.A(\"First\", owner.id), core.A(\"Second\", id), core.A(\"FirstName\", core.Shown(owner.name)), core.A(\"SecondName\", core.Shown(name)))",
        "which differ only in case. Most filesystems treat those as one file, so one would be written over the other and the manifest would describe both\",\n\t\t\tcore.A(\"First\", owner.id), core.A(\"Second\", id), core.A(\"FirstName\", owner.name), core.A(\"SecondName\", name))",
        "TestTheWindowsWordsForARefusalShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "a refusal of a taken name prints the path raw",
        "internal/engine/errors.go",
        "remove the file first\",\n\t\tcore.A(\"Path\", core.Shown(e.Path)))",
        "remove the file first\",\n\t\tcore.A(\"Path\", e.Path))",
        "TestTheWindowsWordsForARefusalShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "the files of a boundary set are listed raw",
        "internal/cli/generate.go",
        "\"  %-26s %s\\n\", core.Shown(f.Name), ",
        "\"  %-26s %s\\n\", f.Name, ",
        "TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "the line naming a run's manifest prints the path raw",
        "internal/cli/record.go",
        "fmt.Fprintf(errOut, \"manifest: %s\\n\", core.Shown(path))",
        "fmt.Fprintf(errOut, \"manifest: %s\\n\", path)",
        "TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "a note about another run lists its files raw",
        "internal/cli/verify.go",
        "\tnames = core.ShownEach(names)\n",
        "",
        "TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "a note about another run names its record raw",
        "internal/cli/verify.go",
        "\t\t\tcore.Shown(name), core.Count(len(files), \"file\", \"files\"), belongs(len(files)), someOf(files))",
        "\t\t\tname, core.Count(len(files), \"file\", \"files\"), belongs(len(files)), someOf(files))",
        "TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "a refusal of an output directory that is a file prints it raw",
        "internal/engine/preflight.go",
        "\"the output directory %s is a file, not a directory\", core.A(\"Dir\", core.Shown(opt.OutDir)))",
        "\"the output directory %s is a file, not a directory\", core.A(\"Dir\", opt.OutDir))",
        "TestTheWindowsWordsForARefusalShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "a refusal of a held directory prints it raw",
        "internal/engine/errors.go",
        "core.A(\"Dir\", core.Shown(e.Dir)), core.A(\"Lock\", core.Shown(e.Path)))",
        "core.A(\"Dir\", e.Dir), core.A(\"Lock\", e.Path))",
        "TestTheWindowsWordsForARefusalShowANameNobodyCanReadAsAnEscape",
    ),
    (
        # A right to left override typed into a comment of shipped code - the
        # Trojan Source shape this guard is for.
        "a character nobody can see gets into a tracked file",
        "internal/core/unseen.go",
        "// HoldsUnseen reports whether s holds a character a person reading it cannot",
        "// HoldsUnseen reports whether s holds a character a person\u202e reading it cannot",
        "TestNoTrackedFileCarriesACharacterNobodyCanSee",
    ),
    (
        "the detector stops recognising a character nobody can see",
        "internal/guard/hiddencharacters_test.go",
        "\tcase unseenHere(r):",
        "\tcase false:",
        "TestTheHiddenCharacterDetectorFindsWhatItIsFor",
    ),
    (
        "a preset applies a default other than the one it declares for --format",
        "internal/preset/sizeboundaries.go",
        "formatID := defaultFormat",
        "formatID := \"txt\"",
        "TestEveryFlagAPresetReadsDefaultsToOneValueOnBothSurfaces",
    ),
    (
        "the full width extension is written in plain letters",
        "internal/preset/filenamehandling.go",
        "\t\t\tr += 0xFF01 - '!'",
        "\t\t\tr += 0",
        "TestTheFileNamePresetAsksForEveryNameInEveryFormat",
    ),
    (
        "the decomposed names are written composed",
        "internal/preset/filenamehandling.go",
        "\t\tstem = norm.NFD.String(stem)",
        "\t\tstem = norm.NFC.String(stem)",
        "TestTheFileNamePresetAsksForEveryNameInEveryFormat",
    ),
    (
        "a name about length comes out a copy short",
        "internal/preset/filenamehandling.go",
        "\t\tcopies := (c.bytes - len(ext)) / len(c.fill)",
        "\t\tcopies := (c.bytes-len(ext))/len(c.fill) - 1",
        "TestTheFileNamePresetAsksForEveryNameInEveryFormat",
    ),
    (
        "a name that means something only as itself gets the format's extension",
        "internal/preset/filenamehandling.go",
        "\tcase noExtension:\n\t\tsuffix = \"\"",
        "\tcase noExtension:\n\t\tsuffix = ext",
        "TestTheFileNamePresetAsksForEveryNameInEveryFormat",
    ),
    (
        "the names a system must take stop being promised",
        "internal/preset/filenamehandling.go",
        "\t\treturn \"accept\", \"\"",
        "\t\treturn \"unspecified\", \"filename_invalid\"",
        "TestTheFileNamePresetPromisesOnlyWhatASystemMustDo",
    ),
    (
        # The one long name the preset asks for, 255 bytes, needs the short
        # temporary name of O239 to be written at all.
        "a name of 255 bytes is written under a temporary name too long again",
        "internal/core/sibling.go",
        "\tif len(name)+len(suffix) <= MaxNameBytes {",
        "\tif true {",
        "TestTheFileNamePresetWritesEveryNameByteForByte",
    ),
    (
        "the preset ignores a format that is named",
        "internal/preset/filenamehandling.go",
        "\t\tformatID = v\n",
        "\t\tformatID = \"pdf\"\n",
        "TestThePresetScreenWritesTheNamesTheCommandLineWrites",
    ),
    (
        # Provable only since a second preset reads --format with another
        # default: with one, every default the window could take was right.
        "the window opens a flag a preset reads on some other default",
        "internal/preset/preset.go",
        "\t\t\tdeclared.Default = p.ReadDefaults[name]",
        "\t\t\tdeclared.Default = \"pdf\"",
        "TestEveryFlagAPresetReadsDefaultsToOneValueOnBothSurfaces",
    ),
    (
        # Review of #138, 2026-09-25: a system error wrapped under our sentence
        # repeated the output directory raw.
        "a message of the command line is printed without escaping",
        "internal/cli/errors.go",
        "\treturn core.ShownText(inOurWords(err))",
        "\treturn inOurWords(err)",
        "TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape",
    ),
    (
        "verify names the directory raw in its mismatch line",
        "internal/cli/verify.go",
        "\"tfg: %s does not match %s - %s:\\n\", core.Shown(dir), core.Shown(path), ",
        "\"tfg: %s does not match %s - %s:\\n\", dir, path, ",
        "TestVerifyShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "cleanup names the directory raw in its preview",
        "internal/cli/cleanup.go",
        "core.Count(countRemovable(cands, force), \"file\", \"files\"), core.Shown(run.dir))",
        "core.Count(countRemovable(cands, force), \"file\", \"files\"), run.dir)",
        "TestCleanupShowsANameNobodyCanReadAsAnEscape",
    ),
    (
        "the window's refusal at the foot of the form is raw",
        "internal/gui/window/runrefuse.go",
        "loose = append(loose, core.ShownText(text.Refusal(one, \"\")))",
        "loose = append(loose, text.Refusal(one, \"\"))",
        "TestTheWindowShowsADirectoryNobodyCanReadAsAnEscape",
    ),
    (
        # Review of #138: nothing asked the names whose point is an ASCII
        # character at the edge, so trimming one would have passed.
        "the leading space of a name is lost",
        "internal/preset/filenamehandling.go",
        "stem: \" leading space\",",
        "stem: \"leading space\",",
        "TestTheFileNamePresetAsksForEveryNameInEveryFormat",
    ),
    # --- #140 review: cleanup --with-manifest says what it takes besides the
    # files, in the preview and in the run (TestCleanupNamesTheRecordBeforeItTakesIt,
    # TestCleanupSaysWhyTheRecordStays, TestInstructionsAlreadyGoneHoldNothingBack),
    # and the two guards CI found red on the branch.
    (
        "the cleanup preview goes back to naming only the files the manifest lists",
        "internal/cli/cleanup.go",
        "record := previewRecord(cands, run)",
        "record := previewRecord(cands, cleanupRun{})",
        "TestCleanupNamesTheRecordBeforeItTakesIt",
    ),
    (
        "the run stops saying what it did with the manifest and the instructions",
        "internal/cli/cleanup.go",
        'sayRecord(out, report.Record, "removed", "kept   ")',
        'sayRecord(io.Discard, report.Record, "removed", "kept   ")',
        "TestCleanupNamesTheRecordBeforeItTakesIt",
    ),
    (
        "a file that will stay no longer holds the record back in the preview",
        "internal/cli/cleanup.go",
        "if !c.Removable(run.force) && c.Disposition != audit.Absent {",
        "if !c.Removable(run.force) && c.Disposition == audit.Ready {",
        "TestCleanupSaysWhyTheRecordStays",
    ),
    (
        "a record held back by a file left behind is reported as removed",
        "internal/cli/cleanup.go",
        'Action: "kept", Reason: keptBecause(i, blocked)})',
        'Action: "removed", Reason: keptBecause(i, blocked)})',
        "TestCleanupSaysWhyTheRecordStays",
    ),
    (
        "a record that would not go ends the run with no report",
        "internal/cli/cleanup.go",
        "return writeJSON(errOut, errOut, report, code)",
        "return code",
        "TestCleanupSaysWhyTheRecordStays",
    ),
    (
        "instructions already gone are promised for removal in the preview",
        "internal/cli/cleanup.go",
        "case i > 0 && isGone(p):",
        "case i > 1 && isGone(p):",
        "TestInstructionsAlreadyGoneHoldNothingBack",
    ),
    (
        "instructions already gone are reported as removed by the run",
        "internal/cli/cleanup.go",
        'entries[i].Action, entries[i].Reason = "kept", "it is already gone"',
        'entries[i].Action, entries[i].Reason = "removed", ""',
        "TestInstructionsAlreadyGoneHoldNothingBack",
    ),
    (
        "the purpose box takes the whole row of the manifest notes again",
        "internal/gui/window/recipefolds.go",
        "\t\tadd(recipe.KeyPurpose, text.FieldPurpose(), text.HintPurpose(),\n\t\t\tr.tips.Say(text.DetailPurpose()), parts.Text(n.purpose)),",
        "\t\tparts.Wide(add(recipe.KeyPurpose, text.FieldPurpose(), text.HintPurpose(),\n\t\t\tr.tips.Say(text.DetailPurpose()), n.purpose)),",
        "TestOnlyAPathTakesTheWholeRow",
    ),
    # --- O245: eject writes the file itself, and a UTF-16 recipe says where it
    # came from (internal/guard/ejectfile_test.go).
    (
        "eject -o puts the recipe on standard output as well as into the file",
        "internal/cli/presetcmd.go",
        "\tif to.set {\n\t\treturn writeEjected(to.name, expanded.Source, errOut)\n\t}",
        "\tif to.set {\n\t\t_ = writeEjected(to.name, expanded.Source, errOut)\n\t}",
        "TestEjectWritesTheFileByteForByteWhatItWouldPrint",
    ),
    (
        "eject -o takes a dash for the name of a file",
        "internal/cli/presetcmd.go",
        'if to.set && (to.name == "" || to.name == "-") {',
        'if to.set && to.name == "" {',
        "TestEjectRefusesAFileNameThatIsNotOne",
    ),
    (
        "a UTF-16 recipe gets the general refusal again",
        "internal/recipe/recipe.go",
        "if isUTF16(src) {",
        "if isUTF16(src[:0]) {",
        "TestARecipeSavedAsUTF16IsRefusedAndSaysWhere",
    ),
    (
        "only one byte order of UTF-16 is recognised",
        "internal/recipe/canonical.go",
        'return strings.HasPrefix(s, "\\xff\\xfe") || strings.HasPrefix(s, "\\xfe\\xff")',
        'return strings.HasPrefix(s, "\\xff\\xfe")',
        "TestARecipeSavedAsUTF16IsRefusedAndSaysWhere",
    ),
    (
        "the command line stops naming the way round a UTF-16 recipe",
        "internal/cli/errors.go",
        "if errors.As(err, &syntax) && syntax.UTF16 {",
        "if errors.As(err, &syntax) && !syntax.UTF16 {",
        "TestARecipeSavedAsUTF16IsRefusedAndSaysWhere",
    ),
    # --- #140 second review: cleanup takes only the instructions named after
    # its manifest, and the window escapes the instructions it could not write.
    (
        "cleanup takes instructions named after another manifest",
        "internal/cli/cleanup.go",
        "if name != manifest.InstructionsName(filepath.Base(path)) {",
        "if name == manifest.InstructionsName(filepath.Base(path))+\"/\" {",
        "TestCleanupLeavesInstructionsNotNamedAfterItsManifest",
    ),
    (
        "instructions cleanup left alone vanish from the run's report",
        "internal/cli/cleanup.go",
        "if run.notOurs != \"\" {\n\t\treport.Record = append(",
        "if run.notOurs == \"/\" {\n\t\treport.Record = append(",
        "TestCleanupLeavesInstructionsNotNamedAfterItsManifest",
    ),
    (
        "the preview stops naming instructions cleanup will leave alone",
        "internal/cli/cleanup.go",
        "if run.notOurs != \"\" {\n\t\tentries = append(",
        "if run.notOurs == \"/\" {\n\t\tentries = append(",
        "TestCleanupLeavesInstructionsNotNamedAfterItsManifest",
    ),
    (
        "the window names the instructions it could not write raw",
        "internal/gui/window/run.go",
        "text.InstructionsNotSaved(core.Shown(saved.Missed.Path), core.ShownText(text.Refusal(saved.Missed.Err, \"\")))",
        "text.InstructionsNotSaved(saved.Missed.Path, core.ShownText(text.Refusal(saved.Missed.Err, \"\")))",
        "TestTheWindowSaysInstructionsItCouldNotWriteWithTheEscape",
    ),
    # --- the WinGet and Chocolatey packages and their renderer (2026-09-25) ---
    (
        "the Chocolatey nuspec is written under the template's name instead of the package's",
        ".github/scripts/build_packages.py",
        '        rest = package.choco_id + ".nuspec"',
        '        rest = "package.nuspec"',
        "TestThePackagesRenderFromAReleasesChecksums",
    ),
    (
        "a WinGet template carries a version typed by hand",
        "packaging/winget/version.yaml.in",
        "PackageVersion: {{VERSION}}",
        "PackageVersion: 0.4.0",
        "TestNoPackageSourceCarriesAVersionNumber",
    ),
    (
        "WinGet reaches the window through a link that leaves its renderer behind",
        "packaging/winget/installer.window.yaml.in",
        "ArchiveBinariesDependOnPath: true",
        "ArchiveBinariesDependOnPath: false",
        "TestEveryWingetPackageKeepsItsProgramBesideItsFiles",
    ),
    (
        "the command line package stops offering the arm64 archive the release builds",
        ".github/scripts/build_packages.py",
        '"tfg_{version}_windows_{arch}.zip", ("amd64", "arm64"), "tfg"),',
        '"tfg_{version}_windows_{arch}.zip", ("amd64",), "tfg"),',
        "TestEachWingetPackageOffersTheArchitecturesTheReleaseBuilds",
    ),
    (
        "the release renames the window archive and the packages still download the old name",
        ".github/workflows/release.yml",
        'base="tfg-gui_${version}_${label}_${arch}"',
        'base="tfg-window_${version}_${label}_${arch}"',
        "TestThePackagesDownloadWhatTheReleaseBuilds",
    ),
    (
        "the window package installs whatever the address answers, unchecked",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        "    -Checksum64 '{{SHA256_AMD64}}' `\n",
        "",
        "TestTheChocolateyPackagesCheckWhatTheyDownload",
    ),
    (
        "the command line package checks its download with a weaker hash",
        "packaging/chocolatey/tools/chocolateyinstall.cli.ps1.in",
        "-ChecksumType64 'sha256'",
        "-ChecksumType64 'md5'",
        "TestTheChocolateyPackagesCheckWhatTheyDownload",
    ),
    (
        "the Chocolatey icon follows a branch instead of the tag",
        ".github/scripts/build_packages.py",
        '"https://cdn.jsdelivr.net/gh/%s@%s/%s" % (repo, tag, ICON)',
        '"https://cdn.jsdelivr.net/gh/%s@%s/%s" % (repo, "main", ICON)',
        "TestTheChocolateyIconIsAPinnedCdnAddress",
    ),
    (
        "typing tfg-gui holds the terminal until the window closes",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        'New-Item -ItemType File -Path "$exe.gui" -Force | Out-Null',
        "Write-Host ''",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    (
        "the Start menu shortcut starts the window in a folder the person cannot write to",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        "    -WorkingDirectory '%USERPROFILE%' `",
        "    -WorkingDirectory $programDir `",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    (
        "the uninstall removes a shortcut that belongs to somebody else",
        "packaging/chocolatey/tools/chocolateyuninstall.window.ps1.in",
        "if ($target.StartsWith($toolsDir + '\\', [StringComparison]::OrdinalIgnoreCase)) {",
        "if ($target -ne $null) {",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    (
        "the command line shim stops waiting, so a script reads no exit code",
        "packaging/chocolatey/tools/chocolateyinstall.cli.ps1.in",
        "Write-Host 'Type tfg help to see the commands.'",
        "New-Item -ItemType File -Path (Join-Path $toolsDir 'tfg/tfg.exe.gui') -Force | Out-Null",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    (
        "a package ends the program before an upgrade",
        "packaging/chocolatey/tools/chocolateybeforemodify.ps1.in",
        "if ($running.Count -gt 0) {",
        "if ($running.Count -gt 0) {\n    $running | Stop-Process -Force",
        "TestNoPackageScriptEndsTheProgram",
    ),
    (
        "a package script picks up a character outside ASCII",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        "Write-Host 'Start it from the Start menu, or type tfg-gui in a terminal.'",
        "Write-Host 'Start it from the Start menu, or type tfg-gui in a terminal, café.'",
        "TestEveryPackageScriptIsASCII",
    ),
    (
        "the description in both feeds picks up a semicolon",
        ".github/scripts/build_packages.py",
        '"anything that takes a file. Pick a format and a size and you get a file of exactly "',
        '"anything that takes a file; pick a format and a size and you get a file of exactly "',
        "TestThePackageTextFollowsThePunctuationRule",
    ),
    (
        "the packages name a licence the program is not under",
        ".github/scripts/build_packages.py",
        'LICENCE = "GPL-3.0-only"',
        'LICENCE = "GPL-3.0-or-later"',
        "TestThePackagesNameTheProductAndLicenceTheProgramDoes",
    ),
    (
        "a release candidate is refused without saying it is one",
        ".github/scripts/build_packages.py",
        '-.+", tag):',
        '-never-matches", tag):',
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "the renderer writes packages into the repository",
        ".github/scripts/build_packages.py",
        "    if os.path.normcase(out) == root or os.path.normcase(out).startswith(root + os.sep):",
        "    if False:",
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "a refusal leaves half a package beside the destination",
        ".github/scripts/build_packages.py",
        "        if os.path.isdir(work):\n            shutil.rmtree(work)",
        "        pass",
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "the renderer writes into a directory that already holds something",
        ".github/scripts/build_packages.py",
        "holds = not os.path.isdir(out) or bool(os.listdir(out))",
        "holds = not os.path.isdir(out)",
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "a missing archive crashes the renderer instead of being named",
        ".github/scripts/build_packages.py",
        '            refuse("%s has no line for %s.%s Download the checksum file of the release "',
        '            print("%s has no line for %s.%s Download the checksum file of the release "',
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "a checksum written in capitals reaches the Chocolatey package as it is",
        ".github/scripts/build_packages.py",
        "        sums[found.group(2).strip()] = found.group(1).lower()",
        "        sums[found.group(2).strip()] = found.group(1)",
        "TestEverySpellingOfAChecksumFileRendersTheSamePackages",
    ),
    (
        "a checksum file saved with a byte order mark is refused",
        ".github/scripts/build_packages.py",
        "text = data.decode(\"utf-8-sig\").replace(\"\\r\\n\", \"\\n\")",
        "text = data.decode(\"utf-8\").replace(\"\\r\\n\", \"\\n\")",
        "TestEverySpellingOfAChecksumFileRendersTheSamePackages",
    ),
    (
        "a quote in a value ends the PowerShell string early",
        ".github/scripts/build_packages.py",
        '    (".ps1", "\'", "a single quote ends the PowerShell string it sits in"),\n',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "a colon in a value turns the rest of a YAML line into a key",
        ".github/scripts/build_packages.py",
        '    (".yaml", ": ", "YAML reads the rest as a key"),\n',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "a value no template uses any more stays in the renderer",
        ".github/scripts/build_packages.py",
        "        if known - used:",
        "        if False:",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "the README counts a hundred formats more than the program ships",
        "README.md",
        "has an opinion about it. You pick one of its ",
        "has an opinion about it. You pick one of its 1",
        "TestTheReadmeCountsTheFormatsItShips",
    ),
    # --- the package renderer refuses instead of crashing (2026-09-25, review of #143) ---
    (
        "a link walks the packages into the repository",
        ".github/scripts/build_packages.py",
        "    out = os.path.realpath(out)",
        "    out = os.path.abspath(out)",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    # --- the package renderer after the review of #145 (2026-09-28) ---
    (
        "a checksum file that is not UTF-8 crashes the renderer",
        ".github/scripts/build_packages.py",
        "    except UnicodeDecodeError:\n        refuse(\"%s is not UTF-8 text, so it is not a release's checksum file. %s\" % (path, hint))\n",
        "    except KeyError:\n        pass\n",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    (
        "a missing checksum file crashes the renderer",
        ".github/scripts/build_packages.py",
        "    except OSError as err:\n        refuse(\"cannot read the checksum file %s: %s. %s\" % (path, err.strerror or err, hint))\n",
        "    except KeyError:\n        pass\n",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    (
        "a file far too big is read as if it were a checksum file",
        ".github/scripts/build_packages.py",
        "    if len(data) > SUMS_LIMIT:",
        "    if False:",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    (
        "a device is read as if it were a checksum file",
        ".github/scripts/build_packages.py",
        "            if not stat.S_ISREG(os.fstat(handle.fileno()).st_mode):",
        "            if False:",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    (
        "a destination under a file crashes the renderer",
        ".github/scripts/build_packages.py",
        "    except OSError as err:\n        remove_empty(made)\n",
        "    except KeyError:\n        remove_empty(made)\n",
        "TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree",
    ),
    (
        "a failed run leaves behind the folders it made",
        ".github/scripts/build_packages.py",
        "        if not finished:\n            remove_empty(made)",
        "        pass",
        "TestTheRendererLeavesNothingBehindWhenItFailsPartWay",
    ),
    (
        "a write that fails after the working folder exists crashes the renderer",
        ".github/scripts/build_packages.py",
        "    except OSError as err:\n        refuse(\"cannot write the packages to %s",
        "    except KeyError:\n        refuse(\"cannot write the packages to %s",
        "TestTheRendererLeavesNothingBehindWhenItFailsPartWay",
    ),
    (
        "a destination nobody can look into crashes the renderer",
        ".github/scripts/build_packages.py",
        "        except OSError as err:\n            refuse(\"cannot look inside --out",
        "        except KeyError:\n            refuse(\"cannot look inside --out",
        "TestTheRendererLeavesNothingBehindWhenItFailsPartWay",
    ),
    (
        "the window stops asking the rule with its own directory, so a double click in the program folder offers to write there",
        "internal/gui/window/open.go",
        "return OfferedDirectory(dir, programDirectory(), home)",
        'return OfferedDirectory(dir, "", home)',
        "TestTheWindowStartedWhereItShouldNotWriteOffersTheHomeDirectory",
    ),
    (
        "the window checks a remembered folder without its own directory, so the old offer under the program folder comes back",
        "internal/gui/window/open.go",
        "LeftByTheOldOffer(last, programDirectory())",
        'LeftByTheOldOffer(last, "")',
        "TestTheWindowDoesNotOfferTheFolderTheOldOfferLeftBehind",
    ),
    (
        "a remembered bare tfg-out is kept, so a window started from Finder offers /tfg-out again",
        "internal/gui/window/offered.go",
        "clean := filepath.Clean(dir)",
        "clean := filepath.Clean(dir); if !filepath.IsAbs(clean) { return false }",
        "TestAFolderTheOldOfferLeftBehindIsNotOfferedAgain",
    ),
    (
        "the installer installs for one account, so the command line is not on PATH for a build agent",
        "packaging/msi/tfg-setup.wxs.in",
        'Scope="perMachine">',
        'Scope="perUser">',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "an upgrade removes the old version before the new files are in place",
        "packaging/msi/tfg-setup.wxs.in",
        'Schedule="afterInstallExecute"',
        'Schedule="afterInstallInitialize"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "a rebuild of the same version installs beside the first build",
        "packaging/msi/tfg-setup.wxs.in",
        'AllowSameVersionUpgrades="yes"',
        'AllowSameVersionUpgrades="no"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the Restart Manager stays on, so an upgrade while tfg runs fails and ends the run",
        "packaging/msi/tfg-setup.wxs.in",
        '<Property Id="MSIRESTARTMANAGERCONTROL" Value="Disable" />',
        '<Property Id="ARPCOMMENTS" Value="Disable" />',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the PATH entry stays behind after an uninstall",
        "packaging/msi/tfg-setup.wxs.in",
        'System="yes" Permanent="no" />',
        'System="yes" Permanent="yes" />',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the installer puts the folder on the installing account's PATH only",
        "packaging/msi/tfg-setup.wxs.in",
        'Action="set" System="yes"',
        'Action="set" System="no"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the shortcut starts the window in the Start menu folder instead of its own",
        "packaging/msi/tfg-setup.wxs.in",
        'WorkingDirectory="INSTALLFOLDER"',
        'WorkingDirectory="ProgramMenuFolder"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the shortcut starts the command line instead of the window",
        "packaging/msi/tfg-setup.wxs.in",
        'Target="[INSTALLFOLDER]{{WINDOW_EXE}}"',
        'Target="[INSTALLFOLDER]tfg.exe"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the installer brings in a WiX extension, whose code goes into the package",
        "packaging/msi/tfg-setup.wxs.in",
        '<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">',
        '<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the installer ends the running program before it upgrades",
        "packaging/msi/tfg-setup.wxs.in",
        '<MediaTemplate EmbedCab="yes" />',
        '<MediaTemplate EmbedCab="yes" /><CustomAction Id="Stop" Directory="INSTALLFOLDER" ExeCommand="taskkill /F /IM tfg.exe" Execute="immediate" Return="ignore" />',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the programs install into the 32-bit Program Files",
        "packaging/msi/tfg-setup.wxs.in",
        '<StandardDirectory Id="ProgramFiles64Folder">',
        '<StandardDirectory Id="ProgramFilesFolder">',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the installer gets a new UpgradeCode, so every machine keeps the old install beside the new",
        ".github/scripts/build_msi.py",
        'UPGRADE_CODE = "7DB637B7-3BEB-4FBD-BE84-60822854AA2F"',
        'UPGRADE_CODE = "7DB637B7-3BEB-4FBD-BE84-60822854AA20"',
        "TestTheInstallerIsTheShapeThatWasMeasured",
    ),
    (
        "the one sentence the installer shows a person breaks the punctuation rule",
        "packaging/msi/tfg-setup.wxs.in",
        "is already installed. Uninstall it first",
        "is already installed; uninstall it first",
        "TestTheInstallerTextFollowsThePunctuationRule",
    ),
    (
        "a release candidate gets an installer that takes the release's own version",
        ".github/scripts/build_msi.py",
        'if "-" in tag:',
        "if False:",
        "TestTheInstallerIsBuiltForAReleaseOnly",
    ),
    (
        "a version the installer cannot hold is built anyway",
        ".github/scripts/build_msi.py",
        "if major > 255 or minor > 255 or patch > 65535:",
        "if False:",
        "TestTheInstallerIsBuiltForAReleaseOnly",
    ),
    (
        "two different copies of one document go into the installer, the second one silently dropped",
        ".github/scripts/build_msi.py",
        "if held.read() != archive.read(entry):",
        "if False:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "a name inside an archive writes outside the folder it is unpacked into",
        ".github/scripts/build_msi.py",
        "if not safe_name(entry.filename):",
        "if False:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "an installer is built without one of the two programs",
        ".github/scripts/build_msi.py",
        "if program.casefold() not in came_from:",
        "if False:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "a run that stops leaves its unpacked archives behind",
        ".github/scripts/build_msi.py",
        "shutil.rmtree(staging, ignore_errors=True)",
        "pass",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "an installer already in the folder is written over",
        ".github/scripts/build_msi.py",
        "if os.path.exists(target):",
        "if False:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "a missing archive ends in an error about something else",
        ".github/scripts/build_msi.py",
        "if not os.path.isfile(path):",
        "if False:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "an archive that is not a zip ends in a traceback",
        ".github/scripts/build_msi.py",
        "except zipfile.BadZipFile as err:",
        "except ZeroDivisionError as err:",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "without WiX the script says so without the command that installs the pinned version",
        ".github/scripts/build_msi.py",
        '"version this project builds with:\\n" + install)',
        '"version this project builds with.")',
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "a double quote in a value ends the attribute it lands in",
        ".github/scripts/build_packages.py",
        '(".wxs", \'"\', "a double quote ends the attribute it sits in"),',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "a WiX variable in a value is read by WiX",
        ".github/scripts/build_packages.py",
        '(".wxs", "$(", "WiX reads it as one of its own variables"),',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "markup in a value is read as markup by the installer source",
        ".github/scripts/build_packages.py",
        '(".wxs", "<", "the installer source reads it as markup"),',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "an ampersand in a value is read as markup by the installer source",
        ".github/scripts/build_packages.py",
        '(".wxs", "&", "the installer source reads it as markup"),',
        "",
        "TestTheRendererRefusesAValueThatWouldBreakItsFile",
    ),
    (
        "the installer's signature carries no timestamp and dies with the certificate",
        ".github/scripts/sign_release.py",
        '"/tr", TIMESTAMP_URL, "/td", "sha256", "/v", path]',
        '"/td", "sha256", "/v", path]',
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "the installer's certificate is taken on trust instead of read back out of the file",
        ".github/scripts/sign_release.py",
        "signer = certificate_of(path)",
        "signer = pin",
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "an installer signed by another certificate goes on the draft",
        ".github/scripts/sign_release.py",
        "if signer != pin:",
        "if False:",
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "nothing asks whether the installer can be built before the card signs the programs",
        ".github/scripts/sign_release.py",
        "        check_installer(args.tag, tree)",
        "        pass",
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "a release's draft counts as complete without its installer",
        ".github/scripts/sign_release.py",
        "if not is_candidate(tag) and len(installers) != 1:",
        "if False:",
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "a candidate's draft carrying an installer counts as complete",
        ".github/scripts/sign_release.py",
        "if is_candidate(tag) and installers:",
        "if False:",
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "the installer is built by the checkout's build_msi.py instead of the tag's",
        ".github/scripts/sign_release.py",
        'script = os.path.join(tree, ".github", "scripts", "build_msi.py")',
        'script = os.path.join(".github", "scripts", "build_msi.py")',
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "the exported tree is only part of what was tagged",
        ".github/scripts/sign_release.py",
        'archive = subprocess.run(["git", "archive", "--format=tar", tag], capture_output=True)',
        'archive = subprocess.run(["git", "archive", "--format=tar", tag, "--", "internal"], capture_output=True)',
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "a tag missing from the clone ends in git's error instead of the command that fetches it",
        ".github/scripts/sign_release.py",
        "if found.returncode != 0:",
        "if False:",
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "the signing script takes only a tag with -rc for a candidate, so a -beta gets an installer",
        ".github/scripts/sign_release.py",
        'return "-" in tag',
        'return "-rc" in tag',
        "TestEveryPlaceAsksTheSameQuestionOfACandidate",
    ),
    (
        "the installer job installs another WiX than the one build_msi.py builds with",
        ".github/workflows/ci.yml",
        "dotnet tool install --global wix --version 5.0.2",
        "dotnet tool install --global wix --version 5.0.1",
        "TestTheInstallerIsInstalledOnARunner",
    ),
    (
        "the installer job no longer asks tfg through PATH",
        ".github/workflows/ci.yml",
        "$said = ((& cmd.exe /c 'tfg version' 2>&1) | Out-String).Trim()",
        "$said = $env:VERSION",
        "TestTheInstallerIsInstalledOnARunner",
    ),
    (
        "the installer job no longer uninstalls, so what an uninstall leaves is asked of nothing",
        ".github/workflows/ci.yml",
        "$code = Msi '/x' $second 'uninstall.log'",
        "$code = 0",
        "TestTheInstallerIsInstalledOnARunner",
    ),
    (
        "the installer job runs on Linux, where there is nothing to install",
        ".github/workflows/ci.yml",
        "    runs-on: windows-latest\n    timeout-minutes: 20\n    steps:\n      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1  # v7.0.1\n        with:\n          persist-credentials: false\n\n      - uses: actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97  # v7.0.0\n        with:\n          python-version: \"3.14\"\n\n      - name: build the installer twice",
        "    runs-on: ubuntu-latest\n    timeout-minutes: 20\n    steps:\n      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1  # v7.0.1\n        with:\n          persist-credentials: false\n\n      - uses: actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97  # v7.0.0\n        with:\n          python-version: \"3.14\"\n\n      - name: build the installer twice",
        "TestTheInstallerIsInstalledOnARunner",
    ),
    (
        "the release check asks for an installer under another name than the one the build gives it",
        ".github/workflows/verify-release.yml",
        'test -f "tfg-setup_${TAG#v}_windows_amd64.msi"',
        'test -f "tfg_setup_${TAG#v}_windows_amd64.msi"',
        "TestTheReleaseCheckAsksForTheInstaller",
    ),
    (
        "the build names the installer differently from what the release check asks for",
        ".github/scripts/build_msi.py",
        'NAME = "tfg-setup_{version}_windows_amd64.msi"',
        'NAME = "tfg-installer_{version}_windows_amd64.msi"',
        "TestTheReleaseCheckAsksForTheInstaller",
    ),
    (
        "the release check accepts an installer whose signature carries no timestamp",
        ".github/workflows/verify-release.yml",
        "if (-not $sig.TimeStamperCertificate) { throw \"$($msi.Name): the signature carries no timestamp\" }",
        "",
        "TestTheReleaseCheckAsksForTheInstaller",
    ),
    (
        "the release check accepts an installer signed by another certificate",
        ".github/workflows/verify-release.yml",
        "if ($actual -ne $pinned) { throw \"$($msi.Name) was signed by a DIFFERENT certificate: $actual\" }",
        "",
        "TestTheReleaseCheckAsksForTheInstaller",
    ),
    (
        "the release check takes only a tag with -rc for a candidate, so a -beta page passes with an installer",
        ".github/workflows/verify-release.yml",
        '*-*) test "${#installers[@]}" = "0"',
        '*-rc*) test "${#installers[@]}" = "0"',
        "TestEveryPlaceAsksTheSameQuestionOfACandidate",
    ),
    (
        "the installer sorts below the command line's archives on the release page",
        ".github/scripts/build_msi.py",
        'NAME = "tfg-setup_{version}_windows_amd64.msi"',
        'NAME = "tfg_setup_{version}_windows_amd64.msi"',
        "TestTheInstallerSortsAmongThePrograms",
    ),
    (
        "the installer sorts above the window's archives on the release page",
        ".github/scripts/build_msi.py",
        'NAME = "tfg-setup_{version}_windows_amd64.msi"',
        'NAME = "tfg-aaa-setup_{version}_windows_amd64.msi"',
        "TestTheInstallerSortsAmongThePrograms",
    ),
    (
        "two archives holding LICENSE and License with different bytes put one over the other on Windows without a word",
        ".github/scripts/build_msi.py",
        "key = entry.filename.casefold()",
        "key = entry.filename",
        "TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll",
    ),
    (
        "a refusal while the tagged tree is out leaves it beside the release's files",
        ".github/scripts/sign_release.py",
        "shutil.rmtree(into, ignore_errors=True)",
        "pass",
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "the installer is signed with a name other than the one it carries",
        ".github/scripts/build_msi.py",
        'print(values(parse_version(args.tag), args.tag)["APP_NAME"])',
        'print(values(parse_version(args.tag), args.tag)["PUBLISHER"])',
        "TestTheInstallerIsSignedWithTheNameItCarries",
    ),
    (
        "the installer is signed with no name, so the elevation prompt shows digits",
        ".github/scripts/sign_release.py",
        'command = [signtool, "sign", "/sha1", thumbprint, "/fd", "sha256", "/d", name,',
        'command = [signtool, "sign", "/sha1", thumbprint, "/fd", "sha256",',
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "the product name keeps the line end the tag's script printed after it",
        ".github/scripts/sign_release.py",
        "name = said.stdout.strip()",
        "name = said.stdout",
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "a tag whose script names no product signs the installer with an empty name",
        ".github/scripts/sign_release.py",
        "if said.returncode != 0 or not name:",
        "if False:",
        "TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout",
    ),
    (
        "the name is first asked for after the card has signed the programs",
        ".github/scripts/sign_release.py",
        'print("  its signature will name it %r" % product_name(tag, tree))',
        'print("  its signature will name it")',
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "the installer is signed with a name typed into the signing script",
        ".github/scripts/sign_release.py",
        "name = product_name(tag, tree)",
        'name = "Testing Files Generator"',
        "TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft",
    ),
    (
        "the README's table of presets leaves out a preset the binary ships",
        "README.md",
        "| `upload-validation` | Does my upload form take what it should and turn the rest away? |",
        "",
        "TestTheReadmeListsEveryPresetItShips",
    ),
    (
        "the README's table of presets words a question differently from the preset",
        "README.md",
        "Is a size limit enforced exactly where it is declared?",
        "Is a size limit enforced?",
        "TestTheReadmeListsEveryPresetItShips",
    ),
    (
        "the README's table of presets names a preset nothing registers",
        "README.md",
        "| `text-encoding` |",
        "| `text-encodings` |",
        "TestTheReadmeListsEveryPresetItShips",
    ),
    (
        "the social card shows a file over the limit as one the system should take",
        "web/templates/social.html",
        '<span class="file">1mb_over_1b.pdf</span><span class="bytes">1 048 577 B</span></div><span class="tag reject">reject</span>',
        '<span class="file">1mb_over_1b.pdf</span><span class="bytes">1 048 577 B</span></div><span class="tag accept">accept</span>',
        "TestTheLimitExampleIsWhatThePresetWrites",
    ),
    (
        "the README shows the file at the limit a byte short of what the preset writes",
        "README.md",
        "| `1mb_at_limit.pdf` | 1048576 |",
        "| `1mb_at_limit.pdf` | 1048575 |",
        "TestTheLimitExampleIsWhatThePresetWrites",
    ),
    (
        "the Polish first page tells the reader to take the file over the limit",
        "web/content/pl/index.html",
        '<td><code>1mb_over_1b.pdf</code></td><td class="num">1048577</td><td><strong>odrzucić</strong>',
        '<td><code>1mb_over_1b.pdf</code></td><td class="num">1048577</td><td><strong>przyjąć</strong>',
        "TestTheLimitExampleIsWhatThePresetWrites",
    ),
    (
        "a wrong row of the README's presets table hides behind a right one below it",
        "README.md",
        "| `size-boundaries` | Is a size limit enforced exactly where it is declared? |",
        "| `size-boundaries` | Is a size limit enforced? |\n| `size-boundaries` | Is a size limit enforced exactly where it is declared? |",
        "TestTheReadmeListsEveryPresetItShips",
    ),
    (
        # Pages per preset, 2026-09-29 (docs/PRESET-PAGES-2026-09-29.md). The
        # English words of a preset are copies of the registry, and these
        # four entries break one copy each.
        "the English page says a preset catches something the registry does not",
        "web/content/en/site.json",
        '"off by one errors at the limit",',
        '"off by one errors near the limit",',
        "TestEveryPresetIsDescribedInEveryLanguage",
    ),
    (
        "the English page asks a preset question the registry no longer asks",
        "web/content/en/site.json",
        '"question": "Is a size limit enforced exactly where it is declared?",',
        '"question": "Is a size limit enforced where it is declared?",',
        "TestEveryPresetIsDescribedInEveryLanguage",
    ),
    (
        "the English page describes a preset setting differently from the registry",
        "web/content/en/site.json",
        '"spread": "How far either side of the limit to reach, as a list of sizes."',
        '"spread": "How far either side of the limit to go, as a list of sizes."',
        "TestEveryPresetIsDescribedInEveryLanguage",
    ),
    (
        "the English page words what a preset setting takes differently from the registry",
        "web/content/en/site.json",
        '"sizes separated by commas": "sizes separated by commas",',
        '"sizes separated by commas": "a list of sizes",',
        "TestEveryPresetIsDescribedInEveryLanguage",
    ),
    (
        # A Polish page one sentence short renders without a word missing,
        # which is why the count is asked rather than left to the render.
        "the Polish page of a preset loses one thing it catches and still renders",
        "web/content/pl/site.json",
        '"błędy o jeden na samym limicie",',
        '',
        "TestEveryPresetIsDescribedInEveryLanguage",
    ),
    (
        "the English formats page words what a format setting takes differently from the registry",
        "web/content/en/site.json",
        '"the password, in plain text": "the password, in plain text"',
        '"the password, in plain text": "the password"',
        "TestEveryLanguageDescribesEverythingTheProgramCanProduce",
    ),
    (
        "a reaction a manifest can declare loses its Polish meaning",
        "web/content/pl/site.json",
        '"sanitize": "System powinien',
        '"sanitized": "System powinien',
        "TestEveryReactionHasItsMeaningInEveryLanguage",
    ),
    (
        "the sitemap leaves out the pages made from the registry",
        "internal/site/render.go",
        '\t\t\tb.WriteString("  <url>\\n")',
        '\t\t\tif page.Parent != "" {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tb.WriteString("  <url>\\n")',
        "TestTheSitemapNeedsNoSchemaButItsOwn",
    ),
    (
        "the header lists the page of every preset as well as the list of them",
        "internal/site/render.go",
        '\t\tif item.Parent != "" {\n\t\t\tcontinue\n\t\t}',
        '\t\tif false && item.Parent != "" {\n\t\t\tcontinue\n\t\t}',
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the page of a preset leads back to the home page instead of the list above it",
        "internal/site/render.go",
        '\t\tv.Up = &NavItem{Label: up.Nav, URL: pageURL(lang, up)}',
        '\t\tv.Up = &NavItem{Label: up.Nav, URL: pageURL(lang, lang.Pages[0])}',
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "a text setting is described as text again rather than by what it takes",
        "internal/site/view.go",
        '\t\tif p.Shape != "" {\n\t\t\treturn v.Term(p.Shape)',
        '\t\tif false && p.Shape != "" {\n\t\t\treturn v.Term(p.Shape)',
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the page of a preset stops marking a default as our placeholder",
        "internal/site/presets.go",
        "Detail: detail, Placeholder: s.Placeholder})",
        "Detail: detail, Placeholder: false})",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        # Every id today passes, so this is aimed at whether the check runs on
        # every id rather than at a bad one - there is none to feed it.
        "the check that a preset id can be an address stops passing ids with a dash",
        "internal/site/presets.go",
        "regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)",
        "regexp.MustCompile(`^[a-z0-9]+$`)",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the page of a preset counts its targets as its files",
        "internal/guard/sitepresets_test.go",
        "Files: b.Files,",
        "Files: b.Targets,",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        "the reactions on the page of a preset are counted twice over",
        "internal/guard/sitepresets_test.go",
        "\t\tcounts[f.Expected.Outcome]++",
        "\t\tcounts[f.Expected.Outcome] += 2",
        "TestTheSiteSaysWhatTheToolSays",
    ),
    (
        # O252, 2026-09-29 (docs/O252-ANALIZA-2026-09-29.md). The primitive
        # going back to what it replaced.
        "the publish goes back to a rename, which replaces a name somebody holds",
        "internal/core/publish.go",
        "\treturn PublishThrough(tmp, final, renameNoReplace, os.Link)",
        "\treturn os.Rename(tmp, final)",
        "TestPublishNeverReplacesANameSomebodyHolds",
    ),
    (
        "a generated file is given its name by a rename again, over a file put there during the run",
        "internal/engine/parallel.go",
        "\tif err := core.Publish(tmp, final); err != nil {",
        "\tif err := os.Rename(tmp, final); err != nil {",
        "TestAFileTakenDuringTheRunIsNotWrittenOver",
    ),
    (
        "a failure that is not unsupported walks on to the next way of naming the file",
        "internal/core/publish.go",
        "\tif !errors.Is(err, errors.ErrUnsupported) {\n\t\treturn &os.LinkError{Op: \"publish\", Old: tmp, New: final, Err: err}",
        "\tif false && !errors.Is(err, errors.ErrUnsupported) {\n\t\treturn &os.LinkError{Op: \"publish\", Old: tmp, New: final, Err: err}",
        "TestPublishFallbacksRefuseATakenNameToo",
    ),
    (
        "a clean-up removes by name again, whatever holds the name by then",
        "internal/core/writenew.go",
        "\tif !sameWrite(own, now) {\n\t\treturn &NotOursError{Path: path}",
        "\tif false && !sameWrite(own, now) {\n\t\treturn &NotOursError{Path: path}",
        "TestGivingAClaimedNameBackSparesWhatTookItsName",
    ),
    (
        # A real run meets a left reservation twice, a dry run once - so this is
        # aimed at the one road where the preflight is the whole answer.
        "a preview stops asking whether a run holds the manifest's name",
        "internal/engine/preflight.go",
        "\tif path := manifest.ReservationPath(ManifestPath(opt)); exists(path) {",
        "\tif path := manifest.ReservationPath(ManifestPath(opt)); exists(path) && false {",
        "TestALeftReservationStopsEvenADryRun",
    ),
    (
        "the instructions and an ejected recipe are named before their bytes reach the disk",
        "internal/core/writenew.go",
        "\tif err := f.Sync(); err != nil {\n\t\treturn Finish(f, err)",
        "\tif err := error(nil); err != nil {\n\t\treturn Finish(f, err)",
        "TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst",
    ),
    (
        # NOT CAUGHT on 2026-09-29, and the reason is the machine, not the
        # guard: the guard's path to MoveFile is over 260 characters, but the
        # Go runtime marks every process long path aware on Windows 10 1703 and
        # later, and LongPathsEnabled is 1 here, so MoveFile takes the path
        # without the prefix. Only a machine with LongPathsEnabled 0 can catch
        # it (docs/O252-ANALIZA-2026-09-29.md, O256).
        "a long path reaches MoveFile without the prefix that lets it past 260 characters",
        "internal/core/publish_windows.go",
        "\tif len(path) < 248 ||",
        "\tif true ||",
        "TestANameAsLongAsEverySystemStoresIsWritten",
    ),
    (
        # Repointed on 2026-09-29, when O252 moved the write path (the lock now
        # keeps what it is, the reservation is the temporary name, eject goes
        # through core.WriteNew). Same labels, same guards, new code.
        "the run stops taking the directory, so nothing holds it while the files are written",
        "internal/engine/engine.go",
        "\tlock, err := claimRunLock(lockPath)",
        "\tlock, err := os.FileInfo(nil), error(nil)",
        "TestTheDirectoryIsHeldWhileTheRunWritesAndGivenBackWhenItEnds",
    ),
    (
        "the run keeps the directory after it ends, so nothing can generate there again",
        "internal/engine/engine.go",
        "defer releaseRunLock(lockPath, lock)",
        "defer func() { _, _ = lockPath, lock }()",
        "TestAStoppedRunGivesTheDirectoryBack",
    ),
    (
        "eject -o writes over a file that is already there",
        "internal/cli/presetcmd.go",
        "\tif _, err := core.WriteNew(path, source, 0o644); err != nil {",
        "\tif err := core.ReplaceFile(path, source); err != nil {",
        "TestEjectLeavesAFileAlreadyThereAsItIs",
    ),
    (
        "eject -o writes something other than what eject prints",
        "internal/cli/presetcmd.go",
        "\tif _, err := core.WriteNew(path, source, 0o644); err != nil {",
        "\tif _, err := core.WriteNew(path, source[:len(source)/2], 0o644); err != nil {",
        "TestEjectWritesTheFileByteForByteWhatItWouldPrint",
    ),
    (
        'the manifest goes back to being renamed into place without reaching the disk',
        'internal/manifest/reservation.go',
        'if err := f.Sync(); err != nil {',
        'if err := error(nil); err != nil {',
        'TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst',
    ),
    (
        'the manifest is written through its reservation without asking whether it is still ours',
        'internal/manifest/reservation.go',
        '\tf, err := core.OpenOwn(r.tmp, r.own)',
        '\tf, err := os.OpenFile(r.tmp, os.O_WRONLY, 0)',
        'TestAReservationSwappedForALinkIsNotWrittenThrough',
    ),
    (
        'the manifest is named before its bytes reach the disk',
        'internal/manifest/reservation.go',
        '\tif err := f.Sync(); err != nil {\n\t\treturn core.Finish(f, err)',
        '\tif err := error(nil); err != nil {\n\t\treturn core.Finish(f, err)',
        'TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst',
    ),
    (
        'the marker gets a second spelling again, the way it had one until 2026-09-06',
        'internal/manifest/reservation.go',
        '\treturn core.SiblingPath(path, core.WritingMarker)',
        '\treturn core.SiblingPath(path, ".tfg-writing")',
        'TestEachLeftoverMarkerIsSpelledInOnePlace',
    ),
    (
        'the manifest writes its temporary file with a plain create again',
        'internal/manifest/reservation.go',
        '\tf, err := core.CreateNew(tmp, 0o666)',
        '\tf, err := os.Create(tmp)',
        'TestAHeldTemporaryNameStopsTheWriteRatherThanGoingThroughIt',
    ),
    (
        'the manifest name stops being exclusive, so two runs both take it',
        'internal/manifest/reservation.go',
        '\tf, err := core.CreateNew(tmp, 0o666)',
        '\tf, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE, 0o666)',
        'TestAClaimedNameCannotBeClaimedTwice',
    ),
    (
        'the claimed name is never filled in with the manifest',
        'internal/manifest/reservation.go',
        '\treturn own, core.Publish(tmp, final)',
        '\treturn own, nil',
        'TestAManifestIsStillWrittenWhenNothingIsThere',
    ),
    (
        'saving a manifest goes back to reading every look failure as an empty slot',
        'internal/manifest/reservation.go',
        '\tcase !errors.Is(err, fs.ErrNotExist):',
        '\tcase err != nil && !errors.Is(err, err):',
        'TestSavingAManifestTellsAnEmptySlotFromAnUnreadableOne',
    ),
    (
        "every failure to take the run's lock is called another run writing there",
        'internal/engine/engine.go',
        "\t\tif errors.Is(err, fs.ErrExist) {\n\t\t\treturn res, &RunInProgressError{Path: lockPath, Dir: opt.OutDir}",
        "\t\tif !errors.Is(err, fs.ErrExist) {\n\t\t\treturn res, &RunInProgressError{Path: lockPath, Dir: opt.OutDir}",
        'TestADirectoryWeCannotWriteInIsAPermissionProblem',
    ),
    (
        'a long manifest name is written under the plain join again',
        'internal/manifest/reservation.go',
        '\treturn core.SiblingPath(path, core.WritingMarker)',
        '\treturn path + core.WritingMarker',
        'TestAManifestNamedAsLongAsEverySystemStoresIsSaved',
    ),
    (
        'the manifest stops going through the function that flushes before the rename',
        'internal/manifest/reservation.go',
        '\town, err := saveReserved(f, m, r.tmp, r.final)',
        '\town, err := core.Finish(f, m.Encode(f))',
        'TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst',
    ),
    (
        # Review of #150, 2026-09-29: a leftover temporary name of eject -o
        # was reported as the recipe itself being there.
        "eject -o calls its own leftover temporary name the recipe",
        "internal/cli/presetcmd.go",
        "\t\tif errors.As(err, &held) {\n\t\t\tfmt.Fprintf(errOut, \"tfg: %s is already there. It is the temporary name",
        "\t\tif false && errors.As(err, &held) {\n\t\t\tfmt.Fprintf(errOut, \"tfg: %s is already there. It is the temporary name",
        "TestEjectNamesItsOwnLeftoverRatherThanTheRecipe",
    ),
    (
        'the file ceiling is left one line above the biggest file, which grants headroom nobody decided to grant and lets the next arrival in unnoticed',
        'internal/guard/codeshape_test.go',
        "\tlongestFile = ",
        "\tlongestFile = 1 + ",
        'TestTheCeilingsAreTodaysMeasurementAndNotALooserNumber',
    ),
    (
        'the last resort renames over a name that is taken',
        'internal/core/publish.go',
        '\tcase lookErr == nil:\n\t\treturn &os.LinkError{Op: "publish", Old: tmp, New: final, Err: fs.ErrExist}',
        '\tcase false:\n\t\treturn &os.LinkError{Op: "publish", Old: tmp, New: final, Err: fs.ErrExist}',
        'TestPublishFallbacksRefuseATakenNameToo',
    ),
    (
        'the lock stops being empty, so a reader could find something half written in it',
        'internal/engine/engine.go',
        '\town, err := core.Finish(fh, nil)',
        '\t_, _ = fh.WriteString("x")\n\town, err := core.Finish(fh, nil)',
        'TestTheRunLockCarriesNothing',
    ),
    (
        'a manifest that could not be written leaves its empty claim behind, so the files it should have named cannot be cleaned up',
        'internal/manifest/reservation.go',
        '\t\t_ = core.RemoveOwn(r.tmp, r.own)\n\t\treturn err\n\t}\n\tm.saved = size\n\treturn nil',
        '\t\treturn err\n\t}\n\tm.saved = size\n\treturn nil',
        'TestAManifestThatCouldNotBeWrittenLeavesNoManifest',
    ),
    (
        # The only half of plantLink a machine off CI can prove. Whether CI
        # really fails rather than skips is answered by the Windows job itself.
        "a case built on a symbolic link skips on CI too, where no one sees it did not run",
        "internal/guard/symlinkescape_test.go",
        "\treturn ci == \"\"",
        "\treturn true",
        "TestACaseBuiltOnALinkSkipsOnlyOffCI",
    ),
    (
        # Outside review of #151: a guard on the predicate alone stayed green
        # when plantLink stopped asking it.
        "a refused link skips on CI too, because plantLink stops asking whether it may",
        "internal/guard/symlinkescape_test.go",
        "\tif !linkCasesMaySkip(ci) {",
        "\tif false {",
        "TestACaseBuiltOnALinkSkipsOnlyOffCI",
    ),
    (
        "every refused link counts as a missing privilege, so a real refusal skips off CI",
        "internal/guard/symlinkescape_test.go",
        "\treturn errors.Is(err, errPrivilegeNotHeld) || errors.Is(err, fs.ErrPermission)",
        "\treturn true || errors.Is(err, errPrivilegeNotHeld) || errors.Is(err, fs.ErrPermission)",
        "TestACaseBuiltOnALinkSkipsOnlyOffCI",
    ),
    (
        "a PDF drops the author it was asked for",
        "internal/format/pdf/info.go",
        "\tentry(&b, \"Author\", i.author)\n",
        "\tentry(&b, \"Author\", \"\")\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a PDF writes text outside ASCII between brackets, where a reader takes it for PDFDocEncoding",
        "internal/format/pdf/info.go",
        "\t\tif s[i] < 0x20 || s[i] > 0x7e {\n",
        "\t\tif false && (s[i] < 0x20 || s[i] > 0x7e) {\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a PDF date loses the zone it was asked in",
        "internal/format/pdf/info.go",
        "\t\tpdf += zone[:3] + \"'\" + zone[4:] + \"'\"\n",
        "\t\tpdf += \"\"\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a PDF date that no calendar has is written instead of refused",
        "internal/format/pdf/info.go",
        "\tif t.Year() != y || int(t.Month()) != mo || t.Day() != d {\n",
        "\tif false && (t.Year() != y || int(t.Month()) != mo || t.Day() != d) {\n",
        "TestABadPropertyValueIsTheCallersMistakeNotOurs",
    ),
    (
        "a landscape PDF page keeps standing upright",
        "internal/format/pdf/settings.go",
        "\tif o.orientation == orientLandscape || (o.orientation == mixed && i%2 == 1) {\n",
        "\tif false && (o.orientation == orientLandscape || (o.orientation == mixed && i%2 == 1)) {\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a mixed PDF gives every page the first size of the cycle",
        "internal/format/pdf/settings.go",
        "\ts := o.sizes[i%len(o.sizes)]\n",
        "\ts := o.sizes[0]\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a mixed PDF of one page is written as a uniform page instead of refused",
        "internal/format/pdf/settings.go",
        "\t\tif raw == mixed && pages < 2 {\n",
        "\t\tif false && raw == mixed && pages < 2 {\n",
        "TestABadPropertyValueIsTheCallersMistakeNotOurs",
    ),
    (
        "a PDF page that asks to be turned never says so",
        "internal/format/pdf/document.go",
        "\tif m.opts.rotate != 0 {\n",
        "\tif false && m.opts.rotate != 0 {\n",
        "TestEveryPdfSettingSurvivesItsReferenceTool",
    ),
    (
        "a PDF page that nobody turned says /Rotate 0, bytes the default file never had",
        "internal/format/pdf/document.go",
        "\tif m.opts.rotate != 0 {\n",
        "\tif true || m.opts.rotate != 0 {\n",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a landscape PDF grows by a byte or two depending on the seed",
        "internal/format/pdf/document.go",
        "\tbody.Write([]byte{'%', 0xe2, 0xe3, 0xcf, 0xd3, '\\n'})\n",
        "\tbody.Write([]byte{'%', 0xe2, 0xe3, 0xcf, 0xd3, '\\n'})\n\tif m.opts.orientation == orientLandscape {\n\t\tbody.WriteString(\"%\" + strings.Repeat(\"x\", int(m.seed%3)) + \"\\n\")\n\t}\n",
        "TestTheMinimumStandsWhateverTheSeedIsUnderEveryDeclaredChoice",
    ),
    (
        "a block of settings declared in two places registers",
        "internal/format/registry.go",
        "\tif g, split := splitGroup(d.Properties); split {\n",
        "\tif g, split := splitGroup(d.Properties); split && false {\n",
        "TestABlockOfSettingsDeclaredInTwoPlacesIsRefused",
    ),
    (
        "tfg formats stops naming a block of settings",
        "internal/cli/formats.go",
        "\t\tif p.Group != \"\" && (i == 0 || d.Properties[i-1].Group != p.Group) {\n",
        "\t\tif false && p.Group != \"\" && (i == 0 || d.Properties[i-1].Group != p.Group) {\n",
        "TestASettingsBlockIsNamedWhereItStartsOnBothOutputs",
    ),
    (
        "the machine readable list stops saying which block a setting is in",
        "internal/cli/formats.go",
        "\t\t\tGroup: p.Group,\n",
        "\t\t\tGroup: \"\",\n",
        "TestASettingsBlockIsNamedWhereItStartsOnBothOutputs",
    ),
    (
        "the window stops drawing the name of a block of settings",
        "internal/gui/parts/property.go",
        "\t\t\tobjects = append(objects, Subheading(text.SettingGroup(owner, p.Group)))\n",
        "\t\t\t_ = Subheading\n",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        "an empty text box says its value is worked out from the size again",
        "internal/gui/parts/property.go",
        "\tcase p.Kind == format.PropertyText || ofATool:\n\t\treturn text.PlaceholderNotSet()\n",
        "\tcase false && p.Kind == format.PropertyText || ofATool:\n\t\treturn text.PlaceholderNotSet()\n",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        "the packages give a copyright the program does not print",
        ".github/scripts/build_packages.py",
        'COPYRIGHT = "Copyright (C) 2026 DonislawDev"',
        'COPYRIGHT = "Copyright (C) 2025 DonislawDev"',
        "TestEveryPackageCarriesTheCopyrightTheProgramPrints",
    ),
    (
        "the Chocolatey package loses its copyright element",
        "packaging/chocolatey/package.nuspec.in",
        "<copyright>{{COPYRIGHT}}</copyright>",
        "<!-- no copyright -->",
        "TestEveryPackageCarriesTheCopyrightTheProgramPrints",
    ),
    (
        "the WinGet package loses its copyright field",
        "packaging/winget/locale.en-US.yaml.in",
        "Copyright: {{COPYRIGHT}}",
        "# no copyright",
        "TestEveryPackageCarriesTheCopyrightTheProgramPrints",
    ),
    (
        "the window knows its screens by their translated word again",
        "internal/gui/window/open.go",
        "			showing = item.ID",
        "			showing = item.Text",
        "TestTwoTabsCalledTheSameAreStillTwoScreens",
    ),
    (
        "opening Preferences saves what its list only shows",
        "internal/gui/window/preferences.go",
        "p.pick.OnChanged = nil",
        "p.pick.OnChanged = p.chose",
        "TestOpeningPreferencesChangesNothingSaved",
    ),
    (
        "choosing a language does not save it",
        "internal/gui/window/preferences.go",
        "p.host.Remembered().RememberLanguage(p.choices.tagOf[option])",
        "_ = p.choices.tagOf[option]",
        "TestChoosingALanguageSavesItAndOffersARestart",
    ),
    (
        "Restart now stays on while the screen is busy",
        "internal/gui/window/preferences.go",
        "if p.pending() && !p.busy() {",
        "if p.pending() {",
        "TestRestartWaitsForTheRunAndClosesTheWayAPersonDoes",
    ),
    (
        "Restart now asks for no new window",
        "internal/gui/window/preferences.go",
        "p.host.RestartWhenClosed()",
        "_ = p.host",
        "TestRestartWaitsForTheRunAndClosesTheWayAPersonDoes",
    ),
    (
        "the busy state tells nobody that it changed",
        "internal/gui/window/runbusy.go",
        "if was != occupied && b.changed != nil {",
        "if was != occupied && false && b.changed != nil {",
        "TestRestartWaitsForTheRunAndClosesTheWayAPersonDoes",
    ),
    (
        "Restart now closes the window without the close button's way",
        "internal/gui/window/open.go",
        "prefs := NewPreferences(h, func() { leave() },",
        "prefs := NewPreferences(h, func() { _ = leave; h.Close() },",
        "TestRestartWaitsForTheRunAndClosesTheWayAPersonDoes",
    ),
    (
        "Forget stops forgetting for the rest of the run",
        "internal/gui/window/remembered.go",
        "f.forgot = true",
        "f.forgot = false",
        "TestForgetClearsWhatTheWindowKeepsAndTheCloseWritesNothingBack",
    ),
    (
        "a language is matched loosely enough to hand one script for another",
        "internal/gui/text/languages.go",
        "if confidence < language.High {",
        "if confidence < language.Low {",
        "TestTheWindowSpeaksTheLanguageItWasToldTo",
    ),
    (
        "a language somebody chose loses to the system",
        "internal/gui/text/languages.go",
        "if tag, ok := closest(saved, carried); ok {",
        "if tag, ok := closest(system, carried); ok {",
        "TestTheWindowSpeaksTheLanguageItWasToldTo",
    ),
    (
        "the pseudo language disguises the values it is handed",
        "internal/gui/text/languages.go",
        'if !strings.HasPrefix(s, "{{") {',
        "if true {",
        "TestThePseudoLanguageDisguisesTheWordsAndKeepsTheValues",
    ),
    (
        "asking for the pseudo language gives English",
        "internal/gui/text/catalogue.go",
        "if lang == Pseudo {",
        'if lang == "never" {',
        "TestThePseudoLanguageDisguisesTheWordsAndKeepsTheValues",
    ),
    (
        "the Polish catalogue loses an entry",
        "internal/gui/text/locale/pl.json",
        "  \"ButtonForget\": {",
        "  \"ButtonForgetX\": {",
        "TestEveryLanguageSaysEverythingTheWindowSays",
    ),
    (
        "the Polish catalogue says a letter the typeface lacks",
        "internal/gui/text/locale/pl.json",
        "\"other\": \"Polski\" }",
        "\"other\": \"Polski \u8a9e\" }",
        "TestEveryLetterEveryLanguageSaysIsInTheTypeface",
    ),
    (
        "a Donate button stands on Preferences while the tab guard still names it as the screen without one",
        "internal/gui/window/preferences.go",
        "\t\t\tp.forget)),",
        "\t\t\tp.forget, donate(h, parts.Secondary))),",
        "TestTabbingReachesTheControlsAndSaysInWhatOrder",
    ),
    (
        "a box to type in stands on Preferences while the menu guards still name it as holding none",
        "internal/gui/window/preferences.go",
        "\t\tparts.Prose(h.SettingsFolder()),",
        "\t\tparts.Prose(h.SettingsFolder()), parts.NewEntry(),",
        "TestAMenuIsNotDrawnAsWideAsABoxToTypeIn",
    ),
    (
        "a sentence of the registries is answered with its English in every language",
        "internal/gui/text/registry.go",
        "\treturn localise(key, english)",
        "\treturn english",
        "TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised",
    ),
    (
        "a setting's sentence is drawn as the registry wrote it",
        "internal/gui/parts/property.go",
        'if said := text.SettingDetail(owner, p.Name, p.Detail); said != "" {',
        'if said := p.Detail; said != "" {',
        "TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised",
    ),
    (
        "a preset's question is drawn as the registry wrote it",
        "internal/gui/window/preset.go",
        "parts.Prose(text.PresetQuestion(chosen.ID, chosen.Question))",
        "parts.Prose(chosen.Question)",
        "TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised",
    ),
    (
        "what a preset finds is drawn as the registry wrote it",
        "internal/gui/window/preset.go",
        "parts.Bullets(text.PresetCatches(chosen.ID, chosen.Catches))",
        "parts.Bullets(chosen.Catches)",
        "TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised",
    ),
    (
        "the English copy of the registries answers for them in an English window",
        "internal/gui/text/catalogue.go",
        "err := loadFiles(bundle, fsys, path.Join(dir, folder), English+\".json\")",
        "err := loadFiles(bundle, fsys, path.Join(dir, folder), \"\")",
        "TestTheEnglishCopyOfTheRegistriesNeverAnswersForThem",
    ),
    (
        "the words of the registries are never read into the language chosen",
        "internal/gui/text/catalogue.go",
        "err := loadFiles(bundle, fsys, path.Join(dir, folder), English+\".json\")",
        "_ = folder\n\t\tvar err error",
        "TestThePolishWindowSaysTheRegistriesInPolish",
    ),
    (
        "a size in the Polish window keeps the English decimal point",
        "internal/gui/text/registry.go",
        "return core.HumanBytesIn(n, DecimalMark())",
        "return core.HumanBytes(n)",
        "TestThePolishWindowSaysTheRegistriesInPolish",
    ),
    (
        "a preset's note stops saying which value it is about, so no language can find it",
        "internal/preset/expansion.go",
        "out = append(out, Note{About: name, Said: core.Says(\"preset.Defaulted\",",
        "out = append(out, Note{Said: core.Says(\"preset.Defaulted\",",
        "TestThePolishWindowSaysTheRegistriesInPolish",
    ),
    (
        "the window stops saying a setting's default where tfg formats says it",
        "internal/gui/parts/property.go",
        "return text.AllowedWithDefault(what, p.Default)",
        "return what",
        "TestTheWindowDescribesASettingAsTheCommandLineDoes",
    ),
    (
        "what a preset finds leaves the English copy of the registries",
        "internal/gui/text/registrywords.go",
        "\tfor i, line := range p.Catches {",
        "\tfor i, line := range p.Catches[:0] {",
        "TestTheRegistryCatalogueSaysWhatTheRegistriesSay",
    ),
    (
        "a Polish sentence of the registries translates an English one the registry no longer says",
        "internal/gui/text/locale/registry/pl.json",
        '"Detail.format/pdf.pages": { "hash": "sha256-298488d25c0b",',
        '"Detail.format/pdf.pages": { "hash": "sha256-000000000000",',
        "TestEveryLanguageSaysEverythingTheRegistriesSay",
    ),
    (
        "one English sentence of the registries is two Polish ones",
        "internal/gui/text/locale/registry/pl.json",
        '"Detail.format/gif.height": { "hash": "sha256-4b336c279584", "other": "Jak wysoki jest obraz.',
        '"Detail.format/gif.height": { "hash": "sha256-4b336c279584", "other": "Wysokość obrazu.',
        "TestOneEnglishSentenceIsOneSentenceInEveryLanguage",
    ),
    (
        "a preset's question on the Polish site stops being the one the Polish window asks",
        "web/content/pl/site.json",
        '"question": "Czy mój formularz przesyłania plików przyjmuje to, co powinien, i odrzuca resztę?",',
        '"question": "Czy mój formularz przesyłania plików działa?",',
        "TestThePresetsSayInTheWindowWhatTheySayOnTheSite",
    ),
    (
        # A CRC-32 of another polynomial is eight hex digits too, and it is the
        # one a person comparing with a ZIP or a PNG would not reproduce.
        "crc32 is worked out with the Castagnoli polynomial instead of the IEEE one",
        "internal/tool/checksum/checksum.go",
        '\t{name: "crc32", digits: 8, make: func() hash.Hash { return crc32.NewIEEE() }},',
        '\t{name: "crc32", digits: 8, make: func() hash.Hash { return crc32.New(crc32.MakeTable(crc32.Castagnoli)) }},',
        "TestTheChecksumsAreTheKnownAnswers",
    ),
    (
        # sha512/256 is a SHA-512 as well, by name, and sha512sum disagrees.
        "sha512 is worked out as the truncated sha512/256",
        "internal/tool/checksum/checksum.go",
        '\t{name: "sha512", digits: 128, make: sha512.New},',
        '\t{name: "sha512", digits: 128, make: sha512.New512_256},',
        "TestTheChecksumsAgreeWithTheSystemsOwnTools",
    ),
    (
        "the command line words a match one way and the window another",
        "internal/tool/tool.go",
        'return fmt.Sprintf("Matches: the %s is %s, as expected.", v.About, v.Got)',
        'return fmt.Sprintf("Matches: the %s is %s.", v.About, v.Got)',
        "TestTheWindowSaysAVerdictAsTheCommandLineDoes",
    ),
    (
        "the Tools tab draws what a tool works on and none of its settings",
        "internal/gui/window/tools.go",
        "\tfor _, o := range objects {\n\t\tt.work.Add(o)\n\t}",
        "\tfor _, o := range objects {\n\t\t_ = o\n\t}",
        "TestTheToolsScreenOffersEveryToolWithEveryBox",
    ),
    (
        "a tool run from the window finds the answer and never shows it",
        "internal/gui/window/toolsresult.go",
        "\t\trows.Add(t.resultRow(row))",
        "\t\t_ = row",
        "TestTheWindowRunsAToolAndSaysWhatTheCommandLineSays",
    ),
    (
        "a checksum that does not match ends tfg tool with success",
        "internal/cli/toolcmd.go",
        "\tif failed {\n\t\treturn ExitVerify\n\t}",
        "\tif failed {\n\t\treturn ExitOK\n\t}",
        "TestToolsEndWithTheCodeOfWhatHappened",
    ),
    (
        # The flag package leaves a lone "-" where it is, so handing it back
        # to the flags again is a loop that never ends - measured before the
        # fix: killed by timeout, code 124.
        "a lone dash among the paths of tfg tool is handed back to the flags for ever",
        "internal/cli/toolcmd.go",
        '\t\tfor len(args) > 0 && (args[0] == "-" || !strings.HasPrefix(args[0], "-")) {',
        '\t\tfor len(args) > 0 && !strings.HasPrefix(args[0], "-") {',
        "TestALoneDashIsAFileNameAndNotAHang",
    ),
    (
        "Copy on the Tools tab puts the name of the algorithm on the clipboard instead of the checksum",
        "internal/gui/window/toolsresult.go",
        "\t\tt.host.Copy(value)",
        "\t\tt.host.Copy(row[0])",
        "TestTheToolsScreenTakesAChosenFileAndCopiesTheChecksum",
    ),
    (
        # A Fields of its own, which the window hands its shortcuts to and no
        # box of the screen is in - the Tools tab alone loses them.
        "the boxes of the Tools tab are not the ones the window hands its shortcuts to",
        "internal/gui/window/tools.go",
        "func (t *Tools) Fields() *parts.Fields { return t.fields }",
        "func (t *Tools) Fields() *parts.Fields { return parts.NewFields() }",
        "TestTheKeyboardRunsAToolFromInsideItsBox",
    ),
    (
        "the keyboard acts on the last screen with a directory rather than on the tab on show",
        "internal/gui/window/open.go",
        "\twireKeyboard(h, keyed, &looking)",
        "\t_ = looking\n\twireKeyboard(h, keyed, &showing)",
        "TestTheKeyboardRunsAToolFromInsideItsBox",
    ),
    (
        # The one os.OpenFile outside the one claim is let through because
        # its flags only read. A flag that writes has to stop being let
        # through - read from the source, so a file built only off Windows
        # is held on the Windows machine too.
        "the checksum tool opens its file for writing as well, and the one claim does not notice",
        "internal/core/openregular_unix.go",
        "os.O_RDONLY|syscall.O_NONBLOCK, 0)",
        "os.O_RDWR|syscall.O_NONBLOCK, 0)",
        "TestEveryFileThisToolWritesIsCreatedThroughOneClaim",
    ),
    (
        "a value declared long gets a box as narrow as a name again",
        "internal/gui/parts/property.go",
        "\t\t\tfield = Wide(field)\n",
        "\t\t\t_ = field\n",
        "TestALongValueTakesTheRowAndOnTheToolsTabNothingElseButAPathDoes",
    ),
    (
        "the checksum file separates checksum and path with one space, which sha256sum -c still reads",
        "internal/tool/checksum/sums.go",
        'return sum + "  " + name + "\\n"',
        'return sum + " " + name + "\\n"',
        "TestTheChecksumFileIsTheOneSha256sumWrites",
    ),
    (
        "a tagged line is matched to the algorithm under its capitals, so SHA256 is an algorithm nobody knows",
        "internal/tool/checksum/sums.go",
        "algorithm = strings.ToLower(tag)",
        "algorithm = tag",
        "TestTheCheckReadsTheLinesOtherToolsWrite",
    ),
    (
        "the escaping of a name in a checksum file is kept rather than undone",
        "internal/tool/checksum/sums.go",
        "if path, ok = unescaped(path); !ok {",
        "if ok = true; !ok {",
        "TestTheCheckUndoesTheEscapingOfANameAndKeepsItInTheFolder",
    ),
    (
        "the mode character after the checksum is dropped only when it is a star, so a name after two spaces gains one",
        "internal/tool/checksum/sums.go",
        'if strings.HasPrefix(path, " ") || strings.HasPrefix(path, "*") {',
        'if strings.HasPrefix(path, "*") {',
        "FuzzChecksumFile",
    ),
    (
        "a setting of a tool refused by its declaration ends as a disk that would not cooperate",
        "internal/tool/refusals.go",
        "func (e *SettingError) Class() Class { return Asked }",
        "func (e *SettingError) Class() Class { return Reading }",
        "TestTheFolderToolsEndWithTheCodeOfWhatHappened",
    ),
    (
        "a file a stopped run left half written is listed in the checksum file as one of the folder's",
        "internal/tool/checksum/write.go",
        "case unfinished(path.Base(e.Path)):",
        "case false && unfinished(path.Base(e.Path)):",
        "TestAChecksumFileIsNeverWrittenOver",
    ),
    (
        "a write stopped in the middle writes down the checksums it had",
        "internal/tool/checksum/write.go",
        "\tif stopped != nil {\n\t\treturn nil, stopped\n\t}\n\tsums := make([]string, len(answers))",
        "\t_ = stopped\n\tsums := make([]string, len(answers))",
        "TestAStoppedWriteLeavesNothing",
    ),
    (
        "the Tools tab offers a file picker for a folder",
        "internal/gui/window/tools.go",
        "\tif kind == tool.Folder {\n",
        "\tif false {\n",
        "TestTheToolsTabWritesAndChecksAFolder",
    ),
    (
        "the Tools tab draws every item of a note, however many there are",
        "internal/gui/window/toolsresult.go",
        "noteLines(d, n, parts.NoteItemsShown)",
        "noteLines(d, n, parts.NoteItemsShown*1000)",
        "TestTheToolsTabCutsALongListShort",
    ),
    (
        "a pipe or a junction in a folder is read as a file",
        "internal/tool/checksum/write.go",
        "\t\tcase e.Kind != audit.Regular:\n",
        "\t\tcase false:\n",
        "TestAFolderToolNeverWaitsOnAPipeOrAJunction",
    ),
    (
        "the command line and the window word the verdict about a checksum file two ways",
        "internal/tool/tool.go",
        'return fmt.Sprintf("Matches: every file %s lists is what it says.", v.About)',
        'return fmt.Sprintf("Matches: every file %s lists.", v.About)',
        "TestTheWindowSaysAVerdictAsTheCommandLineDoes",
    ),
    (
        # Review of #158: the walk, the reading and the writing each followed
        # the folder's name, so a link pointed elsewhere half way put the
        # checksum file of one folder into another.
        "checksum-write follows the folder's name again at every step",
        "internal/tool/checksum/write.go",
        "\tdir := followed(named)\n",
        "\tdir := named\n",
        "TestAFolderSwappedHalfWayGetsNoChecksumFileOfAnother",
    ),
    (
        # Review of #158: a number for every line that is not a checksum made a
        # checksum file of 64 MiB of line breaks into two gigabytes.
        "lines that are not checksums are kept one number a line again",
        "internal/tool/checksum/sums.go",
        "last >= 0 && s.NotSums[last].To == number-1 {",
        "last >= 0 && s.NotSums[last].To == number-1 && false {",
        "TestTheCheckReadsTheLinesOtherToolsWrite",
    ),
    (
        "the Tools tab stops laying itself out after a change of tool",
        "internal/gui/window/toolsresult.go",
        "\tt.view.scroll.Refresh()\n",
        "",
        "TestChoosingAToolLaysTheScreenOutAsIfItOpenedWithIt",
    ),
    (
        "the menu of tools loses the name each tool has on the command line",
        "internal/gui/text/screens.go",
        '"{{.Tool}} - {{.Question}}"',
        '"{{.Question}}"',
        "TestTheMenuOfToolsNamesEachToolAsTheCommandLineDoes",
    ),
    (
        "a verdict that matches is drawn in the grey of every other line",
        "internal/gui/parts/verdict.go",
        "\t\tlabel.Importance = widget.SuccessImportance\n",
        "\t\tlabel.Importance = widget.MediumImportance\n",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        "Copy all copies a long list cut short the way the screen draws it",
        "internal/gui/window/toolstext.go",
        "lines = append(lines, noteLines(d, n, 0)...)",
        "lines = append(lines, noteLines(d, n, 20)...)",
        "TestCopyAllPutsTheWholeResultOnTheClipboard",
    ),
    (
        "Open folder never learns where the run worked",
        "internal/gui/window/tools.go",
        "\tt.offer.folder = ranIn(t.chosen, in)\n",
        "",
        "TestOpenFolderLeadsToTheFolderTheToolWorkedIn",
    ),
    (
        "Open folder leads to the file a tool was given rather than its folder",
        "internal/gui/window/toolstext.go",
        "\t\tfolder = filepath.Dir(named)\n",
        "\t\tfolder = named\n",
        "TestOpenFolderLeadsToTheFolderTheToolWorkedIn",
    ),
    (
        "Open folder outlives a change of tool",
        "internal/gui/window/tools.go",
        "\tt.chosen = d\n\tt.offer.withdraw(t.busy.relay)\n",
        "\tt.chosen = d\n",
        "TestOpenFolderLeadsToTheFolderTheToolWorkedIn",
    ),
    (
        "a language leaves out a word the toolkit asks for",
        "internal/gui/text/locale/toolkit/pl.json",
        "  \"Cancel\": { \"hash\": \"sha256-19766ed6ccb2\", \"other\": \"Anuluj\" },\n",
        "",
        "TestEveryLanguageGivesTheToolkitEveryWord",
    ),
    (
        "the toolkit's words lose one it asks for",
        "internal/gui/text/locale/toolkit/en.json",
        "  \"Cancel\": { \"description\": \"A word of the graphics toolkit the window is built with - in its menus and dialogs. The window gives the toolkit these in the language it speaks.\", \"hash\": \"sha256-19766ed6ccb2\", \"other\": \"Cancel\" },\n",
        "",
        "TestTheToolkitIsGivenEveryWordItAsksFor",
    ),
    (
        "a Polish engine sentence names a value the code does not give",
        "internal/gui/text/locale/said/pl.json",
        "{{.Setting}} {{.Value}} nie ma liczby",
        "{{.Setting}} {{.Wartosc}} nie ma liczby",
        "TestEveryLanguageSaysEverythingTheEngineSays",
    ),
    (
        "two values of one list share a Polish name",
        "internal/gui/text/locale/registry/pl.json",
        "\"Choice.orientation.landscape\": { \"hash\": \"sha256-acb955df82b8\", \"other\": \"poziomo\" }",
        "\"Choice.orientation.landscape\": { \"hash\": \"sha256-acb955df82b8\", \"other\": \"pionowo\" }",
        "TestEveryListSaysEachValueOnceInEveryLanguage",
    ),
    (
        "an engine sentence names a value by the name the setting slot keeps",
        "internal/core/size.go",
        "has no number: write something like 10mb or 1048576\", A(\"Value\", s))",
        "has no number: write something like 10mb or 1048576\", A(\"Setting\", s))",
        "TestEverySentenceTheEngineSaysCanBeTranslated",
    ),
    (
        "the code says an engine sentence the English copy does not hold",
        "internal/core/size.go",
        "{setting} %q has no number: write something like 10mb or 1048576",
        "{setting} %q has no number: write a number like 10mb or 1048576",
        "TestTheEnglishCopyOfTheEngineSentencesSaysWhatTheCodeSays",
    ),
    (
        "a tool refusal is said again as bare text",
        "internal/tool/refusals.go",
        "func (e *SettingError) Unwrap() error { return e.Err }",
        "func (e *SettingError) Unwrap() error { return errors.New(e.Err.Error()) }",
        "TestNoRefusalOfTheEngineIsWrittenAsBareText",
    ),
    (
        "the pseudo language says the engine's sentences around the catalogue",
        "internal/gui/text/said.go",
        "\tif pseudo {\n\t\treturn fill(pseudoOf(core.NamedLayout(",
        "\tif false && pseudo {\n\t\treturn fill(pseudoOf(core.NamedLayout(",
        "TestNoSentenceOfTheEngineReachesTheWindowUndisguised",
    ),
    (
        "a Polish description names a list value in English",
        "internal/gui/text/locale/registry/pl.json",
        "Mieszana zmienia je na przemian",
        "Mixed zmienia je na przemian",
        "TestNoTranslationNamesAListValueTheListCallsOtherwise",
    ),
    (
        "the closed box of a list draws the value rather than its name",
        "internal/gui/parts/menulook.go",
        "\tif m.menu.ShownAs == nil || m.menu.Selected == \"\" {",
        "\tif true || m.menu.ShownAs == nil || m.menu.Selected == \"\" {",
        "TestTheEnginesRefusalsReachThePolishWindowInPolish",
    ),
    (
        "a refusal wrapping nothing spreads into no refusal and marks no box",
        "internal/core/refusal.go",
        "func (r *Refusal) Unwrap() error {\n\tif wrapped := r.said.wrapped(); len(wrapped) > 0 {\n\t\treturn wrapped[0]\n\t}\n\treturn nil\n}",
        "func (r *Refusal) Unwrap() []error { return r.said.wrapped() }",
        "TestTheEnginesRefusalsReachThePolishWindowInPolish",
    ),
    (
        "the English licence notice loses the gap between its paragraphs",
        "internal/gui/text/registry.go",
        "\treturn strings.Join(out, \"\\n\\n\")\n",
        "\treturn strings.Join(out, \"\\n\")\n",
        "TestTheEnglishWindowSaysTimeAndTheLicenceAsTheCommandLineDoes",
    ),
    (
        "a preset title in the window drifts from the site's",
        "internal/gui/text/locale/registry/pl.json",
        "\"Title.size-boundaries\": { \"hash\": \"sha256-21dd0ecec76d\", \"other\": \"Granice rozmiaru\" }",
        "\"Title.size-boundaries\": { \"hash\": \"sha256-21dd0ecec76d\", \"other\": \"Granice rozmiarów\" }",
        "TestThePresetTitlesInTheWindowAreTheSites",
    ),
    # --- the window's WinGet package installs the Windows installer first (2026-10-06) ---
    (
        "the command line's WinGet package reaches tfg through a link",
        "packaging/winget/installer.cli.yaml.in",
        "ArchiveBinariesDependOnPath: true",
        "ArchiveBinariesDependOnPath: false",
        "TestEveryWingetPackageKeepsItsProgramBesideItsFiles",
    ),
    (
        "the window's Windows installer is declared for one account instead of the machine",
        "packaging/winget/installer.window.yaml.in",
        "  Scope: machine",
        "  Scope: user",
        "TestTheWindowInstallsWithAStartMenuShortcutWhereWinGetCan",
    ),
    (
        "WinGet looks for the installed window under an UpgradeCode without its braces",
        ".github/scripts/build_packages.py",
        '"UPGRADE_CODE": "{%s}" % upgrade_code,',
        '"UPGRADE_CODE": "%s" % upgrade_code,',
        "TestTheWindowInstallsWithAStartMenuShortcutWhereWinGetCan",
    ),
    (
        "the window's Windows installer carries its checksum in small letters",
        ".github/scripts/build_packages.py",
        '"MSI_SHA256": sums[msi].upper(),',
        '"MSI_SHA256": sums[msi],',
        "TestEachWingetPackageOffersTheArchitecturesTheReleaseBuilds",
    ),
    (
        "a WinGet render without the installer's line is not refused by name",
        ".github/scripts/build_packages.py",
        '    if "winget" in feeds:',
        "    if False:",
        "TestTheRendererRefusesInputsThatLookFineAndAreWrong",
    ),
    (
        "a render of one feed writes the other feed as well",
        ".github/scripts/build_packages.py",
        "    for feed in feeds:",
        "    for feed in FEEDS:",
        "TestAReleaseWithoutTheInstallerStillRendersForChocolatey",
    ),
    (
        "the window's Chocolatey install takes over the Windows installer's shortcut",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        "if ($keep) {",
        "if ($false) {",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    # --- review of #164 (2026-10-06) ---
    (
        "the window's Chocolatey install takes over a shortcut that points at no file",
        "packaging/chocolatey/tools/chocolateyinstall.window.ps1.in",
        "    if (-not $target) {",
        "    if ($false) {",
        "TestTheWindowPackageStartsWhereAPersonCanWrite",
    ),
    # --- a length of time, the kind a video's duration is written in
    #     (2026-10-06, docs/WIDEO-2026-10-06.md section 12) ---
    (
        "a minute counts as a second",
        "internal/core/duration.go",
        "return 1, 60_000, true",
        "return 1, 1_000, true",
        "TestDurationsCountInWholeMilliseconds",
    ),
    (
        "a length that is not a whole millisecond is rounded down instead of refused",
        "internal/core/duration.go",
        "if f*per%scale != 0 {",
        "if f*per%scale != 0 && false {",
        "TestADurationThatIsNotAWholeMillisecondIsRefused",
    ),
    (
        "units written twice or out of order are added up",
        "internal/core/duration.go",
        "if index <= previous {",
        "if index <= previous && false {",
        "TestNonsenseDurationsAreRefusedWithTheReason",
    ),
    (
        "a written length drops the leading zeros of its milliseconds",
        "internal/core/duration.go",
        'strings.TrimRight(strconv.FormatInt(1000+rest%1000, 10)[1:], "0")',
        'strings.TrimRight(strconv.FormatInt(rest%1000, 10), "0")',
        "TestAWrittenDurationReadsBackAsTheSameMilliseconds",
    ),
    (
        "a duration setting takes a length outside its range",
        "internal/format/format.go",
        "if ms < p.Min || ms > p.Max {",
        "if ms < p.Min && ms > p.Max {",
        "TestADurationSettingTakesALengthInsideItsRangeAndSaysSo",
    ),
    (
        "a fraction of a unit is divided before it is multiplied, so 0.025h comes to nothing",
        "internal/core/duration.go",
        "part := f * per / scale",
        "part := f / scale * per",
        "TestEveryWayOfWritingOneLengthGivesTheSameMilliseconds",
    ),
    # --- the first film format, WebM (2026-10-06, docs/WIDEO-2026-10-06.md
    #     section 13) ---
    (
        "every frame after the first shows the key frame again instead of its copy",
        "internal/format/video/av1.go",
        "w.bits(copySlot, 3)",
        "w.bits(0, 3)",
        "TestAFilmNeverShowsAKeyFrameASecondTime",
    ),
    (
        "the hidden copy is written as not showable, so showing it is refused",
        "internal/format/video/av1.go",
        "w.bit(1)     // showable_frame",
        "w.bit(0)     // showable_frame",
        "TestEveryFrameOfAFilmSurvivesItsReferenceTool",
    ),
    (
        "the container says the film lasts a millisecond longer than it does",
        "internal/format/webm/layout.go",
        "appendFloat(b, idDuration, float64(s.DurationMs))",
        "appendFloat(b, idDuration, float64(s.DurationMs+1))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        "level 4.0 admits a lower display rate than the specification gives it",
        "internal/format/video/level.go",
        "{8, 2359296, 6144, 3456, 70_778_880, 12_000, 4},",
        "{8, 2359296, 6144, 3456, 60_000_000, 12_000, 4},",
        "TestAFilmDeclaresTheLowestLevelTheSpecificationAllows",
    ),
    (
        "a length that does not end on a frame is cut to the frame before it",
        "internal/format/video/timeline.go",
        "if ms*int64(fps)%1000 == 0 && ms > 0 {",
        "if ms > 0 {",
        "TestALengthThatDoesNotEndOnAFrameIsRefusedWithTheTwoNearest",
    ),
    (
        "a film's track declares its width as its height, and a player shapes every frame wrongly",
        "internal/format/webm/layout.go",
        "	v = appendUint(v, idPixelHeight, uint64(s.Height))",
        "	v = appendUint(v, idPixelHeight, uint64(s.Width))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        # Every picture looks like the first, so the film is coded once and
        # shown for its whole length - the photograph the owner saw.
        "a film's pictures all look like its first, so the picture never moves",
        "internal/format/video/pictures.go",
        "		l := p.crew.film.lookOf(p.next)",
        "		l := p.crew.film.lookOf(0)",
        "TestAFilmPictureChangesWhereTheManifestSays",
    ),
    (
        # A change opens with the picture shown again instead of the new one,
        # and the file stays well formed in both containers - only the decoded
        # frames show the picture standing still.
        "a change of picture is written as the old picture shown again",
        "internal/format/video/timeline.go",
        "	case t.StartsChange(i), t.IsKey(i - 1):",
        "	case t.IsKey(i - 1):",
        "TestAFilmPictureChangesWhereTheManifestSays",
    ),
    (
        # The reserve falls back to a tenth above the first picture, which a
        # small film's later pictures pass by up to 42 percent.
        "a film of a named size reserves a tenth above its first picture instead of above its sample",
        "internal/format/video/choose.go",
        "	for _, tiles := range pictures {",
        "	for _, tiles := range pictures[:1] {",
        "TestAFilmOfANamedSizeKeepsEveryPictureInsideItsReserve",
    ),
    (
        # Back to the model that counted two pictures a second whatever the
        # film, which declares too low a level for a film coded every frame.
        "the level's one second buffer counts two pictures whatever the film carries",
        "internal/format/video/level.go",
        "		if int64(perSecond)*8*int64(frameBytes) > l.mainKbps*1000 {",
        "		if 2*8*int64(frameBytes) > l.mainKbps*1000 {",
        "TestAFilmDeclaresTheLowestLevelTheSpecificationAllows",
    ),
    (
        # One object per frame, the defect the film's own allocation ceiling
        # sits below: measured at 698 objects against a ceiling of 512.
        "a film allocates a block header for every frame it writes",
        "internal/format/webm/webm.go",
        "		out.write(block[:])",
        "		out.write(append([]byte(nil), block[:]...))",
        "TestNoGeneratorHoldsTheWholeFileInMemory",
    ),
    (
        # A painter that forgets the square in its strips paints a different
        # picture from the whole painter, which draws it.
        "a film's strip painter leaves the square out",
        "internal/format/video/picture.go",
        "		draw.Draw(s, image.Rect(l.x, f.squareY, l.x+f.side, f.squareY+f.side), f.square, image.Point{}, draw.Src)",
        "		_ = f.square",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        # A helper that codes a picture other than the one it was given makes
        # a film whose bytes differ from the film of one goroutine.
        "a helper codes its picture with the square at the start",
        "internal/format/video/ahead.go",
        "			c.code(p, j, true)",
        "			j.look.x = 0\n			c.code(p, j, true)",
        "TestAFilmCodedBySeveralGoroutinesHasTheBytesOfOne",
    ),
    (
        # A crew that never takes a helper leaves the comparison a film with
        # itself - the guard has to say nothing was coded beside.
        "a film never takes a helper",
        "internal/format/video/ahead.go",
        "	if most := min(int64(runtime.GOMAXPROCS(0)-1), jobs-1); most > 0 {",
        "	if most := int64(0); most > 0 {",
        "TestAFilmCodedBySeveralGoroutinesHasTheBytesOfOne",
    ),
    (
        # A film written without closing its pictures leaves its helpers
        # waiting on a queue nobody will close.
        "a written film does not close its pictures",
        "internal/format/webm/webm.go",
        "	defer pics.Close()",
        "	_ = pics.Close",
        "TestAFilmStoppedHalfWayLeavesNoHelperRunning",
    ),
    (
        # A film that never reports its pictures' work leaves the bar counting
        # the padding as the film - five percent for a minute, then all of it.
        "a film's clusters never report the work of their pictures",
        "internal/format/webm/webm.go",
        "			format.Worked(ctx, pics.Work())",
        "			_ = pics.Work()",
        "TestAFilmsProgressMovesWithItsPicturesNotItsPadding",
    ),
    (
        # The command line works its percentage out of the bytes again, the
        # way a film's bar stood at five percent for its whole minute.
        "the command line's bar takes its percentage from the bytes",
        "internal/cli/progress.go",
        "		pr.Percent(),",
        "		int(100*pr.BytesDone/max(pr.BytesTotal, 1)),",
        "TestTheBarAndTheTimeLeftComeFromTheWorkOnBothSurfaces",
    ),
    (
        # Progress gives its percentage by the bytes, which both bars draw.
        "a report's percentage counts the bytes rather than the work",
        "internal/engine/progress.go",
        "func (p Progress) Percent() int { return core.Percent(p.WorkDone, p.WorkTotal) }",
        "func (p Progress) Percent() int { return core.Percent(p.BytesDone, p.BytesTotal) }",
        "TestTheBarAndTheTimeLeftComeFromTheWorkOnBothSurfaces",
    ),
    (
        # The writer swallows a helper's panic and goes on with an empty
        # picture, where the run should end in an error.
        "the writer swallows the panic a helper kept",
        "internal/format/video/ahead.go",
        "		panic(j.panicked)",
        "\t\treturn tileCoded{}, nil",
        "TestAPanicOnAHelperComesOutOnTheGoroutineThatAskedForIt",
    ),
    (
        # A tile's key leaves out the clock's characters, so a tile of the
        # clock from an earlier picture is taken for a later one - the film
        # decodes, and its clock reads wrong.
        "a tile's key leaves out the clock",
        "internal/format/video/tilekey.go",
        "	for c := p.chars[0]; c < p.chars[1]; c++ {",
        "	for c := p.chars[0]; c < p.chars[0]; c++ {",
        "TestEveryTileOfAFilmIsGav1dsCodingOfItsPixelsAndDecodesAsItDoesAlone",
    ),
    (
        # tile_info says a row of tiles one superblock taller than it is, so
        # every tile after it starts in the wrong place.
        "tile_info says a row of tiles taller than it is",
        "internal/format/video/grid.go",
        "		w.ns(s-1, min(b.sbRows-start, max(b.maxArea()/widest, 1))) // height_in_sbs_minus_1",
        "		w.ns(min(s, min(b.sbRows-start, max(b.maxArea()/widest, 1))-1), min(b.sbRows-start, max(b.maxArea()/widest, 1))) // height_in_sbs_minus_1",
        "TestEveryTileOfAFilmIsGav1dsCodingOfItsPixelsAndDecodesAsItDoesAlone",
    ),
    (
        # A tile is read from the picture's planes with its own width as the
        # stride, so gav1d codes pixels from the wrong rows.
        "a tile is read with its own width as the stride",
        "internal/format/video/still.go",
        "Src: p.Y[luma:], SrcStride: p.Width,",
        "Src: p.Y[luma:], SrcStride: r.Dx(),",
        "TestEveryTileOfAFilmIsGav1dsCodingOfItsPixelsAndDecodesAsItDoesAlone",
    ),
    (
        # Every film is one tile again, so the guards about films of tiles
        # ask about films that have none - each has to say so.
        "every film is one tile, asked of the reference tool's guard",
        "internal/format/video/grid.go",
        "			break\n		}\n		return out\n",
        "			break\n		}\n		break\n",
        "TestEveryFrameOfAFilmSurvivesItsReferenceTool",
    ),
    (
        "every film is one tile, asked of the guard on where the picture changes",
        "internal/format/video/grid.go",
        "			break\n		}\n		return out\n",
        "			break\n		}\n		break\n",
        "TestAFilmPictureChangesWhereTheManifestSays",
    ),
    (
        "every film is one tile, asked of the guard on a named size's reserve",
        "internal/format/video/grid.go",
        "			break\n		}\n		return out\n",
        "			break\n		}\n		break\n",
        "TestAFilmOfANamedSizeKeepsEveryPictureInsideItsReserve",
    ),
    (
        "every film is one tile, asked of the guard on helpers",
        "internal/format/video/grid.go",
        "			break\n		}\n		return out\n",
        "			break\n		}\n		break\n",
        "TestAFilmCodedBySeveralGoroutinesHasTheBytesOfOne",
    ),
    (
        # A picture wider than 4096 and only a superblock and a little tall:
        # without the narrowing its last row of tiles is one superblock
        # holding a pixel or two, which Annex A refuses (81 900 layouts of the
        # sweep on 2026-10-07, docs/WEBM-LIMIT-2026-10-07.md section 2).
        "a film's columns are no longer narrowed so a row of tiles can be two superblocks, and the last row of a wide low picture is under 8 pixels",
        "internal/format/video/grid.go",
        "\t\twidest = min(widest, max(area/2, 1))\n",
        "\t\t_ = area\n",
        "TestEveryFilmLayoutIsOneTheAV1SpecificationAllows",
    ),
    (
        "a film's column may be one superblock wider than MAX_TILE_WIDTH",
        "internal/format/video/grid.go",
        "\twidest := maxTileWidth / superblock\n",
        "\twidest := maxTileWidth/superblock + 1\n",
        "TestEveryFilmLayoutIsOneTheAV1SpecificationAllows",
    ),
    (
        "a film's row of tiles may be twice as tall as tile_info can code",
        "internal/format/video/grid.go",
        "\tg.rows = cutEach(g.rows, max(area/slices.Max(g.cols), 1))\n",
        "\tg.rows = cutEach(g.rows, 2*max(area/slices.Max(g.cols), 1))\n",
        "TestEveryFilmLayoutIsOneTheAV1SpecificationAllows",
    ),
    (
        # The order of the parts is in the bytes of every film cut this way.
        "a film's cut parts put the larger ones first, and the bytes of a wide film move",
        "internal/format/video/grid.go",
        "\t\t\tout = append(out, s/n+(k+s%n)/n)\n",
        "\t\t\tout = append(out, s/n+(n-1-k+s%n)/n)\n",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "a film picture one pixel over the largest an AV1 level describes is no longer refused",
        "internal/format/video/settings.go",
        "\tmaxPixels = 8192 * 4352\n",
        "\tmaxPixels = 8192*4352 + 4352\n",
        "TestAFilmPictureLargerThanAnyAV1LevelIsRefusedNotBroken",
    ),
    (
        "a film picture wider than 4096 is refused again",
        "internal/format/video/settings.go",
        "\tmaxWidth  = 16384\n",
        "\tmaxWidth  = 4096\n",
        "TestAFilmPictureLargerThanAnyAV1LevelIsRefusedNotBroken",
    ),
    (
        # A tile the clock or the square crosses is coded from the film's own
        # planes, which hold the gradient alone - the film decodes, and shows
        # neither.
        "a film's changing tiles are taken from the gradient, without the clock and the square",
        "internal/format/video/picture.go",
        "\tif !p.f.changes(r) {\n",
        "\tif true {\n",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        "a film's tile copies its chroma from the left of the picture rather than from under the tile",
        "internal/format/video/picture.go",
        "\t\td, s := (at.Y/2+cy-r.Min.Y/2)*dcw+at.X/2, cy*scw+r.Min.X/2\n",
        "\t\td, s := (at.Y/2+cy-r.Min.Y/2)*dcw+at.X/2, cy*scw\n",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        "a film's tile converts its luma into the wrong columns",
        "internal/format/video/picture.go",
        "\t\t\tp.Y[(y-at.Y)*w+x-at.X] = uint8(16 + (47*red+157*g+16*b+128)>>8)\n",
        "\t\t\tp.Y[(y-at.Y)*w+x-at.X/2] = uint8(16 + (47*red+157*g+16*b+128)>>8)\n",
        "TestEveryTileOfAFilmIsGav1dsCodingOfItsPixelsAndDecodesAsItDoesAlone",
    ),
    (
        # The rows of a tile outside its strips keep what the painter's last
        # tile left there rather than the gradient.
        "a film's painted tile does not start from the film's planes",
        "internal/format/video/picture.go",
        "\tcopyRect(out, image.Point{}, f.basePlanes, r)\n",
        "\t_ = copyRect\n",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        # The label's last row comes out as the gradient under it - the film
        # made a band at a time differs from the picture painted whole.
        "a film's ground leaves the last row of the label's band to the gradient",
        "internal/format/video/picture.go",
        "\t\tif y < g.label.Rect.Max.Y {\n",
        "\t\tif y < g.label.Rect.Max.Y-1 {\n",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        "a film's ground leaves the last two rows of every band of sixty four unconverted",
        "internal/format/video/picture.go",
        "\t\tr := image.Rect(0, y, p.Width, min(p.Height, y+superblock))\n",
        "\t\tr := image.Rect(0, y, p.Width, min(p.Height, y+superblock-2))\n",
        "TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture",
    ),
    (
        # The codec configuration keeps its four bytes and loses the sequence
        # header, which every key sample still carries - so libaom plays the
        # film, and only the comparison with the WebM sees the difference.
        "an MP4's codec configuration leaves out the sequence header",
        "internal/format/mp4/layout.go",
        "	e = append(e, boxEnd(append(boxStart(\"av1C\"), s.Config()...))...)",
        "	e = append(e, boxEnd(append(boxStart(\"av1C\"), s.Config()[:4]...))...)",
        "TestAnMP4CarriesTheFramesOfTheWebMOfTheSameRequest",
    ),
    (
        "an MP4 lists the frame after each key frame as the one to start from",
        "internal/format/mp4/mp4.go",
        "		o.u32(uint32(k*t.KeyEvery + 1))",
        "		o.u32(uint32(k*t.KeyEvery + 2))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        # Every chunk a byte late: the tables still add up, and every sample
        # read from them is the wrong bytes.
        "an MP4 places every chunk a byte after its samples",
        "internal/format/mp4/mp4.go",
        "			o.u32(uint32(front + content))",
        "			o.u32(uint32(front + content + 1))",
        "TestAFilmNeverShowsAKeyFrameASecondTime",
    ),
    (
        "an MP4's media header says the film is a frame longer",
        "internal/format/mp4/layout.go",
        "	b = appendU32s(b, 0, 0, uint32(s.FPS), uint32(s.Frames))",
        "	b = appendU32s(b, 0, 0, uint32(s.FPS), uint32(s.Frames+1))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        "an MP4's track declares its width as its height",
        "internal/format/mp4/layout.go",
        "	return boxEnd(appendU32s(b, uint32(s.Width)<<16, uint32(s.Height)<<16))",
        "	return boxEnd(appendU32s(b, uint32(s.Height)<<16, uint32(s.Height)<<16))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        # The same bytes in twice the time, which only counting the tiles
        # coded can see.
        "an MP4's second pass takes none of the tiles its first pass kept",
        "internal/format/video/pictures.go",
        "		known: p.known, kept: p.kept, seen: p.seen,",
        "		known: map[tileKey]*job{}, kept: p.kept, seen: p.seen,",
        "TestAnMP4CodesNoTileItsFirstPassKept",
    ),
    (
        "an MP4's first pass never reports the work of its pictures",
        "internal/format/mp4/mp4.go",
        "		format.Worked(ctx, first.Work())",
        "		_ = first.Work()",
        "TestAFilmsProgressMovesWithItsPicturesNotItsPadding",
    ),
    (
        "a written MP4 does not close the pictures of its second pass",
        "internal/format/mp4/mp4.go",
        "	defer pics.Close()",
        "	_ = pics.Close",
        "TestAFilmStoppedHalfWayLeavesNoHelperRunning",
    ),
    (
        "an MP4 stopped in its first pass does not close its pictures",
        "internal/format/mp4/mp4.go",
        "	defer first.Close()",
        "	_ = first.Close",
        "TestAFilmStoppedHalfWayLeavesNoHelperRunning",
    ),
    (
        "an MP4 carries its padding in a box that is not free",
        "internal/format/isobmff/isobmff.go",
        "		if err := WriteBoxHeader(w, \"free\", payload); err != nil {",
        "		if err := WriteBoxHeader(w, \"skip\", payload); err != nil {",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        "an MP4's chunk runs say each chunk holds a sample more",
        "internal/format/mp4/mp4.go",
        "			o.u32(uint32(samples))",
        "			o.u32(uint32(samples + 1))",
        "TestAFilmHoldsTheFramesTheManifestDeclares",
    ),
    (
        # sdtp is read by no decoder here, so only the structural check, which
        # reads every sample's frames against it, can see a wrong one.
        "an MP4 says a key frame depends on other samples",
        "internal/format/mp4/mp4.go",
        "	dependsOnNone   = 2 << 4",
        "	dependsOnNone   = 1 << 4",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # One object per frame, the defect the film's own allocation ceiling
        # sits below: measured at 1241 objects against a ceiling of 1024.
        "an MP4 allocates a copy of every frame it writes",
        "internal/format/mp4/mp4.go",
        "			o.write(frame[t.SampleAt(i)])",
        "			o.write(append([]byte(nil), frame[t.SampleAt(i)]...))",
        "TestNoGeneratorHoldsTheWholeFileInMemory",
    ),
    (
        # Without the step down, padding a byte past a full box leaves a
        # second box of one byte, which no box can be.
        "free boxes past two gigabytes leave a leftover too short to be a box",
        "internal/format/isobmff/isobmff.go",
        "		payload -= BoxHeader",
        "		payload -= 0",
        "TestFreeBoxesCarryPaddingOfAnyLength",
    ),
    (
        # Without the cap the remainder of a one-feature file goes into the note
        # whole, and past ten million characters GDAL refuses the file. The guard
        # asks the checker how many spaces it found and refuses nought.
        "a geojson note is never capped, so spaces never take the remainder",
        "internal/format/geojson/feature.go",
        "\tnote := min(fill, noteCap)",
        "\tnote := fill",
        "TestAGeoJSONNotePastAMebibyteBecomesSpacesAndSurvivesItsReferenceTool",
    ),
    (
        "a geojson feature number is not handed back, so the ids skip",
        "internal/format/geojson/feature.go",
        "\tr.next--\n\tr.seen = r.before\n",
        "\tr.seen = r.before\n",
        "TestRecordNumbersRunFromOneWithoutAGap",
    ),
    (
        # Every kind the same: the right size, valid GeoJSON, and a mixed file
        # that is points from end to end.
        "geojson mixed draws one kind for every feature",
        "internal/format/geojson/settings.go",
        "\treturn s.kinds[(n-1)%int64(len(s.kinds))]",
        "\treturn s.kinds[0]",
        "TestEveryFormatSurvivesItsReferenceTool",
    ),
    (
        # A closing feature measured a byte short per point than the one
        # written, at every vertex count above three.
        "the smallest geojson feature is carried forward with the wrong step",
        "internal/format/geojson/feature.go",
        "\t\tlongest = max(longest, three+int64(r.s.vertices-minVertices)*(four-three))",
        "\t\tlongest = max(longest, three+int64(r.s.vertices-minVertices)*(four-three-1))",
        "TestTheSmallestGeoJSONFeatureCarriedForwardIsTheOneMeasuredWhole",
    ),
    (
        "geojson winding=reversed writes outlines counter-clockwise all the same",
        "internal/format/geojson/geometry.go",
        "\tr.run(from, outline, r.s.reversed)",
        "\tr.run(from, outline, false)",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # The longitude past 180 is written as it was drawn, 180.5 rather than
        # -179.5 - a position outside the range RFC 7946 allows.
        "a geojson shape across the antimeridian is never wrapped",
        "internal/format/geojson/shape.go",
        "\tif x > g.lon {",
        "\tif x > g.lon && false {",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # .005 written as .5 - fewer places than ordered, and a different value.
        "geojson coordinates lose the leading zeros of their fraction",
        "internal/format/geojson/layout.go",
        "\tfor lead := scale / 10; lead > frac && lead > 1; lead /= 10 {",
        "\tfor lead := scale / 10; lead > frac && lead > 1 && false; lead /= 10 {",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # An outline that does not end where it starts, one position short.
        "a geojson outline is never closed",
        "internal/format/geojson/shape.go",
        "\td.reverse(from)\n\td.pos = append(d.pos, d.pos[first:first+d.stride]...)\n}\n\n// half",
        "\td.reverse(from)\n\td.pos = append(d.pos, d.pos[first:first]...)\n}\n\n// half",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # Both halves of an outline above its middle, so the way back can cross
        # the way out - the shape shapely is there to judge.
        "the bottom of a geojson outline is drawn above its middle",
        "internal/format/geojson/shape.go",
        "\td.half(b, bottom, c-1, -1)",
        "\td.half(b, bottom, c+1, 1)",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        "geojson offers a line one point more than the globe has room for",
        "internal/format/geojson/shape.go",
        "\t\treturn g.room() + 1",
        "\t\treturn g.room() + 2",
        "TestGeoJSONRefusesMorePointsThanTheGlobeHasRoomFor",
    ),
    (
        # The feature built only to learn it does not fit still widens the box
        # of the collection, so the box takes in a place the file does not hold.
        # Reddened by hand on 2026-10-07: the bbox case at its smallest size.
        "a geojson feature thrown away still widens the collection box",
        "internal/format/geojson/feature.go",
        "\tr.next--\n\tr.seen = r.before\n",
        "\tr.next--\n",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # Holes wound like their outline - GDAL and shapely take both, so only
        # the checker's signed area sees it.
        "geojson holes run the same way round as their outline",
        "internal/format/geojson/geometry.go",
        "\t\tr.run(at, at+holePositions, !r.s.reversed)",
        "\t\tr.run(at, at+holePositions, r.s.reversed)",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # The box of a shape across the antimeridian written the naive way, from
        # the smaller written longitude to the larger - a box round the other
        # side of the globe.
        "a geojson bbox across the antimeridian is written the naive way round",
        "internal/format/geojson/geometry.go",
        "\twest, east := g.wrap(x.lo[0]), g.wrap(x.hi[0])",
        "\twest, east := min(g.wrap(x.lo[0]), g.wrap(x.hi[0])), max(g.wrap(x.lo[0]), g.wrap(x.hi[0]))",
        "TestAGeoJSONBoxAcrossTheAntimeridianOrRoundTheGlobeSurvivesItsReferenceTool",
    ),
    (
        "a geojson bbox reaching round the globe is not the whole circle",
        "internal/format/geojson/geometry.go",
        "\tif x.hi[0]-x.lo[0] >= 2*g.lon {",
        "\tif x.hi[0]-x.lo[0] >= 2*g.lon && false {",
        "TestAGeoJSONBoxAcrossTheAntimeridianOrRoundTheGlobeSurvivesItsReferenceTool",
    ),
    (
        "geojson offers one hole more than half the globe has room for",
        "internal/format/geojson/shape.go",
        "func mostHoles(g grid) int64 { return (g.lon - 2) / holeCell }",
        "func mostHoles(g grid) int64 { return (g.lon + 2) / holeCell }",
        "TestGeoJSONRefusesHolesTheOutlineHasNoRoomFor",
    ),
    (
        "geojson takes holes in an outline of five points",
        "internal/format/geojson/geojson.go",
        "s.vertices < minVerticesWithHoles || ",
        "",
        "TestGeoJSONRefusesHolesTheOutlineHasNoRoomFor",
    ),
    (
        # Holes a step from each other and from the outline. At fifteen places a
        # step is less than a double tells apart, and GEOS read such holes as
        # touching and the polygon as cut in two (measured 2026-10-07).
        "geojson holes keep a step clear rather than a share of their cell",
        "internal/format/geojson/shape.go",
        "\tmargin := max(1, cell/holeCell)",
        "\tmargin := int64(1)",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        "geojson unlocated=some takes no place from any feature",
        "internal/format/geojson/settings.go",
        "\tif s.unlocated == Some && n%unlocatedEvery == 0 {",
        "\tif s.unlocated == Some && n%unlocatedEvery == 0 && false {",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        "geojson ids=mixed writes every id as a number",
        "internal/format/geojson/settings.go",
        "\t\tif n%2 == 0 {\n\t\t\treturn String",
        "\t\tif n%2 == 0 && false {\n\t\t\treturn String",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # The closing feature measured without the box of the collection, so a
        # file at its smallest size cannot be reached.
        "the smallest geojson feature leaves out the collection's bbox",
        "internal/format/geojson/feature.go",
        "r.closing = r.epilogue(r.closing[:0], widestBox(r.s, r.g))",
        "r.closing = r.epilogue(r.closing[:0], extent{})",
        "TestEveryGeoJSONSettingSurvivesItsReferenceTool",
    ),
    (
        # The odd byte alternative cannot split goes into the HTML as a letter
        # rather than a space: the size holds, and the HTML says a word the
        # text part does not.
        "eml alternative puts its odd byte in the html as a letter",
        "internal/format/eml/message.go",
        "\t\to.put(\" \")\n",
        "\t\to.put(\"x\")\n",
        "TestAlternativePutsTheOddByteInTheHTML",
    ),
    (
        # The label is left out of a message too small for it, and the note
        # says something else: the size holds and label_embedded is right.
        "eml leaves out its label under a code nothing reads",
        "internal/format/eml/eml.go",
        "\t\tCode: \"label_omitted\",\n",
        "\t\tCode: \"label_dropped\",\n",
        "TestAnEMLTooSmallForItsLabelSaysItCarriesNone",
    ),
    (
        # rfc2231 writes the name in Content-Type as well - counted by the code
        # that writes it, so the size holds, and every reader still finds the
        # name. Only the structural check knows where it was ordered to be.
        "eml rfc2231 also writes the name where content-type puts it",
        "internal/format/eml/attach.go",
        "\tcase RFC2231:\n\t\tinDisposition = rfc2231Params(s, name)\n",
        "\tcase RFC2231:\n\t\tinType = []string{quotedParam(s, \"name\", name)}\n\t\tinDisposition = rfc2231Params(s, name)\n",
        "TestEveryEMLSettingSurvivesItsReferenceTool",
    ),
    (
        # Every attached file is made with its neighbour's seed: the sizes,
        # the names and the structure hold, and the files are not the ones the
        # message's seed makes (RC4).
        "eml attaches files made with the next file's seed",
        "internal/format/eml/attach.go",
        "c, err := p.plan(desc, g.Bytes, len(p.out))",
        "c, err := p.plan(desc, g.Bytes, len(p.out)+1)",
        "TestGosStandardLibraryReadsEveryEMLTheWayItWasMeasured",
    ),
    # --- eml, the second set (2026-10-13, docs/EML-2-2026-10-13.md) ---
    (
        "a text in base64 at an odd size with crlf stops taking the long Message-ID",
        "internal/format/eml/eml.go",
        "m.blanks, m.longID = int(left/eol), left%eol == 1",
        "m.blanks, m.longID = int(left/eol), false",
        "TestEveryBase64TextSizeIsExact",
    ),
    (
        "windows-1250 writes the s with an acute the way iso-8859-2 does",
        "internal/format/eml/text.go",
        "'ś': 0x9c",
        "'ś': 0xb6",
        "TestTheCharsetsWriteTheLettersTheyDifferIn",
    ),
    (
        "a message attached to a message stops taking the line endings of the one around it",
        "internal/format/eml/attach.go",
        "r.Properties = map[string]string{LineEndings: p.s.endings}",
        "r.Properties = map[string]string{}",
        "TestAMessageCarriesAMessageAsItIs",
    ),
    (
        "message_depth forwards one message fewer than asked",
        "internal/format/eml/attach.go",
        "if p.s.depth < 2 {",
        "if p.s.depth < 3 {",
        "TestDepthForwardsTheSmallestMessagesInsideEachOther",
    ),
    (
        "the pictures take a seed that is not the one of their place",
        "internal/format/eml/inline.go",
        "first := len(p.out)",
        "first := len(p.out) + 1",
        "TestPicturesAreTheFilesTheirSeedsMakeAndTheHTMLShowsEach",
    ),
    (
        "To loses its last recipient",
        "internal/format/eml/header.go",
        "for i := 2; i <= m.s.recipients && !o.cancelled(); i++ {",
        "for i := 2; i < m.s.recipients && !o.cancelled(); i++ {",
        "TestToCarriesEveryRecipientOnALineOfItsOwn",
    ),
    (
        "any format is taken as a picture in the text",
        "internal/format/picture/picture.go",
        'return desc, strings.HasPrefix(desc.MediaType, "image/"), nil',
        'return desc, true, nil',
        "TestAPictureInTheTextHasToBeAPicture",
    ),
    (
        "a message with files refused as too small stops saying they travel in base64",
        "internal/format/eml/eml.go",
        "if len(m.children)+len(m.pictures) > 0 {",
        "if false {",
        "TestEMLRefusesASizeBelowItsAttachmentsInBase64",
    ),
    (
        "a label written by hand is left behind under a key no setting declares",
        "internal/gui/text/labels.go",
        '"inline":        "Pictures",',
        '"inlines":       "Pictures",',
        "TestEveryLabelWrittenByHandBelongsToADeclaredSetting",
    ),
    # --- HTML carries pictures inside the page (2026-10-14,
    #     docs/HTML-3-2026-10-14.md) ---
    (
        "a picture in the page takes the seed of the page instead of the seed of its place",
        "internal/format/htmlfile/pictures.go",
        "Seed: core.FileSeed(r.Seed, i), Label: r.Label}",
        "Seed: r.Seed, Label: r.Label}",
        "TestAPictureInAPageIsTheFileItsFormatWritesAlone",
    ),
    (
        "the floor of a page leaves its pictures out",
        "internal/format/htmlfile/html.go",
        "min := minimumFor(s) + pictureBytes",
        "min := minimumFor(s)",
        "TestEveryHTMLSettingAnswersForItsOwnMinimum",
    ),
    (
        "the manifest of a page stops recording its pictures",
        "internal/format/htmlfile/html.go",
        "\tdescribePictures(p.Properties, s.pictures, pictures)\n",
        "\t_ = pictures\n",
        "TestEveryHTMLSettingIsRecordedInTheManifest",
    ),
    (
        "a page takes any format as a picture",
        "internal/format/htmlfile/pictures.go",
        "\tif !ok {\n\t\treturn nil, 0, &format.PropertyValueError{",
        "\tif !ok && false {\n\t\treturn nil, 0, &format.PropertyValueError{",
        "TestAPageRefusesPicturesThatAreNotPictures",
    ),
    (
        "the pictures of a page add up past the ceiling the sums are counted under",
        "internal/format/htmlfile/pictures.go",
        "if sum += line.bytes(page.leadBytes()); sum > maxPage {",
        "if sum += line.bytes(page.leadBytes()); false {",
        "TestAPageOfPicturesTooLargeToCountIsRefused",
    ),
    (
        "html declares only some of the picture settings",
        "internal/format/htmlfile/settings.go",
        "return append(own, pictureProperties()...)",
        "return append(own, pictureProperties()[:2]...)",
        "TestEveryFormatWithPicturesDeclaresThemAlike",
    ),
    (
        "the structural check stops being told the pictures of a page",
        "internal/oracle/strict.py",
        "        fail(\"the page carries a script, which no setting of this format writes\")\n    html_links(bare, settings, nl)\n    html_pictures(bare, settings, nl)\n",
        "        fail(\"the page carries a script, which no setting of this format writes\")\n    html_links(bare, settings, nl)\n",
        "TestTheStructuralCheckIsToldEveryHTMLSetting",
    ),
    # --- Stopping: a picture format after the stop, and a container while it
    #     plans the files inside it (2026-10-14, review of #178, O279) ---
    (
        "webp codes its picture to the last row after the run was stopped",
        "internal/format/webp/picture.go",
        "\t\tif stopped != nil {\n\t\t\treturn nil\n\t\t}\n",
        "",
        "TestEveryPictureFormatStopsSoonAfterARunIsStopped",
    ),
    (
        "a zip plans every file inside it whatever the run says",
        "internal/format/zip/children.go",
        "cp, err := format.PlanWith(ctx, desc.Generator, format.Request{",
        "cp, err := desc.Generator.Plan(format.Request{",
        "TestAContainerStopsBetweenTheFilesItPlans",
    ),
    (
        "a mail message hands a stop back as a file that cannot be made",
        "internal/format/eml/attach.go",
        "return child{}, format.StopOr(p.ctx, core.Refuse(",
        "return child{}, (core.Refuse(",
        "TestAContainerStopsBetweenTheFilesItPlans",
    ),
    (
        "the engine plans a container without the run's context",
        "internal/engine/crash.go",
        "return format.PlanWith(ctx, desc.Generator, r)",
        "return format.PlanWith(context.Background(), desc.Generator, r)",
        "TestTheEngineHandsTheStopToAContainerItIsPlanning",
    ),
    (
        "a broken picture inside a page ends the structural check with a traceback",
        "internal/oracle/strict.py",
        "    except Exception as exc:\n        fail(f\"picture {n} is not a good {kind} file: {type(exc).__name__}: {str(exc)[:120]}\")\n",
        "",
        "TestTheStructuralCheckGivesAVerdictOnABrokenPicture",
    ),
    # --- html: a page on one line and a page with styles (2026-10-14,
    #     docs/HTML-4-2026-10-14.md) ---
    (
        "a page on one line keeps the line breaks of the catalogue of every element",
        "internal/format/htmlfile/markup.go",
        "m[bodyCatalogue] = oneLineCatalogue(m[bodyCatalogue])",
        "m[bodyCatalogue] = m[bodyCatalogue]",
        "TestAMinifiedPageIsTheSamePageOnOneLine",
    ),
    (
        "the catalogue on one line drops its line breaks with nothing in their place, the one in pre included",
        "internal/format/htmlfile/markup.go",
        "m[bodyCatalogue] = oneLineCatalogue(m[bodyCatalogue])",
        "m[bodyCatalogue] = strings.Join(strings.Split(m[bodyCatalogue], string(rune(10))), \"\")",
        "TestTheCatalogueOnOneLineReadsAsItDoesOnMany",
    ),
    (
        "the style attributes are spelled in the other layout than the sheet",
        "internal/format/htmlfile/markup.go",
        "css[r.element] = r.css(minified)",
        "css[r.element] = r.css(!minified)",
        "TestInlineStylesAreTheRulesOfTheSheet",
    ),
    (
        "a fragment ordered with embedded styles carries no sheet",
        "internal/format/htmlfile/page.go",
        "p := m[sheet]",
        "p := \"\"",
        "TestAStyleSheetSitsWhereAParserPutsItInTheHead",
    ),
    # --- html: the review of #179 - styles that say nothing, a stray carriage
    #     return, and the rules of a sheet on one line (2026-10-14) ---
    (
        'every style rule declares nothing, in both layouts, sheet and attributes alike',
        'internal/format/htmlfile/markup.go',
        'return strings.Join(parts, sep)',
        'return strings.Join(parts[:0], sep)',
        "TestInlineStylesAreTheRulesOfTheSheet",
    ),
    (
        'the readable style rules lose their colons, so a page of lines declares nothing',
        'internal/format/htmlfile/markup.go',
        'colon, sep := ": ", "; "',
        'colon, sep := " ", "  "',
        "TestInlineStylesAreTheRulesOfTheSheet",
    ),
    (
        'the short style rules lose their colons, so a page on one line declares nothing',
        'internal/format/htmlfile/markup.go',
        'colon, sep = ":", ";"',
        'colon, sep = "", ""',
        "TestInlineStylesAreTheRulesOfTheSheet",
    ),
    (
        'the structural check lets a carriage return through a page of lines',
        "internal/oracle/strict.py",
        "if \"\\r\" in text:\n        fail(f\"formatting={layout} was ordered and the page has",
        "if False:\n        fail(f\"formatting={layout} was ordered and the page has",
        "TestTheStructuralCheckRefusesStylesThatSayNothingAndStrayCarriageReturns",
    ),
    (
        'the structural check takes an empty rule of the sheet for a rule',
        'internal/oracle/strict.py',
        'declared.get(name, False) or bool(HTML_DECLARATION.search(body))',
        'True',
        "TestTheStructuralCheckRefusesStylesThatSayNothingAndStrayCarriageReturns",
    ),
    (
        'the structural check takes an empty style attribute for a style',
        'internal/oracle/strict.py',
        'rest.startswith(\' style="\') and HTML_DECLARATION.search(',
        'rest.startswith(\' style="\') or HTML_DECLARATION.search(',
        "TestTheStructuralCheckRefusesStylesThatSayNothingAndStrayCarriageReturns",
    ),
    (
        'the structural check skips every other rule of a sheet on one line',
        'internal/oracle/strict.py',
        '(?<![a-z0-9-])([a-z0-9]+)',
        '(?:^|[}\\s>])([a-z0-9]+)',
        "TestTheStructuralCheckIsToldEveryHTMLSetting",
    ),
    (
        "GitHub leaves the bar of the four screens it shares with Donate",
        "internal/gui/window/open.go",
        "return []fyne.CanvasObject{donate(h, parts.Quiet).InTheBar(), gitHub(h, parts.Quiet).InTheBar()}",
        "return []fyne.CanvasObject{donate(h, parts.Quiet).InTheBar()}",
        "TestTheGitHubButtonStandsBesideDonateAndOpensTheProjectPage",
    ),
    (
        "GitHub opens the support page rather than the project's",
        "internal/gui/window/open.go",
        "func() { h.OpenLink(text.RepositoryURL) }",
        "func() { h.OpenLink(text.SupportURL) }",
        "TestTheGitHubButtonStandsBesideDonateAndOpensTheProjectPage",
    ),
    (
        "the project address on the About screen is folded into the sentence "
        "beside it, so nobody can select it in one gesture",
        "internal/gui/window/about.go",
        "gitHub(h, parts.Secondary), text.RepositoryURL),",
        'gitHub(h, parts.Secondary), text.DetailGitHub()+" "+text.RepositoryURL),',
        "TestTheProjectAddressStandsOnItsOwnLine",
    ),
    (
        "the licence of the fonts the toolkit carries blocks every toolkit update again",
        ".github/scripts/dependency_gate.py",
        '    "OFL-1.1",\n})',
        "})",
        "TestTheDependencyGateRefusesALicenceThisProjectCannotDistribute",
    ),
    (
        "the packages the semgrep job installs are asked for a licence as if they were shipped",
        ".github/scripts/dependency_gate.py",
        '            and str(d.get("manifest", "")) not in SKIPPED_MANIFESTS]',
        "            ]",
        "TestTheDependencyGateRefusesALicenceThisProjectCannotDistribute",
    ),
    (
        "an indented page indents three spaces a level",
        "internal/format/htmlfile/indent.go",
        "const indentUnit = \"  \"",
        "const indentUnit = \"   \"",
        "TestAnIndentedPageIsThePageOfLinesIndentedByItsElements",
    ),
    (
        "a line that closes an element of an indented page stands at the depth inside it",
        "internal/format/htmlfile/indent.go",
        "\tif strings.HasPrefix(raw, \"</\") {\n\t\tdepth--\n",
        "\tif strings.HasPrefix(raw, \"</\") {\n\t\tdepth -= 0\n",
        "TestAnIndentedPageIsThePageOfLinesIndentedByItsElements",
    ),
    (
        "an indented page indents the text of pre like the rest of its lines",
        "internal/format/htmlfile/indent.go",
        "\tif inPre {\n\t\treturn line{text: raw, pre: true}",
        "\tif false && inPre {\n\t\treturn line{text: raw, pre: true}",
        "TestAnIndentedPageIsThePageOfLinesIndentedByItsElements",
    ),
    (
        "an indented fragment with no style sheet opens with a blank line",
        "internal/format/htmlfile/indent.go",
        "\tif text == \"\" {",
        "\tif false {",
        "TestAnIndentedPageIsThePageOfLinesIndentedByItsElements",
    ),
    (
        "a link of the page goes to a host that is not kept for examples",
        "internal/format/htmlfile/links.go",
        "https://example.org/new",
        "https://exemple.org/new",
        "TestTheLinksAreTheKindsInTurnOnDomainsKeptForExamples",
    ),
    (
        "a page writing its levels and links a chunk at a time never asks whether the run was stopped",
        "internal/format/htmlfile/content.go",
        "\tif err := ctx.Err(); err != nil {",
        "\tif err := error(nil); err != nil {",
        "TestADeepIndentedPageAndAMillionLinksStopSoonAfterARunIsStopped",
    ),
    (
        "the window spells the bom setting out of its key again, as Bom",
        "internal/gui/text/labels.go",
        "\t\"bom\":           \"BOM\",\n",
        "",
        "TestTheRegistryCatalogueSaysWhatTheRegistriesSay",
    ),
    (
        "the structural check stops being told the links of a page",
        "internal/oracle/strict.py",
        "    html_links(bare, settings, nl)\n    html_pictures(bare, settings, nl)\n",
        "    html_pictures(bare, settings, nl)\n",
        "TestTheStructuralCheckIsToldEveryHTMLSetting",
    ),
    (
        "the structural check takes whatever indentation a line has",
        "internal/oracle/strict.py",
        "            want = 2 * max(lead, 0) if indented else 0\n",
        "            want = len(line) - len(body)\n",
        "TestTheStructuralCheckIsToldEveryHTMLSetting",
    ),
    (
        "the label of an indented page stands inside the levels, at the depth of the content",
        "internal/format/htmlfile/html.go",
        "\t\theading = indentation(s.bodyDepth()) + heading",
        "\t\theading = indentation(s.depth()) + heading",
        "TestTheLabelOfAnIndentedPageSitsInTheBody",
    ),
    (
        # Review of #185: the link list read up to the next closing tag, and
        # when there was none the slice ran to the end of the page. The verdict
        # says so before anything is sliced.
        "the html checker slices a list of links that is never closed",
        "internal/oracle/strict.py",
        '    if end < 0:\n        fail(f"links={want} was ordered and the list of links is never closed")\n',
        "    if end < 0:\n        pass\n",
        "TestTheStructuralCheckRefusesAListOfLinksThatIsMoreThanItsLinks",
    ),
    (
        # Review of #185: counted item by item, a list closed only at the end of
        # the page carried every block and still had the right count.
        "the html checker counts the links and stops asking what else the list holds",
        "internal/oracle/strict.py",
        "    if covered != len(inside):\n",
        "    if False:\n",
        "TestTheStructuralCheckRefusesAListOfLinksThatIsMoreThanItsLinks",
    ),
    (
        # Review of #185: the prefix taken out of the text first turned
        # "xn---limit" into "-limit", and the flag in it was gone.
        "the flag guard takes xn-- out of the text before it looks for flags",
        "internal/guard/flagnames_test.go",
        "\tfor _, at := range flagLike.FindAllStringIndex(value, -1) {\n",
        '\tfor _, at := range flagLike.FindAllStringIndex(strings.ReplaceAll(value, "xn--", ""), -1) {\n',
        "TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling",
    ),
    (
        # The prefix opens a label. After a dash it sits inside a name, and the
        # dashes after it are a flag.
        "the flag guard forgives xn-- inside a name",
        "internal/guard/flagnames_test.go",
        "inName := c == '-' || '0' <= c",
        "inName := c == '~' || '0' <= c",
        "TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling",
    ),
    (
        # A newsletter's address to unsubscribe at is the manifest's only for
        # a newsletter (docs/EML-3-2026-10-15.md section 9a).
        "the manifest gives an address to unsubscribe at for every message",
        "internal/format/eml/eml.go",
        "\tif m.s.layout == Newsletter {\n\t\tp.Properties[UnsubscribeURL] = m.id.unsubscribe\n",
        "\tif true {\n\t\tp.Properties[UnsubscribeURL] = m.id.unsubscribe\n",
        "TestANewsletterUnsubscribesAtTheOneAddressItsManifestGives",
    ),
    (
        # The opaque part of the address is the message's own digits.
        "the address to unsubscribe at stops carrying the message's digits",
        "internal/format/eml/header.go",
        "\t\tunsubscribe:   unsubscribeWeb + hex,\n",
        "\t\tunsubscribe:   unsubscribeWeb + hex[1:] + hex[:1],\n",
        "TestANewsletterUnsubscribesAtTheOneAddressItsManifestGives",
    ),
    (
        # The plain text writes out the address its footer unsubscribes at.
        "the plain text of a newsletter stops writing out where to unsubscribe",
        "internal/format/eml/newsletter.go",
        'frameLine(s, "Unsubscribe ("+unsubscribe+")"),',
        'frameLine(s, "Unsubscribe"),',
        "TestANewsletterUnsubscribesAtTheOneAddressItsManifestGives",
    ),
    (
        "the eml checker stops asking what an HTML part would fetch on opening",
        "internal/oracle/strict.py",
        "        if fetched:\n",
        "        if False:\n",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes any footer link as the one to unsubscribe at",
        "internal/oracle/strict.py",
        "        (f'<a href=\"{unsubscribe}\">Unsubscribe</a>' in body, ",
        "        (True, ",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes a frame in a message laid out simple",
        "internal/oracle/strict.py",
        "        if framed:\n",
        "        if False:\n",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker stops reading the lines around a newsletter's plain text",
        "internal/oracle/strict.py",
        '        if lines[:2] != EML_PLAIN_OPEN or lines[-5:] != EML_PLAIN_CLOSE + [f"Unsubscribe ({unsubscribe})"]:\n',
        "        if False:\n",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes the list header lines by their names alone",
        "internal/oracle/strict.py",
        "        if values[name] != value:\n",
        "        if False:\n",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        # Review of #186: every word with :// went from the plain text, so an
        # address added in the middle passed as the same words.
        "the eml checker drops every address from the plain text before comparing words",
        "internal/oracle/strict.py",
        "    return words[:read_more] + words[read_more + 1:-1]\n",
        '    return [w for w in words if "://" not in w]\n',
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        # Review of #186: object data, background, srcset, poster and a
        # refresh carry an address a mail program loads on opening.
        "the eml checker asks only src for an address it would fetch",
        "internal/oracle/strict.py",
        'or (remote and (tag, name) != ("a", "href")):\n',
        'or (remote and name == "src"):\n',
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes a picture in the HTML from anywhere but beside it",
        "internal/oracle/strict.py",
        'if (name == "src" and not value.startswith("cid:")) or',
        'if (name == "src" and False) or',
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes a style sheet linked by a relative address",
        "internal/oracle/strict.py",
        '        if tag == "link":\n            self.found.append("<link>")\n',
        '        if False:\n            self.found.append("<link>")\n',
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker stops reading url( in the styles",
        "internal/oracle/strict.py",
        'EML_FETCHING = re.compile(r"url\\(|@import", re.IGNORECASE)',
        'EML_FETCHING = re.compile(r"@import", re.IGNORECASE)',
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the eml checker takes a newsletter whose sheet declares nothing",
        "internal/oracle/strict.py",
        '    return bool(sheet and re.search(r"\\{[^}:]*:[^}]*\\}", sheet.group(1)))\n',
        "    return True\n",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the pull request scan runs the OSV image without the step that holds it to the recorded digest",
        ".github/workflows/osv-pull-request.yml",
        "- name: the scanner image is the one recorded",
        "- name: the scanner image is checked",
        "TestTheScannerImageIsCheckedBeforeItRuns",
    ),
    (
        "the pull request step named for the image check runs something else, so nothing compares the digest",
        ".github/workflows/osv-pull-request.yml",
        "        run: bash .github/scripts/osv_scanner_image.sh",
        "        run: true",
        "TestTheScannerImageIsCheckedBeforeItRuns",
    ),
    (
        "the weekly scan runs the OSV image without the step that holds it to the recorded digest",
        ".github/workflows/osv-weekly.yml",
        "- name: the scanner image is the one recorded",
        "- name: the scanner image is checked",
        "TestTheScannerImageIsCheckedBeforeItRuns",
    ),
    (
        "the recorded scanner digest loses its algorithm, so the check before every scan fails on every run",
        ".github/osv-scanner-digest",
        "sha256:",
        "",
        "TestTheScannerImageIsCheckedBeforeItRuns",
    ),
    (
        "the scanner image check lets every image through without looking",
        ".github/scripts/osv_scanner_image.sh",
        "set -euo pipefail",
        "set -euo pipefail; exit 0",
        "TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne",
    ),
    (
        "an image pulled under another digest passes the scanner image check",
        ".github/scripts/osv_scanner_image.sh",
        "  status=1",
        "  status=0",
        "TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne",
    ),
    (
        "the scanner image check joins a digest split in two into one that passes",
        ".github/scripts/osv_scanner_image.sh",
        "raw=$(cat -- \"$file\" 2>/dev/null || true)",
        "raw=$(tr -d '[:space:]' 2>/dev/null < \"$file\" || true)",
        "TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne",
    ),
    (
        "a runner with no scanner image passes the check that relies on one",
        ".github/scripts/osv_scanner_image.sh",
        "if [ -z \"$ids\" ]; then",
        "if false; then",
        "TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne",
    ),
    (
        "the structural check takes a page whose title says nothing",
        "internal/oracle/strict.py",
        "        if end < 0 or not text[start + len(\"<title>\"):end].strip():",
        "        if False:",
        "TestTheStructuralCheckRefusesAPageWhoseTitleSaysNothing",
    ),
    (
        "the structural check takes a newsletter page that does not say its language",
        "internal/oracle/strict.py",
        "        ('<html lang=\"en\">' in body, \"language of the page\"),",
        "        (True, \"language of the page\"),",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "a page with no label goes back to an empty title, which the Nu HTML Checker calls an error",
        "internal/format/htmlfile/page.go",
        "const placeholderTitle = `<title>Page</title>`",
        "const placeholderTitle = `<title></title>`",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the page of a newsletter stops saying its language",
        "internal/format/eml/newsletter.go",
        "\t\thtmlTag:  frameLine(s, `<html lang=\"en\">`),",
        "\t\thtmlTag:  frameLine(s, `<html>`),",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the catalogue's forms article is named forms again, which DOMPurify takes out and two links lose",
        "internal/format/htmlfile/catalogue.go",
        "<article id=\"tfg-forms\">",
        "<article id=\"forms\">",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the catalogue's hidden field is named hidden again, which DOMPurify takes out of a submitted form",
        "internal/format/htmlfile/catalogue.go",
        "name=\"tfg-hidden\"",
        "name=\"hidden\"",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        "the structural check takes a newsletter whose Polish line is not marked as Polish",
        "internal/oracle/strict.py",
        "        page.append(('<span lang=\"pl\">' in body, \"the Polish line marked as Polish\"))",
        "        page.append((True, \"the Polish line marked as Polish\"))",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "the structural check takes a newsletter whose Japanese line is not marked as Japanese",
        "internal/oracle/strict.py",
        "            page.append(('<span lang=\"ja\">' in body, \"the Japanese line marked as Japanese\"))",
        "            page.append((True, \"the Japanese line marked as Japanese\"))",
        "TestTheEMLCheckRefusesANewsletterBrokenOnPurpose",
    ),
    (
        "a newsletter's line in Polish and Japanese stops saying its languages under a page that says English",
        "internal/format/eml/text.go",
        "\tif s.layout != Newsletter {",
        "\tif true {",
        "TestOurOwnGeneratorsHaveNotDrifted",
    ),
    (
        # A line end with no space in front of it: RTF ignores the line end, so the
        # two words on either side of it become one in every reader. The size holds,
        # since the closing words take up the byte.
        "rtf breaks a line between two letters",
        "internal/format/rtf/blocks.go",
        "\tlineBreak = \" \\r\\n\"\n",
        "\tlineBreak = \"\\r\\n\"\n",
        "TestAnRTFCutAtAnySizeEndsOnItsBraces",
    ),
    (
        # Two fallback bytes after a number the count says one byte follows: a
        # reader that keeps the count shows the second byte as a letter. The size
        # holds, and only the structural check reads the count.
        "rtf writes the japanese fallback under a count of one",
        "internal/format/rtf/text.go",
        "\t\tdst, uc = appendCount(dst, len(inPage), uc)\n",
        "\t\tdst, uc = appendCount(dst, 1, uc)\n",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # The code page byte of one Polish letter from ISO-8859-2 rather than
        # Windows-1250: one byte for one byte, so the size holds, and the letter reads
        # as another one in every reader that honours the font.
        "rtf writes the polish a in latin 2",
        "internal/format/rtf/text.go",
        "'ą': \"\\xb9\"",
        "'ą': \"\\xb1\"",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # An emoji as one number instead of a surrogate pair - past the signed 16
        # bit number the specification allows. The closing words take up the bytes.
        "rtf writes an emoji as one number past the range",
        "internal/format/rtf/text.go",
        "\tif r < 0x10000 {\n\t\treturn []rune{r}\n",
        "\tif r < 0x110000 {\n\t\treturn []rune{r}\n",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # The properties, the page header and the footer after the label instead of
        # in front of the text: the same bytes in another place, so the size holds.
        "rtf puts the page header and footer after the label",
        "internal/format/rtf/rtf.go",
        "\tif s.every {\n\t\tl.head += catalogueHead\n\t}\n",
        "\tif s.every {\n\t\tl.body += catalogueHead\n\t}\n",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # The hidden text of every shown in italics: one letter for one, so the
        # size holds, and pandoc returns a sentence elements says it leaves out.
        "rtf writes the hidden text in italics",
        "internal/format/rtf/catalogue.go",
        "{\\v This sentence is hidden text.}",
        "{\\i This sentence is hidden text.}",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # literal writes the question mark as a byte, the way unicode does: the
        # closing words take up the difference, and pandoc then reads the Polish
        # line text_encoding says it reads wrong.
        "rtf writes literal the way unicode does",
        "internal/format/rtf/text.go",
        "\t\t\tdst = appendNumber(dst, u)\n\t\t\tdst = append(dst, '?')\n",
        "\t\t\tdst = appendNumber(dst, u)\n\t\t\tdst = appendHex(dst, \"?\")\n",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # The label is left out of a document too small for it, and the note says
        # something else: the size holds and label_embedded is right.
        "rtf leaves out its label under a code nothing reads",
        "internal/format/rtf/rtf.go",
        "\t\t\t\tCode:   \"label_omitted\",\n",
        "\t\t\t\tCode:   \"label_dropped\",\n",
        "TestAnRTFTooSmallForItsLabelSaysItCarriesNone",
    ),
    (
        # The manifest key of the code page spelled another way: every file the
        # same, and a test asserting on the public name finds nothing.
        "rtf records the code page under another key",
        "internal/format/rtf/settings.go",
        "\tCodePage = \"code_page\"\n",
        "\tCodePage = \"codepage\"\n",
        "TestTheRTFManifestRecordsEverySetting",
    ),
    (
        # Every picture planned with the seed of the place after its own: each is
        # still a real file of the size ordered, so the document is the same size,
        # and none is the file its format writes alone for that place (RC4).
        "rtf plans a picture with the seed of the next place",
        "internal/format/rtf/pictures.go",
        "Seed: core.FileSeed(r.Seed, i), Label: r.Label}",
        "Seed: core.FileSeed(r.Seed, i+1), Label: r.Label}",
        "TestAPictureInAnRTFIsTheFileItsFormatWritesAlone",
    ),
    (
        # Width and height swapped in the group: 640 and 480 are three digits each,
        # so the size holds, and the group says a picture the file is not.
        "rtf writes the size of a picture in pixels the other way round",
        "internal/format/rtf/pictures.go",
        "`\\picw%d\\pich%d\\picwgoal%d\\pichgoal%d`, w, h, gw, gh)",
        "`\\picw%d\\pich%d\\picwgoal%d\\pichgoal%d`, h, w, gw, gh)",
        "TestAPictureInAnRTFIsTheFileItsFormatWritesAlone",
    ),
    (
        # The largest picture is no longer narrowed: 9600 and 7200 twips are as many
        # digits as 8640 and 6480, so the size holds, and it stands past the column.
        "rtf lets a picture past the column",
        "internal/format/rtf/pictures.go",
        "\tif gw > columnTwips {\n",
        "\tif gw > columnTwips+960 {\n",
        "TestAnRTFPictureStaysInsideTheColumn",
    ),
    (
        # A gif written as if it were a png: planned and written to the size ordered,
        # and a reader that trusts the word RTF names it by gets a file that is not one.
        "rtf takes gif as a picture it can name",
        "internal/format/rtf/pictures.go",
        "var blips = map[string]string{\"png\": `\\pngblip`, ",
        "var blips = map[string]string{\"gif\": `\\pngblip`, \"png\": `\\pngblip`, ",
        "TestAnRTFRefusesPicturesItCannotCarry",
    ),
    (
        # Without the check of each picture on its own, a picture so large that its
        # hex does not fit in an int64 wraps round and the document is planned at a
        # size that is a lie - measured on 2026-10-15, it came back planned.
        "rtf counts a picture twice its own size without asking whether that fits",
        "internal/format/rtf/pictures.go",
        "\t\tif cp.Bytes > maxDocument {\n\t\t\treturn nil, 0, documentTooLarge(cp.Bytes)\n\t\t}\n",
        "",
        "TestAnRTFOfPicturesTooLargeToCountIsRefused",
    ),
    (
        # The two reasons of the refusal swapped: the same minimum, refused and then
        # taken, and the reason tells a tester who ordered binary that hex doubles it.
        "rtf names hex where the pictures are binary",
        "internal/format/rtf/rtf.go",
        "\tcase s.pictures.Count > 0 && s.binary:\n",
        "\tcase s.pictures.Count > 0 && !s.binary:\n",
        "TestAnRTFTooSmallForItsPicturesSaysHowTheyAreWritten",
    ),
    (
        # The end of the bookmark after the first link rather than the last: the
        # same bytes in another place, so the size holds, and the bookmark the first
        # kind points at no longer spans the list.
        "rtf closes the bookmark of the links after the first one",
        "internal/format/rtf/links.go",
        "\t\tif i == n-1 {\n\t\t\tout.add(bookmarkEnd)\n",
        "\t\tif i == 0 {\n\t\t\tout.add(bookmarkEnd)\n",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # One letter for one in the text of a kind: the size holds, and the link no
        # longer says which kind it is in the words a test reads.
        "rtf writes the text of the relative link wrong",
        "internal/format/rtf/links.go",
        "{`HYPERLINK \"page-2.rtf\"`, \"relative link\"},",
        "{`HYPERLINK \"page-2.rtf\"`, \"relative lynk\"},",
        "TestEveryRTFSettingSurvivesItsReferenceTool",
    ),
    (
        # The list of containers asked to stop shrinks to the two archives, and the
        # guard of stopping goes on green without a format that carries pictures.
        "the containers asked to stop are found by a setting nobody declares",
        "internal/guard/containerstop_test.go",
        "\t\t\tif p.Name == pic.Count {\n",
        "\t\t\tif p.Name == pic.Count+\"_count\" {\n",
        "TestEveryFormatThatCarriesPicturesIsAskedToStop",
    ),
    # --- guards that reached the repository while the entries lived outside it,
    #     found when the coverage guard first ran in CI (2026-10-15) ---
    (
        "a manifest is saved by writing at its name, over whatever another run left there",
        "internal/manifest/manifest.go",
        "func (m *Manifest) Save(path string) error {\n\tr, err := Claim(path)\n\tif err != nil {\n\t\treturn err\n\t}\n\treturn r.Save(m)\n}",
        "func (m *Manifest) Save(path string) error {\n\tf, err := os.Create(path)\n\tif err != nil {\n\t\treturn err\n\t}\n\tdefer f.Close()\n\treturn m.Encode(f)\n}",
        "TestAManifestIsNeverWrittenOverEvenWhenItAppearsMidRun",
    ),
    (
        "the reserved manifest takes its name by a rename that replaces a manifest put there during the run",
        "internal/manifest/reservation.go",
        "\treturn own, core.Publish(tmp, final)\n",
        "\treturn own, os.Rename(tmp, final)\n",
        "TestAManifestPutThereDuringTheRunIsNotWrittenOver",
    ),
    (
        "a run that gives its name back leaves its reservation behind",
        "internal/manifest/reservation.go",
        "\tr.used = true\n\treturn core.RemoveOwn(r.tmp, r.own)\n}",
        "\tr.used = true\n\treturn nil\n}",
        "TestAClaimIsGivenBackAndLeavesNoManifest",
    ),
    (
        "a file is given its name by a rename that replaces whatever another writer put there",
        "internal/core/publish.go",
        "\treturn PublishThrough(tmp, final, renameNoReplace, os.Link)\n",
        "\treturn os.Rename(tmp, final)\n",
        "TestAWriterSpinningOnTheNameNeverLosesItsFile",
    ),
    (
        "eject claims the recipe's name with an empty file before it writes, as it did before O252",
        "internal/cli/presetcmd.go",
        "\tif _, err := core.WriteNew(path, source, 0o644); err != nil {",
        "\tif f, cerr := core.CreateNew(path, 0o644); cerr == nil {\n\t\t_ = f.Close()\n\t}\n\tif _, err := core.WriteNew(path, source, 0o644); err != nil {",
        "TestEjectThatCannotWriteLeavesNoEmptyFile",
    ),
    (
        "pages stop naming the language a search engine serves to everybody else",
        "internal/site/render.go",
        "\t\t\tv.Alternates = append(v.Alternates, Alternate{Lang: \"x-default\", URL: s.Origin() + pageURL(root, mate)})\n",
        "\t\t\t_ = mate\n",
        "TestEveryPageSaysWhichLanguageItIsAndWhereItsTranslationsAre",
    ),
    (
        "the German page describes a damage under a name the program does not have, and loses the one it has",
        "web/content/de/site.json",
        "\"zero-head\":",
        "\"zero-heads\":",
        "TestEveryLanguageDescribesEveryDamage",
    ),
    (
        "archive-scan takes only a part of two dots for the folder above, not one Windows trims to it",
        "internal/tool/archivescan/names.go",
        "return strings.Trim(part, \". \") == \"\" && strings.Count(part, \".\") >= 2",
        "return part == \"..\"",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan takes a backslash for part of a name, as Linux does",
        "internal/tool/archivescan/names.go",
        "func isSeparator(r rune) bool { return r == '/' || r == '\\\\' }",
        "func isSeparator(r rune) bool { return r == '/' }",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan lets a drive letter stay in the folder",
        "internal/tool/archivescan/names.go",
        "return len(name) >= 2 && name[1] == ':' && isLetter(name[0])",
        "return false",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan tells names apart by case, as Linux does and Windows does not",
        "internal/tool/archivescan/names.go",
        "return strings.ToLower(norm.NFC.String(part))",
        "return norm.NFC.String(part)",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan compares accents as they are written, not as macOS stores them",
        "internal/tool/archivescan/names.go",
        "return strings.ToLower(norm.NFC.String(part))",
        "return strings.ToLower(part) + norm.NFC.String(\"\")",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan misses a file standing where another entry needs a folder",
        "internal/tool/archivescan/names.go",
        "\t\tif b.file[id] || file {\n",
        "\t\tif file {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan never sees two entries sharing their bytes",
        "internal/tool/archivescan/zip.go",
        "\t\tif i > 0 && sp.from < reach {\n",
        "\t\tif i > 0 && sp.from < reach && false {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan never compares the name at an entry with the listed one",
        "internal/tool/archivescan/zip.go",
        "\tif !bytes.Equal(local, []byte(e.name)) {\n",
        "\tif false && !bytes.Equal(local, []byte(e.name)) {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan calls a ZIP built to explode only at four gigabytes and more",
        "internal/tool/archivescan/findings.go",
        "if !s.exploded && s.unpacked >= BombBytes && s.unpacked > uint64(s.size)*BombRatio {",
        "if !s.exploded && s.unpacked >= 4*BombBytes && s.unpacked > uint64(s.size)*BombRatio {",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan counts only the contents of a TAR.GZ, so zeros after its TAR are unpacked unseen",
        "internal/tool/archivescan/tar.go",
        "\tin := bufio.NewReaderSize(&inflating{r: z, s: s}, 64<<10)\n",
        "\tin := bufio.NewReaderSize(z, 64<<10)\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan reads a gzip of zeros as an empty TAR",
        "internal/tool/archivescan/tar.go",
        "\tif len(head) < 257+len(tarMagic) || !bytes.Equal(head[257:257+len(tarMagic)], tarMagic) {\n",
        "\tif !isTar(head) && !bytes.Equal(head, nil) {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan never holds contents to their checksum",
        "internal/tool/archivescan/zip.go",
        "\tcase sum.Sum32() != e.crc:\n",
        "\tcase false:\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan misses an entry holding more than the list says",
        "internal/tool/archivescan/zip.go",
        "\tcase got > e.unpacked:\n\t\tz.note(noteLonger, e.name)\n",
        "\tcase false:\n\t\tz.note(noteLonger, e.name)\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan reads an encrypted entry as if it were not",
        "internal/tool/archivescan/zip.go",
        "\tcase e.flags&1 != 0 || e.method == methodAES:\n",
        "\tcase e.method == methodAES:\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan takes a hard link in a TAR for something else than a link",
        "internal/tool/archivescan/tar.go",
        "\tcase tar.TypeSymlink, tar.TypeLink:\n",
        "\tcase tar.TypeSymlink:\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan misses a TAR entry that runs with its owner's rights",
        "internal/tool/archivescan/tar.go",
        "\tif h.Typeflag != tar.TypeDir && h.Mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "\tif false && h.Mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan calls an archive unsafe for holding another archive",
        "internal/tool/archivescan/findings.go",
        "{tool.Note{ID: noteNested, Says: \"Archives inside it, read as well:\"}, true},",
        "{tool.Note{ID: noteNested, Says: \"Archives inside it, read as well:\"}, false},",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan never compares what an archive unpacks to with the limit given",
        "internal/tool/archivescan/findings.go",
        "if limit := s.tree.limit; limit >= 0 && all > uint64(limit) {",
        "if limit := s.tree.limit; limit < 0 && all > uint64(limit) {",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan reads a ZIP of more than 65 535 entries without its ZIP64 record",
        "internal/tool/archivescan/ziplist.go",
        "\t\tif e.entries == 0xFFFF || e.listSize == 0xFFFFFFFF || e.listAt == 0xFFFFFFFF {\n",
        "\t\tif false {\n",
        "TestTheArchiveScanReadsTheListArchiveZipReads",
    ),
    (
        "a conclusion a tool declares failing ends the command line with success",
        "internal/tool/tool.go",
        "\treturn ok && c.Fails\n",
        "\treturn false && ok && c.Fails\n",
        "TestTheArchiveScanEndsWithTheCodeOfWhatHappened",
    ),
    (
        "the Tools tab leaves out the conclusion a tool came to",
        "internal/gui/window/toolsresult.go",
        "\tif said, ok := conclusionSaid(d, *r); ok {\n",
        "\tif said, ok := conclusionSaid(d, *r); ok && false {\n",
        "TestTheToolsTabLooksIntoAnArchive",
    ),
    (
        "the conclusions a tool declares never reach the window's catalogue",
        "internal/gui/text/registrywords.go",
        "\tfor _, c := range d.Conclusions {\n",
        "\tfor _, c := range d.Conclusions[:0] {\n",
        "TestTheRegistryCatalogueSaysWhatTheRegistriesSay",
    ),
    (
        "an empty limit of a tool says it is worked out from the size, as a format's does",
        "internal/gui/parts/property.go",
        "\tcase p.Kind == format.PropertyText || ofATool:\n",
        "\tcase p.Kind == format.PropertyText:\n",
        "TestEveryScreenStillDrawsItsStoredPicture",
    ),
    (
        "archive-scan counts the header git archive writes for every entry after it as an entry of an unknown type",
        "internal/tool/archivescan/tar.go",
        "\t\tif h.Typeflag == tar.TypeXGlobalHeader {\n",
        "\t\tif false && h.Typeflag == tar.TypeXGlobalHeader {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan passes over a header that gives every entry after it a name",
        "internal/tool/archivescan/tar.go",
        "\t\tif !globalHarmless[key] && (first == \"\" || key < first) {\n",
        "\t\tif false && (first == \"\" || key < first) {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan takes the commit git archive writes for something that changes the entries",
        "internal/tool/archivescan/tar.go",
        "\t\"comment\": true, ",
        "\t\"comment\": false, ",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan says a TAR folder a group shares runs with its owner's rights",
        "internal/tool/archivescan/tar.go",
        "\tif h.Typeflag != tar.TypeDir && h.Mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "\tif h.Mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan says a ZIP folder a group shares runs with its owner's rights",
        "internal/tool/archivescan/tar.go",
        "\tif mode&0xF000 != modeFolder && mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "\tif mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan misses a ZIP entry that runs with its owner's rights",
        "internal/tool/archivescan/tar.go",
        "\tif mode&0xF000 != modeFolder && mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "\tif false && mode&(modeSetUser|modeSetGroup) != 0 {\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan reads every deflated ZIP entry after the first from where the first one ended",
        "internal/tool/archivescan/zip.go",
        "\t_ = z.inflate.(flate.Resetter).Reset(src, nil)\n",
        "\t_ = src\n",
        "TestTheArchiveScanFindsEveryTrapItNames",
    ),
    (
        "archive-scan never finds the archives inside one built to explode together",
        "internal/tool/archivescan/inside.go",
        "\tif t.unpacked == r.unpacked || t.unpacked < BombBytes || t.unpacked <= uint64(r.size)*BombRatio {\n",
        "\tif true || t.unpacked == r.unpacked || t.unpacked < BombBytes || t.unpacked <= uint64(r.size)*BombRatio {\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan looks into archives inside a ZIP however deep they go",
        "internal/tool/archivescan/zip.go",
        "\tz.hold.start(z.tree, e.unpacked, look && z.depth < MostDepth)\n",
        "\tz.hold.start(z.tree, e.unpacked, look)\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan holds an archive inside however large it grows",
        "internal/tool/archivescan/inside.go",
        "\tif want <= 0 || want > int64(MostInside-h.t.held) {\n",
        "\tif want <= 0 {\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan passes over an archive inside it does not read, as if nothing were there",
        "internal/tool/archivescan/inside.go",
        "\tcase !readable(h.kind):\n",
        "\tcase false:\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan keeps what it finds in an archive inside out of the notes",
        "internal/tool/archivescan/inside.go",
        "\t\t\ts.tree.add(s.found.deep, s.found.deepMore, d.ID, deeper(name, f))\n",
        "\t\t\t_ = f\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan calls an archive safe with an archive inside that is not",
        "internal/tool/archivescan/inside.go",
        "\ts.nest.unsafe = true\n",
        "\ts.nest.unsafe = false\n",
        "TestTheArchiveScanReportsAnArchiveInsideOnItsOwn",
    ),
    (
        "archive-scan reads an archive held in pieces from the first piece only",
        "internal/tool/archivescan/inside.go",
        "\t\tc := copy(p[n:], h.parts[off/piece][off%piece:])\n",
        "\t\tc := copy(p[n:], h.parts[0][off%piece:])\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan reads text that begins with the letters of bzip2 as bzip2",
        "internal/tool/archivescan/sniff.go",
        "\treturn bytes.Equal(magic, []byte{0x31, 0x41, 0x59, 0x26, 0x53, 0x59}) || bytes.Equal(magic, []byte{0x17, 0x72, 0x45, 0x38, 0x50, 0x90})\n",
        "\treturn len(magic) >= 0\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "archive-scan takes any file beginning with the two bytes of gzip for gzip",
        "internal/tool/archivescan/sniff.go",
        "gzipMagic = []byte{0x1f, 0x8b, 0x08}",
        "gzipMagic = []byte{0x1f, 0x8b}",
        "TestTheArchiveScanEndsWithTheCodeOfWhatHappened",
    ),
    (
        "archive-scan lists every finding in a note, however many there are",
        "internal/tool/archivescan/report.go",
        "\tif len(list) >= MostListed || t.listed >= MostListedBytes {\n",
        "\tif t.listed >= MostListedBytes {\n",
        "TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds",
    ),
    (
        "archive-scan lists a name whole, however long it is",
        "internal/tool/archivescan/report.go",
        "\tn := min(len(name), MostShownName)\n",
        "\tn := len(name)\n",
        "TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds",
    ),
    (
        "archive-scan stops a note at its limit without saying how many more there are",
        "internal/tool/archivescan/report.go",
        "\tif more > 0 {\n\t\twords := core.Says(\"archivescan.More\"",
        "\tif false {\n\t\twords := core.Says(\"archivescan.More\"",
        "TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds",
    ),
    (
        "archive-scan leaves the archive inside out of what --json reports",
        "internal/tool/archivescan/report.go",
        "\t\tr.Inside = append(r.Inside, c.report())\n",
        "\t\t_ = c\n",
        "TestTheArchiveScanReportsAnArchiveInsideOnItsOwn",
    ),
    (
        "archive-scan holds a name set in a PAX header as a slice of the whole header",
        "internal/tool/archivescan/names.go",
        "\tname = strings.Clone(name)\n",
        "\tname = name + \"\"\n",
        "TestTheArchiveScanHoldsNoMoreOfANameThanTheName",
    ),
    (
        "archive-scan keeps the name of an archive inside as a slice of a PAX header",
        "internal/tool/archivescan/inside.go",
        "\tc := blank(cutName(name), h.size, s.tree)\n",
        "\tc := blank(name, h.size, s.tree)\n",
        "TestTheArchiveScanHoldsNoMoreOfANameThanTheName",
    ),
    (
        "archive-scan cuts a name inside a character",
        "internal/tool/archivescan/report.go",
        "\tfor back := 1; back < utf8.UTFMax && n < len(name) && !utf8.RuneStart(name[n]); back++ {\n",
        "\tfor back := 1; false && back < utf8.UTFMax && n < len(name) && !utf8.RuneStart(name[n]); back++ {\n",
        "TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds",
    ),
    (
        "archive-scan calls an archive inside too large when only the room left is too small",
        "internal/tool/archivescan/inside.go",
        "\tcase h.need > MostInside:\n",
        "\tcase h.need > 0:\n",
        "TestTheArchiveScanLooksIntoTheArchivesInside",
    ),
    (
        "a changelog entry grows past a hundred words on a line of its own",
        "CHANGELOG.md",
        "  card of its own with the address written out, for a desktop that opens\n",
        "  card of its own with the address written out, for a desktop that opens\n  word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word\n",
        "TestEveryChangelogEntryFitsInAHundredWords",
    ),
    (
        "a changelog entry grows past a hundred words in a second paragraph",
        "CHANGELOG.md",
        "  card of its own with the address written out, for a desktop that opens\n  nothing.\n",
        "  card of its own with the address written out, for a desktop that opens\n  nothing.\n\n  word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word word\n",
        "TestEveryChangelogEntryFitsInAHundredWords",
    ),
    # --- the container image, ghcr.io/donislawdev/tfg (2026-10-20) ---
    (
        "the image puts an operating system under the program",
        ".github/docker/Dockerfile",
        "FROM scratch",
        "FROM alpine:3.20",
        "TestTheImageHoldsTheReleasedProgramAndNothingElse",
    ),
    (
        "the image runs the program as root",
        ".github/docker/Dockerfile",
        "USER 65532:65532",
        "USER 0:0",
        "TestTheImageHoldsTheReleasedProgramAndNothingElse",
    ),
    (
        "the image runs something while it is built",
        ".github/docker/Dockerfile",
        'ENTRYPOINT ["/tfg"]',
        'RUN ["/tfg", "version"]\nENTRYPOINT ["/tfg"]',
        "TestTheImageHoldsTheReleasedProgramAndNothingElse",
    ),
    (
        "the image leaves out the notices the licences require",
        ".github/docker/Dockerfile",
        "COPY LICENSE THIRD-PARTY-NOTICES.md /licenses/",
        "COPY LICENSE /licenses/",
        "TestTheImageHoldsTheReleasedProgramAndNothingElse",
    ),
    (
        "the image starts the program through a shell form, so docker stop no longer reaches it",
        ".github/docker/Dockerfile",
        'ENTRYPOINT ["/tfg"]',
        "ENTRYPOINT /tfg",
        "TestTheImageHoldsTheReleasedProgramAndNothingElse",
    ),
    (
        "the image workflow unpacks the archive before its checksum is asked",
        ".github/workflows/docker.yml",
        "          sha256sum -c this.txt\n",
        '          tar -xzf "$archive" -C ../context\n          sha256sum -c this.txt\n',
        "TestTheImageIsMadeOfAReleaseThatCheckedOut",
    ),
    (
        "the image workflow accepts an attestation from any workflow of the repository",
        ".github/workflows/docker.yml",
        '--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yml"',
        '--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/docker.yml"',
        "TestTheImageIsMadeOfAReleaseThatCheckedOut",
    ),
    (
        "the image is pushed from the job a pull request runs",
        ".github/workflows/docker.yml",
        '        run: docker save --output "image-$ARCH.tar" "tfg:$ARCH"',
        '        run: docker save --output "image-$ARCH.tar" "tfg:$ARCH" && docker push "tfg:$ARCH"',
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the image is pushed without waiting for the images to be tried",
        ".github/workflows/docker.yml",
        "    needs: [which, image]",
        "    needs: [which]",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the job that pushes the image runs for a pull request too",
        ".github/workflows/docker.yml",
        "    if: needs.which.outputs.publish == 'true'\n    runs-on: ubuntu-latest",
        "    if: always()\n    runs-on: ubuntu-latest",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the image is kept without comparing what it writes with what the program writes",
        ".github/workflows/docker.yml",
        "          diff program.sums image.sums\n",
        "          true\n",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "every job of the image workflow can write packages",
        ".github/workflows/docker.yml",
        "permissions:\n  contents: read\n\nconcurrency:",
        "permissions:\n  contents: read\n  packages: write\n\nconcurrency:",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the image workflow logs out of the registry before the attestation is pushed there",
        ".github/workflows/docker.yml",
        "      - uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8  # v4.2.2\n"
        "        with:\n"
        "          subject-name: ${{ env.IMAGE }}\n"
        "          subject-digest: ${{ steps.push.outputs.digest }}\n"
        "          push-to-registry: true\n"
        "\n"
        "      - name: log out of the registry\n"
        "        if: always()\n"
        "        run: docker logout ghcr.io\n",
        "      - name: log out of the registry\n"
        "        if: always()\n"
        "        run: docker logout ghcr.io\n"
        "\n"
        "      - uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8  # v4.2.2\n"
        "        with:\n"
        "          subject-name: ${{ env.IMAGE }}\n"
        "          subject-digest: ${{ steps.push.outputs.digest }}\n"
        "          push-to-registry: true\n",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the image workflow skips the logout when a step before it failed",
        ".github/workflows/docker.yml",
        "        if: always()\n        run: docker logout ghcr.io",
        "        run: docker logout ghcr.io",
        "TestOnlyAnImageThatWasTriedIsPushed",
    ),
    (
        "the check of a published release reads the list of sentences directly",
        ".github/workflows/verify-release.yml",
        '          latest="$(gh api',
        '          grep -F -f .github/release-note-must-say.txt notes.md\n          latest="$(gh api',
        "TestWhatAReleaseNoteHasToSayIsDecidedInOnePlace",
    ),
    (
        "the README install step stops checking the attestation",
        "README.md",
        '    gh attestation verify "$archive" --repo donislawdev/TestingFilesGenerator \\\n'
        "      --predicate-type https://spdx.dev/Document/v2.3\n",
        "",
        "TestTheReadmeInstallStepChecksTheArchiveBeforeOpeningIt",
    ),
    (
        "the README install step names a predicate type the signed archives do not carry",
        "README.md",
        "      --predicate-type https://spdx.dev/Document/v2.3\n    mkdir tfg",
        "      --predicate-type https://slsa.dev/provenance/v1\n    mkdir tfg",
        "TestTheReadmeInstallStepChecksTheArchiveBeforeOpeningIt",
    ),
    (
        "the README install step unpacks the archive before its attestation is checked",
        "README.md",
        '    gh attestation verify "$archive"',
        '    tar -xzf "$archive" -C tfg\n    gh attestation verify "$archive"',
        "TestTheReadmeInstallStepChecksTheArchiveBeforeOpeningIt",
    ),
    (
        "the README install step gives gh no token",
        "README.md",
        "  env:\n    GH_TOKEN: ${{ github.token }}\n  run: |\n    version=",
        "  run: |\n    version=",
        "TestTheReadmeInstallStepChecksTheArchiveBeforeOpeningIt",
    ),
    # Each writes a 9 in front of the version rather than naming one, so these
    # three need no change when the version is raised.
    (
        "the README pulls an image of another version than this build",
        "README.md",
        '-v "${PWD}:/work" ghcr.io/donislawdev/tfg:',
        '-v "${PWD}:/work" ghcr.io/donislawdev/tfg:9',
        "TestEveryVersionTheReadmeWritesOutIsThisOne",
    ),
    (
        "the README install step for GitHub Actions downloads another version than this build",
        "README.md",
        "    version=",
        "    version=9",
        "TestEveryVersionTheReadmeWritesOutIsThisOne",
    ),
    (
        "the README example manifest names another version than this build",
        "README.md",
        '"version": "',
        '"version": "9',
        "TestEveryVersionTheReadmeWritesOutIsThisOne",
    ),
    (
        # R01 of docs/STABILITY-REVIEW-2026-10-26.md: a file judged ours and
        # changed before the deletion reached it went, without --force.
        "cleanup deletes a file that changed after it was judged ours, without asking the disk again",
        "internal/audit/cleanup.go",
        "if why, changed := changedSince(full, c.seen); changed {",
        "if why, changed := changedSince(full, c.seen); false && changed {",
        "TestCleanupLeavesAFileThatChangedAfterItWasChecked",
    ),
    (
        "a file put in the place of the one judged ours is taken for it when its size and time match",
        "internal/audit/cleanup.go",
        "if !os.SameFile(seen, now) || now.Size()",
        "if now.Size()",
        "TestCleanupLeavesAFileReplacedAfterItWasChecked",
    ),
    (
        "a file written over in place at its own length is still taken for the one judged ours",
        "internal/audit/cleanup.go",
        "now.Size() != seen.Size() || !now.ModTime().Equal(seen.ModTime()) {",
        "now.Size() != seen.Size() {",
        "TestCleanupLeavesAFileThatChangedAfterItWasChecked",
    ),
    (
        "verify calls every failure to look a missing file again",
        "internal/audit/audit.go",
        "if absent(err) {\n\t\treturn Difference{Kind: Missing",
        "if err != nil {\n\t\treturn Difference{Kind: Missing",
        "TestVerifyCallsANameItCouldNotLookUpUnreadableRatherThanMissing",
    ),
    (
        "a listed file is read to its end after Ctrl+C",
        "internal/audit/read.go",
        "if err := ctx.Err(); err != nil {\n\t\t\treturn read, err",
        "if err := ctx.Err(); false && err != nil {\n\t\t\treturn read, err",
        "TestHashingAListedFileStopsPartWayWhenCancelled",
    ),
    (
        "a listed file is read past its size and one byte, for as long as it grows",
        "internal/audit/read.go",
        "piece = piece[:left]",
        "_ = left",
        "TestHashingAListedFileReadsNoMoreThanItIsAskedFor",
    ),
    (
        # Unix only: a pipe exists nowhere else. The mutation runner of the CI
        # is Linux, so it is judged there.
        "verify and cleanup open a listed file with a plain open again, which waits on a pipe",
        "internal/audit/read.go",
        "f, seen, err := core.OpenRegular(path)",
        "f, seen, err := func() (*os.File, os.FileInfo, error) { _ = core.OpenRegular; f, err := os.Open(path); if err != nil { return nil, nil, err }; i, err := f.Stat(); return f, i, err }()",
        "TestVerifyAndCleanupNeverWaitOnAPipe",
    ),
    (
        # R03 of docs/STABILITY-REVIEW-2026-10-26.md: a manifest past the
        # ceiling went out without the field that says so.
        "a saved manifest past the ceiling no longer says it cannot be read back",
        "internal/manifest/size.go",
        "header.Summary.TooLargeToReadBack = true",
        "header.Summary.TooLargeToReadBack = false",
        "TestASavedManifestSaysItIsTooBigExactlyWhenItIs",
    ),
    (
        "a manifest of exactly the ceiling's length says it cannot be read back, though it is read",
        "internal/manifest/size.go",
        "size <= MaxBytes {",
        "size < MaxBytes {",
        "TestASavedManifestSaysItIsTooBigExactlyWhenItIs",
    ),
    (
        "Save writes the manifest without settling the field from its real length",
        "internal/manifest/reservation.go",
        "size, err := m.settleReadBack()",
        "size, err := m.SavedBytes(), error(nil)",
        "TestASavedManifestSaysItIsTooBigExactlyWhenItIs",
    ),
    (
        "the counted length of a manifest leaves out the indentation of the first line of every entry",
        "internal/manifest/size.go",
        "return int64(len(entryIndent) + len(b)), nil",
        "return int64(len(b)), nil",
        "TestTheManifestSizeIsTheLengthOfTheManifest",
    ),
    (
        # R08: the free space was held against the files alone.
        "the free space is held against the files alone again, so a disk with no room for the manifest takes the files",
        "internal/engine/preflight.go",
        "core.AddSizes(TotalBytes(files), record.Bytes())",
        "core.AddSizes(TotalBytes(files), 0)",
        "TestARunWithRoomForItsFilesButNotItsRecordIsRefused",
    ),
    (
        "the forecast of a run's record leaves out the instructions written beside the manifest",
        "internal/engine/recordsize.go",
        "return int64(len(whole.Instructions(filepath.Base(ManifestPath(opt))))), nil",
        "return 0, nil",
        "TestTheRecordARunIsToldBeforehandIsTheRecordItWrites",
    ),
    # R17 (docs/STABILITY-REVIEW-2026-10-26.md): a size with a decimal point is
    # read to the byte, not through a float, and a hostile one is answered
    # without reading all of it.
    (
        "a fraction longer than sixty digits is taken as whole when its first sixty are",
        "internal/core/size.go",
        "return n, !long && rest.Sign() == 0",
        "return n, rest.Sign() == 0",
        "TestASizeWithADecimalPointIsReadToTheByte",
    ),
    (
        "a fraction just under the largest size asks for a byte past it",
        "internal/core/size.go",
        "if !n.IsInt64() || (!exact && n.Int64() == math.MaxInt64) {",
        "if !n.IsInt64() {",
        "TestASizeWithADecimalPointIsReadToTheByte",
    ),
    (
        "four million digits before the point are read one by one",
        "internal/core/size.go",
        "if len(whole) > 19 {",
        "if len(whole) > 19 && false {",
        "TestASizeWithADecimalPointIsReadToTheByte",
    ),
    (
        "a fraction of four million digits is read to its end",
        "internal/core/size.go",
        "long := len(fraction) > fractionDigits",
        "long := false",
        "TestASizeWithADecimalPointIsReadToTheByte",
    ),
    (
        "a decimal size forgets its unit",
        "internal/core/size.go",
        "n.Mul(n, big.NewInt(unit))",
        "n.Mul(n, big.NewInt(1))",
        "TestEveryExactDecimalOfAUnitReadsBackAsItself",
    ),
    (
        "a negative size with a decimal point is taken as its size without the sign",
        "internal/core/size.go",
        "if negative && (whole != \"\" || fraction != \"\") {",
        "if negative && false {",
        "TestNonsenseSizesAreRefusedWithAUsefulMessage",
    ),
    (
        "a size with two signs is read as the number after them",
        "internal/core/size.go",
        "if s != \"\" && (s[0] == '+' || s[0] == '-') {\n\t\ts = s[1:]\n\t}",
        "s = strings.TrimPrefix(strings.TrimPrefix(s, \"+\"), \"-\")",
        "TestNonsenseSizesAreRefusedWithAUsefulMessage",
    ),
    # R13 (docs/STABILITY-REVIEW-2026-10-26.md): a set laid out around a limit
    # is refused, not wrapped, when the limit has no room above it or the file
    # far over it is past the largest size.
    (
        "a set around a limit with no room above it wraps its step over the limit",
        "internal/preset/limitset.go",
        "if s.limit > math.MaxInt64-far.bytes {",
        "if s.limit > math.MaxInt64-far.bytes && false {",
        "TestEveryPresetAnswersTheLargestSizeAboutTheTop",
    ),
    (
        "a preset's refusal of a limit with no room above it stops being the one --boundary gives",
        "internal/preset/limitset.go",
        "Cause: core.ErrBoundaryTooLarge,",
        "Cause: nil,",
        "TestEveryPresetAnswersTheLargestSizeAboutTheTop",
    ),
    (
        "a preset's refusal hides the refusal it is",
        "internal/preset/preset.go",
        "func (e *ImpossibleError) Unwrap() error { return e.Cause }",
        "func (e *ImpossibleError) Unwrap() error { return nil }",
        "TestThePresetAnswersALimitTooLargeTheSameWayTheFlagDoes",
    ),
    (
        "the file far over the limit wraps round instead of being refused",
        "internal/preset/uploadset.go",
        "if s.farOver == 0 || s.limit <= math.MaxInt64/s.farOver {",
        "if true {",
        "TestTheFileFarOverTheLimitIsThatManyTimesTheLimitOrNothing",
    ),
    (
        "the largest limit the file far over it fits is refused as well",
        "internal/preset/uploadset.go",
        "if s.farOver == 0 || s.limit <= math.MaxInt64/s.farOver {",
        "if s.farOver == 0 || s.limit < math.MaxInt64/s.farOver {",
        "TestTheFileFarOverTheLimitIsThatManyTimesTheLimitOrNothing",
    ),
    # R15 (docs/STABILITY-REVIEW-2026-10-26.md): a message above the largest
    # the format counts in is refused before anything is counted at its size.
    (
        "a message above the largest the format counts in is planned, and in base64 never comes back",
        "internal/format/eml/eml.go",
        "if !r.SizeFromContents && r.Bytes > maxMessage {",
        "if !r.SizeFromContents && r.Bytes > maxMessage && false {",
        "TestAnEMLAboveTheLargestMessageIsRefusedAtOnce",
    ),
    (
        "the largest message itself is refused",
        "internal/format/eml/eml.go",
        "if !r.SizeFromContents && r.Bytes > maxMessage {",
        "if !r.SizeFromContents && r.Bytes >= maxMessage {",
        "TestAnEMLAboveTheLargestMessageIsRefusedAtOnce",
    ),
    # R16 (docs/STABILITY-REVIEW-2026-10-26.md): the spreadsheet is expected
    # whole only as wide as a spreadsheet is.
    (
        "a spreadsheet wider than a spreadsheet shows is still expected to be taken whole",
        "internal/preset/tabularimport.go",
        "if wide, _ := strconv.Atoi(columns); wide > spreadsheetColumns {",
        "if wide, _ := strconv.Atoi(columns); wide > spreadsheetColumns && false {",
        "TestTheSpreadsheetIsExpectedWholeOnlyAsWideAsASpreadsheet",
    ),
    (
        "a spreadsheet exactly as wide as a spreadsheet shows stops being expected whole",
        "internal/preset/tabularimport.go",
        "if wide, _ := strconv.Atoi(columns); wide > spreadsheetColumns {",
        "if wide, _ := strconv.Atoi(columns); wide >= spreadsheetColumns {",
        "TestTheSpreadsheetIsExpectedWholeOnlyAsWideAsASpreadsheet",
    ),
    # R14 (docs/STABILITY-REVIEW-2026-10-26.md): a picture says it carries its
    # label only when the text of the label is whole in it.
    (
        "a label cut off at the foot of a low picture is said to be carried",
        "internal/format/imagelabel/imagelabel.go",
        "if !drawn || Readable(width, height, chars) {",
        "if true {",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    (
        "a label is called readable from its width alone",
        "internal/format/imagelabel/imagelabel.go",
        "return Fits(width, chars) && height >= textHeight(width, chars)",
        "return Fits(width, chars)",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    (
        "a label whole in its picture is called cut off",
        "internal/format/imagelabel/imagelabel.go",
        "return Fits(width, chars) && height >= textHeight(width, chars)",
        "return Fits(width, chars) && height > 1<<20",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    (
        "a film says it carries a label cut off at the foot of its picture",
        "internal/format/video/choose.go",
        "return Labelled(c.Width, c.Label) && imagelabel.Readable(c.Width, c.Height, len(c.Label))",
        "return Labelled(c.Width, c.Label)",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    (
        "a BMP says it carries a label cut off at the foot of its picture",
        "internal/format/bmp/bmp.go",
        "labelled, p.Notes = imagelabel.Claim(labelled, w, h, len(label), p.Notes)",
        "labelled, p.Notes = imagelabel.Claim(labelled, w, 1<<20, len(label), p.Notes)",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    (
        "an AVIF says it carries a label cut off at the foot of its picture",
        "internal/format/avif/avif.go",
        "carries, p.Notes = imagelabel.Claim(carries, m.width, m.height, len(label), p.Notes)",
        "carries, p.Notes = imagelabel.Claim(carries, m.width, 1<<20, len(label), p.Notes)",
        "TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole",
    ),
    # The review of 2026-10-27 on the R13 and R14 guards.
    (
        "the farthest step of a set is not the one asked whether it fits above the limit",
        "internal/preset/limitset.go",
        "if o.bytes > far.bytes {",
        "if false {",
        "TestEveryPresetAnswersTheLargestSizeAboutTheTop",
    ),
    (
        "a limit too large for its spread is told the wrong limit to use instead",
        "internal/preset/limitset.go",
        "core.A(\"Most\", math.MaxInt64-far.bytes)",
        "core.A(\"Most\", math.MaxInt64-1)",
        "TestEveryPresetAnswersTheLargestSizeAboutTheTop",
    ),
    (
        "a label whose text exactly fills the picture is called cut off",
        "internal/format/imagelabel/imagelabel.go",
        "return Fits(width, chars) && height >= textHeight(width, chars)",
        "return Fits(width, chars) && height > textHeight(width, chars)",
        "TestALabelIsReadableFromTheFirstHeightThatHoldsItsText",
    ),
]
