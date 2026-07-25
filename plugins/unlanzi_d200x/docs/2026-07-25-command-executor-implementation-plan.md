# Ulanzi D200X 命令执行器实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `plugins/unlanzi_d200x/command_executor/` 实现一个 macOS-only、D200X Keypad-only 的 Ulanzi Studio 插件，使每个拖入按键的 Action 都能按 `context` 独立配置并执行接近终端输入体验的完整 Shell 命令。

**Architecture:** Property Inspector 只负责编辑当前 `context` 的 `title`、`command`、`workingDirectory`、`environment`，并把配置发送给 Ulanzi Studio。Node.js 主服务按 `context` 缓存配置和运行计数，通过当前用户的 `$SHELL -lc` 异步启动命令；显式环境变量在登录配置加载后导出，stdout/stderr 分别限制为 16 KiB 后写入本地插件日志。插件原样复用固定版本的 Ulanzi `common-html` 和 `common-node`，保留官方 SDK 的完整 payload 诊断日志，方便在本机排查配置和协议问题。Studio 是否落盘以及重启后能否恢复必须通过实机验证。

**Tech Stack:** Ulanzi JS Plugin Protocol V2.1.2、Node.js 20、ES modules、`node:test`、Webpack 5、原生 HTML/CSS/JavaScript、macOS `/bin/zsh`、GitHub Actions `macos-latest`。

---

## 0. 已确认基线

| 项目 | 固定值 |
|---|---|
| 开发 worktree | `<worktree-root>/life_tools-ulanzi_d200x` |
| 开发分支 | `feat/cq/ulanzi_d200x` |
| 插件包目录 | `com.ulanzi.commandexecutor.ulanziPlugin` |
| 主服务 UUID | `com.ulanzi.ulanzistudio.commandexecutor` |
| Action UUID | `com.ulanzi.ulanzistudio.commandexecutor.runcommand` |
| 初始版本 | `0.1.0` |
| SDK 根仓库 commit | `550ab80c69285ecf259bd494a7fff767c14f0c0f` |
| `plugin-common-node` commit | `112bd13a7ff9d45bd68656f7e069fd61851d1812` |
| `plugin-common-html` commit | `79de0b0b087546e684afd23f97223f7a7bc392da` |
| Node SDK 依赖 | `ws@8.18.0` |
| Studio 最低版本 | `3.0.11` |
| macOS 最低版本 | `10.15` |
| 单流日志上限 | stdout 16 KiB；stderr 16 KiB |
| Property Inspector 发送防抖 | 200 ms |
| 成功状态显示时间 | 1200 ms |

当前不实现 Windows、Encoder、参数文件、预设管理器、交互式终端、`sudo` 密码输入、超时、终止进程、守护进程或执行历史。

## 1. 最终目录

```text
plugins/unlanzi_d200x/
├── command_executor/
│   ├── com.ulanzi.commandexecutor.ulanziPlugin/
│   │   ├── package.json
│   │   ├── manifest.json
│   │   ├── en.json
│   │   ├── zh_CN.json
│   │   ├── THIRD_PARTY_NOTICES.md
│   │   ├── LICENSES/
│   │   │   └── UlanziDeckPlugin-SDK-APACHE-2.0.txt
│   │   ├── assets/icons/
│   │   │   ├── plugin.svg
│   │   │   ├── action.svg
│   │   │   ├── running.svg
│   │   │   └── success.svg
│   │   ├── libs/
│   │   │   ├── assets/
│   │   │   ├── css/
│   │   │   └── js/
│   │   ├── plugin/
│   │   │   ├── app.js
│   │   │   ├── command-plugin.js
│   │   │   ├── command-runner.js
│   │   │   └── vendor/ulanzi-api/
│   │   │       ├── constants.js
│   │   │       ├── ulanziApi.js
│   │   │       └── utils.js
│   │   ├── property-inspector/
│   │   │   ├── inspector.html
│   │   │   ├── inspector.css
│   │   │   ├── inspector.js
│   │   │   └── settings.js
│   │   └── dist/
│   │       └── app.js
│   ├── scripts/
│   │   └── validate-package.mjs
│   ├── tests/
│   │   ├── command-plugin.test.mjs
│   │   ├── command-runner.test.mjs
│   │   ├── inspector-settings.test.mjs
│   │   ├── manifest.test.mjs
│   │   └── sdk-vendor.test.mjs
│   ├── build.sh
│   ├── package.json
│   ├── package-lock.json
│   └── webpack.config.js
└── docs/
    ├── README.md
    ├── 2026-07-25-command-executor-design.md
    ├── 2026-07-25-command-executor-implementation-plan.md
    ├── ulanzi-plugin-development-reference.md
    ├── command-executor-installation.md
    ├── command-executor-development-guide.md
    ├── command-executor-user-guide.md
    ├── command-executor-validation.md
    └── assets/command_executor/
```

`dist/`、`output/` 和 `node_modules/` 都是生成物。安装和打包只复制 `.ulanziPlugin` 目录，不复制仓库级构建依赖。

## 2. 核心接口约定

### 2.1 按键配置

```js
{
  title: "执行命令",
  command: "ls -alh /home",
  workingDirectory: "~/work",
  environment: "FOO=bar\nHTTP_PROXY=http://127.0.0.1:7890"
}
```

- `command` 是完整 Shell 内容，不拆成参数数组，也不限制参数数量。
- `workingDirectory` 只接受空值、绝对路径、`~` 或 `~/...`。
- `environment` 每行一个 `KEY=VALUE`，按第一个 `=` 拆分；空行忽略；同名项最后一行覆盖前面值。
- 环境变量名必须满足 `[A-Za-z_][A-Za-z0-9_]*`。
- 命令和环境变量值都拒绝 NUL 字符；Toast 和按键错误反馈只报告字段或行号。官方 SDK 日志保留完整事件 payload，可能包含这些配置值。

### 2.2 `command-runner.js`

必须导出以下符号，测试直接覆盖它们：

```js
OUTPUT_LIMIT_BYTES
parseEnvironment(text)
quoteShellValue(value)
buildShellScript(command, environment)
resolveWorkingDirectory(value, options)
resolveShell(options)
runCommand(settings, options)
```

`runCommand` 的稳定返回值：

```js
{
  code: 0,
  signal: null,
  stdout: "",
  stderr: "",
  stdoutTruncated: false,
  stderrTruncated: false,
  spawnError: null
}
```

`options` 只用于测试注入：

```js
{
  spawnFn,
  fileSystem,
  homeDirectory,
  baseEnvironment,
  outputLimitBytes
}
```

真实执行必须等价于：

