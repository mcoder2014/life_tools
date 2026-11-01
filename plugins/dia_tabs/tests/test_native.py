import io
import importlib.util
import json
import os
from pathlib import Path
import select
import socket
import stat
import struct
import subprocess
import sys
import tempfile
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
HOST = ROOT / "native" / "host.py"


def frame(value):
    payload = json.dumps(value, ensure_ascii=False).encode("utf-8")
    return struct.pack("=I", len(payload)) + payload


def read_frame(stream, timeout=3):
    if not select.select([stream], [], [], timeout)[0]:
        raise AssertionError("native response timed out")
    length = struct.unpack("=I", stream.read(4))[0]
    return json.loads(stream.read(length))


class NativeIntegrationTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(HOST.exists(), "native host 尚未实现")
        self.temp = tempfile.TemporaryDirectory(prefix="dt-", dir="/tmp")
        self.run_dir = Path(self.temp.name) / "run"
        env = dict(os.environ, DIA_TABS_RUN_DIR=str(self.run_dir), DIA_TABS_TIMEOUT="0.3")
        self.proc = subprocess.Popen([sys.executable, str(HOST)], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, bufsize=0)
        deadline = time.monotonic() + 3
        self.paths = []
        while time.monotonic() < deadline:
            self.paths = list(self.run_dir.glob("*.sock"))
            if self.paths:
                break
            if self.proc.poll() is not None:
                self.fail(self.proc.stderr.read().decode())
            time.sleep(0.01)
        self.assertEqual(len(self.paths), 1)

    def tearDown(self):
        if hasattr(self, "proc"):
            if not self.proc.stdin.closed:
                self.proc.stdin.close()
            try:
                self.proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()
            self.proc.stdout.close()
            self.proc.stderr.close()
        if hasattr(self, "temp"):
            self.temp.cleanup()

    def client(self, request):
        client = socket.socket(socket.AF_UNIX)
        client.settimeout(3)
        client.connect(str(self.paths[0]))
        client.sendall(json.dumps(request).encode() + b"\n")
        return client

    def response(self, client):
        with client:
            return json.loads(client.makefile("rb").readline())

    def test_forwards_unicode_and_fragmented_native_frame(self):
        client = self.client({"v": 1, "id": "client-id", "op": "list"})
        request = read_frame(self.proc.stdout)
        self.assertNotEqual(request["id"], "client-id")
        response = {"v": 1, "id": request["id"], "ok": True,
                    "sessionId": "s", "tabs": [{"title": "中文标题"}]}
        payload = frame(response)
        for chunk in [payload[:2], payload[2:7], payload[7:]]:
            self.proc.stdin.write(chunk)
            self.proc.stdin.flush()
        actual = self.response(client)
        self.assertEqual(actual, dict(response, id="client-id"))

    def test_duplicate_client_ids_are_isolated(self):
        clients = [self.client({"v": 1, "id": "same", "op": "list"}),
                   self.client({"v": 1, "id": "same", "op": "activate", "sessionId": "s", "tabId": 9})]
        requests = [read_frame(self.proc.stdout) for _ in clients]
        self.assertNotEqual(requests[0]["id"], requests[1]["id"])
        for request in reversed(requests):
            self.proc.stdin.write(frame({"v": 1, "id": request["id"], "ok": True,
                                        "sessionId": request["op"], "tabs": []}))
        self.proc.stdin.flush()
        results = [self.response(client) for client in clients]
        self.assertEqual([result["sessionId"] for result in results], ["list", "activate"])
        self.assertTrue(all(result["id"] == "same" for result in results))

    def test_rejects_unknown_operation(self):
        result = self.response(self.client({"v": 1, "id": "bad", "op": "exec"}))
        self.assertEqual(result["error"], "BAD_REQUEST")
        self.assertFalse(select.select([self.proc.stdout], [], [], 0.05)[0])

    def test_activate_preserves_stale_session_error(self):
        client = self.client({"v": 1, "id": "a", "op": "activate", "sessionId": "old", "tabId": 7})
        request = read_frame(self.proc.stdout)
        self.assertEqual(request["sessionId"], "old")
        self.proc.stdin.write(frame({"v": 1, "id": request["id"], "ok": False,
                                    "error": "STALE_SESSION"}))
        self.proc.stdin.flush()
        self.assertEqual(self.response(client)["error"], "STALE_SESSION")

    def test_timeout_returns_error(self):
        client = self.client({"v": 1, "id": "slow", "op": "list"})
        read_frame(self.proc.stdout)
        self.assertEqual(self.response(client)["error"], "TIMEOUT")

    def test_eof_wakes_waiter_and_removes_socket(self):
        client = self.client({"v": 1, "id": "eof", "op": "list"})
        read_frame(self.proc.stdout)
        self.proc.stdin.close()
        self.assertEqual(self.response(client)["error"], "BRIDGE_UNAVAILABLE")
        self.proc.wait(timeout=3)
        self.assertFalse(self.paths[0].exists())

    def test_private_permissions(self):
        self.assertEqual(stat.S_IMODE(self.run_dir.stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE(self.paths[0].stat().st_mode), 0o600)

    def test_invalid_native_frame_exits_and_cleans_up(self):
        self.proc.stdin.write(struct.pack("=I", 2 ** 21))
        self.proc.stdin.flush()
        self.proc.wait(timeout=3)
        self.assertFalse(self.paths[0].exists())

    def test_invalid_json_client_is_rejected(self):
        client = socket.socket(socket.AF_UNIX)
        client.settimeout(2)
        client.connect(str(self.paths[0]))
        client.sendall(b"{broken}\n")
        self.assertEqual(self.response(client)["error"], "BAD_REQUEST")

    def test_invalid_native_json_wakes_waiter(self):
        client = self.client({"v": 1, "id": "eof", "op": "list"})
        read_frame(self.proc.stdout)
        self.proc.stdin.write(struct.pack("=I", 1) + b"{")
        self.proc.stdin.flush()
        self.assertEqual(self.response(client)["error"], "BRIDGE_UNAVAILABLE")
        self.proc.wait(timeout=3)
        self.assertFalse(self.paths[0].exists())

    def test_oversized_client_request_is_rejected(self):
        client = socket.socket(socket.AF_UNIX)
        client.settimeout(3)
        client.connect(str(self.paths[0]))
        try:
            client.sendall(b"x" * (1024 * 1024 + 1))
        except BrokenPipeError:
            pass
        self.assertEqual(self.response(client)["error"], "BAD_REQUEST")


class NativeProtocolTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location("dia_native", HOST)
        cls.host = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.host)

    def test_exact_reader_accepts_split_utf8_payload(self):
        class Fragments(io.BytesIO):
            def read(self, count=-1):
                return super().read(min(count, 2))
        value = {"v": 1, "id": "中文", "ok": True}
        self.assertEqual(self.host.read_native(Fragments(frame(value))), value)

    def test_partial_native_frames_are_rejected(self):
        for data in (b"\x01", struct.pack("=I", 5) + b"{}", struct.pack("=I", 0),
                     struct.pack("=I", 1024 * 1024 + 1)):
            with self.subTest(data=data), self.assertRaises(ValueError):
                self.host.read_native(io.BytesIO(data))

    def test_request_validation_rejects_paths_boolean_ids_and_extra_fields(self):
        for request in ({"v": True, "id": "r", "op": "list"},
                        {"v": 1, "id": "r", "op": "list", "path": "/tmp/file"},
                        {"v": 1, "id": "r", "op": "activate", "sessionId": "s", "tabId": True},
                        {"v": 1, "id": "", "op": "list"}):
            with self.subTest(request=request):
                self.assertFalse(self.host.validate_request(request))

    def test_socket_length_uses_utf8_bytes_and_rejects_before_mkdir(self):
        with tempfile.TemporaryDirectory(prefix="dtp-", dir="/tmp") as directory:
            path = Path(directory) / ("中" * 32)
            with self.assertRaises(ValueError):
                self.host.Bridge(path).run()
            self.assertFalse(path.exists())


if __name__ == "__main__":
    unittest.main()
