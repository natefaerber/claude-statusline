// Package segments registers the named renderers used in the config's lines.
//
// Each renderer takes the parsed input + config + cached git state and returns
// a styled string (or "" to drop itself from its line).
package segments

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/natefaerber/claude-statusline/internal/compose"
	"github.com/natefaerber/claude-statusline/internal/config"
	"github.com/natefaerber/claude-statusline/internal/endpoints"
	"github.com/natefaerber/claude-statusline/internal/gh"
	"github.com/natefaerber/claude-statusline/internal/git"
	"github.com/natefaerber/claude-statusline/internal/input"
)

type Ctx struct {
	In  *input.Input
	Cfg *config.Config
	Git *git.Status
}

type Renderer func(Ctx) string

var registry = map[string]Renderer{}

func Register(name string, r Renderer) { registry[name] = r }
func Get(name string) (Renderer, bool) { r, ok := registry[name]; return r, ok }

// Default color palette — overridable per-key via the [colors] TOML table.
// Values accept any lipgloss color literal: ANSI names ("red", "brightCyan"),
// 0-255 indices ("208"), or hex ("#ff8800").
var defaultColors = map[string]string{
	"model":      "14", // cyan
	"project":    "11", // yellow
	"git":        "13", // magenta
	"branch":     "14", // cyan
	"dim":        "8",
	"bar_low":    "10", // green
	"bar_med":    "11", // yellow
	"bar_high":   "9",  // red
	"usage_low":  "12", // blue
	"agent":      "12", // blue
	"worktree":   "11", // yellow
	"custom":     "208",
	"cache_miss": "9", // red — cache bust
}

func style(c Ctx, key string) lipgloss.Style {
	if v, ok := c.Cfg.Colors[key]; ok && v != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(v))
	}
	if v, ok := defaultColors[key]; ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(v))
	}
	return lipgloss.NewStyle()
}

func dim(c Ctx) lipgloss.Style { return style(c, "dim") }

func init() {
	Register("model", renderModel)
	Register("context_bar", renderContextBar)
	Register("context_value", renderContextValue)
	Register("project", renderProject)
	Register("git", renderGit)
	Register("usage_5h", renderUsage5h)
	Register("usage_7d", renderUsage7d)
	Register("cost", renderCost)
	Register("duration", renderDuration)
	Register("session_name", renderSessionName)
	Register("agent", renderAgent)
	Register("vim_mode", renderVimMode)
	Register("output_style", renderOutputStyle)
	Register("version", renderVersion)
	Register("worktree", renderWorktree)
	Register("tokens", renderTokens)
	Register("cache", renderCache)
	Register("diff", renderDiff)
	Register("unpushed", renderUnpushed)
	Register("ci", renderCI)
	Register("pr", renderPR)
	Register("stash", renderStash)
	Register("branch_diff", renderBranchDiff)
	Register("pr_checks", renderPRChecks)
	Register("last_commit", renderLastCommit)
	Register("pending_review", renderPendingReview)
	Register("burn", renderBurn)
	Register("todos", renderTodos)
	Register("pomodoro", renderPomodoro)
	Register("endpoints", renderEndpoints)
	Register("compose", renderCompose)
	Register("custom", renderCustom)
}

// renderCompose surfaces running docker-compose services. Tries cwd first,
// then walks up to a compose file, then falls back to the worktree's
// original cwd if nothing's running locally — useful when the worktree
// shares its compose stack with the main checkout.
func renderCompose(c Ctx) string {
	cwd := c.In.Workspace.CurrentDir
	original := c.In.Worktree.OriginalCWD

	services, err := compose.Running(cwd, original)
	if err != nil || len(services) == 0 {
		return ""
	}
	parts := []string{dim(c).Render("🐳")}
	for _, s := range services {
		parts = append(parts, style(c, "bar_low").Render(s))
	}
	return strings.Join(parts, " ")
}

