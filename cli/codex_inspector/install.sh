#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_DIR="$ROOT_DIR/output"
PREFIX="${CODEX_INSPECTOR_PREFIX:-$HOME/.local}"
ALLOW_SUDO=0
OS_NAME="$(uname -s)"

usage() {
  cat <<'EOF'
Usage:
  ./cli/codex_inspector/install.sh [options]

Options:
  --prefix DIR       Install under DIR/bin. Default: $HOME/.local.
  --system           Install under /usr/local and allow sudo if needed.
  --allow-sudo       Allow sudo when the selected prefix is not writable.
  -h, --help         Show this help.

Examples:
  ./cli/codex_inspector/install.sh
  ./cli/codex_inspector/install.sh --prefix "$HOME/.local"
  ./cli/codex_inspector/install.sh --system
EOF
}

die() {
  echo "codex_inspector install: $*" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "$1 command not found"
  fi
}

run_privileged() {
  if [ "$ALLOW_SUDO" -ne 1 ]; then
    die "cannot write target path; choose --prefix \"\$HOME/.local\" or pass --allow-sudo/--system explicitly"
  fi
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
    return
  fi
  require_command sudo
  sudo "$@"
}

ensure_supported_os() {
  case "$OS_NAME" in
    Darwin|Linux)
      ;;
    *)
      die "unsupported OS: $OS_NAME; supported systems are macOS and Linux"
      ;;
  esac
}

ensure_bin_dir() {
  local bin_dir="$1"

  if [ -d "$bin_dir" ]; then
    if [ -w "$bin_dir" ]; then
      return
    fi
    run_privileged mkdir -p "$bin_dir"
    return
  fi

  if mkdir -p "$bin_dir" 2>/dev/null; then
    return
  fi
  run_privileged mkdir -p "$bin_dir"
}

install_binary() {
  local src="$1"
  local dest="$2"

  if install -m 0755 "$src" "$dest" 2>/dev/null; then
    return
  fi
  run_privileged install -m 0755 "$src" "$dest"
}

print_path_hint() {
  local bin_dir="$1"

  case ":$PATH:" in
    *":$bin_dir:"*)
      return
      ;;
  esac

  cat <<EOF

Warning: $bin_dir is not in PATH.
Add it to your shell profile before running codex_inspector by name:
  export PATH="$bin_dir:\$PATH"
EOF
}

print_cache_hint() {
  case "$OS_NAME" in
    Darwin)
      echo "Default cache: $HOME/Library/Caches/life_tools/codex_inspector/session_summary_cache.sqlite"
      ;;
    Linux)
      echo "Default cache: ${XDG_CACHE_HOME:-$HOME/.cache}/life_tools/codex_inspector/session_summary_cache.sqlite"
      ;;
  esac
}

print_start_hint() {
  local install_path="$1"

  cat <<EOF

Start:
  $install_path -addr 127.0.0.1:8787
EOF

  case "$OS_NAME" in
    Darwin)
      echo "  open http://127.0.0.1:8787"
      ;;
    Linux)
      if command -v xdg-open >/dev/null 2>&1; then
        echo "  xdg-open http://127.0.0.1:8787"
      else
        echo "  visit http://127.0.0.1:8787 in your browser"
      fi
      ;;
  esac
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --prefix)
        if [ "$#" -lt 2 ]; then
          die "--prefix requires a directory"
        fi
        PREFIX="${2%/}"
        shift 2
        ;;
      --prefix=*)
        PREFIX="${1#--prefix=}"
        PREFIX="${PREFIX%/}"
        shift
        ;;
      --system)
        PREFIX="/usr/local"
        ALLOW_SUDO=1
        shift
        ;;
      --allow-sudo)
        ALLOW_SUDO=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

main() {
  local bin_dir
  local output_bin
  local install_path

  parse_args "$@"
  ensure_supported_os
  require_command go

  bin_dir="$PREFIX/bin"
  output_bin="$OUTPUT_DIR/codex_inspector"
  install_path="$bin_dir/codex_inspector"

  mkdir -p "$OUTPUT_DIR"
  echo "building codex_inspector"
  (cd "$ROOT_DIR" && go build -v -o "$output_bin" ./cli/codex_inspector/...)

  ensure_bin_dir "$bin_dir"
  install_binary "$output_bin" "$install_path"

  echo "installed codex_inspector: $install_path"
  print_cache_hint
  print_path_hint "$bin_dir"
  print_start_hint "$install_path"
}

main "$@"
