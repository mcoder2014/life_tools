import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { constants as fileSystemConstants } from "node:fs";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { PassThrough } from "node:stream";
import test from "node:test";

import {
  OUTPUT_LIMIT_BYTES,
  buildShellScript,
  parseEnvironment,
  quoteShellValue,
  runCommand,
  resolveShell,
  resolveWorkingDirectory
} from "../com.ulanzi.commandexecutor.ulanziPlugin/plugin/command-runner.js";

test("exports a 16 KiB default output limit", () => {
  assert.equal(OUTPUT_LIMIT_BYTES, 16 * 1024);
});

test("parseEnvironment accepts empty input and ignores empty CRLF lines", () => {
  assert.deepEqual(parseEnvironment(), {});
  assert.deepEqual(parseEnvironment("\r\n \r\n"), {});
  assert.deepEqual(parseEnvironment("FIRST=one\r\n\r\nSECOND=two\r\n"), {
    FIRST: "one",
    SECOND: "two"
  });
});

test("parseEnvironment splits on the first equals and lets the last name win", () => {
  assert.deepEqual(
    parseEnvironment(" TOKEN =first=value\nTOKEN=last=value=kept"),
    { TOKEN: "last=value=kept" }
  );
});

test("parseEnvironment preserves __proto__ as an own enumerable variable", () => {
  const parsed = parseEnvironment("__proto__=first\n__proto__=explicit");

  assert.equal(Object.hasOwn(parsed, "__proto__"), true);
  assert.deepEqual(Object.keys(parsed), ["__proto__"]);
  assert.equal(parsed.__proto__, "explicit");
  assert.equal(
    buildShellScript("printf '%s' \"$__proto__\"", parsed),
    "export __proto__='explicit'\nprintf '%s' \"$__proto__\""
  );
});

test("parseEnvironment reports an invalid name by line without leaking its value", () => {
  assert.throws(
    () => parseEnvironment("VALID=ok\nBAD-NAME=do-not-leak"),
    (error) => {
      assert.match(error.message, /2/);
      assert.doesNotMatch(error.message, /do-not-leak/);
      return true;
    }
  );
});

test("parseEnvironment reports a NUL value by line without leaking its value", () => {
  assert.throws(
    () => parseEnvironment("VALID=ok\nSECRET=before\0after"),
    (error) => {
      assert.match(error.message, /2/);
      assert.doesNotMatch(error.message, /before|after/);
      return true;
    }
  );
});

test("quoteShellValue preserves single quotes and empty values", () => {
  assert.equal(quoteShellValue("a'b"), "'a'\"'\"'b'");
  assert.equal(quoteShellValue(""), "''");
});

test("buildShellScript exports configured values before the original command", () => {
  const command = "printf '%s' \"$VALUE\"";

  assert.equal(
    buildShellScript(command, { VALUE: "a'b", EMPTY: "" }),
    `export VALUE='a'"'"'b'\nexport EMPTY=''\n${command}`
  );
  assert.equal(buildShellScript(command, {}), command);
});

test("buildShellScript rejects blank and NUL-containing commands", () => {
  assert.throws(() => buildShellScript(" \n", {}), /命令不能为空/);
  assert.throws(() => buildShellScript("printf x\0ignored", {}), /NUL/);
});

function createFileSystem({
  directories = [],
  files,
  executablePaths = []
} = {}) {
  const statCalls = [];
  const accessCalls = [];
  const regularFiles =
    files ?? executablePaths.filter((path) => !directories.includes(path));

  return {
    statCalls,
    accessCalls,
    async stat(path) {
      statCalls.push(path);
      if (!directories.includes(path) && !regularFiles.includes(path)) {
        throw new Error("path does not exist");
      }
      return {
        isDirectory() {
          return directories.includes(path);
        },
        isFile() {
          return regularFiles.includes(path);
        }
      };
    },
    async access(path, mode) {
      accessCalls.push([path, mode]);
      if (!executablePaths.includes(path)) {
        throw new Error("path is not executable");
      }
    }
  };
}

