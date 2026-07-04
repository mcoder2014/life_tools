# 安装说明

这份文档面向人工用户和 AI Agent。目标是快速判断该装什么、怎么装、装完怎么验证。不要猜工具名，直接按下面的清单执行。

## 支持范围

- 系统：Linux、macOS。
- Shell：`bash`。
- Go 工具：需要本机有 `go`，版本以 `go.mod` 为准。
- `video_subtitle`：需要 `python3`；运行时还需要 `ffmpeg` 和 `ffprobe`。
- `InterviewTimer`：macOS 图形应用，需要 macOS 13+ 和 Swift/Apple 开发工具链，不由根目录 `install.sh` 默认安装。
- 默认安装路径：可执行文件放到 `/usr/local/bin`，Python 工具文件放到 `/usr/local/lib/life_tools`。
- 默认配置路径：`/etc/life_tools`，可用 `--config-dir` 改变安装脚本写入位置。
- `codex_inspector` 是实验工具，使用 `cli/codex_inspector/install.sh` 单独安装；默认安装到 `$HOME/.local/bin`，不写系统目录。
- `life_codex_server` 和 `life_codex_agent` 是实验工具，默认不安装；必须显式指定 `--tool`。

写入 `/usr/local`、`/etc/life_tools` 和 Linux 的 `/var/log` 时可能需要 `sudo`。脚本会在需要时调用 `sudo`，不会覆盖已经存在的配置文件。

## 工具清单

| 工具名 | 安装后的命令 | 类型 | 默认安装 | 配置文件 |
|---|---|---|---|---|
| `renameV1` | `renameV1` | Go | 是 | 使用工作目录下的 `rename_v1.json` |
| `check_keywords` | `check_keywords` | Go | 是 | `/etc/life_tools/check_keywords.json` |
| `retry_exec` | `retry_exec` | Go | 是 | `/etc/life_tools/retry_exec.json` |
| `codex_hook_notify` | `codex_hook_notify` | Go | 是 | `/etc/life_tools/codex_hook_notify.json` |
| `video_subtitle` | `video_subtitle` | Python | 是 | `/etc/life_tools/video_subtitle.json` |
| `file_share` | `file_share` | Go | 是 | `/etc/life_tools/file_share.json` |
| `codex_inspector` | `codex_inspector` | Go | 否 | 无配置文件，默认只读 `~/.codex` |
| `life_codex_server` | `life_codex_server` | Go + Web | 否 | `/etc/life_tools/life_codex_server.json` |
| `life_codex_agent` | `life_codex_agent` | Go | 否 | `/etc/life_tools/life_codex_agent.json` |
| `InterviewTimer` | `InterviewTimer.app` | SwiftPM macOS App | 否 | `~/Library/Application Support/InterviewTimer/` |

## 快速安装

安装默认稳定工具：

```bash
./install.sh
```

只安装指定工具：

```bash
./install.sh --tool retry_exec
./install.sh --tool renameV1 --tool check_keywords
./install.sh --tools retry_exec,codex_hook_notify
./install.sh --tool file_share
./install.sh --tool life_codex_server --tool life_codex_agent
```

安装到自定义前缀：

```bash
./install.sh --prefix "$HOME/.local"
```

配置目录也可以改，适合无 sudo 权限或测试安装：

```bash
./install.sh --prefix "$HOME/.local" --config-dir "$HOME/.config/life_tools"
```

这只改变安装脚本写入示例配置的位置。部分工具源码里的默认配置路径仍是 `/etc/life_tools`，运行时需要用命令参数指定自定义配置路径。

## life_codex

`life_codex_server` 提供网页和中心服务，`life_codex_agent` 在每台计算机器前台运行并调用本机 `codex app-server --stdio`。

安装前先构建网页：

```bash
npm --prefix web/life_codex install
npm --prefix web/life_codex run build
```

安装：

```bash
./install.sh --tool life_codex_server --tool life_codex_agent
```

配置文件：

```text
/etc/life_tools/life_codex_server.json
/etc/life_tools/life_codex_agent.json
```

运行：

```bash
life_codex_server -config /etc/life_tools/life_codex_server.json
life_codex_agent enroll -config /etc/life_tools/life_codex_agent.json -server http://127.0.0.1:8899 -token <token> -name local -roots /path/to/workspace
life_codex_agent serve -config /etc/life_tools/life_codex_agent.json
```

