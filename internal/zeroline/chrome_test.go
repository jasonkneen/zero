package zeroline

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTitleBarSegmentsAndWidth(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	h := Header{Cwd: "~/src/shop", Branch: "main", Dirty: true, Model: "claude-sonnet-4-5"}
	out := s.topBar("normal", h, 110, false)
	if lipgloss.Height(out) != 1 {
		t.Fatalf("title bar must be one row, got %d", lipgloss.Height(out))
	}
	if lipgloss.Width(out) != 110 {
		t.Fatalf("title bar width = %d, want 110 (full width)", lipgloss.Width(out))
	}
	plain := stripANSI(out)
	for _, want := range []string{"zero", "model", "claude-sonnet-4-5", "~/src/shop", "TEXT", "JSON", "main"} {
		if !strings.Contains(plain, want) {
			t.Errorf("title bar missing %q in %q", want, plain)
		}
	}
}

func TestTitleBarHidesCwdWhenTight(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	h := Header{Cwd: "~/src/shop", Branch: "main", Model: "m"}
	out := s.topBar("normal", h, 48, false)
	if lipgloss.Width(out) != 48 {
		t.Fatalf("tight title bar width = %d, want 48", lipgloss.Width(out))
	}
	if strings.Contains(stripANSI(out), "~/src/shop") {
		t.Errorf("cwd must be hidden at tight width, got %q", stripANSI(out))
	}
}

func TestStatusBarSegmentsAndMode(t *testing.T) {
	s := newStyles(Resolve(0, true), 0, true)
	h := Header{CtxPct: 42, Cost: 0.04, TotalTokens: 1284}
	out := s.botBar("work", h, 0, 12, 110)
	if lipgloss.Height(out) != 1 {
		t.Fatalf("status bar must be one row, got %d", lipgloss.Height(out))
	}
	if lipgloss.Width(out) != 110 {
		t.Fatalf("status bar width = %d, want 110 (full width)", lipgloss.Width(out))
	}
	plain := stripANSI(out)
	for _, want := range []string{"WORKING", "tok/s", "tok ", "ctx", "$"} {
		if !strings.Contains(plain, want) {
			t.Errorf("status bar missing %q in %q", want, plain)
		}
	}
	if got := stripANSI(s.botBar("done", h, 0, 0, 110)); !strings.Contains(got, "DONE") {
		t.Errorf("status bar should show DONE for run=done, got %q", got)
	}
	if got := stripANSI(s.botBar("blocked", h, 0, 0, 110)); !strings.Contains(got, "BLOCKED") {
		t.Errorf("status bar should show BLOCKED for run=blocked, got %q", got)
	}
}
