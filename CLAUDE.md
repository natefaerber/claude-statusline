# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

All workflows go through `mise` (tasks defined in `mise.toml`):

- `mise run build` — compile to `./claude-statusline` (incremental via `sources`/`outputs`)
- `mise run install` — `go install ./cmd/claude-statusline` to `$GOBIN`
- `mise run run` (alias: `preview`) — build and render `sample.json` through `config.example.toml`
- `mise run test` — `go test ./...`
- `mise run check` — `fmt` + `vet` + `test`
- `mise run tidy` — `go mod tidy`

Single-package test: `go test ./internal/segments/...`. Single test: `go test ./internal/segments -run TestName`.

`CGO_ENABLED=0` is set in `mise.toml` — build static binaries, don't introduce cgo deps.

Local rendering dev loop:

```bash
cat sample.json | ./claude-statusline --config ./config.example.toml
```

`scripts/preview` does the same with a build step baked in.

## Architecture

The binary is a stdin-driven pipe filter. Claude Code invokes the configured `statusLine.command` with a JSON blob on stdin; the process writes one or more styled lines to stdout and exits.

**Render pipeline** (`cmd/claude-statusline/main.go` → `internal/render`):

1. `input.Parse(os.Stdin)` decodes Claude's JSON into `input.Input` (see `internal/input/input.go` for the full schema — model, cost, context window, rate limits, vim, agent, worktree, etc.)
2. `config.Load(path)` reads the TOML. Unknown keys are a **hard error** so typos surface immediately. Missing file → defaults (`config.Default()`).
3. `git.Read(cwd)` shells out once for branch/ahead-behind/porcelain status and caches nothing — the statusline runs per-render and relies on git being fast locally.
4. `render.Render` walks `cfg.Lines`, looks up each segment name in `segments.registry`, calls its `Renderer(Ctx) string`, drops empties, and joins with the line's effective separators (supports a cycling list — see `Line.EffectiveSeparators`).

**Segments** (`internal/segments/segments.go`) are the extension point. Each is a `func(Ctx) string`:

- Registered by name in `init()` via `Register("name", renderFn)`.
- Returning `""` means "drop me from my line" — used liberally (no git repo, no PR, nothing running, below threshold, etc.). This is the mechanism for conditional display; do not add boolean flags where an empty return works.
- Colors come from `style(c, key)` which reads `[colors]` from TOML first, then falls back to the `defaultColors` map. Never hardcode `lipgloss.Color(...)`; add a key to `defaultColors` and use `style()`.
- Terminal profile is forced to TrueColor (or Ascii for `--no-color` / `NO_COLOR`) because stdout is always a pipe — lipgloss's auto-detection would strip escapes. Don't revert to auto-detect.

**Adding a segment**:

1. Write `func renderX(c Ctx) string` in `internal/segments/segments.go`.
2. `Register("x", renderX)` in `init()`.
3. Document it in `README.md` and `config.example.toml`.
4. If it needs a tunable, add a field to `config.SegmentOpts` (prefer a flat field over a new table until there's a real reason).

**External-command segments** (`gh`, `docker compose`, `git log`) must tolerate the tool being missing or failing — return `""`, never error. `gh`-backed segments go through `internal/gh.Run`, which caches 30s per `(cwd, branch, args)` in `$TMPDIR/claude-statusline-gh/`. `docker compose` goes through `internal/compose`, cached 10s. If you add a new `gh` segment, reuse `gh.Run` — do not shell out directly.

**Per-session state** lives under `$XDG_CACHE_HOME/claude-statusline/` (or `~/.cache/claude-statusline/`): `burn/<session_id>` tracks high-watermark $/hr, `cache/<session_id>` tracks low-watermark cache hit rate, `pomodoro.json` holds the singleton pomodoro timer. Writes are best-effort (ignore IO errors).

**Subcommands** in `main.go`:

- `claude-statusline pomo {start [minutes]|stop|status}` — manages `pomodoro.json`.
- `claude-statusline endpoint {add [LABEL=]URL | rm LABEL_OR_URL | list | clear | path}` — edits the nearest `.endpoint` file walking up from cwd. See `internal/endpoints` for the file format; comments/blanks are preserved on save.

Anything other than `pomo`/`endpoint` as `os.Args[1]` falls through to the default render path.

## Conventions

- Go 1.24. Only three direct deps: `BurntSushi/toml`, `charmbracelet/lipgloss`, `muesli/termenv`. Don't add more without a clear reason.
- Package-per-concern under `internal/` — `input`, `config`, `git`, `gh`, `compose`, `endpoints`, `render`, `segments`. Keep `segments` as the only package that imports all the others; other packages should not import each other horizontally.
- `Ctx` is the single argument threaded through every renderer (`In *input.Input`, `Cfg *config.Config`, `Git *git.Status`). If a renderer needs something new, add it to `Ctx` rather than passing extra args.
- Unknown TOML keys are rejected — keep `config.Load` strict, and update `config.example.toml` whenever you add a field.
