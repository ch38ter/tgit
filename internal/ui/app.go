package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/muesli/reflow/truncate"
	"tgit/internal/git"
)

const headerHeight = 3

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

	untrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	modifiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	deletedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("174")).Strikethrough(true)
	selectedStyle  = lipgloss.NewStyle().Reverse(true)
	inactiveSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	filePathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	deletedPathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Strikethrough(true)

	// Commit row styles
	commitHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	commitRefsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan

	// Diff view styles
	diffTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	headerPathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	headerBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	headerUserStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	headerSepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	headerCleanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Bold(true)
	headerCountStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	badgeModifiedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	badgeDeletedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("174"))
	badgeUntrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
)

const (
	focusedBorderColor   = "63"
	unfocusedBorderColor = "240"
)

type focusedPane int

const (
	focusFiles focusedPane = iota
	focusCommits
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
	selectedFile   int
	selectedCommit int
	focusedPane    focusedPane
	currentView   string
	diffContent   string
	diffTitle     string
	diffViewport  viewport.Model

	commitsExhausted bool

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

type commitsAppendedMsg struct {
	rows      []git.CommitRow
	exhausted bool
}

func InitialModel() *model {
	return &model{
		isGitRepo:      true,
		currentView:    "files",
		diffViewport:   viewport.New(80, 20),
		selectedFile:   -1,
		selectedCommit: -1,
		focusedPane:    focusFiles,
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
				return m, tea.ClearScreen
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
		case "tab", "shift+tab":
			if m.currentView != "files" {
				return m, nil
			}
			if m.focusedPane == focusFiles {
				m.focusedPane = focusCommits
			} else {
				m.focusedPane = focusFiles
			}
		case "j", "down":
			if m.currentView == "diff" {
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				return m, cmd
			}
			switch m.focusedPane {
			case focusFiles:
				if len(m.changes) > 0 {
					if m.selectedFile < 0 {
						m.selectedFile = 0
					} else {
						m.selectedFile++
						if m.selectedFile >= len(m.changes) {
							m.selectedFile = 0
						}
					}
				}
			case focusCommits:
				if len(m.commits) > 0 {
					if m.selectedCommit < 0 {
						m.selectedCommit = 0
					} else {
						next := m.selectedCommit + 1
						if next >= len(m.commits) {
							if !m.commitsExhausted {
								m.selectedCommit = len(m.commits) - 1
								return m, m.loadMoreCommits()
							}
							m.selectedCommit = 0
						} else {
							m.selectedCommit = next
						}
					}
				}
			}
		case "k", "up":
			if m.currentView == "diff" {
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				return m, cmd
			}
			switch m.focusedPane {
			case focusFiles:
				if len(m.changes) > 0 {
					if m.selectedFile < 0 {
						m.selectedFile = 0
					} else {
						m.selectedFile--
						if m.selectedFile < 0 {
							m.selectedFile = len(m.changes) - 1
						}
					}
				}
			case focusCommits:
				if len(m.commits) > 0 {
					if m.selectedCommit < 0 {
						m.selectedCommit = 0
					} else {
						m.selectedCommit--
						if m.selectedCommit < 0 {
							m.selectedCommit = len(m.commits) - 1
						}
					}
				}
			}
		case "enter":
			if m.currentView != "files" {
				return m, nil
			}
			switch m.focusedPane {
			case focusFiles:
				if len(m.changes) > 0 && m.selectedFile >= 0 && m.selectedFile < len(m.changes) {
					fc := m.changes[m.selectedFile]
					diff, err := git.FileDiff(fc.Path)
					if err != nil {
						diff = err.Error()
					}
					m.diffContent = diff
					m.diffTitle = fc.Path
					m.diffViewport.YOffset = 0
					m.diffViewport.SetContent(sanitizeTabs(diff))
					m.currentView = "diff"
					return m, tea.ClearScreen
				}
			case focusCommits:
				if len(m.commits) > 0 && m.selectedCommit >= 0 && m.selectedCommit < len(m.commits) {
					commit := m.commits[m.selectedCommit]
					diff, err := git.ShowCommit(commit.Hash)
					if err != nil {
						diff = err.Error()
					}
					m.diffContent = diff
					m.diffTitle = commit.Hash
					m.diffViewport.YOffset = 0
					m.diffViewport.SetContent(sanitizeTabs(styleCommitStat(diff, commitDiffWidth(m.width))))
					m.currentView = "diff"
					return m, tea.ClearScreen
				}
			}
		case "esc":
			if m.currentView == "diff" {
				m.currentView = "files"
				return m, tea.ClearScreen
			}
		}
	case tea.MouseMsg:
		if m.currentView != "files" {
			return m, nil
		}
		if msg.Type != tea.MouseLeft {
			return m, nil
		}
		if m.width == 0 || m.height == 0 {
			return m, nil
		}
		middleHeight, bottomHeight := m.paneHeights()
		headerH := headerHeight + 2
		middleH := middleHeight + 2
		bottomH := bottomHeight + 2
		y := msg.Y
		if y < headerH {
			return m, nil
		} else if y < headerH+middleH {
			m.focusedPane = focusFiles
			if len(m.changes) > 0 {
				rel := y - headerH - 1
				if rel < 0 {
					rel = 0
				}
				if rel >= len(m.changes) {
					rel = len(m.changes) - 1
				}
				m.selectedFile = rel
			}
		} else if y < headerH+middleH+bottomH {
			m.focusedPane = focusCommits
			if len(m.commits) > 0 {
				rel := y - headerH - middleH - 1
				if rel < 0 {
					rel = 0
				}
				if rel >= len(m.commits) {
					rel = len(m.commits) - 1
				}
				m.selectedCommit = rel
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.diffViewport.Width = msg.Width - 2
		if m.diffViewport.Width < 1 {
			m.diffViewport.Width = 1
		}
	case repoDataMsg:
		m.toplevel = msg.info.Toplevel
		m.branch = msg.info.Branch
		m.userName = msg.info.UserName
		m.changes = msg.changes
		m.commits = msg.commits
		m.commitsExhausted = false
		if len(m.changes) == 0 {
			m.selectedFile = -1
		} else if m.selectedFile >= len(m.changes) {
			m.selectedFile = len(m.changes) - 1
		}
		if len(m.commits) == 0 {
			m.selectedCommit = -1
		} else if m.selectedCommit >= len(m.commits) {
			m.selectedCommit = len(m.commits) - 1
		}
	case reloadMsg:
		return m, tea.Batch(m.reload(), m.waitForReload())
	case commitsAppendedMsg:
		existing := make(map[string]bool, len(m.commits))
		for _, c := range m.commits {
			existing[c.Hash] = true
		}
		appended := 0
		for _, r := range msg.rows {
			if r.Hash == "" || existing[r.Hash] {
				continue
			}
			m.commits = append(m.commits, r)
			existing[r.Hash] = true
			appended++
		}
		m.selectedCommit += appended
		if msg.exhausted || len(msg.rows) < 200 {
			m.commitsExhausted = true
		}
		return m, nil
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

	middleHeight, bottomHeight := m.paneHeights()

	w := m.width - 2
	if w < 1 {
		w = 1
	}
	middleAvail := middleHeight
	bottomAvail := bottomHeight

	// Dynamic border colors: focused pane bright (63), unfocused dim (240); header always 63
	headerSt := headerStyle.BorderForeground(lipgloss.Color(focusedBorderColor))
	middleBorder := unfocusedBorderColor
	bottomBorder := unfocusedBorderColor
	if m.currentView == "files" {
		if m.focusedPane == focusFiles {
			middleBorder = focusedBorderColor
		} else {
			bottomBorder = focusedBorderColor
		}
	} else {
		middleBorder = focusedBorderColor
	}
	header := headerSt.Width(w).Height(headerHeight).Render(m.renderHeader())

	var middle string
	if m.currentView == "diff" {
		middle = middleStyle.BorderForeground(lipgloss.Color(middleBorder)).Width(w).Height(middleHeight).Render(m.renderDiff(middleHeight))
	} else {
		middle = middleStyle.BorderForeground(lipgloss.Color(middleBorder)).Width(w).Height(middleHeight).Render(m.renderMiddleSized(middleAvail))
	}

	bottom := bottomStyle.BorderForeground(lipgloss.Color(bottomBorder)).Width(w).Height(bottomHeight).Render(m.renderBottomSized(bottomAvail))

	return lipgloss.JoinVertical(lipgloss.Left, header, middle, bottom)
}

func (m *model) paneHeights() (int, int) {
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
	return middleHeight, bottomHeight
}

func joinSegments(segs []string) string {
	var out string
	for i, seg := range segs {
		if i > 0 {
			// ASCII pipe, not U+00B7: the middle dot is East-Asian-ambiguous
			// width and renders 2 cells in some terminal fonts, pushing the
			// right border out of alignment (lipgloss counts it as 1).
			out += headerSepStyle.Render(" | ")
		}
		out += seg
	}
	return out
}

func (m *model) renderHeader() string {
	w := m.width - 4
	if w < 1 {
		w = 1
	}

	toplevel := m.toplevel
	if home := os.Getenv("HOME"); home != "" {
		if strings.HasPrefix(toplevel, home) {
			toplevel = "~" + strings.TrimPrefix(toplevel, home)
		}
	}

	var line1Segs []string
	if toplevel != "" {
		line1Segs = append(line1Segs, headerPathStyle.Render(toplevel))
	}
	line1 := truncate.StringWithTail(joinSegments(line1Segs), uint(w), "...")

	var line2Segs []string
	if m.branch != "" {
		line2Segs = append(line2Segs, headerBranchStyle.Render(m.branch))
	}
	if m.userName != "" {
		line2Segs = append(line2Segs, headerUserStyle.Render(m.userName))
	}
	line2 := truncate.StringWithTail(joinSegments(line2Segs), uint(w), "...")

	var line3 string
	if len(m.changes) == 0 {
		line3 = headerCleanStyle.Render("clean")
	} else {
		left := headerCountStyle.Render(strconv.Itoa(len(m.changes))) + " changed"

		counts := make(map[byte]int)
		for _, c := range m.changes {
			counts[c.Status]++
		}

		var badges []string
		if n := counts['M']; n > 0 {
			badges = append(badges, badgeModifiedStyle.Render("[M]"+strconv.Itoa(n)))
		}
		if n := counts['D']; n > 0 {
			badges = append(badges, badgeDeletedStyle.Render("[D]"+strconv.Itoa(n)))
		}
		if n := counts['U']; n > 0 {
			badges = append(badges, badgeUntrackedStyle.Render("[U]"+strconv.Itoa(n)))
		}
		right := strings.Join(badges, " ")

		gap := w - lipgloss.Width(left) - lipgloss.Width(right)
		if gap >= 1 {
			line3 = left + strings.Repeat(" ", gap) + right
		} else {
			line3 = truncate.StringWithTail(left+" "+right, uint(w), "...")
		}
	}

	return strings.Join([]string{line1, line2, line3}, "\n")
}

func (m *model) loadMoreCommits() tea.Cmd {
	loaded := len(m.commits)
	return func() tea.Msg {
		rows, err := git.LogGraphAppendAt(".", loaded, 200)
		if err != nil {
			return commitsAppendedMsg{exhausted: true}
		}
		return commitsAppendedMsg{rows: rows, exhausted: len(rows) < 200}
	}
}

func (m *model) renderMiddle() string {
	_, mid := m.paneHeights()
	return m.renderMiddleSized(mid)
}

func (m *model) renderMiddleSized(availableRows int) string {
	if len(m.changes) == 0 {
		return ""
	}

	contentWidth := m.width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	if availableRows < 0 {
		availableRows = 0
	}

	start := 0
	if m.selectedFile >= 0 && len(m.changes) > availableRows && availableRows > 0 {
		start = m.selectedFile - availableRows + 1
		if start < 0 {
			start = 0
		}
		maxStart := len(m.changes) - availableRows
		if start > maxStart {
			start = maxStart
		}
	}

	end := len(m.changes)
	if availableRows > 0 && end-start > availableRows {
		end = start + availableRows
	}

	var lines []string
	for i := start; i < end; i++ {
		fc := m.changes[i]
		line := truncate.StringWithTail(m.renderFileLine(fc), uint(contentWidth), "...")
		if i == m.selectedFile {
			if m.focusedPane == focusFiles && m.currentView == "files" {
				lines = append(lines, selectedStyle.Render(line))
			} else {
				lines = append(lines, inactiveSelectedStyle.Render(line))
			}
		} else {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// renderFileLine renders one change row as "[<status>] <name>  /<path>", with
// the basename first and the full path (root = /filename) in a lighter gray.
// The status style applies only to the ANSI-free prefix and the gray path is a
// separate segment — nesting pre-styled text inside a strikethrough style
// (lipgloss renders it per character) would mangle the embedded ANSI.
func (m *model) renderFileLine(fc git.FileChange) string {
	prefix := fmt.Sprintf("[%c] %s", fc.Status, filepath.Base(fc.Path))
	row := statusStyle(fc.Status).Render(prefix)
	pathStyle := filePathStyle
	if fc.Status == 'D' {
		pathStyle = deletedPathStyle
	}
	return row + "  " + pathStyle.Render("/"+fc.Path)
}

// renderDiff renders the diff view with title line and viewport.
func (m *model) renderDiff(middleHeight int) string {
	// Content box inside the bordered, padded pane: terminal width minus the
	// outer 2-column trim minus border (2) and padding (2). Clamping every
	// rendered line to this width prevents terminal line-wrapping, which would
	// push rows out of sync with bubbletea's renderer and leave stale
	// characters on screen when switching back to the files view.
	vpWidth := commitDiffWidth(m.width)
	vpHeight := middleHeight - 3 // account for borders and title line
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.diffViewport.Width = vpWidth
	m.diffViewport.Height = vpHeight

	title := truncate.StringWithTail(diffTitleStyle.Render(m.diffTitle), uint(vpWidth), "...")
	content := m.diffViewport.View()

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

// sanitizeTabs replaces tabs with spaces so a terminal never expands a tab
// beyond the width lipgloss assumed (lipgloss counts a tab as a single cell,
// but terminals render it as up to 8 columns), which would wrap lines and
// leave residue on screen.
func sanitizeTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
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

func (m *model) renderBottom(height int) string {
	visibleRows := height - 2
	if visibleRows < 1 {
		visibleRows = 1
	}
	return m.renderBottomSized(visibleRows)
}

func (m *model) renderBottomSized(visibleRows int) string {
	if len(m.commits) == 0 {
		return "no commits"
	}

	if visibleRows < 1 {
		visibleRows = 1
	}

	selectedIdx := m.selectedCommit
	if selectedIdx >= len(m.commits) {
		selectedIdx = len(m.commits) - 1
	}

	contentWidth := m.width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	start := 0
	if selectedIdx >= 0 && len(m.commits) > visibleRows && visibleRows > 0 {
		start = selectedIdx - visibleRows + 1
		if start < 0 {
			start = 0
		}
		maxStart := len(m.commits) - visibleRows
		if start > maxStart {
			start = maxStart
		}
	}

	end := len(m.commits)
	if visibleRows > 0 && end-start > visibleRows {
		end = start + visibleRows
	}

	var lines []string
	for i := start; i < end; i++ {
		line := truncate.StringWithTail(renderCommitLine(m.commits[i]), uint(contentWidth), "...")
		if i == selectedIdx {
			if m.focusedPane == focusCommits && m.currentView == "files" {
				lines = append(lines, selectedStyle.Render(line))
			} else {
				lines = append(lines, inactiveSelectedStyle.Render(line))
			}
		} else {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// renderCommitLine renders a single commit row with graph, hash, refs, and message.
func renderCommitLine(commit git.CommitRow) string {
	graph := colorGraph(mapGraphChars(commit.Graph))
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