```js
spawn(shell, ["-lc", wrappedScript], {
  cwd: workingDirectory,
  env: baseEnvironment,
  stdio: ["ignore", "pipe", "pipe"]
});
```

显式环境变量不能直接只合并进 `spawn(..., {env})`，否则登录 Shell 初始化脚本可能再次覆盖它。正确包装顺序是：

```sh
export KEY='value'
export PATH='/explicit/path'
<用户的完整 command 原文>
```

### 2.3 `command-plugin.js`

只导出一个注册入口：

```js
export function registerCommandPlugin(api, options = {}) {}
```

`options` 包含：

```js
{
  runCommandFn,
  setTimeoutFn,
  clearTimeoutFn
}
```

内部只维护一张 `Map<context, instance>`：

```js
{
  settings,
  runningCount: 0,
  failedInBurst: false,
  restoreTimer: null
}
```

事件映射固定如下：

| Ulanzi 事件 | 处理 |
|---|---|
| `onAdd` | 创建或刷新当前 `context`，显示默认图标和标题 |
| `onParamFromApp` | 用 Studio 回传参数刷新当前 `context` |
| `onParamFromPlugin` | 用 Property Inspector 新参数刷新当前 `context` |
| `onSetActive` | `active=true` 时重画当前按键状态 |
| `onRun` | 使用当前 `context` 配置异步执行一次 |
| `onClear` | 遍历 `jsn.param`，清理对应实例和恢复定时器 |

同一按键连续触发时每次都启动独立子进程。只要 `runningCount > 0` 就保持运行图标；一个并发批次全部成功才显示成功图标，任何一次失败都调用 `showAlert` 并在全部结束后恢复默认图标。

### 2.4 Property Inspector

`settings.js` 以经典脚本加载，并在 `globalThis.CommandExecutorSettings` 暴露以下值：

```js
globalThis.CommandExecutorSettings = {
  DEFAULT_SETTINGS: {
    title: "执行命令",
    command: "",
    workingDirectory: "",
    environment: ""
  },
  normalizeSettings,
  validateEnvironmentText,
  buildExecutionPreview
};
```

执行预览只显示 Shell、工作目录、环境变量名称和命令原文，不复制环境变量值：

```text
Shell: $SHELL -lc
工作目录: $HOME
环境变量: FOO, HTTP_PROXY
命令:
ls -alh /home
```

配置有格式错误时仍发送给 Studio，且不会被面板立即清空；真正执行时由主服务再次严格校验并拒绝启动。Studio 是否落盘以及切换按键或重启后能否恢复必须通过实机验证。

## 3. Task 1：建立 npm 工程并固定 SDK 快照

**Files:**

- Modify: `.gitignore`
- Create: `plugins/unlanzi_d200x/command_executor/package.json`
- Create: `plugins/unlanzi_d200x/command_executor/package-lock.json`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/package.json`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/THIRD_PARTY_NOTICES.md`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/plugin/vendor/ulanzi-api/{constants.js,ulanziApi.js,utils.js}`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/libs/{assets,css,js}/**`
- Create: `plugins/unlanzi_d200x/command_executor/tests/sdk-vendor.test.mjs`

- [ ] 在 `.gitignore` 增加 `node_modules/`；保留现有 `plugins/**/dist/` 和 `output` 规则。

- [ ] 先写 SDK 来源回归测试。测试必须确认 vendored 文件、许可证、第三方声明和固定 commit 都存在：

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const pluginRoot = new URL(
  "../com.ulanzi.commandexecutor.ulanziPlugin/",
  import.meta.url
);

test("vendored SDK files and provenance are complete", async () => {
  const requiredFiles = [
    "plugin/vendor/ulanzi-api/constants.js",
    "plugin/vendor/ulanzi-api/ulanziApi.js",
    "plugin/vendor/ulanzi-api/utils.js",
    "libs/js/ulanziApi.js",
    "libs/js/utils.js",
    "LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt"
  ];
  for (const path of requiredFiles) {
    const source = await readFile(new URL(path, pluginRoot), "utf8");
    assert.ok(source.length > 0, `${path} must not be empty`);
  }

  const notices = await readFile(
    new URL("THIRD_PARTY_NOTICES.md", pluginRoot),
    "utf8"
  );
  assert.match(notices, /112bd13a7ff9d45bd68656f7e069fd61851d1812/);
  assert.match(notices, /79de0b0b087546e684afd23f97223f7a7bc392da/);
  assert.match(notices, /Apache License 2\.0/);
});
```

- [ ] 运行单测，确认它先因 vendored 文件不存在而失败：

```bash
cd plugins/unlanzi_d200x/command_executor
node --test tests/sdk-vendor.test.mjs
```

预期：`ENOENT`，退出码非 0。

- [ ] 从两个官方仓库的固定 commit 复制文件。下载必须发生在 `mktemp -d` 创建的临时目录，不把 `.git/`、`node_modules/` 或上游 demo 带入仓库：

```bash
vendor_tmp="$(mktemp -d)"
git clone --filter=blob:none https://github.com/UlanziTechnology/plugin-common-node.git "$vendor_tmp/common-node"
git -C "$vendor_tmp/common-node" checkout 112bd13a7ff9d45bd68656f7e069fd61851d1812
git clone --filter=blob:none https://github.com/UlanziTechnology/plugin-common-html.git "$vendor_tmp/common-html"
git -C "$vendor_tmp/common-html" checkout 79de0b0b087546e684afd23f97223f7a7bc392da
```

复制范围固定为：

```text
common-node/libs/constants.js -> plugin/vendor/ulanzi-api/constants.js
common-node/libs/ulanziApi.js  -> plugin/vendor/ulanzi-api/ulanziApi.js
common-node/libs/utils.js      -> plugin/vendor/ulanzi-api/utils.js
common-html/assets/            -> libs/assets/
common-html/css/               -> libs/css/
common-html/js/                -> libs/js/
```

- [ ] vendored SDK 保持固定 commit 的原始内容，不删除或改写完整 payload、`setSettings`、`send` 等诊断日志。Property Inspector 直接使用官方脚本末尾创建的全局 `$UD`。

- [ ] `THIRD_PARTY_NOTICES.md` 记录三个 commit、原始路径和 Apache-2.0 许可证，并明确这些 vendored 文件未做业务修改。官方 SDK 日志可能包含命令和环境变量配置，这属于本机调试行为。

- [ ] 创建仓库级 `package.json`：

```json
{
  "name": "life-tools-ulanzi-command-executor",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "engines": {
    "node": ">=20"
  },
  "scripts": {
    "test": "node --test tests/*.test.mjs",
    "bundle": "webpack --config webpack.config.js",
    "build": "./build.sh"
  },
  "dependencies": {
    "ws": "8.18.0"
  },
  "devDependencies": {
    "webpack": "5.94.0",
    "webpack-cli": "5.1.4"
  }
}
```

- [ ] 创建插件包内的运行时 `package.json`，它只声明 ESM 边界，不携带安装脚本或依赖：

```json
{
  "name": "com.ulanzi.commandexecutor",
  "version": "0.1.0",
  "private": true,
  "type": "module"
}
```

- [ ] 执行 `npm install --package-lock-only` 生成锁文件，再执行 `npm ci` 安装构建依赖。

- [ ] 重跑 SDK 来源测试，预期 1 个测试通过。

- [ ] 检查本任务 diff 后提交：

```bash
git add .gitignore plugins/unlanzi_d200x/command_executor
git commit -m "build: vendor Ulanzi plugin SDK"
```

## 4. Task 2：用 TDD 实现 Shell 配置与命令执行

**Files:**

- Create: `plugins/unlanzi_d200x/command_executor/tests/command-runner.test.mjs`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/plugin/command-runner.js`

