package guard

import (
	"reflect"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Two requests that can be planned differently never share a key.
//
// format.RequestKey keys what was worked out for a request - the smallest size
// a format takes, whether a file of a set was already planned. Two requests
// under one key share one answer, so a field of Request left out of the key is
// a preset laid out on another request's floor, and a manifest carrying a hash
// of bytes that should not have been. Nothing would say so: the answer is a
// number, and a wrong number looks like a right one.
//
// Asked of every field of Request by reflection rather than of a list written
// here, so the day Request grows a field, this goes red until the key carries
// it. A list copied by hand is the kind that goes stale green.
func TestEveryFieldOfARequestIsInItsKey(t *testing.T) {
	base, ok := format.RequestKey("png", format.Request{})
	if !ok {
		t.Fatal("an empty request has no key, so nothing below compares against anything")
	}
	typ := reflect.TypeOf(format.Request{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		var r format.Request
		v := reflect.ValueOf(&r).Elem().Field(i)
		switch field.Type.Kind() {
		case reflect.Int64:
			v.SetInt(1)
		case reflect.Uint64:
			v.SetUint(1)
		case reflect.Bool:
			v.SetBool(true)
		case reflect.Map:
			v.Set(reflect.ValueOf(map[string]string{"delimiter": ";"}))
		case reflect.Slice:
			v.Set(reflect.MakeSlice(field.Type, 1, 1))
			// A request with contents is worked out every time rather than
			// keyed, and that has to stay true rather than become a key that
			// ignores them.
			if _, keyed := format.RequestKey("png", r); keyed {
				t.Errorf("a request with %s set has a key, and the key cannot tell what is inside one archive from another", field.Name)
			}
			continue
		default:
			t.Fatalf("Request.%s is a %s, which this guard does not know how to set - teach it, and the key, before anything is keyed by it",
				field.Name, field.Type.Kind())
		}
		got, keyed := format.RequestKey("png", r)
		if !keyed {
			t.Errorf("a request with only %s set has no key", field.Name)
			continue
		}
		if got == base {
			t.Errorf("a request with %s set has the key of an empty one, so the two would share an answer", field.Name)
		}
	}

	// The format as well, which is not a field.
	if other, _ := format.RequestKey("jpg", format.Request{}); other == base {
		t.Error("two formats asked the same thing share a key")
	}

	// And a value holding what the key separates settings with - a space and
	// an equals sign. A value is text somebody typed, so it can hold anything,
	// and quoting is what keeps one setting from reading as two.
	two, _ := format.RequestKey("csv", format.Request{Properties: map[string]string{"a": "x", "b": "y"}})
	one, _ := format.RequestKey("csv", format.Request{Properties: map[string]string{"a": "x b=y"}})
	if two == one {
		t.Errorf("one setting whose value holds the separators has the key of two settings: %s", one)
	}
}
