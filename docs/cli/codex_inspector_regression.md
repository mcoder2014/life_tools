# codex_inspector 功能回归测试样例

本文档记录 `codex_inspector` 的功能回归样例，供人工和 AI 工具在改动后执行。样例重点覆盖页面行为、API 合约、隐私边界、响应式布局和 SQLite summary cache 生命周期。

`codex_inspector` 只读展示本机 Codex 数据。回归测试默认使用脱敏 fixture，不直接依赖真实 `~/.codex`；需要真实数据验证时，只允许只读访问，不能把真实会话、token、auth、cookie、secret 或内部路径写入截图、日志和文档。

## 基本原则

| 原则 | 要求 |
|---|---|
| 数据安全 | 测试 fixture 必须脱敏；`auth.json`、token、cookie、secret、真实 API key 不得进入仓库。 |
| 只读边界 | 测试不得写入 `~/.codex`；cache 只能写到 `-cache-path` 指定的临时文件或系统 tmp 下的用户隔离目录。 |
| 可重复 | 每个样例必须说明前置数据、启动参数、操作步骤和断言点。 |
| 可自动化 | 页面样例需要给出可由 Chrome/Playwright 检查的 selector、网络请求或文本断言。 |
| 跨端 | 涉及 UI 的改动至少检查 `390x844`、`768x1024`、`1440x900` 三类视口。 |
| 降级可用 | cache 缺失、损坏、禁用或不可用时，页面仍应能通过实时 JSONL 解析展示核心数据。 |

## 推荐验证命令

从仓库根目录执行：

```bash
GOMODCACHE=/tmp/codex_inspector_mod_cache GOCACHE=/tmp/codex_inspector_go_cache go test ./cli/codex_inspector
GOMODCACHE=/tmp/codex_inspector_mod_cache GOCACHE=/tmp/codex_inspector_go_cache go build -o output/codex_inspector ./cli/codex_inspector/...
node --check cli/codex_inspector/static/app.js
```

启动服务时使用脱敏 fixture：

```bash
./output/codex_inspector \
  -codex-home /tmp/codex-inspector-fixture \
  -cache-path /tmp/codex-inspector-cache/session_summary_cache.sqlite \
  -addr 127.0.0.1:8787
```

禁用 cache 的启动方式：

```bash
./output/codex_inspector \
  -codex-home /tmp/codex-inspector-fixture \
  -no-cache \
  -addr 127.0.0.1:8787
```

## Fixture 要求

| Fixture | 目的 | 最小内容 |
|---|---|---|
| `base` | 正常功能回归 | `session_index.jsonl`、跨 3 天的 `sessions/YYYY/MM/DD/rollout-*.jsonl`、`memories/MEMORY.md`、`memory_summary.md`、`rollout_summaries/*.md`。 |
| `bad_jsonl` | 容错回归 | 至少一个 rollout 文件包含坏 JSONL 行、未知字段、缺少 `timestamp` 的事件。 |
| `missing_sources` | 缺文件回归 | 缺少 `session_index.jsonl` 或 `memories/`，但服务可启动。 |
| `token_usage` | token 聚合回归 | 至少两个 session 含 `event_msg.payload.type=token_count`，覆盖 input、cached input、output、reasoning、context window、rate limit。 |
| `privacy` | 脱敏回归 | JSONL 和 memory 中包含假 `token`、`cookie`、`secret`、`api_key` 字段，期望页面只显示脱敏值。 |
| `cache_missing` | cache 创建回归 | 指定一个不存在的 `-cache-path`。 |
| `cache_corrupt` | cache 损坏回归 | 指定一个内容为普通文本的 `.sqlite` 文件。 |
| `cache_disabled` | cache 禁用回归 | 使用 `-no-cache`。 |

fixture 中的路径可使用 `/tmp/codex-demo/project`、`/tmp/demo-home/.codex` 等假路径，不得使用真实工作路径。

## API 回归样例

