package ui

import (
	"strings"
	"testing"

	"tgit/internal/git"
)

// laneFixture builds CommitRows with short stand-in hashes. Order matters:
// rows must be newest-first, exactly as git log emits them.
func laneFixture(msg, fullHash string, parents ...string) git.CommitRow {
	return git.CommitRow{
		Hash:     fullHash[:7],
		FullHash: fullHash,
		Parents:  parents,
		Msg:      msg,
	}
}

// dotCol returns the display column of the lane dot in a cell string.
func dotCol(t *testing.T, cell string) int {
	t.Helper()
	for i, r := range []rune(stripANSI(cell)) {
		if r == '●' || r == '◉' {
			return i
		}
	}
	t.Fatalf("no lane dot in cell %q", stripANSI(cell))
	return -1
}

const (
	hX  = "1111111111111111111111111111111111111111" // merge commit X
	hM1 = "2222222222222222222222222222222222222222" // main commit M1
	hF2 = "3333333333333333333333333333333333333333" // feature tip F2
	hF1 = "4444444444444444444444444444444444444444" // feature base F1
	hB  = "5555555555555555555555555555555555555555" // fork/root commit B
)

func TestBuildLanesLinear(t *testing.T) {
	rows := []git.CommitRow{
		laneFixture("c3", "a000000", "b000000"),
		laneFixture("c2", "b000000", "c000000"),
		laneFixture("c1", "c000000", "d000000"),
		laneFixture("root", "d000000"), // 0 parents
	}

	cells, open := buildLanes(rows, nil)

	if len(cells) != 4 {
		t.Fatalf("expected 4 cell rows, got %d", len(cells))
	}
	for i, want := range []int{0, 0, 0, 0} {
		if got := dotCol(t, cells[i]); got != want {
			t.Errorf("row %d dot at lane %d, want %d (%q)", i, got, want, stripANSI(cells[i]))
		}
	}
	if n := len(open); n != 0 {
		t.Errorf("after root commit all lanes must be freed and trimmed, got %d open lanes: %v", n, open)
	}
}

func TestBuildLanesForkAndMerge(t *testing.T) {
	// History: B forks into main (M1) and feature (F1->F2); X merges F2 back.
	rows := []git.CommitRow{
		laneFixture("merge X", hX, hM1, hF2),
		laneFixture("main M1", hM1, hB),
		laneFixture("feat F2", hF2, hF1),
		laneFixture("feat F1", hF1, hB),
		laneFixture("fork B", hB),
	}

	cells, open := buildLanes(rows, nil)

	wantDots := []int{0, 0, 1, 1, 0}
	for i, want := range wantDots {
		if got := dotCol(t, cells[i]); got != want {
			t.Errorf("row %d (%s) dot at lane %d, want %d (%q)",
				i, rows[i].Msg, got, want, stripANSI(cells[i]))
		}
	}

	// Merge row must carry a connection into the second-parent lane, and the
	// convergence row (fork B) must carry the joining curve — neither may be
	// a bare dot.
	if strings.HasPrefix(stripANSI(cells[0]), "◉ ") || stripANSI(cells[0]) == "◉" {
		t.Errorf("merge row must connect to the second-parent lane, got %q", stripANSI(cells[0]))
	}
	if stripANSI(cells[4]) == "●" {
		t.Errorf("converged fork row must show the join curve, got %q", stripANSI(cells[4]))
	}

	if n := len(open); n != 0 {
		t.Errorf("fully consumed history must leave no open lanes, got %v", open)
	}
}

func TestBuildLanesParallelOpen(t *testing.T) {
	// Two independent root lines interleaved: lanes must coexist.
	rows := []git.CommitRow{
		laneFixture("a1", "a100000", "a000000"),
		laneFixture("b1", "b100000", "b000000"),
		laneFixture("a0", "a000000"),
		laneFixture("b0", "b000000"),
	}

	cells, open := buildLanes(rows, nil)

	wantDots := []int{0, 1, 0, 1}
	for i, want := range wantDots {
		if got := dotCol(t, cells[i]); got != want {
			t.Errorf("row %d dot at lane %d, want %d (%q)", i, got, want, stripANSI(cells[i]))
		}
	}

	// While lane 0 dots along, lane 1 must persist as a colored │.
	if !strings.Contains(cells[2], graphPalette[1].Render("│")) {
		t.Errorf("row 2 must keep lane 1 alive with palette[1] │, got %q", stripANSI(cells[2]))
	}
	if n := len(open); n != 0 {
		t.Errorf("both roots consumed, open must be empty, got %v", open)
	}
}