test("resolveWorkingDirectory expands home forms and accepts absolute paths", async () => {
  const homeDirectory = "/Users/example";
  const fileSystem = createFileSystem({
    directories: [homeDirectory, "/Users/example/work", "/private/tmp/work"],
    executablePaths: [homeDirectory, "/Users/example/work", "/private/tmp/work"]
  });

  assert.equal(
    await resolveWorkingDirectory("", { fileSystem, homeDirectory }),
    homeDirectory
  );
  assert.equal(
    await resolveWorkingDirectory("~", { fileSystem, homeDirectory }),
    homeDirectory
  );
  assert.equal(
    await resolveWorkingDirectory("~/work", { fileSystem, homeDirectory }),
    "/Users/example/work"
  );
  assert.equal(
    await resolveWorkingDirectory("/private/tmp/work", {
      fileSystem,
      homeDirectory
    }),
    "/private/tmp/work"
  );
  assert.deepEqual(
    fileSystem.accessCalls.map(([, mode]) => mode),
    Array(4).fill(fileSystemConstants.X_OK)
  );
});

test("resolveWorkingDirectory rejects relative and named-home paths before stat", async () => {
  const fileSystem = createFileSystem();
  const options = { fileSystem, homeDirectory: "/Users/example" };

  await assert.rejects(resolveWorkingDirectory("relative/path", options), /绝对路径/);
  await assert.rejects(resolveWorkingDirectory("~other/work", options), /绝对路径/);
  assert.deepEqual(fileSystem.statCalls, []);
});

test("resolveWorkingDirectory rejects non-directories and inaccessible directories", async () => {
  const notDirectory = {
    async stat() {
      return {
        isDirectory() {
          return false;
        }
      };
    },
    async access() {
      assert.fail("access must not run for a non-directory");
    }
  };
  const inaccessible = createFileSystem({ directories: ["/private/tmp/locked"] });

  await assert.rejects(
    resolveWorkingDirectory("/private/tmp/file", {
      fileSystem: notDirectory,
      homeDirectory: "/Users/example"
    }),
    /目录/
  );
  await assert.rejects(
    resolveWorkingDirectory("/private/tmp/locked", {
      fileSystem: inaccessible,
      homeDirectory: "/Users/example"
    }),
    /访问/
  );
});

test("resolveShell uses an executable absolute SHELL candidate", async () => {
  const fileSystem = createFileSystem({
    executablePaths: ["/opt/homebrew/bin/zsh"]
  });

  assert.equal(
    await resolveShell({
      fileSystem,
      baseEnvironment: { SHELL: "/opt/homebrew/bin/zsh" }
    }),
    "/opt/homebrew/bin/zsh"
  );
  assert.deepEqual(fileSystem.accessCalls, [
    ["/opt/homebrew/bin/zsh", fileSystemConstants.X_OK]
  ]);
});

test("resolveShell falls back for missing, relative, or inaccessible candidates", async () => {
  for (const shell of [undefined, "bin/zsh", "/missing/shell"]) {
    const fileSystem = createFileSystem({ executablePaths: ["/bin/zsh"] });

    assert.equal(
      await resolveShell({
        fileSystem,
        baseEnvironment: shell === undefined ? {} : { SHELL: shell }
      }),
      "/bin/zsh"
    );
  }
});

test(
  "resolveShell rejects an executable directory candidate and falls back",
  { skip: process.platform !== "darwin" },
  async (t) => {
    const temporaryDirectory = await mkdtemp(
      path.join(tmpdir(), "command-runner-shell-dir-")
    );
    t.after(async () => {
      await rm(temporaryDirectory, { recursive: true, force: true });
    });

    assert.equal(
      await resolveShell({
        baseEnvironment: { SHELL: temporaryDirectory }
      }),
      "/bin/zsh"
    );
  }
);

