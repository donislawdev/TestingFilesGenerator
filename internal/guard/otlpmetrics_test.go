package guard

import (
	"bytes"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/otlp"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// The structural check of a file of metrics adds the points up: a count
// against its buckets, a counter against the collection before, a date
// against the interval, the temporality against the setting, and the
// exemplars against the buckets the collection measured in. Each of those is
// broken here in a file that passes, and the check has to refuse it with the
// reason that names what was broken - refused for another reason would mean
// the check it was aimed at never ran.
//
// These are the canaries of that check. They run on every machine with
// python, where a mutation of it would run on a schedule.
func TestTheOTLPCheckRefusesMetricsThatDoNotAddUp(t *testing.T) {
	dir := t.TempDir()
	mixed := map[string]string{otlp.Signal: otlp.Metrics}
	named := map[string]string{otlp.Signal: otlp.Metrics, otlp.MetricKind: otlp.Histogram, otlp.Exemplars: "true"}
	unnamed := map[string]string{otlp.Signal: otlp.Metrics, otlp.MetricKind: otlp.Histogram}
	good := writeOTLP(t, filepath.Join(dir, "good.jsonl"), 120000, 9, mixed)
	withExemplars := writeOTLP(t, filepath.Join(dir, "named.jsonl"), 60000, 9, named)
	if bytes.Count(good, []byte("\n")) < 3 {
		t.Fatal("the file holds fewer than three collections, so no counter has a collection before it to be compared with")
	}

	write := func(name string, body []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// first changes the first match of pattern in the file, the group it
	// holds handed to change, and fails the guard when there is none.
	first := func(body []byte, pattern string, change func(string) string) []byte {
		re := regexp.MustCompile(pattern)
		at := re.FindSubmatchIndex(body)
		if at == nil {
			t.Fatalf("nothing in the file matches %s, so there is nothing to break", pattern)
		}
		out := append([]byte{}, body[:at[2]]...)
		out = append(out, change(string(body[at[2]:at[3]]))...)
		return append(out, body[at[3]:]...)
	}
	plus := func(n int64) func(string) string {
		return func(s string) string {
			v, _ := strconv.ParseInt(s, 10, 64)
			return strconv.FormatInt(v+n, 10)
		}
	}
	lines := bytes.SplitAfter(good, []byte("\n"))
	secondFalling := append(append([]byte{}, lines[0]...),
		first(lines[1], `"name":"jvm.class.loaded".*?"asInt":"(\d+)"`, func(string) string { return "1" })...)
	for _, l := range lines[2:] {
		secondFalling = append(secondFalling, l...)
	}

	cases := []struct {
		what, because string
		body          []byte
		settings      map[string]string
	}{
		{"a temporality other than the one asked", "states the temporality 1",
			first(good, `"aggregationTemporality":(2),"isMonotonic"`, func(string) string { return "1" }), mixed},
		{"an exponential histogram without its empty negative buckets", "at scale 3",
			bytes.Replace(good, []byte(`"negative":{},`), nil, 1), mixed},
		// A bucket rather than the count: a count one more is refused
		// earlier, by the sum lying outside the least and the most times it,
		// and this check would never be asked (measured 2026-11-11).
		{"a bucket holding one more than the count says", "and its count says",
			first(good, `"bucketCounts":\["(\d+)"`, plus(1)), mixed},
		{"a collection dated a second late", "is dated",
			first(good, `"timeUnixNano":"(\d+)"`, plus(1_000_000_000)), mixed},
		{"a counter below what it reached a collection before", "below the",
			secondFalling, mixed},
		{"exemplars where none were asked", "exemplars=false asks for none", withExemplars, unnamed},
	}
	reqs := []oracle.StrictRequest{
		{Format: "otlp", Path: filepath.Join(dir, "good.jsonl"), Settings: mixed},
		{Format: "otlp", Path: filepath.Join(dir, "named.jsonl"), Settings: named},
	}
	for i, c := range cases {
		reqs = append(reqs, oracle.StrictRequest{Format: "otlp", Path: write("broken"+strconv.Itoa(i)+".jsonl", c.body), Settings: c.settings})
	}
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural check needs python")
	}
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if got[i].Verdict != oracle.StrictOK {
			t.Fatalf("%s is itself refused, so no refusal below says anything: %s", filepath.Base(reqs[i].Path), got[i].Reason)
		}
	}
	for i, c := range cases {
		if j := got[i+2]; j.Verdict != oracle.StrictFail {
			t.Errorf("%s: the checker answered %s (%s), and it has to refuse", c.what, j.Verdict, j.Reason)
		} else if !strings.Contains(j.Reason, c.because) {
			t.Errorf("%s: refused, but for another reason - %q, where it has to say %q", c.what, j.Reason, c.because)
		}
	}
}

// The exponential buckets are written out as whole microseconds rather than
// worked out with a logarithm, so a duration falls in the same bucket on
// every machine. Each entry has to be the largest whole number of
// microseconds not above 2^(k/8) seconds, which is asked here in whole
// numbers: f^8 <= 2^k * 10^48 < (f+1)^8.
//
// The canary first: a table with one entry a microsecond high and one a
// microsecond low, which the same question has to refuse.
func TestTheExponentialBucketsEndWhereTheirBoundsSay(t *testing.T) {
	scale, first, tops := otlp.BucketTops()
	if scale != 3 {
		t.Fatalf("the table is read at scale %d and this guard asks about eight buckets a doubling, scale 3", scale)
	}
	for _, off := range []int64{1, -1} {
		bad := append([]int64{}, tops...)
		bad[len(bad)/2] += off
		if wrong := wrongBucketTops(first, bad); len(wrong) != 1 {
			t.Fatalf("a table with one entry moved by %d is refused at %d entries, not at the one moved", off, len(wrong))
		}
	}
	if len(tops) != -first+8 {
		t.Errorf("the table holds %d buckets from index %d, and a bucket ending at 2 s is index 7, so it has %d", len(tops), first, -first+8)
	}
	for _, i := range wrongBucketTops(first, tops) {
		t.Errorf("bucket %d ends at %d us, which is not the largest whole number of microseconds under 2^(%d/8) s", first+i, tops[i], first+i+1)
	}
}

// wrongBucketTops is the places in a table of bucket ends where an entry is
// not the floor of 10^6 * 2^(k/8).
func wrongBucketTops(first int, tops []int64) []int {
	var wrong []int
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(48), nil)
	for i, f := range tops {
		k := first + i + 1
		low := new(big.Int).Exp(big.NewInt(f), big.NewInt(8), nil)
		high := new(big.Int).Exp(big.NewInt(f+1), big.NewInt(8), nil)
		bound := new(big.Int).Set(limit)
		if k >= 0 {
			bound.Lsh(bound, uint(k))
		} else {
			low.Lsh(low, uint(-k))
			high.Lsh(high, uint(-k))
		}
		if low.Cmp(bound) > 0 || high.Cmp(bound) <= 0 {
			wrong = append(wrong, i)
		}
	}
	return wrong
}
