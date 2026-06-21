#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_NAME="codex_hook_notify"
OUTPUT_BIN="$ROOT_DIR/output/$BIN_NAME"
INSTALL_BIN="/usr/local/bin/$BIN_NAME"
CONFIG_DIR="/etc/life_tools"
CONFIG_FILE="$CONFIG_DIR/codex_hook_notify.json"
SAMPLE_CONFIG="$ROOT_DIR/sample/life_tools/codex_hook_notify.json"
OS_NAME="$(uname -s)"
case "$OS_NAME" in
  Linux)
    LOG_DIR="/var/log/codex_hook_notify"
    ;;
  Darwin)
    LOG_DIR="$HOME/Library/Logs/codex_hook_notify"
    ;;
  *)
    LOG_DIR="$HOME/.codex_hook_notify/logs"
    ;;
esac
HOOKS_FILE="$HOME/.codex/hooks.json"
INSTALL_CODEX_HOOK=0
WITH_PERMISSION_REQUEST=0

for arg in "$@"; do
  case "$arg" in
    --install-codex-hook)
      INSTALL_CODEX_HOOK=1
      ;;
    --with-permission-request)
      WITH_PERMISSION_REQUEST=1
      ;;
    -h|--help)
      echo "Usage: ./install.sh [--install-codex-hook] [--with-permission-request]"
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

command -v go >/dev/null 2>&1 || {
  echo "go command not found" >&2
  exit 1
}

mkdir -p "$ROOT_DIR/output"
go build -v -o "$OUTPUT_BIN" "$ROOT_DIR/codex_hook_notify/..."

sudo install -m 0755 "$OUTPUT_BIN" "$INSTALL_BIN"
sudo mkdir -p "$CONFIG_DIR"
if [ "$OS_NAME" = "Linux" ]; then
  sudo mkdir -p "$LOG_DIR"
  sudo chown "$(id -u):$(id -g)" "$LOG_DIR"
  sudo chmod 0755 "$LOG_DIR"
else
  mkdir -p "$LOG_DIR"
  chmod 0755 "$LOG_DIR"
fi

if [ ! -f "$CONFIG_FILE" ]; then
  sudo install -m 0644 "$SAMPLE_CONFIG" "$CONFIG_FILE"
  echo "installed sample config: $CONFIG_FILE"
else
  echo "config exists, skip overwrite: $CONFIG_FILE"
fi

if [ "$INSTALL_CODEX_HOOK" -eq 1 ]; then
  python3 - "$HOOKS_FILE" "$INSTALL_BIN" "$CONFIG_FILE" "$WITH_PERMISSION_REQUEST" <<'PYCODE'
import json
import sys
from pathlib import Path

hooks_file = Path(sys.argv[1])
bin_path = sys.argv[2]
config_file = sys.argv[3]
with_permission_request = sys.argv[4] == "1"
command = f"{bin_path} --config {config_file}"
entry = {
    "type": "command",
    "command": command,
    "timeout": 10,
}

if hooks_file.exists():
    try:
        data = json.loads(hooks_file.read_text())
    except json.JSONDecodeError as exc:
        raise SystemExit(f"parse {hooks_file} failed: {exc}")
else:
    data = {}

hooks = data.setdefault("hooks", {})
changed = False


def ensure_hook(event):
    global changed
    groups = hooks.setdefault(event, [])
    if not groups:
        groups.append({"hooks": []})
        changed = True
    commands = [hook.get("command") for group in groups for hook in group.get("hooks", [])]
    if command in commands:
        return
    groups[0].setdefault("hooks", []).append(dict(entry))
    changed = True


def remove_hook(event):
    global changed
    for group in hooks.get(event, []):
        old_hooks = group.get("hooks", [])
        new_hooks = [hook for hook in old_hooks if hook.get("command") != command]
        if len(new_hooks) != len(old_hooks):
            group["hooks"] = new_hooks
            changed = True


ensure_hook("Stop")
if with_permission_request:
    ensure_hook("PermissionRequest")
else:
    remove_hook("PermissionRequest")

if changed:
    hooks_file.parent.mkdir(parents=True, exist_ok=True)
    hooks_file.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")
    print(f"updated {hooks_file}")
else:
    print(f"hooks already installed in {hooks_file}")
PYCODE
else
  cat <<EOF
codex_hook_notify installed to $INSTALL_BIN

Next steps:
1. Edit $CONFIG_FILE and fill feishu_custom_robot_urls.
2. Run ./install.sh --install-codex-hook to install the Stop hook.
3. Optional: run ./install.sh --install-codex-hook --with-permission-request to also install PermissionRequest reminders.
EOF
fi
