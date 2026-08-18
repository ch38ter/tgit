package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	repoRoot     string
	repoRootOnce sync.Once
	repoRootErr  error
)

func resolveRepoRoot() (string, error) {
	repoRootOnce.Do(func() {
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		out, err := cmd.Output()
		if err != nil {
			repoRootErr = fmt.Errorf("not a git repository: %w", err)
			return
		}
		repoRoot = strings.TrimSpace(string(out))
	})
	return repoRoot, repoRootErr
}

// RunGit executes a git command with the repo root as cwd and
// GIT_OPTIONAL_LOCKS=0 set in the environment. The repo root is resolved
// once via `git rev-parse --show-toplevel` on first call and cached.
func RunGit(args ...string) (string, error) {
	root, err := resolveRepoRoot()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %v failed: %w (stderr: %s)", args, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %v failed: %w", args, err)
	}
	return string(out), nil
}
