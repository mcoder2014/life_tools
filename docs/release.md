# 发布说明

本仓库通过 GitHub Actions 在推送 `v*` tag 时自动构建发布包，并把 zip 上传到 GitHub Release。PR 会执行发布包打包 dry-run，但不会创建 Release。Go 和 Python 单元测试在单独 workflow 里运行，失败只作为提醒，不阻塞发布包流程。

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
```

触发条件：

```yaml
on:
  pull_request:
    paths:
      - '.github/workflows/release.yml'
      - '.github/workflows/go-test.yml'
      - '.github/workflows/python-test.yml'
      - 'docs/release.md'
      - 'README.MD'
      - 'AGENTS.md'
      - 'go.mod'
      - 'go.sum'
      - '**/*.go'
      - 'video_subtitle/**'
      - 'emby_plugins/video_subtitle/**'
      - 'sample/life_tools/**'
  push:
    tags:
      - 'v*'
```

`pull_request` 在 release workflow 中只做 Python 编译检查、Emby 插件测试和打包 dry-run；只有 tag push 才执行 `gh release create` 或 `gh release upload`。Go 测试由 `.github/workflows/go-test.yml` 单独执行，Python 单元测试由 `.github/workflows/python-test.yml` 单独执行。测试失败时 reminder workflow 只写 GitHub warning 和 summary，本身仍返回成功，不阻塞发布包流程。

## 发布操作流程

1. 先把发布相关 PR 合并到 `master`。
2. 在本地同步最新 `master`，创建新的 `v*` tag，并推送到远端。不要复用已经发布过的 tag；新版本用新 tag。
3. 打开 GitHub Actions 的 `Release` workflow，确认 tag 触发的 `Build release assets` job 成功。
4. 打开 GitHub 仓库的 Releases 页面，进入对应 tag，例如 `v0.0.3`，下载需要的 zip。

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
checksums.txt
```

Go 二进制包包含：

```text
bin/renameV1
bin/check_keywords
bin/retry_exec
bin/codex_hook_notify
bin/file_share
install.sh
sample/life_tools/*.json
docs/*.md
```

`video_subtitle` 发布包包含 Python 源码、prompts、`requirements.txt`、示例配置和文档。它不是纯二进制工具，使用前仍需要 Python 依赖、ffmpeg、TOS、ASR、LLM 配置。

Emby 插件发布包只包含部署需要的插件 DLL 和文档。安装到 Emby 插件目录时只复制：

```text
LifeTools.Emby.VideoSubtitle.Emby.dll
```

不要把 `MediaBrowser.*`、`Emby.*` 或核心库 DLL 放进 Emby 插件目录。

## CI 验证

发布前 release workflow 会运行：

```bash
python3 -m py_compile video_subtitle/video_subtitle.py video_subtitle/video_subtitle_test.py video_subtitle/lib/*.py
dotnet test emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
dotnet build emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
```

测试提示 workflow 会运行：

```bash
go test ./...
python3 -m unittest video_subtitle/video_subtitle_test.py
```

这些 workflow 失败时只写 GitHub warning 和 summary，不阻塞 release workflow，也不阻止 tag 发布资产。

## 权限

Release workflow 需要：

```yaml
permissions:
  contents: write
```

这是 `gh release create` 和 `gh release upload` 上传资产所需的最小仓库权限。
