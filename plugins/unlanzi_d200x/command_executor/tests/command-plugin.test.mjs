import assert from "node:assert/strict";
import test from "node:test";

import { registerCommandPlugin } from "../com.ulanzi.commandexecutor.ulanziPlugin/plugin/command-plugin.js";
import UlanziApi from "../com.ulanzi.commandexecutor.ulanziPlugin/plugin/vendor/ulanzi-api/ulanziApi.js";

const EVENT_METHODS = {
  add: "onAdd",
  paramFromApp: "onParamFromApp",
  paramFromPlugin: "onParamFromPlugin",
  run: "onRun",
  setActive: "onSetActive",
  clear: "onClear",
  error: "onError"
};

class FakeUlanziApi {
  constructor() {
    this.callbacks = new Map();
    this.states = [];
    this.alerts = [];
    this.toasts = [];
    this.logs = [];
  }

  register(method, callback) {
    assert.equal(typeof callback, "function");
    this.callbacks.set(method, callback);
    return this;
  }

  onAdd(callback) {
    return this.register("onAdd", callback);
  }

  onParamFromApp(callback) {
    return this.register("onParamFromApp", callback);
  }

  onParamFromPlugin(callback) {
    return this.register("onParamFromPlugin", callback);
  }

  onRun(callback) {
    return this.register("onRun", callback);
  }

  onSetActive(callback) {
    return this.register("onSetActive", callback);
  }

  onClear(callback) {
    return this.register("onClear", callback);
  }

  onError(callback) {
    return this.register("onError", callback);
  }

  setStateIcon(context, state, text) {
    this.states.push({ context, state, text });
  }

  showAlert(context) {
    this.alerts.push(context);
  }

  toast(message) {
    this.toasts.push(message);
  }

  logMessage(message, level) {
    this.logs.push({ message, level });
  }

  trigger(event, payload) {
    const method = EVENT_METHODS[event];
    const callback = this.callbacks.get(method);
    assert.ok(callback, `${method} callback was not registered`);
    return callback(payload);
  }
}

class Deferred {
  constructor() {
    this.promise = new Promise((resolve, reject) => {
      this.resolve = resolve;
      this.reject = reject;
    });
  }
}

function createFakeTimers() {
  let nextId = 1;
  const timers = new Map();
  const cleared = [];

  return {
    cleared,
    setTimeoutFn(callback, delay) {
      const token = nextId;
      nextId += 1;
      timers.set(token, { callback, delay });
      return token;
    },
    clearTimeoutFn(token) {
      cleared.push(token);
      timers.delete(token);
    },
    pending() {
      return [...timers.entries()].map(([token, timer]) => ({
        token,
        delay: timer.delay
      }));
    },
    fire(token) {
      const timer = timers.get(token);
      assert.ok(timer, `timer ${token} does not exist`);
      timers.delete(token);
      timer.callback();
    }
  };
}

function successResult(overrides = {}) {
  return {
    code: 0,
    signal: null,
    stdout: "",
    stderr: "",
    stdoutTruncated: false,
    stderrTruncated: false,
    spawnError: null,
    ...overrides
  };
}

async function settleAsyncWork() {
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));
}

function statesFor(api, context) {
  return api.states
    .filter((entry) => entry.context === context)
    .map(({ state, text }) => ({ state, text }));
}

test("registers every required official SDK event", () => {
  const api = new FakeUlanziApi();

  registerCommandPlugin(api, {
    runCommandFn: async () => successResult()
  });

  assert.deepEqual(
    [...api.callbacks.keys()].sort(),
    Object.values(EVENT_METHODS).sort()
  );
});

test("handles the official SDK error event with a local diagnostic", () => {
  const api = new UlanziApi();
  const diagnostics = [];
  const originalConsoleError = console.error;
  console.error = (...parts) => {
    diagnostics.push(parts.map(String).join(" "));
  };

  try {
    registerCommandPlugin(api);

    assert.doesNotThrow(() => {
      api.emit("error", "connection refused");
    });
    assert.deepEqual(diagnostics, [
      "[Ulanzi] 连接错误: connection refused"
    ]);
  } finally {
    console.error = originalConsoleError;
  }
});

