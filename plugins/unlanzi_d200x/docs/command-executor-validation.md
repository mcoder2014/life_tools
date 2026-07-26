# Ulanzi D200X 命令执行器验证报告

## 1. 状态说明

本文分开记录自动化证据和真实 Studio / D200X 证据。`已验证` 只表示本轮已执行且有当前输出支持；自动化覆盖不能冒充实体按键证据。当前版本已经完成本机安装、Studio 重启、双按键配置和实体 D200X 执行验证。

当前本地自动化基线：

| 项目 | 值 |
|---|---|
| 日期 | 2026-07-26 |
| PR base | `d09f101f8031e03b2e5d62bfaf31d71f00e4a396` |
| 验证对象 | 当前 `git rev-parse HEAD` |
| macOS | 14.7.8（23H730） |
| 本地测试 Node.js | v24.10.0 |
| 本地 npm | 11.13.0 |
| Ulanzi Studio | 3.1.9 |
| Ulanzi Studio 内置 Node.js | v20.18.0 |
| Webpack target | `node20` |
| 设备 | D200X，Studio 显示已连接 |

## 2. 可复制验证命令

以下自动化命令都从仓库根目录进入同一个工作目录：

```bash
cd plugins/unlanzi_d200x/command_executor
```

记录环境并使用隔离 npm cache 从公共 registry 安装锁定依赖：

```bash
/usr/local/bin/node --version
/usr/bin/sw_vers
git rev-parse HEAD
git diff --name-only origin/master...HEAD
npm ci \
  --cache=/private/tmp/life_tools_ulanzi_npm_cache_task8 \
  --registry=https://registry.npmjs.org
npm --version
npm audit --registry=https://registry.npmjs.org
```

`git rev-parse HEAD` 返回执行命令时的分支顶端；`git diff --name-only origin/master...HEAD` 返回当前 PR 的完整文件范围。

当前 `npm audit` 因发现 advisory 而预期退出 1；退出 1 不是命令调用失败，也不能写成依赖安全检查通过。

单测、构建和包校验：

```bash
npm test
node --test tests/command-runner.test.mjs
node --test tests/command-plugin.test.mjs
bash -n build.sh
./build.sh
node scripts/validate-package.mjs \
  output/com.ulanzi.commandexecutor.ulanziPlugin
find output/com.ulanzi.commandexecutor.ulanziPlugin -type f |
  sed 's#^output/com\.ulanzi\.commandexecutor\.ulanziPlugin/##' |
  sort
unzip -t output/life_tools_ulanzi_d200x_command_executor.zip
shasum -a 256 \
  output/com.ulanzi.commandexecutor.ulanziPlugin/dist/app.js \
  output/life_tools_ulanzi_d200x_command_executor.zip \
  tests/fixtures/ulanzi-sdk-sha256.json
```

检查 zip 污染文件：

```bash
unzip -Z1 output/life_tools_ulanzi_d200x_command_executor.zip |
  rg '(__MACOSX|\.DS_Store|/\._|node_modules|\.map$)'
```

该 `rg` 预期无输出且退出码为 1，含义是没有匹配污染文件，不是验证失败。

Node 20 验证继续使用上述工作目录。前提是 Ulanzi Studio 已运行，并监听主服务 WebSocket `127.0.0.1:3906`：

```bash
/usr/sbin/lsof -nP -iTCP:3906 -sTCP:LISTEN
"/Applications/Ulanzi Studio.app/Contents/MacOS/NodeJS/node" --version
"/Applications/Ulanzi Studio.app/Contents/MacOS/NodeJS/node" --check \
  output/com.ulanzi.commandexecutor.ulanziPlugin/dist/app.js
"/Applications/Ulanzi Studio.app/Contents/MacOS/NodeJS/node" \
  output/com.ulanzi.commandexecutor.ulanziPlugin/dist/app.js
```

`lsof` 退出 0，摘要为监听项 `127.0.0.1:3906`；内置 Node 输出 `v20.18.0`，`--check` 退出 0。安装前直接运行 bundle 时输出 `MAIN WEBSOCKET OPEN: com.ulanzi.ulanzistudio.commandexecutor` 并保持连接，看到连接成功后人工按 Ctrl-C 停止。安装后由 Studio 启动的插件进程也在专用日志中记录了同一连接成功事件，并继续完成 UI 和 D200X 验证。

