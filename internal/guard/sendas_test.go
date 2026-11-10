package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The name a file is sent under is words in a manifest and nothing else.
//
// send-as exists for the names no file system stores: one that climbs out of a
// directory, an absolute path, a name Windows refuses. The file keeps an
// ordinary name on disk, the manifest carries the other one, and a test hands
// the file to the system it is testing under it.
//
// So the value is, on purpose, exactly what the name check beside it refuses -
// and the whole safety of the setting is that nothing here ever treats it as a
// place. These guards hold that, each from the outside: by what is on the disk
// afterwards, never by reading which function was called.

// sentFile is the part of a manifest entry these guards read. Decoded from the
// JSON a consumer receives rather than into the manifest package's own type,
// so a field renamed there fails here instead of following along.
type sentFile struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	SendAs  *string `json:"send_as"`
	Group   string  `json:"group"`
	Purpose string  `json:"purpose"`
	Hashes  struct {
		SHA256 string `json:"sha256"`
	} `json:"hashes"`
	Expected struct {
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	} `json:"expected"`
}

// sentUnderAnotherName is the entries of a run that carry a name to send
// them under, as on-disk name to that name.
func sentUnderAnotherName(t *testing.T, dir string) map[string]sentFile {
	t.Helper()
	out := map[string]sentFile{}
	for _, f := range sentFiles(t, dir) {
		if f.SendAs != nil {
			out[f.Name] = f
		}
	}
	return out
}