详细行为、审计风险和本机模拟部署见 [cli/life_codex.md](cli/life_codex.md)。


## file_share

只安装 `file_share`：

```bash
./install.sh --tool file_share
```

脚本会构建并安装 `file_share`，并在配置目录不存在 `file_share.json` 时安装示例配置。默认配置路径是：

```text
/etc/life_tools/file_share.json
```

运行示例：

```bash
file_share /path/to/file-or-dir
file_share -addr 127.0.0.1:9000 /path/a /path/b
file_share -config /etc/life_tools/file_share.json
```

`file_share` 默认无认证，用于个人临时分享。不要把含敏感文件、隐藏文件或符号链接的目录暴露到不可信网络。

## codex_inspector

`codex_inspector` 是实验工具，用来在本机浏览器里只读查看 Codex 会话、活跃度统计和记忆内容。它不在 `install.sh` 默认稳定安装清单中，也不会写入 `~/.codex`。

快速安装：

```bash
./cli/codex_inspector/install.sh
codex_inspector -addr 127.0.0.1:8787
```

默认安装路径是：

```text
$HOME/.local/bin/codex_inspector
```

macOS 和 Linux 权限策略一致：默认只写当前用户目录，不调用 `sudo`。如果目标目录不在 `PATH` 中，脚本会打印需要加入 shell profile 的提示。

安装到自定义用户目录：

```bash
./cli/codex_inspector/install.sh --prefix "$HOME/.local"
```

系统级安装必须显式确认：

```bash
./cli/codex_inspector/install.sh --system
```

`--system` 会安装到 `/usr/local/bin/codex_inspector`，并在目录不可写时使用 `sudo`。如果使用其他系统目录，可以组合 `--prefix` 和 `--allow-sudo`：

```bash
./cli/codex_inspector/install.sh --prefix /opt/life_tools --allow-sudo
```

也可以不安装，直接构建和启动：

```bash
go build -o output/codex_inspector ./cli/codex_inspector/...
./output/codex_inspector -addr 127.0.0.1:8787
```

可指定脱敏 fixture 或其他 Codex home：

```bash
./output/codex_inspector -codex-home /tmp/codex-fixture -addr 127.0.0.1:8787
```

历史会话 summary 默认缓存到本机 SQLite 文件，用于加速重复浏览。可指定或禁用：

```bash
./output/codex_inspector -cache-path /tmp/codex_inspector_cache.sqlite
./output/codex_inspector -no-cache
./output/codex_inspector -cache-workers 2
./output/codex_inspector -cache-workers 0
```

默认 cache 路径放在系统临时目录下的用户隔离子目录，避免 macOS/Linux 上用户 cache 目录权限异常影响页面使用：

| 系统 | 默认 cache |
|---|---|
| macOS | `${TMPDIR:-/tmp}/life_tools-codex-inspector-<uid>/session_summary_cache.sqlite` |
| Linux | `/tmp/life_tools-codex-inspector-<uid>/session_summary_cache.sqlite` |

cache 生命周期：

| 状态 | 含义 | 操作 |
|---|---|---|
| `missing` | cache 文件不存在，页面实时解析 JSONL | Diagnostics 点击 `Create cache` |
| `healthy` | cache 可用，历史 summary 可复用 | Diagnostics 点击 `Fill missing cache` |
| `corrupt` | SQLite 明确报 `malformed`、`file is not a database` 或 `schema is corrupt` | Diagnostics 二次确认后点击 `Backup and rebuild cache` |
| `rebuilding` | 后台正在补齐或重建 | 查看 total/done/cached/skipped/failed |
| `disabled` | 使用 `-no-cache` 禁用 | 仅实时解析 JSONL |
| `unavailable` | 权限、busy timeout 或其他非损坏错误 | 页面实时解析 JSONL，先处理错误原因 |

`-cache-workers` 只控制自动补齐。默认值是 2；设为 0 时不会在启动后自动补齐，但 Diagnostics 上的手动创建或重建仍可执行一次。

安全边界：