## 3. 固定验证矩阵

| 验证项 | 命令/操作 | 证据 | 结果 |
|---|---|---|---|
| 锁定依赖安装 | `npm ci --cache=/private/tmp/life_tools_ulanzi_npm_cache_task8 --registry=https://registry.npmjs.org` | 新 cache；125 packages；退出码 0；wall 18.64 s；安装命令当时的摘要为 2 vulnerabilities（1 low、1 high），不是当前 audit 结果 | 已验证，有依赖告警 |
| 当前依赖 audit | `npm audit --registry=https://registry.npmjs.org` | npm 11.13.0；退出码 1；wall 4.93 s；7 vulnerabilities（6 low、1 high）；`webpack` 依赖链和 `ws` 均显示 `No fix available` | 已验证，有未修复依赖告警 |
| Node 单测 | `npm test` | 最终本地 `node:test` 摘要：75 tests，75 pass，0 fail；`duration_ms` 2516.232708，wall 2.86 s；包含真实登录 Shell、环境覆盖、200 inline 参数、`context` 隔离、并发和打包校验用例 | 已验证 |
| Command runner 单测 | `node --test tests/command-runner.test.mjs` | 29 tests，29 pass，0 fail；`duration_ms` 354.196791，wall 0.42 s | 已验证 |
| Command plugin 单测 | `node --test tests/command-plugin.test.mjs` | 14 tests，14 pass，0 fail；`duration_ms` 1504.510417，wall 1.57 s | 已验证 |
| 构建脚本语法 | `bash -n build.sh` | 退出码 0；wall 0.00 s | 已验证 |
| 构建 | `./build.sh` | 最终 Webpack 454 ms；`dist/app.js` 181170 bytes，SHA-256 `ad6b0f6d524f51b74f4bb6dcfc021848247ed2c9a590f900ae41e4b49c29f36e`；构建目标 `node20` | 已验证，有可选依赖告警 |
| 包结构校验 | `node scripts/validate-package.mjs output/com.ulanzi.commandexecutor.ulanziPlugin` | 输出 `Package validation passed`；退出码 0；wall 0.12 s；安装目录共 40 个文件 | 已验证 |
| zip 完整性 | `unzip -t output/life_tools_ulanzi_d200x_command_executor.zip` | 输出 `No errors detected`；退出码 0；本轮最终 zip 95933 bytes，SHA-256 `bcc11057c9b5c2605f2d6b7460a1e6b10f3e4e44aea12f013bb127599cf7b1c4` | 已验证 |
| zip 污染文件 | `unzip -Z1 output/life_tools_ulanzi_d200x_command_executor.zip \| rg '(__MACOSX\|\\.DS_Store\|/\\._\|node_modules\|\\.map$)'` | `rg` 无输出且退出码 1，表示未匹配污染文件 | 已验证 |
| SDK digest | `shasum -a 256 tests/fixtures/ulanzi-sdk-sha256.json` | 23 个固定 vendored 文件；digest manifest SHA-256 `3a4cb77b517dc67ea00bb29dfb7531e2cafaf3658bdbb1f39faa9d573f2e40bc`；对应 SDK 测试包含在 75/75 总测试中 | 已验证 |
| Node 20 bundle 语法与入口连接 | 见第 2 节监听与 Studio 内置 Node 命令 | v20.18.0；`--check` 退出 0，wall 0.11 s；3906 有一个监听项；看到 `MAIN WEBSOCKET OPEN` 后立即人工中断，不以中断状态判定插件失败 | 已验证 |
| 插件加载 | 安装后完全重启 Studio | Studio 3.1.9 显示 D200X 已连接；插件列表出现“命令执行器 / 执行命令”；Property Inspector 正常加载 | 已验证 |
| 两按键隔离 | 分别配置按键 A / B，再切换读取 | `key=1_0` 和 `key=2_0` 使用不同 action id；A 恢复“环境验证”，B 恢复“语法与200参数”；截图见下文 | 已验证 |
| 无参数命令 | 实体 D200X 按键执行 `pwd > /private/tmp/life_tools_ulanzi_command_executor_e2e/no-args.txt` | `no-args.txt` 精确等于专用工作目录；日志存在 `key=2_0` 的实体 `run` 事件 | 已验证 |
| 管道 / 引号 / 重定向 | 实体 D200X 按键执行 `printf ... \| /usr/bin/sort > .../shell-syntax.txt` | 文件精确为两行 `alpha beta`、`gamma`，顺序符合 `sort` | 已验证 |
| 200 参数 | 实体 D200X 按键执行 inline `set --` | `arg-count.txt` 精确为 `200`；首尾参数精确为 `arg001`、`arg200` | 已验证 |
| 用户环境 | 实体按键输出 `$HOME` / `$USER` / `$PATH` | 三个值均非空；只记录断言结果，不把实际值写入仓库 | 已验证 |
| 自定义环境/目录 | 面板配置 `CUSTOM_ENV` 和专用工作目录 | `PWD` 精确等于专用目录，`CUSTOM_ENV` 精确等于测试值；日志存在 `key=1_0` 的实体 `run` 事件 | 已验证 |
| 并发 | 快速连续按两次包含 `sleep` 的命令 | `command-plugin` 自动化覆盖并发状态和收敛；本轮未再占用实体设备重复验证 | 自动化已验证 |
| 失败反馈 | 按键执行 `exit 7` | `command-plugin` 自动化覆盖 Alert、Toast、状态和日志；本轮实体配置保持为用户可用的成功命令 | 自动化已验证 |
| 配置持久化 | 完全重启 Studio 后重选两个按键 | 重启日志重新收到两个 action id 的配置；两按键恢复各自标题、命令、目录和环境变量 | 已验证 |
| 完整 payload 日志 | 触发配置和执行事件后检查 Studio 日志 | 专用日志确认 `MAIN WEBSOCKET OPEN`、两个按键的 `run` 和退出码 0 完成记录；原始命令和用户环境值不入库 | 已验证 |

