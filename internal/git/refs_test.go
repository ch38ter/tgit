package git

import (
	"os"
	"testing"
)

// chdirTemp moves the process into dir for the duration of the test (RunGit
// resolves the repo root from the working directory).
func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestListRefs_LocalsThenRemotes(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")

	writeFile(t, dir, "a.txt", "one")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "first")

	gitRun(t, dir, "branch", "feature")
	// Fake remote-tracking ref without configuring a real remote.
	gitRun(t, dir, "update-ref", "refs/remotes/origin/feat", "HEAD")

	chdirTemp(t, dir)

	refs, err := ListRefs()
	if err != nil {
		t.Fatalf("ListRefs error: %v", err)
	}

	want := []Ref{
		{Name: "feature", Head: false},
		{Name: "main", Head: true},
		{Name: "origin/feat", Head: false},
	}
	if len(refs) != len(want) {
		t.Fatalf("got %d refs %+v, want %+v", len(refs), refs, want)
	}
	for i, r := range refs {
		if r != want[i] {
			t.Errorf("refs[%d] = %+v, want %+v", i, r, want[i])
		}
	}
}

func TestListRefs_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")

	chdirTemp(t, dir)

	refs, err := ListRefs()
	if err != nil {
		t.Fatalf("empty repo must degrade silently, got error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %+v", refs)
	}
}
