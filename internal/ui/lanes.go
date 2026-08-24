package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tgit/internal/git"
)

// Graph lane palette cycles one color per lane so parallel branches are
// visually separable and every branch keeps its color across rows AND pages —
// the payoff of computing lanes from parent data instead of consuming git's
// per-page text graph.
//
// Dots get their own style instances: lipgloss v0.10's Bold(true) mutates the
// shared rules map, so rendering a bold dot through a style also used for │
// lines would permanently bold that palette slot (lesson from the colorGraph
// era, where graphFillers existed solely to dodge this).
var (
	graphLaneColors = []string{"39", "171", "114", "214", "176"}
	graphPalette    = laneStyles(graphLaneColors, false)
	graphDotStyles  = laneStyles(graphLaneColors, true)
)

func laneStyles(colors []string, bold bool) []lipgloss.Style {
	out := make([]lipgloss.Style, len(colors))
	for i, c := range colors {
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(c))
		if bold {
			st = st.Bold(true)
		}
		out[i] = st
	}
	return out
}

// buildLanes computes one styled graph-cell string per row (newest first, as
// git log emits them), continuing from the open-lane state carried over from
// previously built pages.
//
// open holds one entry per lane: a queue (in practice depth 1) whose head is
// the full hash of the commit expected next on that lane; an empty queue is a
// free lane. Per row:
//
//  1. The commit claims the leftmost lane expecting its hash; any OTHER lane
//     expecting it converges into this row with a join curve (fork points
//     surface here, when two children re-meet at their parent).
//  2. The claimed lane is re-pointed at the first parent, or freed at a root.
//  3. Extra parents (merges, octopus) each get a lane: reused if one already
//     expects them, else the nearest free lane, else a new lane inserted just
//     right of the commit — connected to the dot with a start curve.
//
// Cell grid: one display column per lane; '●' (bold, palette[lane]) at the
// commit, '│' (palette[lane]) on every other occupied lane, box-drawing arcs
// where lanes are born ('╮'/'╭') or converge ('╯'/'╰'), '─' bridging curves
// across intervening columns.
//
// Returns the cell strings and the updated open state for the next batch;
// trailing free lanes are trimmed so state stays compact across pages.
func buildLanes(rows []git.CommitRow, open [][]string) ([]string, [][]string) {
	lanes := cloneLanes(open)
	cells := make([]string, len(rows))

	for i := range rows {
		cells[i], lanes = buildLaneRow(rows[i], lanes)
	}

	return cells, trimTrailingFree(lanes)
}

