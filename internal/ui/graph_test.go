package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"tgit/internal/git"
)

func TestRenderCommitLineAuthorSuffix(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "msg", Author: "Chester Cheng"}, "●", 100, selNone)
	if !strings.Contains(out, authorStyle.Render("  Chester Cheng")) {
		t.Errorf("author suffix missing from commit line: %q", out)
	}

	noAuthor := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "msg"}, "●", 100, selNone)
	if strings.Contains(noAuthor, authorStyle.Render("  ")) {
		t.Errorf("empty author must not emit suffix: %q", noAuthor)
	}
}

func TestRefsRightAligned(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "short", Refs: "(HEAD -> master)"}, "●", 60, selNone)
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

	fits := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "m", Refs: "(h)"}, "●", 20, selNone)
	if got := lipgloss.Width(fits); got > 20 {
		t.Errorf("degraded inline row width = %d, want <= 20: %q", got, stripANSI(fits))
	}
	if !strings.Contains(stripANSI(fits), "(h)") {
		t.Errorf("refs must stay present in degraded layout: %q", stripANSI(fits))
	}

	long := renderCommitLineStyled(git.CommitRow{
		Hash: "abc1234",
		Msg:  "a very long commit message that will never fit",
		Refs: "(HEAD -> master, tag: v1.2.3, origin/main)",
	}, "●", 10, selNone)
	if !strings.Contains(stripANSI(long), "(HEAD -> master, tag: v1.2.3, origin/main)") {
		t.Errorf("malformed narrow input must not drop refs: %q", stripANSI(long))
	}
}

func TestRefsSurviveBottomTruncate(t *testing.T) {
	forceANSI256(t)

	m := InitialModel()
	m.width, m.height = 80, 24
	m.commits = []git.CommitRow{
		{Hash: "abc1234", Msg: "short", Refs: "(HEAD -> master)"},
	}
	m.graphCells, _ = buildLanes(m.commits, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	out := updated.(*model).renderBottomSized(7)

	if !strings.Contains(stripANSI(out), "(HEAD -> master)") {
		t.Errorf("exact-fit refs row must not be eaten by truncate tail-reserve: %q", stripANSI(out))
	}
	if strings.Contains(stripANSI(out), "...") {
		t.Errorf("fitting row must not be truncated: %q", stripANSI(out))
	}
}

func containsReverseSGR(s string) bool {
	for _, seq := range strings.Split(s, "\x1b[")[1:] {
		for _, p := range strings.Split(strings.SplitN(seq, "m", 2)[0], ";") {
			if p == "7" {
				return true
			}
		}
	}
	return false
}

func TestSelectionHighlightsHash(t *testing.T) {
	forceANSI256(t)

	commit := git.CommitRow{Hash: "abc1234", Msg: "msg"}
	cells := "●"

	// Focused-selected: reverse video (SGR code 7) must sit on the hash
	// itself, and nothing may wrap the graph cells segment.
	sel := renderCommitLineStyled(commit, cells, 100, selFocused)
	idx := strings.Index(sel, "abc1234")
	if idx < 0 {
		t.Fatalf("hash missing from selected row: %q", sel)
	}
	sgrStart := strings.LastIndex(sel[:idx], "\x1b[")
	if sgrStart < 0 {
		t.Fatalf("no SGR sequence precedes the selected hash: %q", sel)
	}
	params := strings.Split(strings.TrimSuffix(sel[sgrStart+2:idx], "m"), ";")
	reverse := false
	for _, p := range params {
		if p == "7" {
			reverse = true
		}
	}
	if !reverse {
		t.Errorf("selected hash must carry reverse video (code 7), got SGR %v: %q", params, sel)
	}
	if head := sel[:sgrStart]; strings.Contains(head, "\x1b[") {
		t.Errorf("selection styling must not wrap the cells segment: %q", head)
	}

	// Inactive-selected: dim foreground on the hash, no reverse anywhere.
	inactive := renderCommitLineStyled(commit, cells, 100, selInactive)
	if !strings.Contains(inactive, inactiveHashStyle.Render("abc1234")) {
		t.Errorf("inactive-selected hash must use dim foreground: %q", inactive)
	}
	if strings.Contains(inactive, "\x1b[7m") || strings.Contains(inactive, ";7m") {
		t.Errorf("inactive row must not reverse video: %q", inactive)
	}

	// Unselected: byte-identical to the plain rendering.
	none := renderCommitLineStyled(commit, cells, 100, selNone)
	want := fmt.Sprintf("%s %s %s", cells, commitHashStyle.Render(commit.Hash), commit.Msg)
	if none != want {
		t.Errorf("unselected row changed:\n got %q\nwant %q", none, want)
	}
}

func TestRenderCommitLinePreservesValidUTF8(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "修复中文路径排序"}, "●", 100, selNone)
	if !strings.Contains(stripANSI(out), "修复中文路径排序") {
		t.Errorf("valid Chinese message must be preserved verbatim: %q", stripANSI(out))
	}
}

func TestRenderCommitLineSanitizesTabsAndInvalidUTF8(t *testing.T) {
	forceANSI256(t)

	tabbed := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "col1\tcol2"}, "●", 100, selNone)
	if strings.Contains(tabbed, "\t") {
		t.Errorf("commit-row tab must be sanitized away: %q", tabbed)
	}
	if !strings.Contains(stripANSI(tabbed), "col1    col2") {
		t.Errorf("tab must expand to spaces in place: %q", stripANSI(tabbed))
	}

	bad := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "bad\xff\xfe tail"}, "●", 100, selNone)
	if !strings.Contains(bad, "\uFFFD") {
		t.Errorf("invalid UTF-8 message must surface as U+FFFD: %q", bad)
	}
	if strings.Contains(bad, "\xff") || strings.Contains(bad, "\xfe") {
		t.Errorf("raw invalid bytes must not reach the rendered row: %q", bad)
	}

	badAuthor := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "m", Author: "\xc3\x28"}, "●", 100, selNone)
	if !strings.Contains(badAuthor, "\uFFFD") {
		t.Errorf("invalid UTF-8 author must surface as U+FFFD: %q", badAuthor)
	}
}

func TestCommitRowTruncateBoundedMixedScript(t *testing.T) {
	forceANSI256(t)

	msg := strings.Repeat("提交commit", 40) // mixed CJK/ASCII, ~400 display cells
	row := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: msg}, "●", 500, selFocused)

	const w = 80
	line := truncate.StringWithTail(row, w, "...")
	if got := lipgloss.Width(line); got > w {
		t.Errorf("truncated mixed-script row width = %d, want <= %d: %q", got, w, stripANSI(line))
	}
	if !strings.HasSuffix(stripANSI(line), "...") {
		t.Errorf("overlong row must carry the truncate tail: %q", stripANSI(line))
	}
}