- [ ] 先写纯逻辑测试，覆盖以下精确断言：

| 场景 | 断言 |
|---|---|
| 空环境配置 | 返回空对象 |
| 空行和 CRLF | 空行忽略，`\r` 不进入值 |
| 值含多个 `=` | 只按第一个 `=` 拆分 |
| 重复变量 | 最后一行覆盖 |
| 非法变量名 | 错误包含 1-based 行号，不包含原值 |
| NUL | 命令或环境值被拒绝 |
| 单引号 | `a'b` 编码为 `'a'"'"'b'` |
| 空工作目录 | 解析为 `homeDirectory` |
| `~` 和 `~/x` | 正确展开 |
| 相对路径和 `~user` | 被拒绝 |
| 非目录或不可执行目录 | 被拒绝 |
| Shell 候选有效 | 使用 `baseEnvironment.SHELL` |
| Shell 候选无效 | 回退 `/bin/zsh` |
| 两者都不可执行 | 在 spawn 前失败 |
| spawn 参数 | 严格为 `[shell, ["-lc", script], options]` |
| stdin | `stdio[0] === "ignore"` |
| 输出截断 | stdout/stderr 各自最多 `outputLimitBytes`，并设置标记 |
| spawn 失败 | 返回 `spawnError`，不产生未处理 rejection |

环境包装测试使用真实 Shell 验证，不能只比较字符串：

```js
test("configured environment overrides login shell values", async () => {
  const result = await runCommand(
    {
      command: "printf '%s' \"$COMMAND_EXECUTOR_TEST_VALUE\"",
      workingDirectory: "",
      environment: "COMMAND_EXECUTOR_TEST_VALUE=explicit"
    },
    {
      baseEnvironment: {
        ...process.env,
        SHELL: "/bin/zsh",
        COMMAND_EXECUTOR_TEST_VALUE: "inherited"
      }
    }
  );

  assert.equal(result.code, 0);
  assert.equal(result.stdout, "explicit");
});
```

200 参数测试直接构造完整命令内容，不使用参数文件：

```js
test("passes 200 inline shell arguments without parsing them in JavaScript", async () => {
  const args = Array.from({ length: 200 }, (_, index) =>
    `arg${String(index + 1).padStart(3, "0")}`
  );
  const result = await runCommand({
    command: `set -- ${args.join(" ")}; printf '%s' "$#"`,
    workingDirectory: "",
    environment: ""
  });

  assert.equal(result.code, 0);
  assert.equal(result.stdout, "200");
});
```

- [ ] 运行测试并确认因模块不存在而失败：

```bash
node --test tests/command-runner.test.mjs
```

- [ ] 实现 `parseEnvironment`。伪代码必须落实为同等语义，不增加 JSON/YAML 或参数数组模式：

```js
export function parseEnvironment(text = "") {
  const parsed = {};
  const lines = String(text).split(/\r?\n/);

  lines.forEach((line, index) => {
    if (line.trim() === "") return;
    const separator = line.indexOf("=");
    const name = separator >= 0 ? line.slice(0, separator).trim() : "";
    const value = separator >= 0 ? line.slice(separator + 1) : "";
    if (!ENVIRONMENT_NAME.test(name)) {
      throw new Error(`环境变量第 ${index + 1} 行格式错误`);
    }
    if (value.includes("\0")) {
      throw new Error(`环境变量第 ${index + 1} 行包含不支持的 NUL 字符`);
    }
    parsed[name] = value;
  });

  return parsed;
}
```

- [ ] 实现 Shell 单引号编码和包装：

```js
export function quoteShellValue(value) {
  const text = String(value);
  const escaped = text.replaceAll("'", `'\"'\"'`);
  return `'${escaped}'`;
}

export function buildShellScript(command, environment) {
  if (typeof command !== "string" || command.trim() === "") {
    throw new Error("命令不能为空");
  }
  if (command.includes("\0")) {
    throw new Error("命令包含不支持的 NUL 字符");
  }

  const exports = Object.entries(environment).map(
    ([name, value]) => `export ${name}=${quoteShellValue(value)}`
  );
  return exports.length === 0 ? command : `${exports.join("\n")}\n${command}`;
}
```

- [ ] 实现工作目录和 Shell 校验。目录必须经过 `stat().isDirectory()` 与 `access(X_OK)`；`SHELL` 必须是绝对路径并可执行。

- [ ] 实现有界输出收集。stdout 和 stderr 必须持续 drain，但每个流只保存前 `outputLimitBytes` 个字节：

```js
function appendBounded(chunks, state, chunk, limit) {
  const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
  const remaining = Math.max(0, limit - state.bytes);
  if (remaining > 0) {
    const accepted = buffer.subarray(0, remaining);
    chunks.push(accepted);
    state.bytes += accepted.length;
  }
  if (buffer.length > remaining) state.truncated = true;
}
```

- [ ] `runCommand` 关闭 stdin、监听 `error` 和 `close`，保证只 resolve 一次；不要记录 `settings.command` 或 `settings.environment`。

- [ ] 运行：

```bash
npm test
```

预期：命令执行测试和 SDK 来源测试全部通过；真实 Shell 测试的退出码为 0。

- [ ] 提交：

```bash
git add plugins/unlanzi_d200x/command_executor
git commit -m "feat: execute configured macOS shell commands"
```

## 5. Task 3：用 TDD 实现按 context 隔离的 Ulanzi 事件控制器

**Files:**

- Create: `plugins/unlanzi_d200x/command_executor/tests/command-plugin.test.mjs`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/plugin/command-plugin.js`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/plugin/app.js`

- [ ] 在测试里实现最小 fake API。fake 只模拟本插件使用的方法，不复制 WebSocket SDK：

```js
class FakeUlanziApi {
  handlers = new Map();
  calls = [];

