package core

import "errors"

// Refusal is a refusal that is one sentence and nothing else - the shape every
// fmt.Errorf in the engine had before refusals were said as data.
//
// It answers the three questions the surfaces ask of any refusal: what it says
// (Error, for the command line), what it says in parts (Said, for a window that
// speaks another language) and which setting it is about (AboutSetting, for a
// window that marks a box). What it wraps stays reachable through Unwrap, so a
// cause that decides the exit code is still found by errors.As.
type Refusal struct {
	said    Said
	setting string
}

// Refuse is a refusal about the run as a whole.
func Refuse(s Said) error { return &Refusal{said: s} }

// RefuseAbout is a refusal about one setting, named by its recipe key. The
// layout may name the setting through SettingSlot, and each surface puts its
// own name there - see setting.go.
func RefuseAbout(setting string, s Said) error { return &Refusal{said: s, setting: setting} }

// Error is the sentence the command line prints.
func (r *Refusal) Error() string { return r.InTheWordsOf(r.setting) }

// InTheWordsOf is the sentence with the setting named the way one surface names
// it. The slots are filled in the layout before the values go in, so a value
// somebody typed as "{setting}" is printed as typed rather than read as a slot.
func (r *Refusal) InTheWordsOf(name string) string {
	if name == "" {
		name = r.setting
	}
	return r.said.InTheWordsOf(name)
}

// Said is the sentence in parts.
func (r *Refusal) Said() Said { return r.said }

// AboutSetting is the recipe key this refusal is about, or empty.
func (r *Refusal) AboutSetting() string { return r.setting }

// Unwrap is every error the layout wraps with %w.
func (r *Refusal) Unwrap() []error { return r.said.wrapped() }

// InTheWordsOf is String with the setting slots of the layout filled first, so
// a value somebody typed as "{setting}" is printed as typed rather than read as
// a slot.
func (s Said) InTheWordsOf(name string) string {
	if name == "" {
		return s.String()
	}
	filled := s
	filled.layout = InTheWordsOf(s.layout, name)
	if s.one != "" {
		filled.one = InTheWordsOf(s.one, name)
	}
	return filled.String()
}

// wrapped is every value the layout takes with %w.
func (s Said) wrapped() []error {
	var out []error
	for i, d := range directivesOf(s.Layout()) {
		if i >= len(s.args) || d[len(d)-1] != 'w' {
			continue
		}
		if err, ok := s.args[i].Value.(error); ok {
			out = append(out, err)
		}
	}
	return out
}

// SaidOf is the sentence an error says: its own, when it is one of ours and
// the whole of what it says, and otherwise the error itself as the one value
// of a sentence - which a window words as an error of the system, of a library
// or of the program. Either way the English is the error's text to the byte.
func SaidOf(err error) Said {
	var said interface{ Said() Said }
	if errors.As(err, &said) && said.Said().String() == err.Error() {
		return said.Said()
	}
	return Says("core.Error", "%v", A("Error", err))
}

// DefectError is an error that says the program is wrong rather than the
// request: a plan this generator did not make, an encoder breaking its own
// contract. Nobody can correct it from a form, so a window says that in its
// own words and shows this text as it came, for whoever reports it - the
// owner's decision of 2026-10-05.
type DefectError struct {
	Err error
}

// Defect marks an error as one only a fault in the program can produce.
func Defect(err error) error {
	if err == nil {
		return nil
	}
	return &DefectError{Err: err}
}

func (d *DefectError) Error() string { return d.Err.Error() }
func (d *DefectError) Unwrap() error { return d.Err }

// IsDefect is whether an error, anywhere in its chain, is a defect.
func IsDefect(err error) bool {
	var d *DefectError
	return errors.As(err, &d)
}
