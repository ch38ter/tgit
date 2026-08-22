package ui

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

const commitShowSample = `commit 1a2b3c4d5e6f7a8b9c0d
Author:     Chester <chester@example.com>
AuthorDate: Thu Aug 20 10:00:00 2026 +0800
Commit:     Chester <chester@example.com>
CommitDate: Thu Aug 20 10:00:00 2026 +0800

    add feature

 internal/ui/app.go | 8 ++++----
 logo.png           | Bin 0 -> 3521 bytes
 old.go => new.go   | 2 +-
 main.go            | 2 ++
 4 files changed, 11 insertions(+), 3 deletions(-)

diff --git a/main.go b/main.go
index 1234567..89abcde 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old line
+new line
 context
`

func TestStyleCommitStatWrapsStatBlockWithSeparators(t *testing.T) {
	const sepWidth = 50
	got := stripANSI(styleCommitStat(commitShowSample, sepWidth))
	wantSep := strings.Repeat("=", sepWidth)

	const (
		firstFileRow = " internal/ui/app.go | 8 ++++----"
		summaryRow   = " 4 files changed, 11 insertions(+), 3 deletions(-)"
	)
	inLines := strings.Split(commitShowSample, "\n")
	outLines := strings.Split(got, "\n")

	if n := len(outLines) - len(inLines); n != 2 {
		t.Fatalf("output added %d lines, want 2", n)
	}
	for _, row := range []string{firstFileRow, summaryRow} {
		if !slices.Contains(inLines, row) {
			t.Fatalf("fixture broken, %q not in sample", row)
		}
	}

	outFirst := indexOf(outLines, firstFileRow)
	outSummary := indexOf(outLines, summaryRow)
	if outLines[outFirst-1] != wantSep {
		t.Errorf("no === separator above first file row:\n%q", outLines[outFirst-1])
	}
	if outLines[outSummary+1] != wantSep {
		t.Errorf("no === separator below summary line:\n%q", outLines[outSummary+1])
	}
	if strings.Count(got, wantSep) != 2 {
		t.Errorf("separator count != 2")
	}

	inFirst := indexOf(inLines, firstFileRow)
	inSummary := indexOf(inLines, summaryRow)
	want := insertLines(commitShowSample, []int{inFirst, inSummary + 1}, wantSep)
	if got != want {
		t.Errorf("plain-text layout mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestStyleCommitStatColorizesCountsDistinctly(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(prev)

	got := styleCommitStat(commitShowSample, 40)

	filesSeq := statFilesCountStyle.Render("4")
	insSeq := statInsertCountStyle.Render("11")
	delSeq := statDeleteCountStyle.Render("3")

	for _, tc := range []struct{ seq, label string }{
		{filesSeq, "files count"},
		{insSeq, "insertions count"},
		{delSeq, "deletions count"},
	} {
		if !strings.Contains(got, tc.seq) {
			t.Errorf("%s not colorized (seq %q missing)", tc.label, tc.seq)
		}
	}
	if filesSeq == insSeq || insSeq == delSeq || filesSeq == delSeq {
		t.Errorf("summary colors are not distinct: %q %q %q", filesSeq, insSeq, delSeq)
	}

	if !strings.Contains(got, statPerFileCountStyle.Render("8")) {
		t.Error("per-file count not colorized")
	}
	if !strings.Contains(got, statPlusBarStyle.Render("+")) || !strings.Contains(got, statMinusBarStyle.Render("-")) {
		t.Error("+/- bar marks not colorized")
	}
	if !strings.Contains(got, statPerFileCountStyle.Render("Bin")) {
		t.Error("binary row marker not colorized")
	}

	sep := statSepStyle.Render(strings.Repeat("=", 40))
	if strings.Count(got, sep) != 2 {
		t.Error("=== separators not styled or wrong width")
	}
}

func TestStyleCommitStatNoStatSectionUnchanged(t *testing.T) {
	raw := "fatal: bad revision 'deadbeef'\n"
	if got := stripANSI(styleCommitStat(raw, 40)); got != raw {
		t.Errorf("input without stat section was modified:\n%q", got)
	}
}

func TestStyleCommitStatSummaryOnlyStillWrapped(t *testing.T) {
	raw := "commit abc\n\n    msg\n\n 0 files changed\n"
	got := stripANSI(styleCommitStat(raw, 10))
	if n := strings.Count(got, "=========="); n != 2 {
		t.Fatalf("separator count = %d, want 2:\n%s", n, got)
	}
	lines := strings.Split(got, "\n")
	summaryIdx := indexOf(lines, " 0 files changed")
	if summaryIdx < 1 || lines[summaryIdx-1] != "==========" || lines[summaryIdx+1] != "==========" {
		t.Errorf("summary line not sandwiched between separators:\n%q", lines)
	}
}

func TestStyleCommitStatPatchBodyUntouched(t *testing.T) {
	got := styleCommitStat(commitShowSample, 30)
	patchStart := strings.Index(commitShowSample, "diff --git")
	wantPatch := commitShowSample[patchStart:]
	idx := strings.Index(stripANSI(got), "diff --git")
	if idx < 0 {
		t.Fatal("patch body missing from output")
	}
	if tail := stripANSI(got[idx:]); tail != wantPatch {
		t.Errorf("patch body was modified:\ngot:\n%s\nwant:\n%s", tail, wantPatch)
	}
}

func TestCommitDiffWidth(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{80, 74},
		{8, 2},
		{5, 1},
		{0, 1},
	} {
		if got := commitDiffWidth(tc.in); got != tc.want {
			t.Errorf("commitDiffWidth(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func indexOf(lines []string, want string) int {
	for i, l := range lines {
		if l == want {
			return i
		}
	}
	return -1
}

func insertLines(src string, at []int, line string) string {
	srcLines := strings.Split(src, "\n")
	set := map[int]bool{}
	for _, i := range at {
		set[i] = true
	}
	var out []string
	for i, l := range srcLines {
		if set[i] {
			out = append(out, line)
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
