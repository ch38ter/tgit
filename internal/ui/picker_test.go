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

func pressSpace(m *model) {
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
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
		t.Fatalf("cursor must start on the (all branches) entry when viewRefs is empty, got %d", m.refCursor)
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

func TestPickerSpaceTogglesCheck(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)
	pressRune(m, 'j') // cursor -> "feature"

	pressSpace(m)
	if !m.checked["feature"] {
		t.Fatal("space must check the entry under the cursor")
	}
	if v := m.View(); !strings.Contains(v, "[x]") {
		t.Errorf("checked row must render an [x] prefix, got:\n%s", v)
	}

	pressSpace(m)
	if m.checked["feature"] {
		t.Fatal("space again must uncheck the entry")
	}
	if v := m.View(); strings.Contains(v, "[x]") {
		t.Errorf("no [x] may remain after untoggle, got:\n%s", v)
	}
	if v := m.View(); !strings.Contains(v, "[ ]") {
		t.Errorf("unchecked rows must render an [ ] prefix, got:\n%s", v)
	}
}

func TestPickerSpaceOnAllEntryNoOp(t *testing.T) {
	m := pickerTestModel()
	openPicker(m) // cursor at index 0

	pressSpace(m)
	if len(m.checked) != 0 {
		t.Fatalf("space on the (all branches) entry must be a no-op, checked = %v", m.checked)
	}
	if v := m.View(); strings.Contains(v, "[x]") {
		t.Errorf("(all branches) entry must never render checked, got:\n%s", v)
	}
}

func TestPickerEnterAppliesCheckedSetInListOrder(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)

	for i := 0; i < 4; i++ {
		pressRune(m, 'j') // cursor -> "origin/main"
	}
	pressSpace(m)
	pressRune(m, 'k')
	pressRune(m, 'k') // cursor -> "main"
	pressSpace(m)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickerOpen {
		t.Fatal("enter must close the picker")
	}
	want := []string{"main", "origin/main"}
	if !reflect.DeepEqual(m.viewRefs, want) {
		t.Fatalf("applied set must follow picker list order, got %v want %v", m.viewRefs, want)
	}
	msg := cmd()
	rcm, ok := msg.(refCommitsMsg)
	if !ok {
		t.Fatalf("cmd must produce refCommitsMsg, got %T", msg)
	}
	if !reflect.DeepEqual(rcm.refs, want) {
		t.Fatalf("refCommitsMsg.refs = %v, want %v", rcm.refs, want)
	}
}

func TestPickerPrechecksCurrentSelection(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"origin/main", "main"}
	openPicker(m)

	if !m.checked["main"] || !m.checked["origin/main"] {
		t.Fatalf("opening must pre-check entries matching viewRefs, checked = %v", m.checked)
	}
	if n := strings.Count(m.View(), "[x]"); n != 2 {
		t.Fatalf("exactly 2 rows must render [x], got %d", n)
	}
	if m.refCursor != 2 {
		t.Fatalf("multi-selection cursor must fall back to the HEAD-marked entry (2), got %d", m.refCursor)
	}
}

func TestPickerEnterSelectsRef(t *testing.T) {
	m := pickerTestModel()
	openPicker(m)
	pressRune(m, 'j') // cursor -> "feature" (index 1), nothing checked

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickerOpen {
		t.Fatal("enter must close the picker")
	}
	if !reflect.DeepEqual(m.viewRefs, []string{"feature"}) {
		t.Fatalf("enter with no checks must select the cursor entry, got %v", m.viewRefs)
	}
	if cmd == nil {
		t.Fatal("enter must return a loadCommitsForRefs cmd")
	}
	msg := cmd()
	rcm, ok := msg.(refCommitsMsg)
	if !ok {
		t.Fatalf("cmd must produce refCommitsMsg, got %T", msg)
	}
	if !reflect.DeepEqual(rcm.refs, []string{"feature"}) {
		t.Fatalf("refCommitsMsg.refs = %v, want [feature]", rcm.refs)
	}
}

func TestPickerAllBranchesEntrySelected(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"feature"}
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
	if len(m.viewRefs) != 0 {
		t.Fatalf("selecting the all-branches entry must reset viewRefs to empty, got %v", m.viewRefs)
	}
	msg := cmd()
	rcm, ok := msg.(refCommitsMsg)
	if !ok {
		t.Fatalf("cmd must produce refCommitsMsg, got %T", msg)
	}
	if len(rcm.refs) != 0 {
		t.Fatalf("refCommitsMsg.refs = %v, want empty (all-branches mode)", rcm.refs)
	}

	m.Update(refCommitsMsg{refs: nil, commits: []git.CommitRow{
		{Hash: "aaa1111", FullHash: strings.Repeat("a", 40), Msg: "combined"},
	}})
	if len(m.commits) != 1 || m.commits[0].Msg != "combined" {
		t.Fatalf("empty-selection response must be applied when viewRefs is empty, commits = %+v", m.commits)
	}
}

func TestPickerCursorStartsAtCurrentViewRef(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"origin/main"}
	openPicker(m)
	if m.refCursor != 4 {
		t.Fatalf("cursor must land on the current viewRefs row (4), got %d", m.refCursor)
	}

	m2 := pickerTestModel()
	m2.viewRefs = []string{"ghost-branch"}
	openPicker(m2)
	if m2.refCursor != 2 {
		t.Fatalf("unmatched viewRefs must fall back to the HEAD-marked entry (2), got %d", m2.refCursor)
	}
}

func TestRefCommitsMsgStaleIgnored(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"main"}
	m.commits = []git.CommitRow{{Hash: "old1", FullHash: strings.Repeat("a", 40), Msg: "old"}}

	m.Update(refCommitsMsg{refs: []string{"other"}, commits: []git.CommitRow{{Hash: "new1"}}})

	if len(m.commits) != 1 || m.commits[0].Hash != "old1" {
		t.Fatalf("stale refCommitsMsg must be discarded, commits = %+v", m.commits)
	}
}

func TestRefCommitsStaleKeyOrderSensitive(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"a", "b"}
	m.commits = []git.CommitRow{{Hash: "old1", FullHash: strings.Repeat("a", 40), Msg: "old"}}

	sameSetDiffOrder := []git.CommitRow{{Hash: "new1"}}
	m.Update(refCommitsMsg{refs: []string{"b", "a"}, commits: sameSetDiffOrder})
	if len(m.commits) != 1 || m.commits[0].Hash != "old1" {
		t.Fatalf("response key must match selection key exactly (order included), commits = %+v", m.commits)
	}

	m.Update(refCommitsMsg{refs: []string{"a", "b"}, commits: sameSetDiffOrder})
	if len(m.commits) != 1 || m.commits[0].Hash != "new1" {
		t.Fatalf("matching key must be applied, commits = %+v", m.commits)
	}
}

func TestRefCommitsMsgFreshResetsState(t *testing.T) {
	m := pickerTestModel()
	m.viewRefs = []string{"main"}
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
	m.Update(refCommitsMsg{refs: []string{"main"}, commits: fresh})

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
	if !strings.Contains(v, "feature") {
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
