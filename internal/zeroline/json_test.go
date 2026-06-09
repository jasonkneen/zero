package zeroline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestJSONModeRendersEvents(t *testing.T) {
	d := chatWith([]Row{
		{Kind: "user", Text: "add a --version flag"},
		{Kind: "tool", Tool: "edit_file", Text: "cli/root.go", Status: "ok", Detail: "+x"},
		{Kind: "final", Text: "Done."},
		{Kind: "done", Text: "1 tool", Status: "ok"},
	})
	d.JSONMode = true
	out := stripANSI(RenderChat(d))
	for _, want := range []string{
		`$ `, `zero run`, `--format json`,
		`"type"`, `"user"`, `"tool_use"`, `"edit_file"`, `"result"`, `"done"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON view missing %q", want)
		}
	}
	// the transcript card border must NOT appear in JSON mode
	if strings.Contains(out, "╭") {
		t.Error("JSON mode should not render tool-card borders")
	}
}

func TestJSONModeTitleToggle(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	h := Header{Model: "m"}
	// In text mode the toggle highlights TEXT; in JSON mode it highlights JSON.
	// Both labels are always present; assert the bar still renders one full row.
	for _, jm := range []bool{false, true} {
		out := s.topBar("normal", h, 100, jm)
		if lipgloss.Height(out) != 1 || lipgloss.Width(out) != 100 {
			t.Fatalf("title bar not 1x100 (jsonMode=%v): %dx%d", jm, lipgloss.Width(out), lipgloss.Height(out))
		}
		plain := stripANSI(out)
		if !strings.Contains(plain, "TEXT") || !strings.Contains(plain, "JSON") {
			t.Errorf("toggle missing TEXT/JSON (jsonMode=%v): %q", jm, plain)
		}
	}
}

func TestJSONModeFrameExact(t *testing.T) {
	d := chatWith([]Row{{Kind: "user", Text: "hi"}, {Kind: "assistant", Text: "ok"}})
	d.JSONMode = true
	d.Width, d.Height = 100, 20
	out := RenderChat(d)
	if h := lipgloss.Height(out); h != 20 {
		t.Fatalf("JSON frame height = %d, want 20", h)
	}
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 100 {
			t.Fatalf("JSON line exceeds width 100: %d", lipgloss.Width(line))
		}
	}
}
