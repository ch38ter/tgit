# tgit

[中文](README.md) | English

> A three-pane Git viewer for your terminal: working-tree status, commit diffs, and a colored commit graph on a single screen. Strictly read-only — it never touches your worktree.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## ✨ Highlights

- **Three panes, one screen**: repo/branch/change counters up top, file list or commit diff in the middle, colorized commit graph at the bottom — no more juggling `git status`, `git log`, and `git show`.
- **Read-only by design**: everything runs through the git CLI via `os/exec` with `GIT_OPTIONAL_LOCKS=0` injected. It never writes to `.git` or touches the index, so you can open it on any repo without hesitation.
- **Commit graphs that make sense**: multi-lane layout, `◉` marking merge points, `┼` crossings that preserve vertical lane lines, refs right-aligned. Octopus merges and nested merge chains are drawn correctly.
- **Live repo tracking**: watches the repository (`.git` included) via fsnotify; events are debounced for 400ms before an automatic refresh. Prefer manual? Hit `r`.
- **Polished against real-world repos**: non-ASCII paths, tab characters in filenames, quote.c escaping, variable-length abbreviated hashes (core.abbrev auto-extension) — all covered by parser tests.

## 📦 Installation

Requires Go 1.22+ and `git` on your PATH:

```bash
git clone https://github.com/ch38ter/tgit.git
cd tgit
make build
cp tgit ~/.local/bin/    # anywhere on your PATH works
```

The build produces a single static binary (CGO_ENABLED=0) with no runtime dependencies beyond git itself.

## 🚀 Quick Start

```bash
cd some-git-repo
tgit
```

You land on the three-pane view: `j`/`k` to move, `enter` to open a diff, `b` to filter branches, `q` to quit.

## ⌨️ Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` (or `↓` / `↑`) | Move selection; scrolls content in diff view |
| `tab` / `shift+tab` | Toggle focus: file list ⇄ commit graph |
| `enter` | Open diff for selected file / full diff for selected commit |
| `b` | Open the branch filter picker |
| `space` | Toggle branch checkmarks in the picker (multi-select supported) |
| `esc` | Close picker / return from diff |
| `r` | Manual refresh |
| `q` / `ctrl+c` | Quit |

The commit list pages automatically when you scroll past the end — older history loads on demand. Mouse clicks work too, for switching pane focus and selecting rows.

## 🧭 Usage Example

A typical session:

1. On launch, the bottom graph shows history across all branches (`--all`);
2. Press `b` to open the branch picker, move with `j`/`k`, check multiple branches with `space` for combined filtering (or just `enter` to pick one);
3. `tab` to focus the commit graph, then `enter` to inspect a commit — the stat block gets `===` separators with green `+` / red `-` count coloring;
4. Save a file elsewhere and the UI refreshes itself, always reflecting the latest state and history.

## 📁 Project Structure

```
tgit/
├── main.go            # Entry point: boots the bubbletea program
├── internal/git/      # Typed wrapper over the git CLI (zero UI dependencies)
│   ├── exec.go        # RunGit: the single process execution gateway
│   ├── info.go        # Repo metadata (root/branch/user name)
│   ├── status.go      # porcelain=v2 status parsing
│   ├── log.go         # Commit graph data (--graph --oneline --decorate --all)
│   └── diff.go        # File diffs and commit diffs
└── internal/ui/       # bubbletea Elm-architecture UI
    ├── app.go         # Model state machine + rendering
    ├── lanes.go       # Multi-lane commit graph layout algorithm
    └── commitdiff.go  # Commit --stat block coloring
```

Architectural rule: `internal/git` only parses git output into typed data; `internal/ui` depends on it one-way. Neither layer crosses the boundary.

## ❓ When to Use

- Checking repo state and history quickly over SSH or on a server, without a GUI;
- Reviewing merge history where lane topology and merge points matter;
- Any situation where you want a look-only tool that cannot disturb your worktree.

## 🛠️ Development

```bash
make test      # go test ./...
go vet ./...   # static analysis
make build     # CGO_ENABLED=0 static build
make clean     # remove build artifacts
```

## 🤝 Contributing

Issues and PRs welcome. If you touch parsing logic, add tests first — `internal/git/*_test.go` has reference cases for CJK paths, tab filenames, and variable-length hashes.

## 📝 License

[MIT](LICENSE) — free to use, copy, modify, and distribute (commercial use included), as long as the copyright notice is preserved.