test("routes normalized snapshots independently by context", async () => {
  const api = new FakeUlanziApi();
  const calls = [];
  registerCommandPlugin(api, {
    runCommandFn: async (settings) => {
      calls.push(settings);
      return successResult();
    }
  });

  api.trigger("add", {
    context: "context-a",
    param: {
      title: "  第一项  ",
      command: " printf 'a' ",
      workingDirectory: "/tmp/a",
      environment: "A=1"
    }
  });
  api.trigger("add", {
    context: "context-b",
    param: {
      title: "第二项",
      command: "printf 'b'",
      workingDirectory: "/tmp/b",
      environment: "B=2"
    }
  });
  api.trigger("run", { context: "context-a" });
  api.trigger("run", { context: "context-b" });
  await settleAsyncWork();

  assert.deepEqual(calls, [
    {
      title: "第一项",
      command: " printf 'a' ",
      workingDirectory: "/tmp/a",
      environment: "A=1"
    },
    {
      title: "第二项",
      command: "printf 'b'",
      workingDirectory: "/tmp/b",
      environment: "B=2"
    }
  ]);
  assert.deepEqual(statesFor(api, "context-a").slice(0, 2), [
    { state: 0, text: "第一项" },
    { state: 1, text: "第一项" }
  ]);
  assert.deepEqual(statesFor(api, "context-b").slice(0, 2), [
    { state: 0, text: "第二项" },
    { state: 1, text: "第二项" }
  ]);
});

test("plugin updates affect only later runs and explicit empty command clears", async () => {
  const api = new FakeUlanziApi();
  const calls = [];
  registerCommandPlugin(api, {
    runCommandFn: async (settings) => {
      calls.push(settings);
      return successResult();
    }
  });

  api.trigger("add", {
    context: "context-a",
    param: {
      title: "原始",
      command: "old",
      workingDirectory: "/tmp/old",
      environment: "OLD=1"
    }
  });
  api.trigger("paramFromPlugin", {
    context: "context-a",
    param: {
      title: "  ",
      command: "new"
    }
  });
  api.trigger("run", { context: "context-a", param: {} });
  await settleAsyncWork();

  assert.deepEqual(calls[0], {
    title: "执行命令",
    command: "new",
    workingDirectory: "/tmp/old",
    environment: "OLD=1"
  });

  api.trigger("paramFromPlugin", {
    context: "context-a",
    param: { command: "" }
  });
  api.trigger("run", { context: "context-a" });
  await settleAsyncWork();

  assert.equal(calls[1].command, "");
});

test("run can initialize uncached context from params and reports validation throws", async () => {
  const api = new FakeUlanziApi();
  const calls = [];
  registerCommandPlugin(api, {
    runCommandFn: async (settings) => {
      calls.push(settings);
      if (settings.command === "") {
        throw new Error("命令不能为空");
      }
      return successResult();
    }
  });

  api.trigger("run", {
    context: "created-on-run",
    param: {
      title: "  即时执行 ",
      command: "date",
      workingDirectory: null,
      environment: 42
    }
  });
  api.trigger("run", { context: "empty-on-run" });
  await settleAsyncWork();

  assert.deepEqual(calls, [
    {
      title: "即时执行",
      command: "date",
      workingDirectory: "",
      environment: ""
    },
    {
      title: "执行命令",
      command: "",
      workingDirectory: "",
      environment: ""
    }
  ]);
  assert.deepEqual(api.alerts, ["empty-on-run"]);
  assert.match(api.toasts[0], /命令不能为空/);
  assert.equal(api.logs.at(-1).level, "error");
  assert.match(api.logs.at(-1).message, /validation error: 命令不能为空/);
  assert.deepEqual(statesFor(api, "empty-on-run"), [
    { state: 1, text: "执行命令" },
    { state: 0, text: "执行命令" }
  ]);
});

test("a rejected runner is contained and cannot create an unhandled rejection", async () => {
  const api = new FakeUlanziApi();
  const unhandled = [];
  const onUnhandled = (reason) => {
    unhandled.push(reason);
  };
  process.on("unhandledRejection", onUnhandled);

  try {
    registerCommandPlugin(api, {
      runCommandFn: async () => {
        throw new Error("runner rejected");
      }
    });
    api.trigger("add", {
      context: "context-a",
      param: { command: "private-command", environment: "SECRET=value" }
    });

    const callbackResult = api.trigger("run", { context: "context-a" });
    assert.equal(callbackResult, undefined);
    await settleAsyncWork();

    assert.deepEqual(unhandled, []);
    assert.deepEqual(api.alerts, ["context-a"]);
    assert.equal(api.logs[0].level, "error");
    assert.match(api.logs[0].message, /validation error: runner rejected/);
    assert.deepEqual(statesFor(api, "context-a").slice(-2), [
      { state: 1, text: "执行命令" },
      { state: 0, text: "执行命令" }
    ]);
  } finally {
    process.off("unhandledRejection", onUnhandled);
  }
});

