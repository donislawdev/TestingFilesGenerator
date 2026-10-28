package core

import (
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// How a process on Linux finds the memory limit of the control group it runs
// in. Written as a function of the files rather than of the system, so a guard
// on any system can hand it the files of a container and check the answer -
// machinememory_linux.go is the only caller that hands it the real ones.
//
// The shape follows the reader in the Go runtime that finds the processor
// limit (internal/runtime/cgroup in Go 1.27.3, read 2026-10-27), because the
// hard part is the same: /proc/self/cgroup says which group, mountinfo says
// where that group is mounted, and in a container with its own cgroup
// namespace the two have to be put together carefully.
//
// Both versions are read from the group up to the top of its mount, because a
// parent's limit holds every group under it in both - version 1 accounts up the
// hierarchy too (docs.kernel.org cgroup-v1/memory, section 6, read 2026-10-28).
//
// What it does not read, and why. memory.high is a point where the kernel
// starts to reclaim rather than a limit it ends the process at. A parent above
// the top of the mount - the slice a container with its own namespace sits in -
// is not read in either version, because no path this process can see reaches
// it. Version 1 would name it in memory.stat (hierarchical_memory_limit), and
// version 2 has no file that does, so reading it for the old layout alone would
// make the two answer differently for the same container.

// CgroupMemoryLimit is the memory limit of this process's control group, read
// through read from the paths Linux keeps the answer at. limited is false when
// there is no limit to find - no control group, no memory controller, "max" at
// every level - and then the memory of the machine is the answer.
//
// A file that is there and does not say what it should is an error rather than
// no limit. It is input from outside (untouchable rule 23), and a number made
// up from it would be a limit nobody set.
func CgroupMemoryLimit(read func(name string) ([]byte, error)) (limit int64, limited bool, err error) {
	self, err := read("/proc/self/cgroup")
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	group, v1, err := memoryGroup(string(self))
	if err != nil || group == "" {
		return 0, false, err
	}

	mounts, err := read("/proc/self/mountinfo")
	if err != nil {
		return 0, false, err
	}
	mount, rel, err := memoryMount(string(mounts), v1, group)
	if err != nil || mount == "" {
		return 0, false, err
	}

	if v1 {
		return limitAbove(read, mount, rel, "memory.limit_in_bytes")
	}
	return limitAbove(read, mount, rel, "memory.max")
}

// memoryGroup is the path of the group holding this process's memory, from
// /proc/self/cgroup, and whether it is a version 1 group.
//
// Each line is "hierarchy:controllers:path". Version 2 is hierarchy 0 with no
// controllers named. A version 1 line naming memory wins over it, because a
// controller can be mounted in one of the two and not both.
func memoryGroup(text string) (group string, v1 bool, err error) {
	for _, line := range lines(text) {
		hierarchy, controllers, where, err := groupLine(line)
		if err != nil {
			return "", false, err
		}
		switch {
		case hierarchy == "0":
			group = where
		case hasWord(controllers, "memory"):
			return where, true, nil
		}
	}
	return group, false, nil
}

// groupLine is one line of /proc/self/cgroup in its three parts.
func groupLine(line string) (hierarchy, controllers, where string, err error) {
	hierarchy, rest, ok1 := strings.Cut(line, ":")
	controllers, where, ok2 := strings.Cut(rest, ":")
	// A group that was removed while this process was still in it is shown
	// with this after its path. The limit is still the one that applies until
	// the process moves, so it is read rather than refused.
	where = strings.TrimSuffix(where, " (deleted)")
	if !ok1 || !ok2 || !strings.HasPrefix(where, "/") {
		return "", "", "", malformed("/proc/self/cgroup", line)
	}
	return hierarchy, controllers, where, nil
}

// memoryMount is where the group's hierarchy is mounted, from
// /proc/self/mountinfo, and where the group is under that mount.
//
// The root matters in a container. Its own namespace shows its group as "/",
// and its mount's root as "/" too, so the group is the mount. Without a
// namespace the group is a path under a mount whose root is the hierarchy's
// root. A group shown as starting with "/.." is outside the namespace this
// process can see, and no mount reaches it.
//
// Any other "." or ".." in the group is refused rather than followed. Linux
// writes the path clean and gives no group either name, so one that is there
// is a file that does not say what Linux writes - and joined to the mount it
// would walk out of it and read a limit from wherever it lands.
func memoryMount(text string, v1 bool, group string) (mount, rel string, err error) {
	for _, line := range lines(text) {
		m, err := mountLine(line)
		switch {
		case err != nil:
			return "", "", err
		case !m.holdsMemory(v1):
			continue
		}
		root, point, err := m.paths()
		if err != nil {
			return "", "", err
		}
		rel, reached := under(group, root)
		switch {
		case !reached || rel == "/.." || strings.HasPrefix(rel, "/../"):
			continue
		case rel != "" && path.Clean(rel) != rel:
			return "", "", malformed("/proc/self/cgroup", group)
		}
		return point, rel, nil
	}
	return "", "", nil
}

// mountEntry is the four fields of one line of mountinfo this reads: the root
// of the mount within its hierarchy and where it is mounted, both as Linux
// wrote them, the kind of filesystem and its own options.
type mountEntry struct {
	root, point, kind, options string
}

// mountLine splits one line of mountinfo. The optional fields in the middle
// are as many as the mount has, which is why the line is cut at the
// separator rather than counted.
func mountLine(line string) (mountEntry, error) {
	before, after, ok := strings.Cut(line, " - ")
	fields, tail := strings.Fields(before), strings.Fields(after)
	if !ok || len(fields) < 5 || len(tail) < 1 {
		return mountEntry{}, malformed("/proc/self/mountinfo", line)
	}
	m := mountEntry{root: fields[3], point: fields[4], kind: tail[0]}
	if len(tail) >= 3 {
		m.options = tail[2]
	}
	return m, nil
}

// holdsMemory says whether this mount is the hierarchy the memory controller
// lives in: the one version 2 hierarchy, or the version 1 one naming memory.
func (m mountEntry) holdsMemory(v1 bool) bool {
	if v1 {
		return m.kind == "cgroup" && hasWord(m.options, "memory")
	}
	return m.kind == "cgroup2"
}

// paths is the root and the mount point with the escapes undone. Asked only
// of the mount this reads through, so a line about some other filesystem is
// never refused for how its paths are written.
func (m mountEntry) paths() (root, point string, err error) {
	if root, err = unescapeMount(m.root); err == nil {
		point, err = unescapeMount(m.point)
	}
	return root, point, err
}

// lines is the lines of a file that have anything on them.
func lines(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool { return r == '\n' })
}

