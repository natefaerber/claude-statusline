package endpoints

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	in := `# header comment

http://localhost:3000
vite=http://localhost:5173
# inline note
admin=http://localhost:3000/admin
`
	got := Parse(in)
	want := []Entry{
		{Comment: "# header comment"},
		{Comment: ""},
		{URL: "http://localhost:3000"},
		{Label: "vite", URL: "http://localhost:5173"},
		{Comment: "# inline note"},
		{Label: "admin", URL: "http://localhost:3000/admin"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestAddNewLabel(t *testing.T) {
	got := Add(nil, "vite", "http://localhost:5173")
	want := []Entry{{Label: "vite", URL: "http://localhost:5173"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestAddUpdatesExistingLabel(t *testing.T) {
	entries := []Entry{{Label: "vite", URL: "http://old"}}
	got := Add(entries, "vite", "http://new")
	if len(got) != 1 || got[0].URL != "http://new" {
		t.Fatalf("label update failed: %#v", got)
	}
}

func TestAddBareURL(t *testing.T) {
	got := Add(nil, "", "http://localhost:3000")
	want := []Entry{{URL: "http://localhost:3000"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestAddDuplicateNoop(t *testing.T) {
	entries := []Entry{{URL: "http://localhost:3000"}}
	got := Add(entries, "", "http://localhost:3000")
	if len(got) != 1 {
		t.Fatalf("expected dedupe, got %#v", got)
	}
}

func TestRemoveByLabel(t *testing.T) {
	entries := []Entry{
		{Label: "vite", URL: "http://localhost:5173"},
		{URL: "http://localhost:3000"},
	}
	got := Remove(entries, "vite")
	if len(got) != 1 || got[0].URL != "http://localhost:3000" {
		t.Fatalf("remove by label failed: %#v", got)
	}
}

func TestRemoveByURL(t *testing.T) {
	entries := []Entry{
		{Label: "vite", URL: "http://localhost:5173"},
		{URL: "http://localhost:3000"},
	}
	got := Remove(entries, "http://localhost:3000")
	if len(got) != 1 || got[0].Label != "vite" {
		t.Fatalf("remove by URL failed: %#v", got)
	}
}

func TestRemovePreservesComments(t *testing.T) {
	entries := []Entry{
		{Comment: "# header"},
		{Label: "vite", URL: "http://localhost:5173"},
	}
	got := Remove(entries, "vite")
	if len(got) != 1 || got[0].Comment != "# header" {
		t.Fatalf("comments should be preserved: %#v", got)
	}
}

func TestParseAddArg(t *testing.T) {
	cases := []struct {
		in, wantLabel, wantURL string
	}{
		{"http://localhost", "", "http://localhost"},
		{"vite=http://localhost:5173", "vite", "http://localhost:5173"},
		{"  vite = http://localhost  ", "vite", "http://localhost"},
	}
	for _, tc := range cases {
		l, u := ParseAddArg(tc.in)
		if l != tc.wantLabel || u != tc.wantURL {
			t.Errorf("ParseAddArg(%q) = (%q, %q); want (%q, %q)", tc.in, l, u, tc.wantLabel, tc.wantURL)
		}
	}
}