  onAdd(fn) {
    this.handlers.set("add", fn);
    return this;
  }

  onParamFromApp(fn) {
    this.handlers.set("paramfromapp", fn);
    return this;
  }

  onParamFromPlugin(fn) {
    this.handlers.set("paramfromplugin", fn);
    return this;
  }

  onSetActive(fn) {
    this.handlers.set("setactive", fn);
    return this;
  }

  onRun(fn) {
    this.handlers.set("run", fn);
    return this;
  }

  onClear(fn) {
    this.handlers.set("clear", fn);
    return this;
  }

  setStateIcon(...args) {
    this.calls.push(["setStateIcon", ...args]);
  }

  showAlert(...args) {
    this.calls.push(["showAlert", ...args]);
  }

  toast(...args) {
    this.calls.push(["toast", ...args]);
  }

  logMessage(...args) {
    this.calls.push(["logMessage", ...args]);
  }

  async emit(name, payload) {
    return this.handlers.get(name)?.(payload);
  }
}
```

- [ ] 先写失败测试，覆盖：

  1. 两个 `context` 维护不同命令，分别 `onRun` 时传给 runner 的配置不串线。
  2. `onParamFromPlugin` 更新后下一次执行使用新值。
  3. `onRun` 自带 `param` 时可以补建尚未缓存的实例。
  4. 命令为空时不调用 runner，调用 `showAlert` 和 `toast`。
  5. 成功时依次出现 state 1、state 2、1200 ms 后 state 0。
  6. 非零退出码或 `spawnError` 时调用 `showAlert`，Toast 不包含命令和环境变量。
  7. stdout/stderr 写入 `logMessage`，截断时带“输出已截断”。
  8. 两次并发执行完成一个后仍保持 state 1；全部成功后才 state 2。
  9. 并发批次任意一次失败，则批次结束不显示 state 2。
  10. `onClear` 清除不存在的 context 不抛错；清除运行中的 context 后，晚到的 Promise 不再改图标。

- [ ] 运行：

```bash
node --test tests/command-plugin.test.mjs
```

预期：因 `command-plugin.js` 不存在而失败。

- [ ] 实现配置归一化，字段保留为字符串，标题空值回退“执行命令”，不得对 `command` 做 `trim()` 后再传递：

```js
function normalizeSettings(value = {}) {
  const title = typeof value.title === "string" ? value.title.trim() : "";
  return {
    title: title || "执行命令",
    command: typeof value.command === "string" ? value.command : "",
    workingDirectory:
      typeof value.workingDirectory === "string" ? value.workingDirectory : "",
    environment:
      typeof value.environment === "string" ? value.environment : ""
  };
}
```

- [ ] 实现 `registerCommandPlugin`。所有 `onRun` 异步错误都在函数内部处理，不能让 EventEmitter 收到未处理 Promise rejection。

运行状态核心流程固定为：

```js
instance.runningCount += 1;
instance.failedInBurst =
  instance.runningCount === 1 ? false : instance.failedInBurst;
api.setStateIcon(context, 1, instance.settings.title);

try {
  const result = await runCommandFn(instance.settings);
  if (result.spawnError || result.code !== 0 || result.signal) {
    instance.failedInBurst = true;
    reportFailure(api, context, result);
  } else {
    reportOutput(api, context, result);
  }
} catch (error) {
  instance.failedInBurst = true;
  reportValidationFailure(api, context, error);
} finally {
  instance.runningCount -= 1;
  finishRun(api, context, instance);
}
```

`finishRun` 在更新 UI 前必须确认 `instances.get(context) === instance`，避免已移除按键被晚到回调重新写状态。

- [ ] `app.js` 只做明确的三步启动：

```js
import UlanziApi from "./vendor/ulanzi-api/ulanziApi.js";
import { registerCommandPlugin } from "./command-plugin.js";

const api = new UlanziApi();
registerCommandPlugin(api);
api.connect("com.ulanzi.ulanzistudio.commandexecutor");
```

- [ ] 运行全部测试，预期通过。

- [ ] 提交：

```bash
git add plugins/unlanzi_d200x/command_executor
git commit -m "feat: route Ulanzi actions by button context"
```

## 6. Task 4：实现 manifest、图标和 Property Inspector

**Files:**

- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/manifest.json`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/{en.json,zh_CN.json}`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/assets/icons/{plugin.svg,action.svg,running.svg,success.svg}`
- Create: `plugins/unlanzi_d200x/command_executor/com.ulanzi.commandexecutor.ulanziPlugin/property-inspector/{inspector.html,inspector.css,inspector.js,settings.js}`
- Create: `plugins/unlanzi_d200x/command_executor/tests/{inspector-settings.test.mjs,manifest.test.mjs}`

- [ ] 先写 `manifest.test.mjs`，验证：

  - JSON 可解析。
  - 主 UUID 恰好 4 段，Action UUID 至少 5 段。
  - `Type === "JavaScript"`。
  - 仅一个 `OS` 项且 `Platform === "mac"`。
  - Action 的 `Devices` 严格等于 `["D200X"]`。
  - Action 的 `Controllers` 严格等于 `["Keypad"]`。
  - `DisableAutomaticStates === true`。
  - `SupportedInMultiActions === false`。
  - `States` 严格为 3 个，所有 `Image` 和 `Icon` 路径存在。
  - `PropertyInspectorPath`、顶层 `Icon`、`CategoryIcon` 存在。
  - 包内 `package.json` 为 `type: module`。

- [ ] 先写 `inspector-settings.test.mjs`，用 `node:vm` 在独立上下文执行经典脚本并读取 `context.CommandExecutorSettings`，覆盖默认值、CRLF、非法变量名行号、值含等号、预览不含环境变量值、长命令不被截断：

```js
const source = await readFile(settingsPath, "utf8");
const context = vm.createContext({});
vm.runInContext(source, context);
const {
  normalizeSettings,
  validateEnvironmentText,
  buildExecutionPreview
} = context.CommandExecutorSettings;
```

- [ ] 运行两个测试并确认先失败。

- [ ] 创建完整 `manifest.json`：

