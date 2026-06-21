# AGENTS.md

## 基本判断

这是一个 Go 1.18 的个人小工具仓库，不是框架项目。改动前先问三件事：

1. 这是当前工具真实存在的问题吗？
2. 有没有更简单的改法？
3. 会不会破坏已有命令、配置文件或用户数据？

不要为了抽象而抽象。小工具的正确方向是少概念、少状态、少分支。

## 仓库结构

- `renameV1/`：批量重命名命令行工具，入口是 `package main`。
- `save_work/`：敏感词检测工具，构建产物名是 `check_keywords`。
- `video_subtitle/`：单视频自动生成中文字幕工具，详细说明见 `docs/video_subtitle.md`。
- `docs/`：项目文档，`video_subtitle` 的使用和维护说明在 `docs/video_subtitle.md`。
- `sample/life_tools/`：示例配置文件。
- `renameV1/testData/`：`renameV1` 的测试数据，包含媒体文件、nfo、bif 等样例。
- `output/`：构建产物目录，不要把它当源码维护。

没有 `Makefile`、`justfile` 或 `Taskfile.yml`。优先遵循 README 和现有脚本。

## 构建与测试

优先使用仓库已有构建入口：

```bash
./build.sh
```

该脚本会构建：

- `./output/renameV1` from `./renameV1/...`
- `./output/check_keywords` from `./save_work/...`
- `./output/retry_exec` from `./retry_exec/...`
- `./output/codex_hook_notify` from `./codex_hook_notify/...`

测试优先使用：

```bash
go test ./...
```

也可以按包缩小范围：

```bash
go test ./renameV1
go test ./save_work
```

涉及代码、配置、依赖或构建脚本的改动，交付前必须跑相应测试或构建。纯文档改动不需要假装跑 Go 测试，但要检查 diff。

## Go 代码规则

- 保持 Go 1.18 兼容，不随手升级语言版本。
- 跟随现有风格：简单 `package main` 工具、标准库优先、必要时使用已有依赖。
- 日志沿用 `github.com/sirupsen/logrus`。
- JSON 沿用 `github.com/json-iterator/go`。
- 测试断言沿用 `github.com/stretchr/testify/require`。
- 不为小工具引入新框架、新配置系统或复杂依赖。
- 不吞掉 `.Error` 或返回值错误；现在已有代码有粗糙处，新增代码不要继续扩大问题。

## `renameV1` 规则

`renameV1` 会实际修改文件名。默认把它当高风险工具处理。

- `-rename` 会调用 `os.Rename` 执行真实重命名。
- 默认不要使用 `-skip_double_check`，除非测试环境可回滚。
- 修改重命名逻辑前，优先用 `renameV1/testData/` 或临时副本验证。
- 不要直接在真实 NAS、媒体库、下载目录上试新逻辑。
- 配置文件结构以 `ConfigRenameApp`、`ConfigRename`、`RenamePolicy` 为准。
- `fileNames` 里的 `skip` 是占位语义，用来跳过集数，不是普通文件名。
- 当前实现使用临时 UUID 文件名做两阶段重命名，避免目标名互相覆盖；改动时不能破坏这个保护。

## `check_keywords` 规则

`save_work` 目录构建出的命令叫 `check_keywords`。

- 程序从 `stdin` 读取内容。
- `PrepareContents` 只保留以 `+` 开头的行，适配 `git diff` 新增内容检查。
- 默认配置路径是 `/etc/life_tools/check_keywords.json`。
- 示例配置在 `sample/life_tools/check_keywords.json`。
- git hook 示例在 `save_work/git-hooks/pre-commit`。

不要把真实公司域名、内部仓库、密钥或敏感词写进仓库。示例只能保留假数据或公开无害字符串。

## `video_subtitle` 规则

`video_subtitle` 是 Python 工具，不走 Go 构建入口。完整使用方式、缓存结构和排障流程见 `docs/video_subtitle.md`。

- 真实配置默认在 `/etc/life_tools/video_subtitle.json`，示例配置在 `sample/life_tools/video_subtitle.json`。
- 不要把 TOS、ASR、LLM、TMDB 的真实 key、token、预签名 URL 写入仓库或日志。
- 修改 ASR、split、translation、缓存、CLI 参数时，同步更新 `docs/video_subtitle.md` 和 README 中的简要入口。
- 涉及 LLM 输出结构时，必须保留本地 JSON 校验和失败响应落盘，不能只信 prompt。
- 修改 split 逻辑时，优先用已有 `utterances.json` 或离线 fake LLM 测试，不要直接从真实视频重新烧 ASR。
- 交付前至少运行：`PYTHONDONTWRITEBYTECODE=1 python3 -m unittest video_subtitle/video_subtitle_test.py`。
- 如果改了 Python 文件，还要运行 `PYTHONDONTWRITEBYTECODE=1 python3 -m py_compile video_subtitle/video_subtitle.py video_subtitle/video_subtitle_test.py video_subtitle/lib/*.py`。

## 配置与生成物

- `go.mod` / `go.sum` 只有在确实需要依赖变更时才改。
- 不要随手运行 `go get`、`go mod tidy` 或升级依赖；需要时先说明原因和影响。
- `.gitignore` 忽略了 `output`、`bin`、IDE 文件等生成物。
- `build.sh` 虽然出现在 `.gitignore`，但它是被 git 跟踪的构建入口；不要因为被 ignore 就随手改坏。

## 工作区规则

- 允许读取当前仓库和已安装工具来验证事实。
- 不要覆盖用户未提交改动。
- 改动前先看 `git status --short`。
- 只改和任务直接相关的文件。
- 不使用 `git reset --hard`、`git checkout --`、批量删除或其他破坏性命令，除非用户明确要求。

## 文档规则

- README 已经是中文，新增仓库文档优先中文。
- 说明命令时给出可复制命令。
- 设计文档或流程图优先放在 `docs/`；当前仓库没有 `docs/` 时，先确认是否需要创建。
- 修改代码后必须 review 相关文档，确认 README、`docs/`、示例配置和使用说明与实际行为一致；发现不一致时，主动同步更新文档。
- 不要把通用 agent 人格模板塞进项目文档。`AGENTS.md` 只记录这个仓库的真实约束。
