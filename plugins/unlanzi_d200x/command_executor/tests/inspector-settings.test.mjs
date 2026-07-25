import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const testDirectory = path.dirname(fileURLToPath(import.meta.url));
const pluginDirectory = path.resolve(
  testDirectory,
  "..",
  "com.ulanzi.commandexecutor.ulanziPlugin"
);
const inspectorDirectory = path.join(pluginDirectory, "property-inspector");

async function loadSettings() {
  const source = await readFile(
    path.join(inspectorDirectory, "settings.js"),
    "utf8"
  );
  const context = vm.createContext({});
  vm.runInContext(source, context);
  return context.CommandExecutorSettings;
}

function copyFromVm(value) {
  return JSON.parse(JSON.stringify(value));
}

function createInspectorHarness({ language = "zh_CN" } = {}) {
  const listeners = new Map();
  const windowListeners = new Map();
  const timers = [];
  let now = 0;

  class FakeElement {
    constructor() {
      this.listeners = new Map();
      this.textContent = "";
      Object.defineProperty(this, "innerHTML", {
        set() {
          assert.fail("inspector must not write user content through innerHTML");
        }
      });
    }

    addEventListener(name, callback) {
      this.listeners.set(name, callback);
    }

    dispatch(name) {
      const callback = this.listeners.get(name);
      assert.ok(callback, `${name} listener was not registered`);
      callback({ type: name });
    }
  }

  const form = new FakeElement();
  form.values = {
    title: "执行命令",
    command: "",
    workingDirectory: "",
    environment: ""
  };
  const validationMessage = new FakeElement();
  const executionPreview = new FakeElement();
  const wrapper = new FakeElement();
  wrapper.hidden = true;
  wrapper.classList = {
    remove(name) {
      assert.equal(name, "hidden");
      wrapper.hidden = false;
    }
  };

  const elements = new Map([
    ["#property-inspector", form],
    ["#validation-message", validationMessage],
    ["#execution-preview", executionPreview],
    [".udpi-wrapper", wrapper]
  ]);
  const document = {
    querySelector(selector) {
      return elements.get(selector) ?? null;
    }
  };
  const fakeWindow = {
    addEventListener(name, callback) {
      windowListeners.set(name, callback);
    },
    dispatch(name) {
      const callback = windowListeners.get(name);
      assert.ok(callback, `${name} listener was not registered`);
      callback({ type: name });
    }
  };
  const Utils = {
    debounce(callback, wait) {
      let pendingTimer = null;
      return (...args) => {
        if (pendingTimer) {
          pendingTimer.active = false;
        }
        pendingTimer = {
          active: true,
          callback,
          args,
          dueAt: now + wait
        };
        timers.push(pendingTimer);
      };
    },
    getFormValue(target) {
      assert.equal(target, form);
      return { ...form.values };
    },
    setFormValue(settings, target) {
      assert.equal(target, form);
      form.values = { ...settings };
    }
  };
  const api = {
    language,
    sent: [],
    connect(uuid) {
      this.connectedUuid = uuid;
    },
    onConnected(callback) {
      listeners.set("connected", callback);
    },
    onAdd(callback) {
      listeners.set("add", callback);
    },
    onParamFromApp(callback) {
      listeners.set("paramFromApp", callback);
    },
    sendParamFromPlugin(settings) {
      this.sent.push(JSON.parse(JSON.stringify(settings)));
    },
    emit(name, message = {}) {
      const callback = listeners.get(name);
      assert.ok(callback, `${name} callback was not registered`);
      callback(message);
    }
  };

  return {
    api,
    document,
    executionPreview,
    form,
    Utils,
    validationMessage,
    window: fakeWindow,
    advance(milliseconds) {
      now += milliseconds;
      let ranTimer;
      do {
        ranTimer = false;
        for (const timer of timers) {
          if (timer.active && timer.dueAt <= now) {
            timer.active = false;
            timer.callback(...timer.args);
            ranTimer = true;
          }
        }
      } while (ranTimer);
    }
  };
}

