package git

import (
	"os/exec"
	"strconv"
	"strings"
)

// CommitRow represents a single commit from `git log` output.
type CommitRow struct {
	Hash     string   // abbreviated hash for display (%h), variable length
	FullHash string   // full 40-hex hash (%H); Parents entries reference these
	Parents  []string // full hashes of parent commits, empty for root commit
	Refs     string   // decoration like "(HEAD -> main)", empty if none
	Msg      string   // commit subject
	Author   string   // author name from %an, empty if absent
}

// commitRowsPrettyFormat is the NUL-separated field format:
// abbrev hash \0 full hash \0 parents \0 refs \0 subject \0 author.
const commitRowsPrettyFormat = "--pretty=format:%h%x00%H%x00%P%x00%d%x00%s%x00%an"

// LogCommitRowsAt runs `git log` in dir with a NUL-separated pretty format
// and parses each line into a CommitRow. Two modes by ref:
//
//   - ref == "": `git log --skip=N -n M --abbrev-commit --decorate=short
//     --all <format>` — combined history across all refs (the default view).
//   - ref != "": `git log <ref> --skip=N -n M --abbrev-commit
//     --decorate=short <format>` — only commits reachable from ref (--all is
//     omitted; a rev plus --all would union back to every ref).
//
// Every returned row is a real commit — topology lives in Parents and lanes
// are computed by the UI layer.
//
// Silent-degradation contract: an empty repo or an unresolvable ref produces
// empty stdout and a non-zero exit; that yields an empty slice and nil error.
// max defaults to 200 if <= 0; negative skip is treated as 0.
func LogCommitRowsAt(dir string, ref string, skip int, max int) ([]CommitRow, error) {
	if max <= 0 {
		max = 200
	}
	if skip < 0 {
		skip = 0
	}

	args := []string{"log"}
	if ref != "" {
		args = append(args, ref)
	}
	args = append(args,
		"--skip="+strconv.Itoa(skip),
		"-n", strconv.Itoa(max),
		"--abbrev-commit", "--decorate=short")
	if ref == "" {
		args = append(args, "--all")
	}
	args = append(args, commitRowsPrettyFormat)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Empty repo (no commits) produces an exit error with no stdout.
		if len(out) == 0 {
			return []CommitRow{}, nil
		}
		return nil, err
	}

	var rows []CommitRow
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if row, ok := parseCommitLine(line); ok {
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// parseCommitLine splits one output line on NUL into six fields:
// abbrev-hash \0 full-hash \0 parents \0 refs \0 subject \0 author.
// Lines that do not validate are rejected rather than guessed at — every
// accepted row must be a real commit with resolvable parent references.
func parseCommitLine(line string) (CommitRow, bool) {
	fields := strings.Split(line, "\x00")
	if len(fields) < 6 {
		return CommitRow{}, false
	}

	hash := fields[0]
	if !isAllHex(hash) {
		return CommitRow{}, false
	}

	full := fields[1]
	if len(full) != 40 || !isAllHex(full) {
		return CommitRow{}, false
	}

	return CommitRow{
		Hash:     hash,
		FullHash: full,
		Parents:  strings.Fields(fields[2]),
		Refs:     strings.TrimSpace(fields[3]),
		Msg:      fields[4],
		Author:   fields[5],
	}, true
}

// isAllHex reports whether s is non-empty and consists solely of hex digits.
func isAllHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexChar(s[i]) {
			return false
		}
	}
	return true
}

// isHexChar reports whether b is a hexadecimal character.
func isHexChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
