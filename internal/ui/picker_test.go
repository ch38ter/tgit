package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tgit/internal/git"
)

func pickerTestModel() *model {
	m := InitialModel()
	m.width, m.height = 80, 24
	return m
}

func pressRune(m *model, r rune) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func pickerRefs() []git.Ref {
	return []git.Ref{
		{Name: "feature", Head: false},
		{Name: "main", Head: true},
		{Name: "origin/feat", Head: false},
		{Name: "origin/main", Head: false},
	}
}

func openPicker(m *model) {
	pressRune(m, 'b')
	m.Update(refsLoadedMsg{refs: pickerRefs()})
}

func TestPickerOpenReplacesCommits(t *testing.T) {
	m := pickerTestModel()
	m.commits = []git.CommitRow{{Hash: "abc1234", FullHash: strings.Repeat("a", 40), Msg: "unique commit message"}}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("opening picker must return a loadRefs cmd")
	}
	if !m.pickerOpen {
		t.Fatal("picker must be open after 'b'")
	}

	m.Update(refsLoadedMsg{refs: pickerRefs()})
	if m.refCursor != 0 {
		t.Fatalf("cursor must start on the (all branches) entry when viewRef is empty, got %d", m.refCursor)
	}

	v := m.View()
	if !strings.Contains(v, "(all branches)") {
		t.Errorf("view must show the leading '(all branches)' entry, got:\n%s", v)
	}
	if !strings.Contains(v, "* main") {
		t.Errorf("view must show HEAD marker row '* main', got:\n%s", v)
	}
	if !strings.Contains(v, "origin/feat") {
		t.Errorf("view must list remote-tracking refs, got:\n%s", v)
	}
	if strings.Contains(v, "unique commit message") {
		t.Error("picker must replace the commit list in the bottom pane")
	}
}

func TestPickerToggleAndEscClose(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if m.pickerOpen {
		t.Fatal("'b' again must close the picker")
	}
	if cmd != nil {
		t.Fatal("closing picker must not return a cmd")
	}

	openPicker(m)
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pickerOpen {
		t.Fatal("esc must close the picker")
	}
}

func TestPickerCursorClampsNoWrap(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)

	for i := 0; i < 10; i++ {
		pressRune(m, 'j')
	}
	if m.refCursor != 4 {
		t.Fatalf("j must clamp at %d, got %d", len(pickerRefs()), m.refCursor)
	}

	for i := 0; i < 10; i++ {
		pressRune(m, 'k')
	}
	if m.refCursor != 0 {
		t.Fatalf("k at first entry must clamp at 0, got %d", m.refCursor)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.refCursor != 1 {
		t.Fatalf("down arrow must move cursor to 1, got %d", m.refCursor)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.refCursor != 0 {
		t.Fatalf("up arrow must move cursor to 0, got %d", m.refCursor)
	}
}

func TestPickerEnterSelectsRef(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)
	pressRune(m, 'j') // cursor -> "feature" (index 1)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickerOpen {
		t.Fatal("enter must close the picker")
	}
	if m.viewRef != "feature" {
		t.Fatalf("enter must set viewRef to selected ref, got %q", m.viewRef)
	}
	if cmd == nil {
		t.Fatal("enter must return a loadCommitsForRef cmd")
	}
	msg := cmd()
	rcm, ok := msg.(refCommitsMsg)
	if !ok {
		t.Fatalf("cmd must produce refCommitsMsg, got %T", msg)
	}
	if rcm.ref != "feature" {
		t.Fatalf("refCommitsMsg.ref = %q, want %q", rcm.ref, "feature")
	}
}

