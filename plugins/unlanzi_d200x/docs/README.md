# Ulanzi D200X 插件文档

`plugins/unlanzi_d200x/` 用于维护面向 Ulanzi D200X 的插件。不同插件放在独立子目录中，共用本目录里的 SDK 接入、安装、调试和测试经验。

## 文档索引

| 文档 | 状态 | 用途 |
|---|---|---|
| [命令执行器设计](2026-07-25-command-executor-design.md) | 已有 | 定义 `command_executor` 的范围、配置模型、运行行为和验收标准 |
| [命令执行器实现计划](2026-07-25-command-executor-implementation-plan.md) | 已有 | 按 TDD 顺序拆解源码、构建、文档和 Studio/D200X 实机验证 |
| [Ulanzi 插件开发参考](ulanzi-plugin-development-reference.md) | 已有 | 提炼官方开发指南、本地安装指南和 SDK 的核心接入知识 |
| [命令执行器安装说明](command-executor-installation.md) | 已实机验证 | 说明 macOS 构建、安装、升级、回滚、验收、日志和卸载 |
| [命令执行器开发指南](command-executor-development-guide.md) | 已完成 | 记录目录职责、SDK 快照、运行模型、测试、构建、安装和调试 |
| [命令执行器用户指南](command-executor-user-guide.md) | 已实机验证 | 说明安装、拖拽、配置、长命令、状态、限制和排障 |
| [命令执行器验证报告](command-executor-validation.md) | 自动化与实机证据已记录 | 区分自动化、Studio 安装和实体 D200X 验证证据 |

## 目录约定

```text
plugins/unlanzi_d200x/
├── command_executor/       # 命令执行器源码、测试和构建入口
└── docs/                   # 共用资料和各插件文档
```

新增插件时应保持以下边界：

- 插件源码、依赖和构建脚本放在自己的子目录内。
- 可复用的 Ulanzi SDK 接入经验写入共用开发参考。
- 插件特有的配置、行为、风险和验收方式写入独立文档。
- 实机截图放在 `docs/assets/<plugin-name>/`，不得包含密钥、令牌或其他敏感数据。
- 文档中的版本、路径和命令必须标明来源或实测环境，避免把历史结论当成当前事实。
