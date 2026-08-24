package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupMergeRepo builds a repo with a branch + merge:
//
//	main:   initial -> second -> third -> merge(feature)
//	feature: branched from initial, two commits.
func setupMergeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "file.txt", "initial")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial commit")

	writeFile(t, dir, "file.txt", "main work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "second commit")

	writeFile(t, dir, "file.txt", "more main work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "third commit")

	gitRun(t, dir, "checkout", "-b", "feature", "HEAD~2")

	writeFile(t, dir, "feature.txt", "feature work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature commit 1")

	writeFile(t, dir, "feature.txt", "more feature work")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feature commit 2")

	gitRun(t, dir, "checkout", "main")
	gitRun(t, dir, "merge", "feature", "-m", "merge feature into main")

	return dir
}

func is40Hex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexChar(s[i]) {
			return false
		}
	}
	return true
}

func TestLogCommitRowsAt_MergeRepo(t *testing.T) {
	dir := setupMergeRepo(t)

	rows, err := LogCommitRowsAt(dir, "HEAD", 0, 200)
	if err != nil {
		t.Fatalf("LogCommitRowsAt error: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows (5 commits + 1 merge), got %d", len(rows))
	}

	byMsg := map[string]CommitRow{}
	for _, r := range rows {
		byMsg[r.Msg] = r
	}

	// Every row: full hash is 40-hex and display hash is its prefix.
	for i, r := range rows {
		if !is40Hex(r.FullHash) {
			t.Errorf("row %d (%s): FullHash = %q, want 40 hex chars", i, r.Msg, r.FullHash)
		}
		if r.Hash == "" || !strings.HasPrefix(r.FullHash, r.Hash) {
			t.Errorf("row %d (%s): Hash %q must be a prefix of FullHash %q", i, r.Msg, r.Hash, r.FullHash)
		}
		if r.Author != "Test" {
			t.Errorf("row %d (%s): author = %q, want %q", i, r.Msg, r.Author, "Test")
		}
	}

	// Merge commit has exactly 2 parents, both full hashes of real rows.
	merge := byMsg["merge feature into main"]
	if len(merge.Parents) != 2 {
		t.Fatalf("merge parents = %v, want exactly 2 full hashes", merge.Parents)
	}
	parentMsgs := map[string]bool{}
	for _, p := range merge.Parents {
		found := false
		for _, r := range rows {
			if r.FullHash == p {
				found = true
				parentMsgs[r.Msg] = true
			}
		}
		if !found {
			t.Errorf("merge parent %q does not match any row's FullHash", p)
		}
	}
	if !parentMsgs["third commit"] || !parentMsgs["feature commit 2"] {
		t.Errorf("merge parents must be third commit + feature commit 2, got %v", parentMsgs)
	}

	// Linear commits have exactly 1 parent; root has none.
	if got := byMsg["third commit"].Parents; len(got) != 1 || got[0] != byMsg["second commit"].FullHash {
		t.Errorf("third commit parents = %v, want [%s]", got, byMsg["second commit"].FullHash)
	}
	if got := byMsg["initial commit"].Parents; len(got) != 0 {
		t.Errorf("root commit parents = %v, want empty", got)
	}

	// Refs preserved: HEAD -> main on the merge, feature branch on its tip.
	if !strings.Contains(merge.Refs, "HEAD -> main") {
		t.Errorf("merge refs = %q, want to contain HEAD -> main", merge.Refs)
	}
	if refs := byMsg["feature commit 2"].Refs; !strings.Contains(refs, "feature") {
		t.Errorf("feature tip refs = %q, want to contain feature", refs)
	}
}

func TestLogCommitRowsAt_SkipAndMaxPaging(t *testing.T) {
	dir := setupMergeRepo(t)

	all, err := LogCommitRowsAt(dir, "HEAD", 0, 200)
	if err != nil {
		t.Fatalf("full page error: %v", err)
	}

	page, err := LogCommitRowsAt(dir, "HEAD", 2, 2)
	if err != nil {
		t.Fatalf("paged error: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("expected exactly 2 rows (skip=2,max=2), got %d", len(page))
	}
	if page[0].FullHash != all[2].FullHash || page[1].FullHash != all[3].FullHash {
		t.Errorf("page hashes mismatch: got %s,%s want %s,%s",
			page[0].FullHash, page[1].FullHash, all[2].FullHash, all[3].FullHash)
	}

	exhausted, err := LogCommitRowsAt(dir, "HEAD", 6, 200)
	if err != nil {
		t.Fatalf("past-end error: %v", err)
	}
	if len(exhausted) != 0 {
		t.Errorf("expected empty page past end, got %d rows", len(exhausted))
	}
}

func TestLogCommitRowsAt_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")

	rows, err := LogCommitRowsAt(dir, "HEAD", 0, 200)
	if err != nil {
		t.Fatalf("empty repo must degrade silently, got error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func setupTwoBranchRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "f.txt", "base")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "base")

	gitRun(t, dir, "checkout", "-b", "side")
	writeFile(t, dir, "s.txt", "side")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "side work")

	gitRun(t, dir, "checkout", "main")
	writeFile(t, dir, "m.txt", "main")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "main work")

	return dir
}

