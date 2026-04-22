// Package endpoints reads and writes per-project `.endpoint` files.
//
// File format (line-oriented, comments and blank lines preserved on save):
//
//	# header comment
//	http://localhost:3000
//	vite=http://localhost:3200
//	sidekiq=http://localhost:3300
//
// Lookup walks up from a starting directory to the filesystem root looking
// for an existing file. Mutations either edit that file or, if none exists,
// create one in the chosen target directory.
package endpoints

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const Filename = ".endpoint"

// Entry is one parsed line. Comment is non-empty for blank/`#` lines so we
// can preserve them on save.
type Entry struct {
	Comment string // raw line for comments and blanks; URL/Label both empty
	Label   string // empty for bare URLs
	URL     string
}

// Find returns the first existing .endpoint file by walking up from each
// dir in order. Returns "" if none found.
func Find(dirs ...string) string {
	for _, start := range dirs {
		if start == "" {
			continue
		}
		dir := start
		for {
			candidate := filepath.Join(dir, Filename)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

// Load reads and parses a .endpoint file. Returns (nil, nil) if missing —
// callers can treat that as an empty list and create on save.
func Load(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(data)), nil
}

func Parse(content string) []Entry {
	var entries []Entry
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			entries = append(entries, Entry{Comment: raw})
			continue
		}
		label, url := "", trimmed
		if idx := strings.Index(trimmed, "="); idx > 0 {
			label = strings.TrimSpace(trimmed[:idx])
			url = strings.TrimSpace(trimmed[idx+1:])
		}
		entries = append(entries, Entry{Label: label, URL: url})
	}
	// Drop trailing blank from the trailing newline split.
	if n := len(entries); n > 0 && entries[n-1].Comment == "" && entries[n-1].URL == "" {
		entries = entries[:n-1]
	}
	return entries
}

// Save writes entries back to path, creating parent dirs as needed.
func Save(path string, entries []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, e := range entries {
		if e.URL == "" {
			b.WriteString(e.Comment)
			b.WriteString("\n")
			continue
		}
		if e.Label != "" {
			fmt.Fprintf(&b, "%s=%s\n", e.Label, e.URL)
		} else {
			fmt.Fprintf(&b, "%s\n", e.URL)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Add inserts (or updates) an entry. Match precedence:
//  1. Same Label (non-empty) → update its URL
//  2. Same URL with no label clash → no change
//  3. Otherwise → append
func Add(entries []Entry, label, url string) []Entry {
	if label != "" {
		for i, e := range entries {
			if e.Label == label {
				entries[i].URL = url
				return entries
			}
		}
	}
	for _, e := range entries {
		if e.URL == url && e.Label == label {
			return entries
		}
	}
	return append(entries, Entry{Label: label, URL: url})
}

// Remove drops entries matching by label OR URL (whichever the caller
// passed). Returns the modified slice.
func Remove(entries []Entry, key string) []Entry {
	out := entries[:0]
	for _, e := range entries {
		if e.URL == "" {
			out = append(out, e)
			continue
		}
		if e.Label == key || e.URL == key {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ParseAddArg accepts "URL" or "LABEL=URL" forms.
func ParseAddArg(s string) (label, url string) {
	if idx := strings.Index(s, "="); idx > 0 {
		return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
	}
	return "", strings.TrimSpace(s)
}
