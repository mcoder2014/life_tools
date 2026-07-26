import assert from "node:assert/strict";
import { access, readFile, stat } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const testDirectory = path.dirname(fileURLToPath(import.meta.url));
const pluginDirectory = path.resolve(
  testDirectory,
  "..",
  "com.ulanzi.commandexecutor.ulanziPlugin"
);

const expectedManifest = {
  Version: "0.1.0",
  Author: "mcoder2014",
  Name: "命令执行器",
  Description: "在当前 macOS 用户的登录 Shell 中执行每个按键独立配置的命令",
  Icon: "assets/icons/plugin.svg",
  Category: "命令执行器",
  CategoryIcon: "assets/icons/plugin.svg",
  CodePath: "dist/app.js",
  Type: "JavaScript",
  SupportedInMultiActions: false,
  UUID: "com.ulanzi.ulanzistudio.commandexecutor",
  Actions: [
    {
      Name: "执行命令",
      Icon: "assets/icons/action.svg",
      PropertyInspectorPath: "property-inspector/inspector.html",
      state: 0,
      States: [
        { Image: "assets/icons/action.svg" },
        { Image: "assets/icons/running.svg" },
        { Image: "assets/icons/success.svg" }
      ],
      Tooltip: "执行配置的 Shell 命令",
      UUID: "com.ulanzi.ulanzistudio.commandexecutor.runcommand",
      Controllers: ["Keypad"],
      Devices: ["D200X"],
      DisableAutomaticStates: true,
      SupportedInMultiActions: false
    }
  ],
  OS: [
    {
      Platform: "mac",
      MinimumVersion: "10.15"
    }
  ],
  Software: {
    MinVersion: "3.0.11"
  }
};

async function readJson(relativePath) {
  return JSON.parse(
    await readFile(path.join(pluginDirectory, relativePath), "utf8")
  );
}

function resolvePluginPath(relativePath) {
  assert.equal(typeof relativePath, "string");
  assert.ok(relativePath.length > 0);
  assert.equal(path.isAbsolute(relativePath), false);

  const resolved = path.resolve(pluginDirectory, relativePath);
  const relative = path.relative(pluginDirectory, resolved);
  assert.ok(
    relative !== ".." &&
      !relative.startsWith(`..${path.sep}`) &&
      !path.isAbsolute(relative),
    `${relativePath} must stay inside the plugin directory`
  );
  return resolved;
}

test("manifest has the exact command executor contract", async () => {
  const manifest = await readJson("manifest.json");

  assert.deepEqual(manifest, expectedManifest);
  assert.equal(manifest.UUID.split(".").length, 4);
  assert.ok(manifest.Actions[0].UUID.split(".").length >= 5);
  assert.equal(Object.hasOwn(manifest, "Inspect"), false);
  assert.equal(Object.hasOwn(manifest, "PrivateAPI"), false);
});

test("manifest resource paths exist and cannot escape the plugin directory", async () => {
  const manifest = await readJson("manifest.json");
  const action = manifest.Actions[0];
  const requiredPaths = [
    manifest.Icon,
    manifest.CategoryIcon,
    action.Icon,
    action.PropertyInspectorPath,
    ...action.States.map((state) => state.Image)
  ];

  assert.equal(manifest.CodePath, "dist/app.js");
  resolvePluginPath(manifest.CodePath);

  for (const relativePath of requiredPaths) {
    await access(resolvePluginPath(relativePath));
  }
});

test("generated bundle is non-empty and does not expose worktree paths when present", async () => {
  const bundlePath = path.join(pluginDirectory, "dist", "app.js");
  let bundleStat;
  try {
    bundleStat = await stat(bundlePath);
  } catch (error) {
    if (error?.code === "ENOENT") {
      return;
    }
    throw error;
  }

  assert.equal(bundleStat.isFile(), true);
  assert.ok(bundleStat.size > 0, "dist/app.js must not be empty");

  const bundle = await readFile(bundlePath, "utf8");
  const worktreeRoot = path.resolve(testDirectory, "../../../..");
  assert.equal(
    bundle.includes(worktreeRoot),
    false,
    "dist/app.js must not contain an absolute worktree path"
  );
});

test("runtime package is an ES module and localization files are usable", async () => {
  const packageJson = await readJson("package.json");
  assert.equal(packageJson.type, "module");

  const inspectorHtml = await readFile(
    path.join(pluginDirectory, "property-inspector", "inspector.html"),
    "utf8"
  );
  const localizationKeys = [
    ...inspectorHtml.matchAll(/data-localize="([^"]+)"/g)
  ].map((match) => match[1]);
  assert.ok(localizationKeys.length > 0);

  for (const locale of ["en.json", "zh_CN.json"]) {
    const translation = await readJson(locale);
    assert.equal(
      typeof translation.Localization,
      "object",
      `${locale} must contain a Localization object`
    );
    assert.notEqual(translation.Localization, null);
    for (const key of localizationKeys) {
      assert.equal(
        typeof translation.Localization[key],
        "string",
        `${locale} must localize ${key}`
      );
      assert.ok(translation.Localization[key].length > 0);
    }
  }
});

test("original SVG resources use the required 144 square viewBox", async () => {
  for (const icon of ["plugin.svg", "action.svg", "running.svg", "success.svg"]) {
    const source = await readFile(
      path.join(pluginDirectory, "assets", "icons", icon),
      "utf8"
    );
    assert.ok(source.trim().length > 0, `${icon} must not be empty`);
    assert.match(source, /<svg\b[^>]*\bviewBox="0 0 144 144"/);
  }
});
