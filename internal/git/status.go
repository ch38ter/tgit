package git

import "strings"

// FileChange represents a single file change from git status porcelain v2.
type FileChange struct {
	Path   string
	Status byte // U (untracked/added), M (modified), D (deleted)
	Staged bool // true if change is in the index
}

// ParseStatus parses the output of `git status --porcelain=v2 --branch`.
// Returns the branch name (empty for initial/unborn branch) and a slice of FileChange.
func ParseStatus(raw string) (Branch string, Changes []FileChange, err error) {
	var headBranch string
	emptyRepo := false

	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}

		// Branch head line: "# branch.head <name>" or "# branch.head (initial)"
		if strings.HasPrefix(line, "# branch.head ") {
			headBranch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}

		// Detect empty repo (no commits yet) via branch.oid
		if strings.HasPrefix(line, "# branch.oid (initial)") {
			emptyRepo = true
			continue
		}

		// Skip all other header lines (# branch.ab, # branch.upstream, etc.)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Untracked files: "? <path>"
		if strings.HasPrefix(line, "? ") {
			Changes = append(Changes, FileChange{
				Path:   strings.TrimPrefix(line, "? "),
				Status: 'U',
				Staged: false,
			})
			continue
		}

		// Ordinary changed entries: "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"
		if strings.HasPrefix(line, "1 ") {
			if fc, ok := parseOrdinaryEntry(line); ok {
				Changes = append(Changes, fc)
			}
			continue
		}

		// Renamed/Copied entries: "2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <score> <path>\t<origPath>"
		if strings.HasPrefix(line, "2 ") {
			if fc, ok := parseRenamedEntry(line); ok {
				Changes = append(Changes, fc)
			}
			continue
		}

		// Skip unmerged (u), ignored (!), and any other lines
	}

	// Determine branch: empty repo or "(initial)" → empty string
	if headBranch != "(initial)" && !emptyRepo {
		Branch = headBranch
	}

	return Branch, Changes, nil
}

func parseOrdinaryEntry(line string) (FileChange, bool) {
	fields := strings.SplitN(line, " ", 9)
	if len(fields) < 9 {
		return FileChange{}, false
	}

	// XY is at index 1: "1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>"
	xy := fields[1]
	if len(xy) != 2 {
		return FileChange{}, false
	}

	x := xy[0]
	y := xy[1]

	return FileChange{
		Path:   fields[8],
		Status: statusFromXY(x, y),
		Staged: stagedFromXY(x, y),
	}, true
}

func parseRenamedEntry(line string) (FileChange, bool) {
	fields := strings.SplitN(line, " ", 10)
	if len(fields) < 10 {
		return FileChange{}, false
	}

	// XY is at index 1: "2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <score> <path>\t<origPath>"
	xy := fields[1]
	if len(xy) != 2 {
		return FileChange{}, false
	}

	x := xy[0]
	y := xy[1]

	// path and origPath are separated by tab
	pathParts := strings.SplitN(fields[9], "\t", 2)
	path := pathParts[0]

	return FileChange{
		Path:   path,
		Status: statusFromXY(x, y),
		Staged: stagedFromXY(x, y),
	}, true
}

// statusFromXY maps the porcelain v2 XY status codes to a single byte.
// Priority: Y=D → D; Y≠. → M; X=A → U; X=D → D; X=R/C → M; X=M → M; else → U
func statusFromXY(x, y byte) byte {
	switch {
	case y == 'D':
		return 'D'
	case y != '.':
		return 'M'
	case x == 'A':
		return 'U'
	case x == 'D':
		return 'D'
	case x == 'R' || x == 'C':
		return 'M'
	case x == 'M':
		return 'M'
	default:
		return 'U'
	}
}

// stagedFromXY determines if a change is staged based on XY codes.
// Y≠. and Y≠? → unstaged (false); otherwise X≠. → staged (true)
func stagedFromXY(x, y byte) bool {
	if y != '.' && y != '?' {
		return false
	}
	return x != '.'
}
