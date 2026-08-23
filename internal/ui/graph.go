package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// graphPalette cycles one color per graph column so parallel branches are
// visually separable. Colors chosen distinct from semantic colors already
// in use (hash yellow 3, refs cyan 6, badges 180/174/108).
var graphPalette = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("171")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("176")),
}

func colorGraph(graph string) string {
	if graph == "" {
		return ""
	}

	var b strings.Builder
	colIdx := 0
	for _, r := range graph {
		switch r {
		case '|':
			b.WriteString(graphPalette[colIdx%len(graphPalette)].Render("|"))
			colIdx++
		case '*':
			b.WriteString(graphPalette[colIdx%len(graphPalette)].Bold(true).Render("*"))
		case '/':
			b.WriteString(graphPalette[colIdx%len(graphPalette)].Render("/"))
			if colIdx > 0 {
				colIdx--
			}
		case '\\':
			b.WriteString(graphPalette[colIdx%len(graphPalette)].Render("\\"))
		case '_':
			b.WriteString(graphPalette[colIdx%len(graphPalette)].Render("_"))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