| 编号 | API | 前置条件 | 操作 | 断言 |
|---|---|---|---|---|
| API-001 | `GET /api/overview` | `base` fixture | 请求默认范围 | HTTP 200；包含 `stats.totalSessions`、`stats.byDay`、`stats.byWeek`、`stats.byMonth`、`stats.heatmap`、`recentSessions`。 |
| API-002 | `GET /api/overview?from=YYYY-MM-DD&to=YYYY-MM-DD` | `base` fixture | 请求一个只覆盖部分 session 的范围 | `stats.totalSessions` 只统计范围内 session；`stats.heatmap` 仍固定最近 183 天，不随范围缩短。 |
| API-003 | `GET /api/sessions?limit=300` | `base` fixture | 请求 session 列表 | HTTP 200；session 按更新时间倒序；每项包含 `id`、`title`、`updatedAt`、`eventCount`、`tokenStats`。 |
| API-004 | `GET /api/sessions?q=demo` | `base` fixture | 搜索标题、cwd 或 model | 只返回匹配项；大小写不应导致明显误判。 |
| API-005 | `GET /api/sessions/{id}?event_limit=300&raw=0` | `base` fixture | 请求一条存在的 session | HTTP 200；包含 `summary`、分页后的 `events`、`eventTotal`、`hasMore`；不默认返回 `rawLines`；events 顺序与 JSONL 行序一致。 |
| API-005A | `GET /api/sessions/{id}/raw?line=N` | `privacy` fixture | 请求单行 raw JSONL | HTTP 200；只返回目标行的脱敏 raw 内容；不存在的行返回结构化 404。 |
| API-006 | `GET /api/sessions/not-found` | `base` fixture | 请求不存在的 id | HTTP 404；返回结构化 `error`，服务不 panic。 |
| API-007 | `GET /api/memory` | `base` fixture | 请求 memory 列表 | HTTP 200；包含 `MEMORY.md`、`memory_summary.md`、rollout summary 条目。 |
| API-008 | `GET /api/memory?q=keyword` | `base` fixture | 搜索关键词 | 只展示匹配文件或匹配计数；不返回 auth/token 原文。 |
| API-009 | `GET /api/memory/file?path=...` | `base` fixture | 请求存在文件详情 | HTTP 200；返回 `file` 和 `content`；内容已脱敏。 |
| API-010 | `GET /api/diagnostics` | `base` fixture | 请求诊断 | HTTP 200；包含 `sources`、`schemas`；`auth.json` 只标记排除，不读取内容。 |
| API-011 | 所有 API | `bad_jsonl` fixture | 请求 Overview、Sessions、Session detail | HTTP 200 或目标 session 的可解释错误；坏行进入 warnings；服务不 panic。 |

## Cache API 回归样例

当实现 cache lifecycle API 后，必须覆盖以下样例。cache 是派生数据，不是用户原始数据；失败时必须降级实时解析 JSONL。

| 编号 | API | 前置条件 | 操作 | 断言 |
|---|---|---|---|---|
| C-001 | `GET /api/cache/status` | `-no-cache` | 请求 cache 状态 | `status=disabled`；展示禁用原因；`canBuild=false`；页面不展示误导性重建按钮。 |
| C-002 | `GET /api/cache/status` | `cache_missing` | 请求 cache 状态 | `status=missing`；展示 cache 路径；允许 `Create cache`。 |
| C-003 | `POST /api/cache/build` | `cache_missing` | 点击或请求创建 cache | 返回当前 job；服务后台补齐今天以前的 rollout；今天的 rollout 不写入 cache。 |
| C-004 | `POST /api/cache/build` | job 正在运行 | 连续请求两次 build | 第二次不启动新 job；返回同一个 job 的进度状态。 |
| C-005 | `GET /api/cache/status` | build 运行中 | 轮询状态 | `status=rebuilding` 或等价运行态；`job.total`、`job.done`、`job.cached`、`job.skipped` 单调推进。 |
| C-006 | `GET /api/cache/status` | 健康 cache | 请求状态 | `status=healthy`；允许 `Fill missing cache`；不显示 rebuild 警告。 |
| C-007 | `POST /api/cache/build` | 健康 cache 且有历史缺口 | 请求补齐 | 只补齐缺失或过期历史 summary；SQLite 写入保持串行或低并发。 |
| C-008 | `GET /api/cache/status` | `cache_corrupt` | 请求状态 | `status=corrupt`；错误只在明确 `malformed`、`file is not a database`、`schema is corrupt` 时判定为损坏。 |
| C-009 | `POST /api/cache/rebuild` | `cache_corrupt`，用户已在应用内确认弹窗二次确认 | 请求重建 | 原 cache rename 为 `.corrupt.<timestamp>.bak`；新建 cache；不直接删除损坏文件。 |
| C-010 | `POST /api/cache/rebuild` | 普通权限失败或 busy timeout | 请求重建 | 不 rename；状态为 `unavailable`；展示原因；页面仍通过 JSONL 实时解析可用。 |
| C-011 | Overview/Sessions | cache 损坏或不可用 | 打开页面 | 页面仍可展示统计和列表；warnings 可见；无未捕获 JS error。 |

