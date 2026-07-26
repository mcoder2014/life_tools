import {
  lstat,
  readFile,
  readdir,
  realpath
} from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const PLUGIN_NAME = "com.ulanzi.commandexecutor.ulanziPlugin";
const COMMAND_EXECUTOR_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  ".."
);
const SOURCE_PLUGIN_ROOT = path.join(COMMAND_EXECUTOR_ROOT, PLUGIN_NAME);
const WORKTREE_ROOT = path.resolve(COMMAND_EXECUTOR_ROOT, "../../..");
const REQUIRED_VENDOR_FILES = [
  "plugin/vendor/ulanzi-api/constants.js",
  "plugin/vendor/ulanzi-api/ulanziApi.js",
  "plugin/vendor/ulanzi-api/utils.js",
  "libs/assets/u_active.svg",
  "libs/assets/u_active_none.svg",
  "libs/assets/u_check_checkbox.svg",
  "libs/assets/u_check_none.svg",
  "libs/assets/u_check_radio.svg",
  "libs/assets/u_down.svg",
  "libs/assets/u_file.svg",
  "libs/assets/u_folder.svg",
  "libs/assets/u_refresh.svg",
  "libs/assets/u_tip_error.svg",
  "libs/assets/u_tip_info.svg",
  "libs/assets/u_tip_success.svg",
  "libs/assets/u_tip_warn.svg",
  "libs/css/uspi.css",
  "libs/js/constants.js",
  "libs/js/eventEmitter.js",
  "libs/js/timers.js",
  "libs/js/ulanziApi.js",
  "libs/js/utils.js"
];
const REQUIRED_RUNTIME_FILES = [
  "manifest.json",
  "package.json",
  "THIRD_PARTY_NOTICES.md",
  "LICENSES/UlanziDeckPlugin-SDK-APACHE-2.0.txt",
  "plugin/app.js",
  "plugin/command-plugin.js",
  "plugin/command-runner.js",
  "property-inspector/inspector.html",
  "property-inspector/inspector.css",
  "property-inspector/settings.js",
  "property-inspector/inspector.js",
  "en.json",
  "zh_CN.json",
  ...REQUIRED_VENDOR_FILES
];
const SDK_COMMITS = [
  "112bd13a7ff9d45bd68656f7e069fd61851d1812",
  "79de0b0b087546e684afd23f97223f7a7bc392da",
  "550ab80c69285ecf259bd494a7fff767c14f0c0f"
];

function isInside(root, candidate) {
  const relative = path.relative(root, candidate);
  return (
    relative === "" ||
    (relative !== ".." &&
      !relative.startsWith(`..${path.sep}`) &&
      !path.isAbsolute(relative))
  );
}

async function readJson(filePath, label) {
  let source;
  try {
    source = await readFile(filePath, "utf8");
  } catch (error) {
    throw new Error(`${label} is missing or unreadable: ${error.message}`);
  }

  try {
    return JSON.parse(source);
  } catch (error) {
    throw new Error(`${label} is not valid JSON: ${error.message}`);
  }
}

async function requireNonEmptyFile(filePath, label) {
  let fileStat;
  try {
    fileStat = await lstat(filePath);
  } catch {
    throw new Error(`${label} must be a non-empty regular file`);
  }
  if (fileStat.isSymbolicLink()) {
    throw new Error(`symbolic links are forbidden: ${label}`);
  }
  if (!fileStat.isFile() || fileStat.size === 0) {
    throw new Error(`${label} must be a non-empty regular file`);
  }
}

function collectManifestResources(manifest) {
  if (!Array.isArray(manifest.Actions)) {
    throw new Error("manifest Actions must be an array");
  }

  const resources = [
    ["Icon", manifest.Icon],
    ["CategoryIcon", manifest.CategoryIcon],
    ["CodePath", manifest.CodePath]
  ];
  manifest.Actions.forEach((action, actionIndex) => {
    resources.push(
      [`Actions[${actionIndex}].Icon`, action?.Icon],
      [
        `Actions[${actionIndex}].PropertyInspectorPath`,
        action?.PropertyInspectorPath
      ]
    );
    if (!Array.isArray(action?.States)) {
      throw new Error(`manifest Actions[${actionIndex}].States must be an array`);
    }
    action.States.forEach((state, stateIndex) => {
      resources.push([
        `Actions[${actionIndex}].States[${stateIndex}].Image`,
        state?.Image
      ]);
    });
  });
  return resources;
}

async function validateManifestResources(packageDirectory, manifest) {
  for (const [label, relativePath] of collectManifestResources(manifest)) {
    if (
      typeof relativePath !== "string" ||
      relativePath.length === 0 ||
      path.isAbsolute(relativePath) ||
      path.win32.isAbsolute(relativePath)
    ) {
      throw new Error(`manifest resource path escapes package: ${label}`);
    }

    const resourcePath = path.resolve(packageDirectory, relativePath);
    if (!isInside(packageDirectory, resourcePath)) {
      throw new Error(`manifest resource path escapes package: ${label}`);
    }
    await requireNonEmptyFile(
      resourcePath,
      `manifest resource ${relativePath}`
    );
    const realResourcePath = await realpath(resourcePath);
    if (!isInside(packageDirectory, realResourcePath)) {
      throw new Error(`manifest resource path escapes package: ${label}`);
    }
  }
}

