package zeroline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The empty state renders INSIDE the chat chrome (title bar + status bar +
// composer), with the 0 mark + tagline + model hint + chips centered in the body.
func TestEmptyStateShowsChromeAndContent(t *testing.T) {
	d := ChatData{
		Variant: 0, Dark: true, Width: 90, Height: 30,
		Header: Header{Model: "claude-sonnet-4-5", Cwd: "~/code/zero"},
		Chips:  []string{"Add a --version flag", "Why is go vet failing?", "Create hello.txt"},
	}
	out := RenderChat(d)
	if h := lipgloss.Height(out); h != 30 {
		t.Fatalf("empty-state height = %d, want 30 (frame-exact)", h)
	}
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 90 {
			t.Fatalf("line exceeds width 90: %d (%q)", lipgloss.Width(line), stripANSI(line))
		}
	}
	plain := stripANSI(out)
	for _, want := range []string{
		"zero",                                                     // title bar brand
		"std-lib-first", "running", "against", "claude-sonnet-4-5", // empty-state hint
		"Add a --version flag", "Why is go vet failing?", "Create hello.txt", // chips
		"tok", // status bar present (chrome, not a bare Zen page)
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("empty state missing %q", want)
		}
	}
}

// Chip selection/border behavior is covered by TestChipBoxBorderedAndSelected.
