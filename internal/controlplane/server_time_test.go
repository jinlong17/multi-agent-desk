package controlplane

import (
	"testing"
	"time"
)

func TestServerTimeFixedWidthCanonicalAndStrict(t *testing.T) {
	for _, test := range []struct {
		name  string
		input time.Time
		want  string
	}{
		{"whole", time.Date(2030, 3, 1, 2, 3, 4, 0, time.UTC), "2030-03-01T02:03:04.000000Z"},
		{"microsecond", time.Date(2030, 3, 1, 2, 3, 4, 1_000, time.UTC), "2030-03-01T02:03:04.000001Z"},
		{"truncate-nanoseconds", time.Date(2030, 3, 1, 2, 3, 4, 999_999_999, time.UTC), "2030-03-01T02:03:04.999999Z"},
		{"offset", time.Date(2030, 3, 1, 3, 3, 4, 100_000_000, time.FixedZone("plus-one", 3600)), "2030-03-01T02:03:04.100000Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := formatServerTime(test.input)
			if got != test.want || len(got) != 27 {
				t.Fatalf("format=%q len=%d want=%q", got, len(got), test.want)
			}
			parsed, err := parseServerTime(got)
			if err != nil || !parsed.Equal(normalizeServerTime(test.input)) {
				t.Fatalf("parsed=%v err=%v", parsed, err)
			}
		})
	}
	for _, invalid := range []string{
		"2030-03-01T02:03:04Z",
		"2030-03-01T02:03:04.1Z",
		"2030-03-01T02:03:04.00000Z",
		"2030-03-01T02:03:04.0000000Z",
		"2030-03-01T02:03:04.000000+00:00",
		"2030-03-01t02:03:04.000000Z",
	} {
		if _, err := parseServerTime(invalid); err == nil {
			t.Fatalf("accepted non-canonical stored time %q", invalid)
		}
	}
}
