import importlib.util
import io
import json
import os
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "workflow" / "scripts" / "dia_tabs.py"


class WorkflowTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not SCRIPT.exists():
            return
        spec = importlib.util.spec_from_file_location("dia_workflow", SCRIPT)
        cls.workflow = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.workflow)

    def setUp(self):
        self.assertTrue(SCRIPT.exists(), "Alfred workflow 尚未实现")

    def tab(self, **changes):
        return dict(tabId=7, windowId=2, index=0, url="https://example.test/path",
                    title="原始标题", titleState="captured", active=True, **changes)

    def test_keyword_parsing_and_filter(self):
        self.assertIsNone(self.workflow.parse_query(""))
        self.assertEqual(self.workflow.parse_query("list"), "")
        self.assertEqual(self.workflow.parse_query("list  Example"), "Example")
        self.assertIsNone(self.workflow.parse_query("listing"))
        item = self.workflow.tab_item("abc123", "session", self.tab())
        self.assertIn("原始标题", item["title"])
        self.assertEqual(json.loads(item["arg"])["tabId"], 7)
        self.assertEqual(json.loads(item["arg"])["action"], "activate")

    def test_modifiers_use_json_arguments(self):
        title = "`$(touch /tmp/nope)` [title]"
        tab = self.tab()
        tab["title"] = title
        item = self.workflow.tab_item("abcdef", "session", tab)
        url_arg = json.loads(item["mods"]["cmd"]["arg"])
        md_arg = json.loads(item["mods"]["alt"]["arg"])
        self.assertEqual(url_arg, {"action": "copy", "text": tab["url"]})
        self.assertEqual(md_arg["text"], self.workflow.markdown(title, tab["url"]))

    def test_markdown_shared_cases(self):
        cases = json.loads((ROOT / "tests" / "markdown_cases.json").read_text())
        for case in cases:
            with self.subTest(case=case):
                self.assertEqual(self.workflow.markdown(case["title"], case["url"]), case["expected"])

    def test_pending_title_uses_url_for_markdown(self):
        tab = self.tab()
        tab.update(title="", titleState="pending")
        item = self.workflow.tab_item("abcdef", "session", tab)
        self.assertIn("待访问", item["title"])
        self.assertEqual(json.loads(item["mods"]["alt"]["arg"])["text"],
                         "[https://example.test/path](https://example.test/path)")

    def test_disconnected_socket_never_shows_old_data(self):
        with tempfile.TemporaryDirectory(prefix="dw-", dir="/tmp") as directory:
            server = socket.socket(socket.AF_UNIX)
            path = Path(directory) / "deadbeef.sock"
            server.bind(str(path))
            server.close()
            result = self.workflow.list_items("", Path(directory))
            self.assertEqual(len(result["items"]), 1)
            self.assertFalse(result["items"][0]["valid"])
            self.assertIn("桥接", result["items"][0]["title"])

    def test_list_queries_live_connection_and_filters_case_insensitively(self):
        with tempfile.TemporaryDirectory(prefix="dw-", dir="/tmp") as directory:
            server = socket.socket(socket.AF_UNIX)
            server.bind(str(Path(directory) / "abcdef.sock"))
            server.listen()
            def serve():
                with server:
                    client, _ = server.accept()
                    with client:
                        request = json.loads(client.makefile("rb").readline())
                        client.sendall(json.dumps({"v": 1, "id": request["id"], "ok": True,
                                                   "sessionId": "session", "tabs": [self.tab()]}).encode() + b"\n")
            thread = threading.Thread(target=serve)
            thread.start()
            result = self.workflow.list_items("EXAMPLE", Path(directory))
            thread.join(timeout=2)
            self.assertEqual(len(result["items"]), 1)
            self.assertTrue(result["items"][0]["valid"])

    def test_copy_uses_stdin_and_reports_pbcopy_failure(self):
        with mock.patch.object(self.workflow.subprocess, "run") as run, mock.patch("sys.stderr", new=io.StringIO()):
            run.return_value.returncode = 0
            self.assertEqual(self.workflow.perform({"action": "copy", "text": "a `b` $(c)"}), 0)
            self.assertEqual(run.call_args.args[0], ["/usr/bin/pbcopy"])
            self.assertEqual(run.call_args.kwargs["input"], "a `b` $(c)".encode())
            self.assertNotIn("shell", run.call_args.kwargs)
            run.return_value.returncode = 1
            self.assertNotEqual(self.workflow.perform({"action": "copy", "text": "x"}), 0)

    def test_activate_rejects_connection_path_traversal(self):
        with mock.patch("sys.stderr", new=io.StringIO()):
            self.assertNotEqual(self.workflow.perform({"action": "activate", "connectionId": "../other",
                                                        "sessionId": "s", "tabId": 1}), 0)

    def test_activation_sends_exact_identity_without_url_lookup(self):
        with mock.patch.object(self.workflow, "request_socket") as request:
            request.return_value = {"v": 1, "id": "reply", "ok": True}
            value = {"action": "activate", "connectionId": "abcdef123456",
                     "sessionId": "current-session", "tabId": 123, "url": "https://same.test/"}
            self.assertEqual(self.workflow.perform(value, Path("/tmp/dia-test-run")), 0)
            self.assertEqual(request.call_args.args[0], Path("/tmp/dia-test-run/abcdef123456.sock"))
            payload = request.call_args.args[1]
            self.assertEqual(set(payload), {"v", "id", "op", "sessionId", "tabId"})
            self.assertEqual(payload["sessionId"], "current-session")
            self.assertEqual(payload["tabId"], 123)


if __name__ == "__main__":
    unittest.main()
