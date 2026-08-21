package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	return dir
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain=v2", "--branch")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	return string(out)
}

func TestParseStatus_EmptyRepo(t *testing.T) {
	dir := initGitRepo(t)
	raw := gitStatus(t, dir)

	branch, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}
	if branch != "" {
		t.Errorf("expected empty branch for initial/unborn, got %q", branch)
	}
	if len(changes) != 0 {
		t.Errorf("expected no changes in empty repo, got %d", len(changes))
	}
}

func TestParseStatus_Untracked(t *testing.T) {
	dir := initGitRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Path != "newfile.txt" {
		t.Errorf("expected path newfile.txt, got %q", changes[0].Path)
	}
	if changes[0].Status != 'U' {
		t.Errorf("expected status U, got %c", changes[0].Status)
	}
	if changes[0].Staged {
		t.Errorf("expected untracked file to be unstaged")
	}
}

func TestParseStatus_StagedNewFile(t *testing.T) {
	dir := initGitRepo(t)

	file := filepath.Join(dir, "staged.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != 'U' {
		t.Errorf("expected status U (added), got %c", changes[0].Status)
	}
	if !changes[0].Staged {
		t.Errorf("expected staged file to have Staged=true")
	}
}

func TestParseStatus_UnstagedModification(t *testing.T) {
	dir := initGitRepo(t)

	file := filepath.Join(dir, "mod.txt")
	if err := os.WriteFile(file, []byte("v1"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "mod.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Modify file without staging
	if err := os.WriteFile(file, []byte("v2"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != 'M' {
		t.Errorf("expected status M, got %c", changes[0].Status)
	}
	if changes[0].Staged {
		t.Errorf("expected unstaged modification to have Staged=false")
	}
}

func TestParseStatus_StagedAndUnstaged(t *testing.T) {
	dir := initGitRepo(t)

	file := filepath.Join(dir, "partial.txt")
	if err := os.WriteFile(file, []byte("v1"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "partial.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Stage a modification
	if err := os.WriteFile(file, []byte("v2"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd = exec.Command("git", "add", "partial.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	// Then modify again (unstaged)
	if err := os.WriteFile(file, []byte("v3"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != 'M' {
		t.Errorf("expected status M, got %c", changes[0].Status)
	}
	// Y='M' overrides X='M' for Staged → false
	if changes[0].Staged {
		t.Errorf("expected Staged=false when both staged and unstaged (Y overrides)")
	}
}

func TestParseStatus_Deleted(t *testing.T) {
	dir := initGitRepo(t)

	file := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(file, []byte("bye"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "gone.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Delete the file from working tree
	os.Remove(file)

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != 'D' {
		t.Errorf("expected status D, got %c", changes[0].Status)
	}
	if changes[0].Staged {
		t.Errorf("expected unstaged deletion to have Staged=false")
	}
}

func TestParseStatus_StagedDeletion(t *testing.T) {
	dir := initGitRepo(t)

	file := filepath.Join(dir, "rm.txt")
	if err := os.WriteFile(file, []byte("bye"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "rm.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Stage the deletion
	cmd = exec.Command("git", "rm", "rm.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git rm failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Status != 'D' {
		t.Errorf("expected status D, got %c", changes[0].Status)
	}
	if !changes[0].Staged {
		t.Errorf("expected staged deletion to have Staged=true")
	}
}

func TestParseStatus_MixedScenario(t *testing.T) {
	dir := initGitRepo(t)

	// Create and commit a base file
	base := filepath.Join(dir, "base.txt")
	if err := os.WriteFile(base, []byte("base"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "base.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 1. Untracked file
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// 2. Unstaged modification
	if err := os.WriteFile(base, []byte("modified"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// 3. Staged new file
	staged := filepath.Join(dir, "added.txt")
	if err := os.WriteFile(staged, []byte("added"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd = exec.Command("git", "add", "added.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	raw := gitStatus(t, dir)
	branch, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	// Branch should be "main" or "master"
	if branch == "" {
		t.Errorf("expected non-empty branch name, got empty")
	}

	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d: %+v", len(changes), changes)
	}

	// Build a map for easier assertions
	changeMap := map[string]FileChange{}
	for _, c := range changes {
		changeMap[c.Path] = c
	}

	// untracked.txt → U, unstaged
	if c, ok := changeMap["untracked.txt"]; !ok {
		t.Errorf("untracked.txt not found in changes")
	} else {
		if c.Status != 'U' {
			t.Errorf("untracked.txt: expected status U, got %c", c.Status)
		}
		if c.Staged {
			t.Errorf("untracked.txt: expected unstaged")
		}
	}

	// base.txt → M, unstaged
	if c, ok := changeMap["base.txt"]; !ok {
		t.Errorf("base.txt not found in changes")
	} else {
		if c.Status != 'M' {
			t.Errorf("base.txt: expected status M, got %c", c.Status)
		}
		if c.Staged {
			t.Errorf("base.txt: expected unstaged")
		}
	}

	// added.txt → U, staged
	if c, ok := changeMap["added.txt"]; !ok {
		t.Errorf("added.txt not found in changes")
	} else {
		if c.Status != 'U' {
			t.Errorf("added.txt: expected status U, got %c", c.Status)
		}
		if !c.Staged {
			t.Errorf("added.txt: expected staged")
		}
	}
}

func TestUnquoteGitPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain.txt", "plain.txt"},
		{`"中文文件.md"`, "中文文件.md"},
		{`"dir/中文 文件.md"`, "dir/中文 文件.md"},
		{`"has\\backslash.txt"`, `has\backslash.txt`},
		{`"has\"quote.txt"`, `has"quote.txt`},
		{`"tab\there.txt"`, "tab\there.txt"},
		{"", ""},
		{`"only"`, "only"},
	}
	for _, tt := range tests {
		if got := unquoteGitPath(tt.in); got != tt.want {
			t.Errorf("unquoteGitPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseStatus_QuotedPath(t *testing.T) {
	dir := initGitRepo(t)

	// Commit a base so the branch is born, then create a Chinese-named file.
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "base.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	name := "中文文件.md"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	raw := gitStatus(t, dir)
	_, changes, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}

	found := false
	for _, c := range changes {
		if c.Path == name {
			found = true
			if c.Status != 'U' {
				t.Errorf("expected status U for %q, got %c", name, c.Status)
			}
		}
	}
	if !found {
		t.Errorf("quoted path %q not found in changes: %+v", name, changes)
	}
}

func TestParseStatus_BranchName(t *testing.T) {
	dir := initGitRepo(t)

	// Create initial commit so branch is born
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	raw := gitStatus(t, dir)
	branch, _, err := ParseStatus(raw)
	if err != nil {
		t.Fatalf("ParseStatus failed: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected branch main or master, got %q", branch)
	}
}