func sentFiles(t *testing.T, dir string) []sentFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("the run left no manifest: %v", err)
	}
	var m struct {
		Files []sentFile `json:"files"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("the manifest is not JSON: %v", err)
	}
	return m.Files
}

// A run whose send-as names a real file above the output directory, once as a
// climb and once as an absolute path, and that file is still there with its
// own contents after generate, after verify and after cleanup.
//
// The victim is a file that exists, so anything that resolved the value as a
// path would have something to overwrite or to remove. A value pointing at
// nothing would let all three commands pass while reading it as a place.
func TestANameAFileIsSentUnderNeverReachesTheDisk(t *testing.T) {
	outDir, victim := outsideVictim(t)
	root := filepath.Dir(outDir)
	src := "version: 1\ntargets:\n" +
		"  - id: climbing\n    format: txt\n    size: 1kb\n    send-as: ../VICTIM.txt\n" +
		"  - id: absolute\n    format: txt\n    size: 1kb\n    send-as: '" + victim + "'\n"
	rec := writeRecipe(t, root, src)

	if code, _, said := run(t, "generate", rec, "--out", outDir); code != cli.ExitOK {
		t.Fatalf("the run ended %d: %s", code, said)
	}

	// The state this guard is about, asserted rather than assumed: the
	// manifest really carries both values, separators and all.
	files := sentFiles(t, outDir)
	if len(files) != 2 {
		t.Fatalf("the manifest lists %d files, and the recipe has two targets", len(files))
	}
	for i, want := range []string{"../VICTIM.txt", victim} {
		got := files[i].SendAs
		if got == nil || *got != want {
			t.Fatalf("entry %d carries send_as %v, and the recipe asked for %q - without it this guard proves nothing", i+1, got, want)
		}
		if !strings.ContainsAny(*got, `/\`) {
			t.Fatalf("send_as %q holds no separator, so it could not have left the directory anyway", *got)
		}
		if files[i].Name != files[i].Path || strings.ContainsAny(files[i].Name, `/\`) {
			t.Errorf("entry %d is on disk as path %q and name %q - the name on disk has to stay an ordinary one", i+1, files[i].Path, files[i].Name)
		}
	}

	want := []string{"absolute_0001.txt", "climbing_0001.txt", "manifest.json"}
	if got := namesIn(t, outDir); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the output directory holds %v, expected %v", got, want)
	}
	if got := namesIn(t, root); strings.Join(got, " ") != "VICTIM.txt out recipe.yaml" {
		t.Errorf("the directory above the output holds %v - the run wrote something outside the directory it was given", got)
	}
	victimSurvives(t, victim)

	manifestPath := filepath.Join(outDir, "manifest.json")
	if code, _, said := run(t, "verify", manifestPath); code != cli.ExitOK {
		t.Errorf("verify ended %d on the run's own output: %s", code, said)
	}
	if code, _, said := run(t, "cleanup", manifestPath, "--yes"); code != cli.ExitOK {
		t.Errorf("cleanup ended %d: %s", code, said)
	}
	victimSurvives(t, victim)
	for _, name := range namesIn(t, outDir) {
		if strings.HasSuffix(name, ".txt") {
			t.Errorf("cleanup left %s behind - it removes the files the manifest lists by their path", name)
		}
	}
}

// One target written twice, with a name to send it under and without, and the
// two files are the same bytes. A manifest that was never asked for the name
// does not hold the key at all.
//
// The second half is what keeps every manifest written before the setting
// existed byte for byte what it was. An empty string under the key would be a
// new line in all of them.
func TestTheBytesOfAFileDoNotDependOnTheNameItIsSentUnder(t *testing.T) {
	dir := t.TempDir()
	with, without := filepath.Join(dir, "with"), filepath.Join(dir, "without")

	common := []string{"generate", "--format", "pdf", "--size", "8kb", "--id", "invoice", "--seed", "7"}
	if code, _, said := run(t, append(common, "--out", with, "--send-as", `..\..\invoice.pdf`)...); code != cli.ExitOK {
		t.Fatalf("the run with a name ended %d: %s", code, said)
	}
	if code, _, said := run(t, append(common, "--out", without)...); code != cli.ExitOK {
		t.Fatalf("the run without one ended %d: %s", code, said)
	}

	a, b := sentFiles(t, with), sentFiles(t, without)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("the runs listed %d and %d files, expected one each", len(a), len(b))
	}
	if a[0].SendAs == nil || *a[0].SendAs != `..\..\invoice.pdf` {
		t.Fatalf("the run given a name carries send_as %v, so the two runs below are the same run", a[0].SendAs)
	}
	if a[0].Hashes.SHA256 != b[0].Hashes.SHA256 || a[0].Hashes.SHA256 == "" {
		t.Errorf("the file is %s with a name to send it under and %s without - the name takes no part in the bytes",
			a[0].Hashes.SHA256, b[0].Hashes.SHA256)
	}
	if a[0].Name != b[0].Name {
		t.Errorf("the file is called %q with a name to send it under and %q without", a[0].Name, b[0].Name)
	}

	raw, err := os.ReadFile(filepath.Join(without, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "send_as") {
		t.Error("a manifest of a run that named nothing holds the key send_as - absent means absent, so that manifests written before it stay what they were")
	}
}

// What a recipe writes under send-as is what the manifest carries, character
// for character: a backslash is a backslash, an escape inside double quotes is
// the character it stands for, and a placeholder of the name template is plain
// text here.
func TestTheNameAFileIsSentUnderIsTakenExactlyAsWritten(t *testing.T) {
	cases := []struct {
		id      string
		written string
		want    string
	}{
		{"bare_backslash", `..\climbed_out.txt`, `..\climbed_out.txt`},
		{"single_quoted_drive", `'C:\Windows\Temp\climbed_out.txt'`, `C:\Windows\Temp\climbed_out.txt`},
		{"escapes", `"a.php\0.jpg\r\nX-Injected: 1"`, "a.php\x00.jpg\r\nX-Injected: 1"},
		{"template_text", `"{{7*7}} ${7*7} {index:04}.txt"`, "{{7*7}} ${7*7} {index:04}.txt"},
		{"windows_refuses", `"con<>|?*: . "`, `con<>|?*: . `},
		{"only_dots", `..`, `..`},
		{"longest", strings.Repeat("a", core.MaxSendAsBytes), strings.Repeat("a", core.MaxSendAsBytes)},
	}

	dir := t.TempDir()
	var src strings.Builder
	src.WriteString("version: 1\ntargets:\n")
	for _, c := range cases {
		src.WriteString("  - id: " + c.id + "\n    format: txt\n    size: 1kb\n    count: 2\n    send-as: " + c.written + "\n")
	}
	out := filepath.Join(dir, "out")
	if code, _, said := run(t, "generate", writeRecipe(t, dir, src.String()), "--out", out); code != cli.ExitOK {
		t.Fatalf("the run ended %d: %s", code, said)
	}

	files := sentFiles(t, out)
	if len(files) != 2*len(cases) {
		t.Fatalf("the manifest lists %d files, expected two for each of %d targets", len(files), len(cases))
	}
	for i, c := range cases {
		// Both files of a target: the value is not a template, so the second
		// carries what the first does.
		for _, f := range files[2*i : 2*i+2] {
			if f.SendAs == nil || *f.SendAs != c.want {
				t.Errorf("%s: %s carries send_as %v, expected %q", c.id, f.Name, f.SendAs, c.want)
			}
			if !strings.HasPrefix(f.Name, c.id+"_000") {
				t.Errorf("%s: the file on disk is %q - the name on disk comes from the target, not from the name it is sent under", c.id, f.Name)
			}
		}
	}
}

// A value the window composed into a recipe reads back as itself, with every
// character the setting allows and a recipe refuses anywhere else.
func TestARecipeComposedWithANameToSendUnderReadsBackTheSame(t *testing.T) {
	for _, value := range []string{
		"../climbed_out.jpg",
		`..\climbed_out.jpg`,
		"a.php\x00.jpg",
		"line\nbreak\ttab.txt",
		"photo\xe2\x80\xaegpj.exe",
		` "quoted" and # hashed: yes `,
	} {
		src, err := recipe.Compose(recipe.Document{Targets: []recipe.TargetDraft{{
			ID: "one", Format: "txt", Size: "1kb", SendAs: value,
		}}})
		if err != nil {
			t.Errorf("composing with send-as %q was refused: %v", value, err)
			continue
		}
		back, err := recipe.Parse(src, "composed.yaml")
		if err != nil {
			t.Errorf("the recipe composed with send-as %q does not read back: %v\n%s", value, err, src)
			continue
		}
		if got := back.Targets[0].SendAs; got != value {
			t.Errorf("send-as %q came back as %q\n%s", value, got, src)
		}
	}
}

