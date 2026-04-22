// Package input parses the JSON Claude Code pipes into the statusline command.
package input

import (
	"encoding/json"
	"io"
)

type Input struct {
	CWD            string      `json:"cwd"`
	SessionID      string      `json:"session_id"`
	SessionName    string      `json:"session_name"`
	TranscriptPath string      `json:"transcript_path"`
	Model          Model       `json:"model"`
	Workspace      Workspace   `json:"workspace"`
	Version        string      `json:"version"`
	OutputStyle    OutputStyle `json:"output_style"`
	Cost           Cost        `json:"cost"`
	Context        Context     `json:"context_window"`
	Exceeds200k    bool        `json:"exceeds_200k_tokens"`
	RateLimits     RateLimits  `json:"rate_limits"`
	Vim            Vim         `json:"vim"`
	Agent          Agent       `json:"agent"`
	Worktree       Worktree    `json:"worktree"`
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Workspace struct {
	CurrentDir  string   `json:"current_dir"`
	ProjectDir  string   `json:"project_dir"`
	AddedDirs   []string `json:"added_dirs"`
	GitWorktree string   `json:"git_worktree"`
}

type OutputStyle struct {
	Name string `json:"name"`
}

type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

type Context struct {
	TotalInputTokens    int          `json:"total_input_tokens"`
	TotalOutputTokens   int          `json:"total_output_tokens"`
	ContextWindowSize   int          `json:"context_window_size"`
	UsedPercentage      float64      `json:"used_percentage"`
	RemainingPercentage float64      `json:"remaining_percentage"`
	CurrentUsage        CurrentUsage `json:"current_usage"`
}

type CurrentUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type RateLimits struct {
	FiveHour RateWindow `json:"five_hour"`
	SevenDay RateWindow `json:"seven_day"`
}

type RateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type Vim struct {
	Mode string `json:"mode"`
}

type Agent struct {
	Name string `json:"name"`
}

type Worktree struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalCWD    string `json:"original_cwd"`
	OriginalBranch string `json:"original_branch"`
}

func Parse(r io.Reader) (*Input, error) {
	var in Input
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return nil, err
	}
	return &in, nil
}
