// The shapes: the ways a log line can be written.
//
// Every one of these was taken from a real file on 2026-08-31 rather than from
// memory, which is the rule this project has for claims about the world outside
// the repository. A real nginx and a real Apache in containers, and rsyslog on
// this machine. The measurements and the samples are in
// docs/MVP-FORMATS.md section 5.1.
//
// Two of them corrected what would otherwise have been written from memory:
// nginx's default log_format ends with $http_x_forwarded_for, so a real nginx
// line carries one more quoted field than "combined" does, and the Apache
// image's default is common rather than combined - no referrer and no agent.
//
// Every shape has to answer the same two questions, because the exact size
// promise is built on them: what is the fewest bytes a line can be for ANY
// draw, and where does the difference go when a line has to hit a length. The
// answer to the second is one stretchable field per shape - a request path for
// the web shapes, a message for the rest.
package logfile

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// shape is one way a log line can be written.
//
// Function fields rather than six types implementing an interface, because
// what differs between them is a layout and a minimum, not behaviour.
type shape struct {
	id string
	// web says whether this shape carries a request, which is what decides
	// if the method, status and address settings mean anything for it. A
	// setting that would do nothing is refused rather than ignored.
	web bool
	// levelled says whether this shape carries a severity, which decides the
	// same thing for level_mix. Separate from web rather than derived from it,
	// because the two do not line up: no web shape carries a level, and of the
	// three that are not web, only two do.
	levelled bool
	// appendTo writes one entry. want below zero means whatever length it
	// comes out. Any other value is the exact length the line must have, its
	// terminator included, and the stretchable field reaches it.
	appendTo func(dst []byte, st *state, want int64) []byte
	// shortest is the fewest bytes appendTo can produce for ANY draw, which
	// has to hold for the unluckiest one or the closing entry cannot reach
	// the length it was asked for.
	shortest func(o options) int64
	// label wraps the self describing text as a line this shape's readers
	// accept. Every line has to be a whole record, so JSON lines cannot take
	// a hash comment and gets an object instead.
	label func(text string, o options) string
}

// state is everything one entry needs. Held across entries so the clock keeps
// its place and nothing is allocated per line.
type state struct {
	rng   *rand.Rand
	clock clock
	opt   options
}

var shapes = map[string]*shape{
	"apache-common":   {id: "apache-common", web: true, appendTo: appendApacheCommon, shortest: shortestApacheCommon, label: hashLabel},
	"apache-combined": {id: "apache-combined", web: true, appendTo: appendApacheCombined, shortest: shortestApacheCombined, label: hashLabel},
	"nginx":           {id: "nginx", web: true, appendTo: appendNginx, shortest: shortestNginx, label: hashLabel},
	"syslog":          {id: "syslog", web: false, appendTo: appendSyslog, shortest: shortestSyslog, label: hashLabel},
	"plain":           {id: "plain", web: false, levelled: true, appendTo: appendPlain, shortest: shortestPlain, label: hashLabel},
	"json-lines":      {id: "json-lines", web: false, levelled: true, appendTo: appendJSONLine, shortest: shortestJSONLine, label: jsonLabel},
}

// shapeIDs is the closed set the registry offers, in one order so that every
// surface lists them the same way.
var shapeIDs = []string{"apache-combined", "apache-common", "nginx", "syslog", "plain", "json-lines"}

// hashLabel is the label line for every shape whose readers treat a leading
// hash as a comment. It says in words that it is not an entry.
func hashLabel(text string, o options) string { return "# " + text + o.eol }

// jsonLabel is the label for JSON lines, where every line must be a whole
// object. A hash comment would be the one unparseable line in the file.
func jsonLabel(text string, o options) string {
	return `{"label":"` + text + `"}` + o.eol
}

// --- the web shapes -------------------------------------------------------
//
// A real Apache common line, captured 2026-08-31:
//
//	192.0.2.10 - - [31/Aug/2026:18:44:46 +0000] "GET /index.html HTTP/1.1" 200 191
//
// The address is the only part of that line not as captured. The real one was
// the container bridge address, and this repository's own guard against
// publishing private content refuses those - correctly, and it caught this one
// rather than a reader doing so. Swapped for the documentation range, which
// changes nothing about the shape being shown.
//
// combined adds the referrer and the agent, and nginx adds one more quoted
// field after those - $http_x_forwarded_for, which its default format carries
// and plain "combined" does not.

func appendApacheCommon(dst []byte, st *state, want int64) []byte {
	return appendWeb(dst, st, want, webTail{})
}

func appendApacheCombined(dst []byte, st *state, want int64) []byte {
	return appendWeb(dst, st, want, webTail{referrer: true, agent: true})
}