表中标记为实体已验证的命令都由 D200X 普通按键触发。Studio 设备预览没有被用来替代硬件触发证据。

### 构建包文件清单

以下命令返回相对插件根目录的 40 个文件，与后续清单格式一致：

```bash
find output/com.ulanzi.commandexecutor.ulanziPlugin -type f |
  sed 's#^output/com\.ulanzi\.commandexecutor\.ulanziPlugin/##' |
  sort
```

```text
LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt
THIRD_PARTY_NOTICES.md
assets/icons/action.svg
assets/icons/plugin.svg
assets/icons/running.svg
assets/icons/success.svg
dist/app.js
en.json
libs/assets/u_active.svg
libs/assets/u_active_none.svg
libs/assets/u_check_checkbox.svg
libs/assets/u_check_none.svg
libs/assets/u_check_radio.svg
libs/assets/u_down.svg
libs/assets/u_file.svg
libs/assets/u_folder.svg
libs/assets/u_refresh.svg
libs/assets/u_tip_error.svg
libs/assets/u_tip_info.svg
libs/assets/u_tip_success.svg
libs/assets/u_tip_warn.svg
libs/css/uspi.css
libs/js/constants.js
libs/js/eventEmitter.js
libs/js/timers.js
libs/js/ulanziApi.js
libs/js/utils.js
manifest.json
package.json
plugin/app.js
plugin/command-plugin.js
plugin/command-runner.js
plugin/vendor/ulanzi-api/constants.js
plugin/vendor/ulanzi-api/ulanziApi.js
plugin/vendor/ulanzi-api/utils.js
property-inspector/inspector.css
property-inspector/inspector.html
property-inspector/inspector.js
property-inspector/settings.js
zh_CN.json
```

## 4. 自动化证据边界

本轮自动化已确认：

