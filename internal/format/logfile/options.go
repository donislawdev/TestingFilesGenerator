// The settings, and the combinations this format refuses.
//
// Most of these only mean something for some shapes: a request method has no
// place in a syslog line, and a rate has none while the clock is held still.
// Accepting them there and quietly doing nothing is the failure rule 6 exists
// against - the recipe would say one thing, the file would be another, and
// nothing would say so. They are refused instead, by a message naming both
// settings, which is how a pair of settings that disagree is already reported
// when contains sits beside an archive's own properties.
package logfile

import (
	"fmt"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	defaultShape = "apache-combined"

	minRate = 1
	maxRate = 1_000_000
	// One a second, so the clock moves on EVERY line rather than every
	// tenth. The defect this default exists to fix was noticed by looking
	// at a file, so a default that only advances after ten entries would
	// leave a small log looking exactly as wrong as it did before.
	defaultRate = 1
)

// options is every setting for one target, already checked and turned into
// what the shapes actually use.
type options struct {
	shape *shape

	lineEnding string
	eol        string

	timestamps string
	advancing  bool
	rate       int

	methodMix string
	methods   []string

	statusMix string
	statuses  []int

	levelMix string
	levels   []string

	ipVersion string
	ipv6      bool
	ipMixed   bool
}

func defaultOptions() options {
	o := options{
		shape:      shapes[defaultShape],
		lineEnding: "lf",
		eol:        "\n",
		timestamps: "advancing",
		advancing:  true,
		rate:       defaultRate,
		methodMix:  "get",
		methods:    methodSets["get"],
		statusMix:  "realistic",
		statuses:   statusSets["realistic"],
		levelMix:   "realistic",
		levels:     levelSets["realistic"],
		ipVersion:  "v4",
	}
	return o
}

var methodSets = map[string][]string{
	"get":   {"GET"},
	"read":  {"GET", "HEAD"},
	"mixed": {"GET", "GET", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
}

var statusSets = map[string][]int{
	// realistic is the mix this format has always written: mostly success,
	// with a tail of the codes a real service actually returns.
	"realistic":     statuses,
	"success":       {200, 200, 201, 204, 206},
	"client-errors": {400, 401, 403, 404, 409, 410, 422, 429},
	"server-errors": {500, 500, 502, 503, 504},
}

// parseOptions reads the settings and refuses the pairs that disagree.
//
// One reader per setting, run in order, because the order matters: the shape
// and the clock have to be settled before anything can be asked whether it
// disagrees with them. It was one function until the code shape gates called
// it at 32 decision points against a ceiling of 22 - and those ceilings only
// ever go down, so the branches came out rather than the number going up.
func parseOptions(props map[string]string) (options, error) {
	o := defaultOptions()
	for _, read := range []func(map[string]string, *options) error{
		readShape, readLineEnding, readTimestamps, readRate,
		readMethods, readStatusMix, readLevelMix, readIPVersion,
	} {
		if err := read(props, &o); err != nil {
			return options{}, err
		}
	}
	return o, nil
}

func readShape(props map[string]string, o *options) error {
	v, ok := value(props, "entry_format")
	if !ok {
		return nil
	}
	s, known := shapes[v]
	if !known {
		return badValue("entry_format", v, "it is not one of the shapes this format writes")
	}
	o.shape = s
	return nil
}

func readLineEnding(props map[string]string, o *options) error {
	v, ok := value(props, "line_ending")
	if !ok {
		return nil
	}
	switch v {
	case "lf":
		o.eol = "\n"
	case "crlf":
		o.eol = "\r\n"
	default:
		return badValue("line_ending", v, "it has to be lf or crlf")
	}
	o.lineEnding = v
	return nil
}

func readTimestamps(props map[string]string, o *options) error {
	v, ok := value(props, "timestamps")
	if !ok {
		return nil
	}
	if v != "advancing" && v != "fixed" {
		return badValue("timestamps", v, "it has to be advancing or fixed")
	}
	o.timestamps = v
	o.advancing = v == "advancing"
	return nil
}

func readRate(props map[string]string, o *options) error {
	v, ok := value(props, "rate")
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("log: rate must be a whole number of entries per second, got %q", v)
	}
	if n < minRate || n > maxRate {
		return fmt.Errorf("log: rate must be between %d and %d entries per second, got %d", minRate, maxRate, n)
	}
	// A rate somebody CHOSE while the clock is held still would change nothing
	// at all, so it is said out loud rather than ignored. A rate sitting at its
	// default was not chosen - see asked.
	if _, chosen := asked(props, "rate", strconv.Itoa(defaultRate)); chosen && !o.advancing {
		return conflict("rate and timestamps", v,
			"a rate says how fast entries arrive and timestamps=fixed puts them all at one instant")
	}
	o.rate = n
	return nil
}

