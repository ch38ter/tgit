package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogGraph_MultiBranchWithMerge(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo with a default branch.
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	// Initial commit.
	writeFile(t, dir, "file.txt", "initial")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial commit")

	// Two more commits on main.
	writeFile(t, dir, "file.txt", "main work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "second commit")

	writeFile(t, dir, "file.txt", "more main work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "third commit")

	// Create feature branch from the initial commit.
	gitRun(t, dir, "checkout", "-b", "feature", "HEAD~2")

	// Two commits on feature.
	writeFile(t, dir, "feature.txt", "feature work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature commit 1")

	writeFile(t, dir, "feature.txt", "more feature work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature commit 2")

	// Go back to main and merge feature.
	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "merge", "feature", "-m", "merge feature into main")

	// Run LogGraphAt.
	rows, err := LogGraphAt(dir, 200)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}

	if len(rows) < 6 {
		t.Fatalf("expected >= 6 rows, got %d", len(rows))
	}

	// Verify each row has a non-empty hex hash (length varies with repo size).
	for i, row := range rows {
		if row.Hash == "" {
			t.Errorf("row %d: hash is empty", i)
		}
		if len(row.Hash) < 7 {
			t.Errorf("row %d: hash length = %d, want >= 7 (hash=%q)", i, len(row.Hash), row.Hash)
		}
		for _, c := range row.Hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				t.Errorf("row %d: hash %q contains non-hex char", i, row.Hash)
			}
		}
	}

	// Verify at least one row has '*' in graph.
	hasStar := false
	for _, row := range rows {
		if strings.Contains(row.Graph, "*") {
			hasStar = true
			break
		}
	}
	if !hasStar {
		t.Error("no row has '*' in graph")
	}

	// Verify merge commit is present and has refs.
	foundMerge := false
	for _, row := range rows {
		if row.Msg == "merge feature into main" {
			foundMerge = true
			if row.Refs == "" {
				t.Error("merge commit has empty refs (expected HEAD -> main)")
			}
		}
		if row.Author != "Test" {
			t.Errorf("row %q: author = %q, want %q", row.Hash, row.Author, "Test")
		}
	}
	if !foundMerge {
		t.Error("merge commit not found in rows")
	}
}

func TestLogGraph_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")

	rows, err := LogGraphAt(dir, 200)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestLogGraph_MessageWithSpecialChars(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "file.txt", "content")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "fix: handle (parentheses) and *stars* in message")

	rows, err := LogGraphAt(dir, 200)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if rows[0].Msg != "fix: handle (parentheses) and *stars* in message" {
		t.Errorf("unexpected message: %q", rows[0].Msg)
	}

	if rows[0].Author != "Test" {
		t.Errorf("author = %q, want %q", rows[0].Author, "Test")
	}

	// HEAD commit on main always has decorate refs.
	if !strings.Contains(rows[0].Refs, "HEAD -> main") {
		t.Errorf("expected refs to contain HEAD -> main, got %q", rows[0].Refs)
	}
}

func TestLogGraph_DefaultMax(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "file.txt", "content")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "test commit")

	// max=0 should use default (200).
	rows, err := LogGraphAt(dir, 0)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestLogGraph_LongAbbreviatedHash(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	// Force a long abbreviation, like large repos emit automatically.
	gitRun(t, dir, "config", "core.abbrev", "12")

	writeFile(t, dir, "file.txt", "content")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "commit with a real message")

	rows, err := LogGraphAt(dir, 200)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}

	if len(rows[0].Hash) != 12 {
		t.Errorf("expected 12-char hash, got %q (len %d)", rows[0].Hash, len(rows[0].Hash))
	}
	if rows[0].Msg != "commit with a real message" {
		t.Errorf("expected message to survive long-hash parsing, got %q", rows[0].Msg)
	}
	if !strings.Contains(rows[0].Refs, "HEAD -> main") {
		t.Errorf("expected refs to survive long-hash parsing, got %q", rows[0].Refs)
	}
}

