package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tgit/internal/git"
)

func TestBottomPaneScrollsWindow(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 30; i++ {
		m.commits = append(m.commits, git.CommitRow{
			Hash: fmt.Sprintf("h%02d", i),
			Msg:  fmt.Sprintf("commit number %02d", i),
		})
	}
	m.selectedCommit = 29
	m.focusedPane = focusCommits
	m.currentView = "files"

	// Direct renderer check: window must follow the selection.
	out := m.renderBottomSized(7)
	lines := strings.Split(out, "\n")
	if len(lines) != 7 {
		t.Fatalf("bottom pane must render exactly visibleRows lines, got %d", len(lines))
	}
	if !strings.Contains(stripANSI(out), "commit number 29") {
		t.Fatalf("selected commit #29 must be visible in the scrolled window:\n%s", stripANSI(out))
	}
	if !strings.Contains(lines[6], "\x1b[7m") {
		t.Errorf("last window row (selected #29) must carry the selection highlight: %q", stripANSI(lines[6]))
	}

	// Full-view check through Update/View.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := updated.View()
	if !strings.Contains(stripANSI(v), "commit number 29") {
		t.Fatalf("full view must show selected commit #29 in bottom pane")
	}
}

func TestLoadMoreAppendsOlderCommits(t *testing.T) {
	newModel := func() *model {
		m := InitialModel()
		m.width, m.height = 80, 24
		m.commits = []git.CommitRow{
			{Hash: "aaa1111", Msg: "loaded newest"},
			{Hash: "bbb2222", Msg: "loaded second"},
		}
		m.selectedCommit = 1
		m.focusedPane = focusCommits
		m.currentView = "files"
		return m
	}

	m := newModel()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatal("j past last loaded commit with history remaining must return a loadMore cmd")
	}
	got := updated.(*model)
	if got.selectedCommit != 1 {
		t.Errorf("selection must stay clamped at last index until page arrives, got %d", got.selectedCommit)
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("loadMore cmd returned nil msg")
	}
	after, _ := got.Update(msg)
	m2 := after.(*model)
	appended := len(m2.commits) - 2
	if appended <= 0 {
		t.Fatalf("commits must grow after load-more msg, before=2 after=%d", len(m2.commits))
	}
	if m2.selectedCommit != 1+appended {
		t.Errorf("selection must advance by appended rows: want %d, got %d", 1+appended, m2.selectedCommit)
	}
	cam := msg.(commitsAppendedMsg)
	if want := len(cam.rows) < 200 || cam.exhausted; m2.commitsExhausted != want {
		t.Errorf("commitsExhausted = %v, want %v (raw page=%d)", m2.commitsExhausted, want, len(cam.rows))
	}

	// Exhausted: j wraps instead of returning a cmd.
	me := newModel()
	me.commitsExhausted = true
	_, cmdE := me.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmdE != nil {
		t.Fatal("j on exhausted history must wrap in place, not return a loadMore cmd")
	}
	if me.selectedCommit != 0 {
		t.Errorf("exhausted j should wrap to first commit, got %d", me.selectedCommit)
	}
}
