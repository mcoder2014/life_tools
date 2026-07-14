# 发布说明

本仓库通过 GitHub Actions 在推送 `v*` tag 时自动构建发布包，并上传到 GitHub Release。PR 会执行 Go 单测和发布包打包 dry-run，但不会创建 Release。Python 单元测试在单独 workflow 里运行，失败只作为提醒，不阻塞发布包流程。Swift macOS App 和 Alfred Workflow 使用独立的 macOS workflow 验证。

## 触发方式

从最新 `master` 创建 tag 并推送。只有推送匹配 `v*` 的 tag 才会创建或更新 GitHub Release，PR 上的 release workflow 只做 dry-run：

```bash
git switch master
git pull --ff-only origin master
git tag -a v0.0.3 -m "life_tools v0.0.3"
git push origin v0.0.3
```

workflow 文件：

```text
.github/workflows/release.yml
.github/workflows/swift-mac-app.yml
.github/workflows/alfred-workflow.yml
```

触发条件：

```yaml
on:
  pull_request:
    paths:
      - '.github/workflows/release.yml'
      - '.github/workflows/go-test.yml'
      - '.github/workflows/python-test.yml'
      - 'docs/**'
      - 'README.MD'
      - 'AGENTS.md'
      - 'go.mod'
      - 'go.sum'
      - '**/*.go'
      - 'cli/video_subtitle/**'
      - 'emby_plugins/video_subtitle/**'
      - 'plugins/alfred_remote_upload/**'
      - 'sample/life_tools/**'
  push:
    tags:
      - 'v*'
```

`pull_request` 在 release workflow 中只做 Python 编译检查、Emby 插件测试和打包 dry-run；只有 tag push 才进入独立的 `Publish GitHub release` job，执行 `gh release create` 或 `gh release upload`。Go 测试由 `.github/workflows/go-test.yml` 单独执行，失败会阻塞 PR。Python 单元测试由 `.github/workflows/python-test.yml` 单独执行，失败时只写 GitHub warning 和 summary，本身仍返回成功，不阻塞发布包流程。

`swift-mac-app.yml` 使用 `macos-latest` runner。PR 和 `master` 推送会验证 `gui/interview_timer` 的 Swift 单测、可执行产物构建和 `.app` 打包；`v*` tag 会额外上传未签名的 `InterviewTimer.app` zip。

`alfred-workflow.yml` 使用 `macos-latest` runner。相关 PR 会检查 JXA、Shell、plist、离线测试和 `.alfredworkflow` 打包，并上传 dry-run artifact；不会创建 tag 或 Release。

## 发布操作流程

1. 先把发布相关 PR 合并到 `master`。
2. 在本地同步最新 `master`，创建新的 `v*` tag，并推送到远端。不要复用已经发布过的 tag；新版本用新 tag。
3. 打开 GitHub Actions 的 `Release` workflow，确认 tag 触发的 `Build release assets` job 成功。
4. 打开 GitHub 仓库的 Releases 页面，进入对应 tag，例如 `v0.0.3`，下载需要的 zip。
5. 如果需要 `InterviewTimer.app`，下载 `life_tools_interview_timer_macos_<tag>.zip`，解压后把 `InterviewTimer.app` 放到 `~/Applications` 或 `/Applications`。

也可以用 GitHub CLI 下载产物：

```bash
gh release view v0.0.3 --web
gh release download v0.0.3 --dir /tmp/life_tools_v0.0.3
cd /tmp/life_tools_v0.0.3
sha256sum -c checksums.txt
```

如果 tag push 后没有出现 Release，先检查 tag 名是否以 `v` 开头，再检查 Actions 里的 `Release` workflow 日志。

## 发布包

每次 tag 发布会生成这些资产：

```text
life_tools_linux_amd64_<tag>.zip
life_tools_linux_arm64_<tag>.zip
life_tools_darwin_amd64_<tag>.zip
life_tools_darwin_arm64_<tag>.zip
life_tools_video_subtitle_source_<tag>.zip
life_tools_emby_video_subtitle_plugin_<tag>.zip
life_tools_alfred_remote_upload_<tag>.alfredworkflow
life_tools_interview_timer_macos_<tag>.zip
life_tools_interview_timer_macos_<tag>.sha256
checksums.txt
```

Go 二进制包包含：

```text
bin/renameV1
bin/check_keywords
bin/retry_exec
bin/codex_hook_notify
bin/file_share
bin/codex_inspector
install.sh
sample/life_tools/*.json
docs/**
```

`codex_inspector` 虽然进入 Go 二进制发布包，但仍是实验工具，不进入根目录 `install.sh` 默认稳定安装清单。需要安装时优先使用 `cli/codex_inspector/install.sh` 或直接从 release zip 取 `bin/codex_inspector`。

`video_subtitle` 发布包包含 Python 源码、prompts、`requirements.txt`、示例配置和文档。它不是纯二进制工具，使用前仍需要 Python 依赖、ffmpeg、TOS、ASR、LLM 配置。

Emby 插件发布包只包含部署需要的插件 DLL 和文档。安装到 Emby 插件目录时只复制：

```text
LifeTools.Emby.VideoSubtitle.Emby.dll
```

不要把 `MediaBrowser.*`、`Emby.*` 或核心库 DLL 放进 Emby 插件目录。

`InterviewTimer` 发布包包含：

```text
InterviewTimer.app
```

该包由 `gui/interview_timer/scripts/build_app.sh` 生成，当前不做代码签名和 notarization。macOS 首次打开时可能需要用户在系统安全设置中手动允许。

Alfred 产物是可直接导入 Alfred 5 的 `life_tools_alfred_remote_upload_<tag>.alfredworkflow`。用户的 `hosts_json` 和 MRU 状态由 Alfred 及其 workflow data 目录管理，不进入发布包。

## CI 验证

发布前 release workflow 会运行：

```bash
python3 -m py_compile cli/video_subtitle/video_subtitle.py cli/video_subtitle/video_subtitle_test.py cli/video_subtitle/lib/*.py
dotnet test emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
dotnet build emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
```

Swift macOS App workflow 会运行：

```bash
cd gui/interview_timer
swift test
swift build --product InterviewTimerApp
./scripts/build_app.sh
```

Go 测试 workflow 会运行：

```bash
go test ./...
```

Python 测试提示 workflow 会运行：

```bash
python3 -m unittest cli/video_subtitle/video_subtitle_test.py
```

Alfred Workflow macOS CI 会运行：

```bash
plugins/alfred_remote_upload/tests/run.sh
plugins/alfred_remote_upload/tests/clipboard_integration.sh
plugins/alfred_remote_upload/build.sh
plutil -lint plugins/alfred_remote_upload/workflow/info.plist
unzip -t /tmp/life_tools_alfred_remote_upload_ci.alfredworkflow
```

Go 测试失败会阻塞 PR；Python 测试失败时只写 GitHub warning 和 summary，不阻塞 release workflow，也不阻止 tag 发布资产。

## 权限

Release workflow 顶层和构建 job 只需要只读权限：

```yaml
permissions:
  contents: read
```

只有 tag 触发的 `publish` job 提升为 `contents: write`，并通过 `GH_REPO` 显式指定当前仓库，用于 `gh release create` 和 `gh release upload`。PR dry-run 不具备仓库写权限。