// under is where group is below root, when root reaches it at all. A root of
// "/" reaches every group, and any other reaches itself and what is under it -
// at a whole step of the path, so /sandbox does not reach /sandbox2.
func under(group, root string) (string, bool) {
	if root == "/" {
		return group, true
	}
	rest, ok := strings.CutPrefix(group, root)
	if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
		return "", false
	}
	return rest, true
}

// limitAbove is the smallest limit in file from the group up to the top of the
// mount - memory.max in version 2, memory.limit_in_bytes in version 1. A
// parent's limit holds every group under it, so a container whose own group
// says max can still be held by the slice it sits in.
func limitAbove(read func(string) ([]byte, error), mount, rel, file string) (int64, bool, error) {
	var least int64
	var found bool
	for dir := path.Join(mount, rel); ; dir = path.Dir(dir) {
		n, ok, err := limitIn(read, path.Join(dir, file))
		if err != nil {
			return 0, false, err
		}
		if ok && (!found || n < least) {
			least, found = n, true
		}
		if dir == path.Clean(mount) || dir == "/" || dir == "." {
			return least, found, nil
		}
	}
}

// limitIn reads one limit file. Missing means no limit at that level - the
// root of a hierarchy has no memory.max at all. "max" is version 2 for none,
// and a number is the limit in bytes.
func limitIn(read func(string) ([]byte, error), name string) (int64, bool, error) {
	b, err := read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	text := strings.TrimSpace(string(b))
	if text == "max" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n < 0 {
		return 0, false, malformed(name, text)
	}
	return n, true, nil
}

// unescapeMount undoes what Linux does to a path in mountinfo: a space, a tab,
// a new line and a backslash are written as a backslash and three octal
// digits.
func unescapeMount(s string) (string, error) {
	if !strings.Contains(s, `\`) {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		if i+4 > len(s) {
			return "", malformed("/proc/self/mountinfo", s)
		}
		v, err := strconv.ParseUint(s[i+1:i+4], 8, 8)
		if err != nil {
			return "", malformed("/proc/self/mountinfo", s)
		}
		b.WriteByte(byte(v))
		i += 3
	}
	return b.String(), nil
}

// hasWord says whether a comma separated list holds one word.
func hasWord(list, word string) bool {
	for _, w := range strings.Split(list, ",") {
		if w == word {
			return true
		}
	}
	return false
}

// malformed is the refusal for a file that does not say what it should. Said
// as a sentence because it reaches a person: it is the cause the note after a
// run gives for the memory it could not read.
func malformed(name, text string) error {
	return Refuse(Says("core.CgroupMalformed", "%s does not say what Linux writes there: %q", A("File", name), A("Text", text)))
}
