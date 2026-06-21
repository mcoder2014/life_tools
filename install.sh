#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUTPUT_DIR="$ROOT_DIR/output"
PREFIX="/usr/local"
CONFIG_DIR="${LIFE_TOOLS_CONFIG_DIR:-/etc/life_tools}"
HOOKS_FILE="$HOME/.codex/hooks.json"
OS_NAME="$(uname -s)"

STABLE_TOOLS=(renameV1 check_keywords retry_exec codex_hook_notify video_subtitle)
ALL_TOOLS=(renameV1 check_keywords retry_exec codex_hook_notify video_subtitle dav)
REQUESTED_TOOLS=()
SELECTED_TOOLS=()
INSTALL_ALL=0
INSTALL_CODEX_HOOK=0
WITH_PERMISSION_REQUEST=0
WITH_PYTHON_DEPS=0

usage() {
  cat <<'EOF'
Usage:
  ./install.sh [options]

Options:
  --tool NAME                  Install only this tool. Can be repeated.
  --tool=NAME                  Same as --tool NAME.
  --tools a,b                  Install a comma-separated tool list.
  --all                        Install all tools, including experimental dav.
  --prefix DIR                 Install executables under DIR/bin. Default: /usr/local.
  --config-dir DIR             Install sample configs under DIR. Default: /etc/life_tools.
  --with-python-deps           Run python3 -m pip install for video_subtitle dependencies.
  --install-codex-hook         Install codex_hook_notify Stop hook.
  --with-permission-request    Also install codex_hook_notify PermissionRequest hook.
  -h, --help                   Show this help.

Stable tools installed by default:
  renameV1, check_keywords, retry_exec, codex_hook_notify, video_subtitle

All tool names:
  renameV1, check_keywords, retry_exec, codex_hook_notify, video_subtitle, dav

Examples:
  ./install.sh
  ./install.sh --tool retry_exec
  ./install.sh --tool renameV1 --tool check_keywords
  ./install.sh --tool video_subtitle --with-python-deps
  ./install.sh --tool codex_hook_notify --install-codex-hook
EOF
}

run_privileged() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
    return
  fi
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo command not found; cannot write privileged path" >&2
    exit 1
  fi
  sudo "$@"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 command not found" >&2
    exit 1
  fi
}

ensure_dir() {
  local dir="$1"
  local mode="$2"

  if [ -d "$dir" ]; then
    return
  fi
  if mkdir -p "$dir" 2>/dev/null; then
    chmod "$mode" "$dir" 2>/dev/null || true
    return
  fi
  run_privileged mkdir -p "$dir"
  run_privileged chmod "$mode" "$dir"
}

ensure_user_writable_dir() {
  local dir="$1"

  if [ -d "$dir" ] && [ -w "$dir" ]; then
    chmod 0755 "$dir" 2>/dev/null || true
    return
  fi
  if mkdir -p "$dir" 2>/dev/null && [ -w "$dir" ]; then
    chmod 0755 "$dir" 2>/dev/null || true
    return
  fi
  run_privileged mkdir -p "$dir"
  run_privileged chown "$(id -u):$(id -g)" "$dir"
  run_privileged chmod 0755 "$dir"
}

install_file() {
  local src="$1"
  local dest="$2"
  local mode="$3"

  ensure_dir "$(dirname "$dest")" 0755
  if install -m "$mode" "$src" "$dest" 2>/dev/null; then
    return
  fi
  run_privileged install -m "$mode" "$src" "$dest"
}

install_executable() {
  install_file "$1" "$2" 0755
  echo "installed executable: $2"
}

install_config_if_missing() {
  local sample="$1"
  local dest="$2"

  ensure_dir "$CONFIG_DIR" 0755
  if [ -f "$dest" ]; then
    echo "config exists, skip overwrite: $dest"
    return
  fi
  install_file "$sample" "$dest" 0644
  echo "installed sample config: $dest"
}

copy_dir_contents() {
  local src="$1"
  local dest="$2"

  ensure_dir "$dest" 0755
  if cp -R "$src/." "$dest/" 2>/dev/null; then
    chmod -R a+rX "$dest" 2>/dev/null || true
    return
  fi
  run_privileged cp -R "$src/." "$dest/"
  run_privileged chmod -R a+rX "$dest"
}

contains_tool() {
  local target="$1"
  shift
  local item
  for item in "$@"; do
    if [ "$item" = "$target" ]; then
      return 0
    fi
  done
  return 1
}