- 配置解析、NUL 拒绝、工作目录和 Shell 可执行性。
- `$SHELL -lc` 的完整管道、引号和 200 个 inline 参数语义。
- 面板显式环境变量在登录配置之后覆盖继承值。
- stdout / stderr 各自按 16 KiB 限制，含 UTF-8 边界；被保留的前 16 KiB 仍可能含敏感值。
- 不同 `context` 的配置和运行状态隔离，并发成功 / 失败状态收敛。
- manifest 的 UUID、macOS、D200X、Keypad、Studio 最低版本和资源路径。
- vendored SDK 文件集合、SHA-256、三个 commit、许可证与第三方声明。
- 构建包不存在 `node_modules`、测试、source map、AppleDouble 或绝对 worktree 路径。
- Studio 内置 Node.js v20.18.0 能通过 bundle 语法检查，并能由主服务入口连接当前 Studio WebSocket。

自动化证据本身不能证明 Ulanzi Studio 安装和实体 D200X 行为，因此本轮又独立完成了真实安装与硬件触发。实机证据确认安装目录加载、Property Inspector、两个 `context` 的配置持久化、用户环境、工作目录、Shell 语法和 200 参数。并发与失败反馈仍以自动化回归为证据，未把它们写成实体已验证。

依赖安装命令当时只摘要报告 2 个漏洞（1 low、1 high），该数字是 `npm ci` 的历史输出快照，不能代表当前 advisory 状态。使用 npm 11.13.0 和官方 registry 重新执行 `npm audit` 后，当前结果为 7 个漏洞（6 low、1 high）、退出码 1：`webpack` 依赖链和 `ws` 两组结果均明确显示 `No fix available`。输出末尾的通用修复提示不能证明这些 advisory 存在可用的自动修复路径；测试和构建通过也不能证明依赖风险已经消失。

构建过程中 Webpack 对 `ws` 的可选原生增强模块 `bufferutil` 和 `utf-8-validate` 各给出一条未安装警告，但构建退出码为 0，`ws` 已进入单文件 bundle，包校验和 zip 校验均通过。随后 Studio 3.1.9 和 D200X 实测成功，说明这两个可选模块未阻断当前 macOS 运行路径；这不等于依赖告警已经消失。

重复构建时 bundle SHA-256 保持不变，但 zip SHA-256 会随归档时间戳变化；表格中的 zip digest 只标识本轮最终产物，不是可复现构建承诺。

## 5. 实机验证记录

| 字段 | 实测值 |
|---|---|
| Ulanzi Studio 版本 | 3.1.9，来源为本机 `UlanziDeck/version.txt` |
| 设备型号 / 连接状态 | D200X / Studio 显示已连接 |
| 安装源目录 | `command_executor/output/com.ulanzi.commandexecutor.ulanziPlugin` |
| 预期安装目录 | `~/Library/Application Support/Ulanzi/UlanziDeck/Plugins/com.ulanzi.commandexecutor.ulanziPlugin` |
| 实际安装目录 | 与预期目录一致；40 个文件 |
| 安装 bundle 校验 | 安装目录和构建目录 `dist/app.js` SHA-256 均为 `ad6b0f6d524f51b74f4bb6dcfc021848247ed2c9a590f900ae41e4b49c29f36e` |
| 实际插件日志路径 | `~/Library/Application Support/Ulanzi/UlanziDeck/logs/com.ulanzi.ulanzistudio.commandexecutor/com.ulanzi.ulanzistudio.commandexecutor_<pid>.log` |
| 实体事件 | 日志确认 `key=1_0`、`key=2_0` 的 `run` 事件和退出码 0 完成记录 |
| 安全输出断言 | 用户环境、无参数命令、管道/引号/重定向、200 参数、首尾参数全部通过 |

实机命令只写入：

```text
/private/tmp/life_tools_ulanzi_command_executor_e2e
```

仓库只保留一张使用隔离默认预设重新采集的 Studio 插件加载截图。截图不包含用户 HOME、USER、PATH、磁盘容量、CPU/RAM/GPU 状态、设备序列号、令牌或原有按键布局；SDK 完整 payload 日志只在本机核对。

### 5.1 插件加载

![Studio 已加载命令执行器](assets/command_executor/installed-plugin.jpg)

Property Inspector、两按键独立配置和实体按键执行后的全窗口截图不入库，避免携带无关的本机插件布局与实时资源状态。对应结论由专用日志中的硬件 `run` 事件、退出码 0 记录、配置恢复记录和五项结果文件断言共同证明。
