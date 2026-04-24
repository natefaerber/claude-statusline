# Changelog

All notable changes to this project are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows [SemVer](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `pending_review_scope` config option. Default `"all"` keeps the current org-wide behavior (via `gh search prs`); `"repo"` scopes the count to review requests on the current repository (via `gh pr list`).

## [0.2.0] - 2026-04-23

### Changed

- **`cache` segment rewritten.** Instead of a ratio (`cache 87%`), now renders `cache miss Nk` only when non-cached input (`input_tokens + cache_creation_input_tokens`) exceeds `cache_miss_min` (default 5000). Rationale: warm-cache turns are ~100% hit by volume, so the percentage display was always green and carried no signal. Visibility is now the signal.
- Per-session state files in `$XDG_CACHE_HOME/claude-statusline/burn/` are swept opportunistically on each render — files older than 30 days are removed.
- README reorganized with rendered sample, segment reference table grouped by data source, subcommand docs, color palette reference, and local-state-file inventory.
- `segments.go` modernized for Go 1.24: `min`/`max` builtins, `fmt.Appendf`, `strings.SplitSeq`.

### Removed

- **BREAKING:** `cache_show_low` config key. The low-watermark display is gone with the ratio.
- Color keys `cache_low`, `cache_med`, `cache_high`. Replaced by a single `cache_miss` key (default red).
- `$XDG_CACHE_HOME/claude-statusline/cache/` is no longer written. Existing files from 0.1.0 can be safely deleted: `rm -rf ~/.cache/claude-statusline/cache/`.

### Added

- `cache_miss_min` config key (default 5000) for the render threshold on the `cache` segment.
- `hk.pkl` pre-commit hook config — `go-fmt`/`go-vet`/`go-test` on Go file stages; `hk check` for a full build + test sweep. Contributors: `mise install && hk install`.
- Unit tests for pure helpers: `endpoints.Parse/Add/Remove/ParseAddArg`, `render.joinCycling`, `segments.{formatPct,formatTokens,formatDuration,parseShortstat,sweepOldFiles}`, `config.Line.EffectiveSeparators`.
- `CLAUDE.md` architecture guide.
- `.gitignore` entries for `.claude/settings.local.json` and `.claude/scheduled_tasks.lock`.

## [0.1.0] - 2026-04-22

Initial public release.

### Added

- TOML-driven multi-line statusline for Claude Code. Each `[[lines]]` entry is a list of named segments joined by a configurable separator (or a cycling `separators` list).
- 31 segments across four sources:
  - **Input JSON**: `model`, `context_bar`, `context_value`, `project`, `usage_5h`, `usage_7d`, `cost`, `duration`, `tokens`, `cache`, `diff`, `burn`, `todos`, `session_name`, `agent`, `vim_mode`, `output_style`, `version`, `worktree`, `custom`.
  - **git**: `git`, `unpushed`, `stash`, `branch_diff`, `last_commit`.
  - **`gh` CLI** (cached 30s): `ci`, `pr`, `pr_checks`, `pending_review`.
  - **External processes**: `compose`, `endpoints`, `pomodoro`.
- Per-segment and per-line separator overrides; cycling separators list.
- Full color palette override via `[colors]` table — ANSI names, 256-color indices, or truecolor hex.
- `claude-statusline pomo {start [minutes]|stop|status}` subcommand for a session-local pomodoro timer.
- `claude-statusline endpoint {add|rm|list|clear|path}` subcommand for managing per-project `.endpoint` files.
- `claude-statusline --version` / `-v` / `version` with `main.version`, `main.commit`, `main.date` injected at release build time.
- Per-session state in `$XDG_CACHE_HOME/claude-statusline/` (pomodoro, burn peak, cache low watermark).
- Short-lived caches in `$TMPDIR` for `gh` (30s) and `docker compose` (10s).
- GoReleaser v2 cross-builds for linux / macOS / windows × amd64 / arm64.
- GitHub Actions: `ci` (gofmt / vet / test / build) and `release` (GoReleaser on `v*` tag).
- MIT License.

[Unreleased]: https://github.com/natefaerber/claude-statusline/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/natefaerber/claude-statusline/releases/tag/v0.2.0
[0.1.0]: https://github.com/natefaerber/claude-statusline/releases/tag/v0.1.0