// The three values that are refused, each before a byte is written and each
// naming the setting: nothing at all, more than the limit, and text that is
// not UTF-8.
func TestANameToSendUnderThatCannotBeCarriedIsRefused(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	flags := func(value string) []string {
		return []string{"generate", "--format", "txt", "--size", "1kb", "--out", out, "--send-as", value}
	}
	// A directory for each recipe. The rows below are built before any of them
	// runs, so two recipes written to one path would both be the second.
	recipeWith := func(line string) []string {
		return []string{"generate", writeOneTargetRecipe(t, t.TempDir(), line), "--out", out}
	}

	for _, c := range []struct {
		why  string
		args []string
		says string
	}{
		{"an empty value in a recipe", recipeWith(`send-as: ""` + "\n"), "nothing in it"},
		{"a list in a recipe", recipeWith("send-as: [a, b]\n"), "written as a list or a block"},
		{"one byte past the limit", flags(strings.Repeat("a", core.MaxSendAsBytes+1)), "8193 bytes"},
		{"text that is not UTF-8", flags("report\xff.txt"), "not valid UTF-8"},
	} {
		code, _, said := run(t, c.args...)
		if code != cli.ExitRecipe {
			t.Errorf("%s: exit code %d, expected %d\n%s", c.why, code, cli.ExitRecipe, said)
		}
		if !strings.Contains(said, c.says) || !strings.Contains(said, "send-as") {
			t.Errorf("%s: the refusal does not say %q about send-as:\n%s", c.why, c.says, said)
		}
		if _, err := os.Stat(out); err == nil {
			t.Fatalf("%s: the output directory exists after a refusal - a refused run writes nothing", c.why)
		}
	}

	// The limit itself is taken. Without this the row above passes for a limit
	// set anywhere below it.
	if code, _, said := run(t, flags(strings.Repeat("a", core.MaxSendAsBytes))...); code != cli.ExitOK {
		t.Errorf("a value of exactly %d bytes ended %d: %s", core.MaxSendAsBytes, code, said)
	}
}

