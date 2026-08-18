package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"tgit/internal/git"
)

const headerHeight = 2

// ASCII border: avoids Unicode box-drawing width calculation issues
var asciiBorder = lipgloss.Border{
	Top:         "-",
	Bottom:      "-",
	Left:        "|",
	Right:       "|",
	TopLeft:     "+",
	TopRight:    "+",
	BottomLeft:  "+",
	BottomRight: "+",
}

var (
	headerStyle = lipgloss.NewStyle().
			Border(asciiBorder).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	middleStyle = lipgloss.NewStyle().
			Border(asciiBorder).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	bottomStyle = lipgloss.NewStyle().
			Border(asciiBorder).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	notGitRepoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	// Status line styles (colors chosen for visibility on dark background)
	untrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))            // cyan
	modifiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true) // yellow + bold
	deletedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))            // magenta
	selectedStyle  = lipgloss.NewStyle().Reverse(true)                              // reverse video

	// Commit row styles
	commitHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	commitRefsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan

	// Diff view styles
	diffTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)
)

type model struct {
	width         int
	height        int
	isGitRepo     bool
	branch        string
	userName      string
	toplevel      string
	changes       []git.FileChange
	commits       []git.CommitRow
	selectedIndex int
	currentView   string
	diffContent   string
	diffTitle     string
	diffViewport  viewport.Model

	watcher       *fsnotify.Watcher
	debounceTimer *time.Timer
	needsReload   bool
	reloadChan    chan struct{}
	debounceMu    sync.Mutex
}

// repoDataMsg carries loaded repository data from the init command.
type repoDataMsg struct {
	info    git.RepoInfo
	changes []git.FileChange
	commits []git.CommitRow
}

type reloadMsg struct{}

func InitialModel() *model {
	return &model{
		isGitRepo:     true,
		currentView:   "files",
		diffViewport:  viewport.New(80, 20),
		selectedIndex: -1, // no selection by default
	}
}

func (m *model) Init() tea.Cmd {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "tgit: git not found in PATH")
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	if !git.IsGitRepo() {
		m.isGitRepo = false
		return nil
	}

	m.setupWatcher(wd)

	return tea.Batch(loadRepoData, m.waitForReload())
}

func (m *model) setupWatcher(wd string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tgit: failed to create file watcher:", err, "(auto-refresh disabled, use R to refresh)")
		return
	}
	m.watcher = watcher
	m.reloadChan = make(chan struct{}, 1)

	gitDir := filepath.Join(wd, ".git")
	filepath.Walk(wd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path == gitDir {
				return filepath.SkipDir
			}
			if werr := watcher.Add(path); werr != nil {
				fmt.Fprintln(os.Stderr, "tgit: failed to watch", path, ":", werr)
			}
		}
		return nil
	})

	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		if err := watcher.Add(gitDir); err != nil {
			fmt.Fprintln(os.Stderr, "tgit: failed to watch .git:", err)
		}
	}

	go m.watchLoop()
}

func (m *model) watchLoop() {
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			_ = event
			m.debounceMu.Lock()
			m.needsReload = true
			if m.debounceTimer != nil {
				m.debounceTimer.Stop()
			}
			m.debounceTimer = time.AfterFunc(400*time.Millisecond, func() {
				m.debounceMu.Lock()
				needs := m.needsReload
				m.needsReload = false
				m.debounceMu.Unlock()
				if needs {
					select {
					case m.reloadChan <- struct{}{}:
					default:
					}
				}
			})
			m.debounceMu.Unlock()
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintln(os.Stderr, "tgit: watcher error:", err)
		}
	}
}

func (m *model) waitForReload() tea.Cmd {
	return func() tea.Msg {
		<-m.reloadChan
		return reloadMsg{}
	}
}

