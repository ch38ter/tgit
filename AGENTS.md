# PROJECT KNOWLEDGE BASE

**Generated:** 2026-08-22
**Commit:** 3c0ad83
**Branch:** master

## OVERVIEW

tgit —— 三栏终端 git TUI 查看器（只读）：header（仓库/分支/变更数）、文件列表或 commit diff（中栏）、提交图（下栏）。Go 1.22 + bubbletea/lipgloss/fsnotify，经 os/exec 包装 git CLI，编译为单个静态二进制。

## STRUCTURE

```
tgit/
├── main.go            # 入口：tea.NewProgram(InitialModel, AltScreen+MouseCellMotion)，仅此一事
├── Makefile           # build / test / clean
├── internal/git/      # git CLI 类型化包装层，零 charmbracelet 依赖；一文件一个关注点
│   ├── exec.go        # RunGit：唯一进程执行口；注入 GIT_OPTIONAL_LOCKS=0；repo root 每次重解析
│   ├── info.go        # RepoInfo{Toplevel,Branch,UserName}；三项独立查询、静默降级
│   ├── status.go      # porcelain=v2 → []FileChange；XY 折叠单字节状态；quote.c 路径反转义
│   ├── log.go         # log --graph --oneline --decorate --all → []CommitRow；变长 hash 解析
│   └── diff.go        # ShowCommit(show --format=fuller --stat -p)；FileDiff 回退链 worktree→cached→占位文案
└── internal/ui/       # bubbletea Elm 架构；单向依赖 ui→git
    ├── app.go         # 全部 model 状态 + Init/Update/View + fsnotify 400ms debounce 刷新 + 样式（~714 行）
    └── commitdiff.go  # styleCommitStat：stat 区块定位 + === 分隔线 + 计数着色；commitDiffWidth
```

## WHERE TO LOOK

| 任务 | 位置 | 注意 |
|------|------|------|
| 加按键/鼠标交互 | app.go `Update` 的 KeyMsg/MouseMsg switch | j/k 按 focusedPane 分发；diff 视图中 j/k 归 viewport |
| 改布局/边框/样式 | app.go 顶部 styles var block + `View()` | 先读 ANTI-PATTERNS #2/#4/#5 |
| 新增 git 查询 | internal/git 建新文件，一律经 `RunGit` | 勿绕过（log.go 是历史遗留例外） |
| diff 内容着色 | commitdiff.go `styleCommitStat` | 汇总行锚定 + 向上回溯文件行；patch 正文不碰 |
| 解析器边界情况 | status_test.go / log_test.go 已有中文路径、tab 文件名、core.abbrev=12 用例 | 先补测试再改解析 |

## CODE MAP

| Symbol | Location | 引用热度 | 角色 |
|--------|----------|---------|------|
| RunGit | internal/git/exec.go | 全部 git 调用 | 进程边界唯一入口 |
| ParseStatus | internal/git/status.go | 11 处调用 | 状态解析核心 |
| GetRepoInfo | internal/git/info.go | 2 | header 元数据 |
| LogGraph(At) | internal/git/log.go | 6 | 提交列表数据源 |
| FileDiff / ShowCommit | internal/git/diff.go | 4 / 3 | 中栏 diff 数据源 |
| InitialModel → model | internal/ui/app.go | main | 导出构造器返回未导出类型（既有约定） |
| sanitizeTabs | internal/ui/app.go | 每次 SetContent 前 | tab 宽度消毒 |
| styleCommitStat | internal/ui/commitdiff.go | enter 分支 | commit diff 着色 |

## CONVENTIONS

- 依赖单向：ui→git。git 包禁止 import bubbletea/lipgloss。
- 用户可控值（hash/refname/path）一律作为单个 argv 元素传 `exec.Command("git", ...)`——严禁拼接 shell 字符串（这是含空格路径、特殊字符 ref 安全的原因）。
- 静默降级契约：空仓库/detached HEAD → `Branch=""` 不报错；缺 user.name → `UserName=""` 不报错；空仓库 `git log` 非零退出无 stdout → 视为空提交列表。勿"修复"成 error。
- 提交信息遵循 Conventional Commits（现有历史全部 feat/fix/chore/docs）。

## ANTI-PATTERNS (THIS PROJECT)

1. **勿缓存 repo root**（exec.go `resolveRepoRoot`）——测试用 os.Chdir 进临时目录，缓存必脏。RunGit docstring 里写的 "resolved once and cached" 是陈旧注释，以代码为准（uncached by design）。
2. **diff viewport 宽度 = `commitDiffWidth(m.width)`（width−6）**，不是 width−2——否则终端换行导致 bubbletea 行级渲染失步、切回文件视图残留脏字符（源自 fix 3c0ad83）。回归测试 `TestDiffViewNoLineOverflow` 必须保持绿。
3. **内容进 viewport 前必须过 `sanitizeTabs`**——lipgloss 按 1 格计宽、终端按最多 8 格展开 tab，撑行留残影。
4. **不要把已着色文本嵌进 Strikethrough 样式**——lipgloss 逐字符渲染删除线会搅碎内嵌 ANSI（见 renderFileLine 注释）；应分段各自 Render 后拼接。
5. **边框用 `asciiBorder`（-\|+），不用 Unicode 制表符**——规避跨终端字形宽度计算问题。
6. **视图切换（diff 进入/退出等）发 `tea.ClearScreen`** 强制整屏重绘，防止部分重绘残影。

## UNIQUE STYLES / NOTES

- 缩略 hash 非定长 7：仓库变大后 git 自动加长到 9+，解析须吃全部连续 hex（log_test 以 core.abbrev=12 锁定该行为）。
- 含非 ASCII/特殊字符的路径会被 git quote.c C 转义，展示前须过 `unquoteGitPath`（status_test 有中文名与 tab 文件名用例）。
- fsnotify watcher：walk 时跳过 .git 但单独 watch .git 本身；事件经 400ms debounce 合并触发 reload；R 键手动刷新。
- loadRepoData 排序规则：无 "/" 的文件在前、含 "/" 的目录在后，组内字典序。
- `mapGraphChars` 目前是恒等函数（预留的 graph 字符扩展点）。
- header 分隔符用 ASCII `|`（暗灰 240），不用 U+00B7 中点——后者是东亚歧义宽度，部分终端字体渲染 2 格而 lipgloss 计 1 格，会把右边框顶出对位（fix b7fcaf5）。同理慎用其他歧义宽度字符做行内装饰。
- log.go `LogGraphAt` 绕过 RunGit 自建 exec.Command（无 GIT_OPTIONAL_LOCKS=0）——已知不一致；改动该文件时优先收敛到 RunGit。
- gofmt 对 app.go / status.go / app_test.go 会报预存对齐偏差：改动时勿顺手全文件格式化，避免噪音 diff。
- 无 cmd/ 目录（package main 在根）；go.mod module 名为裸 `tgit`；`tgit` 二进制是 .gitignore 覆盖的构建产物。

## COMMANDS

```bash
make build    # CGO_ENABLED=0 go build -ldflags "-s -w" -o tgit .
make test     # go test ./...
go vet ./...  # 提交前建议
make clean    # rm tgit

# ⚠️ 构建完成后必须替换 PATH 中的二进制（日常使用的就是这份）：
cp -f tgit ~/.local/bin/tgit
```