func TestPickerAllBranchesEntrySelected(t *testing.T) {
	m := pickerTestModel()
	m.viewRef = "feature"
	openPicker(m)

	pressRune(m, 'k')
	pressRune(m, 'k') // cursor -> index 0, the (all branches) entry
	if m.refCursor != 0 {
		t.Fatalf("cursor = %d, want 0 (all-branches entry)", m.refCursor)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickerOpen {
		t.Fatal("enter must close the picker")
	}
	if m.viewRef != "" {
		t.Fatalf("selecting the all-branches entry must reset viewRef to empty, got %q", m.viewRef)
	}
	msg := cmd()
	rcm, ok := msg.(refCommitsMsg)
	if !ok {
		t.Fatalf("cmd must produce refCommitsMsg, got %T", msg)
	}
	if rcm.ref != "" {
		t.Fatalf("refCommitsMsg.ref = %q, want empty (all-branches mode)", rcm.ref)
	}

	m.Update(refCommitsMsg{ref: "", commits: []git.CommitRow{
		{Hash: "aaa1111", FullHash: strings.Repeat("a", 40), Msg: "combined"},
	}})
	if len(m.commits) != 1 || m.commits[0].Msg != "combined" {
		t.Fatalf("empty-ref response must be applied when viewRef is empty, commits = %+v", m.commits)
	}
}

func TestPickerCursorStartsAtCurrentViewRef(t *testing.T) {
	m := pickerTestModel()
	m.viewRef = "origin/main"
	openPicker(m)
	if m.refCursor != 4 {
		t.Fatalf("cursor must land on the current viewRef row (4), got %d", m.refCursor)
	}

	m2 := pickerTestModel()
	m2.viewRef = "ghost-branch"
	openPicker(m2)
	if m2.refCursor != 2 {
		t.Fatalf("unmatched viewRef must fall back to the HEAD-marked entry (2), got %d", m2.refCursor)
	}
}

func TestRefCommitsMsgStaleIgnored(t *testing.T) {
	m := pickerTestModel()
	m.viewRef = "main"
	m.commits = []git.CommitRow{{Hash: "old1", FullHash: strings.Repeat("a", 40), Msg: "old"}}

	m.Update(refCommitsMsg{ref: "other", commits: []git.CommitRow{{Hash: "new1"}}})

	if len(m.commits) != 1 || m.commits[0].Hash != "old1" {
		t.Fatalf("stale refCommitsMsg must be discarded, commits = %+v", m.commits)
	}
}

func TestRefCommitsMsgFreshResetsState(t *testing.T) {
	m := pickerTestModel()
	m.viewRef = "main"
	for i := 0; i < 5; i++ {
		m.commits = append(m.commits, git.CommitRow{Hash: "aaa1111", FullHash: strings.Repeat("a", 40), Msg: "x"})
	}
	m.selectedCommit = 4
	m.commitsExhausted = true
	m.laneOpen = [][]string{{"stale-lane"}}

	fresh := []git.CommitRow{
		{Hash: "bbb2222", FullHash: strings.Repeat("b", 40), Msg: "one"},
		{Hash: "ccc3333", FullHash: strings.Repeat("c", 40), Parents: []string{strings.Repeat("b", 40)}, Msg: "two"},
	}
	m.Update(refCommitsMsg{ref: "main", commits: fresh})

	wantCells, wantOpen := buildLanes(fresh, nil)
	if !reflect.DeepEqual(m.graphCells, wantCells) {
		t.Errorf("graphCells must be rebuilt from scratch")
	}
	if !reflect.DeepEqual(m.laneOpen, wantOpen) {
		t.Errorf("laneOpen must be rebuilt from scratch, got %+v want %+v", m.laneOpen, wantOpen)
	}
	if m.selectedCommit != 0 {
		t.Errorf("selectedCommit = %d, want 0", m.selectedCommit)
	}
	if !m.commitsExhausted {
		t.Error("commitsExhausted must be true when fewer than 200 rows arrive")
	}
}

func TestMouseIgnoredWhilePickerOpen(t *testing.T) {
	m := pickerTestModel()
	m.focusedPane = focusFiles
	openPicker(m)

	m.Update(tea.MouseMsg{X: 5, Y: 20, Type: tea.MouseLeft})
	if m.focusedPane != focusFiles {
		t.Fatalf("mouse must be ignored while picker open, focusedPane = %v", m.focusedPane)
	}
}

func TestPickerWindowFollowsCursor(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 10 // bottom pane fits a single row
	openPicker(m)

	v := m.View()
	if !strings.Contains(v, "(all branches)") {
		t.Errorf("cursor row (all-branches entry) must stay visible in a one-row window, got:\n%s", v)
	}
	if strings.Contains(v, "origin/main") {
		t.Error("one-row window must hide non-adjacent refs")
	}

	pressRune(m, 'j') // cursor -> "feature" (index 1)
	v = m.View()
	if !strings.Contains(v, "  feature") {
		t.Errorf("window must follow the cursor to the feature row, got:\n%s", v)
	}
	if strings.Contains(v, "(all branches)") {
		t.Error("one-row window must scroll the all-branches entry out of view")
	}
}

func TestPickerOpenQStillQuits(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("'q' with picker open must still quit")
	}
	if reflect.ValueOf(cmd).Pointer() != reflect.ValueOf(tea.Quit).Pointer() {
		t.Fatal("'q' with picker open must return tea.Quit")
	}
}
