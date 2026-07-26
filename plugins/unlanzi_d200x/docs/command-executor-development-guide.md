# Ulanzi D200X 命令执行器开发指南

本文面向第一次接触 Ulanzi 插件的开发者，说明 `command_executor` 的目录、运行模型、构建、安装和排障方法。内容以仓库当前实现为准；自动化与 Ulanzi Studio 3.1.9 / D200X 实机结果见[验证报告](command-executor-validation.md)，完整安装步骤见[安装说明](command-executor-installation.md)。

## 1. 支持范围

| 项目 | 当前值 |
|---|---|
| 插件版本 | `0.1.0` |
| 插件包目录 | `com.ulanzi.commandexecutor.ulanziPlugin` |
| 主服务 UUID | `com.ulanzi.ulanzistudio.commandexecutor` |
| Action UUID | `com.ulanzi.ulanzistudio.commandexecutor.runcommand` |
| 控制器 / 设备 | `Keypad` / `D200X` |
| 操作系统 | macOS 10.15 及以上 |
| Ulanzi Studio | 3.0.11 及以上 |
| 构建目标 | Node.js 20 |
| 当前不支持 | Windows、Encoder、交互式终端、超时、停止进程、执行历史 |

这些限制由 `manifest.json` 和实现共同决定。不要只修改文档来扩大兼容范围；设备、系统或 Studio 范围变化必须同步修改 manifest、测试并完成实机回归。

## 2. 最终目录与文件职责

`dist/`、`output/` 和 `node_modules/` 是生成物。真正安装给 Ulanzi Studio 的内容只有构建后的 `.ulanziPlugin` 目录。

### 2.1 工程、构建和测试

| 路径 | 职责 |
|---|---|
| `command_executor/package.json` | 固定 Node 版本边界、`ws` / Webpack 依赖和测试、打包命令 |
| `command_executor/package-lock.json` | 锁定可复现的 npm 依赖版本 |
| `command_executor/webpack.config.js` | 以 Node 20 为目标，把主服务和 `ws` 打成单个 ESM `dist/app.js` |
| `command_executor/build.sh` | 清理本插件的精确生成目标，构建、复制、校验并生成 zip |
| `command_executor/scripts/validate-package.mjs` | 检查目录名、manifest 资源、必需文件、SDK 来源、污染文件和 bundle 路径泄漏 |
| `command_executor/tests/command-runner.test.mjs` | 验证环境变量、工作目录、Shell、完整命令语义、200 参数和输出截断 |
| `command_executor/tests/command-plugin.test.mjs` | 验证事件注册、`context` 隔离、并发状态、失败反馈和日志 |
| `command_executor/tests/inspector-settings.test.mjs` | 验证配置规范化、校验、预览、200 ms 后向 Studio 发送参数和页面脚本接入 |
| `command_executor/tests/manifest.test.mjs` | 验证 manifest、资源、UUID、平台边界、语言文件和图标 |
| `command_executor/tests/package-validator.test.mjs` | 用完整包和故障包验证打包校验器 |
| `command_executor/tests/sdk-vendor.test.mjs` | 按 SHA-256 清单验证 vendored SDK 未被修改，并核对三个来源 commit |
| `command_executor/tests/fixtures/ulanzi-sdk-sha256.json` | 保存 vendored SDK 与许可证文件的固定 SHA-256 |
| `command_executor/output/` | `build.sh` 生成的可安装目录和 zip；不提交 |

### 2.2 插件包自有文件

以下路径都位于 `com.ulanzi.commandexecutor.ulanziPlugin/`。

