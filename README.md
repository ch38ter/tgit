# tgit

中文 | [English](README.en.md)

> 终端里的三栏 Git 查看器：工作区状态、commit diff、彩色提交图一屏尽览，纯只读，不动你的工作区。

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-lightgrey)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## ✨ 项目亮点

- **三栏一屏**：顶部是仓库/分支/变更计数，中间是文件列表或 commit diff，底部是彩色提交图——不用在 `git status`、`git log`、`git show` 之间来回切。
- **只读安全**：全部经 `os/exec` 调用 git CLI 查询，注入 `GIT_OPTIONAL_LOCKS=0`，绝不会写入 `.git` 或触碰索引，随便开在别人的仓库上也不心虚。
- **看得懂的提交图**：多泳道布局，`◉` 醒目标记 merge 点，泳道交叉用 `┼` 保留竖线走向，分支 refs 右对齐，octopus / 多级合并都能画对。
- **实时跟进仓库变化**：基于 fsnotify 监听仓库（含 `.git`），事件经 400ms debounce 合并后自动刷新；也可以按 `r` 手动刷。
- **为真实仓库打磨的细节**：中文路径、含 tab 的文件名、非 ASCII 引号转义、变长缩略 hash（core.abbrev 自动加长）都有解析测试兜底。

## 📦 安装

需要 Go 1.22+ 和 PATH 中的 `git`：

```bash
git clone https://github.com/ch38ter/tgit.git
cd tgit
make build
cp tgit ~/.local/bin/    # 放进 PATH 即可
```

构建产出单个静态二进制（CGO_ENABLED=0），无其他运行时依赖。

## 🚀 快速开始

```bash
cd 某个git仓库
tgit
```

启动即见三栏视图：`j`/`k` 移动选中项，`enter` 看 diff，`b` 筛选分支，`q` 退出。

## ⌨️ 按键说明

| 按键 | 作用 |
|------|------|
| `j` / `k`（或 `↓` / `↑`） | 上下移动选中项；diff 视图中滚动内容 |
| `tab` / `shift+tab` | 切换焦点：文件列表 ⇄ 提交图 |
| `enter` | 查看选中文件的 diff / 选中提交的完整 diff |
| `b` | 打开分支筛选器 |
| `空格` | 筛选器中勾选/取消分支（可多选组合过滤） |
| `esc` | 关闭筛选器 / 从 diff 返回列表 |
| `r` | 手动刷新 |
| `q` / `ctrl+c` | 退出 |

提交列表滚到底会自动分页加载更早的历史，无需手动翻页。也支持鼠标点击切换焦点窗格和选中行。

## 🧭 使用示例

典型工作流：

1. 启动后底部提交图默认展示全部分支（`--all`）的历史；
2. 按 `b` 打开分支筛选器，`j`/`k` 移到目标分支，`空格` 勾选多个分支做组合过滤（或直接 `enter` 单选）；
3. `tab` 把焦点切到提交图，`enter` 查看某次提交的 diff——stat 区块带 `===` 分隔线，增删行数按 绿`+`/红`-` 着色；
4. 改动代码保存后界面自动刷新，随时反映最新的状态与历史。

## 📁 项目结构

```
tgit/
├── main.go            # 入口：启动 bubbletea 程序
├── internal/git/      # git CLI 类型化包装层（零 UI 依赖）
│   ├── exec.go        # RunGit：唯一的进程执行口
│   ├── info.go        # 仓库元信息（根目录/分支/用户名）
│   ├── status.go      # porcelain=v2 状态解析
│   ├── log.go         # 提交图数据（--graph --oneline --decorate --all）
│   └── diff.go        # 文件 diff 与 commit diff
└── internal/ui/       # bubbletea Elm 架构界面
    ├── app.go         # model 状态机 + 渲染
    ├── lanes.go       # 多泳道提交图布局算法
    └── commitdiff.go  # commit --stat 区块着色
```

架构约定：`internal/git` 只负责把 git 输出解析成类型化数据，`internal/ui` 单向依赖它，两层互不越界。

## ❓ 适用场景

- 在服务器或 SSH 会话里快速看一眼仓库状态和历史，不想开 GUI；
- review 合并历史时想直观看到泳道走向和 merge 点；
- 需要一个不会误改工作区的纯查看工具。

## 🛠️ 开发

```bash
make test      # go test ./...
go vet ./...   # 静态检查
make build     # CGO_ENABLED=0 静态编译
make clean     # 清理构建产物
```

## 🤝 贡献

欢迎 Issue 和 PR。改动解析逻辑请先补测试（`internal/git/*_test.go` 已有中文路径、tab 文件名、变长 hash 的用例可参考）。

## 📝 License

[MIT](LICENSE) —— 可自由使用、复制、修改、分发（含商用），只需保留版权声明。
