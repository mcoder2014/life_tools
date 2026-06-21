# 安装说明

这份文档面向人工用户和 AI Agent。目标是快速判断该装什么、怎么装、装完怎么验证。不要猜工具名，直接按下面的清单执行。

## 支持范围

- 系统：Linux、macOS。
- Shell：`bash`。
- Go 工具：需要本机有 `go`，版本以 `go.mod` 为准。
- `video_subtitle`：需要 `python3`；运行时还需要 `ffmpeg` 和 `ffprobe`。
- 默认安装路径：可执行文件放到 `/usr/local/bin`，Python 工具文件放到 `/usr/local/lib/life_tools`。
- 默认配置路径：`/etc/life_tools`，可用 `--config-dir` 改变安装脚本写入位置。

写入 `/usr/local`、`/etc/life_tools`、`/var/log` 时可能需要 `sudo`。脚本会在需要时调用 `sudo`，不会覆盖已经存在的配置文件。

## 工具清单

| 工具名 | 安装后的命令 | 类型 | 默认安装 | 配置文件 |
|---|---|---|---|---|
| `renameV1` | `renameV1` | Go | 是 | 使用工作目录下的 `rename_v1.json` |
| `check_keywords` | `check_keywords` | Go | 是 | `/etc/life_tools/check_keywords.json` |
| `retry_exec` | `retry_exec` | Go | 是 | `/etc/life_tools/retry_exec.json` |
| `codex_hook_notify` | `codex_hook_notify` | Go | 是 | `/etc/life_tools/codex_hook_notify.json` |
| `video_subtitle` | `video_subtitle` | Python | 是 | `/etc/life_tools/video_subtitle.json` |
| `file_share` | `file_share` | Go | 是 | `/etc/life_tools/file_share.json` |

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
```

常用帮助命令：

```bash
renameV1 -h
check_keywords -h
retry_exec --help
codex_hook_notify -h
video_subtitle --help
file_share -h
```

## 脚本行为

- Go 工具会先构建到 `output/`，再安装到目标 `bin` 目录。
- 示例配置只在目标文件不存在时安装，已有配置不会被覆盖。
- `retry_exec` 会创建 `/var/log/retry_exec` 并设置为当前用户可写。
- `codex_hook_notify` 的日志目录按系统选择：
  - Linux：`/var/log/codex_hook_notify`
  - macOS：`~/Library/Logs/codex_hook_notify`
  - 其他系统：`~/.codex_hook_notify/logs`
- `video_subtitle` 会复制 `video_subtitle/` 到 `<prefix>/lib/life_tools/video_subtitle`，再安装一个同名包装命令。