async function runInspector(harness) {
  const [settingsSource, inspectorSource] = await Promise.all([
    readFile(path.join(inspectorDirectory, "settings.js"), "utf8"),
    readFile(path.join(inspectorDirectory, "inspector.js"), "utf8")
  ]);
  const context = vm.createContext({
    document: harness.document,
    window: harness.window,
    Utils: harness.Utils,
    $UD: harness.api
  });

  vm.runInContext(settingsSource, context);
  vm.runInContext(inspectorSource, context);
}

test("normalizes defaults and preserves string configuration verbatim", async () => {
  const { DEFAULT_SETTINGS, normalizeSettings } = await loadSettings();

  assert.deepEqual(copyFromVm(DEFAULT_SETTINGS), {
    title: "执行命令",
    command: "",
    workingDirectory: "",
    environment: ""
  });
  assert.deepEqual(
    copyFromVm(
      normalizeSettings({
        title: "  自定义标题  ",
        command: "  printf '%s' value\n",
        workingDirectory: " ~/work ",
        environment: " FOO=value \n"
      })
    ),
    {
      title: "自定义标题",
      command: "  printf '%s' value\n",
      workingDirectory: " ~/work ",
      environment: " FOO=value \n"
    }
  );
  assert.deepEqual(
    copyFromVm(
      normalizeSettings({
        title: " \t ",
        command: 100,
        workingDirectory: null,
        environment: {}
      })
    ),
    {
      title: "执行命令",
      command: "",
      workingDirectory: "",
      environment: ""
    }
  );
});

test("environment validation accepts blank CRLF input, first equals, and duplicates", async () => {
  const { validateEnvironmentText } = await loadSettings();

  assert.equal(validateEnvironmentText(""), "");
  assert.equal(validateEnvironmentText("\r\n \r\n"), "");
  assert.equal(
    validateEnvironmentText(
      " TOKEN =first=value\r\nTOKEN=last=value=kept\r\n_EMPTY=\r\n"
    ),
    ""
  );
});

test("environment validation reports 1-based lines without leaking values", async () => {
  const { validateEnvironmentText } = await loadSettings();
  const invalidLine = validateEnvironmentText(
    "VALID=ok\nmissing-equals-secret\nAFTER=ok"
  );
  const invalidName = validateEnvironmentText(
    "VALID=ok\nBAD-NAME=do-not-leak"
  );

  assert.match(invalidLine, /2/);
  assert.doesNotMatch(invalidLine, /missing-equals-secret/);
  assert.match(invalidName, /2/);
  assert.doesNotMatch(invalidName, /do-not-leak/);
});

test("environment validation rejects NUL without leaking the value", async () => {
  const { validateEnvironmentText } = await loadSettings();
  const message = validateEnvironmentText(
    "VALID=ok\nSECRET=before\0after"
  );

  assert.match(message, /2/);
  assert.match(message, /NUL/);
  assert.doesNotMatch(message, /before|after/);
});

test("validation and preview follow the selected English or Chinese locale", async () => {
  const {
    validateEnvironmentText,
    buildExecutionPreview
  } = await loadSettings();
  const environment = "VALID=ok\nBAD-NAME=do-not-leak";
  const englishError = validateEnvironmentText(environment, "en_US");
  const chineseError = validateEnvironmentText(environment, "zh_CN");
  const englishPreview = buildExecutionPreview({}, "en");
  const chinesePreview = buildExecutionPreview({}, "zh_CN");

  assert.match(englishError, /Environment variable line 2/);
  assert.doesNotMatch(englishError, /do-not-leak/);
  assert.match(chineseError, /环境变量第 2 行/);
  assert.doesNotMatch(chineseError, /do-not-leak/);
  assert.match(
    englishPreview,
    /Working directory: \$HOME[\s\S]*Environment variables: None[\s\S]*Command:\n\(empty\)/
  );
  assert.doesNotMatch(englishPreview, /工作目录|环境变量|命令|（空）/);
  assert.match(
    chinesePreview,
    /工作目录: \$HOME[\s\S]*环境变量: 无[\s\S]*命令:\n（空）/
  );
});

