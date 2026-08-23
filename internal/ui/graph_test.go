package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tgit/internal/git"
)

// precedingSGR returns the contiguous run of ANSI SGR sequences immediately
// before index idx in s ("" if none).
func precedingSGR(s string, idx int) string {
	start := idx
	for start > 0 {
		prev := strings.LastIndex(s[:start], "\x1b[")
		if prev < 0 {
			break
		}
		endRel := strings.Index(s[prev:], "m")
		if endRel < 0 || prev+endRel+1 != start {
			break
		}
		start = prev
	}
	return s[start:idx]
}

func charIndices(s, c string) []int {
	var out []int
	for i := 0; i+len(c) <= len(s); {
		j := strings.Index(s[i:], c)
		if j < 0 {
			break
		}
		out = append(out, i+j)
		i += j + 1
	}
	return out
}

func lastSGR(run string) string {
	i := strings.LastIndex(run, "\x1b[")
	if i < 0 {
		return ""
	}
	return run[i:]
}

func sgrCodes(seq string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	return strings.Split(trimmed, ";")
}

func TestColorGraphAlternatesColorsByColumn(t *testing.T) {
	forceANSI256(t)

	out := colorGraph("| * | * ")

	starSGRs := map[string]bool{}
	for _, idx := range charIndices(out, "●") {
		starSGRs[lastSGR(precedingSGR(out, idx))] = true
	}
	if len(starSGRs) != 2 {
		t.Errorf("two stars on different columns must carry 2 distinct SGR runs, got %d: %v", len(starSGRs), starSGRs)
	}

	pipeSGRs := map[string]bool{}
	for _, idx := range charIndices(out, "│") {
		pipeSGRs[lastSGR(precedingSGR(out, idx))] = true
	}
	if len(pipeSGRs) != 2 {
		t.Errorf("pipes at col 0 and col 1 must carry 2 distinct SGR runs, got %d: %v", len(pipeSGRs), pipeSGRs)
	}
}

func TestColorGraphStarIsBold(t *testing.T) {
	forceANSI256(t)

	out := colorGraph("* ")
	for _, idx := range charIndices(out, "●") {
		codes := sgrCodes(lastSGR(precedingSGR(out, idx)))
		bold := false
		for _, c := range codes {
			if c == "1" {
				bold = true
			}
		}
		if !bold {
			t.Errorf("star span must contain bold code 1, got %q", lastSGR(precedingSGR(out, idx)))
		}
	}
}

func TestColorGraphMergeDecrementsColumn(t *testing.T) {
	forceANSI256(t)

	merged := colorGraph("|/*")
	plain := colorGraph("* x")

	mStar := charIndices(merged, "●")
	pStar := charIndices(plain, "●")
	if len(mStar) != 1 || len(pStar) != 1 {
		t.Fatalf("fixture broken: merged stars=%d plain stars=%d", len(mStar), len(pStar))
	}
	gotMerge := lastSGR(precedingSGR(merged, mStar[0]))
	gotPlain := lastSGR(precedingSGR(plain, pStar[0]))
	if gotMerge == "" || gotMerge != gotPlain {
		t.Errorf("'*' after '|/' must reuse column-0 color:\nmerge: %q\nplain: %q", gotMerge, gotPlain)
	}
}