```json
{
  "Version": "0.1.0",
  "Author": "mcoder2014",
  "Name": "命令执行器",
  "Description": "在当前 macOS 用户的登录 Shell 中执行每个按键独立配置的命令",
  "Icon": "assets/icons/plugin.svg",
  "Category": "命令执行器",
  "CategoryIcon": "assets/icons/plugin.svg",
  "CodePath": "dist/app.js",
  "Type": "JavaScript",
  "SupportedInMultiActions": false,
  "UUID": "com.ulanzi.ulanzistudio.commandexecutor",
  "Actions": [
    {
      "Name": "执行命令",
      "Icon": "assets/icons/action.svg",
      "PropertyInspectorPath": "property-inspector/inspector.html",
      "state": 0,
      "States": [
        {
          "Image": "assets/icons/action.svg"
        },
        {
          "Image": "assets/icons/running.svg"
        },
        {
          "Image": "assets/icons/success.svg"
        }
      ],
      "Tooltip": "执行配置的 Shell 命令",
      "UUID": "com.ulanzi.ulanzistudio.commandexecutor.runcommand",
      "Controllers": [
        "Keypad"
      ],
      "Devices": [
        "D200X"
      ],
      "DisableAutomaticStates": true,
      "SupportedInMultiActions": false
    }
  ],
  "OS": [
    {
      "Platform": "mac",
      "MinimumVersion": "10.15"
    }
  ],
  "Software": {
    "MinVersion": "3.0.11"
  }
}
```

- [ ] 图标使用仓库自有 SVG，不复制第三方商标。四个图标都使用 `viewBox="0 0 144 144"`，保证深色背景可辨识；运行态使用蓝色圆环，成功态使用绿色勾。

- [ ] `inspector.html` 必须包含四个具名字段和两个只读区域：

```html
<form id="property-inspector">
  <input id="title" name="title" type="text" />
  <textarea id="command" name="command" rows="8"></textarea>
  <input id="workingDirectory" name="workingDirectory" type="text" />
  <textarea id="environment" name="environment" rows="5"></textarea>
  <p id="validation-message" role="status" aria-live="polite"></p>
  <pre id="execution-preview" aria-label="执行预览"></pre>
</form>
```

页面脚本顺序固定为：

```html
<script src="../libs/js/constants.js"></script>
<script src="../libs/js/eventEmitter.js"></script>
<script src="../libs/js/timers.js"></script>
<script src="../libs/js/utils.js"></script>
<script src="../libs/js/ulanziApi.js"></script>
<script src="./settings.js"></script>
<script src="./inspector.js"></script>
```

- [ ] `inspector.js` 与官方 Property Inspector 一样使用经典脚本作用域中的 `$UD`，并同时监听 `onAdd` 与 `onParamFromApp`：

```js
const ACTION_UUID =
  "com.ulanzi.ulanzistudio.commandexecutor.runcommand";
const {
  normalizeSettings,
  validateEnvironmentText,
  buildExecutionPreview
} = globalThis.CommandExecutorSettings;
const api = $UD;
let currentSettings = normalizeSettings();
let form = null;
const debouncedFlush = Utils.debounce(flushSettings, 200);

function handleInput() {
  captureSettings();
  debouncedFlush();
}

api.connect(ACTION_UUID);
api.onConnected(() => {
  form = document.querySelector("#property-inspector");
  document.querySelector(".udpi-wrapper").classList.remove("hidden");
  form.addEventListener("input", handleInput);
  renderSettings();
});
api.onAdd((message) => applyIncomingSettings(message?.param));
api.onParamFromApp((message) => applyIncomingSettings(message?.param));
```

`handleInput()` 先通过 `captureSettings()` 收集表单值并更新校验和预览，再调用 `debouncedFlush()`。`flushSettings()` 只负责把 `currentSettings` 通过 `api.sendParamFromPlugin(currentSettings)` 发送给 Studio。代码中不得 `console.log(currentSettings)`。

- [ ] 环境变量格式错误显示在 `validation-message`；预览使用 `textContent`，不得使用 `innerHTML` 拼入用户命令。

- [ ] 运行：

```bash
npm test
```

预期：所有测试通过。

- [ ] 提交：

```bash
git add plugins/unlanzi_d200x/command_executor
git commit -m "feat: add D200X command configuration panel"
```

## 7. Task 5：构建、校验和可安装包

**Files:**

- Create: `plugins/unlanzi_d200x/command_executor/webpack.config.js`
- Create: `plugins/unlanzi_d200x/command_executor/scripts/validate-package.mjs`
- Create: `plugins/unlanzi_d200x/command_executor/build.sh`
- Modify: `plugins/unlanzi_d200x/command_executor/tests/manifest.test.mjs`

- [ ] 先扩展 manifest 测试：当 `dist/app.js` 存在时，要求它非空且不包含绝对 worktree 路径。

- [ ] 创建最小 Webpack 配置；不引入 Babel、Terser、CopyWebpackPlugin：

```js
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = path.join(
  root,
  "com.ulanzi.commandexecutor.ulanziPlugin"
);

export default {
  mode: "production",
  target: "node20",
  entry: path.join(pluginRoot, "plugin/app.js"),
  output: {
    path: path.join(pluginRoot, "dist"),
    filename: "app.js",
    library: {
      type: "module"
    },
    chunkFormat: "module"
  },
  experiments: {
    outputModule: true
  },
  optimization: {
    minimize: false
  },
  devtool: false
};
```

`ws` 必须打入单个 `dist/app.js`，安装包不携带 `node_modules/`。

- [ ] `validate-package.mjs` 接收一个 `.ulanziPlugin` 目录，检查：

  1. basename 严格为 `com.ulanzi.commandexecutor.ulanziPlugin`。
  2. manifest 和包内 package JSON 可解析。
  3. 所有 manifest 资源路径留在插件目录内，禁止 `..` 逃逸。
  4. 所有声明文件存在。
  5. `dist/app.js` 非空。
  6. 包内不存在 `.DS_Store`、`._*`、`__MACOSX`、`node_modules`、测试文件或源码映射。
  7. vendored Node/HTML SDK 文件和固定 commit 声明存在。
  8. LICENSE 和 THIRD_PARTY_NOTICES 存在。

- [ ] `build.sh` 只清理自己的两个精确生成目标，随后构建、复制、校验、压缩：

```bash
#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
plugin_name="com.ulanzi.commandexecutor.ulanziPlugin"
source_plugin="$script_dir/$plugin_name"
output_root="$script_dir/output"
output_plugin="$output_root/$plugin_name"
archive="$output_root/life_tools_ulanzi_d200x_command_executor.zip"

cd "$script_dir"
npm run bundle
rm -rf "$output_plugin"
rm -f "$archive"
mkdir -p "$output_root"
/usr/bin/ditto --norsrc "$source_plugin" "$output_plugin"
node scripts/validate-package.mjs "$output_plugin"
(
  cd "$output_root"
  /usr/bin/ditto -c -k --norsrc --keepParent "$plugin_name" "$archive"
)
/usr/bin/unzip -t "$archive"
```