func readMethods(props map[string]string, o *options) error {
	v, ok := value(props, "methods")
	if !ok {
		return nil
	}
	set, known := methodSets[v]
	if !known {
		return badValue("methods", v, "it has to be get, read or mixed")
	}
	if err := refuseUnlessWeb(props, o.shape, "methods", "get", v, "request method"); err != nil {
		return err
	}
	o.methodMix, o.methods = v, set
	return nil
}

func readStatusMix(props map[string]string, o *options) error {
	v, ok := value(props, "status_mix")
	if !ok {
		return nil
	}
	set, known := statusSets[v]
	if !known {
		return badValue("status_mix", v, "it has to be realistic, success, client-errors or server-errors")
	}
	// JSON lines carry a status of their own, so this one applies there too - it
	// is the address and the method that have no place outside a web shape.
	if _, chosen := asked(props, "status_mix", "realistic"); chosen &&
		!o.shape.web && o.shape.id != "json-lines" {
		return conflict("status_mix and entry_format", v,
			"the "+o.shape.id+" shape carries no response code")
	}
	o.statusMix, o.statuses = v, set
	return nil
}

// readLevelMix reads the severity mix, which only two shapes carry.
//
// Unlike status_mix, the set chosen here can move the MINIMUM: the shortest
// entry a shape can write has to leave room for the longest level it might
// draw, and quiet draws only INFO. That is why the set is settled here and
// read back out of options by longestLevel, rather than either of them
// reaching for the vocabulary directly.
func readLevelMix(props map[string]string, o *options) error {
	v, ok := value(props, "level_mix")
	if !ok {
		return nil
	}
	set, known := levelSets[v]
	if !known {
		return badValue("level_mix", v, "it has to be realistic, quiet, errors or debug")
	}
	// Only a real choice can disagree with the shape - see asked. A window
	// sends this key on every run, so refusing whenever it arrived would put
	// the four shapes without a level out of reach from the window entirely,
	// which is the defect reported from a screenshot on 2026-08-31.
	if _, chosen := asked(props, "level_mix", "realistic"); chosen && !o.shape.levelled {
		return conflict("level_mix and entry_format", v,
			"the "+o.shape.id+" shape carries no severity")
	}
	o.levelMix, o.levels = v, set
	return nil
}

func readIPVersion(props map[string]string, o *options) error {
	v, ok := value(props, "ip_version")
	if !ok {
		return nil
	}
	switch v {
	case "v4":
	case "v6":
		o.ipv6 = true
	case "mixed":
		o.ipMixed = true
	default:
		return badValue("ip_version", v, "it has to be v4, v6 or mixed")
	}
	if err := refuseUnlessWeb(props, o.shape, "ip_version", "v4", v, "client address"); err != nil {
		return err
	}
	o.ipVersion = v
	return nil
}

// refuseUnlessWeb refuses a CHOSEN setting that only means something for a
// shape carrying a request. A value left at its default was not chosen and so
// cannot disagree with anything - see asked.
func refuseUnlessWeb(props map[string]string, s *shape, key, def, val, what string) error {
	if _, chosen := asked(props, key, def); !chosen {
		return nil
	}
	return needsWeb(s, key, val, what)
}

// needsWeb refuses a setting that only means something for a shape carrying a
// request.
func needsWeb(s *shape, key, val, what string) error {
	if s.web {
		return nil
	}
	return conflict(key+" and entry_format", val, "the "+s.id+" shape carries no "+what)
}

func value(props map[string]string, key string) (string, bool) {
	v, ok := props[key]
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// asked says whether somebody actually asked for this, which is not the same
// question as whether the key arrived.
//
// A window sends every setting it draws, and a menu always carries a value
// because it opens on its declared default - measured and written down in
// internal/gui/parts/property.go on 2026-08-27, and safe there because for a
// format setting a default present and a default absent mean the same thing.
//
// The first version of this file broke that. It refused a setting that could
// do nothing for the chosen shape whenever the KEY was present, so the window,
// which always sends methods, could not reach syslog or JSON lines at all -
// every attempt came back refused for a setting the person had never touched.
// The command line never showed it, because there an unset flag is an absent
// key. Reported from a screenshot on 2026-08-31.
//
// So a value equal to the default was not stated, and only a real choice can
// disagree with the shape.
func asked(props map[string]string, key, def string) (string, bool) {
	v, ok := value(props, key)
	if !ok || v == def {
		return v, false
	}
	return v, true
}

func badValue(key, val, why string) error {
	return &format.PropertyValueError{Format: "log", Key: key, Value: val, Reason: why}
}

// conflict names both settings, because naming one of a pair leaves the reader
// to guess which of the two to change.
func conflict(keys, val, why string) error {
	return &format.PropertyValueError{
		Format: "log", Key: keys, Value: val,
		Reason: why + ". Drop one of the two, or change the other",
	}
}