// Somebody who writes the climbing name where the name on disk goes is told
// about the other setting, and what they are told works.
//
// That refusal is the first place a person meets the rule that a name is not a
// path. Left saying only "choose the directory", it sends away exactly the
// person who wanted the path in the name on purpose.
func TestARefusedPathInANameSaysWhereSuchANameGoes(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	base := []string{"generate", "--format", "txt", "--size", "1kb", "--out", out}

	code, _, said := run(t, append(base, "--name", "../climbed_out.txt")...)
	if code != cli.ExitRecipe {
		t.Fatalf("a path in --name ended %d, expected the refusal %d: %s", code, cli.ExitRecipe, said)
	}
	if !strings.Contains(said, "send-as") {
		t.Errorf("the refusal of a path in a name does not mention send-as:\n%s", said)
	}

	if code, _, said := run(t, append(base, "--name", "climbed_out.txt", "--send-as", "../climbed_out.txt")...); code != cli.ExitOK {
		t.Fatalf("doing what the refusal says ended %d: %s", code, said)
	}
	files := sentFiles(t, out)
	if len(files) != 1 || files[0].Name != "climbed_out.txt" || files[0].SendAs == nil || *files[0].SendAs != "../climbed_out.txt" {
		t.Errorf("following the refusal gave %+v, expected climbed_out.txt sent as ../climbed_out.txt", files)
	}

	// The manifest is refused for the same reason and is never uploaded
	// anywhere, so its refusal names the directory and stops there.
	recipeDir := t.TempDir()
	src := "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n" +
		"output:\n  dir: ./out\n  manifest: sub/run.json\n"
	code, _, said = run(t, "generate", writeRecipe(t, recipeDir, src))
	if code != cli.ExitRecipe || !strings.Contains(said, "which is a path rather than a file name") {
		t.Fatalf("a path in output.manifest ended %d, expected the refusal of a path with %d: %s", code, cli.ExitRecipe, said)
	}
	if !strings.Contains(said, "Choose the directory with the output setting") {
		t.Errorf("the refusal of a path in the manifest name does not say where a directory is chosen:\n%s", said)
	}
	if strings.Contains(said, "send-as") {
		t.Errorf("the refusal of a path in the manifest name points at send-as, which does nothing for a manifest:\n%s", said)
	}
}

// The instructions beside a manifest say under which name to hand a file over,
// for the file that has one and for no other.
//
// The file itself has an ordinary name, so without that line a person
// uploading it from the directory tests nothing of what it is there for.
func TestTheInstructionsSayUnderWhichNameToSendAFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	src := "version: 1\ntargets:\n" +
		"  - id: climbing\n    format: txt\n    size: 1kb\n    purpose: A name that climbs.\n    send-as: ..\\climbed_out.txt\n" +
		"  - id: several\n    format: txt\n    size: 1kb\n    count: 3\n    purpose: Three under one name.\n    send-as: /tmp/climbed_out.txt\n" +
		"  - id: unseen\n    format: txt\n    size: 1kb\n    purpose: A name cut short.\n    send-as: \"a.php\\0.jpg\"\n" +
		"  - id: spaced\n    format: txt\n    size: 1kb\n    purpose: A name that starts with a space.\n    send-as: \" leading.txt\"\n" +
		"  - id: ordinary\n    format: txt\n    size: 1kb\n    purpose: Nothing special.\n"
	if code, _, said := run(t, "generate", writeRecipe(t, dir, src), "--out", out); code != cli.ExitOK {
		t.Fatalf("the run ended %d: %s", code, said)
	}
	raw, err := os.ReadFile(filepath.Join(out, "manifest.instructions.md"))
	if err != nil {
		t.Fatalf("the run wrote no instructions: %v", err)
	}
	text := string(raw)

	for _, want := range []string{
		"Send it under the name `..\\climbed_out.txt`, not the name on disk.\n",
		"Send each of them under the name `/tmp/climbed_out.txt`, not the name on disk.\n",
		"Send it under the name `\"a.php\\x00.jpg\"`, not the name on disk. The name is given in double quotes with escapes",
		"Send it under the name `\" leading.txt\"`, not the name on disk. The name is given in double quotes with escapes",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the instructions do not hold the line %q:\n%s", want, text)
		}
	}
	if n := strings.Count(text, "under the name"); n != 4 {
		t.Errorf("%d entries say under which name to send a file, and four targets asked for one:\n%s", n, text)
	}
	// A name is quoted for a space at its edge as well as for a character
	// nobody can see, and every character of " leading.txt" can be seen. So
	// the sentence about the quotes gives the one reason true of both, and
	// says the quotes are not sent.
	if n := strings.Count(text, "The quotes around it are not part of the name."); n != 2 {
		t.Errorf("%d quoted names say that the quotes are not part of the name, expected both of them:\n%s", n, text)
	}
	if strings.Contains(text, "cannot be seen") {
		t.Errorf("the instructions say part of a name cannot be seen, which is false of a name quoted for a space at its edge:\n%s", text)
	}
	if strings.ContainsRune(text, 0) {
		t.Error("the instructions hold a raw null character - a name nobody can see is written as an escape")
	}
}