- [ ] 给脚本增加可执行位，执行：

```bash
bash -n build.sh
npm test
./build.sh
node scripts/validate-package.mjs \
  output/com.ulanzi.commandexecutor.ulanziPlugin
unzip -Z1 output/life_tools_ulanzi_d200x_command_executor.zip
```

预期：

- 测试退出码 0。
- `output/com.ulanzi.commandexecutor.ulanziPlugin/dist/app.js` 存在。
- zip 校验为 `No errors detected`。
- zip 列表不含 `__MACOSX`、`.DS_Store`、`._*`、`node_modules`。

- [ ] 提交：

```bash
git add plugins/unlanzi_d200x/command_executor
git commit -m "build: package Ulanzi command executor"
```

## 8. Task 6：接入 CI 和仓库入口

**Files:**

- Create: `.github/workflows/ulanzi-command-executor.yml`
- Modify: `README.MD`
- Modify: `AGENTS.md`
- Modify: `plugins/unlanzi_d200x/docs/README.md`

- [ ] 新建 macOS CI，paths 至少覆盖：

```yaml
on:
  pull_request:
    paths:
      - ".github/workflows/ulanzi-command-executor.yml"
      - "plugins/unlanzi_d200x/**"
      - "README.MD"
      - "AGENTS.md"
  push:
    branches:
      - master
    paths:
      - ".github/workflows/ulanzi-command-executor.yml"
      - "plugins/unlanzi_d200x/**"
```

- [ ] Job 使用 `macos-latest` 和 Node 20，步骤固定为：

```yaml
- uses: actions/checkout@v4
- uses: actions/setup-node@v4
  with:
    node-version: "20"
    cache: npm
    cache-dependency-path: plugins/unlanzi_d200x/command_executor/package-lock.json
- run: npm ci
  working-directory: plugins/unlanzi_d200x/command_executor
- run: npm test
  working-directory: plugins/unlanzi_d200x/command_executor
- run: bash -n build.sh
  working-directory: plugins/unlanzi_d200x/command_executor
- run: ./build.sh
  working-directory: plugins/unlanzi_d200x/command_executor
- uses: actions/upload-artifact@v4
  with:
    name: life_tools_ulanzi_d200x_command_executor_ci
    path: plugins/unlanzi_d200x/command_executor/output/life_tools_ulanzi_d200x_command_executor.zip
    if-no-files-found: error
```

- [ ] README 工具清单新增一行：

| 分类 | 工具 | 源码目录 | 产物/入口 | 简述 | 详细文档 |
|---|---|---|---|---|---|
| Plugin | Ulanzi D200X Command Executor | `plugins/unlanzi_d200x/command_executor/` | `.ulanziPlugin` | 每个按键独立配置并执行 macOS Shell 命令 | `plugins/unlanzi_d200x/docs/command-executor-user-guide.md` |

- [ ] README“构建与验证”新增可复制命令：

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
./build.sh
```

- [ ] AGENTS 新增插件目录职责、macOS-only 边界、测试/构建命令、实机验证要求，以及 SDK 日志可能记录完整配置的本机调试边界。

- [ ] 更新插件 docs 索引，登记实现计划、开发指南、用户指南、验证报告。

- [ ] 不修改 `.github/workflows/release.yml`。当前交付通过本地 build 和 PR dry-run artifact 生成安装包；tag release 资产命名和长期发布策略没有用户需求，不在本次引入。

- [ ] 检查 YAML 和 diff：

```bash
git diff --check
git status --short
```

- [ ] 提交：

```bash
git add .github/workflows/ulanzi-command-executor.yml README.MD AGENTS.md \
  plugins/unlanzi_d200x/docs/README.md
git commit -m "ci: verify Ulanzi command executor"
```

## 9. Task 7：补齐开发、使用和测试文档

**Files:**

- Create: `plugins/unlanzi_d200x/docs/command-executor-development-guide.md`
- Create: `plugins/unlanzi_d200x/docs/command-executor-installation.md`
- Create: `plugins/unlanzi_d200x/docs/command-executor-user-guide.md`
- Create: `plugins/unlanzi_d200x/docs/command-executor-validation.md`
- Modify: `plugins/unlanzi_d200x/docs/ulanzi-plugin-development-reference.md`

- [ ] 开发指南必须记录：

  - 最终目录和每个文件的职责。
  - 三个 SDK commit、原样 vendoring 约定和完整 payload 日志行为。
  - manifest UUID、D200X/Keypad/macOS 限制。
  - `context` 配置隔离和并发计数。
  - `$SHELL -lc` 会读取登录配置但不会自动读取交互式 `.zshrc`。
  - 显式环境变量覆盖顺序。
  - stdout/stderr 各 16 KiB 截断。
  - `npm ci`、`npm test`、`./build.sh`。
  - 本地安装目录、调试启动方式、日志定位命令。
  - SDK 升级时重新核对 commit、许可证和日志行为的步骤。

- [ ] 安装说明必须记录 macOS 构建、目录包和 zip 安装、升级备份、回滚、验收、日志、调试端口和可恢复卸载。

- [ ] 用户指南必须按“安装 / 拖拽 / 配置 / 示例 / 状态 / 限制 / 排障”组织，至少给出：

```bash
pwd
ls -alh "$HOME"
find . -type f | sort > /private/tmp/files.txt
printf '%s\n' "$HOME" "$USER" "$PATH"
```

环境变量示例：

```text
CUSTOM_ENV=hello
HTTP_PROXY=http://127.0.0.1:7890
```

明确说明：

- 100～200 个参数直接粘贴在命令框，不需要拆成 200 个表单项。
- 插件不提供终端输入，`sudo`、SSH 密码、交互式 REPL 会等待或失败。
- 每次按键都会启动一次；长时间后台任务由用户自己的命令负责。
- 命令以当前 macOS 用户权限执行，粘贴陌生命令等价于在终端执行。

- [ ] 验证报告先建立固定表格，自动化和实机结果未执行前使用“待验证”，不得提前写“通过”：

| 验证项 | 命令/操作 | 证据 | 结果 |
|---|---|---|---|
| Node 单测 | `npm test` | 测试摘要 | 待验证 |
| 构建 | `./build.sh` | 产物路径与 zip 校验 | 待验证 |
| 插件加载 | Studio 重启 | 截图 | 待验证 |
| 两按键隔离 | 分别配置 A/B | 截图与输出 | 待验证 |
| 200 参数 | inline `set --` | `arg-count.txt` | 待验证 |
| 用户环境 | `$HOME/$USER/$PATH` | `environment.txt` | 待验证 |
| 自定义环境/目录 | 面板配置 | `environment.txt` | 待验证 |
| 失败反馈 | `exit 7` | Alert/Toast/日志 | 待验证 |
| 配置持久化 | 重启 Studio | 重启前后截图 | 待验证 |

- [ ] 开发参考增加“官方 SDK 会默认记录完整 WebSocket payload，处理命令或密钥配置的插件必须审查并最小修改日志”的经验，引用当前 vendored 文件和回归测试。

- [ ] 文档若新增 Mermaid，逐个用 `mmdc` 渲染；没有 Mermaid 时执行 Markdown 链接和路径检查。

- [ ] 提交：

```bash
git add plugins/unlanzi_d200x/docs
git commit -m "docs: document Ulanzi command executor workflow"
```

## 10. Task 8：执行自动化验证

**Files:**

- Modify only if a real defect is found: files under `plugins/unlanzi_d200x/command_executor/`
- Modify: `plugins/unlanzi_d200x/docs/command-executor-validation.md`

- [ ] 记录运行环境：

```bash
/usr/local/bin/node --version
/usr/bin/sw_vers
git rev-parse HEAD
```

- [ ] 从干净依赖状态执行：

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
bash -n build.sh
./build.sh
node scripts/validate-package.mjs \
  output/com.ulanzi.commandexecutor.ulanziPlugin
```

