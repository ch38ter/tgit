package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"tgit/internal/git"
)

const headerHeight = 3

var (
	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	middleStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	bottomStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	notGitRepoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	// Status line styles
	untrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))                // red
	modifiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)    // yellow + bold
	deletedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))                // red
	selectedStyle  = lipgloss.NewStyle().Reverse(true)                                  // reverse video
)

type model struct {
	width         int
	height        int
	isGitRepo     bool
	branch        string
	userName      string
	toplevel      string
	changes       []git.FileChange
	selectedIndex int
	currentView   string
}

// repoDataMsg carries loaded repository data from the init command.
type repoDataMsg struct {
	info    git.RepoInfo
	changes []git.FileChange
}

func InitialModel() *model {
	return &model{
		isGitRepo:   true,
		currentView: "files",
	}
}

func (m *model) Init() tea.Cmd {
	// Check if git exists in PATH
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "tgit: git not found in PATH")
		os.Exit(1)
	}

	// Check if we're inside a git repository
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = wd
	if err := cmd.Run(); err != nil {
		m.isGitRepo = false
		return nil
	}

	return loadRepoData
}

// loadRepoData is a tea.Cmd that loads repo info and git status.
func loadRepoData() tea.Msg {
	info, err := git.GetRepoInfo()
	if err != nil {
		return repoDataMsg{info: git.RepoInfo{}, changes: nil}
	}

	raw, err := git.RunGit("status", "--porcelain=v2", "--branch")
	if err != nil {
		return repoDataMsg{info: info, changes: nil}
	}

	_, changes, err := git.ParseStatus(raw)
	if err != nil {
		return repoDataMsg{info: info, changes: nil}
	}

	// Sort: files (no /) before directories (has /), each group byte-order ascending
	sort.SliceStable(changes, func(i, j int) bool {
		iHasSlash := strings.Contains(changes[i].Path, "/")
		jHasSlash := strings.Contains(changes[j].Path, "/")
		if iHasSlash != jHasSlash {
			return !iHasSlash // files (no slash) before directories (has slash)
		}
		return changes[i].Path < changes[j].Path
	})

	return repoDataMsg{info: info, changes: changes}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if len(m.changes) > 0 {
				m.selectedIndex++
				if m.selectedIndex >= len(m.changes) {
					m.selectedIndex = 0 // wrap to top
				}
			}
		case "k", "up":
			if len(m.changes) > 0 {
				m.selectedIndex--
				if m.selectedIndex < 0 {
					m.selectedIndex = len(m.changes) - 1 // wrap to bottom
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case repoDataMsg:
		m.toplevel = msg.info.Toplevel
		m.branch = msg.info.Branch
		m.userName = msg.info.UserName
		m.changes = msg.changes
		// Clamp selectedIndex after data reload
		if len(m.changes) == 0 {
			m.selectedIndex = 0
		} else if m.selectedIndex >= len(m.changes) {
			m.selectedIndex = len(m.changes) - 1
		} else if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
	}
	return m, nil
}

func (m *model) View() string {
	if !m.isGitRepo {
		return notGitRepoStyle.Render("not a git repository") + "\n\nPress q to quit"
	}

	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Calculate pane heights
	remaining := m.height - headerHeight
	if remaining < 2 {
		remaining = 2
	}
	middleHeight := remaining * 5 / 9
	bottomHeight := remaining - middleHeight
	if middleHeight < 1 {
		middleHeight = 1
	}
	if bottomHeight < 1 {
		bottomHeight = 1
	}

	// Render panes with lipgloss
	header := headerStyle.Width(m.width).Height(headerHeight).Render(m.renderHeader())
	middle := middleStyle.Width(m.width).Height(middleHeight).Render(m.renderMiddle())
	bottom := bottomStyle.Width(m.width).Height(bottomHeight).Render("bottom pane")

	return lipgloss.JoinVertical(lipgloss.Left, header, middle, bottom)
}

// renderHeader renders the top pane with repo info.
func (m *model) renderHeader() string {
	toplevel := m.toplevel
	if home := os.Getenv("HOME"); home != "" {
		if strings.HasPrefix(toplevel, home) {
			toplevel = "~" + strings.TrimPrefix(toplevel, home)
		}
	}

	count := len(m.changes)
	changeStr := fmt.Sprintf("%d changed", count)

	return fmt.Sprintf("%s  %s  %s\n%s", toplevel, m.branch, m.userName, changeStr)
}

// renderMiddle renders the file change list.
func (m *model) renderMiddle() string {
	if len(m.changes) == 0 {
		return "no changes"
	}

	var lines []string
	for i, fc := range m.changes {
		text := fmt.Sprintf("[%c] %s", fc.Status, fc.Path)
		if i == m.selectedIndex {
			lines = append(lines, selectedStyle.Render(text))
		} else {
			lines = append(lines, statusStyle(fc.Status).Render(text))
		}
	}

	return strings.Join(lines, "\n")
}

// statusStyle returns the lipgloss style for a given status byte.
func statusStyle(status byte) lipgloss.Style {
	switch status {
	case 'U':
		return untrackedStyle
	case 'M':
		return modifiedStyle
	case 'D':
		return deletedStyle
	default:
		return lipgloss.NewStyle()
	}
}
