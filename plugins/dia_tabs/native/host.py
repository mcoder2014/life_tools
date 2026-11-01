#!/usr/bin/env python3
"""Forward one live browser Native Messaging connection to private Unix clients."""

import json
import os
from pathlib import Path
import select
import socket
import stat
import struct
import sys
import threading
import time
import uuid

MAX_MESSAGE = 1024 * 1024
MAX_CLIENTS = 32
DEFAULT_RUN_DIR = Path.home() / ".local/share/life-tools-dia-tabs/run"


def read_exact(stream, count):
    data = bytearray()
    while len(data) < count:
        chunk = stream.read(count - len(data))
        if not chunk:
            if not data:
                return None
            raise ValueError("truncated native message")
        data.extend(chunk)
    return bytes(data)


def read_native(stream):
    header = read_exact(stream, 4)
    if header is None:
        return None
    length = struct.unpack("=I", header)[0]
    if not 0 < length <= MAX_MESSAGE:
        raise ValueError("invalid native message length")
    payload = read_exact(stream, length)
    if payload is None:
        raise ValueError("truncated native message")
    return json.loads(payload.decode("utf-8"))


def error_response(request_id, code, message):
    return {"v": 1, "id": request_id, "ok": False, "error": code, "message": message}


def validate_request(value):
    if not isinstance(value, dict) or type(value.get("v")) is not int or value["v"] != 1:
        return False
    if not isinstance(value.get("id"), str) or not 0 < len(value["id"]) <= 128:
        return False
    if value.get("op") == "list":
        return set(value) == {"v", "id", "op"}
    return (value.get("op") == "activate"
            and set(value) == {"v", "id", "op", "sessionId", "tabId"}
            and isinstance(value.get("sessionId"), str) and 0 < len(value["sessionId"]) <= 128
            and type(value.get("tabId")) is int and value["tabId"] >= 0)


class Bridge:
    def __init__(self, run_dir, timeout=2.0):
        self.run_dir = Path(run_dir)
        self.path = self.run_dir / (uuid.uuid4().hex[:12] + ".sock")
        self.timeout = timeout
        self.pending = {}
        self.pending_lock = threading.Lock()
        self.write_lock = threading.Lock()
        self.stopped = threading.Event()
        self.slots = threading.BoundedSemaphore(MAX_CLIENTS)
        self.workers = []

    def stop(self):
        self.stopped.set()
        with self.pending_lock:
            for pending in self.pending.values():
                pending["event"].set()

    def read_browser(self):
        try:
            while not self.stopped.is_set():
                value = read_native(sys.stdin.buffer)
                if value is None:
                    break
                if (not isinstance(value, dict) or value.get("v") != 1
                        or not isinstance(value.get("id"), str) or type(value.get("ok")) is not bool):
                    raise ValueError("invalid native response")
                with self.pending_lock:
                    pending = self.pending.get(value.get("id"))
                    if pending is not None:
                        pending["response"] = value
                        pending["event"].set()
        except (OSError, ValueError, UnicodeError):
            print("Dia Tabs: invalid or disconnected native stream", file=sys.stderr)
        finally:
            self.stop()

    def forward(self, client, request):
        internal_id = uuid.uuid4().hex
        pending = {"event": threading.Event(), "response": None}
        with self.pending_lock:
            self.pending[internal_id] = pending
        try:
            payload = json.dumps(dict(request, id=internal_id), ensure_ascii=False).encode("utf-8")
            with self.write_lock:
                if self.stopped.is_set():
                    return error_response(request["id"], "BRIDGE_UNAVAILABLE", "浏览器连接已断开")
                sys.stdout.buffer.write(struct.pack("=I", len(payload)) + payload)
                sys.stdout.buffer.flush()
            deadline = time.monotonic() + self.timeout
            while not pending["event"].wait(min(0.05, max(0, deadline - time.monotonic()))):
                if time.monotonic() >= deadline:
                    return error_response(request["id"], "TIMEOUT", "浏览器响应超时")
                if select.select([client], [], [], 0)[0] and not client.recv(1, socket.MSG_PEEK):
                    return None
            if pending["response"] is None:
                return error_response(request["id"], "BRIDGE_UNAVAILABLE", "浏览器连接已断开")
            return dict(pending["response"], id=request["id"])
        except (OSError, BrokenPipeError):
            self.stop()
            return error_response(request["id"], "BRIDGE_UNAVAILABLE", "浏览器连接已断开")
        finally:
            with self.pending_lock:
                self.pending.pop(internal_id, None)

    def handle_client(self, client):
        request_id = ""
        try:
            client.settimeout(self.timeout)
            with client.makefile("rb") as stream:
                line = stream.readline(MAX_MESSAGE + 1)
            if len(line) > MAX_MESSAGE or not line.endswith(b"\n"):
                raise ValueError("invalid client message length")
            request = json.loads(line.decode("utf-8"))
            if isinstance(request, dict) and isinstance(request.get("id"), str):
                request_id = request["id"][:128]
            if not validate_request(request):
                raise ValueError("invalid request")
            response = self.forward(client, request)
            if response is not None:
                client.sendall(json.dumps(response, ensure_ascii=False).encode("utf-8") + b"\n")
        except (ValueError, UnicodeError, socket.timeout):
            try:
                client.sendall(json.dumps(error_response(request_id, "BAD_REQUEST", "无效的桥接请求")).encode() + b"\n")
            except OSError:
                pass
        except OSError:
            pass
        finally:
            client.close()
            self.slots.release()

    def run(self):
        if len(os.fsencode(self.path)) >= 104:
            raise ValueError("Unix socket path must be shorter than 104 bytes; set DIA_TABS_RUN_DIR")
        self.run_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        info = self.run_dir.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid():
            raise ValueError("run directory must be a real directory owned by the current user")
        self.run_dir.chmod(0o700)
        try:
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as server:
                server.bind(str(self.path))
                self.path.chmod(0o600)
                server.listen(MAX_CLIENTS)
                server.settimeout(0.1)
                threading.Thread(target=self.read_browser, daemon=True).start()
                while not self.stopped.is_set():
                    try:
                        client, _ = server.accept()
                    except socket.timeout:
                        continue
                    if not self.slots.acquire(blocking=False):
                        client.close()
                        continue
                    self.workers = [worker for worker in self.workers if worker.is_alive()]
                    worker = threading.Thread(target=self.handle_client, args=(client,), daemon=True)
                    self.workers.append(worker)
                    worker.start()
        finally:
            self.stop()
            for worker in self.workers:
                worker.join(timeout=self.timeout + 0.2)
            self.path.unlink(missing_ok=True)


def main():
    os.umask(0o077)
    try:
        timeout = float(os.environ.get("DIA_TABS_TIMEOUT", "2"))
        if not 0.05 <= timeout <= 30:
            raise ValueError("DIA_TABS_TIMEOUT must be between 0.05 and 30 seconds")
        Bridge(os.environ.get("DIA_TABS_RUN_DIR", DEFAULT_RUN_DIR), timeout).run()
        return 0
    except (OSError, ValueError) as error:
        print("Dia Tabs: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
