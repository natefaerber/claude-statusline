package render

import "testing"

func TestJoinCyclingEmpty(t *testing.T) {
	if got := joinCycling(nil, []string{" | "}); got != "" {
		t.Fatalf("empty parts should render empty, got %q", got)
	}
}

func TestJoinCyclingNoSeparators(t *testing.T) {
	got := joinCycling([]string{"a", "b", "c"}, nil)
	if got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestJoinCyclingSingleSeparator(t *testing.T) {
	got := joinCycling([]string{"a", "b", "c"}, []string{" | "})
	if got != "a | b | c" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinCyclingMultipleSeparators(t *testing.T) {
	// docs example: parts=[a,b,c,d,e] seps=[X,Y] → "aXbYcXdYe"
	got := joinCycling([]string{"a", "b", "c", "d", "e"}, []string{"X", "Y"})
	if got != "aXbYcXdYe" {
		t.Fatalf("got %q, want %q", got, "aXbYcXdYe")
	}
}

func TestJoinCyclingSinglePart(t *testing.T) {
	got := joinCycling([]string{"only"}, []string{" | "})
	if got != "only" {
		t.Fatalf("got %q", got)
	}
}
