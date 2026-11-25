package commandline

import "strings"

// Command is what a screen copies: one Line, or a Script that saves a file
// and then runs a line.
type Command interface {
	Write(p Program, s Shell) (Written, error)
}

// Script is a file the shell saves in the folder the terminal is in, and a
// line run after it.
//
// For what one line cannot say. The Several batches screen holds a recipe -
// several batches, the files inside an archive, a group and a purpose for
// each - and the command line takes a recipe as a file, with no flags for
// most of it. So the clipboard gets the recipe as text the shell saves, then
// the command that runs the file: two commands a person can read, and nothing
// new the command line has to promise. docs, the analysis of 2026-11-25,
// section 5, where reading the recipe from standard input was weighed and
// left, because Windows PowerShell 5.1 hands a program its input in ASCII.
type Script struct {
	file    string
	content string
	then    *Line
}

// FileThen is a script that saves content as file, then runs then.
func FileThen(file, content string, then *Line) *Script {
	return &Script{file: file, content: content, then: then}
}

// RecipeFile is the name a recipe is saved under, from its hash as
// recipe.Hash gives it - the name of the algorithm, a colon, the digest in
// hexadecimal. Twelve characters of the digest, so pasting the same commands
// twice writes the same bytes over the same file, and a different recipe gets
// a name of its own rather than replacing the one saved before it.
func RecipeFile(hash string) string {
	_, sum, found := strings.Cut(hash, ":")
	if !found {
		sum = hash
	}
	if len(sum) > 12 {
		sum = sum[:12]
	}
	return "tfg-recipe-" + sum + ".yaml"
}

// endOfFile closes the text in sh. Quoted where it opens, so nothing inside
// is expanded, and only a line that is this word and nothing else ends it.
const endOfFile = "TFG_EOF"

// Write is the script for one shell, its last line started the way this
// computer starts tfg.
//
// PowerShell takes the text as a here-string in single quotes, where nothing
// is read as code, and saves it with Set-Content -Encoding utf8: PowerShell 7
// writes no byte order mark and 5.1 writes one, which the recipe reader
// drops. Only a line that starts with '@ ends that text early - measured on
// 2026-11-25 on 5.1.26100 and 7.7.0, where an indented '@ stayed inside. sh
// takes it as a here-document. Every line is held to what any line here is
// held to, and one the shell would read as the end of the text is refused
// rather than written to save half a file.
func (sc *Script) Write(p Program, s Shell) (Written, error) {
	body := strings.TrimSuffix(sc.content, "\n")
	for _, line := range strings.Split(body, "\n") {
		if err := writable(line, ""); err != nil {
			return Written{}, err
		}
		if endsText(line, s) {
			return Written{}, &Unwritable{Reason: EndsText}
		}
	}
	run, err := sc.then.Write(p, s)
	if err != nil {
		return Written{}, err
	}
	file := quote(sc.file, s)
	save := "cat > " + file + " <<'" + endOfFile + "'\n" + body + "\n" + endOfFile
	if s == PowerShell {
		save = "@'\n" + body + "\n'@ | Set-Content -LiteralPath " + file + " -Encoding utf8"
	}
	run.Text = save + "\n" + run.Text
	run.File = sc.file
	return run, nil
}

// endsText reports whether the shell would read this line of the text as its
// end.
func endsText(line string, s Shell) bool {
	if s == PowerShell {
		return strings.HasPrefix(line, "'@")
	}
	return line == endOfFile
}
