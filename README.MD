# life_tools

个人日常使用的小工具集合，按应用类型组织源码：命令行程序放在 `cli/`，插件程序放在 `emby_plugins/`，图形界面程序放在 `gui/`。

## 快速安装

支持 Linux 和 macOS。仓库根目录执行下面命令，默认安装稳定命令行工具：`renameV1`、`check_keywords`、`retry_exec`、`codex_hook_notify`、`video_subtitle`、`file_share`。

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

自定义安装目录和配置目录：

```bash
./install.sh --prefix "$HOME/.local" --config-dir "$HOME/.config/life_tools"
```

`video_subtitle` 的 Python 依赖默认不会自动安装，需要时显式开启：

```bash
./install.sh --tool video_subtitle --with-python-deps
```

完整安装说明见 [docs/install.md](docs/install.md)。

## 工具清单

| 分类 | 工具 | 源码目录 | 产物/入口 | 简述 | 详细文档 |
|---|---|---|---|---|---|
| CLI | `renameV1` | `cli/renameV1/` | `renameV1` | 按配置批量重命名媒体文件 | [docs/cli/renameV1.md](docs/cli/renameV1.md) |
| CLI | `check_keywords` | `cli/check_keywords/` | `check_keywords` | 检查 git diff 新增行中的敏感词或正则 | [docs/cli/check_keywords.md](docs/cli/check_keywords.md) |
| CLI | `retry_exec` | `cli/retry_exec/` | `retry_exec` | 给任意命令增加重试、退避和失败通知 | [docs/cli/retry_exec.md](docs/cli/retry_exec.md) |
| CLI | `codex_hook_notify` | `cli/codex_hook_notify/` | `codex_hook_notify` | 接收 Codex hook 并推送飞书提醒 | [docs/cli/codex_hook_notify.md](docs/cli/codex_hook_notify.md) |
| CLI | `file_share` | `cli/file_share/` | `file_share` | 临时启动只读 HTTP 文件分享服务 | [docs/cli/file_share.md](docs/cli/file_share.md) |
| CLI | `video_subtitle` | `cli/video_subtitle/` | `video_subtitle` | 为单个视频生成简体中文字幕 | [docs/cli/video_subtitle.md](docs/cli/video_subtitle.md) |
| Plugin | Emby 字幕插件 | `emby_plugins/video_subtitle/` | `LifeTools.Emby.VideoSubtitle.Emby.dll` | 在 Emby 后台调用 `video_subtitle` 生成字幕 | [docs/plugins/emby_video_subtitle.md](docs/plugins/emby_video_subtitle.md) |
| GUI | InterviewTimer | `gui/interview_timer/` | `InterviewTimer.app` | macOS 面试悬浮计时器 | [docs/gui/interview_timer.md](docs/gui/interview_timer.md) |

## 项目结构

| 路径 | 内容 |
|---|---|
| `cli/` | 命令行工具源码。Go CLI 和 Python CLI 都放在这里。 |
| `emby_plugins/` | 插件类应用源码。当前只有 Emby 字幕插件。 |
| `gui/` | GUI 应用源码。当前只有 InterviewTimer macOS App。 |
| `docs/cli/` | CLI 工具的设计、结构、流程和使用文档。 |
| `docs/plugins/` | 插件类应用的设计、结构、流程和部署文档。 |
| `docs/gui/` | GUI 应用的设计、结构、流程和安装文档。 |
| `sample/life_tools/` | 示例配置文件。 |
| `build.sh` | 构建 Go CLI 工具到 `output/`。 |
| `install.sh` | 安装 CLI 工具、示例配置和可选 hook。 |

## 构建与验证

构建 Go CLI：

```bash
./build.sh
```

Python CLI 语法检查：

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m py_compile \
  cli/video_subtitle/video_subtitle.py \
  cli/video_subtitle/video_subtitle_test.py \
  cli/video_subtitle/lib/*.py
```

Emby 插件验证：

```bash
dotnet test emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln
dotnet build emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln
```

InterviewTimer 验证：

```bash
cd gui/interview_timer
swift test
swift build --product InterviewTimerApp
./scripts/build_app.sh
```

具体工具的测试和排障入口见各自详细文档。

## 发布

推送 `v*` tag 会通过 GitHub Actions 构建 GitHub Release 资产，包括 Linux/macOS Go 二进制包、`video_subtitle` 源码包、Emby 插件包和未签名的 `InterviewTimer.app` 包。发布流程见 [docs/release.md](docs/release.md)。
