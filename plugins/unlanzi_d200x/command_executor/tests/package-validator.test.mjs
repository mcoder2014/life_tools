import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  cp,
  mkdir,
  mkdtemp,
  readFile,
  realpath,
  rm,
  symlink,
  writeFile
} from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const testDirectory = path.dirname(fileURLToPath(import.meta.url));
const pluginName = "com.ulanzi.commandexecutor.ulanziPlugin";
const commandExecutorRoot = path.resolve(testDirectory, "..");
const sourcePlugin = path.join(commandExecutorRoot, pluginName);
const worktreeRoot = path.resolve(commandExecutorRoot, "../../..");
const validator = path.resolve(
  commandExecutorRoot,
  "scripts",
  "validate-package.mjs"
);

async function createPackage(t) {
  const temporaryRoot = await mkdtemp(
    path.join(os.tmpdir(), "ulanzi-package-validator-")
  );
  const packageDirectory = path.join(temporaryRoot, pluginName);
  await cp(sourcePlugin, packageDirectory, { recursive: true });
  await mkdir(path.join(packageDirectory, "dist"), { recursive: true });
  await writeFile(
    path.join(packageDirectory, "dist", "app.js"),
    "export default {};\n"
  );
  t.after(() => rm(temporaryRoot, { recursive: true, force: true }));
  return packageDirectory;
}

function validate(packageDirectory, options = {}) {
  return spawnSync(process.execPath, [validator, packageDirectory], {
    encoding: "utf8",
    cwd: options.cwd
  });
}

function outputOf(result) {
  return `${result.stdout}\n${result.stderr}`;
}

test("accepts a complete package with the legal plugin source directories", async (t) => {
  const packageDirectory = await createPackage(t);
  const result = validate(packageDirectory);

  assert.equal(result.status, 0, outputOf(result));
  assert.match(result.stdout, /Package validation passed/);
});

test("rejects a package whose dist app is missing", async (t) => {
  const packageDirectory = await createPackage(t);
  await rm(path.join(packageDirectory, "dist", "app.js"));

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(
    outputOf(result),
    /dist\/app\.js must be a non-empty regular file/
  );
});

test("rejects a manifest resource path that escapes the package", async (t) => {
  const packageDirectory = await createPackage(t);
  const manifestPath = path.join(packageDirectory, "manifest.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  manifest.CodePath = "../outside.js";
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(outputOf(result), /manifest resource path escapes package/);
});

test("rejects test files from an otherwise valid package", async (t) => {
  const packageDirectory = await createPackage(t);
  await writeFile(path.join(packageDirectory, "plugin", "debug.test.js"), "");

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(outputOf(result), /forbidden package entry/);
});

test("rejects an external file symlink used as a required runtime file", async (t) => {
  const packageDirectory = await createPackage(t);
  const settingsPath = path.join(
    packageDirectory,
    "property-inspector",
    "settings.js"
  );
  const externalSettings = path.join(
    path.dirname(packageDirectory),
    "external-settings.js"
  );
  await writeFile(externalSettings, await readFile(settingsPath));
  await rm(settingsPath);
  await symlink(externalSettings, settingsPath);

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(outputOf(result), /symbolic links are forbidden/);
});

test("rejects an external directory symlink anywhere in the package", async (t) => {
  const packageDirectory = await createPackage(t);
  const externalDirectory = path.join(
    path.dirname(packageDirectory),
    "external-directory"
  );
  await mkdir(externalDirectory);
  await writeFile(path.join(externalDirectory, "payload.js"), "");
  await symlink(
    externalDirectory,
    path.join(packageDirectory, "plugin", "external-directory")
  );

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(outputOf(result), /symbolic links are forbidden/);
});

test("rejects a package missing a fixed inspector runtime file", async (t) => {
  const packageDirectory = await createPackage(t);
  await rm(
    path.join(packageDirectory, "property-inspector", "settings.js")
  );

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(
    outputOf(result),
    /property-inspector\/settings\.js must be a non-empty regular file/
  );
});

test("rejects extra files in dist instead of hiding stale output", async (t) => {
  const packageDirectory = await createPackage(t);
  await writeFile(
    path.join(packageDirectory, "dist", "old-runtime.js"),
    "export {};\n"
  );

  const result = validate(packageDirectory);

  assert.notEqual(result.status, 0);
  assert.match(outputOf(result), /dist must contain only app\.js/);
});

test("accepts runtime system paths when invoked from /bin", async (t) => {
  const packageDirectory = await createPackage(t);
  const resolvedBin = await realpath("/bin");
  await writeFile(
    path.join(packageDirectory, "dist", "app.js"),
    [
      'const shell = "/bin/zsh";',
      `const executable = ${JSON.stringify(path.join(resolvedBin, "env"))};`,
      ""
    ].join("\n")
  );

  const result = validate(packageDirectory, { cwd: "/bin" });

  assert.equal(result.status, 0, outputOf(result));
});

test("accepts a sibling path that only shares a source-root prefix", async (t) => {
  const packageDirectory = await createPackage(t);
  await writeFile(
    path.join(packageDirectory, "dist", "app.js"),
    `const cache = ${JSON.stringify(`${worktreeRoot}-cache`)};\n`
  );

  const result = validate(packageDirectory);

  assert.equal(result.status, 0, outputOf(result));
});

test("rejects bundles containing real source roots", async (t) => {
  const leakedRoots = [
    ["source plugin", sourcePlugin],
    ["command executor", commandExecutorRoot],
    ["worktree", worktreeRoot]
  ];

  for (const [label, leakedRoot] of leakedRoots) {
    await t.test(label, async (subtest) => {
      const packageDirectory = await createPackage(subtest);
      await writeFile(
        path.join(packageDirectory, "dist", "app.js"),
        `const leakedBuildPath = ${JSON.stringify(leakedRoot)};\n`
      );

      const result = validate(packageDirectory);

      assert.notEqual(result.status, 0);
      assert.match(
        outputOf(result),
        /dist\/app\.js contains absolute build path/
      );
    });
  }
});