func TestLogCommitRowsAt_RefSelectsBranchHistory(t *testing.T) {
	dir := setupTwoBranchRepo(t)

	mainRows, err := LogCommitRowsAt(dir, "main", 0, 200)
	if err != nil {
		t.Fatalf("log main error: %v", err)
	}
	mainMsgs := map[string]bool{}
	for _, r := range mainRows {
		mainMsgs[r.Msg] = true
	}
	if !mainMsgs["main work"] || !mainMsgs["base"] {
		t.Errorf("main history = %v, want main work + base", mainMsgs)
	}
	if mainMsgs["side work"] {
		t.Errorf("main history must not contain side-only commit, got %v", mainMsgs)
	}

	sideRows, err := LogCommitRowsAt(dir, "side", 0, 200)
	if err != nil {
		t.Fatalf("log side error: %v", err)
	}
	if len(sideRows) != 2 || sideRows[0].Msg != "side work" || sideRows[1].Msg != "base" {
		t.Errorf("side history = %+v, want [side work base]", sideRows)
	}
}

func TestLogCommitRowsAt_EmptyRefCombinesAllBranches(t *testing.T) {
	dir := setupTwoBranchRepo(t)

	rows, err := LogCommitRowsAt(dir, "", 0, 200)
	if err != nil {
		t.Fatalf("log --all error: %v", err)
	}
	msgs := map[string]bool{}
	for _, r := range rows {
		msgs[r.Msg] = true
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 commits across branches, got %d (%v)", len(rows), msgs)
	}
	if !msgs["main work"] || !msgs["side work"] || !msgs["base"] {
		t.Errorf("--all history = %v, want commits from both branches", msgs)
	}
}

func TestParseCommitLine(t *testing.T) {
	const full = "d12d6e809326a83140f4c43eec254c01bbfbe1b0"
	tests := []struct {
		name string
		line string
		want CommitRow
		ok   bool
	}{
		{
			name: "valid six fields with refs",
			line: "d12d6e8\x00" + full + "\x00parent1 parent2\x00 (HEAD -> main)\x00merge feat2\x00Chester",
			want: CommitRow{Hash: "d12d6e8", FullHash: full, Parents: []string{"parent1", "parent2"}, Refs: "(HEAD -> main)", Msg: "merge feat2", Author: "Chester"},
			ok:   true,
		},
		{
			name: "root commit has no parents",
			line: "abc1234\x00" + strings.Repeat("a", 40) + "\x00\x00\x00root\x00A",
			want: CommitRow{Hash: "abc1234", FullHash: strings.Repeat("a", 40), Parents: []string{}, Msg: "root", Author: "A"},
			ok:   true,
		},
		{
			name: "octopus merge three parents",
			line: "abc1234\x00" + strings.Repeat("b", 40) + "\x00p1 p2 p3\x00\x00octopus\x00A",
			want: CommitRow{Hash: "abc1234", FullHash: strings.Repeat("b", 40), Parents: []string{"p1", "p2", "p3"}, Msg: "octopus", Author: "A"},
			ok:   true,
		},
		{
			name: "too few fields rejected",
			line: "abc1234\x00" + full + "\x00p\x00refs",
			ok:   false,
		},
		{
			name: "non-hex abbrev rejected",
			line: "zzz1234\x00" + full + "\x00\x00\x00msg\x00A",
			ok:   false,
		},
		{
			name: "short full hash rejected",
			line: "abc1234\x00abcd\x00\x00\x00msg\x00A",
			ok:   false,
		},
		{
			name: "non-hex full hash rejected",
			line: "abc1234\x00" + strings.Repeat("g", 40) + "\x00\x00\x00msg\x00A",
			ok:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCommitLine(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got.Hash != tc.want.Hash || got.FullHash != tc.want.FullHash ||
				strings.Join(got.Parents, ",") != strings.Join(tc.want.Parents, ",") ||
				got.Refs != tc.want.Refs || got.Msg != tc.want.Msg || got.Author != tc.want.Author {
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
