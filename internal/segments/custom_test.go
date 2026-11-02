package segments

import (
	"strings"
	"testing"

	"github.com/natefaerber/claude-statusline/internal/config"
)

func TestRenderCustomExpandsEnv(t *testing.T) {
	t.Setenv("CS_TEST_LABEL", "nate@example.com (primary)")
	cfg := &config.Config{}
	cfg.Segments.CustomLine = "$CS_TEST_LABEL"
	got := renderCustom(Ctx{Cfg: cfg})
	if !strings.Contains(got, "nate@example.com (primary)") {
		t.Fatalf("custom_line did not expand: %q", got)
	}
	cfg.Segments.CustomLine = "${CS_TEST_UNSET}"
	if got := renderCustom(Ctx{Cfg: cfg}); got != "" {
		t.Fatalf("a line that expands to nothing should be dropped, got %q", got)
	}
	cfg.Segments.CustomLine = "plain text"
	if got := renderCustom(Ctx{Cfg: cfg}); !strings.Contains(got, "plain text") {
		t.Fatalf("plain text lost: %q", got)
	}
}
