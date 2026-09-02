package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary directory, initializes a git repo,
// configures user.name/user.email, creates an initial commit, and
// returns the path to the repo.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}

	must(runGitCmd(dir, "init"))
	must(runGitCmd(dir, "config", "user.name", "Test User"))
	must(runGitCmd(dir, "config", "user.email", "test@example.com"))

	// Initial commit
	filePath := filepath.Join(dir, "file.txt")
	must(os.WriteFile(filePath, []byte("hello\n"), 0o644))
	must(runGitCmd(dir, "add", "file.txt"))
	must(runGitCmd(dir, "commit", "-m", "initial"))

	return dir
}

func runGitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

func TestShowCommit_ValidCommit(t *testing.T) {
	dir := setupTestRepo(t)

	// Get the initial commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	hash, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	commit := strings.TrimSpace(string(hash))

	// ShowCommit runs git in the current working directory
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	out, err := ShowCommit(commit)
	if err != nil {
		t.Fatalf("ShowCommit returned error: %v", err)
	}

	if !strings.Contains(out, "diff --git") {
		t.Errorf("ShowCommit output missing 'diff --git'. Got:\n%s", out)
	}
	if !strings.Contains(out, "file.txt") {
		t.Errorf("ShowCommit output missing stat for file.txt. Got:\n%s", out)
	}
}

func TestShowCommit_InvalidHash(t *testing.T) {
	// Use any git directory (the temp repo) so git itself doesn't complain
	// about not being in a repo — we just want an invalid object reference.
	dir := setupTestRepo(t)

	// Temporarily change to the test repo so git commands work
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, err := ShowCommit("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected error for invalid commit hash, got nil")
	}
}

func TestFileDiff_ModifiedFile(t *testing.T) {
	dir := setupTestRepo(t)

	// Modify the tracked file
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("modified content\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	out, err := FileDiff("file.txt")
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}

	if !strings.Contains(out, "diff --git") {
		t.Errorf("FileDiff output missing 'diff --git'. Got:\n%s", out)
	}
	// Should contain removal of old line and addition of new line
	if !strings.Contains(out, "-hello") {
		t.Errorf("FileDiff output missing removal of 'hello'. Got:\n%s", out)
	}
	if !strings.Contains(out, "+modified content") {
		t.Errorf("FileDiff output missing addition of 'modified content'. Got:\n%s", out)
	}
}

func TestFileDiff_UntrackedFile(t *testing.T) {
	dir := setupTestRepo(t)

	// Create an untracked file
	untrackedPath := filepath.Join(dir, "newfile.txt")
	if err := os.WriteFile(untrackedPath, []byte("brand new\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	out, err := FileDiff("newfile.txt")
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}

	if out != "(untracked file - no diff)" {
		t.Errorf("expected '(untracked file - no diff)', got: %q", out)
	}
}

func TestFileDiff_CleanTrackedFile(t *testing.T) {
	dir := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// file.txt is tracked and unmodified
	out, err := FileDiff("file.txt")
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}

	if out != "(no changes)" {
		t.Errorf("expected '(no changes)', got: %q", out)
	}
}

func TestGetRepoInfo(t *testing.T) {
	dir := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	info, err := GetRepoInfo()
	if err != nil {
		t.Fatalf("GetRepoInfo returned error: %v", err)
	}

	// Toplevel should match the repo root
	if info.Toplevel != dir {
		t.Errorf("Toplevel = %q, want %q", info.Toplevel, dir)
	}

	// Branch should be "main" or "master" (default branch)
	if info.Branch == "" {
		t.Error("Branch is empty, expected a default branch name")
	}

	// UserName should be what we configured
	if info.UserName != "Test User" {
		t.Errorf("UserName = %q, want %q", info.UserName, "Test User")
	}
}

func TestShowCommit_UnicodeOutput(t *testing.T) {
	dir := setupTestRepo(t)

	// Use both a non-ASCII path and content: core.quotePath must not turn
	// either the displayed path into octal escapes or hide the readable text.
	name := "中文.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("第一行\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := runGitCmd(dir, "add", name); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGitCmd(dir, "commit", "-m", "添加中文文件"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	hash, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	out, err := ShowCommit(strings.TrimSpace(string(hash)))
	if err != nil {
		t.Fatalf("ShowCommit returned error: %v", err)
	}
	for _, want := range []string{name, "第一行", "添加中文文件"} {
		if !strings.Contains(out, want) {
			t.Errorf("ShowCommit output missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\\`) {
		t.Errorf("ShowCommit output contains escaped path/content: %q", out)
	}
}

func TestFileDiff_UnicodeOutput(t *testing.T) {
	dir := setupTestRepo(t)
	name := "中文.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("旧内容\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := runGitCmd(dir, "add", name); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGitCmd(dir, "commit", "-m", "添加中文文件"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("新内容\n"), 0o644); err != nil {
		t.Fatalf("modify: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	out, err := FileDiff(name)
	if err != nil {
		t.Fatalf("FileDiff returned error: %v", err)
	}
	for _, want := range []string{name, "旧内容", "新内容"} {
		if !strings.Contains(out, want) {
			t.Errorf("FileDiff output missing %q; got:\n%s", want, out)
		}
	}
}