- [ ] 单独执行真实 Shell 验证并保留摘要：

```bash
node --test tests/command-runner.test.mjs
node --test tests/command-plugin.test.mjs
```

- [ ] 检查构建包：

```bash
find output/com.ulanzi.commandexecutor.ulanziPlugin -type f | sort
unzip -t output/life_tools_ulanzi_d200x_command_executor.zip
unzip -Z1 output/life_tools_ulanzi_d200x_command_executor.zip |
  rg '(__MACOSX|\\.DS_Store|/\\._|node_modules|\\.map$)'
```

最后一个 `rg` 预期无输出且退出码 1；这是“没有污染文件”，不是测试失败。

- [ ] 将真实测试数、耗时、Node/macOS 版本、产物大小和 commit 写入验证报告。只记录摘要，不粘贴用户环境值。

- [ ] 若修复实现缺陷，先补能复现的测试，再改代码，并使用 `fix:` commit 单独提交。

## 11. Task 9：安装到 Ulanzi Studio 并完成 D200X 实机实验

**Files:**

- Create: `plugins/unlanzi_d200x/docs/assets/command_executor/installed-plugin.jpg`
- Modify: `plugins/unlanzi_d200x/docs/command-executor-validation.md`
- Modify: `plugins/unlanzi_d200x/docs/command-executor-development-guide.md`
- Modify: `plugins/unlanzi_d200x/docs/command-executor-user-guide.md`

- [ ] 使用 `computer-use:computer-use` 控制 Ulanzi Studio。安装前只解析精确目标，不删除插件目录中的其他内容：

```text
源：
plugins/unlanzi_d200x/command_executor/output/
  com.ulanzi.commandexecutor.ulanziPlugin

目标：
~/Library/Application Support/Ulanzi/UlanziDeck/Plugins/
  com.ulanzi.commandexecutor.ulanziPlugin
```

若同名目标已存在，先移动为同目录带时间戳的 `.backup-YYYYmmdd-HHMMSS`；不要删除现有 `__MACOSX` 或其他插件。

- [ ] 完全退出 Ulanzi Studio，复制构建包，再用以下参数启动：

```bash
open /Applications/Ulanzi\ Studio.app --args \
  --log \
  --webRemoteDebug
```

当前 manifest 没有 `Inspect`，对应测试也明确要求该字段不存在，因此本轮不启用 Node inspector。若确实需要 Node inspector，必须先单独增加 manifest `Inspect`、回归测试和 Studio / D200X 实机验证，再补充对应启动与连接说明。

- [ ] 确认 Studio 显示 D200X 已连接；在插件列表找到“命令执行器 / 执行命令”，拖到两个普通按键。

- [ ] 创建专用安全目录：

```bash
mkdir -p /private/tmp/life_tools_ulanzi_command_executor_e2e
```

只允许本轮实机命令写这个目录。

- [ ] 按键 A 配置：

```text
标题：环境验证
工作目录：/private/tmp/life_tools_ulanzi_command_executor_e2e
环境变量：CUSTOM_ENV=from_ulanzi
命令：
printf 'HOME=%s\nUSER=%s\nPWD=%s\nCUSTOM_ENV=%s\nPATH=%s\n' \
  "$HOME" "$USER" "$PWD" "$CUSTOM_ENV" "$PATH" \
  > /private/tmp/life_tools_ulanzi_command_executor_e2e/environment.txt
```

- [ ] 按键 B 依次验证下列命令，每次读取结果文件后再进入下一项：

无参数命令：

```bash
pwd > /private/tmp/life_tools_ulanzi_command_executor_e2e/no-args.txt
```

管道、引号和重定向：

```bash
printf '%s\n' "alpha beta" "gamma" |
  /usr/bin/sort \
  > /private/tmp/life_tools_ulanzi_command_executor_e2e/shell-syntax.txt
```

200 个 inline 参数：

```bash
set -- arg001 arg002 arg003 arg004 arg005 arg006 arg007 arg008 arg009 arg010 \
arg011 arg012 arg013 arg014 arg015 arg016 arg017 arg018 arg019 arg020 \
arg021 arg022 arg023 arg024 arg025 arg026 arg027 arg028 arg029 arg030 \
arg031 arg032 arg033 arg034 arg035 arg036 arg037 arg038 arg039 arg040 \
arg041 arg042 arg043 arg044 arg045 arg046 arg047 arg048 arg049 arg050 \
arg051 arg052 arg053 arg054 arg055 arg056 arg057 arg058 arg059 arg060 \
arg061 arg062 arg063 arg064 arg065 arg066 arg067 arg068 arg069 arg070 \
arg071 arg072 arg073 arg074 arg075 arg076 arg077 arg078 arg079 arg080 \
arg081 arg082 arg083 arg084 arg085 arg086 arg087 arg088 arg089 arg090 \
arg091 arg092 arg093 arg094 arg095 arg096 arg097 arg098 arg099 arg100 \
arg101 arg102 arg103 arg104 arg105 arg106 arg107 arg108 arg109 arg110 \
arg111 arg112 arg113 arg114 arg115 arg116 arg117 arg118 arg119 arg120 \
arg121 arg122 arg123 arg124 arg125 arg126 arg127 arg128 arg129 arg130 \
arg131 arg132 arg133 arg134 arg135 arg136 arg137 arg138 arg139 arg140 \
arg141 arg142 arg143 arg144 arg145 arg146 arg147 arg148 arg149 arg150 \
arg151 arg152 arg153 arg154 arg155 arg156 arg157 arg158 arg159 arg160 \
arg161 arg162 arg163 arg164 arg165 arg166 arg167 arg168 arg169 arg170 \
arg171 arg172 arg173 arg174 arg175 arg176 arg177 arg178 arg179 arg180 \
arg181 arg182 arg183 arg184 arg185 arg186 arg187 arg188 arg189 arg190 \
arg191 arg192 arg193 arg194 arg195 arg196 arg197 arg198 arg199 arg200
printf '%s\n' "$#" \
  > /private/tmp/life_tools_ulanzi_command_executor_e2e/arg-count.txt
```

