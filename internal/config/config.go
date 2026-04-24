// Package config loads the user's TOML config and exposes defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk shape. Lines are rendered top-to-bottom; each line
// is a list of segment names joined by `Separator`. Segment names map to
// renderers registered in the segments package.
type Config struct {
	// Separator and Separators set the default joiner(s) between segments
	// on a line. Use Separators for a cycling list (e.g. [" │ ", " · "]
	// renders "a │ b · c │ d · e"); Separator is the simpler single-string
	// form. Per-line overrides take precedence.
	Separator  string            `toml:"separator"`
	Separators []string          `toml:"separators"`
	Lines      []Line            `toml:"lines"`
	Colors     map[string]string `toml:"colors"`
	Segments   SegmentOpts       `toml:"segments"`
}

type Line struct {
	Segments []string `toml:"segments"`
	// Separator / Separators override the global ones for this line only.
	// Same semantics — Separators cycles, Separator is single.
	Separator  string   `toml:"separator"`
	Separators []string `toml:"separators"`
}

// EffectiveSeparators returns the cycling separator list for a line,
// falling back to the line's Separator, then the config's Separators,
// then Config.Separator, then a sane default.
func (l Line) EffectiveSeparators(cfg *Config) []string {
	if len(l.Separators) > 0 {
		return l.Separators
	}
	if l.Separator != "" {
		return []string{l.Separator}
	}
	if len(cfg.Separators) > 0 {
		return cfg.Separators
	}
	if cfg.Separator != "" {
		return []string{cfg.Separator}
	}
	return []string{" │ "}
}

// SegmentOpts holds per-segment knobs. Add fields here as segments grow
// configurable bits — keep them inline rather than inventing a [segments.X]
// table per segment until there's a real reason.
type SegmentOpts struct {
	PathLevels       int    `toml:"path_levels"`
	ContextValue     string `toml:"context_value"` // percent | tokens | both | remaining
	GitShowDirty     bool   `toml:"git_show_dirty"`
	GitShowFileStats bool   `toml:"git_show_file_stats"`
	GitShowAhead     bool   `toml:"git_show_ahead_behind"`
	UsageBar         bool   `toml:"usage_bar"`
	BarWidth         int    `toml:"bar_width"`
	CustomLine       string `toml:"custom_line"`
	// TokensMinContextPct hides the `tokens` segment unless context usage is
	// at least this percent. Default 0 (always show). Try 85 to mirror
	// claude-hud's "only when it matters" behavior.
	TokensMinContextPct float64 `toml:"tokens_min_context_pct"`
	// BaseBranch is the comparison target for the `branch_diff` segment.
	// Defaults to "main".
	BaseBranch string `toml:"base_branch"`
	// BurnShowPeak appends the session's high-watermark $/hr next to the
	// current rate when current is at least 20% below peak.
	BurnShowPeak bool `toml:"burn_show_peak"`
	// CacheMissMin is the smallest non-cached input (input + cache_create)
	// in tokens that makes the `cache` segment render. Below this, the
	// segment stays silent — a warm-cache turn is always near 100% hit and
	// showing that is noise. Default 5000. Increase to suppress first-turn
	// cache writes; decrease to catch smaller busts.
	CacheMissMin int `toml:"cache_miss_min"`
	// PendingReviewScope limits the `pending_review` segment. "" or "all"
	// (default) searches every repo you can see via `gh search prs`. "repo"
	// only counts open review requests on the current repo via `gh pr list`.
	PendingReviewScope string `toml:"pending_review_scope"`
}

func Default() *Config {
	return &Config{
		Separator: " │ ",
		Lines: []Line{
			{Segments: []string{"model", "context_bar"}},
			{Segments: []string{"project", "git"}},
			{Segments: []string{"usage_5h", "usage_7d"}},
			{Segments: []string{"cost", "duration"}},
		},
		Segments: SegmentOpts{
			PathLevels:       1,
			ContextValue:     "percent",
			GitShowDirty:     true,
			GitShowFileStats: true,
			UsageBar:         true,
			BarWidth:         10,
		},
	}
}

// Load reads the config file at path, falling back to defaults when missing.
// Unknown TOML keys are an error so typos surface fast.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	md, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("unknown config keys: %v", undecoded)
	}
	return cfg, nil
}

// DefaultPath returns the conventional config location, honoring XDG_CONFIG_HOME.
func DefaultPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "claude-statusline", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude-statusline", "config.toml")
}
