package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles for the commit stat section (file list + change counts shown by
// `git show --stat`). Each metric gets its own color so counts are easy
// to tell apart at a glance.
var (
	// statSepStyle renders the === separator lines wrapping the stat block.
	statSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // bright yellow, eye-catching
			Bold(true)

	// statFilesCountStyle highlights "N file(s) changed" in the summary line.
	statFilesCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")). // cyan
				Bold(true)

	// statInsertCountStyle highlights insertion counts in the summary line.
	statInsertCountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // green

	// statDeleteCountStyle highlights deletion counts in the summary line.
	statDeleteCountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // red

	// statPerFileCountStyle highlights the per-file change count after "|".
	statPerFileCountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow

	// statPlusBarStyle highlights "+" marks in the per-file bar.
	statPlusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	// statMinusBarStyle highlights "-" marks in the per-file bar.
	statMinusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

var (
	// statSummaryRe matches the trailing summary line of `git show --stat`,
	// e.g. " 2 files changed, 10 insertions(+), 3 deletions(-)"
	// (singular forms "1 file changed", "1 insertion(+)" included).
	statSummaryRe = regexp.MustCompile(`^\s*\d+ files? changed(?:, \d+ insertions?\(\+\))?(?:, \d+ deletions?\(-\))?\s*$`)

	// statFilesChangedRe isolates the count in "N file(s) changed".
	statFilesChangedRe = regexp.MustCompile(`\d+ files? changed`)

	// statInsertionsRe isolates the count in "N insertion(s)(+)".
	statInsertionsRe = regexp.MustCompile(`\d+ insertions?\(\+\)`)

	// statDeletionsRe isolates the count in "N deletion(s)(-)".
	statDeletionsRe = regexp.MustCompile(`\d+ deletions?\(-\)`)

	// statFileNumRe matches a per-file stat row ending in "| N [+/- bars]",
	// e.g. " main.go | 10 +++---" (git separates count and bar with a space).
	// Rename notation "old => new" is covered by the ".+" path part.
	statFileNumRe = regexp.MustCompile(`^ .+\s\|\s+\d+[\s+-]*$`)

	// statFileBinRe matches a binary-file stat row,
	// e.g. " logo.png | Bin 0 -> 3521 bytes".
	statFileBinRe = regexp.MustCompile(`^ .+\s\|\s+Bin\b`)

	// statRowRightRe parses the value column of a numeric stat row into
	// leading spaces, the count, then the optional gap+bar ("8 ++++----").
	statRowRightRe = regexp.MustCompile(`^(\s*)(\d+)(\s*[+-]*)$`)
)

// styleCommitStat decorates the --stat section of a raw `git show` output.
//
// The stat block (per-file rows plus the "N files changed ..." summary) is
// wrapped in full-width === separator lines rendered in bright yellow, and
// every count is colorized: files-changed in cyan, insertions in green,
// deletions in red, per-file counts in yellow (+/- bars in green/red).
//
// The commit header, message, and patch body are returned untouched. If no
// stat section is detected (error text, empty commit, ...), raw is returned
// unchanged. sepWidth is the separator length in cells; values < 3 are
// clamped to 3.
func styleCommitStat(raw string, sepWidth int) string {
	lines := strings.Split(raw, "\n")

	summaryIdx := -1
	for i, l := range lines {
		if statSummaryRe.MatchString(l) {
			summaryIdx = i
			break
		}
	}
	if summaryIdx == -1 {
		return raw
	}

	start := summaryIdx
	for start > 0 && isStatRow(lines[start-1]) {
		start--
	}

	sep := statSepStyle.Render(strings.Repeat("=", max(sepWidth, 3)))

	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:start]...)
	out = append(out, sep)
	for i := start; i <= summaryIdx; i++ {
		out = append(out, styleStatLine(lines[i]))
	}
	out = append(out, sep)
	out = append(out, lines[summaryIdx+1:]...)

	return strings.Join(out, "\n")
}

// isStatRow reports whether l looks like one per-file row of a git stat block.
func isStatRow(l string) bool {
	return statFileNumRe.MatchString(l) || statFileBinRe.MatchString(l)
}

// styleStatLine colorizes a single stat-block line (per-file row or summary).
func styleStatLine(l string) string {
	if statSummaryRe.MatchString(l) {
		return styleStatSummary(l)
	}
	return styleStatFileRow(l)
}

// styleStatSummary colorizes the counts in "N files changed, X insertions(+),
// Y deletions(-)": files in cyan, insertions in green, deletions in red.
func styleStatSummary(line string) string {
	line = statFilesChangedRe.ReplaceAllStringFunc(line, func(m string) string {
		num, _, _ := strings.Cut(m, " ")
		return statFilesCountStyle.Render(num) + strings.TrimPrefix(m, num)
	})
	line = statInsertionsRe.ReplaceAllStringFunc(line, func(m string) string {
		num, _, _ := strings.Cut(m, " ")
		return statInsertCountStyle.Render(num) + strings.TrimPrefix(m, num)
	})
	line = statDeletionsRe.ReplaceAllStringFunc(line, func(m string) string {
		num, _, _ := strings.Cut(m, " ")
		return statDeleteCountStyle.Render(num) + strings.TrimPrefix(m, num)
	})
	return line
}

// styleStatFileRow colorizes one per-file stat row: the count after "|" in
// yellow and each "+" green / "-" red. Unrecognized shapes pass through as-is.
func styleStatFileRow(line string) string {
	idx := strings.LastIndex(line, "|")
	if idx < 0 {
		return line
	}

	left, right := line[:idx+1], line[idx+1:]
	if m := statRowRightRe.FindStringSubmatch(right); m != nil {
		return left + m[1] + statPerFileCountStyle.Render(m[2]) + styleStatBar(m[3])
	}

	trimmed := strings.TrimSpace(right)
	if binIdx := strings.Index(right, "Bin"); trimmed != "" && strings.HasPrefix(trimmed, "Bin") && binIdx >= 0 {
		return left + right[:binIdx] + statPerFileCountStyle.Render("Bin") + right[binIdx+3:]
	}
	return line
}

// commitDiffWidth returns the content width of the diff viewport (terminal
// width minus the outer 2-column trim, border and padding), so decorative
// lines such as the === stat separators match the visible area.
func commitDiffWidth(terminalWidth int) int {
	w := terminalWidth - 6
	if w < 1 {
		w = 1
	}
	return w
}

// styleStatBar renders each "+" green and each "-" red, preserving order.
func styleStatBar(bar string) string {
	var b strings.Builder
	b.Grow(len(bar))
	for _, r := range bar {
		switch r {
		case '+':
			b.WriteString(statPlusBarStyle.Render("+"))
		case '-':
			b.WriteString(statMinusBarStyle.Render("-"))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
