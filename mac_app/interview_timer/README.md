# Interview Timer

一个独立的 macOS 面试悬浮计时工具，适合在另一个系统旁边主持面试时使用。

该应用在 `life_tools` 仓库中归档于 `mac_app/interview_timer/`。仓库级背景、安装和维护说明见 `../../docs/interview_timer.md`。

## 功能

- 小型悬浮窗，始终置顶
- 预设 JSON 模板驱动的多环节面试流程
- 主按钮开始/进入下一环节
- 右键菜单回到上一环节、下一环节、重置、退出
- 菜单栏入口：显示/隐藏面板、打开当前模板、打开模板目录、重新加载、切换模板
- 同时显示当前环节剩余时间、整体剩余时间、提前/落后进度
- 环节超时和整体超时的视觉提醒
- macOS 系统通知
- 支持跨显示器拖动，并记住上次位置

## 目录

- `Sources/InterviewTimerCore/`: 模板、会话状态机、时间计算、配置存储
- `Sources/InterviewTimerApp/`: AppKit 悬浮面板、SwiftUI 视图、通知
- `Tests/InterviewTimerCoreTests/`: 核心逻辑测试
- `scripts/build_app.sh`: 打包 `.app`

## 模板文件

兼容旧版单模板文件：

`~/Library/Application Support/InterviewTimer/template.json`

多模板目录：

`~/Library/Application Support/InterviewTimer/templates/`

活动模板选择文件：

`~/Library/Application Support/InterviewTimer/template-selection.json`

如果你已经在使用旧的 `template.json`，新版本会继续兼容它，不强制迁移。

如果你想支持多种面试时间分布模板，直接把多个 JSON 放到 `templates/` 目录里，然后从菜单栏 `切换模板` 选择当前生效模板即可。菜单栏里的 `打开当前模板文件` 和 `打开模板目录` 就是给这个场景准备的。

示例：

```json
{
  "templateName": "60min Interview",
  "warnings": {
    "stageLastSeconds": 60,
    "overallLastSeconds": 300
  },
  "stages": [
    { "id": "intro", "name": "开场与破冰", "durationMinutes": 5 },
    { "id": "resume", "name": "经历深挖", "durationMinutes": 15 },
    { "id": "coding", "name": "技术问题", "durationMinutes": 20 },
    { "id": "qa", "name": "候选人提问", "durationMinutes": 10 },
    { "id": "close", "name": "结束收口", "durationMinutes": 10 }
  ]
}
```

## 开发命令

```bash
swift test
swift build --product InterviewTimerApp
./scripts/build_app.sh
open dist/InterviewTimer.app
```

## 注意

- 需要 macOS 13+
- 推荐完整 Xcode 或正确配置的 Apple 开发工具链
- 如果只有残缺的 Command Line Tools，`swift build` 可能会因为 SDK 平台路径缺失而失败
- `dist/InterviewTimer.app` 是本地构建产物，不纳入版本控制
