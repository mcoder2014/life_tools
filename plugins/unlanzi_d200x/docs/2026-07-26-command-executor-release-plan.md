# Command Executor Release Asset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every future `v*` GitHub Release contain a versioned, checksummed Ulanzi D200X command executor installation zip.

**Architecture:** Keep the existing Ubuntu release builder and add one macOS Ulanzi builder in the same workflow. The existing publish job remains the only Release writer; it downloads both artifacts, regenerates `checksums.txt`, and uploads the combined set.

**Tech Stack:** GitHub Actions YAML, Bash, Node.js 20, npm, macOS `ditto`, Python standard-library contract test.

---

### Task 1: Add a failing Release contract test

**Files:**
- Create: `tests/release_workflow_test.py`

- [ ] **Step 1: Write the failing test**

Create a standard-library Python test that reads `.github/workflows/release.yml` and asserts:

```python
required_fragments = [
    "ulanzi-release:",
    "runs-on: macos-latest",
    "name: Build Ulanzi release asset",
    "plugins/unlanzi_d200x/command_executor",
    "life_tools_ulanzi_d200x_command_executor_${tag}.zip",
    "name: ulanzi-release-asset",
    "needs: [release, ulanzi-release]",
    "sha256sum *.zip *.alfredworkflow > checksums.txt",
]
```

Also assert that the Ulanzi artifact download occurs before final checksum generation and before `gh release create/upload`.

- [ ] **Step 2: Verify RED**

Run:

```bash
python3 -m unittest tests/release_workflow_test.py -v
```

Expected: failure reporting the missing `ulanzi-release` job.

### Task 2: Add the macOS Ulanzi build job

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Add `ulanzi-release`**

The job must:

```yaml
ulanzi-release:
  name: Build Ulanzi release asset
  runs-on: macos-latest
```

Use `actions/setup-node@v4` with Node.js 20 and the command executor lockfile cache. Run `npm ci`, `npm test`, `bash -n build.sh`, and `./build.sh` in `plugins/unlanzi_d200x/command_executor`.

- [ ] **Step 2: Create the versioned asset**

Resolve the tag exactly like the existing release job:

```bash
tag="${GITHUB_REF_NAME}"
if [ "${GITHUB_REF_TYPE:-}" != "tag" ]; then
  tag="v0.0.0-ci"
fi
```

Copy the validated build output to:

```text
$RUNNER_TEMP/ulanzi-release/life_tools_ulanzi_d200x_command_executor_${tag}.zip
```

- [ ] **Step 3: Upload the job artifact**

Use `actions/upload-artifact@v4`, artifact name `ulanzi-release-asset`, and `if-no-files-found: error`.

### Task 3: Aggregate assets in the single publish job

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Require both build jobs**

Set:

```yaml
needs: [release, ulanzi-release]
```

- [ ] **Step 2: Download the Ulanzi artifact**

Download `ulanzi-release-asset` into `dist` after downloading `release-assets`.

- [ ] **Step 3: Regenerate final checksums**

Before calling `gh release`, run:

```bash
cd dist
sha256sum *.zip *.alfredworkflow > checksums.txt
```

The existing create-or-upload behavior remains unchanged.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
python3 -m unittest tests/release_workflow_test.py -v
```

Expected: all tests pass.

### Task 4: Update release documentation

**Files:**
- Modify: `docs/release.md`
- Modify: `plugins/unlanzi_d200x/docs/README.md`

- [ ] **Step 1: Document the new asset**

Add:

```text
life_tools_ulanzi_d200x_command_executor_<tag>.zip
```

Explain that it is built on macOS, is directly installable in Ulanzi Studio, and is included in `checksums.txt`.

- [ ] **Step 2: Document historical-version behavior**

State that an existing Release is not automatically rebuilt after workflow changes; backfills require an explicit upload or a new tag.

### Task 5: Run full relevant verification

**Files:**
- No source changes.

- [ ] **Step 1: Run Release contract tests**

```bash
python3 -m unittest tests/release_workflow_test.py -v
```

- [ ] **Step 2: Parse workflow YAML**

```bash
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml", aliases: true); puts "workflow yaml ok"'
```

- [ ] **Step 3: Install and test Ulanzi dependencies**

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
./build.sh
```

Expected: 75 tests pass, package validation passes, and zip integrity passes.

- [ ] **Step 4: Validate documentation diagram**

Render the Mermaid diagram from the design document with `mmdc`.

- [ ] **Step 5: Review repository state**

```bash
git diff --check
git status --short
```

Confirm no generated output, `node_modules`, user-specific path, or credential is tracked.