func TestBuildLanesPageContinuation(t *testing.T) {
	page1 := []git.CommitRow{
		laneFixture("merge X", hX, hM1, hF2),
	}
	cells1, open1 := buildLanes(page1, nil)
	if got := dotCol(t, cells1[0]); got != 0 {
		t.Fatalf("page1 dot at lane %d, want 0", got)
	}
	if len(open1) != 2 || open1[0][0] != hM1 || open1[1][0] != hF2 {
		t.Fatalf("page1 must leave lanes [M1, F2] open, got %v", open1)
	}

	// Page 2 continues from the carried-over state: F2 must land on lane 1,
	// not restart from lane 0.
	page2 := []git.CommitRow{
		laneFixture("main M1", hM1, hB),
		laneFixture("feat F2", hF2, hF1),
	}
	cells2, open2 := buildLanes(page2, open1)

	if got := dotCol(t, cells2[0]); got != 0 {
		t.Errorf("page2 row 0 dot at lane %d, want 0 (continuation)", got)
	}
	if got := dotCol(t, cells2[1]); got != 1 {
		t.Errorf("page2 row 1 dot at lane %d, want 1 (must NOT restart columns)", got)
	}
	if len(open2) != 2 || open2[0][0] != hB || open2[1][0] != hF1 {
		t.Errorf("page2 must leave lanes [B, F1] open, got %v", open2)
	}
}

func TestBuildLanesOctopusDoesNotPanic(t *testing.T) {
	rows := []git.CommitRow{
		laneFixture("octopus", "o000000", "p100000", "p200000", "p300000"),
		laneFixture("p1", "p100000"),
		laneFixture("p2", "p200000"),
		laneFixture("p3", "p300000"),
	}

	cells, open := buildLanes(rows, nil)

	wantDots := []int{0, 0, 1, 2}
	for i, want := range wantDots {
		if got := dotCol(t, cells[i]); got != want {
			t.Errorf("row %d dot at lane %d, want %d (%q)", i, got, want, stripANSI(cells[i]))
		}
	}
	if n := len(open); n != 0 {
		t.Errorf("octopus history fully consumed, open must be empty, got %v", open)
	}
}

// Regression for the connect() bridge-overwrite bug: the outermost extra's
// bridge '─' used to erase the inner extra's start curve '╮', collapsing a
// three-way fan-out into what reads as a single branch (●─╮).
func TestBuildLanesOctopusFanout(t *testing.T) {
	rows := []git.CommitRow{
		laneFixture("octopus", "o000000", "p100000", "p200000", "p300000"),
		laneFixture("p1", "p100000"),
		laneFixture("p2", "p200000"),
		laneFixture("p3", "p300000"),
	}

	cells, _ := buildLanes(rows, nil)

	// Every extra parent gets its own start curve leaving the dot; no
	// bridge may degrade them.
	if got := stripANSI(cells[0]); got != "◉╮╮" {
		t.Errorf("octopus row must fan out as ◉╮╮, got %q", got)
	}
}

// Regression: three children of one commit re-meet at their fork point. The
// later join's bridge '─' used to overwrite the earlier join's curve '╯',
// visually disconnecting the middle lane (●─╯ instead of ●╯╯).
func TestBuildLanesForkPointTripleJoin(t *testing.T) {
	rows := []git.CommitRow{
		laneFixture("c1", "c100000", "b000000"),
		laneFixture("c2", "c200000", "b000000"),
		laneFixture("c3", "c300000", "b000000"),
		laneFixture("fork b", "b000000"),
	}

	cells, open := buildLanes(rows, nil)

	wantDots := []int{0, 1, 2, 0}
	for i, want := range wantDots {
		if got := dotCol(t, cells[i]); got != want {
			t.Errorf("row %d dot at lane %d, want %d (%q)", i, got, want, stripANSI(cells[i]))
		}
	}
	// Both joining lanes must keep their convergence curves into the dot.
	if got := stripANSI(cells[3]); got != "●╯╯" {
		t.Errorf("fork-point row must converge as ●╯╯, got %q", got)
	}
	if n := len(open); n != 0 {
		t.Errorf("fully consumed fork history must leave no open lanes, got %v", open)
	}
}

func TestBuildLanesEmpty(t *testing.T) {
	cells, open := buildLanes(nil, nil)
	if len(cells) != 0 {
		t.Errorf("nil rows must yield no cells, got %d", len(cells))
	}
	if len(open) != 0 {
		t.Errorf("nil rows must yield no open lanes, got %v", open)
	}
}

func TestBuildLanesDotStyling(t *testing.T) {
	forceANSI256(t)

	rows := []git.CommitRow{
		laneFixture("a1", "a100000", "a000000"),
		laneFixture("b1", "b100000", "b000000"),
	}
	cells, _ := buildLanes(rows, nil)

	// Dots carry their lane's palette slot, bolded; the style must come from
	// the dedicated dot styles (lipgloss v0.10 Bold mutates shared rules —
	// reusing line styles would permanently bold them).
	if !strings.Contains(cells[0], graphDotStyles[0].Render("●")) {
		t.Errorf("lane 0 dot must render with graphDotStyles[0], got %q", cells[0])
	}
	if !strings.Contains(cells[1], graphDotStyles[1].Render("●")) {
		t.Errorf("lane 1 dot must render with graphDotStyles[1], got %q", cells[1])
	}
}
