package tools

import (
	"strings"
	"testing"
)

func TestUnifiedDiffNewFile(t *testing.T) {
	got := UnifiedDiff("hello.txt", "", "hi\nthere")
	want := "+hi\n+there"
	if got != want {
		t.Fatalf("new-file diff = %q, want %q", got, want)
	}
}

func TestUnifiedDiffEdit(t *testing.T) {
	before := "line1\nline2\nline3"
	after := "line1\nline2-changed\nline3"
	got := UnifiedDiff("f", before, after)
	for _, want := range []string{" line1", "-line2", "+line2-changed", " line3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("edit diff missing %q in:\n%s", want, got)
		}
	}
}

func TestUnifiedDiffInsertion(t *testing.T) {
	got := UnifiedDiff("f", "a\nb", "a\nx\nb")
	want := " a\n+x\n b"
	if got != want {
		t.Fatalf("insertion diff = %q, want %q", got, want)
	}
}

func TestUnifiedDiffDeletion(t *testing.T) {
	got := UnifiedDiff("f", "a\nb\nc", "a\nc")
	want := " a\n-b\n c"
	if got != want {
		t.Fatalf("deletion diff = %q, want %q", got, want)
	}
}

func TestUnifiedDiffEmpty(t *testing.T) {
	if got := UnifiedDiff("f", "", ""); got != "" {
		t.Fatalf("empty diff = %q, want empty", got)
	}
}

func TestDiffStat(t *testing.T) {
	add, del := DiffStat(" a\n+x\n+y\n-b\n c")
	if add != 2 || del != 1 {
		t.Fatalf("DiffStat = (+%d -%d), want (+2 -1)", add, del)
	}
}
