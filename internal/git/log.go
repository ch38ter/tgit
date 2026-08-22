package git

import (
	"os/exec"
	"strconv"
	"strings"
)

// CommitRow represents a single parsed row from `git log --graph --oneline --decorate`.
type CommitRow struct {
	Graph string // ASCII graph prefix (e.g. "* ", "| * ", "|/")
	Hash  string // Short commit hash (up to 7 hex chars)
	Refs  string // Decoration string like "(HEAD -> main, tag: v1)", empty if none
	Msg   string // Commit message
}

// LogGraph runs `git log --graph --oneline --decorate --all -n <max>` in the
// current directory and parses the output into CommitRow slices.
// max defaults to 200 if <= 0.
func LogGraph(max int) ([]CommitRow, error) {
	return LogGraphAt(".", max)
}

// LogGraphAt runs LogGraph in the specified directory.
func LogGraphAt(dir string, max int) ([]CommitRow, error) {
	if max <= 0 {
		max = 200
	}

	cmd := exec.Command("git", "log", "--graph", "--oneline", "--decorate", "--all", "-n", strconv.Itoa(max))
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
		if row, ok := parseLine(line); ok {
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// LogGraphAppendAt fetches the next page of history after alreadyLoaded rows,
// running `git log --skip=<alreadyLoaded> -n <max> --graph --oneline --decorate --all`.
// Empty stdout (nothing left) yields an empty slice and nil error.
//
// git draws each page's ASCII graph standalone, so an appended page's
// connector glyphs may not visually continue the previous page's graph
// lines — accepted trade-off for on-demand paging.
func LogGraphAppendAt(dir string, alreadyLoaded int, max int) ([]CommitRow, error) {
	if max <= 0 {
		max = 200
	}
	if alreadyLoaded < 0 {
		alreadyLoaded = 0
	}

	cmd := exec.Command("git", "log",
		"--skip="+strconv.Itoa(alreadyLoaded),
		"-n", strconv.Itoa(max),
		"--graph", "--oneline", "--decorate", "--all")
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
		if row, ok := parseLine(line); ok {
			rows = append(rows, row)
		}
	}

	return rows, nil
}

// graphChars is the set of characters that can appear in the ASCII graph prefix.
const graphChars = "|*/\\_- "

// parseLine parses a single line of `git log --graph --oneline --decorate` output.
// Returns the parsed CommitRow and true if the line contains a commit,
// or false if it's a pure graph line (e.g. "|/", "|\").
func parseLine(line string) (CommitRow, bool) {
	// Extract graph prefix: leading chars from the graph character set.
	graphLen := 0
	for graphLen < len(line) && strings.ContainsRune(graphChars, rune(line[graphLen])) {
		graphLen++
	}

	graph := line[:graphLen]
	rest := line[graphLen:]

	// Pure graph line with no commit.
	if rest == "" {
		return CommitRow{}, false
	}

	// Extract hash: all consecutive hex characters. The abbreviated length is
	// NOT fixed at 7 — git auto-lengthens it as the repository grows, so a
	// large repo can emit 9+ chars. The hash is always followed by a space.
	hashLen := 0
	for hashLen < len(rest) && hashLen < 64 && isHexChar(rest[hashLen]) {
		hashLen++
	}

	if hashLen == 0 {
		return CommitRow{}, false
	}

	hash := rest[:hashLen]
	rest = rest[hashLen:]

	// Skip the space after hash.
	if rest == "" || rest[0] != ' ' {
		return CommitRow{Graph: graph, Hash: hash}, true
	}
	rest = rest[1:] // skip space

	// Parse refs and message.
	var refs, msg string
	if rest != "" && rest[0] == '(' {
		// Refs are a single parenthesized group; first ')' closes it.
		if idx := strings.Index(rest, ")"); idx >= 0 {
			refs = rest[:idx+1]
			rest = rest[idx+1:]
			// Skip space after refs.
			if rest != "" && rest[0] == ' ' {
				rest = rest[1:]
			}
		}
	}
	msg = rest

	return CommitRow{
		Graph: graph,
		Hash:  hash,
		Refs:  refs,
		Msg:   msg,
	}, true
}

// isHexChar reports whether b is a hexadecimal character.
func isHexChar(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
