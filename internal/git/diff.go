package git

import (
	"fmt"
	"strings"
)

// ShowCommit runs `git show --format=fuller --stat -p <commit>` and returns
// the combined output (patch + stat).  The commit is passed as a single
// argv element, so hashes/refnames with special characters are safe.
func ShowCommit(commit string) (string, error) {
	out, err := RunGit("show", "--format=fuller", "--stat", "-p", commit)
	if err != nil {
		return "", fmt.Errorf("git show %s: %w", commit, err)
	}
	return out, nil
}

// FileDiff returns the diff for a single file path.
//
//   - First tries `git diff -- <path>`.
//   - If empty, tries `git diff --cached -- <path>` (staged changes).
//   - If still empty and the file is untracked, returns "(untracked file - no diff)".
//   - If still empty and the file is tracked & clean, returns "(no changes)".
//
// The path is passed as a single argv element, so paths with spaces are safe.
func FileDiff(path string) (string, error) {
	// 1. Working-tree diff
	out, err := RunGit("diff", "--", path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
	}

	// 2. Staged diff
	out, err = RunGit("diff", "--cached", "--", path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return out, nil
	}

	// 3. Determine whether the file is untracked or clean
	untracked, err := isUntracked(path)
	if err != nil {
		return "", err
	}
	if untracked {
		return "(untracked file - no diff)", nil
	}
	return "(no changes)", nil
}

// isUntracked reports whether path is an untracked file (not ignored).
func isUntracked(path string) (bool, error) {
	out, err := RunGit("ls-files", "--others", "--exclude-standard", path)
	if err != nil {
		return false, fmt.Errorf("git ls-files: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}
