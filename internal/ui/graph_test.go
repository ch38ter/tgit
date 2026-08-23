package ui

import (
	"strings"
	"testing"

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
	for _, idx := range charIndices(out, "*") {
		starSGRs[lastSGR(precedingSGR(out, idx))] = true
	}
	if len(starSGRs) != 2 {
		t.Errorf("two stars on different columns must carry 2 distinct SGR runs, got %d: %v", len(starSGRs), starSGRs)
	}

	pipeSGRs := map[string]bool{}
	for _, idx := range charIndices(out, "|") {
		pipeSGRs[lastSGR(precedingSGR(out, idx))] = true
	}
	if len(pipeSGRs) != 2 {
		t.Errorf("pipes at col 0 and col 1 must carry 2 distinct SGR runs, got %d: %v", len(pipeSGRs), pipeSGRs)
	}
}

func TestColorGraphStarIsBold(t *testing.T) {
	forceANSI256(t)

	out := colorGraph("* ")
	for _, idx := range charIndices(out, "*") {
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

	mStar := charIndices(merged, "*")
	pStar := charIndices(plain, "*")
	if len(mStar) != 1 || len(pStar) != 1 {
		t.Fatalf("fixture broken: merged stars=%d plain stars=%d", len(mStar), len(pStar))
	}
	gotMerge := lastSGR(precedingSGR(merged, mStar[0]))
	gotPlain := lastSGR(precedingSGR(plain, pStar[0]))
	if gotMerge == "" || gotMerge != gotPlain {
		t.Errorf("'*' after '|/' must reuse column-0 color:\nmerge: %q\nplain: %q", gotMerge, gotPlain)
	}
}

func TestRenderCommitLineColoredGraph(t *testing.T) {
	forceANSI256(t)

	out := renderCommitLine(git.CommitRow{Graph: "| * ", Hash: "abc1234", Msg: "x"})
	if strings.Index(out, "\x1b[") >= strings.Index(out, "abc1234") {
		t.Errorf("graph segment must be colorized before the hash: %q", out)
	}
	if !strings.Contains(out, graphPalette[0].Render("|")) {
		t.Errorf("column-0 pipe not colored with palette[0]: %q", out)
	}
}