test("runCommand rejects a directory fallback before spawning", async () => {
  let spawnCalls = 0;
  const fileSystem = createFileSystem({
    directories: ["/Users/example", "/bin/zsh"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });

  await assert.rejects(
    runCommand(
      { command: "printf ok", workingDirectory: "", environment: "" },
      {
        spawnFn() {
          spawnCalls += 1;
          return createChildProcess();
        },
        fileSystem,
        homeDirectory: "/Users/example",
        baseEnvironment: { SHELL: "relative/zsh" }
      }
    ),
    /Shell/
  );
  assert.equal(spawnCalls, 0);
});

test("resolveShell fails when the fallback is not executable", async () => {
  const fileSystem = createFileSystem();

  await assert.rejects(
    resolveShell({
      fileSystem,
      baseEnvironment: { SHELL: "/missing/shell" }
    }),
    /Shell/
  );
});

function createChildProcess({
  stdoutChunks = [],
  stderrChunks = [],
  code = 0,
  signal = null,
  spawnError = null,
  closeAfterError = false
} = {}) {
  const child = new EventEmitter();
  child.stdout = new PassThrough();
  child.stderr = new PassThrough();

  queueMicrotask(() => {
    for (const chunk of stdoutChunks) {
      child.stdout.write(chunk);
    }
    for (const chunk of stderrChunks) {
      child.stderr.write(chunk);
    }
    child.stdout.end();
    child.stderr.end();

    if (spawnError) {
      child.emit("error", spawnError);
      if (closeAfterError) {
        child.emit("close", code, signal);
      }
      return;
    }
    child.emit("close", code, signal);
  });

  return child;
}

test("runCommand passes the complete script and exact spawn options", async () => {
  const baseEnvironment = {
    SHELL: "/custom/zsh",
    INHERITED: "kept"
  };
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/custom/zsh"]
  });
  const calls = [];
  const spawnFn = (...args) => {
    calls.push(args);
    return createChildProcess({
      stdoutChunks: ["done"],
      stderrChunks: ["warning"],
      code: null,
      signal: "SIGTERM"
    });
  };

  const result = await runCommand(
    {
      command: "printf '%s' \"$VALUE\" | cat",
      workingDirectory: "",
      environment: "VALUE=a'b"
    },
    {
      spawnFn,
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment
    }
  );

  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0], [
    "/custom/zsh",
    [
      "-lc",
      `export VALUE='a'"'"'b'\nprintf '%s' "$VALUE" | cat`
    ],
    {
      cwd: "/Users/example",
      env: baseEnvironment,
      stdio: ["ignore", "pipe", "pipe"]
    }
  ]);
  assert.deepEqual(result, {
    code: null,
    signal: "SIGTERM",
    stdout: "done",
    stderr: "warning",
    stdoutTruncated: false,
    stderrTruncated: false,
    spawnError: null
  });
});

test("runCommand drains streams, truncates each by bytes, and decodes after collection", async () => {
  const multiByte = Buffer.from("你");
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  const spawnFn = () =>
    createChildProcess({
      stdoutChunks: [
        multiByte.subarray(0, 1),
        multiByte.subarray(1),
        Buffer.from("abcdef")
      ],
      stderrChunks: [Buffer.from("123"), Buffer.from("456")]
    });

  const result = await runCommand(
    { command: "ignored by fake", workingDirectory: "", environment: "" },
    {
      spawnFn,
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment: {},
      outputLimitBytes: 5
    }
  );

  assert.equal(Buffer.byteLength(result.stdout), 5);
  assert.equal(result.stdout, "你ab");
  assert.equal(result.stderr, "12345");
  assert.equal(result.stdoutTruncated, true);
  assert.equal(result.stderrTruncated, true);
});

