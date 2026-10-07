package format

import (
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What a container is, apart from the format it writes: how it names its
// contents, and the three ways asking a format for contents can be refused.
// Split out of format.go on 2026-10-07, when eml - the first container that is
// not an archive - pushed that file past the size the shape guard allows.

// Members is how a container names the settings for contents of one format.
type Members struct {
	Count, Format, Size string
}

// NotAContainerError is contains asked of a format that holds nothing.
type NotAContainerError struct {
	Format     string
	Containers []string
}

// What happened, what can do it instead, and what to do about it.
func (e *NotAContainerError) What() string { return e.what().String() }

func (e *NotAContainerError) what() core.Said {
	return core.Says("format.NotAContainer", "%s holds no other files, so it cannot take contains", core.A("Format", e.Format))
}

func (e *NotAContainerError) Why() string { return e.why().String() }

func (e *NotAContainerError) why() core.Said {
	return core.Says("format.NotAContainerWhy", "the formats that can are %s", core.A("Containers", strings.Join(e.Containers, ", ")))
}

func (e *NotAContainerError) Instead() string { return e.instead().String() }

func (e *NotAContainerError) instead() core.Said {
	return core.Says("format.NotAContainerFix", "Drop contains, or change the format")
}

// Parts is what happened, why and what to do instead, for a reader that lays
// them out apart and in its own language.
func (e *NotAContainerError) Parts() (what, why, instead core.Said) {
	return e.what(), e.why(), e.instead()
}

func (e *NotAContainerError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAContainerError) Said() core.Said {
	return core.Says("format.NotAContainerWhole", "%s - %s. %s", core.A("What", e.what()), core.A("Why", e.why()), core.A("Fix", e.instead()))
}

// ContentsConflictError is contains stated beside format properties saying the
// same thing. Picking one would build an archive holding something other than
// what the recipe says, and the recipe is what somebody reads in a review.
type ContentsConflictError struct {
	Format string
	Keys   []string
}

func (e *ContentsConflictError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *ContentsConflictError) Said() core.Said {
	if len(e.Keys) == 1 {
		return core.Says("format.ContentsConflictOne",
			"%s: contains and the %s property both say what the archive holds. Keep contains and drop the properties, or the other way round",
			core.A("Format", e.Format), core.A("Key", e.Keys[0]))
	}
	return core.Says("format.ContentsConflict",
		"%s: contains and the %s properties both say what the archive holds. Keep contains and drop the properties, or the other way round",
		core.A("Format", e.Format), core.A("Keys", strings.Join(e.Keys, ", ")))
}

// NestingUnsupportedError is a container asked to hold its own format.
//
// A legitimate test case that needs a depth limit before it is allowed, and
// there is none yet. It says that rather than pretending the format is unknown.
type NestingUnsupportedError struct {
	Format string
}

func (e *NestingUnsupportedError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *NestingUnsupportedError) Said() core.Said {
	return core.Says("format.NestingUnsupported",
		"%s cannot hold %s yet - an archive inside an archive needs a depth limit first. Hold a different format, or build the inner archive as its own target",
		core.A("Format", e.Format), core.A("Inner", e.Format))
}

// Containers lists the formats that accept contains, for a message that tells
// somebody what to write instead.
func Containers() []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for id, d := range registry {
		if d.Container {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