normalize_tool_name() {
  case "$1" in
    rename|rename_v1|renameV1)
      echo "renameV1"
      ;;
    check-keywords|check_keywords|save_work)
      echo "check_keywords"
      ;;
    retry|retry-exec|retry_exec)
      echo "retry_exec"
      ;;
    codex|codex-hook-notify|codex_hook_notify)
      echo "codex_hook_notify"
      ;;
    video|video-subtitle|video_subtitle)
      echo "video_subtitle"
      ;;
    dav|webdav)
      echo "dav"
      ;;
    *)
      return 1
      ;;
  esac
}

add_requested_tool() {
  local normalized
  if ! normalized="$(normalize_tool_name "$1")"; then
    echo "unknown tool: $1" >&2
    echo "run ./install.sh --help to list supported tools" >&2
    exit 1
  fi
  if ! contains_tool "$normalized" "${REQUESTED_TOOLS[@]}"; then
    REQUESTED_TOOLS+=("$normalized")
  fi
}

add_requested_tools_csv() {
  local value="$1"
  local names=()
  local old_ifs="$IFS"
  IFS=,
  read -r -a names <<< "$value"
  IFS="$old_ifs"

  local name
  for name in "${names[@]}"; do
    if [ -n "$name" ]; then
      add_requested_tool "$name"
    fi
  done
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --tool)
        if [ "$#" -lt 2 ]; then
          echo "--tool requires a tool name" >&2
          exit 1
        fi
        add_requested_tool "$2"
        shift 2
        ;;
      --tool=*)
        add_requested_tool "${1#--tool=}"
        shift
        ;;
      --tools)
        if [ "$#" -lt 2 ]; then
          echo "--tools requires a comma-separated tool list" >&2
          exit 1
        fi
        add_requested_tools_csv "$2"
        shift 2
        ;;
      --tools=*)
        add_requested_tools_csv "${1#--tools=}"
        shift
        ;;
      --all)
        INSTALL_ALL=1
        shift
        ;;
      --prefix)
        if [ "$#" -lt 2 ]; then
          echo "--prefix requires a directory" >&2
          exit 1
        fi
        PREFIX="${2%/}"
        shift 2
        ;;
      --prefix=*)
        PREFIX="${1#--prefix=}"
        PREFIX="${PREFIX%/}"
        shift
        ;;
      --config-dir)
        if [ "$#" -lt 2 ]; then
          echo "--config-dir requires a directory" >&2
          exit 1
        fi
        CONFIG_DIR="${2%/}"
        shift 2
        ;;
      --config-dir=*)
        CONFIG_DIR="${1#--config-dir=}"
        CONFIG_DIR="${CONFIG_DIR%/}"
        shift
        ;;
      --with-python-deps)
        WITH_PYTHON_DEPS=1
        shift
        ;;
      --install-codex-hook)
        INSTALL_CODEX_HOOK=1
        shift
        ;;
      --with-permission-request)
        WITH_PERMISSION_REQUEST=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "unknown argument: $1" >&2
        exit 1
        ;;
    esac
  done
}

select_tools() {
  if [ "$INSTALL_ALL" -eq 1 ]; then
    SELECTED_TOOLS=("${ALL_TOOLS[@]}")
  elif [ "${#REQUESTED_TOOLS[@]}" -gt 0 ]; then
    SELECTED_TOOLS=("${REQUESTED_TOOLS[@]}")
  else
    SELECTED_TOOLS=("${STABLE_TOOLS[@]}")
  fi

  if [ "$INSTALL_CODEX_HOOK" -eq 1 ] && ! contains_tool codex_hook_notify "${SELECTED_TOOLS[@]}"; then
    echo "--install-codex-hook requires installing codex_hook_notify" >&2
    exit 1
  fi
}

needs_go() {
  local tool
  for tool in "${SELECTED_TOOLS[@]}"; do
    case "$tool" in
      renameV1|check_keywords|retry_exec|codex_hook_notify|dav)
        return 0
        ;;
    esac
  done
  return 1
}

needs_python() {
  local tool
  for tool in "${SELECTED_TOOLS[@]}"; do
    case "$tool" in
      video_subtitle|codex_hook_notify)
        return 0
        ;;
    esac
  done
  return 1
}

build_go_tool() {
  local tool="$1"
  local bin_name="$2"
  local package_path="$3"
  local output_bin="$OUTPUT_DIR/$bin_name"

  mkdir -p "$OUTPUT_DIR"
  echo "building $tool"
  (cd "$ROOT_DIR" && go build -v -o "$output_bin" "$package_path")
}

install_go_tool() {
  local tool="$1"
  local bin_name="$2"
  local package_path="$3"

  build_go_tool "$tool" "$bin_name" "$package_path"
  install_executable "$OUTPUT_DIR/$bin_name" "$PREFIX/bin/$bin_name"
}

