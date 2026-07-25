(function () {
  "use strict";

  const ENVIRONMENT_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
  const DEFAULT_SETTINGS = Object.freeze({
    title: "执行命令",
    command: "",
    workingDirectory: "",
    environment: ""
  });
  const TEXT = Object.freeze({
    en: Object.freeze({
      environmentLine: "Environment variable line",
      nul: "contains an unsupported NUL character",
      missingEquals: "is missing an equals sign",
      invalidName: "has an invalid name",
      workingDirectory: "Working directory",
      environment: "Environment variables",
      command: "Command",
      none: "None",
      empty: "(empty)"
    }),
    zh_CN: Object.freeze({
      environmentLine: "环境变量第",
      nul: "行包含不支持的 NUL 字符",
      missingEquals: "行缺少等号",
      invalidName: "行格式错误",
      workingDirectory: "工作目录",
      environment: "环境变量",
      command: "命令",
      none: "无",
      empty: "（空）"
    })
  });

  function getText(locale) {
    return typeof locale === "string" &&
      locale.toLowerCase().startsWith("en")
      ? TEXT.en
      : TEXT.zh_CN;
  }

  function normalizeSettings(value = {}) {
    const settings = value && typeof value === "object" ? value : {};
    const title =
      typeof settings.title === "string" ? settings.title.trim() : "";

    return {
      title: title || DEFAULT_SETTINGS.title,
      command:
        typeof settings.command === "string" ? settings.command : "",
      workingDirectory:
        typeof settings.workingDirectory === "string"
          ? settings.workingDirectory
          : "",
      environment:
        typeof settings.environment === "string" ? settings.environment : ""
    };
  }

  function validateEnvironmentText(text = "", locale = "zh_CN") {
    const lines = String(text).split(/\r?\n/);
    const messages = getText(locale);
    const messageForLine = (line, message) =>
      `${messages.environmentLine} ${line} ${message}`;

    for (let index = 0; index < lines.length; index += 1) {
      const line = lines[index];
      if (line.trim() === "") {
        continue;
      }
      if (line.includes("\0")) {
        return messageForLine(index + 1, messages.nul);
      }

      const separator = line.indexOf("=");
      if (separator < 0) {
        return messageForLine(index + 1, messages.missingEquals);
      }

      const name = line.slice(0, separator).trim();
      if (!ENVIRONMENT_NAME.test(name)) {
        return messageForLine(index + 1, messages.invalidName);
      }
    }

    return "";
  }

  function listEnvironmentNames(text) {
    const names = [];
    const knownNames = new Set();

    String(text).split(/\r?\n/).forEach((line) => {
      const separator = line.indexOf("=");
      const name = separator >= 0 ? line.slice(0, separator).trim() : "";
      if (
        !line.includes("\0") &&
        ENVIRONMENT_NAME.test(name) &&
        !knownNames.has(name)
      ) {
        knownNames.add(name);
        names.push(name);
      }
    });

    return names;
  }

  function buildExecutionPreview(value = {}, locale = "zh_CN") {
    const settings = normalizeSettings(value);
    const environmentNames = listEnvironmentNames(settings.environment);
    const messages = getText(locale);

    return [
      "Shell: $SHELL -lc",
      `${messages.workingDirectory}: ${settings.workingDirectory || "$HOME"}`,
      `${messages.environment}: ${
        environmentNames.length > 0
          ? environmentNames.join(", ")
          : messages.none
      }`,
      `${messages.command}:`,
      settings.command || messages.empty
    ].join("\n");
  }

  globalThis.CommandExecutorSettings = Object.freeze({
    DEFAULT_SETTINGS,
    normalizeSettings,
    validateEnvironmentText,
    buildExecutionPreview
  });
}());
