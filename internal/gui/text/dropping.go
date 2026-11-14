package text

// The words about a folder or a file dropped onto the window.
//
// A drop puts a path into a path box exactly as Choose... does, so most of
// what can be said about it is said by the box itself afterwards. What is here
// is what only a drop can get wrong: the wrong kind, more than one, something
// that is not there, and a screen that is busy. Each is said under the box it
// is about, which is where the person is looking.
//
// A screen whose work has started says nothing about a drop and takes none,
// the way a frozen box takes no typing. A sentence under the box would outlive
// the work it is about, because nothing clears a box when work ends.

// HintDropFolder and HintDropFile are what an empty path box says: that the
// path can be typed, and that the folder or the file can be dropped there.
// The only place on the screen that tells somebody dropping works at all.
func HintDropFolder() string { return say("HintDropFolder", "Drop a folder here, or type its path") }
func HintDropFile() string   { return say("HintDropFile", "Drop a file here, or type its path") }

// DropTakesFolder is a file dropped where a folder goes, and DropTakesFile the
// other way round. The box is named by the label above it and the dropped
// thing by its name, so the sentence says which of the two was the mistake.
func DropTakesFolder(box, name string) string {
	return sayf("DropTakesFolder", "{{.Box}} takes a folder. {{.Name}} is a file.", map[string]any{"Box": box, "Name": name})
}

func DropTakesFile(box, name string) string {
	return sayf("DropTakesFile", "{{.Box}} takes a file. {{.Name}} is a folder.", map[string]any{"Box": box, "Name": name})
}

// DropOneAtATime is several things dropped onto a box that holds one path.
// Nothing is taken: which of them came first is the order a file manager
// listed them in, not a choice anybody made.
func DropOneAtATime(count int) string {
	return sayN("DropOneAtATime", "Drop one at a time. {{.Count}} item was dropped.", "Drop one at a time. {{.Count}} items were dropped.", count, nil)
}

// DropNeither is something dropped that is there and is neither a file nor a
// folder - a device or a pipe on Linux and macOS.
func DropNeither(name string) string {
	return sayf("DropNeither", "{{.Name}} is neither a file nor a folder.", map[string]any{"Name": name})
}

// DropNotText is a dropped path whose name is not text the box can hold. A
// file system on Linux can hold such names, and a box that showed it would
// show a path that is not the one dropped.
func DropNotText() string {
	return say("DropNotText", "The name of what was dropped is not text this box can hold. Type the path instead.")
}

// DropAimless is a drop onto a screen with more than one box that takes it,
// landing on none of them.
func DropAimless() string { return say("DropAimless", "Drop it onto the box it is for.") }
