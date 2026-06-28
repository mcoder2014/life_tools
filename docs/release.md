# 发布说明

本仓库通过 GitHub Actions 在推送 `v*` tag 时自动构建发布包，并把 zip 上传到 GitHub Release。PR 会执行同一套测试和打包 dry-run，但不会创建 Release。

## 触发方式

从最新 `master` 创建 tag 并推送：

```bash
git checkout master
git pull --ff-only origin master
git tag v0.0.3
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

`pull_request` 只做测试和打包 dry-run；只有 tag push 才执行 `gh release create` 或 `gh release upload`。

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

发布前 workflow 会运行：

```bash
go test ./...
python3 -m unittest video_subtitle/video_subtitle_test.py
python3 -m py_compile video_subtitle/video_subtitle.py video_subtitle/video_subtitle_test.py video_subtitle/lib/*.py
dotnet test emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
dotnet build emby_plugins/video_subtitle/LifeTools.Emby.VideoSubtitle.sln --configuration Release
```

## 权限

Release workflow 需要：

```yaml
permissions:
  contents: write
```

这是 `gh release create` 和 `gh release upload` 上传资产所需的最小仓库权限。
