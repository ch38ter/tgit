package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tgit/internal/git"
)

func TestInitialModel_IsGitRepo(t *testing.T) {
	// Save and restore original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Test in a git directory
	m := InitialModel()
	m.Init()
	if !m.isGitRepo {
		t.Error("expected isGitRepo to be true in a git directory")
	}

	// Test in a non-git directory
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	m2 := InitialModel()
	m2.Init()
	if m2.isGitRepo {
		t.Error("expected isGitRepo to be false in a non-git directory")
	}
}

func TestViewFixedLayout(t *testing.T) {
	m := InitialModel()
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := model.View()
	if !strings.Contains(v, "+") {
		t.Fatal("view should contain border")
	}
	lines := strings.Split(v, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "+") {
		t.Fatalf("top border missing, first line=%q", lines[0])
	}
	if len(lines) != 24 {
		t.Fatalf("view should have exactly height (24) lines, got %d, view=%q", len(lines), v)
	}
}

func TestViewClipsOverflowNoScroll(t *testing.T) {
	m := InitialModel()
	for i := 0; i < 100; i++ {
		m.changes = append(m.changes, newFileChange("a/b/file.go", 'M'))
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := updated.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 24 {
		t.Fatalf("overflow view must still be exactly height, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "+") {
		t.Fatalf("top border missing under overflow, first=%q", lines[0])
	}
}

func newFileChange(path string, status byte) git.FileChange {
	return git.FileChange{Path: path, Status: status}
}



func TestJKIsolated(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 3; i++ {
		m.changes = append(m.changes, newFileChange("a/file.go", 'M'))
		m.commits = append(m.commits, git.CommitRow{Hash: "abc1234", Msg: "x"})
	}
	m.focusedPane = focusFiles
	m.selectedFile = 0
	m.selectedCommit = 1
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.selectedCommit != 1 {
		t.Fatalf("j in focusFiles should not move selectedCommit, got %d", m.selectedCommit)
	}
	if m.selectedFile != 1 {
		t.Fatalf("j in focusFiles should move selectedFile to 1, got %d", m.selectedFile)
	}
	m.focusedPane = focusCommits
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.selectedCommit != 2 {
		t.Fatalf("j in focusCommits should move selectedCommit to 2, got %d", m.selectedCommit)
	}
	if m.selectedFile != 1 {
		t.Fatalf("j in focusCommits should not move selectedFile, got %d", m.selectedFile)
	}
}

func TestMouseFocusSwitch(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 10; i++ {
		m.changes = append(m.changes, newFileChange("a/file.go", 'M'))
		m.commits = append(m.commits, git.CommitRow{Hash: "abc1234", Msg: "x"})
	}
	m.selectedFile = 0
	m.selectedCommit = 0
	// click in middle pane (header 4 lines, middle 10 lines approx)
	m.Update(tea.MouseMsg{X: 5, Y: 6, Type: tea.MouseLeft})
	if m.focusedPane != focusFiles {
		t.Fatalf("click in middle pane should focus files, got %v", m.focusedPane)
	}
	// click in bottom pane
	m.Update(tea.MouseMsg{X: 5, Y: 20, Type: tea.MouseLeft})
	if m.focusedPane != focusCommits {
		t.Fatalf("click in bottom pane should focus commits, got %v", m.focusedPane)
	}
}

func TestMouseIgnoredInDiff(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 24
	m.currentView = "diff"
	m.focusedPane = focusFiles
	m.Update(tea.MouseMsg{X: 5, Y: 20, Type: tea.MouseLeft})
	if m.focusedPane != focusFiles {
		t.Fatalf("mouse in diff should not switch focus, got %v", m.focusedPane)
	}
}

func TestPaneViewportWindow(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 24
	for i := 0; i < 100; i++ {
		m.changes = append(m.changes, newFileChange("a/file.go", 'M'))
		m.commits = append(m.commits, git.CommitRow{Hash: "abc1234", Msg: "msg"})
	}
	m.selectedFile = 99
	m.selectedCommit = 99
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := model.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 24 {
		t.Fatalf("viewport window must keep 24 rows, got %d", len(lines))
	}
}

func TestDiffViewNoLineOverflow(t *testing.T) {
	m := InitialModel()
	m.width, m.height = 80, 24
	long := strings.Repeat("x", 500)
	m.diffTitle = "very/long/path/" + long + ".go"
	m.diffViewport.SetContent(long + "\n" + long)
	m.currentView = "diff"
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := model.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 24 {
		t.Fatalf("diff view must keep 24 rows, got %d", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Fatalf("line %d width %d exceeds terminal width 80: %q", i, w, l)
		}
	}
}

func TestGitNotFound(t *testing.T) {
	// Save and restore original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty so git is not found
	os.Setenv("PATH", "")

	// This should call os.Exit(1), which we can't test directly
	// Just verify that LookPath fails
	_, err := exec.LookPath("git")
	if err == nil {
		t.Error("expected LookPath to fail with empty PATH")
	}
}
