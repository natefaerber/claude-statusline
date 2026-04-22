// Package git collects branch and working-tree state for the cwd.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type Status struct {
	Branch    string
	IsDirty   bool
	Modified  int
	Added     int
	Deleted   int
	Untracked int
	Ahead     int
	Behind    int
}

func Read(cwd string) *Status {
	branch := run(cwd, "git", "branch", "--show-current")
	if branch == "" {
		return nil
	}
	s := &Status{Branch: branch}

	if counts := run(cwd, "git", "rev-list", "--left-right", "--count", "@{u}...HEAD"); counts != "" {
		var behind, ahead int
		fmt.Sscanf(counts, "%d\t%d", &behind, &ahead)
		s.Behind = behind
		s.Ahead = ahead
	}

	porcelain := run(cwd, "git", "status", "--porcelain=v1")
	if porcelain == "" {
		return s
	}
	s.IsDirty = true
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		switch {
		case x == '?' && y == '?':
			s.Untracked++
		case x == 'A' || y == 'A':
			s.Added++
		case x == 'D' || y == 'D':
			s.Deleted++
		default:
			s.Modified++
		}
	}
	return s
}

func run(cwd string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