// renderEndpoints reads a per-project `.endpoint` file (walks up from cwd,
// then tries worktree.original_cwd). Drops itself silently when no file is
// found — this segment is opt-in via the file's existence.
//
// .endpoint format:
//
//	# comments and blank lines ignored
//	http://localhost:3000
//	admin=http://localhost:3000/admin
//	web=http://localhost:3200
//
// Lines without "=" render the URL as the label.
func renderEndpoints(c Ctx) string {
	path := endpoints.Find(c.In.Workspace.CurrentDir, c.In.Worktree.OriginalCWD)
	if path == "" {
		return ""
	}
	entries, err := endpoints.Load(path)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var parts []string
	for _, e := range entries {
		if e.URL == "" {
			continue
		}
		text := e.URL
		if e.Label != "" {
			text = fmt.Sprintf("%s:%s", e.Label, e.URL)
		}
		parts = append(parts, style(c, "bar_low").Render(text))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func renderModel(c Ctx) string {
	name := c.In.Model.DisplayName
	if name == "" {
		name = c.In.Model.ID
	}
	return style(c, "model").Bold(true).Render("[" + name + "]")
}

func renderContextBar(c Ctx) string {
	width := c.Cfg.Segments.BarWidth
	if width <= 0 {
		width = 10
	}
	pct := c.In.Context.UsedPercentage
	color := barColor(c, pct)
	return makeBar(c, pct, width, color) + " " + color.Render(formatPct(pct))
}

func renderContextValue(c Ctx) string {
	pct := c.In.Context.UsedPercentage
	switch c.Cfg.Segments.ContextValue {
	case "tokens":
		return dim(c).Render(fmt.Sprintf("%s/%s",
			formatTokens(c.In.Context.TotalInputTokens+c.In.Context.TotalOutputTokens),
			formatTokens(c.In.Context.ContextWindowSize)))
	case "remaining":
		return dim(c).Render(formatPct(100 - pct))
	case "both":
		return dim(c).Render(fmt.Sprintf("%s (%s/%s)", formatPct(pct),
			formatTokens(c.In.Context.TotalInputTokens+c.In.Context.TotalOutputTokens),
			formatTokens(c.In.Context.ContextWindowSize)))
	default:
		return barColor(c, pct).Render(formatPct(pct))
	}
}

func renderProject(c Ctx) string {
	dir := c.In.Workspace.CurrentDir
	if dir == "" {
		dir = c.In.CWD
	}
	if dir == "" {
		return ""
	}
	levels := max(c.Cfg.Segments.PathLevels, 1)
	parts := strings.Split(filepath.ToSlash(dir), "/")
	cleaned := parts[:0]
	for _, p := range parts {
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) > levels {
		cleaned = cleaned[len(cleaned)-levels:]
	}
	return style(c, "project").Render(strings.Join(cleaned, "/"))
}

func renderGit(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	parts := []string{c.Git.Branch}
	if c.Cfg.Segments.GitShowDirty && c.Git.IsDirty {
		parts = append(parts, "*")
	}
	if c.Cfg.Segments.GitShowFileStats {
		var stats []string
		if c.Git.Modified > 0 {
			stats = append(stats, fmt.Sprintf("!%d", c.Git.Modified))
		}
		if c.Git.Added > 0 {
			stats = append(stats, fmt.Sprintf("+%d", c.Git.Added))
		}
		if c.Git.Deleted > 0 {
			stats = append(stats, fmt.Sprintf("✘%d", c.Git.Deleted))
		}
		if c.Git.Untracked > 0 {
			stats = append(stats, fmt.Sprintf("?%d", c.Git.Untracked))
		}
		if len(stats) > 0 {
			parts = append(parts, " "+strings.Join(stats, " "))
		}
	}
	gs, bs := style(c, "git"), style(c, "branch")
	return gs.Render("git:(") + bs.Render(strings.Join(parts, "")) + gs.Render(")")
}

func renderUsage5h(c Ctx) string { return usageWindow(c, "5h", c.In.RateLimits.FiveHour) }
func renderUsage7d(c Ctx) string { return usageWindow(c, "7d", c.In.RateLimits.SevenDay) }

func usageWindow(c Ctx, label string, w input.RateWindow) string {
	if w.ResetsAt == 0 && w.UsedPercentage == 0 {
		return ""
	}
	pct := w.UsedPercentage
	color := usageColor(c, pct)
	reset := formatReset(w.ResetsAt)
	if c.Cfg.Segments.UsageBar {
		body := fmt.Sprintf("%s %s", makeBar(c, pct, c.Cfg.Segments.BarWidth, color), color.Render(formatPct(pct)))
		if reset != "" {
			body += dim(c).Render(fmt.Sprintf(" (%s)", reset))
		}
		return dim(c).Render(label+" ") + body
	}
	out := fmt.Sprintf("%s: %s", label, color.Render(formatPct(pct)))
	if reset != "" {
		out += dim(c).Render(fmt.Sprintf(" (%s)", reset))
	}
	return dim(c).Render(out)
}

func renderCost(c Ctx) string {
	if c.In.Cost.TotalCostUSD <= 0 {
		return ""
	}
	return dim(c).Render(fmt.Sprintf("$%.4f", c.In.Cost.TotalCostUSD))
}

func renderDuration(c Ctx) string {
	if c.In.Cost.TotalDurationMS <= 0 {
		return ""
	}
	d := time.Duration(c.In.Cost.TotalDurationMS) * time.Millisecond
	return dim(c).Render("⏱  " + formatDuration(d))
}

func renderSessionName(c Ctx) string {
	if c.In.SessionName == "" {
		return ""
	}
	return dim(c).Render(c.In.SessionName)
}

func renderAgent(c Ctx) string {
	if c.In.Agent.Name == "" {
		return ""
	}
	return style(c, "agent").Render("⚙ " + c.In.Agent.Name)
}

func renderVimMode(c Ctx) string {
	if c.In.Vim.Mode == "" {
		return ""
	}
	return dim(c).Render(c.In.Vim.Mode)
}

func renderOutputStyle(c Ctx) string {
	if c.In.OutputStyle.Name == "" || c.In.OutputStyle.Name == "default" {
		return ""
	}
	return dim(c).Render(c.In.OutputStyle.Name)
}

func renderVersion(c Ctx) string {
	if c.In.Version == "" {
		return ""
	}
	return dim(c).Render("CC v" + c.In.Version)
}

func renderWorktree(c Ctx) string {
	if c.In.Worktree.Name == "" {
		return ""
	}
	return style(c, "worktree").Render("⌥ " + c.In.Worktree.Name)
}

func renderTokens(c Ctx) string {
	if c.In.Context.UsedPercentage < c.Cfg.Segments.TokensMinContextPct {
		return ""
	}
	u := c.In.Context.CurrentUsage
	total := u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	if total == 0 {
		return ""
	}
	return dim(c).Render(fmt.Sprintf("tok: %s (in: %s, out: %s, cache: %s)",
		formatTokens(total),
		formatTokens(u.InputTokens),
		formatTokens(u.OutputTokens),
		formatTokens(u.CacheCreationInputTokens+u.CacheReadInputTokens)))
}

// renderCache flags cache busts: the non-cached input tokens this turn.
// Silent in the steady state — every warm-cache turn is ~100% hit by volume,
// so a percentage display is always green and never actionable. Instead we
// render only when the miss (input + cache_create) is large enough to matter,
// so the segment's visibility IS the signal.
func renderCache(c Ctx) string {
	u := c.In.Context.CurrentUsage
	miss := u.InputTokens + u.CacheCreationInputTokens
	threshold := c.Cfg.Segments.CacheMissMin
	if threshold <= 0 {
		threshold = 5000
	}
	if miss < threshold {
		return ""
	}
	return style(c, "cache_miss").Render("cache miss " + formatTokens(miss))
}

func renderCustom(c Ctx) string {
	if c.Cfg.Segments.CustomLine == "" {
		return ""
	}
	return style(c, "custom").Render(c.Cfg.Segments.CustomLine)
}

// ghCheck mirrors entries from `gh pr view --json statusCheckRollup`.
type ghCheck struct {
	State      string `json:"state"`      // SUCCESS | FAILURE | PENDING | ...
	Status     string `json:"status"`     // COMPLETED | IN_PROGRESS | QUEUED
	Conclusion string `json:"conclusion"` // SUCCESS | FAILURE | NEUTRAL | ...
}

type ghPRWithChecks struct {
	StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
}

// renderPRChecks shows the full check rollup on the current branch's PR —
// this includes non-Actions checks (e.g. external CI providers, branch
// protection requirements) that `ci` doesn't see.
func renderPRChecks(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out := gh.Run(cwd, c.Git.Branch, "pr", "view", "--json", "statusCheckRollup")
	if out == "" {
		return ""
	}
	var pr ghPRWithChecks
	if err := json.Unmarshal([]byte(out), &pr); err != nil || len(pr.StatusCheckRollup) == 0 {
		return ""
	}
	var pass, fail, pending int
	for _, ch := range pr.StatusCheckRollup {
		// Status checks (commit statuses) use State; check runs use Status+Conclusion.
		state := strings.ToUpper(ch.State)
		if state == "" {
			if ch.Status == "COMPLETED" {
				state = strings.ToUpper(ch.Conclusion)
			} else {
				state = strings.ToUpper(ch.Status)
			}
		}
		switch state {
		case "SUCCESS":
			pass++
		case "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED":
			fail++
		case "PENDING", "IN_PROGRESS", "QUEUED", "WAITING":
			pending++
		}
	}
	total := pass + fail + pending
	if total == 0 {
		return ""
	}
	switch {
	case fail > 0:
		return style(c, "bar_high").Render(fmt.Sprintf("checks %d/%d", pass, total))
	case pending > 0:
		return style(c, "bar_med").Render(fmt.Sprintf("checks %d/%d ⏳", pass, total))
	default:
		return style(c, "bar_low").Render(fmt.Sprintf("checks %d/%d ✓", pass, total))
	}
}

// renderLastCommit shows time since last commit on current branch.
// Useful nudge when you've been editing without committing for a while.
func renderLastCommit(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out, err := exec.Command("git", "-C", cwd, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return ""
	}
	var unix int64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &unix)
	if unix == 0 {
		return ""
	}
	d := time.Since(time.Unix(unix, 0))
	return dim(c).Render("commit " + formatDuration(d) + " ago")
}

// renderPendingReview counts open PRs assigned to the current user for review.
// Cached 30s. Scope is configurable: default "all" (org-wide via `gh search`,
// same answer regardless of cwd) or "repo" (current repo only via `gh pr list`,
// naturally scoped by git remote).
func renderPendingReview(c Ctx) string {
	cwd := c.In.Workspace.CurrentDir
	var out string
	if c.Cfg.Segments.PendingReviewScope == "repo" {
		if c.Git == nil {
			return ""
		}
		out = gh.Run(cwd, "_repo_",
			"pr", "list",
			"--state", "open",
			"--search", "review-requested:@me",
			"--json", "number", "--limit", "100",
		)
	} else {
		out = gh.Run(cwd, "_global_",
			"search", "prs", "--review-requested", "@me", "--state", "open",
			"--json", "number", "--limit", "100",
		)
	}
	if out == "" {
		return ""
	}
	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil || len(prs) == 0 {
		return ""
	}
	return style(c, "bar_med").Render(fmt.Sprintf("👁  %d", len(prs)))
}

// renderBurn shows extrapolated cost rate. Returns "" until duration > 30s
// (numbers below that are noise). Tracks per-session peak in
// ~/.cache/claude-statusline/burn/<session_id> so the high watermark survives
// across statusline renders.
func renderBurn(c Ctx) string {
	costUSD := c.In.Cost.TotalCostUSD
	durMS := c.In.Cost.TotalDurationMS
	if costUSD <= 0 || durMS < 30_000 {
		return ""
	}
	hours := float64(durMS) / 1000 / 3600
	rate := costUSD / hours

	out := dim(c).Render(fmt.Sprintf("$%.2f/hr", rate))

	if c.Cfg.Segments.BurnShowPeak && c.In.SessionID != "" {
		peak := updateBurnPeak(c.In.SessionID, rate)
		// Show peak only when current is meaningfully below it — otherwise
		// we'd just print the same number twice.
		if peak > rate*1.2 {
			out += dim(c).Render(fmt.Sprintf(" ⤒$%.2f", peak))
		}
	}
	return out
}

// sessionStateTTL bounds how long per-session state files stick around.
// Long enough that you can resume a session hours later with the peak intact;
// short enough that thousands of dead session files don't accumulate on disk.
const sessionStateTTL = 30 * 24 * time.Hour

// sweepOldFiles removes regular files in dir with mtime older than maxAge.
// Best-effort — any error (missing dir, unreadable entry, failed unlink) is
// silently swallowed. Safe to call on directories that don't exist.
func sweepOldFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// updateBurnPeak reads, updates, and returns the per-session peak rate.
// Best-effort: any IO failure just returns the current rate. Opportunistically
// sweeps expired siblings so the burn/ dir doesn't grow unbounded.
func updateBurnPeak(sessionID string, current float64) float64 {
	path := burnPeakPath(sessionID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return current
	}
	sweepOldFiles(dir, sessionStateTTL)
	var stored float64
	if data, err := os.ReadFile(path); err == nil {
		fmt.Sscanf(string(data), "%f", &stored)
	}
	if current > stored {
		_ = os.WriteFile(path, fmt.Appendf(nil, "%f", current), 0o644)
		return current
	}
	return stored
}

func burnPeakPath(sessionID string) string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "claude-statusline", "burn", sessionID)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude-statusline", "burn", sessionID)
}