test("spawn errors, nonzero exits, and signals are failures with safe feedback", async () => {
  const api = new FakeUlanziApi();
  const results = [
    successResult({ code: null, spawnError: new Error("spawn EACCES") }),
    successResult({ code: 7 }),
    successResult({ code: null, signal: "SIGTERM" })
  ];
  registerCommandPlugin(api, {
    runCommandFn: async () => results.shift()
  });
  api.trigger("add", {
    context: "context-a",
    param: {
      command: "do-not-show-this-command",
      environment: "SECRET=do-not-show-this-value"
    }
  });

  for (let index = 0; index < 3; index += 1) {
    api.trigger("run", { context: "context-a" });
    await settleAsyncWork();
  }

  assert.equal(api.alerts.length, 3);
  assert.equal(api.toasts.length, 3);
  assert.match(api.toasts[0], /启动失败/);
  assert.match(api.toasts[1], /退出码 7/);
  assert.match(api.toasts[2], /SIGTERM/);
  assert.doesNotMatch(
    api.toasts.join("\n"),
    /do-not-show-this-command|do-not-show-this-value/
  );
  assert.match(api.logs[0].message, /spawn error: spawn EACCES/);
  assert.match(api.logs[1].message, /code: 7/);
  assert.match(api.logs[2].message, /signal: SIGTERM/);
  assert.ok(api.logs.every((entry) => entry.level === "error"));
});

test("logs bounded output and restores a successful state after 1200 ms", async () => {
  const api = new FakeUlanziApi();
  const timers = createFakeTimers();
  registerCommandPlugin(api, {
    runCommandFn: async () =>
      successResult({
        stdout: "standard output",
        stderr: "standard error",
        stdoutTruncated: true,
        stderrTruncated: true
      }),
    setTimeoutFn: timers.setTimeoutFn,
    clearTimeoutFn: timers.clearTimeoutFn
  });
  api.trigger("add", {
    context: "context-a",
    param: { title: "成功按钮", command: "run" }
  });

  api.trigger("run", { context: "context-a" });
  await settleAsyncWork();

  assert.deepEqual(statesFor(api, "context-a"), [
    { state: 0, text: "成功按钮" },
    { state: 1, text: "成功按钮" },
    { state: 2, text: "成功按钮" }
  ]);
  assert.deepEqual(timers.pending().map(({ delay }) => delay), [1200]);
  assert.equal(api.logs[0].level, "info");
  assert.match(api.logs[0].message, /context: context-a/);
  assert.match(api.logs[0].message, /code: 0/);
  assert.match(api.logs[0].message, /signal: none/);
  assert.match(api.logs[0].message, /stdout: standard output/);
  assert.match(api.logs[0].message, /stderr: standard error/);
  assert.match(api.logs[0].message, /stdout 输出已截断/);
  assert.match(api.logs[0].message, /stderr 输出已截断/);

  timers.fire(timers.pending()[0].token);
  assert.deepEqual(statesFor(api, "context-a").at(-1), {
    state: 0,
    text: "成功按钮"
  });
});

test("keeps running state until all concurrent runs succeed", async () => {
  const api = new FakeUlanziApi();
  const first = new Deferred();
  const second = new Deferred();
  const pending = [first, second];
  registerCommandPlugin(api, {
    runCommandFn: () => pending.shift().promise
  });
  api.trigger("add", {
    context: "context-a",
    param: { title: "并发", command: "run" }
  });

  api.trigger("run", { context: "context-a" });
  api.trigger("run", { context: "context-a" });
  first.resolve(successResult());
  await settleAsyncWork();

  const statesAfterFirst = statesFor(api, "context-a");
  assert.deepEqual(statesAfterFirst.at(-1), { state: 1, text: "并发" });
  assert.equal(statesAfterFirst.some(({ state }) => state === 2), false);

  second.resolve(successResult());
  await settleAsyncWork();
  assert.deepEqual(statesFor(api, "context-a").at(-1), {
    state: 2,
    text: "并发"
  });
});

