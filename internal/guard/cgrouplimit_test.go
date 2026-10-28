package guard

import (
	"io/fs"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The memory limit of a container is found in the files Linux keeps about it -
// docs/STABILITY-REVIEW-2026-10-26.md section 10.8.
//
// Every layout below is one a process really meets, handed to the reader as
// files, so the table runs on every system rather than only on the Linux
// runner - and only on the layouts somebody wrote down, which is why the real
// reading has a guard of its own beside this one. The shapes are the ones
// docs.kernel.org (cgroup-v2, read 2026-10-27) and proc_pid_mountinfo(5)
// describe: "0::$PATH" for version 2, "N:controllers:$PATH" for version 1, a
// mount root that is the group itself inside a namespace.
func TestTheLimitOfAContainerIsFoundInItsControlGroup(t *testing.T) {
	for _, c := range containerLayouts {
		read := func(name string) ([]byte, error) {
			if text, ok := c.files[name]; ok {
				return []byte(text), nil
			}
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		limit, limited, err := core.CgroupMemoryLimit(read)
		switch {
		case c.fails && err == nil:
			t.Errorf("%s: read as limit %d limited %v, expected a refusal - a number made up from a file that does not say one is a limit nobody set", c.name, limit, limited)
		case !c.fails && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case !c.fails && (limit != c.limit || limited != c.limited):
			t.Errorf("%s: limit %d limited %v, expected %d %v", c.name, limit, limited, c.limit, c.limited)
		}
	}
}

// Two lines of mountinfo most layouts below share: the one cgroup2 mount
// where systemd and every container runtime put it, and a filesystem that is
// not a control group at all, which the reader has to step over.
const (
	cgroupV2Mount    = "30 23 0:26 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime shared:4 - cgroup2 cgroup2 rw,nsdelegate\n"
	cgroupOtherMount = "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n"
)

// containerLayouts is every layout the guard above hands the reader, with the
// answer it has to give.
var containerLayouts = []struct {
	name    string
	files   map[string]string
	limit   int64
	limited bool
	fails   bool
}{
	{
		name: "a container with its own namespace, 512 MiB",
		files: map[string]string{
			"/proc/self/cgroup":         "0::/\n",
			"/proc/self/mountinfo":      cgroupOtherMount + cgroupV2Mount,
			"/sys/fs/cgroup/memory.max": "536870912\n",
		},
		limit: 512 << 20, limited: true,
	},
	{
		name: "a container with no limit",
		files: map[string]string{
			"/proc/self/cgroup":         "0::/\n",
			"/proc/self/mountinfo":      cgroupV2Mount,
			"/sys/fs/cgroup/memory.max": "max\n",
		},
	},
	{
		name: "a desktop session, max on its own group and a limit on the slice above it",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/user.slice/user-1000.slice/session-2.scope\n",
			"/proc/self/mountinfo": cgroupV2Mount,
			"/sys/fs/cgroup/user.slice/user-1000.slice/session-2.scope/memory.max": "max\n",
			"/sys/fs/cgroup/user.slice/user-1000.slice/memory.max":                 "4294967296\n",
			"/sys/fs/cgroup/user.slice/memory.max":                                 "max\n",
		},
		limit: 4 << 30, limited: true,
	},
	{
		name: "the smaller of two limits on the way up wins",
		files: map[string]string{
			"/proc/self/cgroup":             "0::/a/b\n",
			"/proc/self/mountinfo":          cgroupV2Mount,
			"/sys/fs/cgroup/a/b/memory.max": "8589934592\n",
			"/sys/fs/cgroup/a/memory.max":   "1073741824\n",
		},
		limit: 1 << 30, limited: true,
	},
	{
		name: "a group removed while the process is still in it",
		files: map[string]string{
			"/proc/self/cgroup":             "0::/job (deleted)\n",
			"/proc/self/mountinfo":          cgroupV2Mount,
			"/sys/fs/cgroup/job/memory.max": "268435456\n",
		},
		limit: 256 << 20, limited: true,
	},
	{
		name: "the root group, which has no memory.max at all",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/\n",
			"/proc/self/mountinfo": cgroupV2Mount,
		},
	},
	{
		name: "a mount point with a space in it, escaped the way Linux writes it",
		files: map[string]string{
			"/proc/self/cgroup":      "0::/\n",
			"/proc/self/mountinfo":   `31 23 0:27 / /mnt/cg\040two rw - cgroup2 cgroup2 rw` + "\n",
			"/mnt/cg two/memory.max": "1048576\n",
		},
		limit: 1 << 20, limited: true,
	},
	{
		name: "version 1, the memory controller beside others",
		files: map[string]string{
			"/proc/self/cgroup": "12:pids:/docker/abc\n4:memory:/docker/abc\n1:name=systemd:/docker/abc\n0::/system.slice/docker.service\n",
			"/proc/self/mountinfo": cgroupOtherMount +
				"40 30 0:35 /docker/abc /sys/fs/cgroup/memory ro,nosuid - cgroup cgroup rw,memory\n" +
				"41 30 0:36 /docker/abc /sys/fs/cgroup/pids ro,nosuid - cgroup cgroup rw,pids\n",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes": "2147483648\n",
		},
		limit: 2 << 30, limited: true,
	},
	{
		name: "version 1 with no limit, which it writes as an enormous number",
		files: map[string]string{
			"/proc/self/cgroup":                           "4:cpuset,memory:/\n",
			"/proc/self/mountinfo":                        "40 30 0:35 / /sys/fs/cgroup/memory rw - cgroup cgroup rw,cpuset,memory\n",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712\n",
		},
		limit: 9223372036854771712, limited: true,
	},
	{
		// A systemd host on the old layout: the session's own group has no
		// limit, the user's slice above it has. Version 1 accounts up the
		// hierarchy as version 2 does, so the slice holds the session.
		name: "version 1, no limit on its own group and a limit on the slice above it",
		files: map[string]string{
			"/proc/self/cgroup":    "9:memory:/user.slice/user-1000.slice/session-2.scope\n",
			"/proc/self/mountinfo": "40 30 0:35 / /sys/fs/cgroup/memory rw,nosuid - cgroup cgroup rw,memory\n",
			"/sys/fs/cgroup/memory/user.slice/user-1000.slice/session-2.scope/memory.limit_in_bytes": "9223372036854771712\n",
			"/sys/fs/cgroup/memory/user.slice/user-1000.slice/memory.limit_in_bytes":                 "4294967296\n",
			"/sys/fs/cgroup/memory/user.slice/memory.limit_in_bytes":                                 "9223372036854771712\n",
			"/sys/fs/cgroup/memory/memory.limit_in_bytes":                                            "9223372036854771712\n",
		},
		limit: 4 << 30, limited: true,
	},
	{
		// Linux writes no group with "." or ".." in it past the run that says
		// it is outside the namespace. Joined to the mount, this one walks out
		// of it to a file that holds a limit nobody set on this process.
		name: "a group that would walk out of its mount",
		files: map[string]string{
			"/proc/self/cgroup":         "0::/a/../../etc\n",
			"/proc/self/mountinfo":      cgroupV2Mount,
			"/sys/fs/etc/memory.max":    "1048576\n",
			"/sys/fs/cgroup/memory.max": "max\n",
		},
		fails: true,
	},
	{
		name: "a group with a step that goes nowhere",
		files: map[string]string{
			"/proc/self/cgroup":    "4:memory:/docker/./abc\n",
			"/proc/self/mountinfo": "40 30 0:35 / /sys/fs/cgroup/memory rw - cgroup cgroup rw,memory\n",
			"/sys/fs/cgroup/memory/docker/abc/memory.limit_in_bytes": "1048576\n",
		},
		fails: true,
	},
	{
		name: "a mount whose root does not reach the group",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/elsewhere/job\n",
			"/proc/self/mountinfo": "30 23 0:26 /sandbox /sys/fs/cgroup rw - cgroup2 cgroup2 rw\n",
		},
	},
	{
		name: "a root that only shares the start of the name",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/sandbox2/job\n",
			"/proc/self/mountinfo": "30 23 0:26 /sandbox /sys/fs/cgroup rw - cgroup2 cgroup2 rw\n",
		},
	},
	{
		name: "a group outside the namespace this process can see",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/../../init\n",
			"/proc/self/mountinfo": cgroupV2Mount,
		},
	},
	{
		name:  "no control groups at all",
		files: map[string]string{},
	},
	{
		name: "no cgroup2 mount to read it through",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/job\n",
			"/proc/self/mountinfo": cgroupOtherMount,
		},
	},
	{
		name: "a limit file that says something that is not a number",
		files: map[string]string{
			"/proc/self/cgroup":         "0::/\n",
			"/proc/self/mountinfo":      cgroupV2Mount,
			"/sys/fs/cgroup/memory.max": "lots\n",
		},
		fails: true,
	},
	{
		name: "a negative limit",
		files: map[string]string{
			"/proc/self/cgroup":         "0::/\n",
			"/proc/self/mountinfo":      cgroupV2Mount,
			"/sys/fs/cgroup/memory.max": "-1\n",
		},
		fails: true,
	},
	{
		name: "a line of /proc/self/cgroup with no path",
		files: map[string]string{
			"/proc/self/cgroup": "0::\n",
		},
		fails: true,
	},
	{
		name: "a mountinfo line with no separator",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/\n",
			"/proc/self/mountinfo": "30 23 0:26 / /sys/fs/cgroup rw cgroup2 cgroup2 rw\n",
		},
		fails: true,
	},
	{
		name: "an escape cut short at the end of the mount point",
		files: map[string]string{
			"/proc/self/cgroup":    "0::/\n",
			"/proc/self/mountinfo": `30 23 0:26 / /mnt/x\04 rw - cgroup2 cgroup2 rw` + "\n",
		},
		fails: true,
	},
}
