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
	if len(lines) != 2 {
		t.Fatalf("renderHeader must return 2 lines, got %d: %q", len(lines), out)
	}
	line2 := lines[1]

	if want := headerCountStyle.Render("3"); !strings.Contains(line2, want) {
		t.Errorf("line2 missing colorized count %q: %q", stripANSI(want), stripANSI(line2))
	}
	if want := badgeModifiedStyle.Render("[M]2"); !strings.Contains(line2, want) {
		t.Errorf("line2 missing [M]2 badge: %q", stripANSI(line2))
	}
	if want := badgeUntrackedStyle.Render("[U]1"); !strings.Contains(line2, want) {
		t.Errorf("line2 missing [U]1 badge: %q", stripANSI(line2))
	}
	if strings.Contains(stripANSI(line2), "[D]") {
		t.Errorf("zero-count [D] badge must be hidden: %q", stripANSI(line2))
	}

	const w = 80 - 4
	if got := lipgloss.Width(line2); got != w {
		t.Errorf("right-aligned line2 width = %d, want %d: %q", got, w, stripANSI(line2))
	}
	stripped := stripANSI(line2)
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
	if len(lines) != 2 {
		t.Fatalf("renderHeader must return 2 lines, got %d: %q", len(lines), out)
	}
	line2 := lines[1]

	if want := headerCleanStyle.Render("clean"); !strings.Contains(line2, want) {
		t.Errorf("clean repo line2 missing green clean marker: %q", stripANSI(line2))
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
	line1 := strings.Split(out, "\n")[0]
	stripped := stripANSI(line1)

	if strings.Contains(line1, "|  |") {
		t.Errorf("dangling double separator in line1: %q", stripped)
	}
	if strings.HasPrefix(stripped, "|") || strings.HasSuffix(stripped, "|") {
		t.Errorf("leading/trailing separator in line1: %q", stripped)
	}
	if n := strings.Count(stripped, "|"); n != 0 {
		t.Errorf("single-segment line1 must have no separator, got %d: %q", n, stripped)
	}

	// Two segments (branch set, user empty): exactly one separator, branch last,
	// no trailing separator after the final segment.
	m.branch = "main"
	out = m.renderHeader()
	line1 = strings.Split(out, "\n")[0]
	stripped = stripANSI(line1)
	if n := strings.Count(stripped, "|"); n != 1 {
		t.Errorf("two-segment line1 must have exactly one separator, got %d: %q", n, stripped)
	}
	if !strings.HasSuffix(stripped, "main") {
		t.Errorf("branch must be the last segment without dangling separator: %q", stripped)
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