func TestMapGraphCharsTable(t *testing.T) {
	tests := []struct {
		in   rune
		want string
	}{
		{'|', "│"},
		{'*', "●"},
		{'/', "╯"},
		{'\\', "╰"},
		{'_', "─"},
		{'-', "─"},
		{'x', "x"},
		{' ', " "},
	}
	for _, tc := range tests {
		if got := mapGraphChars(tc.in); got != tc.want {
			t.Errorf("mapGraphChars(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderCommitLineAuthorSuffix(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLine(git.CommitRow{Graph: "* ", Hash: "abc1234", Msg: "msg", Author: "Chester Cheng"}, 100)
	if !strings.Contains(out, authorStyle.Render("  Chester Cheng")) {
		t.Errorf("author suffix missing from commit line: %q", out)
	}

	noAuthor := renderCommitLine(git.CommitRow{Graph: "* ", Hash: "abc1234", Msg: "msg"}, 100)
	if strings.Contains(noAuthor, authorStyle.Render("  ")) {
		t.Errorf("empty author must not emit suffix: %q", noAuthor)
	}
}

func TestRefsRightAligned(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLine(git.CommitRow{Graph: "* ", Hash: "abc1234", Msg: "short", Refs: "(HEAD -> master)"}, 60)
	if got := lipgloss.Width(out); got != 60 {
		t.Errorf("right-aligned row width = %d, want 60: %q", got, stripANSI(out))
	}
	if !strings.HasSuffix(stripANSI(out), "(HEAD -> master)") {
		t.Errorf("refs must end the row: %q", stripANSI(out))
	}
	if !strings.Contains(out, strings.Repeat(" ", 10)) {
		t.Errorf("expected a multi-space gap before refs: %q", stripANSI(out))
	}
}

func TestRefsDegradeNarrow(t *testing.T) {
	forceANSI256(t)

	fits := renderCommitLine(git.CommitRow{Graph: "* ", Hash: "abc1234", Msg: "m", Refs: "(h)"}, 20)
	if got := lipgloss.Width(fits); got > 20 {
		t.Errorf("degraded inline row width = %d, want <= 20: %q", got, stripANSI(fits))
	}
	if !strings.Contains(stripANSI(fits), "(h)") {
		t.Errorf("refs must stay present in degraded layout: %q", stripANSI(fits))
	}

	long := renderCommitLine(git.CommitRow{
		Graph: "* ",
		Hash:  "abc1234",
		Msg:   "a very long commit message that will never fit",
		Refs:  "(HEAD -> master, tag: v1.2.3, origin/main)",
	}, 10)
	if !strings.Contains(stripANSI(long), "(HEAD -> master, tag: v1.2.3, origin/main)") {
		t.Errorf("malformed narrow input must not drop refs: %q", stripANSI(long))
	}
}

func TestRefsSurviveBottomTruncate(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.commits = []git.CommitRow{
		{Graph: "* ", Hash: "abc1234", Msg: "short", Refs: "(HEAD -> master)"},
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out := updated.(*model).renderBottomSized(7)

	if !strings.Contains(stripANSI(out), "(HEAD -> master)") {
		t.Errorf("exact-fit refs row must not be eaten by truncate tail-reserve: %q", stripANSI(out))
	}
	if strings.Contains(stripANSI(out), "...") {
		t.Errorf("fitting row must not be truncated: %q", stripANSI(out))
	}
}

func TestRenderCommitLineColoredGraph(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLine(git.CommitRow{Graph: "| * ", Hash: "abc1234", Msg: "x"}, 100)
	if strings.Index(out, "\x1b[") >= strings.Index(out, "abc1234") {
		t.Errorf("graph segment must be colorized before the hash: %q", out)
	}
	if !strings.Contains(out, graphPalette[0].Render("│")) {
		t.Errorf("column-0 pipe not colored with palette[0]: %q", out)
	}
}

// styledCol is one visible display column: the rune shown plus the SGR
// sequence in effect before it. ANSI sequences occupy no columns.
type styledCol struct {
	ch  rune
	sgr string
}

func styledColumns(s string) []styledCol {
	var cols []styledCol
	cur := ""
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			end := strings.IndexByte(s[i:], 'm')
			if end >= 0 {
				seq := s[i : i+end+1]
				if strings.HasSuffix(seq, "[0m") {
					cur = ""
				} else {
					cur = seq
				}
				i += end + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		cols = append(cols, styledCol{ch: r, sgr: cur})
		i += size
	}
	return cols
}

func TestSynthesizeLanesFillsContinuity(t *testing.T) {
	forceANSI256(t)

	got := synthesizeGraphLanes([]git.CommitRow{
		{Graph: "* "},
		{Graph: "| * "},
		{Graph: "* "},
	})
	if len(got) != 3 {
		t.Fatalf("expected 3 output rows, got %d", len(got))
	}

	structs := make([]string, len(got))
	cols := make([][]styledCol, len(got))
	for i, s := range got {
		structs[i] = stripANSI(s)
		cols[i] = styledColumns(s)
	}

	for i, s := range structs {
		rs := []rune(s)
		if len(rs) > 1 && rs[1] != ' ' {
			t.Errorf("row %d column 1 is a dead lane and must stay a space, got %q", i, s)
		}
	}

	wantFillerSGR := styledColumns(graphFillers[1].Render("│"))[0].sgr
	for _, row := range []int{0, 2} {
		if cols[row][2].ch != '│' {
			t.Errorf("row %d must gain synthesized filler at column 2, got %q", row, structs[row])
		}
		if cols[row][2].sgr != wantFillerSGR {
			t.Errorf("row %d filler must carry palette[1] SGR %q, got %q", row, wantFillerSGR, cols[row][2].sgr)
		}
	}

	if cols[1][0].ch != '│' || cols[1][2].ch != '●' {
		t.Errorf("row 1 must keep │@0 and ●@2, got %q", structs[1])
	}
}

func TestSynthesizeLanesMergeShape(t *testing.T) {
	forceANSI256(t)

	got := synthesizeGraphLanes([]git.CommitRow{
		{Graph: "| * "},
		{Graph: "|\\"},
		{Graph: "| * "},
		{Graph: "|/"},
		{Graph: "* "},
	})
	structs := make([]string, len(got))
	for i, s := range got {
		structs[i] = stripANSI(s)
	}

	const forkRow, dotRow, mergeRow = 1, 2, 3

	dotCols := []rune(structs[dotRow])
	forkCols := []rune(structs[forkRow])
	mergeCols := []rune(structs[mergeRow])

	runeIndex := func(rs []rune, r rune) int {
		for i, x := range rs {
			if x == r {
				return i
			}
		}
		return -1
	}

	dotCol := runeIndex(dotCols, '●')
	if dotCol != 2 {
		t.Fatalf("feature dot expected at column 2, got %d (%q)", dotCol, structs[dotRow])
	}

	if mergeCols[dotCol-1] != '╯' {
		t.Errorf("╯ must sit below-left of the lane dot at column %d, got %q", dotCol-1, structs[mergeRow])
	}

	forkCol := runeIndex(forkCols, '╰')
	if forkCol < 0 {
		t.Fatalf("fork row must contain ╰: %q", structs[forkRow])
	}

	if forkCols[dotCol] != '│' {
		t.Errorf("fork row must bridge to the lane with │ at column %d, got %q", dotCol, structs[forkRow])
	}
	if dotCols[forkCol] != '│' {
		t.Errorf("dot row must bridge to the fork with │ at column %d, got %q", forkCol, structs[dotRow])
	}
	if mergeCols[dotCol] != '│' {
		t.Errorf("merge row must bridge to the lane with │ at column %d, got %q", dotCol, structs[mergeRow])
	}
}

func TestSynthesizeLanesEdges(t *testing.T) {
	forceANSI256(t)

	if got := synthesizeGraphLanes(nil); len(got) != 0 {
		t.Errorf("nil input must yield empty output, got %d rows", len(got))
	}

	lone := synthesizeGraphLanes([]git.CommitRow{{Graph: "|/"}})
	if want := colorGraph("|/"); lone[0] != want {
		t.Errorf("lone graph-only row must equal colorGraph:\ngot  = %q\nwant = %q", lone[0], want)
	}

	top := synthesizeGraphLanes([]git.CommitRow{{Graph: "|\\"}, {Graph: "* "}})
	if !strings.Contains(stripANSI(top[0]), "╰") || !strings.Contains(stripANSI(top[1]), "●") {
		t.Errorf("window-edge rows must keep their own glyphs, got %q / %q", stripANSI(top[0]), stripANSI(top[1]))
	}
}