并发：

```bash
sleep 2
/usr/bin/uuidgen \
  >> /private/tmp/life_tools_ulanzi_command_executor_e2e/concurrent.txt
```

失败反馈：

```bash
exit 7
```

- [ ] 通过实体 D200X 按键触发。若 Studio 的设备预览点击也能产生 `onRun`，额外记录该行为；若预览只负责选中按键，不把它误写成硬件触发证据。

- [ ] 回读并验证：

```bash
wc -l /private/tmp/life_tools_ulanzi_command_executor_e2e/*
cat /private/tmp/life_tools_ulanzi_command_executor_e2e/arg-count.txt
cat /private/tmp/life_tools_ulanzi_command_executor_e2e/no-args.txt
cat /private/tmp/life_tools_ulanzi_command_executor_e2e/shell-syntax.txt
```

环境文件只核对键存在、`PWD` 和 `CUSTOM_ENV` 精确匹配，不在文档或最终消息中复制完整 PATH。

- [ ] 快速连续按两次并发命令，确认运行态持续到两次都结束，`concurrent.txt` 新增两行。

- [ ] 配置两个不同按键后完全退出并重新打开 Studio；重新选择两个按键，确认标题、命令、目录和环境变量都分别恢复。

- [ ] 查找当前实际日志文件，安装前不预设日志目录；确认后把实测路径写入文档：

```bash
find "$HOME/Library/Application Support/Ulanzi/UlanziDeck/logs/com.ulanzi.ulanzistudio.commandexecutor" \
  -type f \
  -name '*.log' \
  -print
```

检查日志包含官方 SDK 的事件 payload、退出码和受限 stdout/stderr，能够定位按键配置与执行问题。日志只留在本机，不把包含命令或环境变量的原文复制进仓库文档。

- [ ] 用 Studio 截图记录：

  1. 完整 Property Inspector。
  2. 两个按键的不同标题和切换后的不同配置。
  3. 运行/成功或失败反馈。

截图前清除或遮挡用户名、设备序列号、真实路径中的个人标识、PATH、令牌和其他插件敏感配置。

- [ ] 把 Studio 版本、设备型号、安装路径、日志路径、每项结果和截图链接写入验证报告；更新开发/用户文档中只有实测后才能确定的细节。

- [ ] 实机发现缺陷时按“复现测试 -> 修复 -> 自动化回归 -> 重新安装 -> 重跑失败用例”闭环，不直接在安装目录手改产物。

- [ ] 提交实机证据：

```bash
git add plugins/unlanzi_d200x/docs
git commit -m "docs: record Ulanzi Studio D200X validation"
```

## 12. Task 10：最终审查与交付

**Files:** Review all changed files only.

- [ ] 执行最终验证：

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
./build.sh
cd ../../..
git diff --check master...HEAD
git status --short
git log --oneline master..HEAD
```

- [ ] 搜索不允许遗留的内容：

```bash
rg -n 'TODO|TBD|FIXME|console\.log\(.*(settings|command|environment)' \
  plugins/unlanzi_d200x \
  --glob '!docs/2026-07-25-command-executor-implementation-plan.md'
rg -n '/Users/[^/[:space:]]+|/home/[^/[:space:]]+|/var/folders/[^[:space:]]+' \
  plugins/unlanzi_d200x README.MD AGENTS.md
git log --format='%an <%ae>' master..HEAD
```

第二条允许命中测试专用的 `/Users/example`；其他真实用户名、用户目录和本机临时目录片段不得进入插件源码、构建包、截图或文档。提交作者信息应使用公开的 GitHub 身份和 `users.noreply.github.com` 地址，不提交个人或公司邮箱。

- [ ] 使用 `superpowers:requesting-code-review` 做一次独立 review，优先检查：

  - Shell 注入是否只来自用户明确输入，环境变量名是否严格验证。
  - 显式环境变量是否在登录配置之后覆盖。
  - 长命令和 200 参数是否没有 JS 层拆分。
  - SDK 完整 payload 日志是否保持上游行为，业务输出日志是否按 16 KiB 上限截断。
  - `context` 隔离、清理和并发完成竞态。
  - 构建包是否自包含且没有 AppleDouble/node_modules。
  - 文档是否与真实 Studio 行为一致。

- [ ] 只修复 review 中有证据的问题；每个修复补测试并重新执行受影响验证。

- [ ] 向用户汇报本地分支、commit 列表、测试结果、构建产物和实机证据。未经用户确认，不推送分支、不创建 GitHub PR。

- [ ] 用户确认后：

```bash
git push -u origin feat/cq/ulanzi_d200x
gh pr create \
  --base master \
  --head feat/cq/ulanzi_d200x \
  --title "feat: add Ulanzi D200X command executor" \
  --body-file /private/tmp/life_tools_ulanzi_d200x_pr.md
```

PR 正文必须包含范围、配置字段、测试/构建结果、Studio/D200X 实机证据、macOS-only 限制和 SDK 固定版本。

## 13. 完成判定

只有同时满足以下条件才可以声称完成：

- `npm test`、`build.sh`、包校验全部通过。
- 安装包不依赖仓库外的 `node_modules`。
- Studio 能显示 Action 和 Property Inspector。
- 两个按键配置隔离，Studio 重启后仍存在。
- 无参数、Shell 语法、200 参数、用户环境和自定义环境/目录有实体回读证据；并发状态有自动化回归。
- `exit 7` 的 Alert、Toast 和日志行为有自动化回归；发布前只有相关代码变化时才要求补实体失败回归。
- 实体 D200X 至少成功触发一次受控临时目录命令。
- SDK 保留完整 payload 诊断日志，业务 stdout/stderr 日志按每个流 16 KiB 截断，文档明确本机落盘边界。
- 安装说明、开发指南、用户指南、验证报告和截图与当前实现一致。
- 推送和 PR 只在用户再次确认后执行。
