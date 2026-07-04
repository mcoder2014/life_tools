# codex_inspector 维护规则

`codex_inspector` 是实验 CLI，用浏览器只读查看本机 Codex session、活跃度、token 统计、memory 和 diagnostics。改动时优先保持实现简单，不引入复杂前端构建链、任务队列或配置系统。

## 回归样例维护

功能回归样例统一维护在：

```text
docs/cli/codex_inspector_regression.md
```

每次修改 `cli/codex_inspector/` 下的代码或静态资源时，必须检查是否需要同步更新该文档。以下改动必须新增或更新回归样例：

- 新增或调整 Overview、Sessions、Memory、Diagnostics 页面内容。
- 新增或调整按钮、菜单、时间范围、搜索、raw JSONL 展开、token usage 展示。
- 修改 `/api/overview`、`/api/sessions`、`/api/memory`、`/api/diagnostics` 或 cache API。
- 修改 JSONL parser、SessionSummary、token 聚合、memory 读取、redaction 规则。
- 修改 SQLite summary cache 的状态、后台补齐、重建、损坏识别、降级或 CLI 参数。
- 修改响应式布局、移动端断点、heatmap 行列规则或数字格式化规则。

如果改动不需要更新回归样例，提交说明或 PR 描述中必须写清原因，例如：

```text
Regression docs unchanged: only internal variable rename; no behavior, API, UI, parser, cache, or privacy boundary changed.
```

## 样例质量要求

新增样例必须包含：

- 前置数据或 fixture。
- 操作步骤。
- 期望结果。
- 可由 AI 工具自动检查的断言点，例如 API 字段、selector、文本、console error、network error、viewport overflow。

样例不得包含真实 `~/.codex` 内容、真实 session 文本、真实路径、auth、token、cookie、secret、API key 或内部信息。需要敏感字段时只能使用假值，并明确期望展示为脱敏值。

## 验证要求

涉及 Go 行为时优先运行：

```bash
go test ./cli/codex_inspector
go build -o output/codex_inspector ./cli/codex_inspector/...
```

涉及前端交互或布局时，启动服务后用 Chrome 检查 Overview、Sessions、Memory、Diagnostics，并至少覆盖：

```text
390x844
768x1024
1440x900
```

纯文档改动不需要运行 Go 测试，但必须检查 diff，确认文档链接、命令、页面名称和 API 名称与当前实现一致。