test("one concurrent failure prevents a final success state", async () => {
  const api = new FakeUlanziApi();
  const first = new Deferred();
  const second = new Deferred();
  const pending = [first, second];
  registerCommandPlugin(api, {
    runCommandFn: () => pending.shift().promise
  });
  api.trigger("add", {
    context: "context-a",
    param: { title: "并发", command: "run" }
  });

  api.trigger("run", { context: "context-a" });
  api.trigger("run", { context: "context-a" });
  first.resolve(successResult({ code: 2 }));
  second.resolve(successResult());
  await settleAsyncWork();

  const states = statesFor(api, "context-a");
  assert.deepEqual(states.at(-1), { state: 0, text: "并发" });
  assert.equal(states.some(({ state }) => state === 2), false);
});

test("running commands keep their settings snapshot across later edits", async () => {
  const api = new FakeUlanziApi();
  const deferred = new Deferred();
  const calls = [];
  registerCommandPlugin(api, {
    runCommandFn: (settings) => {
      calls.push(settings);
      return deferred.promise;
    }
  });
  api.trigger("add", {
    context: "context-a",
    param: {
      title: "旧标题",
      command: "old",
      workingDirectory: "/tmp/old",
      environment: "OLD=1"
    }
  });

  api.trigger("run", { context: "context-a" });
  api.trigger("paramFromApp", {
    context: "context-a",
    param: {
      title: "新标题",
      command: "new",
      workingDirectory: "/tmp/new",
      environment: "NEW=1"
    }
  });

  assert.deepEqual(calls[0], {
    title: "旧标题",
    command: "old",
    workingDirectory: "/tmp/old",
    environment: "OLD=1"
  });
  deferred.resolve(successResult());
  await settleAsyncWork();

  assert.deepEqual(statesFor(api, "context-a").at(-1), {
    state: 2,
    text: "新标题"
  });
});

test("clear is safe, clears timers, and suppresses late run UI updates", async () => {
  const api = new FakeUlanziApi();
  const timers = createFakeTimers();
  const successfulRun = new Deferred();
  const lateRun = new Deferred();
  const pending = [successfulRun, lateRun];
  registerCommandPlugin(api, {
    runCommandFn: () => pending.shift().promise,
    setTimeoutFn: timers.setTimeoutFn,
    clearTimeoutFn: timers.clearTimeoutFn
  });

  assert.doesNotThrow(() => {
    api.trigger("clear", { param: [{ context: "missing" }] });
  });

  api.trigger("add", {
    context: "context-a",
    param: { command: "first" }
  });
  api.trigger("run", { context: "context-a" });
  successfulRun.resolve(successResult());
  await settleAsyncWork();
  const successTimer = timers.pending()[0].token;

  api.trigger("paramFromApp", {
    context: "context-a",
    param: { command: "second" }
  });
  assert.deepEqual(timers.cleared, [successTimer]);

  api.trigger("run", { context: "context-a" });
  const stateCountBeforeClear = api.states.length;
  api.trigger("clear", { param: [{ context: "context-a" }] });
  lateRun.resolve(successResult({ code: 9 }));
  await settleAsyncWork();

  assert.equal(api.states.length, stateCountBeforeClear);
  assert.deepEqual(api.alerts, []);
  assert.deepEqual(api.toasts, []);
});

test("active redraws the current state and can initialize from settings", () => {
  const api = new FakeUlanziApi();
  registerCommandPlugin(api, {
    runCommandFn: async () => successResult()
  });
  api.trigger("add", {
    context: "context-a",
    param: { title: "已有", command: "old" }
  });

  api.trigger("setActive", { context: "context-a", active: false });
  api.trigger("setActive", { context: "context-a", active: true });
  api.trigger("setActive", {
    context: "context-b",
    active: true,
    param: { title: "  新建  ", command: "new" }
  });

  assert.deepEqual(statesFor(api, "context-a"), [
    { state: 0, text: "已有" },
    { state: 0, text: "已有" }
  ]);
  assert.deepEqual(statesFor(api, "context-b"), [
    { state: 0, text: "新建" }
  ]);
});

test("an empty successful result still writes a completion log", async () => {
  const api = new FakeUlanziApi();
  registerCommandPlugin(api, {
    runCommandFn: async () => successResult()
  });
  api.trigger("add", {
    context: "context-a",
    param: { command: "true" }
  });

  api.trigger("run", { context: "context-a" });
  await settleAsyncWork();

  assert.equal(api.logs.length, 1);
  assert.equal(api.logs[0].level, "info");
  assert.match(api.logs[0].message, /命令执行完成/);
});
