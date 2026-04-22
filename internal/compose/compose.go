// Package compose surfaces running services from docker-compose projects.
//
// Looks for a compose file in cwd (then walks up to the git root), runs
// `docker compose ps --services --filter status=running`, and falls back
// to the worktree's original cwd if nothing's running locally.
package compose

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const cacheTTL = 10 * time.Second

var composeFilenames = []string{
	"docker-compose.yaml", "docker-compose.yml",
	"compose.yaml", "compose.yml",
}

// Running returns the names of currently-running compose services for the
// project rooted at any of `searchDirs` (tried in order; first hit wins).
//
// Returns (nil, nil) when docker is missing, no compose file found, or all
// services are stopped — the segment should silently drop in those cases.
func Running(searchDirs ...string) ([]string, error) {
	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		composeDir := findComposeDir(dir)
		if composeDir == "" {
			continue
		}
		services, err := psServices(composeDir)
		if err != nil {
			continue
		}
		if len(services) > 0 {
			return services, nil
		}
	}
	return nil, nil
}

// findComposeDir walks from start up to the filesystem root looking for the
// first directory containing any of composeFilenames. Returns "" if none.
func findComposeDir(start string) string {
	dir := start
	for {
		for _, name := range composeFilenames {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// psServices runs `docker compose ps --services --filter status=running`
// in the given dir, returning one service per line.
func psServices(dir string) ([]string, error) {
	if cached, ok := readCache(dir); ok {
		return cached, nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, nil
	}

	cmd := exec.Command("docker", "compose", "ps", "--services", "--filter", "status=running")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var services []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			services = append(services, line)
		}
	}
	writeCache(dir, services)
	return services, nil
}

func cachePath(dir string) string {
	root := filepath.Join(os.TempDir(), "claude-statusline-compose")
	_ = os.MkdirAll(root, 0o755)
	h := sha1.Sum([]byte(dir))
	return filepath.Join(root, hex.EncodeToString(h[:])[:16])
}

func readCache(dir string) ([]string, bool) {
	path := cachePath(dir)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if len(data) == 0 {
		return []string{}, true
	}
	return strings.Split(string(data), "\n"), true
}

func writeCache(dir string, services []string) {
	_ = os.WriteFile(cachePath(dir), []byte(strings.Join(services, "\n")), 0o644)
}