func appendNginx(dst []byte, st *state, want int64) []byte {
	return appendWeb(dst, st, want, webTail{referrer: true, agent: true, forwarded: true})
}

// webTail says which quoted fields follow the status and the byte count.
type webTail struct{ referrer, agent, forwarded bool }

// width is what the tail costs for an agent of a given length.
func (t webTail) width(agentLen int) int64 {
	var n int64
	if t.referrer {
		n += int64(len(` "-"`))
	}
	if t.agent {
		n += int64(len(` ""`) + agentLen)
	}
	if t.forwarded {
		n += int64(len(` "-"`))
	}
	return n
}

// longest is the tail at its most expensive, for the minimum.
func (t webTail) longest() int64 { return t.width(longestAgent()) }

func appendWeb(dst []byte, st *state, want int64, tail webTail) []byte {
	addr := drawAddress(st.rng, st.opt)
	// The order of these draws is the order the generator used before entry
	// formats existed, and it has to stay that way: timestamps=fixed has to
	// reproduce the old file exactly, and one extra draw would shift every
	// entry after it. pick spends nothing on a list of one.
	method := pick(st.rng, st.opt.methods)
	status := pick(st.rng, st.opt.statuses)
	size := 100000 + st.rng.IntN(899999)
	agent := pick(st.rng, agents)
	path := pick(st.rng, paths)
	at := st.clock.tick()

	// Everything but the path, as arithmetic rather than by building the line
	// and measuring it. Building it allocated once per entry, which over a
	// large log is the size of the file again in garbage.
	base := int64(len(" - - [")+len(apacheTime)+len(`] "`)+len(method)+len(" /")+
		len(` HTTP/1.1" `)+statusWidth+len(" ")+sizeWidth) +
		int64(addr.length()) + tail.width(len(agent)) + int64(len(st.opt.eol))

	dst = addr.append(dst)
	dst = append(dst, " - - ["...)
	dst = at.AppendFormat(dst, apacheTime)
	dst = append(dst, `] "`...)
	dst = append(dst, method...)
	dst = append(dst, " /"...)
	if want < 0 {
		dst = append(dst, path...)
	} else {
		dst = appendPath(dst, want-base)
	}
	dst = append(dst, ` HTTP/1.1" `...)
	dst = strconv.AppendInt(dst, int64(status), 10)
	dst = append(dst, ' ')
	dst = strconv.AppendInt(dst, int64(size), 10)
	if tail.referrer {
		dst = append(dst, ` "-"`...)
	}
	if tail.agent {
		dst = append(dst, ` "`...)
		dst = append(dst, agent...)
		dst = append(dst, '"')
	}
	if tail.forwarded {
		dst = append(dst, ` "-"`...)
	}
	return append(dst, st.opt.eol...)
}

// shortestWeb is the fewest bytes a web line can be, and every part of it is
// the WORST case rather than a typical one: the longest address the settings
// can draw, the longest method, the longest agent, and one character of path.
func shortestWeb(o options, tail webTail) int64 {
	return int64(len(" - - [")+len(apacheTime)+len(`] "`)+longestMethod(o)+len(" /")+
		len(` HTTP/1.1" `)+statusWidth+len(" ")+sizeWidth) +
		int64(longestAddress(o)) + tail.longest() + int64(len(o.eol)) + 1
}

func shortestApacheCommon(o options) int64 { return shortestWeb(o, webTail{}) }
func shortestApacheCombined(o options) int64 {
	return shortestWeb(o, webTail{referrer: true, agent: true})
}
func shortestNginx(o options) int64 {
	return shortestWeb(o, webTail{referrer: true, agent: true, forwarded: true})
}

// --- the message shapes ---------------------------------------------------
//
// A real rsyslog line from this machine, captured 2026-08-31:
//
//	2026-08-31T20:35:01.021105+02:00 Mainnn CRON[3693]: pam_unix(cron:session): session closed for user root
//
// No priority in angle brackets: that belongs on the wire, not in the file,
// and no line in any file on this machine carries one. Nor is there an
// RFC 3164 style "Aug 31 20:35:01" line anywhere here - modern rsyslog writes
// the ISO form, so that is the one a tester's own machine produces.

func appendSyslog(dst []byte, st *state, want int64) []byte {
	tag := pick(st.rng, tags)
	pid := minPid + st.rng.IntN(maxPid-minPid+1)
	at := st.clock.tick()

	// The pid's own width, not the widest one it could have been. It runs
	// from 100 to 9998, so it is three digits about one time in ten - and
	// assuming four made the closing entry a byte short exactly that often.
	// Measured after the fact: 35 files out of 360 missed their size, and
	// only this shape, because only the LAST entry is built to a length.
	base := int64(len(isoTime)+1+len(syslogHost)+1+len(tag)+1+pidWidth(pid)+len("]: ")) + int64(len(st.opt.eol))

	dst = at.AppendFormat(dst, isoTime)
	dst = append(dst, ' ')
	dst = append(dst, syslogHost...)
	dst = append(dst, ' ')
	dst = append(dst, tag...)
	dst = append(dst, '[')
	dst = strconv.AppendInt(dst, int64(pid), 10)
	dst = append(dst, "]: "...)
	return appendMessage(dst, st, want, base)
}

