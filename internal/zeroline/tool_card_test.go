package zeroline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestToolCardDiff(t *testing.T) {
	diff := "+added line\n-removed line\n unchanged"
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "tool", Tool: "edit_file", Text: "internal/cli/root.go", Detail: diff, Status: "ok"}})))
	if !strings.Contains(out, "╭") || !strings.Contains(out, "╰") {
		t.Error("tool card missing rounded border")
	}
	for _, w := range []string{"edit_file", "internal/cli/root.go", "✓", "added line", "removed line"} {
		if !strings.Contains(out, w) {
			t.Errorf("diff card missing %q", w)
		}
	}
}

func TestToolCardBash(t *testing.T) {
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "tool", Tool: "bash", Text: "go test ./...", Detail: "ok  pkg  0.2s", Status: "ok"}})))
	for _, w := range []string{"bash", "$ go test", "exit 0"} {
		if !strings.Contains(out, w) {
			t.Errorf("bash card missing %q in %q", w, out)
		}
	}
}

func TestToolCardBashError(t *testing.T) {
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "tool", Tool: "bash", Text: "go vet", Detail: "vet: bad", Status: "error"}})))
	if !strings.Contains(out, "✗") || !strings.Contains(out, "exit 1") {
		t.Errorf("error bash card missing ✗/exit 1: %q", out)
	}
}

func TestToolCardRunningSpinner(t *testing.T) {
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "tool", Tool: "grep", Text: "pattern", Running: true}})))
	found := false
	for _, f := range spinFrames {
		if strings.Contains(out, f) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("running card missing spinner: %q", out)
	}
}

func TestToolCardGrep(t *testing.T) {
	body := "internal/a.go:41:foo match\ninternal/b.go:12:bar match"
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "tool", Tool: "grep", Text: ".", Detail: body, Status: "ok"}})))
	for _, w := range []string{"a.go", "foo match", "2 matches"} {
		if !strings.Contains(out, w) {
			t.Errorf("grep card missing %q in %q", w, out)
		}
	}
}

func TestToolCardGrepTabsNoOverflow(t *testing.T) {
	body := "a.go:1:if\tx\t{\treturn\tnil}\nb.go:2:foo\tbar\tbaz"
	d := chatWith([]Row{{Kind: "tool", Tool: "grep", Text: ".", Detail: body, Status: "ok"}})
	d.Width, d.Height = 60, 16
	out := RenderChat(d)
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("grep line with tabs overflows width 60: %d (%q)", lipgloss.Width(line), stripANSI(line))
		}
	}
}

func TestToolCardLongNameNoHeadOverflow(t *testing.T) {
	long := "mcp__some_very_long_server__some_very_long_tool_name_indeed"
	d := chatWith([]Row{{Kind: "tool", Tool: long, Text: "and/a/long/target/path/to/some/file.go", Status: "ok", Detail: "x"}})
	d.Width, d.Height = 70, 14
	out := RenderChat(d)
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 70 {
			t.Fatalf("long tool name overflows card head: %d (%q)", lipgloss.Width(line), stripANSI(line))
		}
	}
}

func TestDiffBodyStatSkipsHeaders(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	diff := "--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new"
	out := stripANSI(strings.Join(s.diffBody(diff, 60), "\n"))
	if !strings.Contains(out, "+1") || !strings.Contains(out, "-1") {
		t.Fatalf("stat should be +1/-1 with diff headers excluded, got: %q", out)
	}
}

func TestToolCardFrameExact(t *testing.T) {
	d := chatWith([]Row{{Kind: "tool", Tool: "edit_file", Text: "f.go", Detail: "+x\n-y\n z", Status: "ok"}})
	d.Width, d.Height = 100, 26
	out := RenderChat(d)
	if h := lipgloss.Height(out); h != 26 {
		t.Fatalf("chat height = %d, want 26 (frame-exact)", h)
	}
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 100 {
			t.Fatalf("tool-card line exceeds width 100: %d", lipgloss.Width(line))
		}
	}
}
