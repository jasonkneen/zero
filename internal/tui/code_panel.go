package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Gitlawb/zero/internal/tools"
)

// codePanelMaxLines caps how many diff rows the Code card shows; it keeps the
// most recent window (the live tail of the edit) and notes how many earlier
// rows were elided.
const codePanelMaxLines = 14

// codePanelActive reports whether the floating Code card should be drawn: only
// while a run is in flight (shared right-column gate) AND that run has produced
// at least one file edit whose diff we can show live.
func (m model) codePanelActive() bool {
	if !m.rightColumnBase() {
		return false
	}
	_, diff := m.currentEditDiff()
	return diff != ""
}

// currentEditDiff returns the path and unified diff of the most recent file
// edit in the active run, or ("", "") when the run has not edited anything yet.
// It is the data behind the live Code card and the reason inline edit cards
// collapse while a run is in flight (the full diff lives in the card instead).
func (m model) currentEditDiff() (string, string) {
	if m.activeRunID == 0 {
		return "", ""
	}
	for index := len(m.transcript) - 1; index >= 0; index-- {
		row := m.transcript[index]
		if row.runID != m.activeRunID {
			continue
		}
		if row.kind != rowToolResult || !toolCardAlwaysExpands(row.tool) || row.status == tools.StatusError {
			continue
		}
		diff := strings.TrimSpace(row.detail)
		if diff == "" {
			continue
		}
		return diffPath(diff), diff
	}
	return "", ""
}

// diffPath pulls the edited file's path out of a unified diff (the "+++ b/path"
// line, falling back to the "diff --git" header), or "" when neither is present.
func diffPath(diff string) string {
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			path = strings.TrimPrefix(path, "b/")
			if path != "" && path != "/dev/null" {
				return path
			}
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strings.TrimPrefix(fields[len(fields)-1], "b/")
			}
		}
	}
	return ""
}

// diffCounts tallies added/removed lines in a unified diff, ignoring the
// +++/--- file headers so they are not miscounted as content.
func diffCounts(diff string) (int, int) {
	adds, dels := 0, 0
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// File headers, not content.
		case strings.HasPrefix(line, "+"):
			adds++
		case strings.HasPrefix(line, "-"):
			dels++
		}
	}
	return adds, dels
}

// renderCodeCard draws the live "Code" card: the edited file's path with its
// +adds/−dels tally, then the diff body painted green (additions) / red
// (removals) — the same palette as the inline diff cards, sized to the widget.
func (m model) renderCodeCard(path, diff string) string {
	inner := planPanelWidth - 4

	adds, dels := diffCounts(diff)
	head := zeroTheme.accent.Bold(true).Render("Code")
	if adds > 0 {
		head += "  " + zeroTheme.diffAdd.Render(fmt.Sprintf("+%d", adds))
	}
	if dels > 0 {
		head += "  " + zeroTheme.diffDel.Render(fmt.Sprintf("−%d", dels))
	}

	if path == "" {
		path = "current file"
	}
	body := []string{
		head,
		zeroTheme.faint.Render(cutRunesEllipsis(path, inner)),
		"",
	}
	body = append(body, codeDiffLines(diff, inner)...)
	return framePlanPanel(body)
}

// codeDiffLines renders a unified diff as styled bands for the Code card: "+"
// rows green, "−" rows red, hunk headers faint, context dimmed. The file-header
// noise is dropped (the path is already in the card head). Only the most recent
// codePanelMaxLines rows are kept so the card tracks the live tail of the edit.
func codeDiffLines(diff string, inner int) []string {
	raw := strings.Split(diff, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"),
			strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "index "):
			// File-header noise: the path is shown in the card head instead.
		case strings.HasPrefix(line, "@@"):
			lines = append(lines, zeroTheme.diffMeta.Render(cutRunesEllipsis(line, inner)))
		case strings.HasPrefix(line, "+"):
			lines = append(lines, codeBand(zeroTheme.addLine, "+", strings.TrimPrefix(line, "+"), inner))
		case strings.HasPrefix(line, "-"):
			lines = append(lines, codeBand(zeroTheme.delLine, "−", strings.TrimPrefix(line, "-"), inner))
		default:
			text := strings.TrimPrefix(line, " ")
			lines = append(lines, zeroTheme.faint.Render(cutRunesEllipsis("  "+text, inner)))
		}
	}
	if len(lines) > codePanelMaxLines {
		hidden := len(lines) - codePanelMaxLines
		lines = lines[len(lines)-codePanelMaxLines:]
		header := zeroTheme.faint.Render(fmt.Sprintf("… %d earlier lines", hidden))
		lines = append([]string{header}, lines...)
	}
	return lines
}

// codeBand paints one changed diff row as a solid colored band: "<sign> text"
// padded to the full inner width so the add/del tint reads edge to edge.
func codeBand(style lipgloss.Style, sign, text string, inner int) string {
	content := cutRunesEllipsis(sign+" "+text, inner)
	if pad := inner - lipgloss.Width(content); pad > 0 {
		content += strings.Repeat(" ", pad)
	}
	return style.Render(content)
}
