// Package render composes the configured lines from registered segments.
package render

import (
	"strings"

	"github.com/natefaerber/claude-statusline/internal/config"
	"github.com/natefaerber/claude-statusline/internal/git"
	"github.com/natefaerber/claude-statusline/internal/input"
	"github.com/natefaerber/claude-statusline/internal/segments"
)

func Render(in *input.Input, cfg *config.Config) string {
	ctx := segments.Ctx{
		In:  in,
		Cfg: cfg,
		Git: git.Read(in.Workspace.CurrentDir),
	}

	var lines []string
	for _, line := range cfg.Lines {
		seps := line.EffectiveSeparators(cfg)
		var parts []string
		for _, name := range line.Segments {
			r, ok := segments.Get(name)
			if !ok {
				continue
			}
			out := r(ctx)
			if out != "" {
				parts = append(parts, out)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, joinCycling(parts, seps))
		}
	}
	return strings.Join(lines, "\n")
}

// joinCycling joins parts with separators chosen in round-robin order.
// For parts=[a,b,c,d,e] and seps=[X,Y]: "a X b Y c X d Y e".
func joinCycling(parts, seps []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(seps) == 0 {
		return strings.Join(parts, "")
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		b.WriteString(seps[(i-1)%len(seps)])
		b.WriteString(parts[i])
	}
	return b.String()
}