function isForbidden(relativePath, entryName, isDirectory) {
  if (
    entryName === ".DS_Store" ||
    entryName.startsWith("._") ||
    entryName === "__MACOSX" ||
    entryName === "node_modules"
  ) {
    return true;
  }
  if (isDirectory && (entryName === "test" || entryName === "tests")) {
    return true;
  }
  return (
    relativePath.endsWith(".map") ||
    /(?:^|[._-])(?:test|spec)\.[^/]+$/i.test(entryName)
  );
}

async function validatePackageEntries(packageDirectory, relativeDirectory = "") {
  const directory = path.join(packageDirectory, relativeDirectory);
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const relativePath = path.join(relativeDirectory, entry.name);
    const entryStat = await lstat(path.join(packageDirectory, relativePath));
    if (entry.isSymbolicLink() || entryStat.isSymbolicLink()) {
      throw new Error(`symbolic links are forbidden: ${relativePath}`);
    }
    if (isForbidden(relativePath, entry.name, entry.isDirectory())) {
      throw new Error(`forbidden package entry: ${relativePath}`);
    }
    if (entry.isDirectory()) {
      await validatePackageEntries(packageDirectory, relativePath);
    }
  }
}

async function validateDist(packageDirectory) {
  const distPath = path.join(packageDirectory, "dist");
  let distStat;
  try {
    distStat = await lstat(distPath);
  } catch {
    throw new Error("dist must contain only app.js as a regular file");
  }
  if (distStat.isSymbolicLink()) {
    throw new Error("symbolic links are forbidden: dist");
  }
  if (!distStat.isDirectory()) {
    throw new Error("dist must contain only app.js as a regular file");
  }

  const entries = await readdir(distPath, { withFileTypes: true });
  if (
    entries.length !== 1 ||
    entries[0].name !== "app.js" ||
    !entries[0].isFile()
  ) {
    throw new Error("dist must contain only app.js as a regular file");
  }
  await requireNonEmptyFile(path.join(distPath, "app.js"), "dist/app.js");
}

function containsPathAtBoundary(source, absolutePath) {
  let searchFrom = 0;
  while (searchFrom < source.length) {
    const index = source.indexOf(absolutePath, searchFrom);
    if (index === -1) {
      return false;
    }
    const nextCharacter = source[index + absolutePath.length];
    if (
      nextCharacter === undefined ||
      /[\/\\\s'"`()\]{},;:?#]/.test(nextCharacter)
    ) {
      return true;
    }
    searchFrom = index + absolutePath.length;
  }
  return false;
}

async function validateBundlePaths(packageDirectory) {
  const bundle = await readFile(
    path.join(packageDirectory, "dist", "app.js"),
    "utf8"
  );
  const forbiddenRoots = new Set([
    SOURCE_PLUGIN_ROOT,
    COMMAND_EXECUTOR_ROOT,
    WORKTREE_ROOT
  ]);
  for (const absolutePath of forbiddenRoots) {
    const pathSegments = path.relative(
      path.parse(absolutePath).root,
      absolutePath
    ).split(path.sep).filter(Boolean);
    if (
      pathSegments.length >= 3 &&
      containsPathAtBoundary(bundle, absolutePath)
    ) {
      throw new Error("dist/app.js contains absolute build path");
    }
  }
}

async function validatePackage(packageArgument) {
  if (!packageArgument) {
    throw new Error(
      `usage: node scripts/validate-package.mjs <${PLUGIN_NAME} directory>`
    );
  }

  const requestedDirectory = path.resolve(packageArgument);
  if (path.basename(requestedDirectory) !== PLUGIN_NAME) {
    throw new Error(`package basename must be ${PLUGIN_NAME}`);
  }
  const packageStat = await lstat(requestedDirectory);
  if (packageStat.isSymbolicLink()) {
    throw new Error("symbolic links are forbidden: package root");
  }
  if (!packageStat.isDirectory()) {
    throw new Error("package path must be a directory");
  }
  const packageDirectory = await realpath(requestedDirectory);

  await validatePackageEntries(packageDirectory);
  for (const relativePath of REQUIRED_RUNTIME_FILES) {
    await requireNonEmptyFile(
      path.join(packageDirectory, relativePath),
      relativePath
    );
  }

  const manifest = await readJson(
    path.join(packageDirectory, "manifest.json"),
    "manifest.json"
  );
  await readJson(path.join(packageDirectory, "package.json"), "package.json");
  await validateManifestResources(packageDirectory, manifest);
  await validateDist(packageDirectory);
  await validateBundlePaths(packageDirectory);

  const noticesPath = path.join(packageDirectory, "THIRD_PARTY_NOTICES.md");
  const notices = await readFile(noticesPath, "utf8");
  for (const commit of SDK_COMMITS) {
    if (!notices.includes(commit)) {
      throw new Error(`THIRD_PARTY_NOTICES.md must declare SDK commit ${commit}`);
    }
  }
}

try {
  await validatePackage(process.argv[2]);
  console.log(`Package validation passed: ${path.resolve(process.argv[2])}`);
} catch (error) {
  console.error(`Package validation failed: ${error.message}`);
  process.exitCode = 1;
}