// The widest pid here, because the shortest line has to hold for the
// unluckiest draw rather than the lucky one.
func shortestSyslog(o options) int64 {
	return int64(len(isoTime)+1+len(syslogHost)+1+longestTag()+1+pidWidth(maxPid)+len("]: ")) +
		int64(len(o.eol)) + 1
}

// pidWidth is how many characters a process id takes. Written out rather
// than measured with strconv, because a log of any size is millions of
// entries and formatting one to count it allocates on every line.
func pidWidth(pid int) int {
	if pid < 1000 {
		return 3
	}
	return 4
}

// plain is an application log: an instant, a level, and a sentence. The shape
// most home grown loggers write and the one with the least agreement about it,
// so this is the plainest reading of it.
func appendPlain(dst []byte, st *state, want int64) []byte {
	level := pick(st.rng, st.opt.levels)
	at := st.clock.tick()

	base := int64(len(isoTime)+1+len(level)+1) + int64(len(st.opt.eol))

	dst = at.AppendFormat(dst, isoTime)
	dst = append(dst, ' ')
	dst = append(dst, level...)
	dst = append(dst, ' ')
	return appendMessage(dst, st, want, base)
}

func shortestPlain(o options) int64 {
	return int64(len(isoTime)+1+longestLevel(o)+1) + int64(len(o.eol)) + 1
}

// JSON lines: one object a line, which is what makes it the one shape here
// with a reader that is not a regular expression. The structural checker hands
// each line to Python's own json module.
//
// The message is the stretchable field and the filler is ASCII words, so
// nothing in it ever needs escaping - which matters, because an escape would
// make the line longer than the arithmetic said.
func appendJSONLine(dst []byte, st *state, want int64) []byte {
	level := pick(st.rng, st.opt.levels)
	status := pick(st.rng, st.opt.statuses)
	at := st.clock.tick()

	base := int64(len(`{"time":"","level":"","status":,"msg":""}`)+
		len(isoTime)+len(level)+statusWidth) + int64(len(st.opt.eol))

	dst = append(dst, `{"time":"`...)
	dst = at.AppendFormat(dst, isoTime)
	dst = append(dst, `","level":"`...)
	dst = append(dst, level...)
	dst = append(dst, `","status":`...)
	dst = strconv.AppendInt(dst, int64(status), 10)
	dst = append(dst, `,"msg":"`...)
	if want < 0 {
		dst = core.AppendFiller(dst, messageWords, int64(12+st.rng.IntN(40)), nil)
	} else {
		dst = core.AppendFiller(dst, messageWords, want-base, nil)
	}
	dst = append(dst, `"}`...)
	return append(dst, st.opt.eol...)
}

func shortestJSONLine(o options) int64 {
	return int64(len(`{"time":"","level":"","status":,"msg":""}`)+
		len(isoTime)+longestLevel(o)+statusWidth) + int64(len(o.eol)) + 1
}

// appendMessage writes the sentence the message shapes end with, stretched to
// reach a length when one was asked for.
func appendMessage(dst []byte, st *state, want int64, base int64) []byte {
	if want < 0 {
		dst = core.AppendFiller(dst, messageWords, int64(16+st.rng.IntN(48)), nil)
	} else {
		dst = core.AppendFiller(dst, messageWords, want-base, nil)
	}
	return append(dst, st.opt.eol...)
}

// appendPath writes a URL path of exactly n bytes out of readable segments, so
// a padded entry still looks like a request rather than a run of one letter.
func appendPath(dst []byte, n int64) []byte {
	if n < 1 {
		// Only reachable if the caller ignored the minimum. The check in
		// FillRecords turns that into an error rather than a wrong size.
		return append(dst, 'x')
	}
	return core.AppendFiller(dst, pathFiller, n, func(int) string { return "/" })
}

var pathFiller = []string{"segment"}

// messageWords is the vocabulary the message shapes pad with. ASCII on
// purpose: a character needing a JSON escape would be written as two bytes and
// the length arithmetic would be wrong by one.
var messageWords = []string{
	"request", "handled", "for", "user", "session", "cache", "miss", "queue",
	"retry", "upstream", "timeout", "connection", "closed", "by", "peer",
}
