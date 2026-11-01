import importlib.util
import json
import os
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import unittest
import zipfile

ROOT = Path(__file__).resolve().parents[1]
HOST_NAME = "com.mcoder2014.life_tools.dia_tabs"


class InstallTests(unittest.TestCase):
    def test_default_host_directory_matches_verified_dia_location(self):
        spec = importlib.util.spec_from_file_location("dia_installer", ROOT / "native/install.py")
        installer = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(installer)
        self.assertEqual(installer.DEFAULT_HOST_DIR,
                         Path.home() / "Library/Application Support/Google/Chrome/NativeMessagingHosts")

    def test_stable_install_preserves_other_workflows_and_pins_python(self):
        with tempfile.TemporaryDirectory(prefix="dti-", dir="/tmp") as directory:
            target = Path(directory).resolve()
            prefix = target / "stable ' runtime"
            hosts = target / "hosts"
            workflows = target / "workflows"
            other = workflows / "user.workflow.other"
            other.mkdir(parents=True)
            (other / "keep.txt").write_text("existing")
            result = subprocess.run([sys.executable, str(ROOT / "native/install.py"),
                                     "--prefix", str(prefix), "--python", sys.executable,
                                     "--native-host-dir", str(hosts), "--workflow-dir", str(workflows)],
                                    capture_output=True, text=True, check=True)
            manifest = json.loads((hosts / (HOST_NAME + ".json")).read_text())
            self.assertEqual(manifest["path"], str(prefix / "native-host"))
            self.assertEqual(manifest["allowed_origins"], ["chrome-extension://mcalepoloegbbciipaklojhcapcghdpd/"])
            self.assertNotIn(str(ROOT), (prefix / "native-host").read_text())
            self.assertIn(str(Path(sys.executable).resolve()), (prefix / "native-host").read_text())
            self.assertEqual((other / "keep.txt").read_text(), "existing")
            with (workflows / "user.workflow.life-tools-dia-tabs/info.plist").open("rb") as stream:
                workflow = plistlib.load(stream)
            self.assertEqual(workflow["variables"]["DIA_TABS_HOME"], str(prefix))
            listed = subprocess.run([str(prefix / "dia-tabs"), "list", "list"],
                                    capture_output=True, text=True, check=True)
            self.assertFalse(json.loads(listed.stdout)["items"][0]["valid"])

    def test_refuses_foreign_native_manifest(self):
        with tempfile.TemporaryDirectory(prefix="dti-", dir="/tmp") as directory:
            target = Path(directory)
            host = target / "hosts"
            host.mkdir()
            path = host / (HOST_NAME + ".json")
            path.write_text('{"name":"other-tool"}')
            result = subprocess.run([sys.executable, str(ROOT / "native/install.py"), "--prefix", str(target / "stable"),
                                     "--native-host-dir", str(host)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(json.loads(path.read_text()), {"name": "other-tool"})

    def test_build_archives_exclude_runtime_state(self):
        with tempfile.TemporaryDirectory(prefix="dtb-", dir="/tmp") as directory:
            subprocess.run([sys.executable, str(ROOT / "native/build.py"), directory, "test"],
                           capture_output=True, check=True)
            paths = list(Path(directory).iterdir())
            self.assertEqual(len(paths), 3)
            for path in paths:
                with zipfile.ZipFile(path) as archive:
                    self.assertIsNone(archive.testzip())
                    self.assertFalse(any("__pycache__" in name or name.endswith(".sock")
                                         or name.endswith("install-state.json") or name.endswith("prefs.plist")
                                         for name in archive.namelist()))
            with zipfile.ZipFile(Path(directory) / "life_tools_dia_tabs_extension_test.zip") as archive:
                self.assertIn("manifest.json", archive.namelist())
            with zipfile.ZipFile(Path(directory) / "life_tools_dia_tabs_test.alfredworkflow") as archive:
                self.assertIn("info.plist", archive.namelist())


if __name__ == "__main__":
    unittest.main()