// A value shown to a person is itself when all of it can be seen, and quoted
// with escapes when it cannot or when a line of text would hide part of it.
func TestAValueShownExactlyIsItselfOrQuoted(t *testing.T) {
	for _, c := range []struct{ value, want string }{
		{`..\climbed_out.jpg`, `..\climbed_out.jpg`},
		{"../climbed_out.jpg", "../climbed_out.jpg"},
		{`C:\u202Egpj.exe`, `C:\u202Egpj.exe`},
		{"photo\xe2\x80\xaegpj.exe", `"photo\u202egpj.exe"`},
		{"a.php\x00.jpg", `"a.php\x00.jpg"`},
		{" leading", `" leading"`},
		{"trailing ", `"trailing "`},
		{`"quoted"`, `"\"quoted\""`},
		{"", `""`},
		{"report\xff.txt", `"report\xff.txt"`},
	} {
		if got := core.ShownExactly(c.value); got != c.want {
			t.Errorf("ShownExactly(%q) = %s, expected %s", c.value, got, c.want)
		}
	}
}

// The upload set holds four files sent under a name that leaves the upload
// directory, each a different way of leaving, and every one of them lands on
// the same harmless name.
//
// "Leaves" is judged by the rule this tool applies to a path in its own
// manifest rather than by a list written here, so the set and the tool cannot
// come to mean different things by it. The extension follows the first type
// the form accepts, because the path has to be the only reason to turn the
// file away.
func TestTheUploadSetSendsFourFilesUnderNamesThatLeaveTheDirectory(t *testing.T) {
	for _, c := range []struct{ allow, extension string }{
		{"jpg,png,pdf", ".jpg"},
		{"pdf", ".pdf"},
	} {
		dir := filepath.Join(t.TempDir(), "out")
		if code, _, said := run(t, "generate", "--preset", "upload-validation", "--limit", "64kb",
			"--bulk", "0", "--far-over", "off", "--allow", c.allow, "--out", dir); code != cli.ExitOK {
			t.Fatalf("--allow %s: the preset ended %d: %s", c.allow, code, said)
		}

		sent := sentUnderAnotherName(t, dir)
		if len(sent) != 4 {
			t.Fatalf("--allow %s: %d files carry a name to send them under, expected four: %v", c.allow, len(sent), sent)
		}
		ways := map[string]bool{}
		for name, f := range sent {
			problem := core.ContainmentProblem(*f.SendAs)
			if problem == "" {
				t.Errorf("--allow %s: %s is sent as %q, which stays inside a directory - it tests nothing about leaving one", c.allow, name, *f.SendAs)
			}
			ways[problem+"|"+separatorOf(*f.SendAs)] = true
			if !strings.HasSuffix(*f.SendAs, "tfg_climbed_out"+c.extension) {
				t.Errorf("--allow %s: %s is sent as %q, expected a path ending in tfg_climbed_out%s", c.allow, name, *f.SendAs, c.extension)
			}
			if strings.ContainsAny(name, "/"+`\`) || !strings.HasSuffix(name, c.extension) {
				t.Errorf("--allow %s: the file on disk is %q, expected an ordinary name ending in %s", c.allow, name, c.extension)
			}
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("--allow %s: %s is in the manifest and not on the disk: %v", c.allow, name, err)
			}
			if f.Expected.Outcome != "unspecified" || f.Expected.Reason != "filename_traversal" {
				t.Errorf("--allow %s: %s expects %s (%s), expected unspecified (filename_traversal) - both refusing and storing under a clean name are right",
					c.allow, name, f.Expected.Outcome, f.Expected.Reason)
			}
			// The name to look for afterwards is the one the file was sent
			// under, extension and all. Somebody searching for the bare word
			// as a whole name finds nothing and calls an open form closed.
			if !strings.Contains(f.Purpose, "no file called tfg_climbed_out"+c.extension) {
				t.Errorf("--allow %s: the purpose of %s does not name tfg_climbed_out%s as the file to look for:\n%s", c.allow, name, c.extension, f.Purpose)
			}
			if f.Group != "hostile-names" {
				t.Errorf("--allow %s: %s is in the group %q, expected hostile-names", c.allow, name, f.Group)
			}
		}
		if len(ways) != 4 {
			t.Errorf("--allow %s: the four names leave the directory in %d different ways, expected four: %v", c.allow, len(ways), ways)
		}
	}

	// The four stand last in the run, after the mass upload. A file's id is its
	// place in the manifest, so put anywhere else they would have moved the id
	// of every file after them, and an id is the handle a test written against
	// an earlier version of the set holds.
	dir := filepath.Join(t.TempDir(), "out")
	if code, _, said := run(t, "generate", "--preset", "upload-validation", "--limit", "64kb",
		"--bulk", "2", "--far-over", "off", "--out", dir); code != cli.ExitOK {
		t.Fatalf("the preset with a mass upload ended %d: %s", code, said)
	}
	files := sentFiles(t, dir)
	if len(files) < 6 || files[len(files)-5].Group != "bulk-upload" {
		t.Fatalf("the run of %d files does not hold a mass upload just before its last four, so this says nothing about what follows what", len(files))
	}
	for i, f := range files {
		if last := i >= len(files)-4; (f.SendAs != nil) != last {
			t.Errorf("%s stands at place %d of %d and send_as set is %t - the four names that leave the directory are the last four of the run",
				f.Name, i+1, len(files), f.SendAs != nil)
		}
	}

	_, shown, _ := run(t, "preset", "show", "upload-validation")
	if !strings.Contains(shown, "a name with a path in it") {
		t.Errorf("preset show does not say the set catches a name with a path in it:\n%s", shown)
	}
}

// separatorOf is the separator a path is written with, for telling a climb
// with one from a climb with the other.
func separatorOf(p string) string {
	if strings.Contains(p, `\`) {
		return "backslash"
	}
	return "slash"
}

// The preset screen writes the same four names the command line does.
//
// The window turns a recipe target into an engine target with a list of fields
// of its own, on the same layer as the command line's, so nothing shares them.
// A field left off that list is a setting the window drops without a word,
// which is how a set run from the screen would upload four ordinary names and
// test nothing.
func TestThePresetScreenKeepsTheNameAFileIsSentUnder(t *testing.T) {
	fromCLI, fromWindow := t.TempDir(), t.TempDir()
	if code, _, said := run(t, "generate", "--preset", "upload-validation", "--limit", "64kb", "--out", fromCLI); code != cli.ExitOK {
		t.Fatalf("the command line refused the preset: exit %d\n%s", code, said)
	}

	host, content := presetScreen(t)
	choosePreset(t, content, "upload-validation")
	fill(t, content, text.FieldOutputDir(), fromWindow)
	fill(t, content, text.SettingLabel("limit"), "64kb")
	press(t, content, "Generate")
	waitForManifest(t, host, fromWindow)
	join(host)

	want, got := sentUnderAnotherName(t, fromCLI), sentUnderAnotherName(t, fromWindow)
	if len(want) != 4 {
		t.Fatalf("the command line run carries %d names to send files under, expected four - without them this compares nothing", len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("the window run carries %d names to send files under and the command line run %d", len(got), len(want))
	}
	for name, f := range want {
		w, ok := got[name]
		if !ok || *w.SendAs != *f.SendAs {
			t.Errorf("%s is sent as %q from the command line and as %v from the window", name, *f.SendAs, w.SendAs)
		}
	}
}

// An entry of the manifest keeps its keys where they were: the name to send a
// file under comes straight after its name, and the two keys of a file that
// was not produced are the last two, present only for such a file.
//
// The entry gained send_as and went past what one type should hold, so the two
// keys about a failure moved into a type of their own, written inside the
// entry without a name. That is meant to change nothing in the document, and
// "meant to" is what this holds: a consumer reads these keys from the entry
// itself, and a manifest written before the move has them there.
func TestAnEntryOfTheManifestKeepsItsKeysWhereTheyWere(t *testing.T) {
	failed := manifest.File{ID: "f_0001", Path: "a.txt", Name: "a.txt", SendAs: "../a.txt",
		Failure: manifest.Failure{Failed: true, Error: "the disk said no"}}
	raw, err := json.Marshal(failed)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	if !strings.HasPrefix(doc, `{"id":"f_0001","path":"a.txt","name":"a.txt","send_as":"../a.txt","materialized":false,`) {
		t.Errorf("the entry does not begin with id, path, name, send_as and materialized in that order:\n%s", doc)
	}
	if !strings.HasSuffix(doc, `,"failed":true,"error":"the disk said no"}`) {
		t.Errorf("the entry does not end with the keys failed and error, in the entry itself:\n%s", doc)
	}

	var back manifest.File
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the entry does not read back: %v", err)
	}
	if !back.Failed || back.Error != "the disk said no" || back.SendAs != "../a.txt" {
		t.Errorf("the entry read back as failed %v, error %q and send_as %q", back.Failed, back.Error, back.SendAs)
	}

	whole, err := json.Marshal(manifest.File{ID: "f_0002", Path: "b.txt", Name: "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"failed"`, `"error"`, `"send_as"`} {
		if strings.Contains(string(whole), key) {
			t.Errorf("an entry of a file that was written under its own name holds the key %s:\n%s", key, whole)
		}
	}
}
