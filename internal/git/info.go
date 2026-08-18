package git

import (
	"fmt"
	"strings"
)

// RepoInfo holds basic metadata about the current git repository.
type RepoInfo struct {
	Toplevel string // absolute path to the repo root
	Branch   string // current branch name (empty if detached or empty repo)
	UserName string // git config user.name (empty if unset)
}

// GetRepoInfo queries git for the toplevel directory, current branch,
// and user name.  An empty repo returns Branch="" without error.
// A missing user.name returns UserName="" without error.
func GetRepoInfo() (RepoInfo, error) {
	var info RepoInfo

	// Toplevel
	out, err := RunGit("rev-parse", "--show-toplevel")
	if err != nil {
		return info, fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	info.Toplevel = strings.TrimSpace(out)

	// Branch — empty repo or detached HEAD returns empty string.
	out, err = RunGit("branch", "--show-current")
	if err != nil {
		info.Branch = ""
	} else {
		info.Branch = strings.TrimSpace(out)
	}

	// UserName — missing config returns empty string, no error.
	out, err = RunGit("config", "user.name")
	if err != nil {
		info.UserName = ""
	} else {
		info.UserName = strings.TrimSpace(out)
	}

	return info, nil
}