| 路径 | 职责 |
|---|---|
| `manifest.json` | 声明入口、UUID、Action、资源以及 macOS / D200X / Keypad 范围 |
| `package.json` | 让插件包和 bundle 按 ES module 加载 |
| `en.json` / `zh_CN.json` | Property Inspector 的英文和简体中文文案 |
| `THIRD_PARTY_NOTICES.md` | 记录三个官方 SDK 仓库、固定 commit、原始路径和许可证 |
| `LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt` | 随包保留 Apache License 2.0 |
| `assets/icons/plugin.svg` | Studio 插件分类图标 |
| `assets/icons/action.svg` | Action 默认状态图标 |
| `assets/icons/running.svg` | 命令仍在运行时的图标 |
| `assets/icons/success.svg` | 一批并发执行全部成功后的短暂图标 |
| `plugin/app.js` | 创建官方 Node SDK 实例、注册业务回调并连接主服务 UUID |
| `plugin/command-plugin.js` | 按 `context` 管理配置和运行状态，处理 Ulanzi 事件、反馈与业务日志 |
| `plugin/command-runner.js` | 校验配置，通过 `$SHELL -lc` 启动命令并限制 stdout / stderr |
| `property-inspector/inspector.html` | 标题、命令、工作目录、环境变量表单及只读预览 |
| `property-inspector/inspector.css` | 当前配置面板的局部样式 |
| `property-inspector/inspector.js` | 连接 Action、读取当前按键参数、200 ms 防抖发送给 Studio 并安全渲染预览 |
| `property-inspector/settings.js` | 配置默认值、规范化、环境变量校验和执行预览 |
| `dist/app.js` | Webpack 生成的 Studio 主服务入口；不手工编辑 |

### 2.3 原样 vendored 的官方 SDK 文件

这些文件不是业务源码，不在仓库内格式化或局部修补。

| 路径 | 上游职责 |
|---|---|
| `plugin/vendor/ulanzi-api/constants.js` | Node SDK 事件常量 |
| `plugin/vendor/ulanzi-api/ulanziApi.js` | Node 主服务 WebSocket、事件和 Studio API |
| `plugin/vendor/ulanzi-api/utils.js` | Node SDK 通用工具 |
| `libs/js/constants.js` | Property Inspector 事件常量 |
| `libs/js/eventEmitter.js` | Property Inspector 事件分发 |
| `libs/js/timers.js` | 官方页面定时器工具 |
| `libs/js/utils.js` | 官方表单、国际化和页面工具 |
| `libs/js/ulanziApi.js` | Property Inspector WebSocket 和 Studio API |
| `libs/css/uspi.css` | 官方 Property Inspector 基础样式 |
| `libs/assets/u_active.svg` / `u_active_none.svg` | 官方激活状态资源 |
| `libs/assets/u_check_checkbox.svg` / `u_check_none.svg` / `u_check_radio.svg` | 官方选择控件资源 |
| `libs/assets/u_down.svg` / `u_file.svg` / `u_folder.svg` / `u_refresh.svg` | 官方下拉、文件、目录和刷新资源 |
| `libs/assets/u_tip_error.svg` / `u_tip_info.svg` / `u_tip_success.svg` / `u_tip_warn.svg` | 官方提示状态资源 |

## 3. SDK 快照与日志边界

| 来源 | 固定 commit | 本项目中的内容 |
|---|---|---|
| `UlanziTechnology/UlanziDeckPlugin-SDK` | `550ab80c69285ecf259bd494a7fff767c14f0c0f` | Apache-2.0 许可证基线 |
| `UlanziTechnology/plugin-common-node` | `112bd13a7ff9d45bd68656f7e069fd61851d1812` | `plugin/vendor/ulanzi-api/` |
| `UlanziTechnology/plugin-common-html` | `79de0b0b087546e684afd23f97223f7a7bc392da` | `libs/` |

vendored 文件保持上游原样。`tests/sdk-vendor.test.mjs` 会把文件集合和内容 SHA-256 与 `tests/fixtures/ulanzi-sdk-sha256.json` 比较，防止无意修改。

本插件为本机自用和协议排障保留官方 SDK 的完整诊断行为：

- Node SDK 会记录接收和发送的 WebSocket payload。
- HTML SDK 会记录接收的 WebSocket payload，部分设置调用也会输出参数。
- payload 可能包含完整命令、工作目录和环境变量值。
- 业务层还会通过 `logMessage` 记录退出码以及受限长度的 stdout / stderr。