test("execution preview contains names, cwd, and the complete 200-argument command", async () => {
  const { buildExecutionPreview } = await loadSettings();
  const command = [
    "printf",
    ...Array.from({ length: 200 }, (_, index) => `"arg-${index + 1}"`)
  ].join(" ");
  const preview = buildExecutionPreview({
    title: "忽略",
    command,
    workingDirectory: "/Users/example/work",
    environment: "TOKEN=top-secret\nFOO=value=with=equals\nTOKEN=last-secret"
  });

  assert.match(preview, /\$SHELL -lc/);
  assert.match(preview, /\/Users\/example\/work/);
  assert.match(preview, /TOKEN/);
  assert.match(preview, /FOO/);
  assert.doesNotMatch(preview, /top-secret|last-secret|value=with=equals/);
  assert.ok(preview.includes(command));
  assert.ok(preview.endsWith(command));
});

test("execution preview uses safe empty fallbacks", async () => {
  const { buildExecutionPreview } = await loadSettings();
  const preview = buildExecutionPreview({});

  assert.match(preview, /\$HOME/);
  assert.match(preview, /环境变量:\s*无/);
  assert.match(preview, /命令:\s*\n（空）/);
});

test("pagehide flushes the latest complete command before the debounce expires", async () => {
  const harness = createInspectorHarness({ language: "en" });
  await runInspector(harness);
  harness.api.emit("connected");
  const command = [
    "printf",
    ...Array.from({ length: 200 }, (_, index) => `"value-${index + 1}"`)
  ].join(" ");
  harness.form.values = {
    title: "  Latest command  ",
    command,
    workingDirectory: "/Users/example/work",
    environment: "BAD-NAME=do-not-leak"
  };

  harness.form.dispatch("input");

  assert.ok(harness.executionPreview.textContent.includes(command));
  assert.match(
    harness.validationMessage.textContent,
    /Environment variable line 1/
  );
  assert.doesNotMatch(
    harness.validationMessage.textContent,
    /do-not-leak/
  );
  harness.advance(199);
  assert.deepEqual(harness.api.sent, []);

  harness.window.dispatch("pagehide");

  assert.deepEqual(harness.api.sent, [
    {
      title: "Latest command",
      command,
      workingDirectory: "/Users/example/work",
      environment: "BAD-NAME=do-not-leak"
    }
  ]);
  harness.advance(1);
  assert.equal(harness.api.sent.length, 1);
});

test("change flushes immediately without a later duplicate send", async () => {
  const harness = createInspectorHarness();
  await runInspector(harness);
  harness.api.emit("connected");
  harness.form.values = {
    title: "改变",
    command: "printf changed",
    workingDirectory: "",
    environment: ""
  };

  harness.form.dispatch("input");
  harness.form.dispatch("change");

  assert.deepEqual(harness.api.sent, [
    {
      title: "改变",
      command: "printf changed",
      workingDirectory: "",
      environment: ""
    }
  ]);
  harness.advance(200);
  assert.equal(harness.api.sent.length, 1);
});

test("dirty local edits survive host echoes and later host updates are accepted", async () => {
  const harness = createInspectorHarness();
  await runInspector(harness);
  harness.api.emit("add", {
    param: {
      title: "初始",
      command: "printf initial",
      workingDirectory: "/initial",
      environment: "INITIAL=1"
    }
  });
  harness.api.emit("connected");
  assert.equal(harness.form.values.command, "printf initial");

  harness.form.values = {
    title: "  本地最新  ",
    command: "printf local-latest",
    workingDirectory: "/local",
    environment: "LOCAL=latest"
  };
  harness.form.dispatch("input");
  harness.api.emit("paramFromApp", {
    param: {
      title: "宿主旧值",
      command: "printf stale-host",
      workingDirectory: "/host",
      environment: "HOST=stale"
    }
  });

  assert.equal(harness.form.values.command, "printf local-latest");
  assert.ok(
    harness.executionPreview.textContent.includes("printf local-latest")
  );
  harness.advance(200);
  assert.deepEqual(harness.api.sent, [
    {
      title: "本地最新",
      command: "printf local-latest",
      workingDirectory: "/local",
      environment: "LOCAL=latest"
    }
  ]);

  harness.api.emit("paramFromApp", {
    param: {
      title: "宿主新值",
      command: "printf accepted-host",
      workingDirectory: "/accepted",
      environment: "HOST=accepted"
    }
  });
  assert.equal(harness.form.values.command, "printf accepted-host");
  assert.ok(
    harness.executionPreview.textContent.includes("printf accepted-host")
  );
});