func (m *model) reload() tea.Cmd {
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

	commits, err := git.LogGraph(200)
	if err != nil {
		commits = nil
	}

	return repoDataMsg{info: info, changes: changes, commits: commits}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.currentView == "diff" {
				m.currentView = "files"
				return m, nil
			}
			if m.watcher != nil {
				m.watcher.Close()
			}
			return m, tea.Quit
		case "r", "R":
			m.debounceMu.Lock()
			m.needsReload = false
			if m.debounceTimer != nil {
				m.debounceTimer.Stop()
			}
			m.debounceMu.Unlock()
			return m, m.reload()
		case "j", "down":
			if m.currentView == "diff" {
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				return m, cmd
			}
			if len(m.changes) > 0 {
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				} else {
					m.selectedIndex++
					if m.selectedIndex >= len(m.changes) {
						m.selectedIndex = 0
					}
				}
			}
		case "k", "up":
			if m.currentView == "diff" {
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				return m, cmd
			}
			if len(m.changes) > 0 {
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				} else {
					m.selectedIndex--
					if m.selectedIndex < 0 {
						m.selectedIndex = len(m.changes) - 1
					}
				}
			}
		case "enter":
			if m.currentView == "files" && m.selectedIndex >= 0 {
				if len(m.commits) > 0 && m.selectedIndex < len(m.commits) {
					commit := m.commits[m.selectedIndex]
					diff, err := git.ShowCommit(commit.Hash)
					if err != nil {
						diff = err.Error()
					}
					m.diffContent = diff
					m.diffTitle = commit.Hash
					m.diffViewport.YOffset = 0
					m.diffViewport.SetContent(diff)
					m.currentView = "diff"
					return m, nil
				}
				if len(m.changes) > 0 && m.selectedIndex < len(m.changes) {
					fc := m.changes[m.selectedIndex]
					diff, err := git.FileDiff(fc.Path)
					if err != nil {
						diff = err.Error()
					}
					m.diffContent = diff
					m.diffTitle = fc.Path
					m.diffViewport.YOffset = 0
					m.diffViewport.SetContent(diff)
					m.currentView = "diff"
					return m, nil
				}
			}
		case "esc":
			if m.currentView == "diff" {
				m.currentView = "files"
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.diffViewport.Width = msg.Width - 4
		if m.diffViewport.Width < 1 {
			m.diffViewport.Width = 1
		}
	case repoDataMsg:
		m.toplevel = msg.info.Toplevel
		m.branch = msg.info.Branch
		m.userName = msg.info.UserName
		m.changes = msg.changes
		m.commits = msg.commits
		// Keep selectedIndex = -1 (no selection); only clamp if out of range
		if len(m.changes) == 0 {
			m.selectedIndex = -1
		} else if m.selectedIndex >= len(m.changes) {
			m.selectedIndex = len(m.changes) - 1
		}
	case reloadMsg:
		return m, tea.Batch(m.reload(), m.waitForReload())
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
	// Each pane adds 2 border lines (top+bottom); 3 panes → 6 extra lines.
	remaining := m.height - headerHeight - 6
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

	// Render panes with lipgloss.
	// lipgloss Width(w) sets CONTENT width; borders add 2 chars → total = w+2.
	// Use width-4 for safety margin to ensure right border is visible.
	w := m.width - 4
	if w < 1 {
		w = 1
	}
	header := headerStyle.Width(w).Height(headerHeight).Render(m.renderHeader())

	var middle string
	if m.currentView == "diff" {
		middle = middleStyle.Width(w).Height(middleHeight).Render(m.renderDiff(middleHeight))
	} else {
		middle = middleStyle.Width(w).Height(middleHeight).Render(m.renderMiddle())
	}

	bottom := bottomStyle.Width(w).Height(bottomHeight).Render(m.renderBottom(bottomHeight))

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
		return ""
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

// renderDiff renders the diff view with title line and viewport.
func (m *model) renderDiff(middleHeight int) string {
	vpWidth := m.width - 4
	vpHeight := middleHeight - 3 // account for borders and title line
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.diffViewport.Width = vpWidth
	m.diffViewport.Height = vpHeight

	title := diffTitleStyle.Render(m.diffTitle)
	content := m.diffViewport.View()

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
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

// renderBottom renders the commit graph pane.
func (m *model) renderBottom(height int) string {
	if len(m.commits) == 0 {
		return "no commits"
	}

	// Visible rows: subtract 2 for top/bottom border
	visibleRows := height - 2
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Clamp selected index for bottom pane (keep -1 = no selection)
	selectedIdx := m.selectedIndex
	if selectedIdx >= len(m.commits) {
		selectedIdx = len(m.commits) - 1
	}

	var lines []string
	for i := 0; i < visibleRows && i < len(m.commits); i++ {
		line := renderCommitLine(m.commits[i])
		if i == selectedIdx {
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// renderCommitLine renders a single commit row with graph, hash, refs, and message.
func renderCommitLine(commit git.CommitRow) string {
	graph := mapGraphChars(commit.Graph)
	hash := commitHashStyle.Render(commit.Hash)
	refs := commitRefsStyle.Render(commit.Refs)

	if commit.Refs != "" {
		return fmt.Sprintf("%s %s %s %s", graph, hash, refs, commit.Msg)
	}
	return fmt.Sprintf("%s %s %s", graph, hash, commit.Msg)
}

// mapGraphChars keeps git log --graph ASCII characters as-is for terminal compatibility.
func mapGraphChars(s string) string {
	return s
}