// buildLaneRow processes one commit against the current lane state, returning
// its rendered cell string and the updated state.
func buildLaneRow(row git.CommitRow, lanes [][]string) (string, [][]string) {
	// 1. Locate the commit's lane; extra matching lanes converge here.
	L := -1
	var joins []int
	for j, l := range lanes {
		if len(l) > 0 && l[0] == row.FullHash {
			if L < 0 {
				L = j
			} else {
				joins = append(joins, j)
			}
		}
	}
	if L < 0 {
		L = firstFreeLane(lanes)
		if L < 0 {
			L = len(lanes)
			lanes = append(lanes, nil)
		}
	}
	for _, j := range joins {
		lanes[j] = nil // converged here; the lane ends at this row
	}

	// 2. Re-point the lane at the first parent; a root commit frees it.
	if len(row.Parents) > 0 {
		lanes[L] = []string{row.Parents[0]}
	} else {
		lanes[L] = nil
	}

	// 3. Place extra parents on their own lanes. Insertions shift lanes to
	// the right, so recorded join indices get bumped accordingly; placed
	// extras are resolved to columns only at render time.
	var extras []string
	insertAt := L + 1
	for _, p := range parentsFrom(row, 1) {
		m := laneIndexOf(lanes, p)
		if m == L {
			continue // duplicate of the first parent; nothing to connect
		}
		if m < 0 {
			m = firstFreeLane(lanes)
		}
		if m < 0 {
			pos := insertAt
			if pos > len(lanes) {
				pos = len(lanes)
			}
			lanes = append(lanes, nil)
			copy(lanes[pos+1:], lanes[pos:])
			lanes[pos] = nil
			for jj := range joins {
				if joins[jj] >= pos {
					joins[jj]++
				}
			}
			m = pos
			insertAt++
		}
		lanes[m] = []string{p}
		extras = append(extras, p)
	}

	// 4. Render: one column per lane, dot at L, │ on other occupied lanes,
	// curves where lanes are born or converge.
	grid := make([]string, len(lanes))
	for j := range grid {
		if len(lanes[j]) > 0 {
			grid[j] = graphPalette[j%len(graphPalette)].Render("│")
		} else {
			grid[j] = " "
		}
	}
	if len(row.Parents) > 1 {
		grid[L] = graphDotStyles[L%len(graphDotStyles)].Render("◉")
	} else {
		grid[L] = graphDotStyles[L%len(graphDotStyles)].Render("●")
	}

	// Bridges fill only empty/vertical slots; a curve pinned by an earlier
	// connection is never overwritten — octopus/multi-join arcs share the
	// span between dot and outermost column, and an unconditional bridge
	// severed those lanes visually. Endpoints stay deterministic: glyphs pin
	// in extras-then-joins order, so two curves on one column keep the join.
	curveCols := make(map[int]bool)
	connect := func(col int, glyph string) {
		lo, hi := L, col
		if lo > hi {
			lo, hi = hi, lo
		}
		for c := lo + 1; c < hi; c++ {
			if !curveCols[c] {
				if len(lanes[c]) > 0 {
					// The bridge crosses a live lane: keep its vertical │
					// and the horizontal bridge as a crossing ┼, so the
					// reader doesn't follow the bridge into that branch.
					grid[c] = graphPalette[col%len(graphPalette)].Render("┼")
				} else {
					grid[c] = graphPalette[col%len(graphPalette)].Render("─")
				}
			}
		}
		grid[col] = glyph
		curveCols[col] = true
	}
	for _, p := range extras {
		m := laneIndexOf(lanes, p)
		if m < 0 || m == L {
			continue
		}
		if m > L {
			connect(m, "╮") // west+south: leaves the dot, flows downward
		} else {
			connect(m, "╭") // east+south
		}
	}
	for _, j := range joins {
		if j > L {
			connect(j, "╯") // north+west: arrives from above, ends at the dot
		} else {
			connect(j, "╰") // north+east
		}
	}

	return strings.TrimRight(strings.Join(grid, ""), " "), lanes
}

// parentsFrom returns row.Parents from index i, nil-safe (a root commit has
// no Parents slice and Parents[i:] would panic on it).
func parentsFrom(row git.CommitRow, i int) []string {
	if len(row.Parents) <= i {
		return nil
	}
	return row.Parents[i:]
}

// laneIndexOf returns the index of the lane whose queue head is hash, or -1.
func laneIndexOf(lanes [][]string, hash string) int {
	for j, l := range lanes {
		if len(l) > 0 && l[0] == hash {
			return j
		}
	}
	return -1
}

// firstFreeLane returns the index of the first free lane, or -1.
func firstFreeLane(lanes [][]string) int {
	for j, l := range lanes {
		if len(l) == 0 {
			return j
		}
	}
	return -1
}

// cloneLanes deep-copies lane state so batch building never aliases the
// caller's slices (loadMoreCommits hands in the live model field).
func cloneLanes(open [][]string) [][]string {
	out := make([][]string, len(open))
	for i, l := range open {
		out[i] = append([]string(nil), l...)
	}
	return out
}

// trimTrailingFree drops trailing free lanes; interior free lanes stay as
// insertion slots. The three-index slice detaches capacity so later appends
// by the caller can't clobber shared backing arrays.
func trimTrailingFree(lanes [][]string) [][]string {
	end := len(lanes)
	for end > 0 && len(lanes[end-1]) == 0 {
		end--
	}
	return lanes[:end:end]
}