test("inspector uses official classic scripts and safe automatic persistence", async () => {
  const [html, source] = await Promise.all([
    readFile(path.join(inspectorDirectory, "inspector.html"), "utf8"),
    readFile(path.join(inspectorDirectory, "inspector.js"), "utf8")
  ]);
  const expectedScripts = [
    "../libs/js/constants.js",
    "../libs/js/eventEmitter.js",
    "../libs/js/timers.js",
    "../libs/js/utils.js",
    "../libs/js/ulanziApi.js",
    "./settings.js",
    "./inspector.js"
  ];
  const actualScripts = [
    ...html.matchAll(/<script\b([^>]*)\bsrc="([^"]+)"[^>]*><\/script>/g)
  ].map((match) => {
    assert.doesNotMatch(match[1], /\btype\s*=\s*["']module["']/i);
    return match[2];
  });

  assert.deepEqual(actualScripts, expectedScripts);
  assert.match(
    html,
    /<div class="udpi-wrapper uspi-wrapper hidden">/
  );
  assert.match(html, /<form\b[^>]*\bid="property-inspector"/);
  assert.match(
    html,
    /<textarea\b[^>]*\bid="command"[^>]*\bname="command"[^>]*\brows="(?:[89]|[1-9]\d+)"/
  );
  assert.match(
    html,
    /<textarea\b[^>]*\bid="environment"[^>]*\bname="environment"[^>]*\brows="(?:[5-9]|[1-9]\d+)"/
  );
  assert.match(
    html,
    /id="validation-message"[^>]*role="status"[^>]*aria-live="polite"/
  );
  assert.match(html, /<pre\b[^>]*\bid="execution-preview"/);
  assert.match(html, /命令（含参数）/);
  assert.match(html, /100[～~-]200|100\s*至\s*200/);
  assert.match(html, /name="command"[^>]*spellcheck="false"/);
  assert.match(html, /name="environment"[^>]*spellcheck="false"/);

  assert.match(source, /const api = \$UD/);
  assert.match(
    source,
    /com\.ulanzi\.ulanzistudio\.commandexecutor\.runcommand/
  );
  assert.match(source, /api\.connect\(ACTION_UUID\)/);
  assert.match(source, /api\.onConnected\(/);
  assert.match(source, /api\.onAdd\(/);
  assert.match(source, /api\.onParamFromApp\(/);
  assert.match(source, /Utils\.debounce\(flushSettings,\s*200\)/);
  assert.match(source, /Utils\.getFormValue\(form\)/);
  assert.match(source, /Utils\.setFormValue\(currentSettings,\s*form\)/);
  assert.match(source, /api\.sendParamFromPlugin\(currentSettings\)/);
  assert.match(source, /validationMessage\.textContent\s*=/);
  assert.match(source, /executionPreview\.textContent\s*=/);
  assert.doesNotMatch(source, /\binnerHTML\b/);
  assert.doesNotMatch(source, /console\.log\s*\(/);
});

test("inspector loads official stylesheet before its own stylesheet", async () => {
  const html = await readFile(
    path.join(inspectorDirectory, "inspector.html"),
    "utf8"
  );
  const stylesheets = [
    ...html.matchAll(/<link\b[^>]*\bhref="([^"]+)"[^>]*>/g)
  ].map((match) => match[1]);

  assert.deepEqual(stylesheets, [
    "../libs/css/uspi.css",
    "./inspector.css"
  ]);
});
