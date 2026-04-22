# claude-statusline

A Go statusline for [Claude Code](https://docs.claude.com/claude-code). Reads the JSON Claude pipes to the configured `statusLine.command` and renders multi-line semantic groups with TOML-driven layout.

```
[Opus]
claude-statusline │ git:(main* ?1)
░░░░░░░░░░ 9.0% │ 5h ██░░░░░░░░ 24% │ 7d ░░░░░░░░░░ 5.2%
⏱  12m
```

_(Plain text shown — live output is colored: model cyan, git magenta, usage bars green→yellow→red as they fill.)_

## Why

[claude-hud](https://github.com/jarrodwatts/claude-hud) is great but rigid: layout is built into TS, terminal-width fallback wraps everything to 40 cols when stdout is a pipe, and adding/reordering segments means forking. This is a smaller, opinionated alternative that treats the line composition as data.

## Install

**Download a release binary** from [Releases](https://github.com/natefaerber/claude-statusline/releases) — linux / macOS / windows × amd64 / arm64.

**Via `go install`:**

```bash
go install github.com/natefaerber/claude-statusline/cmd/claude-statusline@latest
```

**Via mise:**

```toml
[tools]
"go:github.com/natefaerber/claude-statusline/cmd/claude-statusline" = "latest"
```

**From source:**

```bash
go build -o claude-statusline ./cmd/claude-statusline
```

## Wire it up

In `~/.claude/settings.json` (or `~/.claude-personal/settings.json`):

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/claude-statusline"
  }
}
```

## Configure

Drop a TOML file at `~/.config/claude-statusline/config.toml`. See [`config.example.toml`](./config.example.toml) for the full surface.

Each `[[lines]]` block becomes one rendered line. Segments inside a line are joined by `separator` (or a cycling `separators` list). A segment that has nothing to render (e.g. `git` outside a repo) is silently dropped.

### Example presets

**Minimal** — model, project, git:

```toml
separator = " │ "

[[lines]]
segments = ["model", "project", "git"]
```

**Full dashboard** — four lines, everything visible:

```toml
separator = " │ "

[[lines]]
segments = ["model", "agent", "vim_mode"]

[[lines]]
segments = ["project", "git", "worktree"]

[[lines]]
segments = ["context_bar", "usage_5h", "usage_7d"]

[[lines]]
segments = ["cost", "burn", "duration", "tokens"]

[segments]
tokens_min_context_pct = 85   # only show token breakdown near the limit
usage_bar = true
bar_width = 10
```

**Git-heavy** — collapses the usage stuff, foregrounds repo state:

```toml
separator = " · "

[[lines]]
segments = ["model", "project", "git"]

[[lines]]
segments = ["branch_diff", "unpushed", "stash", "last_commit"]

[[lines]]
segments = ["ci", "pr", "pr_checks", "pending_review"]

[segments]
base_branch = "main"
git_show_file_stats = true
```

## Segment reference

Any segment returns `""` to silently drop itself from its line when it has nothing useful to show.

### From the input JSON (always fast, no shellouts)

| Segment | Renders | Notes |
|---|---|---|
| `model` | `[Opus]` | Bold, from `model.display_name` |
| `context_bar` | `██████░░░░ 60%` | Progress bar + percentage of context used |
| `context_value` | `60%` or `12k/200k` | Configurable via `context_value = "percent\|tokens\|both\|remaining"` |
| `project` | `claude-statusline` | Last N path components of `workspace.current_dir`; N = `path_levels` |
| `usage_5h` | `5h ██░░░░░░░░ 24% (resets 3h)` | 5-hour rate-limit bar |
| `usage_7d` | `7d ░░░░░░░░░░ 5%` | 7-day rate-limit bar |
| `cost` | `$0.0123` | Cumulative session cost |
| `duration` | `⏱  12m` | Session wall-clock |
| `tokens` | `tok: 15k (in: 8k, out: 1k, cache: 7k)` | Gated by `tokens_min_context_pct` |
| `cache` | `cache 87%` | Prompt cache hit rate — healthy ≥75%; `cache_show_low` adds session low watermark |
| `diff` | `+156 -23` | Lines added/removed this session |
| `burn` | `$1.23/hr` | Extrapolated cost rate; `burn_show_peak` adds session peak |
| `todos` | `▸ 3/7` | Parsed from the transcript's latest `TodoWrite` |
| `session_name` | `moonlit-crafting-clover` | |
| `agent` | `⚙ code-reviewer` | Active subagent |
| `vim_mode` | `NORMAL` | |
| `output_style` | `explanatory` | Drops on `default` |
| `version` | `CC v2.1.90` | |
| `worktree` | `⌥ feature-auth` | Active worktree name |
| `custom` | `whatever you want` | Set `custom_line = "..."` |

### From git (one shell-out, no network)

| Segment | Renders | Notes |
|---|---|---|
| `git` | `git:(main* !2 +1 ?3)` | Branch + dirty + file stats |
| `unpushed` | `↑2 ↓1` | Ahead / behind upstream |
| `stash` | `📦 3` | From `git stash list` |
| `branch_diff` | `vs main +234 -45` | Diff vs `base_branch` (default `main`) |
| `last_commit` | `commit 4h ago` | |

### From `gh` CLI (cached 30s per cwd+branch+args)

Degrade silently if `gh` is missing or not authenticated.

| Segment | Renders | Notes |
|---|---|---|
| `ci` | `✓` / `⏳ 2` / `✗ 4/5` | Latest commit's Actions runs on the branch |
| `pr` | `PR #123 ✓` / `✗` / `👁` | Open PR + review state |
| `pr_checks` | `checks 8/10 ✓` | Full rollup incl. non-Actions checks |
| `pending_review` | `👁  3` | PRs across the org requesting your review |

### From external processes (opt-in, presence-driven)

| Segment | Renders | Notes |
|---|---|---|
| `compose` | `🐳 api web` | Running `docker compose` services (10s cache) |
| `endpoints` | `vite=http://localhost:5173` | Lines from a `.endpoint` file walking up from cwd |
| `pomodoro` | `🍅 18m` | Only while a timer is active (see subcommand below) |

## Subcommands

### `pomo` — session timer

```bash
claude-statusline pomo start          # default 25 minutes
claude-statusline pomo start 45       # custom duration
claude-statusline pomo stop
claude-statusline pomo status         # prints state file path
```

State lives at `$XDG_CACHE_HOME/claude-statusline/pomodoro.json` (or `~/.cache/...`). Add `pomodoro` to a `[[lines]]` block to surface it in the statusline; the segment drops itself when no timer is running.

### `endpoint` — per-project URLs

Walks up from cwd to find the nearest `.endpoint` file, then edits it in place (preserves comments and blank lines).

```bash
claude-statusline endpoint add http://localhost:3000
claude-statusline endpoint add vite=http://localhost:5173
claude-statusline endpoint add admin=http://localhost:3000/admin api=http://localhost:4000
claude-statusline endpoint rm vite
claude-statusline endpoint list
claude-statusline endpoint clear                 # delete the file
claude-statusline endpoint path                  # print target path
```

File format:

```
# dev servers for this project
http://localhost:3000
vite=http://localhost:5173
admin=http://localhost:3000/admin
```

### `version`

```bash
claude-statusline --version
# claude-statusline 0.1.0 (commit abc1234, built 2026-04-22)
```

## Colors

Every segment reads its color from the `[colors]` table, falling back to a built-in default. Values accept anything [lipgloss](https://github.com/charmbracelet/lipgloss) does — ANSI names (`red`, `brightCyan`), 256-color indices (`208`), or truecolor hex (`#ff8800`).

```toml
[colors]
model = "#7dd3fc"
git = "magenta"
bar_low = "green"
cache_high = "#22c55e"
```

Overridable keys: `model`, `project`, `git`, `branch`, `dim`, `bar_low`, `bar_med`, `bar_high`, `usage_low`, `agent`, `worktree`, `custom`, `cache_low`, `cache_med`, `cache_high`. See [`config.example.toml`](./config.example.toml) for the defaults.

`NO_COLOR=1` or `--no-color` strips all ANSI.

## Local state

Anything persistent lives under `$XDG_CACHE_HOME/claude-statusline/` (or `~/.cache/claude-statusline/`):

| Path | Written by | Purpose |
|---|---|---|
| `pomodoro.json` | `pomo start`/`stop` | Active timer |
| `burn/<session_id>` | `burn` segment | Per-session peak $/hr |
| `cache/<session_id>` | `cache` segment | Per-session low cache hit % |

`gh` and `docker compose` responses are cached in `$TMPDIR/claude-statusline-{gh,compose}/` (30s and 10s TTLs respectively) and safe to delete at any time.

## Adding a new segment

1. Write a `func renderX(c Ctx) string` in `internal/segments/segments.go`
2. `Register("x", renderX)` in `init()`
3. Add `"x"` to a `[[lines]]` block in your config
4. If it needs a knob, add a field to `SegmentOpts` in `internal/config/config.go`

See [`CLAUDE.md`](./CLAUDE.md) for the architecture and conventions.

## Test locally

```bash
cat sample.json | ./claude-statusline --config ./config.example.toml
```

Or via mise:

```bash
mise run preview
```

## License

MIT — see [LICENSE](./LICENSE).
