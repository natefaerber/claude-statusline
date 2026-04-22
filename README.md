# claude-statusline

A Go statusline for [Claude Code](https://docs.claude.com/claude-code). Reads the JSON Claude pipes to the configured `statusLine.command` and renders multi-line semantic groups with TOML-driven layout.

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

```toml
separator = " │ "

[[lines]]
segments = ["model", "context_bar"]

[[lines]]
segments = ["project", "git"]

[[lines]]
segments = ["usage_5h", "usage_7d"]

[segments]
git_show_file_stats = true
usage_bar = true
bar_width = 10
```

Available segments: `model`, `context_bar`, `context_value`, `project`, `git`, `usage_5h`, `usage_7d`, `cost`, `duration`, `tokens`, `session_name`, `agent`, `vim_mode`, `output_style`, `version`, `worktree`, `custom`.

A segment that has nothing to render (e.g. `git` outside a repo) is silently dropped.

## Adding a new segment

1. Write a `func renderX(c Ctx) string` in `internal/segments/segments.go`
2. `Register("x", renderX)` in `init()`
3. Add `"x"` to a `[[lines]]` block in your config

## Test locally

```bash
cat sample.json | ./claude-statusline --config ./config.example.toml
```
