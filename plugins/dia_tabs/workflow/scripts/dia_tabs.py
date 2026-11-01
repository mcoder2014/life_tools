#!/usr/bin/env python3
"""Alfred Script Filter and exact-tab actions. No browser history is persisted."""

from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import re
import socket
import stat
import subprocess
import sys
import uuid

MAX_MESSAGE = 1024 * 1024
RUN_DIR = Path(os.environ.get("DIA_TABS_RUN_DIR", Path.home() / ".local/share/life-tools-dia-tabs/run"))
CONNECTION_ID = re.compile(r"[a-f0-9]{6,32}\Z")


def markdown(title, url):
    text = re.sub(r"\r\n|\r|\n", " ", title or url)
    text = text.replace("\\", "\\\\").replace("[", "\\[").replace("]", "\\]")
    target = "".join("%{:02X}".format(ord(char)) if ord(char) <= 0x20 or ord(char) == 0x7F
                     or char in "()<>\\" else char for char in url)
    return "[{}]({})".format(text, target)


def parse_query(query):
    value = query.strip()
    if value == "list":
        return ""
    if value.startswith("list "):
        return value[5:].strip()
    return None


def request_socket(path, request, timeout=1.25):
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
        client.settimeout(timeout)
        client.connect(str(path))
        client.sendall(json.dumps(request).encode("utf-8") + b"\n")
        with client.makefile("rb") as stream:
            line = stream.readline(MAX_MESSAGE + 1)
        if len(line) > MAX_MESSAGE or not line.endswith(b"\n"):
            raise ValueError("invalid bridge message length")
        value = json.loads(line.decode("utf-8"))
        if (not isinstance(value, dict) or value.get("v") != 1 or value.get("id") != request["id"]
                or type(value.get("ok")) is not bool):
            raise ValueError("invalid bridge response")
        return value


def tab_item(connection_id, session_id, tab):
    title_state = tab.get("titleState")
    title = tab.get("title", "") if title_state == "captured" else ""
    display = title or {"pending": "待访问", "unavailable": "无法读取页面标题"}.get(title_state, "无标题")
    identity = {"action": "activate", "connectionId": connection_id,
                "sessionId": session_id, "tabId": tab["tabId"]}
    encode = lambda value: json.dumps(value, ensure_ascii=False)
    return {"uid": "{}:{}:{}".format(connection_id, session_id, tab["tabId"]),
            "title": display, "subtitle": "{} · 窗口 {} · {}".format(tab["url"], tab["windowId"], connection_id[:6]),
            "arg": encode(identity), "valid": True,
            "mods": {"cmd": {"valid": True, "subtitle": "复制 URL",
                              "arg": encode({"action": "copy", "text": tab["url"]})},
                     "alt": {"valid": True, "subtitle": "复制 Markdown",
                              "arg": encode({"action": "copy", "text": markdown(title, tab["url"])})}}}


def read_connection(path):
    try:
        value = request_socket(path, {"v": 1, "id": uuid.uuid4().hex, "op": "list"})
        if value.get("ok") and isinstance(value.get("sessionId"), str) and isinstance(value.get("tabs"), list):
            return [(path.stem, value["sessionId"], tab) for tab in value["tabs"]
                    if isinstance(tab, dict) and type(tab.get("tabId")) is int
                    and type(tab.get("windowId")) is int and isinstance(tab.get("url"), str)
                    and tab["url"].startswith(("http://", "https://"))
                    and isinstance(tab.get("title", ""), str)]
    except (OSError, ValueError, UnicodeError):
        pass
    return None


def list_items(query, run_dir=RUN_DIR):
    paths = []
    for path in sorted(Path(run_dir).glob("*.sock")):
        try:
            info = path.lstat()
            if CONNECTION_ID.fullmatch(path.stem) and stat.S_ISSOCK(info.st_mode) and info.st_uid == os.getuid():
                paths.append(path)
        except OSError:
            continue
    needle = query.casefold()
    items, connected = [], False
    with ThreadPoolExecutor(max_workers=8) as executor:
        for tabs in executor.map(read_connection, paths):
            if tabs is None:
                continue
            connected = True
            for connection_id, session_id, tab in tabs:
                if needle in (tab.get("title", "") + " " + tab["url"]).casefold():
                    items.append(tab_item(connection_id, session_id, tab))
    if not items:
        title = "没有匹配的网页" if connected else "Dia 本机桥接未连接"
        items.append({"title": title, "subtitle": "请在 Dia 中启用扩展并检查控制页的桥接状态", "valid": False})
    return {"items": items}


def perform(value, run_dir=RUN_DIR):
    if not isinstance(value, dict):
        print("无效的操作参数", file=sys.stderr)
        return 1
    try:
        if value.get("action") == "copy" and isinstance(value.get("text"), str):
            result = subprocess.run(["/usr/bin/pbcopy"], input=value["text"].encode("utf-8"),
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=3, check=False)
            if result.returncode:
                raise ValueError("复制失败，请检查系统剪贴板")
            return 0
        connection_id = value.get("connectionId", "")
        if (value.get("action") != "activate" or not isinstance(connection_id, str)
                or not CONNECTION_ID.fullmatch(connection_id)
                or not isinstance(value.get("sessionId"), str) or type(value.get("tabId")) is not int):
            raise ValueError("无效的操作参数")
        response = request_socket(Path(run_dir) / (connection_id + ".sock"),
                                  {"v": 1, "id": uuid.uuid4().hex, "op": "activate",
                                   "sessionId": value["sessionId"], "tabId": value["tabId"]}, timeout=3)
        if not response["ok"]:
            message = {"TAB_CLOSED": "标签已关闭，请重新查询", "STALE_SESSION": "浏览器会话已变化，请重新查询",
                       "TIMEOUT": "浏览器响应超时", "BRIDGE_UNAVAILABLE": "浏览器连接已断开"}
            raise ValueError(message.get(response.get("error"), "激活失败，请重新查询"))
        return 0
    except (OSError, ValueError, UnicodeError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        return 1


def main():
    operation = sys.argv[1] if len(sys.argv) > 1 else "list"
    argument = sys.argv[2] if len(sys.argv) > 2 else ""
    if operation == "action":
        try:
            return perform(json.loads(argument))
        except ValueError:
            print("无效的操作参数", file=sys.stderr)
            return 1
    query = parse_query(argument)
    value = ({"items": [{"title": "列出 Dia 网页", "subtitle": "输入 dia list 搜索当前打开的网页",
                         "autocomplete": "list ", "valid": False}]} if query is None else list_items(query))
    print(json.dumps(value, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
