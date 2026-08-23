package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"tgit/internal/git"
)

// graphPalette cycles one color per graph column so parallel branches are
// visually separable. Colors chosen distinct from semantic colors already
// in use (hash yellow 3, refs cyan 6, badges 180/174/108).
//
// graphFillers holds separate instances of the same colors for synthesized
// lane fillers: lipgloss v0.10's Bold(true) mutates the shared rules map,
// so a dot rendered through graphPalette permanently bolds that slot —
// fillers must not inherit it.
var (
	graphLaneColors = []string{"39", "171", "114", "214", "176"}
	graphPalette    = graphStyles(graphLaneColors)
	graphFillers    = graphStyles(graphLaneColors)
)

func graphStyles(colors []string) []lipgloss.Style {
	out := make([]lipgloss.Style, len(colors))
	for i, c := range colors {
		out[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c))
	}
	return out
}

func colorGraph(graph string) string {
	if graph == "" {
		return ""
	}

	var b strings.Builder
	colIdx := 0
	for _, r := range graph {
		appendGraphRune(&b, r, &colIdx)
	}
	return b.String()
}

// appendGraphRune styles one ASCII graph char into b, advancing *ci with the
// column rules shared by colorGraph and synthesizeGraphLanes ('|' opens a
// column, '/' closes one).
func appendGraphRune(b *strings.Builder, ch rune, ci *int) {
	switch ch {
	case '|':
		b.WriteString(graphPalette[*ci%len(graphPalette)].Render(mapGraphChars(ch)))
		*ci++
	case '*':
		b.WriteString(graphPalette[*ci%len(graphPalette)].Bold(true).Render(mapGraphChars(ch)))
	case '/':
		b.WriteString(graphPalette[*ci%len(graphPalette)].Render(mapGraphChars(ch)))
		if *ci > 0 {
			*ci--
		}
	case '\\', '_', '-':
		b.WriteString(graphPalette[*ci%len(graphPalette)].Render(mapGraphChars(ch)))
	default:
		b.WriteRune(ch)
	}
}

// synthesizeGraphLanes renders consecutive rows' graph prefixes into
// display-ready styled strings with lane continuity synthesized: a cell that
// is blank in its own row but sits in a lane occupied by the row above or
// below gains a │ filler colored like that neighboring lane. Without this,
// git's "* " rows stop short of far lanes and topology lines ("|\", "|/")
// render as isolated corner fragments.
//
// Output is already glyph-mapped and colored; callers must NOT run it through
// colorGraph again — plain fillers would shift colorGraph's column counter
// and recolor dots the wrong palette.
func synthesizeGraphLanes(rows []git.CommitRow) []string {
	n := len(rows)
	out := make([]string, n)
	if n == 0 {
		return out
	}

	grids := make([][]rune, n)
	maxW := 0
	for i, r := range rows {
		grids[i] = []rune(r.Graph)
		if len(grids[i]) > maxW {
			maxW = len(grids[i])
		}
	}

	isOccupied := func(ch rune) bool {
		switch ch {
		case '|', '*', '/', '\\', '_', '-':
			return true
		}
		return false
	}

	occ := make([][]bool, n)
	ciOf := make([][]int, n)
	for i := range grids {
		occ[i] = make([]bool, maxW)
		ciOf[i] = make([]int, maxW)
		ci := 0
		for c, ch := range grids[i] {
			if !isOccupied(ch) {
				continue
			}
			occ[i][c] = true
			ciOf[i][c] = ci
			switch ch {
			case '|':
				ci++
			case '/':
				if ci > 0 {
					ci--
				}
			}
		}
	}

	filler := func(colorIdx int) string {
		return graphFillers[colorIdx%len(graphFillers)].Render("│")
	}

	for i := range grids {
		var b strings.Builder
		rowCi := 0
		for c := 0; c < maxW; c++ {
			if c < len(grids[i]) && grids[i][c] != ' ' {
				appendGraphRune(&b, grids[i][c], &rowCi)
				continue
			}
			switch {
			case i > 0 && occ[i-1][c]:
				b.WriteString(filler(ciOf[i-1][c]))
			case i+1 < n && occ[i+1][c]:
				b.WriteString(filler(ciOf[i+1][c]))
			default:
				b.WriteRune(' ')
			}
		}
		s := b.String()
		for strings.HasSuffix(s, " ") {
			s = s[:len(s)-1]
		}
		out[i] = s
	}
	return out
}