## Overview 页面回归样例

| 编号 | 前置条件 | 操作 | 断言 |
|---|---|---|---|
| O-001 | `base` fixture | 打开首页 | 顶部显示数据源状态；统计卡片展示 sessions、events、messages、tools、tokens、output。 |
| O-002 | `token_usage` fixture | 打开首页 | token 数字使用 `k`、`m`、`b` 简写；hover/title 或详情中保留可读精确值。 |
| O-003 | `base` fixture | 默认加载 | 时间范围默认为最近 7 天；From/To 有默认值，不为空。 |
| O-004 | `base` fixture | 切换 Last 24 hours、Last 7 days、Last 30 days、自定义日期 | 统计卡片、月度趋势、最近 session 跟随范围刷新。 |
| O-005 | `base` fixture | 连续设置 11 个自定义范围 | 下拉历史最多保留 10 条；重复范围去重。 |
| O-006 | `base` fixture | 查看 Activity heatmap | 固定展示最近 183 天；行数固定 7 行；宽窄视口下不改变行数。 |
| O-007 | `base` fixture | hover heatmap 单元格 | tooltip/title 或 aria label 包含日期和当日对话次数。 |
| O-008 | `missing_sources` fixture | 打开首页 | 页面展示空态或 warnings；不出现未捕获异常；菜单仍可切换。 |

## Sessions 页面回归样例

| 编号 | 前置条件 | 操作 | 断言 |
|---|---|---|---|
| S-001 | `base` fixture | 打开 Sessions | 左侧列表展示 session title、时间、事件数、model、token 简写和 preview。 |
| S-002 | `base` fixture | 点击第一条 session | 右侧展示详情，不停留在 `Select a session to inspect.`。 |
| S-003 | `base` fixture | 查看详情事件流 | user、assistant、tool call、tool result、system event 按 JSONL 原始顺序展示。 |
| S-004 | `base` fixture | 展开 Raw JSONL | raw 内容按需加载且已脱敏；能看到行号；不能出现假 token/cookie/secret 原文。 |
| S-005 | `token_usage` fixture | 点击含 token_count 的 session | token usage 摘要展示 total、input、cached、output、reasoning、last turn、context、rate limit。 |
| S-006 | `base` fixture | 搜索 title、cwd、model | 列表过滤；清空搜索后恢复。 |
| S-007 | `bad_jsonl` fixture | 点击含坏行 session | 可展示有效事件；坏行进入 warnings 或 raw 脱敏内容；服务不 panic。 |
| S-008 | 大 session fixture | 点击事件数较多的历史 session | 首屏只渲染有限事件，展示总数和 `Load more events`；不默认下载全部 raw JSONL；有 summary hint 时后端只读取当前事件窗口，不触发浏览器慢标签提示。 |

## Memory 页面回归样例

| 编号 | 前置条件 | 操作 | 断言 |
|---|---|---|---|
| M-001 | `base` fixture | 打开 Memory | 列表展示 summary、index、rollout summary 等分类。 |
| M-002 | `base` fixture | 点击 `MEMORY.md` | 右侧展示标题、路径、大小、更新时间和内容预览。 |
| M-003 | `base` fixture | 搜索关键词 | 匹配项保留；无匹配时展示清晰空态。 |
| M-004 | `privacy` fixture | 打开包含敏感假字段的 memory | token、cookie、secret、api_key 被脱敏；不展示原文。 |
| M-005 | `missing_sources` fixture | 缺少 `memories/` | 页面展示 warnings 或空态；不影响 Overview/Sessions。 |

## Diagnostics 页面回归样例

| 编号 | 前置条件 | 操作 | 断言 |
|---|---|---|---|
| D-001 | `base` fixture | 打开 Diagnostics | Data sources 表展示 session index、sessions、memory、sqlite schema 等来源状态。 |
| D-002 | `base` fixture | 查看 SQLite schema | 只读展示 schema；不打开或展示 `auth.json` 内容。 |
| D-003 | 系统无 `sqlite3` 命令 | 打开 Diagnostics | schema 区域显示不可用原因；页面仍可使用。 |
| D-004 | cache missing | 打开 Diagnostics | cache 区域展示 `missing` 和 `Create cache` 按钮。 |
| D-005 | cache healthy | 打开 Diagnostics | cache 区域展示 `healthy` 和 `Fill missing cache` 按钮。 |
| D-006 | cache corrupt | 打开 Diagnostics 并点击 `Backup and rebuild cache` | cache 区域展示 `corrupt`；页面展示应用内二次确认 modal，不使用浏览器原生 confirm；取消关闭 modal，确认后才开始 rebuild。 |
| D-007 | cache rebuilding | 触发 build/rebuild 后轮询 | 展示 total、done、cached、skipped、failed、startedAt、lastError 或 finishedAt。 |
| D-008 | cache disabled/unavailable | 使用 `-no-cache` 或不可写 cache path | 展示明确原因；不展示可执行但必然失败的按钮。 |