test("runCommand drops an incomplete UTF-8 code point at the byte limit", async () => {
  const outputLimitBytes = 16 * 1024;
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  const result = await runCommand(
    { command: "ignored by fake", workingDirectory: "", environment: "" },
    {
      spawnFn: () =>
        createChildProcess({
          stdoutChunks: [
            Buffer.alloc(outputLimitBytes - 1, "a"),
            Buffer.from("你")
          ]
        }),
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment: {},
      outputLimitBytes
    }
  );

  assert.equal(result.stdoutTruncated, true);
  assert.doesNotMatch(result.stdout, /\uFFFD/);
  assert.equal(result.stdout, "a".repeat(outputLimitBytes - 1));
  assert.ok(Buffer.byteLength(result.stdout) <= outputLimitBytes);
});

test("runCommand replaces incomplete UTF-8 when output was not truncated", async () => {
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  const result = await runCommand(
    { command: "ignored by fake", workingDirectory: "", environment: "" },
    {
      spawnFn: () =>
        createChildProcess({
          stdoutChunks: [Buffer.from([0xe4])]
        }),
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment: {},
      outputLimitBytes: 16
    }
  );

  assert.equal(result.stdoutTruncated, false);
  assert.equal(result.stdout, "\uFFFD");
});

test("runCommand keeps multibyte UTF-8 intact across stream chunks", async () => {
  const text = Buffer.from("命令");
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });

  const result = await runCommand(
    { command: "ignored by fake", workingDirectory: "", environment: "" },
    {
      spawnFn: () =>
        createChildProcess({
          stdoutChunks: [
            text.subarray(0, 1),
            text.subarray(1, 4),
            text.subarray(4)
          ]
        }),
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment: {}
    }
  );

  assert.equal(result.stdout, "命令");
  assert.equal(result.stdoutTruncated, false);
});

test("runCommand returns synchronous and asynchronous spawn failures", async () => {
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  const synchronousError = new Error("synchronous spawn failure");
  const asynchronousError = new Error("asynchronous spawn failure");
  const settings = {
    command: "ignored by fake",
    workingDirectory: "",
    environment: ""
  };
  const options = {
    fileSystem,
    homeDirectory: "/Users/example",
    baseEnvironment: {}
  };

  const synchronousResult = await runCommand(settings, {
    ...options,
    spawnFn() {
      throw synchronousError;
    }
  });
  const asynchronousResult = await runCommand(settings, {
    ...options,
    spawnFn() {
      return createChildProcess({
        spawnError: asynchronousError,
        closeAfterError: true,
        code: 127
      });
    }
  });

  assert.equal(synchronousResult.spawnError, synchronousError);
  assert.equal(synchronousResult.code, null);
  assert.equal(asynchronousResult.spawnError, asynchronousError);
  assert.equal(asynchronousResult.code, null);
});

test("runCommand completes only once when close follows an asynchronous error", async () => {
  const fileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  let thenCalls = 0;

  const result = await runCommand(
    { command: "ignored by fake", workingDirectory: "", environment: "" },
    {
      spawnFn: () =>
        createChildProcess({
          spawnError: new Error("spawn failed"),
          closeAfterError: true,
          code: 127
        }),
      fileSystem,
      homeDirectory: "/Users/example",
      baseEnvironment: {}
    }
  ).then((value) => {
    thenCalls += 1;
    return value;
  });

  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(thenCalls, 1);
  assert.equal(result.spawnError.message, "spawn failed");
});

test("runCommand rejects validation failures before spawning", async () => {
  let spawnCalls = 0;
  const spawnFn = () => {
    spawnCalls += 1;
    return createChildProcess();
  };
  const validFileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example", "/bin/zsh"]
  });
  const commonOptions = {
    spawnFn,
    fileSystem: validFileSystem,
    homeDirectory: "/Users/example",
    baseEnvironment: {}
  };

  await assert.rejects(
    runCommand(
      { command: " ", workingDirectory: "", environment: "" },
      commonOptions
    ),
    /命令不能为空/
  );
  await assert.rejects(
    runCommand(
      {
        command: "printf ok",
        workingDirectory: "",
        environment: "BAD-NAME=secret"
      },
      commonOptions
    ),
    /1/
  );
  await assert.rejects(
    runCommand(
      {
        command: "printf ok",
        workingDirectory: "relative/path",
        environment: ""
      },
      commonOptions
    ),
    /绝对路径/
  );

  const unusableFileSystem = createFileSystem({
    directories: ["/Users/example"],
    executablePaths: ["/Users/example"]
  });
  await assert.rejects(
    runCommand(
      { command: "printf ok", workingDirectory: "", environment: "" },
      { ...commonOptions, fileSystem: unusableFileSystem }
    ),
    /Shell/
  );
  assert.equal(spawnCalls, 0);
});