不要修改、过滤或最小化这份官方 SDK 日志。敏感信息风险由当前用户的 Mac、本机账号权限和日志保管边界管理：不要把令牌、密码等无法接受落盘的值写入面板，也不要把原始日志复制进 issue、PR 或仓库文档。

## 4. 配置、`context` 与并发

每个拖入按键的 Action 都有一个 Studio 分配的唯一 `context`。主服务维护 `Map<context, instance>`，每个实例包含：

| 字段 | 含义 |
|---|---|
| `settings` | 当前按键的 `title`、`command`、`workingDirectory`、`environment` |
| `runningCount` | 当前按键仍未结束的子进程数 |
| `failedInBurst` | 当前并发批次是否已有一次失败 |
| `restoreTimer` | 成功图标恢复默认状态的定时器 |

配置事件只更新对应 `context`。执行前复制一份设置快照，所以命令运行期间继续编辑面板只影响下一次运行。

同一按键可以连续触发多次，每次都会启动独立子进程。`runningCount > 0` 时保持运行图标；全部结束且没有失败才显示 1200 ms 成功图标。只要其中一次失败，该批次就不会显示最终成功状态。`onClear` 会删除对应实例和恢复定时器，但不会强制终止已经启动的系统进程。

## 5. Shell、目录和环境变量

主服务优先使用当前进程的绝对且可执行的 `$SHELL`，否则回退到 `/bin/zsh`。真实执行等价于：

```js
spawn(shell, ["-lc", wrappedScript], {
  cwd: workingDirectory,
  env: process.env,
  stdio: ["ignore", "pipe", "pipe"]
});
```

`-l` 表示登录 Shell，`-c` 表示执行后面的完整脚本。以 zsh 为例，登录 Shell 会读取 `.zprofile` 等登录配置；它不是交互式 Shell，因此不会自动读取只为交互会话加载的 `.zshrc`。如果某个 alias、函数或 PATH 只写在 `.zshrc`，插件里可能不可用。稳定方案是使用绝对命令路径、把公共环境放到登录配置，或在命令中显式 `source` 所需文件。

面板环境变量会先解析，再放到完整命令之前：

```sh
export CUSTOM_ENV='hello'
export PATH='/explicit/path'
<用户命令原文>
```

因此顺序是“继承 Studio 环境 → 登录 Shell 配置 → 面板显式 `export` → 用户命令”，面板同名值最终覆盖前两者。环境变量每行一个 `KEY=VALUE`，按第一个 `=` 分隔；同名项最后一行生效。变量名必须匹配 `[A-Za-z_][A-Za-z0-9_]*`。

工作目录只接受空值、`~`、`~/...` 或绝对路径。空值和 `~` 都解析为当前用户 HOME；目录必须存在、是目录并可访问。

## 6. 输出与错误

stdout 和 stderr 分别最多保留 `16 * 1024` 字节，两个流互不占用额度。超过上限时，日志附加“输出已截断”。限制按字节计算，并在收集后按 UTF-8 解码，避免越界读取持续增长的输出。被保留的前 16 KiB 仍可能包含命令输出的令牌、路径或其他敏感值，不能把截断误解为脱敏。

| 场景 | 反馈 |
|---|---|
| 配置校验失败 | 不启动子进程，设备 `showAlert`，Toast 给出字段或行号 |
| 非零退出码、信号退出、启动失败 | 设备 `showAlert`、简短 Toast、详细本地日志 |
| 成功 | 短暂显示成功图标，写入完成日志 |
| 多次并发 | 最后一个进程结束前保持运行状态 |

## 7. 安装依赖、测试和构建

在仓库根目录执行：

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
./build.sh
```

`npm ci` 严格按 lockfile 安装依赖。`npm test` 运行全部 `node:test` 用例。`./build.sh` 依次完成 Webpack bundle、目录复制、包结构校验和 zip 完整性校验。

构建成功后生成：

```text
output/com.ulanzi.commandexecutor.ulanziPlugin/
output/life_tools_ulanzi_d200x_command_executor.zip
```

两条额外校验命令：

```bash
node scripts/validate-package.mjs \
  output/com.ulanzi.commandexecutor.ulanziPlugin
