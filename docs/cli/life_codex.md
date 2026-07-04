# life_codex 远程 Codex 控制台

`life_codex` 用网页控制远端机器上的 Codex CLI。它只做远程控制台的核心闭环，不复制完整 Codex App。

## 组件

| 组件 | 入口 | 运行位置 | 职责 |
|---|---|---|---|
| `life_codex_server` | `cli/life_codex_server/` | 中心机器 | 提供网页、机器绑定、Session 索引、队列、fork、审计和 agent API |
| `life_codex_agent` | `cli/life_codex_agent/` | 每台计算机器 | 主动连接 server，拉起本机 `codex app-server --stdio`，执行 Session 命令 |
| `web/life_codex` | Vite/React | server 静态资源 | 机器列表、Session 切换、聊天、图片粘贴、BTW/Fork、审计清理 |
| `internal/life_codex` | Go package | 共享 | 配置、协议、状态、审计、附件和 Codex adapter |

## 数据和路径

| 类型 | 默认路径 |
|---|---|
| server 配置 | `/etc/life_tools/life_codex_server.json` |
| agent 配置 | `/etc/life_tools/life_codex_agent.json` |
| server 数据 | `/var/lib/life_tools/life_codex_server/` |
| server 审计 | `/var/log/life_tools/life_codex_server/` |
| agent 图片附件 | `/var/lib/life_tools/life_codex_agent/attachments/` |
| 网页静态资源 | `/usr/local/share/life_tools/life_codex/` |

本机模拟部署时不要写真实系统目录，统一使用 `/private/tmp/life_codex_e2e/` 下的 `config`、`data`、`audit`、`attachments` 和 `web`。

## 行为边界

| 能力 | 当前实现 |
|---|---|
| 机器绑定 | 网页生成 15 分钟 enrolment token，agent enrol 后保存长期 agent token |
| Session | Session 绑定机器；server 保存索引和最近 500 条事件；完整 Codex 状态留在机器本地 |
| 并发 | 不同 Session 可并行；同一 Session 单 active turn；运行中输入进入 Session FIFO 队列 |
| 机器限流 | 按 agent 上报的 `max_active_sessions` 控制同时运行的 Session 数 |
| 失败原因 | Codex turn 失败时保留 `last_error`，网页在当前 Session 顶部展示原始失败原因 |
| 重试 | failed Session 可点击 `Retry last turn`，server 复用最后一条文本用户消息重新入队或立即执行；如果本地 `codex app-server` 重启后找不到旧 thread，agent 会创建替代 thread 后重发本次 turn |
| `/btw` / Fork | 输入 `/btw <问题>` 或右侧 Fork；优先调用 `thread/fork`，本机 Codex 不支持时降级新线程 |
| 图片粘贴 | 网页转 base64，server 校验 MIME/大小/数量，agent 落盘后以 `localImage` 输入交给 Codex |
| allowed roots | 创建 Session 的 cwd 必须在 agent `allowed_roots` 内 |
| 审计 | 文本原文和工具事件写 JSONL；图片只记录 metadata，`data_base64` 在 logger 层强制脱敏 |
| 审计清理 | CLI 和网页都要求二次确认 |

永久保存文本原文审计是高风险选择。不要把 server 部署到不可信网络，也不要把 audit 目录放到宽权限共享盘。

## 安装

先构建网页：

```bash
npm --prefix web/life_codex install
npm --prefix web/life_codex run build
```

安装 server 和 agent：

```bash
./install.sh --tool life_codex_server --tool life_codex_agent
```

安装脚本会安装二进制、网页资源和示例配置。已有配置文件不会被覆盖。

## server 配置

示例文件：`sample/life_tools/life_codex_server.json`

```json
{
  "addr": "127.0.0.1:8899",
  "admin_token": "change-me",
  "data_dir": "/var/lib/life_tools/life_codex_server",
  "audit_dir": "/var/log/life_tools/life_codex_server",
  "web_root": "/usr/local/share/life_tools/life_codex",
  "max_image_bytes": 10485760,
  "max_images_per_turn": 5,
  "default_approval_policy": "on-request",
  "default_approvals_reviewer": "auto_review"
}
```

启动：

```bash
life_codex_server -config /etc/life_tools/life_codex_server.json
```

网页访问：

```text
http://127.0.0.1:8899
```

## agent 绑定和运行

在网页点击 `Token` 生成 enrolment token，然后在目标机器执行：

```bash
life_codex_agent enroll \
  -config /etc/life_tools/life_codex_agent.json \
  -server http://127.0.0.1:8899 \
  -token <enrollment-token> \
  -name local \
  -roots /Users/bytedance/go/src/github.com/mcoder2014 \
  -attachment-dir /var/lib/life_tools/life_codex_agent/attachments \
  -max-active-sessions 2
```

前台运行：

```bash
life_codex_agent serve -config /etc/life_tools/life_codex_agent.json
```

agent 使用本机已安装并登录的 `codex`。如果 Codex 未登录，先在计算机器上完成 Codex 登录；不要用 mock 伪造成功。

## 本机模拟部署

```bash
mkdir -p /private/tmp/life_codex_e2e/{config,data,audit,attachments,web}
cp -R web/life_codex/dist/. /private/tmp/life_codex_e2e/web/
```

server 配置写到 `/private/tmp/life_codex_e2e/config/server.json`，其中目录都指向 `/private/tmp/life_codex_e2e/`。agent 配置写到 `/private/tmp/life_codex_e2e/config/agent.json`。

启动顺序：

```bash
./output/life_codex_server -config /private/tmp/life_codex_e2e/config/server.json
./output/life_codex_agent enroll -config /private/tmp/life_codex_e2e/config/agent.json -server http://127.0.0.1:8899 -token <token> -name local -roots /Users/bytedance/go/src/github.com/mcoder2014/worktrees/life_codex_remote -attachment-dir /private/tmp/life_codex_e2e/attachments -max-active-sessions 2
./output/life_codex_agent serve -config /private/tmp/life_codex_e2e/config/agent.json
```

验收重点：

| 场景 | 期望 |
|---|---|
| 创建 Session 后连续发送两轮 | 同一个 Codex thread 上返回两次结果 |
| 两个 Session 并行运行 | 切换页面不打断后台运行 |
| 主 Session 运行时 `/btw` | fork Session 独立返回，主 Session 继续运行 |
| 粘贴图片提问 | agent 附件目录落盘，Codex 能读取图片内容 |
| 审计清理 | 未勾选确认时拒绝，确认后按日期删除 JSONL |

## 已知限制

- V1 只支持私有网络、VPN 或 Tailscale；TLS 由反向代理提供。
- 不内置 systemd/launchd，不自动升级 Codex CLI。
- 不做完整文件树、diff 编辑器、插件管理或实时语音。
- 当前本机 `codex app-server` 如果不支持 `thread/fork`，BTW 会退化为新线程；这不是完整上下文 fork。
- 网页审批只保留配置字段，V1 仍主要依赖 `approvalPolicy=on-request` 和 `approvalsReviewer=auto_review`。
- failed Session 的 retry 只自动复用最后一条文本 turn；图片 turn 失败后需要重新粘贴图片，避免用脱敏后的图片 metadata 伪造重试。
- 如果 retry 时旧 thread 已不在当前 `codex app-server` 内存中，agent 会启动替代 thread；这能恢复执行能力，但不能保证保留旧 thread 的完整上下文。