codex_log_dir() {
  case "$OS_NAME" in
    Linux)
      echo "/var/log/codex_hook_notify"
      ;;
    Darwin)
      echo "$HOME/Library/Logs/codex_hook_notify"
      ;;
    *)
      echo "$HOME/.codex_hook_notify/logs"
      ;;
  esac
}

install_rename_v1() {
  install_go_tool renameV1 renameV1 ./renameV1/...
  echo "renameV1 uses per-directory config files; sample: sample/life_tools/rename_v1.json"
}

install_check_keywords() {
  install_go_tool check_keywords check_keywords ./save_work/...
  install_config_if_missing "$ROOT_DIR/sample/life_tools/check_keywords.json" "$CONFIG_DIR/check_keywords.json"
}

install_retry_exec() {
  install_go_tool retry_exec retry_exec ./retry_exec/...
  install_config_if_missing "$ROOT_DIR/sample/life_tools/retry_exec.json" "$CONFIG_DIR/retry_exec.json"
  ensure_user_writable_dir "/var/log/retry_exec"
}

install_codex_hook_notify() {
  local config_file="$CONFIG_DIR/codex_hook_notify.json"
  local log_dir

  install_go_tool codex_hook_notify codex_hook_notify ./codex_hook_notify/...
  install_config_if_missing "$ROOT_DIR/sample/life_tools/codex_hook_notify.json" "$config_file"
  log_dir="$(codex_log_dir)"
  ensure_user_writable_dir "$log_dir"

  if [ "$INSTALL_CODEX_HOOK" -eq 1 ]; then
    install_codex_hook "$PREFIX/bin/codex_hook_notify" "$config_file"
    return
  fi

  cat <<EOF
codex_hook_notify installed to $PREFIX/bin/codex_hook_notify

Next steps:
1. Edit $config_file and fill feishu_custom_robot_urls.
2. Run ./install.sh --tool codex_hook_notify --install-codex-hook to install the Stop hook.
3. Optional: add --with-permission-request to also install PermissionRequest reminders.
EOF
}

install_codex_hook() {
  local install_bin="$1"
  local config_file="$2"

  require_command python3
  python3 - "$HOOKS_FILE" "$install_bin" "$config_file" "$WITH_PERMISSION_REQUEST" <<'PYCODE'
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
}

install_video_subtitle() {
  local lib_dir="$PREFIX/lib/life_tools/video_subtitle"
  local wrapper="$OUTPUT_DIR/video_subtitle"

  require_command python3
  mkdir -p "$OUTPUT_DIR"
  copy_dir_contents "$ROOT_DIR/video_subtitle" "$lib_dir"
  install_config_if_missing "$ROOT_DIR/sample/life_tools/video_subtitle.json" "$CONFIG_DIR/video_subtitle.json"

  cat > "$wrapper" <<EOF
#!/bin/sh
exec python3 "$lib_dir/video_subtitle.py" "\$@"
EOF
  chmod 0755 "$wrapper"
  install_executable "$wrapper" "$PREFIX/bin/video_subtitle"

  if [ "$WITH_PYTHON_DEPS" -eq 1 ]; then
    python3 -m pip install -r "$ROOT_DIR/video_subtitle/requirements.txt"
  else
    cat <<EOF
video_subtitle Python package files installed to $lib_dir
Python dependencies were not installed.
Run this if the runtime environment does not already have them:
  python3 -m pip install -r $ROOT_DIR/video_subtitle/requirements.txt
EOF
  fi
}

install_dav() {
  cat <<'EOF'
warning: dav is experimental and has hard-coded BasicAuth in current source.
Do not expose it to an untrusted network without reviewing webdav/main.go.
EOF
  install_go_tool dav dav ./webdav/...
}

install_selected_tool() {
  case "$1" in
    renameV1)
      install_rename_v1
      ;;
    check_keywords)
      install_check_keywords
      ;;
    retry_exec)
      install_retry_exec
      ;;
    codex_hook_notify)
      install_codex_hook_notify
      ;;
    video_subtitle)
      install_video_subtitle
      ;;
    dav)
      install_dav
      ;;
    *)
      echo "unsupported tool: $1" >&2
      exit 1
      ;;
  esac
}

main() {
  local tool

  parse_args "$@"
  select_tools

  if needs_go; then
    require_command go
  fi
  if needs_python; then
    require_command python3
  fi

  echo "selected tools: ${SELECTED_TOOLS[*]}"
  echo "install prefix: $PREFIX"
  echo "config dir: $CONFIG_DIR"

  for tool in "${SELECTED_TOOLS[@]}"; do
    install_selected_tool "$tool"
  done
}

main "$@"
