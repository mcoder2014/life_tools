import { runCommand } from "./command-runner.js";

const DEFAULT_TITLE = "执行命令";
const SUCCESS_RESTORE_DELAY_MS = 1200;
const SETTINGS_FIELDS = [
  "title",
  "command",
  "workingDirectory",
  "environment"
];

function normalizeField(field, value) {
  if (field === "title") {
    if (typeof value !== "string") {
      return DEFAULT_TITLE;
    }
    return value.trim() || DEFAULT_TITLE;
  }

  return typeof value === "string" ? value : "";
}

function createSettings(param) {
  const settings = {
    title: DEFAULT_TITLE,
    command: "",
    workingDirectory: "",
    environment: ""
  };

  return mergeSettings(settings, param);
}

function mergeSettings(settings, param) {
  if (!param || typeof param !== "object") {
    return settings;
  }

  for (const field of SETTINGS_FIELDS) {
    if (Object.hasOwn(param, field)) {
      settings[field] = normalizeField(field, param[field]);
    }
  }
  return settings;
}

function hasSettings(param) {
  return (
    param &&
    typeof param === "object" &&
    SETTINGS_FIELDS.some((field) => Object.hasOwn(param, field))
  );
}

function errorMessage(error) {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function valueOrNone(value) {
  return value === undefined || value === null || value === ""
    ? "none"
    : String(value);
}

function buildLog(context, result, validationError, success) {
  const stdout = valueOrNone(result?.stdout);
  const stderr = valueOrNone(result?.stderr);
  const lines = [
    success ? "命令执行完成" : "命令执行失败",
    `context: ${context}`,
    `code: ${valueOrNone(result?.code)}`,
    `signal: ${valueOrNone(result?.signal)}`,
    `spawn error: ${valueOrNone(
      result?.spawnError ? errorMessage(result.spawnError) : null
    )}`,
    `validation error: ${valueOrNone(
      validationError ? errorMessage(validationError) : null
    )}`,
    `stdout: ${stdout}`,
    `stderr: ${stderr}`
  ];

  if (result?.stdoutTruncated) {
    lines.push("stdout 输出已截断");
  }
  if (result?.stderrTruncated) {
    lines.push("stderr 输出已截断");
  }
  return lines.join("\n");
}

function failureToast(result, validationError) {
  if (validationError) {
    return `命令执行失败：${errorMessage(validationError)}`;
  }
  if (result?.spawnError) {
    return "命令启动失败";
  }
  if (result?.signal) {
    return `命令被信号 ${result.signal} 终止`;
  }
  if (result?.code !== 0) {
    return `命令执行失败（退出码 ${valueOrNone(result?.code)}）`;
  }
  return "命令执行失败";
}

function isSuccess(result) {
  return !result?.spawnError && result?.code === 0 && !result?.signal;
}

/**
 * Registers the command action callbacks on one official SDK instance.
 * State stays private and is isolated by button context; each run receives a
 * settings snapshot, while SDK feedback is emitted only for a live instance.
 */
export function registerCommandPlugin(api, options = {}) {
  const runCommandFn = options.runCommandFn ?? runCommand;
  const setTimeoutFn = options.setTimeoutFn ?? globalThis.setTimeout;
  const clearTimeoutFn = options.clearTimeoutFn ?? globalThis.clearTimeout;
  const instances = new Map();

  function createInstance(param) {
    return {
      settings: createSettings(param),
      runningCount: 0,
      failedInBurst: false,
      restoreTimer: null
    };
  }

  function clearRestoreTimer(instance) {
    if (instance.restoreTimer === null) {
      return;
    }
    clearTimeoutFn(instance.restoreTimer);
    instance.restoreTimer = null;
  }

  function currentState(instance) {
    if (instance.runningCount > 0) {
      return 1;
    }
    return instance.restoreTimer === null ? 0 : 2;
  }

  function redraw(context, instance) {
    api.setStateIcon(
      context,
      currentState(instance),
      instance.settings.title
    );
  }

  function updateInstance(context, param) {
    let instance = instances.get(context);
    if (!instance) {
      instance = createInstance(param);
      instances.set(context, instance);
      return instance;
    }

    clearRestoreTimer(instance);
    mergeSettings(instance.settings, param);
    return instance;
  }

  function scheduleIdleRestore(context, instance) {
    let timer;
    timer = setTimeoutFn(() => {
      if (
        instances.get(context) !== instance ||
        instance.runningCount !== 0 ||
        instance.restoreTimer !== timer
      ) {
        return;
      }

      instance.restoreTimer = null;
      api.setStateIcon(context, 0, instance.settings.title);
    }, SUCCESS_RESTORE_DELAY_MS);
    instance.restoreTimer = timer;
  }

  async function execute(context, instance, settings) {
    let result;
    let validationError = null;
    let success = false;

    try {
      result = await runCommandFn(settings);
      success = isSuccess(result);
    } catch (error) {
      validationError = error;
    }

    try {
      if (!success) {
        instance.failedInBurst = true;
        if (instances.get(context) === instance) {
          api.showAlert(context);
          api.toast(failureToast(result, validationError));
        }
        api.logMessage(
          buildLog(context, result, validationError, false),
          "error"
        );
      } else {
        api.logMessage(buildLog(context, result, null, true), "info");
      }
    } finally {
      instance.runningCount -= 1;
      if (instances.get(context) !== instance) {
        return;
      }
      if (instance.runningCount > 0) {
        api.setStateIcon(context, 1, instance.settings.title);
        return;
      }
      if (instance.failedInBurst) {
        api.setStateIcon(context, 0, instance.settings.title);
        return;
      }

      api.setStateIcon(context, 2, instance.settings.title);
      scheduleIdleRestore(context, instance);
    }
  }

  api.onAdd((jsn) => {
    const previous = instances.get(jsn.context);
    if (previous) {
      clearRestoreTimer(previous);
    }
    const instance = createInstance(jsn.param);
    instances.set(jsn.context, instance);
    api.setStateIcon(jsn.context, 0, instance.settings.title);
  });

  const updateFromParam = (jsn) => {
    const instance = updateInstance(jsn.context, jsn.param);
    redraw(jsn.context, instance);
  };
  api.onParamFromApp(updateFromParam);
  api.onParamFromPlugin(updateFromParam);

  api.onError((error) => {
    console.error(`[Ulanzi] 连接错误: ${errorMessage(error)}`);
  });

  api.onSetActive((jsn) => {
    if (jsn.active !== true) {
      return;
    }

    let instance = instances.get(jsn.context);
    if (!instance && hasSettings(jsn.param)) {
      instance = createInstance(jsn.param);
      instances.set(jsn.context, instance);
    } else if (instance && hasSettings(jsn.param)) {
      instance = updateInstance(jsn.context, jsn.param);
    }
    if (instance) {
      redraw(jsn.context, instance);
    }
  });

  api.onRun((jsn) => {
    let instance = instances.get(jsn.context);
    if (!instance) {
      instance = createInstance(jsn.param);
      instances.set(jsn.context, instance);
    } else if (hasSettings(jsn.param)) {
      instance = updateInstance(jsn.context, jsn.param);
    }

    clearRestoreTimer(instance);
    if (instance.runningCount === 0) {
      instance.failedInBurst = false;
    }
    instance.runningCount += 1;
    api.setStateIcon(jsn.context, 1, instance.settings.title);

    const snapshot = { ...instance.settings };
    void execute(jsn.context, instance, snapshot).catch(() => {});
  });

  api.onClear((jsn) => {
    if (!Array.isArray(jsn.param)) {
      return;
    }

    for (const item of jsn.param) {
      const context =
        typeof item === "string" ? item : item?.context;
      if (!context) {
        continue;
      }

      const instance = instances.get(context);
      if (!instance) {
        continue;
      }
      clearRestoreTimer(instance);
      instances.delete(context);
    }
  });
}
