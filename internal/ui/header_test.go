package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"tgit/internal/git"
)

// forceANSI256 pins the color profile for color-sensitive assertions and
// restores the previous profile on exit so other tests are unaffected.
func forceANSI256(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func TestHeaderDirtySummaryColoredAndRightAligned(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.toplevel = "/opt/proj-x"
	m.branch = "main"
	m.userName = "u"
	m.changes = []git.FileChange{
		{Path: "a.go", Status: 'M'},
		{Path: "b.go", Status: 'M'},
		{Path: "c.txt", Status: 'U'},
	}

	out := m.renderHeader()
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("renderHeader must return 3 lines, got %d: %q", len(lines), out)
	}
	summaryLine := lines[2]

	if want := headerCountStyle.Render("3"); !strings.Contains(summaryLine, want) {
		t.Errorf("summary line missing colorized count %q: %q", stripANSI(want), stripANSI(summaryLine))
	}
	if want := badgeModifiedStyle.Render("[M]2"); !strings.Contains(summaryLine, want) {
		t.Errorf("summary line missing [M]2 badge: %q", stripANSI(summaryLine))
	}
	if want := badgeUntrackedStyle.Render("[U]1"); !strings.Contains(summaryLine, want) {
		t.Errorf("summary line missing [U]1 badge: %q", stripANSI(summaryLine))
	}
	if strings.Contains(stripANSI(summaryLine), "[D]") {
		t.Errorf("zero-count [D] badge must be hidden: %q", stripANSI(summaryLine))
	}

	const w = 80 - 4
	if got := lipgloss.Width(summaryLine); got != w {
		t.Errorf("right-aligned summary line width = %d, want %d: %q", got, w, stripANSI(summaryLine))
	}
	stripped := stripANSI(summaryLine)
	if !strings.Contains(stripped, "  ") {
		t.Errorf("no right-align gap found between left summary and badges: %q", stripped)
	}
	if !strings.HasSuffix(stripped, "[U]1") {
		t.Errorf("badges should end at content column %d, suffix=%q", w, stripped)
	}
}

func TestHeaderCleanShowsGreenClean(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.toplevel = "/opt/proj-x"
	m.branch = "main"
	m.userName = "u"
	m.changes = nil

	out := m.renderHeader()
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("renderHeader must return 3 lines, got %d: %q", len(lines), out)
	}
	summaryLine := lines[2]

	if want := headerCleanStyle.Render("clean"); !strings.Contains(summaryLine, want) {
		t.Errorf("clean repo summary line missing green clean marker: %q", stripANSI(summaryLine))
	}
	stripped := stripANSI(out)
	if strings.Contains(stripped, "[M]") {
		t.Errorf("clean repo must not show [M] badge: %q", stripped)
	}
	if strings.Contains(stripped, "changed") {
		t.Errorf("clean repo must not show change count text: %q", stripped)
	}
}

func TestHeaderSkipsEmptySegments(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.toplevel = "/opt/proj-x"
	m.branch = ""
	m.userName = ""

	out := m.renderHeader()
	segLine := strings.Split(out, "\n")[1]
	stripped := stripANSI(segLine)

	if strings.Contains(segLine, "|  |") {
		t.Errorf("dangling double separator in branch/user line: %q", stripped)
	}
	if strings.HasPrefix(stripped, "|") || strings.HasSuffix(stripped, "|") {
		t.Errorf("leading/trailing separator in branch/user line: %q", stripped)
	}
	if n := strings.Count(stripped, "|"); n != 0 {
		t.Errorf("single-segment line must have no separator, got %d: %q", n, stripped)
	}

	// Two segments (branch + user): exactly one separator, user last,
	// no trailing separator after the final segment.
	m.branch = "main"
	m.userName = "u"
	out = m.renderHeader()
	segLine = strings.Split(out, "\n")[1]
	stripped = stripANSI(segLine)
	if n := strings.Count(stripped, "|"); n != 1 {
		t.Errorf("two-segment line must have exactly one separator, got %d: %q", n, stripped)
	}
	if !strings.HasSuffix(stripped, "main | u") {
		t.Errorf("segments must join as %q without dangling separator: %q", "main | u", stripped)
	}
}

func TestHeaderLinesNeverExceedWidth(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.toplevel = "/" + strings.Repeat("x", 200)
	m.branch = "main"
	m.userName = "u"
	m.changes = []git.FileChange{
		{Path: "a.go", Status: 'M'},
		{Path: "b.go", Status: 'D'},
	}

	const w = 80 - 4
	header := m.renderHeader()
	for i, line := range strings.Split(header, "\n") {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("header line %d width %d exceeds content width %d: %q", i, got, w, stripANSI(line))
		}
	}

	// Full-view guard: header must not wrap-drift the fixed 24-line layout.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := updated.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 24 {
		t.Fatalf("view with long-path header must keep exactly 24 lines, got %d", len(lines))
	}
}
