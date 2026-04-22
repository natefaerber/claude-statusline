// Package gh wraps the `gh` CLI with on-disk caching so the statusline
// doesn't hit GitHub on every render. Cache lives in $TMPDIR keyed by
// (cwd, branch, command) with a TTL.
package gh

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const cacheTTL = 30 * time.Second

// Run executes `gh <args...>` in cwd, returning stdout. Results are cached
// for cacheTTL keyed by cwd+branch+args; cache misses block on the network
// call, so callers should be okay paying ~200ms occasionally.
//
// Returns ("", nil) when gh is missing or the command exits nonzero — the
// statusline shouldn't fail just because the user isn't logged in.
func Run(cwd, branch string, args ...string) string {
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}

	key := cacheKey(cwd, branch, args)
	if cached, ok := readCache(key); ok {
		return cached
	}

	cmd := exec.Command("gh", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		// Cache empty result too — avoids hammering on permission failures.
		writeCache(key, "")
		return ""
	}

	result := string(out)
	writeCache(key, result)
	return result
}

func cacheKey(cwd, branch string, args []string) string {
	h := sha1.New()
	h.Write([]byte(cwd))
	h.Write([]byte{0})
	h.Write([]byte(branch))
	h.Write([]byte{0})
	for _, a := range args {
		h.Write([]byte(a))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func cachePath(key string) string {
	dir := filepath.Join(os.TempDir(), "claude-statusline-gh")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, key)
}

func readCache(key string) (string, bool) {
	path := cachePath(key)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return "", false
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func writeCache(key, value string) {
	_ = os.WriteFile(cachePath(key), []byte(value), 0o644)
}

// Age returns how stale the cached value for these args is, or 0 if no cache.
// Useful for showing "@HH:MM" markers like the user's old bash statusline.
func Age(cwd, branch string, args ...string) time.Duration {
	info, err := os.Stat(cachePath(cacheKey(cwd, branch, args)))
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}
