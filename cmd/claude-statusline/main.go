package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/natefaerber/claude-statusline/internal/config"
	"github.com/natefaerber/claude-statusline/internal/endpoints"
	"github.com/natefaerber/claude-statusline/internal/input"
	"github.com/natefaerber/claude-statusline/internal/render"
	"github.com/natefaerber/claude-statusline/internal/segments"
)

// Injected by GoReleaser at build time via -ldflags "-X main.version=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("claude-statusline %s (commit %s, built %s)\n", version, commit, date)
			return
		}
	}

	// Subcommand dispatch — only `pomo` for now. Anything else falls through
	// to the default render-statusline behavior.
	if len(os.Args) > 1 && os.Args[1] == "pomo" {
		os.Exit(pomoCmd(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "endpoint" {
		os.Exit(endpointCmd(os.Args[2:]))
	}

	configPath := flag.String("config", config.DefaultPath(), "path to config TOML")
	noColor := flag.Bool("no-color", false, "disable ANSI colors")
	flag.Parse()

	// Statusline subprocesses always have a piped stdout, so lipgloss's TTY
	// auto-detection would strip colors. Force a profile so the host terminal
	// (Claude Code, tmux, etc.) gets the escapes it can render. NO_COLOR /
	// --no-color still wins.
	if *noColor || os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	} else {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-statusline:", err)
		os.Exit(1)
	}

	in, err := input.Parse(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-statusline: parse stdin:", err)
		os.Exit(1)
	}

	fmt.Println(render.Render(in, cfg))
}

// pomoCmd handles `claude-statusline pomo {start [minutes]|stop|status}`.
// Returns the process exit code.
func pomoCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: claude-statusline pomo {start [minutes]|stop|status}")
		return 2
	}
	switch args[0] {
	case "start":
		minutes := 25
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n <= 0 {
				fmt.Fprintln(os.Stderr, "claude-statusline: invalid minutes:", args[1])
				return 2
			}
			minutes = n
		}
		if err := segments.PomodoroStart(minutes); err != nil {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		fmt.Printf("🍅 started %dm pomodoro\n", minutes)
		return 0
	case "stop":
		if err := segments.PomodoroStop(); err != nil {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		fmt.Println("🍅 stopped")
		return 0
	case "status":
		fmt.Println("state file:", segments.PomodoroPath())
		return 0
	default:
		fmt.Fprintln(os.Stderr, "claude-statusline: unknown pomo subcommand:", args[0])
		return 2
	}
}

// endpointCmd handles `claude-statusline endpoint {add|rm|list|clear|path}`.
func endpointCmd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: claude-statusline endpoint {add [LABEL=]URL | rm LABEL_OR_URL | list | clear | path}")
		return 2
	}

	cwd, _ := os.Getwd()

	// Resolve or create the file path.
	filePath := endpoints.Find(cwd)
	if filePath == "" {
		filePath = filepath.Join(cwd, endpoints.Filename)
	}

	switch args[0] {
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: claude-statusline endpoint add [LABEL=]URL")
			return 2
		}
		entries, _ := endpoints.Load(filePath)
		for _, arg := range args[1:] {
			label, url := endpoints.ParseAddArg(arg)
			entries = endpoints.Add(entries, label, url)
		}
		if err := endpoints.Save(filePath, entries); err != nil {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		return 0

	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: claude-statusline endpoint rm LABEL_OR_URL")
			return 2
		}
		entries, _ := endpoints.Load(filePath)
		for _, key := range args[1:] {
			entries = endpoints.Remove(entries, key)
		}
		if err := endpoints.Save(filePath, entries); err != nil {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		return 0

	case "list":
		entries, err := endpoints.Load(filePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		for _, e := range entries {
			if e.URL == "" {
				continue
			}
			if e.Label != "" {
				fmt.Printf("%s=%s\n", e.Label, e.URL)
			} else {
				fmt.Println(e.URL)
			}
		}
		return 0

	case "clear":
		if err := os.Remove(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "claude-statusline:", err)
			return 1
		}
		return 0

	case "path":
		fmt.Println(filePath)
		return 0

	default:
		fmt.Fprintln(os.Stderr, "claude-statusline: unknown endpoint subcommand:", args[0])
		return 2
	}
}