func TestLogGraph_CurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "file.txt", "content")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "test commit")

	// Change to the test repo directory.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	// LogGraph (no dir arg) should use ".".
	rows, err := LogGraph(200)
	if err != nil {
		t.Fatalf("LogGraph error: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestLogGraphAppendAt_SkipAndMax(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	for i := 1; i <= 5; i++ {
		writeFile(t, dir, "file.txt", fmt.Sprintf("content %d", i))
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "commit", "-m", fmt.Sprintf("commit %d", i))
	}

	all, err := LogGraphAt(dir, 200)
	if err != nil {
		t.Fatalf("LogGraphAt error: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(all))
	}

	page, err := LogGraphAppendAt(dir, 3, 2)
	if err != nil {
		t.Fatalf("LogGraphAppendAt error: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected exactly 2 rows (skip=3,max=2), got %d", len(page))
	}
	if page[0].Hash != all[3].Hash || page[1].Hash != all[4].Hash {
		t.Errorf("page hashes mismatch: got %s,%s want %s,%s",
			page[0].Hash, page[1].Hash, all[3].Hash, all[4].Hash)
	}
	if page[0].Msg != "commit 2" || page[1].Msg != "commit 1" {
		t.Errorf("page messages mismatch: got %q,%q", page[0].Msg, page[1].Msg)
	}
	if page[0].Author != "Test" || page[1].Author != "Test" {
		t.Errorf("page authors mismatch: got %q,%q", page[0].Author, page[1].Author)
	}

	exhausted, err := LogGraphAppendAt(dir, 5, 200)
	if err != nil {
		t.Fatalf("LogGraphAppendAt past end error: %v", err)
	}
	if len(exhausted) != 0 {
		t.Errorf("expected empty page past end, got %d rows", len(exhausted))
	}
}

func TestParseLineNulFormatAndFallback(t *testing.T) {
	tests := []struct {
		name string
		line string
		want CommitRow
		ok   bool
	}{
		{
			name: "nul format full",
			line: "* abc1234\x00(HEAD -> master)\x00fix: handle (parens)\x00Chester Cheng",
			want: CommitRow{Graph: "* ", Hash: "abc1234", Refs: "(HEAD -> master)", Msg: "fix: handle (parens)", Author: "Chester Cheng"},
			ok:   true,
		},
		{
			name: "nul format no refs chinese author and msg",
			line: "| * deadbeef\x00\x00修复 中文 消息\x00余晨辉",
			want: CommitRow{Graph: "| * ", Hash: "deadbeef", Msg: "修复 中文 消息", Author: "余晨辉"},
			ok:   true,
		},
		{
			name: "nul format missing author field",
			line: "* abc1234\x00\x00msg only\x00",
			want: CommitRow{Graph: "* ", Hash: "abc1234", Msg: "msg only", Author: ""},
			ok:   true,
		},
		{
			name: "nul format only hash",
			line: "* abc1234\x00",
			want: CommitRow{Graph: "* ", Hash: "abc1234"},
			ok:   true,
		},
		{
			name: "nul format invalid hash rejected",
			line: "* zzzz1234\x00\x00msg\x00author",
			want: CommitRow{},
			ok:   false,
		},
		{
			name: "legacy oneline fallback",
			line: "* abc1234 fix some message",
			want: CommitRow{Graph: "* ", Hash: "abc1234", Msg: "fix some message", Author: ""},
			ok:   true,
		},
		{
			name: "legacy oneline with refs",
			line: "* abc1234 (HEAD -> master) msg here",
			want: CommitRow{Graph: "* ", Hash: "abc1234", Refs: "(HEAD -> master)", Msg: "msg here", Author: ""},
			ok:   true,
		},
		{
			name: "pure graph line",
			line: "|/",
			want: CommitRow{},
			ok:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseLine(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("row = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
