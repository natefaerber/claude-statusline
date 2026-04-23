package segments

import (
	"testing"
	"time"
)

func TestFormatPct(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.0%"},
		{5.3, "5.3%"},
		{9.999, "10.0%"},
		{10, "10%"},
		{12.4, "12%"},
		{12.5, "13%"},
		{99.4, "99%"},
		{100, "100%"},
	}
	for _, tc := range cases {
		if got := formatPct(tc.in); got != tc.want {
			t.Errorf("formatPct(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1_000, "1k"},
		{12_345, "12k"},
		{999_999, "999k"},
		{1_000_000, "1.0M"},
		{1_234_567, "1.2M"},
	}
	for _, tc := range cases {
		if got := formatTokens(tc.in); got != tc.want {
			t.Errorf("formatTokens(%d) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m"},
		{45 * time.Minute, "45m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h 30m"},
		{24 * time.Hour, "1d"},
		{25 * time.Hour, "1d 1h"},
		{72 * time.Hour, "3d"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseShortstat(t *testing.T) {
	cases := []struct {
		in                  string
		wantAdd, wantRemove int
	}{
		{" 5 files changed, 234 insertions(+), 45 deletions(-)", 234, 45},
		{" 1 file changed, 10 insertions(+)", 10, 0},
		{" 1 file changed, 10 deletions(-)", 0, 10},
		{"", 0, 0},
	}
	for _, tc := range cases {
		a, r := parseShortstat(tc.in)
		if a != tc.wantAdd || r != tc.wantRemove {
			t.Errorf("parseShortstat(%q) = (%d, %d); want (%d, %d)", tc.in, a, r, tc.wantAdd, tc.wantRemove)
		}
	}
}