## 响应式 UI 回归样例

每个页面至少检查以下视口：

| 编号 | 视口 | 页面 | 断言 |
|---|---|---|---|
| R-001 | `390x844` | Overview | 顶部范围控件不溢出；统计卡片可读；heatmap 不横向失控。 |
| R-002 | `390x844` | Sessions | 左侧列表和右侧详情可上下浏览；按钮和 raw 展开不遮挡内容。 |
| R-003 | `390x844` | Memory | 文件列表与详情不重叠；长路径换行或截断合理。 |
| R-004 | `390x844` | Diagnostics | 表格转换为移动端可读布局；cache 操作按钮不溢出。 |
| R-005 | `768x1024` | Overview/Sessions | iPad 尺寸下主内容列宽稳定；heatmap 仍为 7 行。 |
| R-006 | `1440x900` | 全页面 | 页面无大面积错位；顶部菜单与左侧菜单职责清楚，不重复堆砌。 |

通用断言：

- `document.documentElement.scrollWidth <= window.innerWidth + 1`。
- Chrome console 无应用自身 error。
- Network 面板没有 API 404/500，除非样例明确测试错误路径。
- 文本不重叠，按钮文字不溢出，菜单不遮挡内容。

## 隐私与安全回归样例

| 编号 | 前置条件 | 操作 | 断言 |
|---|---|---|---|
| P-001 | `privacy` fixture | 打开所有页面 | 不出现原始 `token`、`cookie`、`secret`、`api_key` 值。 |
| P-002 | `privacy` fixture | 展开 raw JSONL | raw 行仍脱敏；只保留排查格式所需结构。 |
| P-003 | `base` fixture | 打开 Diagnostics | `auth.json` 只显示 excluded 或不读取说明，不显示内容。 |
| P-004 | 任意 fixture | 检查 cache 文件 | cache 不包含 raw JSONL 和完整对话内容，只包含脱敏 summary 和 token 聚合。 |
| P-005 | 任意 fixture | 服务启动参数 | 默认监听 `127.0.0.1`；文档和 UI 不鼓励绑定公网地址。 |

## AI 回归执行清单

AI 工具执行回归时按以下顺序记录结果：

1. 记录分支、commit、启动命令、fixture 路径和 cache path。
2. 运行 Go 单测、构建和 `node --check`。
3. 启动服务并用 API 检查 Overview、Sessions、Memory、Diagnostics。
4. 用 Chrome 打开页面，检查四个页面和三类视口。
5. 如果本次改动涉及 cache，执行 Cache API 和 Diagnostics cache 操作样例。
6. 如果本次改动涉及 parser、redaction、memory 或 raw JSONL，执行隐私与坏 JSONL 样例。
7. 输出 `通过 / 失败 / 未执行` 表格；未执行必须说明原因，不能写成已通过。

推荐结果表：

| 样例编号 | 结果 | 证据 | 备注 |
|---|---|---|---|
| O-001 | 通过 | 截图或 API 摘要 |  |
| S-002 | 失败 | 错误文本或 console error |  |
| C-009 | 未执行 | cache rebuild 尚未实现 |  |

## 维护要求

新增或修改以下内容时，必须同步更新本文档：

- 新增页面、页面区块、按钮、菜单或响应式布局。
- 修改 `/api/overview`、`/api/sessions`、`/api/memory`、`/api/diagnostics` 或 cache API。
- 修改 JSONL parser、session summary、token 统计、memory 读取、redaction 规则。
- 修改 SQLite summary cache 的状态、补齐、重建、损坏识别或降级逻辑。
- 修改默认时间范围、range history、heatmap、数字格式化、raw JSONL 展示。

如果一个改动不需要新增样例，提交说明或 PR 描述中必须说明原因，例如“只改文案，不改变行为；现有 O-001 覆盖”。