unzip -t output/life_tools_ulanzi_d200x_command_executor.zip
```

## 8. 本地安装和调试

官方本地安装目录是：

```text
~/Library/Application Support/Ulanzi/UlanziDeck/Plugins
```

复制 `output/com.ulanzi.commandexecutor.ulanziPlugin`，不要复制它的父目录，也不要把 `node_modules` 放进插件目录。同名版本已存在时先移动为带时间戳的备份，再复制新包；不要删除其他插件。复制完成后完全退出并重新打开 Ulanzi Studio。

本机已经把构建目录安装到：

```text
~/Library/Application Support/Ulanzi/UlanziDeck/Plugins/
└── com.ulanzi.commandexecutor.ulanziPlugin
```

安装目录共 40 个文件，`dist/app.js` 与本轮构建产物 SHA-256 一致。Studio 3.1.9 重启后能加载插件、显示 Property Inspector，并恢复两个按键各自的配置。

当前插件可用的启动方式只启用 Studio 日志和 HTML / WebView 调试：

```bash
open /Applications/Ulanzi\ Studio.app --args \
  --log \
  --webRemoteDebug
```

- HTML / Property Inspector：本机 Studio 3.1.9 已确认监听 `127.0.0.1:9292`。
- 插件主服务：Studio 已确认监听 `127.0.0.1:3906`，主服务日志出现 `MAIN WEBSOCKET OPEN`。

当前 `manifest.json` 没有 `Inspect` 字段，manifest 回归测试也明确要求该字段不存在，因此 `command_executor` 尚未启用 Node inspector，不能把 `--nodeRemoteDebug` 或 `chrome://inspect` 写成当前可用的调试入口。若确实需要 Node inspector，必须单独设计端口，在 manifest 增加 `Inspect`，同步测试，并重新完成 Studio 和 D200X 实机验证。

本机实测插件日志位于：

```bash
find "$HOME/Library/Application Support/Ulanzi/UlanziDeck/logs/com.ulanzi.ulanzistudio.commandexecutor" \
  -type f \
  -name '*.log' \
  -print
```

如果该目录不存在，再列出 Studio 的其他日志候选：

```bash
find "$HOME/Library/Application Support/Ulanzi" \
  -type f \
  \( -name '*.xlog' -o -name '*.mmap*' -o -name '*.log' \) \
  -print
```

本轮实际文件名为 `com.ulanzi.ulanzistudio.commandexecutor_<pid>.log`。其中已确认两个实体按键的 `run` 事件、退出码 0 和完成日志。查看日志时不要把完整 PATH、命令、环境变量值或 stdout / stderr 原文粘贴到公开位置。

## 9. SDK 升级检查清单

SDK 升级和业务功能修改应分开提交。升级时依次完成：

1. 记录新的 SDK 根仓库、`plugin-common-node` 和 `plugin-common-html` commit。
2. 按 `THIRD_PARTY_NOTICES.md` 的来源路径重新复制文件，保持原样，不做格式化。
3. 核对上游许可证，更新 `THIRD_PARTY_NOTICES.md` 和随包许可证。
4. 重新生成并人工复核 `tests/fixtures/ulanzi-sdk-sha256.json`。
5. 检查 Node 与 HTML SDK 接收、发送、设置相关的完整 payload 日志行为；本项目继续保留上游行为，并更新敏感信息边界说明。
6. 核对 `ws`、Node 目标版本、manifest 字段和 Ulanzi Studio 最低版本。
7. 运行 `npm ci`、`npm test`、`./build.sh` 和包校验。
8. 在当前 Ulanzi Studio 和 D200X 上重跑配置隔离、持久化、环境和 200 参数；并发和失败反馈至少保留自动化回归，发布前再按变更风险决定是否补实体回归。

自动化结果与实机证据统一记录到[验证报告](command-executor-validation.md)。`assets/command_executor/` 中只保存真实 Ulanzi Studio 窗口截图，不使用模拟图替代证据。