// renderTodos parses the transcript file for the latest TodoWrite tool call
// and shows "▸ N/M" — completed / total. The transcript is JSONL; we scan
// backward for efficiency.
func renderTodos(c Ctx) string {
	path := c.In.TranscriptPath
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Walk backward through lines for the most recent TodoWrite tool_use.
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if line == "" || !strings.Contains(line, "TodoWrite") {
			continue
		}
		var event struct {
			Message struct {
				Content []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Input struct {
						Todos []struct {
							Status string `json:"status"`
						} `json:"todos"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		for _, item := range event.Message.Content {
			if item.Type != "tool_use" || item.Name != "TodoWrite" {
				continue
			}
			total := len(item.Input.Todos)
			if total == 0 {
				return ""
			}
			done := 0
			for _, t := range item.Input.Todos {
				if t.Status == "completed" {
					done++
				}
			}
			color := style(c, "bar_med")
			if done == total {
				color = style(c, "bar_low")
			}
			return color.Render(fmt.Sprintf("▸ %d/%d", done, total))
		}
	}
	return ""
}

// pomodoroState lives in ~/.cache/claude-statusline/pomodoro.json. Created
// by `claude-statusline pomo start [minutes]`, removed by `pomo stop`.
type pomodoroState struct {
	StartUnix int64 `json:"start_unix"`
	Minutes   int   `json:"minutes"`
}

// PomodoroPath returns the on-disk state location.
func PomodoroPath() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "claude-statusline", "pomodoro.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude-statusline", "pomodoro.json")
}

func renderPomodoro(c Ctx) string {
	data, err := os.ReadFile(PomodoroPath())
	if err != nil {
		return ""
	}
	var s pomodoroState
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	if s.StartUnix == 0 || s.Minutes == 0 {
		return ""
	}
	end := time.Unix(s.StartUnix, 0).Add(time.Duration(s.Minutes) * time.Minute)
	remaining := time.Until(end)
	if remaining <= 0 {
		return style(c, "bar_high").Render("🍅 done")
	}
	color := style(c, "bar_low")
	if remaining < 5*time.Minute {
		color = style(c, "bar_med")
	}
	return color.Render("🍅 " + formatDuration(remaining))
}

// PomodoroStart writes a new pomodoro state file. Called from the CLI
// subcommand handler in main.go.
func PomodoroStart(minutes int) error {
	if minutes <= 0 {
		minutes = 25
	}
	path := PomodoroPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	s := pomodoroState{StartUnix: time.Now().Unix(), Minutes: minutes}
	data, _ := json.Marshal(s)
	return os.WriteFile(path, data, 0o644)
}

// PomodoroStop removes the state file. No-op if already gone.
func PomodoroStop() error {
	err := os.Remove(PomodoroPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// renderStash counts entries in `git stash list`. Drops if zero.
func renderStash(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out, err := exec.Command("git", "-C", cwd, "stash", "list").Output()
	if err != nil {
		return ""
	}
	count := 0
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			count++
		}
	}
	if count == 0 {
		return ""
	}
	return style(c, "bar_med").Render(fmt.Sprintf("📦 %d", count))
}

// renderBranchDiff shows +N -M lines vs base branch (default "main").
// Skipped when on the base branch itself or when the base ref doesn't exist.
func renderBranchDiff(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	base := c.Cfg.Segments.BaseBranch
	if base == "" {
		base = "main"
	}
	if c.Git.Branch == base {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out, err := exec.Command("git", "-C", cwd, "diff", "--shortstat", base+"...HEAD").Output()
	if err != nil {
		return ""
	}
	// Output looks like: " 5 files changed, 234 insertions(+), 45 deletions(-)"
	text := strings.TrimSpace(string(out))
	if text == "" {
		return ""
	}
	added, removed := parseShortstat(text)
	if added == 0 && removed == 0 {
		return ""
	}
	plus := style(c, "bar_low").Render(fmt.Sprintf("+%d", added))
	minus := style(c, "bar_high").Render(fmt.Sprintf("-%d", removed))
	return dim(c).Render("vs "+base+" ") + plus + " " + minus
}

func parseShortstat(s string) (added, removed int) {
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		var n int
		switch {
		case strings.Contains(part, "insertion"):
			fmt.Sscanf(part, "%d", &n)
			added = n
		case strings.Contains(part, "deletion"):
			fmt.Sscanf(part, "%d", &n)
			removed = n
		}
	}
	return
}

// renderDiff shows the session's net code change footprint from cost.total_lines_*.
func renderDiff(c Ctx) string {
	added, removed := c.In.Cost.TotalLinesAdded, c.In.Cost.TotalLinesRemoved
	if added == 0 && removed == 0 {
		return ""
	}
	plus := style(c, "bar_low").Render(fmt.Sprintf("+%d", added))
	minus := style(c, "bar_high").Render(fmt.Sprintf("-%d", removed))
	return plus + " " + minus
}

// renderUnpushed surfaces ahead/behind counts vs the upstream branch.
func renderUnpushed(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	if c.Git.Ahead == 0 && c.Git.Behind == 0 {
		return ""
	}
	var parts []string
	if c.Git.Ahead > 0 {
		parts = append(parts, style(c, "bar_med").Render(fmt.Sprintf("↑%d", c.Git.Ahead)))
	}
	if c.Git.Behind > 0 {
		parts = append(parts, style(c, "bar_high").Render(fmt.Sprintf("↓%d", c.Git.Behind)))
	}
	return strings.Join(parts, " ")
}

// ghRun struct mirrors `gh run list --json status,conclusion,headSha,...`.
type ghRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadSha    string `json:"headSha"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Name       string `json:"name"`
}

// renderCI shows the latest commit's CI rollup for the current branch.
// Cached 30s by gh.Run; one call returns 30 most recent runs and we filter
// to the latest commit's runs locally.
func renderCI(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out := gh.Run(cwd, c.Git.Branch,
		"run", "list", "--branch", c.Git.Branch, "--limit", "30",
		"--json", "status,conclusion,headSha,createdAt,updatedAt,name",
	)
	if out == "" {
		return ""
	}

	var runs []ghRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil || len(runs) == 0 {
		return ""
	}

	latestSha := runs[0].HeadSha
	var failed, inProgress, success int
	for _, r := range runs {
		if r.HeadSha != latestSha {
			continue
		}
		switch {
		case r.Conclusion == "failure":
			failed++
		case r.Status == "in_progress" || r.Status == "queued":
			inProgress++
		case r.Conclusion == "success":
			success++
		}
	}
	total := failed + inProgress + success
	if total == 0 {
		return ""
	}
	switch {
	case failed > 0:
		return style(c, "bar_high").Render(fmt.Sprintf("✗ %d/%d", success, success+failed))
	case inProgress > 0:
		return style(c, "bar_med").Render(fmt.Sprintf("⏳ %d", inProgress))
	default:
		return style(c, "bar_low").Render("✓")
	}
}

// ghPR struct mirrors `gh pr view --json state,reviewDecision,number,url,statusCheckRollup`.
type ghPR struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
}

// renderPR shows the open PR for the current branch and its review state.
// `gh pr view` exits nonzero when there's no PR — gh.Run swallows that to "".
func renderPR(c Ctx) string {
	if c.Git == nil {
		return ""
	}
	cwd := c.In.Workspace.CurrentDir
	out := gh.Run(cwd, c.Git.Branch,
		"pr", "view", "--json", "number,state,reviewDecision",
	)
	if out == "" {
		return ""
	}

	var pr ghPR
	if err := json.Unmarshal([]byte(out), &pr); err != nil || pr.Number == 0 {
		return ""
	}

	label := fmt.Sprintf("PR #%d", pr.Number)
	switch pr.ReviewDecision {
	case "APPROVED":
		return style(c, "bar_low").Render(label + " ✓")
	case "CHANGES_REQUESTED":
		return style(c, "bar_high").Render(label + " ✗")
	case "REVIEW_REQUIRED":
		return style(c, "bar_med").Render(label + " 👁")
	default:
		return dim(c).Render(label)
	}
}

// --- helpers ---

func makeBar(c Ctx, pct float64, width int, color lipgloss.Style) string {
	if width <= 0 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := min(int((pct/100.0)*float64(width)), width)
	return color.Render(strings.Repeat("█", filled)) + dim(c).Render(strings.Repeat("░", width-filled))
}

func barColor(c Ctx, pct float64) lipgloss.Style {
	switch {
	case pct >= 90:
		return style(c, "bar_high")
	case pct >= 75:
		return style(c, "bar_med")
	default:
		return style(c, "bar_low")
	}
}

func usageColor(c Ctx, pct float64) lipgloss.Style {
	switch {
	case pct >= 90:
		return style(c, "bar_high")
	case pct >= 75:
		return style(c, "bar_med")
	default:
		return style(c, "usage_low")
	}
}

func formatPct(pct float64) string {
	if pct < 10 {
		return fmt.Sprintf("%.1f%%", pct)
	}
	return fmt.Sprintf("%d%%", int(pct+0.5))
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatReset(unix int64) string {
	if unix == 0 {
		return ""
	}
	d := time.Until(time.Unix(unix, 0))
	if d <= 0 {
		return ""
	}
	return "resets " + formatDuration(d)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours() / 24)
	h := int(d.Hours()) - days*24
	if h == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, h)
}