async function createIsolatedZshEnvironment(t, profile = "") {
  const temporaryDirectory = await mkdtemp(
    path.join(tmpdir(), "command-runner-zdotdir-")
  );
  t.after(async () => {
    await rm(temporaryDirectory, { recursive: true, force: true });
  });
  await writeFile(path.join(temporaryDirectory, ".zprofile"), profile);

  return {
    temporaryDirectory,
    baseEnvironment: {
      ...process.env,
      SHELL: "/bin/zsh",
      ZDOTDIR: temporaryDirectory
    }
  };
}

test(
  "configured environment overrides login profile and inherited values",
  { skip: process.platform !== "darwin" },
  async (t) => {
    const { temporaryDirectory, baseEnvironment } =
      await createIsolatedZshEnvironment(
        t,
        "export COMMAND_EXECUTOR_TEST_VALUE='profile'\n"
      );
    const result = await runCommand(
      {
        command: "printf '%s' \"$COMMAND_EXECUTOR_TEST_VALUE\"",
        workingDirectory: temporaryDirectory,
        environment: "COMMAND_EXECUTOR_TEST_VALUE=explicit"
      },
      {
        baseEnvironment: {
          ...baseEnvironment,
          COMMAND_EXECUTOR_TEST_VALUE: "inherited"
        }
      }
    );

    assert.equal(result.code, 0);
    assert.equal(result.signal, null);
    assert.equal(result.stdout, "explicit");
    assert.equal(result.stderr, "");
    assert.equal(result.spawnError, null);
  }
);

test(
  "exports a configured __proto__ variable to the real shell",
  { skip: process.platform !== "darwin" },
  async (t) => {
    const { temporaryDirectory, baseEnvironment } =
      await createIsolatedZshEnvironment(t);
    const result = await runCommand(
      {
        command: "printf '%s' \"$__proto__\"",
        workingDirectory: temporaryDirectory,
        environment: "__proto__=first\n__proto__=explicit"
      },
      { baseEnvironment }
    );

    assert.equal(result.code, 0);
    assert.equal(result.stdout, "explicit");
    assert.equal(result.stderr, "");
  }
);

test(
  "preserves complete pipeline and quote semantics in the login shell",
  { skip: process.platform !== "darwin" },
  async (t) => {
    const { baseEnvironment } = await createIsolatedZshEnvironment(t);
    const result = await runCommand(
      {
        command:
          `printf '%s\\n' "first value" "a'b" | ` +
          `awk 'NR == 2 { printf "%s", $0 }'`,
        workingDirectory: "",
        environment: ""
      },
      { baseEnvironment }
    );

    assert.equal(result.code, 0);
    assert.equal(result.stdout, "a'b");
    assert.equal(result.stderr, "");
  }
);

test(
  "passes 200 inline shell arguments without parsing them in JavaScript",
  { skip: process.platform !== "darwin" },
  async (t) => {
    const { baseEnvironment } = await createIsolatedZshEnvironment(t);
    const args = Array.from(
      { length: 200 },
      (_, index) => `arg${String(index + 1).padStart(3, "0")}`
    );
    const result = await runCommand(
      {
        command: `set -- ${args.join(" ")}; printf '%s' "$#"`,
        workingDirectory: "",
        environment: ""
      },
      { baseEnvironment }
    );

    assert.equal(result.code, 0);
    assert.equal(result.stdout, "200");
    assert.equal(result.stderr, "");
  }
);
