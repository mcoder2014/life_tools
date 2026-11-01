#!/usr/bin/env python3
"""Install Dia Tabs in one stable user directory without sudo or UI automation."""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shlex
import shutil
import subprocess
import sys

HOST_NAME = "com.mcoder2014.life_tools.dia_tabs"
BUNDLE_ID = "com.mcoder2014.life-tools.dia-tabs"
SOURCE = Path(__file__).resolve().parents[1]
DEFAULT_PREFIX = Path.home() / ".local/share/life-tools-dia-tabs"
DEFAULT_HOST_DIR = Path.home() / "Library/Application Support/Google/Chrome/NativeMessagingHosts"


def extension_id(manifest):
    digest = hashlib.sha256(base64.b64decode(manifest["key"], validate=True)).hexdigest()[:32]
    return "".join(chr(ord("a") + int(char, 16)) for char in digest)


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    path.chmod(0o600)


def copy_source(source, target):
    if target.is_symlink():
        raise ValueError("安装目标不能是符号链接: " + str(target))
    shutil.copytree(source, target, dirs_exist_ok=True,
                    ignore=shutil.ignore_patterns("__pycache__", "*.pyc", ".DS_Store", "prefs.plist"))


def launcher(path, python, script, run_dir):
    path.write_text("#!/bin/sh\nexport DIA_TABS_RUN_DIR={}\nexec {} {} \"$@\"\n".format(
        shlex.quote(str(run_dir)), shlex.quote(str(python)), shlex.quote(str(script))), encoding="utf-8")
    path.chmod(0o700)


def install(args):
    prefix = args.prefix.expanduser().resolve()
    if prefix == SOURCE or SOURCE in prefix.parents:
        raise ValueError("请使用工作树之外的稳定安装目录")
    run_dir = prefix / "run"
    if len(os.fsencode(run_dir / ("0" * 12 + ".sock"))) >= 104:
        raise ValueError("安装目录过长，Unix socket 路径必须小于 104 字节")
    python_path = shutil.which(str(args.python))
    if not python_path:
        raise ValueError("未找到 Python 3: " + str(args.python))
    python_path = Path(python_path).resolve()
    subprocess.run([str(python_path), "-c", "import sys; assert sys.version_info >= (3, 9)"],
                   check=True, timeout=10)
    manifest = json.loads((SOURCE / "extension/manifest.json").read_text(encoding="utf-8"))
    ext_id = extension_id(manifest)
    host_dir = args.native_host_dir.expanduser().resolve()
    host_path = host_dir / (HOST_NAME + ".json")
    if host_path.exists():
        previous = json.loads(host_path.read_text(encoding="utf-8"))
        if previous.get("name") != HOST_NAME:
            raise ValueError("拒绝覆盖不属于本工具的 native manifest")
    workflow_target = None
    if args.workflow_dir:
        workflow_target = args.workflow_dir.expanduser().resolve() / "user.workflow.life-tools-dia-tabs"
        if (workflow_target / "info.plist").exists():
            with (workflow_target / "info.plist").open("rb") as stream:
                if plistlib.load(stream).get("bundleid") != BUNDLE_ID:
                    raise ValueError("拒绝覆盖其他 Alfred Workflow")
    prefix.mkdir(parents=True, exist_ok=True, mode=0o700)
    prefix.chmod(0o700)
    for component in ("extension", "native", "workflow"):
        copy_source(SOURCE / component, prefix / component)
    run_dir.mkdir(mode=0o700, exist_ok=True)
    run_dir.chmod(0o700)
    launcher(prefix / "native-host", python_path, prefix / "native/host.py", run_dir)
    launcher(prefix / "dia-tabs", python_path, prefix / "workflow/scripts/dia_tabs.py", run_dir)
    host_dir.mkdir(parents=True, exist_ok=True)
    write_json(host_path, {"name": HOST_NAME, "description": "Dia Tabs live-tab bridge",
                           "path": str(prefix / "native-host"), "type": "stdio",
                           "allowed_origins": ["chrome-extension://" + ext_id + "/"]})
    if workflow_target:
        copy_source(SOURCE / "workflow", workflow_target)
        with (workflow_target / "info.plist").open("rb") as stream:
            workflow = plistlib.load(stream)
        workflow.setdefault("variables", {})["DIA_TABS_HOME"] = str(prefix)
        with (workflow_target / "info.plist").open("wb") as stream:
            plistlib.dump(workflow, stream, sort_keys=False)
        for script in (workflow_target / "scripts").glob("*.sh"):
            script.chmod(0o755)
    write_json(prefix / "install-state.json", {"tool": HOST_NAME, "prefix": str(prefix),
                "nativeManifest": str(host_path), "workflow": str(workflow_target) if workflow_target else None,
                "extensionId": ext_id, "python": str(python_path)})
    print("安装目录: " + str(prefix))
    print("Dia 解压扩展目录: " + str(prefix / "extension"))
    print("Native manifest: " + str(host_path))
    print("扩展 ID: " + ext_id)
    print("Python: " + str(python_path))
    if workflow_target:
        print("Alfred Workflow: " + str(workflow_target))
    else:
        print("请在稳定文件安装后导入构建生成的 Dia Tabs .alfredworkflow")
    print("默认 Native 注册目录已在 Dia 1.55.1 验证；其他版本可用 --native-host-dir 覆盖。")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prefix", type=Path, default=DEFAULT_PREFIX, help="稳定用户安装目录")
    parser.add_argument("--python", default=sys.executable, help="Python 3.9+ 可执行文件")
    parser.add_argument("--native-host-dir", type=Path, default=DEFAULT_HOST_DIR,
                        help="单一 native host 注册目录；默认 Google/Chrome/NativeMessagingHosts")
    parser.add_argument("--workflow-dir", type=Path,
                        help="Alfred workflows 父目录；只更新本工具自己的子目录")
    args = parser.parse_args()
    try:
        install(args)
        return 0
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print("安装失败: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
