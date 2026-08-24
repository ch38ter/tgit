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
	if !containsReverseSGR(lines[6]) {
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

	// Exhausted: j clamps at last index instead of returning a cmd.
	me := newModel()
	me.commitsExhausted = true
	_, cmdE := me.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmdE != nil {
		t.Fatal("j on exhausted history must clamp in place, not return a loadMore cmd")
	}
	if me.selectedCommit != 1 {
		t.Errorf("exhausted j should stay clamped at last index, got %d", me.selectedCommit)
	}
}

func TestCommitSelectionClampsAtEnds(t *testing.T) {
	newModel := func() *model {
		m := InitialModel()
		m.width, m.height = 80, 24
		m.commits = []git.CommitRow{
			{Hash: "aaa1111", Msg: "loaded newest"},
			{Hash: "bbb2222", Msg: "loaded second"},
		}
		m.focusedPane = focusCommits
		m.currentView = "files"
		return m
	}
	last := 1 // len(commits)-1

	// k at index 0 must stay at 0.
	m := newModel()
	m.selectedCommit = 0
	up, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if got := up.(*model).selectedCommit; got != 0 {
		t.Errorf("k at first commit must stay at 0, got %d", got)
	}

	// tea.KeyUp at index 0 must stay at 0.
	m = newModel()
	m.selectedCommit = 0
	up2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := up2.(*model).selectedCommit; got != 0 {
		t.Errorf("up at first commit must stay at 0, got %d", got)
	}

	// j at last index with exhausted history must stay at last index.
	m = newModel()
	m.selectedCommit = last
	m.commitsExhausted = true
	dn, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if got := dn.(*model).selectedCommit; got != last {
		t.Errorf("j at last commit with exhausted history must stay at %d, got %d", last, got)
	}

	// tea.KeyDown at last index with exhausted history must stay at last index.
	m = newModel()
	m.selectedCommit = last
	m.commitsExhausted = true
	dn2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := dn2.(*model).selectedCommit; got != last {
		t.Errorf("down at last commit with exhausted history must stay at %d, got %d", last, got)
	}

	// j at last index with history remaining must trigger load-more and keep selection clamped.
	m = newModel()
	m.selectedCommit = last
	dn3, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd == nil {
		t.Fatal("j past last loaded commit with history remaining must return a loadMore cmd")
	}
	if got := dn3.(*model).selectedCommit; got != last {
		t.Errorf("selection must stay clamped at %d until page arrives, got %d", last, got)
	}
}

func TestStickyScrollWindow(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 30; i++ {
		m.commits = append(m.commits, git.CommitRow{
			Hash: fmt.Sprintf("h%02d", i),
			Msg:  fmt.Sprintf("commit number %02d", i),
		})
	}
	m.focusedPane = focusCommits
	m.currentView = "files"

	// Rendering at the bottom pins the window start at len-visibleRows.
	m.selectedCommit = 29
	out := m.renderBottomSized(7)
	if m.commitScroll != 23 {
		t.Fatalf("render at index 29 must pin window start at 23, got %d", m.commitScroll)
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "commit number 23") || !strings.Contains(plain, "commit number 29") {
		t.Fatalf("window must show rows 23..29:\n%s", plain)
	}

	// step presses one movement key and drives a render, mirroring the
	// Update -> View cycle where the sticky offset is enforced.
	pressAndRender := func(key rune) {
		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = upd.(*model)
		m.renderBottomSized(7)
	}

	// k inside the window moves the selection but never the window.
	for want := 28; want >= 23; want-- {
		pressAndRender('k')
		if m.selectedCommit != want {
			t.Fatalf("k must move selection to %d, got %d", want, m.selectedCommit)
		}
		if m.commitScroll != 23 {
			t.Fatalf("selection %d inside window must keep start 23, got %d", m.selectedCommit, m.commitScroll)
		}
	}

	// Crossing the top edge slides the window with the selection.
	pressAndRender('k')
	if m.selectedCommit != 22 || m.commitScroll != 22 {
		t.Fatalf("k past top edge: want selection 22/start 22, got %d/%d", m.selectedCommit, m.commitScroll)
	}

	// j back inside the window leaves the start alone...
	pressAndRender('j')
	if m.selectedCommit != 23 || m.commitScroll != 22 {
		t.Fatalf("j within window: want selection 23/start 22, got %d/%d", m.selectedCommit, m.commitScroll)
	}

	// ...until the selection crosses the window's bottom edge (row 28).
	for want := 24; want <= 28; want++ {
		pressAndRender('j')
		if m.selectedCommit != want || m.commitScroll != 22 {
			t.Fatalf("j to %d must keep start 22, got selection %d/start %d", want, m.selectedCommit, m.commitScroll)
		}
	}
	pressAndRender('j')
	if m.selectedCommit != 29 || m.commitScroll != 23 {
		t.Fatalf("j past bottom edge: want selection 29/start 23, got %d/%d", m.selectedCommit, m.commitScroll)
	}
}

func TestCommitScrollPreservedAcrossPagination(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 30; i++ {
		m.commits = append(m.commits, git.CommitRow{
			Hash:     fmt.Sprintf("h%02d", i),
			FullHash: fmt.Sprintf("f%02d", i),
			Msg:      fmt.Sprintf("commit number %02d", i),
		})
	}
	m.focusedPane = focusCommits
	m.currentView = "files"
	m.selectedCommit = 29
	m.renderBottomSized(7) // pins commitScroll at 23

	msg := commitsAppendedMsg{
		rows: []git.CommitRow{
			{Hash: "h30", FullHash: "f30", Msg: "commit number 30"},
		},
	}
	updated, _ := m.Update(msg)
	m2 := updated.(*model)
	if m2.commitScroll != 23 {
		t.Fatalf("pagination must preserve window start 23, got %d", m2.commitScroll)
	}
	if m2.selectedCommit != 30 {
		t.Fatalf("selection must advance with appended rows, got %d", m2.selectedCommit)
	}
	if out := stripANSI(m2.renderBottomSized(7)); !strings.Contains(out, "commit number 30") {
		t.Fatalf("appended row must be visible after pagination:\n%s", out)
	}
}

func TestCommitScrollResetsOnHistoryReplace(t *testing.T) {
	forceANSI256(t)

	newModel := func() *model {
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
		m.renderBottomSized(7) // pins commitScroll at 23
		return m
	}

	m := newModel()
	upd, _ := m.Update(repoDataMsg{commits: m.commits})
	if got := upd.(*model).commitScroll; got != 0 {
		t.Errorf("repoDataMsg replaces history wholesale: commitScroll must reset to 0, got %d", got)
	}

	m = newModel()
	upd, _ = m.Update(refCommitsMsg{refs: nil, commits: m.commits})
	if got := upd.(*model).commitScroll; got != 0 {
		t.Errorf("refCommitsMsg replaces history wholesale: commitScroll must reset to 0, got %d", got)
	}
}
