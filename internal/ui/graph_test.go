package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tgit/internal/git"
)

func TestRenderCommitLineAuthorSuffix(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "msg", Author: "Chester Cheng"}, "●", 100)
	if !strings.Contains(out, authorStyle.Render("  Chester Cheng")) {
		t.Errorf("author suffix missing from commit line: %q", out)
	}

	noAuthor := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "msg"}, "●", 100)
	if strings.Contains(noAuthor, authorStyle.Render("  ")) {
		t.Errorf("empty author must not emit suffix: %q", noAuthor)
	}
}

func TestRefsRightAligned(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "short", Refs: "(HEAD -> master)"}, "●", 60)
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

	fits := renderCommitLineStyled(git.CommitRow{Hash: "abc1234", Msg: "m", Refs: "(h)"}, "●", 20)
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
	}, "●", 10)
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
