import { spawn as defaultSpawn } from "node:child_process";
import { constants as fileSystemConstants } from "node:fs";
import * as defaultFileSystem from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { StringDecoder } from "node:string_decoder";

const ENVIRONMENT_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const FALLBACK_SHELL = "/bin/zsh";

export const OUTPUT_LIMIT_BYTES = 16 * 1024;

export function parseEnvironment(text = "") {
  const entries = new Map();
  const lines = String(text).split(/\r?\n/);

  lines.forEach((line, index) => {
    if (line.trim() === "") {
      return;
    }

    const separator = line.indexOf("=");
    const name = separator >= 0 ? line.slice(0, separator).trim() : "";
    const value = separator >= 0 ? line.slice(separator + 1) : "";
    if (!ENVIRONMENT_NAME.test(name)) {
      throw new Error(`环境变量第 ${index + 1} 行格式错误`);
    }
    if (value.includes("\0")) {
      throw new Error(`环境变量第 ${index + 1} 行包含不支持的 NUL 字符`);
    }

    entries.set(name, value);
  });

  return Object.fromEntries(entries);
}

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

export async function resolveWorkingDirectory(value, options = {}) {
  const fileSystem = options.fileSystem ?? defaultFileSystem;
  const homeDirectory = options.homeDirectory ?? homedir();
  let workingDirectory;

  if (value === undefined || value === null || value === "" || value === "~") {
    workingDirectory = homeDirectory;
  } else if (typeof value === "string" && value.startsWith("~/")) {
    workingDirectory = path.join(homeDirectory, value.slice(2));
  } else if (typeof value === "string" && path.isAbsolute(value)) {
    workingDirectory = value;
  } else {
    throw new Error("工作目录必须是绝对路径、~ 或 ~/ 开头的路径");
  }

  let status;
  try {
    status = await fileSystem.stat(workingDirectory);
  } catch {
    throw new Error("工作目录不存在或无法读取");
  }
  if (!status.isDirectory()) {
    throw new Error("工作目录不是目录");
  }

  try {
    await fileSystem.access(workingDirectory, fileSystemConstants.X_OK);
  } catch {
    throw new Error("工作目录无法访问");
  }

  return workingDirectory;
}

async function isExecutableFile(fileSystem, value) {
  try {
    const status = await fileSystem.stat(value);
    if (!status.isFile()) {
      return false;
    }
    await fileSystem.access(value, fileSystemConstants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export async function resolveShell(options = {}) {
  const fileSystem = options.fileSystem ?? defaultFileSystem;
  const baseEnvironment = options.baseEnvironment ?? process.env;
  const candidate = baseEnvironment.SHELL;

  if (
    typeof candidate === "string" &&
    path.isAbsolute(candidate) &&
    (await isExecutableFile(fileSystem, candidate))
  ) {
    return candidate;
  }

  if (await isExecutableFile(fileSystem, FALLBACK_SHELL)) {
    return FALLBACK_SHELL;
  }
  throw new Error("没有可执行的 Shell");
}

function appendBounded(chunks, state, chunk, limit) {
  const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
  const remaining = Math.max(0, limit - state.bytes);

  if (remaining > 0) {
    const accepted = buffer.subarray(0, remaining);
    chunks.push(accepted);
    state.bytes += accepted.length;
  }
  if (buffer.length > remaining) {
    state.truncated = true;
  }
}

function createResult({
  code = null,
  signal = null,
  stdoutChunks = [],
  stderrChunks = [],
  stdoutTruncated = false,
  stderrTruncated = false,
  spawnError = null
} = {}) {
  const stdoutDecoder = new StringDecoder("utf8");
  const stderrDecoder = new StringDecoder("utf8");
  const stdoutBuffer = Buffer.concat(stdoutChunks);
  const stderrBuffer = Buffer.concat(stderrChunks);

  return {
    code,
    signal,
    stdout: stdoutTruncated
      ? stdoutDecoder.write(stdoutBuffer)
      : stdoutDecoder.end(stdoutBuffer),
    stderr: stderrTruncated
      ? stderrDecoder.write(stderrBuffer)
      : stderrDecoder.end(stderrBuffer),
    stdoutTruncated,
    stderrTruncated,
    spawnError
  };
}

export async function runCommand(settings = {}, options = {}) {
  const spawnFn = options.spawnFn ?? defaultSpawn;
  const fileSystem = options.fileSystem ?? defaultFileSystem;
  const homeDirectory = options.homeDirectory ?? homedir();
  const baseEnvironment = options.baseEnvironment ?? process.env;
  const outputLimitBytes = options.outputLimitBytes ?? OUTPUT_LIMIT_BYTES;
  const environment = parseEnvironment(settings.environment);
  const script = buildShellScript(settings.command, environment);
  const workingDirectory = await resolveWorkingDirectory(
    settings.workingDirectory,
    { fileSystem, homeDirectory }
  );
  const shell = await resolveShell({ fileSystem, baseEnvironment });
  const stdoutChunks = [];
  const stderrChunks = [];
  const stdoutState = { bytes: 0, truncated: false };
  const stderrState = { bytes: 0, truncated: false };
  let child;

  try {
    child = spawnFn(shell, ["-lc", script], {
      cwd: workingDirectory,
      env: baseEnvironment,
      stdio: ["ignore", "pipe", "pipe"]
    });
  } catch (spawnError) {
    return createResult({ spawnError });
  }

  return new Promise((resolve) => {
    let settled = false;
    const finish = (code, signal, spawnError) => {
      if (settled) {
        return;
      }
      settled = true;
      resolve(
        createResult({
          code,
          signal,
          stdoutChunks,
          stderrChunks,
          stdoutTruncated: stdoutState.truncated,
          stderrTruncated: stderrState.truncated,
          spawnError
        })
      );
    };

    child.stdout.on("data", (chunk) => {
      appendBounded(stdoutChunks, stdoutState, chunk, outputLimitBytes);
    });
    child.stderr.on("data", (chunk) => {
      appendBounded(stderrChunks, stderrState, chunk, outputLimitBytes);
    });
    child.on("error", (error) => {
      finish(null, null, error);
    });
    child.on("close", (code, signal) => {
      finish(code, signal, null);
    });
  });
}