- 默认监听 `127.0.0.1`，不要绑定到公网地址。
- 只读取 `session_index.jsonl`、`sessions/`、`memories/` 和 SQLite schema。
- 不写入 `~/.codex`；SQLite cache 默认写到系统 tmp 下的用户隔离目录，目录权限为 `0700`，文件权限为 `0600`；也可以用 `-cache-path` 指定位置。
- cache 只保存脱敏后的 `SessionSummary`、token 聚合、文件大小和修改时间，不保存 raw JSONL 或完整对话内容。
- 不读取 `auth.json` 内容，不展示 token、cookie、secret 类字段。
- 详细说明见 [cli/codex_inspector.md](cli/codex_inspector.md)。

## InterviewTimer

`InterviewTimer` 是 macOS 面试悬浮计时 App，源码位于：

```text
gui/interview_timer
```

它不是命令行工具，不会被仓库根目录 `install.sh` 安装。构建和安装：

```bash
cd gui/interview_timer
swift test
./scripts/build_app.sh
mkdir -p "$HOME/Applications"
ditto dist/InterviewTimer.app "$HOME/Applications/InterviewTimer.app"
open "$HOME/Applications/InterviewTimer.app"
```

模板目录：

```text
~/Library/Application Support/InterviewTimer/templates/
```

仓库内预置模板在 `gui/interview_timer/template-presets/`。完整说明见 [gui/interview_timer.md](gui/interview_timer.md)。

## video_subtitle

只安装 `video_subtitle`：

```bash
./install.sh --tool video_subtitle
```

脚本会复制 Python 工具文件，安装 `/usr/local/bin/video_subtitle` 包装命令，并安装示例配置到：

```text
/etc/life_tools/video_subtitle.json
```

Python 依赖默认不自动安装，避免脚本改坏用户的 Python 环境。需要脚本顺手安装依赖时显式加参数：

```bash
./install.sh --tool video_subtitle --with-python-deps
```

运行前还要确保系统里有：

```bash
ffmpeg
ffprobe
```

## codex_hook_notify

只安装命令和示例配置：

```bash
./install.sh --tool codex_hook_notify
```

安装 Codex `Stop` hook：

```bash
./install.sh --tool codex_hook_notify --install-codex-hook
```

同时安装 `PermissionRequest` hook：

```bash
./install.sh --tool codex_hook_notify --install-codex-hook --with-permission-request
```

`PermissionRequest` 在 approval 模式下会很频繁，不要默认开启。

## AI Agent 安装步骤

1. 进入仓库根目录。
2. 读取本文件和 `README.MD`，确认目标工具名。
3. 执行 `./install.sh --tool <工具名>`；需要多个工具就重复 `--tool`。
4. 如果安装 `video_subtitle`，确认 `python3`、`ffmpeg`、`ffprobe` 和 Python 依赖。
5. 编辑 `/etc/life_tools/*.json` 中的真实配置，密钥和 webhook 不要写回仓库。
6. 用 `command -v <命令>` 和对应命令的 `--help` 或 `-h` 做最小验证。


## 验证命令

```bash
command -v renameV1
command -v check_keywords
command -v retry_exec
command -v codex_hook_notify
command -v video_subtitle
command -v file_share
test -d "$HOME/Applications/InterviewTimer.app"
```

常用帮助或启动命令：

```bash
renameV1 -h
check_keywords -h
retry_exec --help
codex_hook_notify -h
video_subtitle --help
file_share -h
open "$HOME/Applications/InterviewTimer.app"
```

## 脚本行为

- Go 工具会先构建到 `output/`，再安装到目标 `bin` 目录。
- 示例配置只在目标文件不存在时安装，已有配置不会被覆盖。
- `retry_exec` 的日志目录按系统选择：
  - Linux：`/var/log/retry_exec`
  - macOS：`~/Library/Logs/retry_exec`
- `codex_hook_notify` 的日志目录按系统选择：
  - Linux：`/var/log/codex_hook_notify`
  - macOS：`~/Library/Logs/codex_hook_notify`
  - 其他系统：`~/.codex_hook_notify/logs`
- `video_subtitle` 会复制 `cli/video_subtitle/` 到 `<prefix>/lib/life_tools/video_subtitle`，再安装一个同名包装命令。
- `InterviewTimer` 需要在 `gui/interview_timer` 下单独构建 `.app`，根目录安装脚本不会复制或覆盖 macOS 应用。
