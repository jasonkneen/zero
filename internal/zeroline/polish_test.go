package zeroline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// FIX 1: .blk-say is plain MUTED prose with no panel/background; only .blk-final
// keeps the accent rail + ink. Glamour formatting would drop the backticks of
// inline code, so their survival proves the prose is rendered plainly.
func TestAssistantSayIsPlainMuted(t *testing.T) {
	out := stripANSI(RenderChat(chatWith([]Row{{Kind: "assistant", Text: "use the `flag` package here"}})))
	if !strings.Contains(out, "`flag`") {
		t.Errorf("assistant prose must render plainly (keep backticks), got: %q", out)
	}
	final := stripANSI(RenderChat(chatWith([]Row{{Kind: "final", Text: "All set."}})))
	if !strings.Contains(final, "│ All set.") {
		t.Errorf("final block must keep the accent rail + ink text, got: %q", final)
	}
}

// FIX 2: each home suggestion is its own bordered rounded box; the selected one
// uses the accent (rendered as a thick border so it's visible without color).
func TestChipBoxBorderedAndSelected(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	un := stripANSI(s.chipBox("hello world", false, 50))
	sel := stripANSI(s.chipBox("hello world", true, 50))

	if !strings.Contains(un, "╭") || !strings.Contains(un, "╰") {
		t.Errorf("unselected chip must be a rounded box, got: %q", un)
	}
	if !strings.Contains(un, "❯") || !strings.Contains(un, "hello world") {
		t.Errorf("chip must carry the accent arrow + label, got: %q", un)
	}
	if lipgloss.Width(strings.Split(un, "\n")[0]) != 50 {
		t.Errorf("chip box must be exactly w wide, got %d", lipgloss.Width(strings.Split(un, "\n")[0]))
	}
	if !strings.Contains(sel, "┏") {
		t.Errorf("selected chip must use the accent (thick) border, got: %q", sel)
	}
	if strings.Contains(un, "┏") {
		t.Errorf("unselected chip must be rounded, not thick, got: %q", un)
	}
}

// The read_file tool already numbers its lines ("N | …") under a "File: … (N
// lines)" header, so the read card must NOT add a second number column or repeat
// the header.
func TestReadCardNoDoubleNumbering(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	detail := "File: x.py (132 lines)\n\n  1 | import random\n  2 | import time"
	body := s.toolBody(Row{Kind: "tool", Tool: "read_file", Detail: detail}, 80)
	joined := stripANSI(strings.Join(body, "\n"))
	if strings.Contains(joined, "File:") {
		t.Errorf("read card should drop the redundant File: header: %q", joined)
	}
	if len(body) == 0 || stripANSI(body[0]) != "  1 | import random" {
		t.Errorf("read card should show the tool's own numbering once, got: %q", joined)
	}
}

// Resolved-permission line is colored by outcome (.perm-resolved): allow/always
// green ✓, deny red ✗, pending amber ⚠.
func TestPermissionLineColoredByOutcome(t *testing.T) {
	allow := stripANSI(RenderChat(chatWith([]Row{{Kind: "permission", Text: "permission: write_file allow risk:medium"}})))
	if !strings.Contains(allow, "✓") || strings.Contains(allow, "⚠") {
		t.Errorf("allowed permission should show ✓ (not ⚠): %q", allow)
	}
	deny := stripANSI(RenderChat(chatWith([]Row{{Kind: "permission", Text: "permission: bash deny risk:high"}})))
	if !strings.Contains(deny, "✗") {
		t.Errorf("denied permission should show ✗: %q", deny)
	}
	pend := stripANSI(RenderChat(chatWith([]Row{{Kind: "permission", Text: "permission: bash prompt risk:high"}})))
	if !strings.Contains(pend, "⚠") {
		t.Errorf("pending permission should show ⚠: %q", pend)
	}
}

func TestHomeChipsAreSeparateBorderedBoxes(t *testing.T) {
	d := ChatData{
		Variant: 0, Dark: true, Width: 100, Height: 34, Header: Header{Model: "m"},
		Chips: []string{"alpha", "beta", "gamma"}, ChipIndex: 1,
	}
	out := RenderChat(d)
	if h := lipgloss.Height(out); h != 34 {
		t.Fatalf("empty-state height = %d, want 34 (frame-exact)", h)
	}
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 100 {
			t.Fatalf("home line exceeds width 100: %d", lipgloss.Width(line))
		}
	}
	plain := stripANSI(out)
	for _, c := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(plain, "❯ "+c) {
			t.Errorf("chip %q missing arrow+label", c)
		}
	}
	// selected chip (index 1) renders the accent thick border
	if !strings.Contains(plain, "┏") {
		t.Error("home should show the selected chip's accent border")
	}
}
