#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIGURATION="Release"
PLUGINS_DIR="/var/lib/emby/plugins"
DLL_PATH=""
RUN_BUILD=1
RUN_TESTS=1
RESTART_SERVICE=0
SERVICE_NAME="emby-server"

usage() {
  cat <<'EOF'
Usage:
  ./install.sh [options]

Options:
  --plugins-dir DIR       Emby plugins directory. Default: /var/lib/emby/plugins.
  --dll PATH              Install an existing LifeTools.Emby.VideoSubtitle.Emby.dll instead of building.
  --configuration NAME    Build configuration when --dll is not set. Default: Release.
  --no-test               Skip tests during build.
  --no-build              Do not build; install the default build output path.
  --restart               Restart emby-server after installing the DLL.
  --no-restart            Do not restart emby-server. This is the default.
  --service NAME          Service name to restart with --restart. Default: emby-server.
  -h, --help              Show this help.

Installed file:
  LifeTools.Emby.VideoSubtitle.Emby.dll

Example:
  ./build.sh
  sudo ./install.sh --restart
  ./install.sh --dll ./src/LifeTools.Emby.VideoSubtitle.Emby/bin/Release/netstandard2.0/LifeTools.Emby.VideoSubtitle.Emby.dll --plugins-dir /tmp/emby-plugins --no-restart
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

require_file() {
  if [ ! -f "$1" ]; then
    echo "file not found: $1" >&2
    exit 1
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --plugins-dir)
      if [ "$#" -lt 2 ]; then
        echo "--plugins-dir requires a directory" >&2
        exit 1
      fi
      PLUGINS_DIR="${2%/}"
      shift 2
      ;;
    --plugins-dir=*)
      PLUGINS_DIR="${1#--plugins-dir=}"
      PLUGINS_DIR="${PLUGINS_DIR%/}"
      shift
      ;;
    --dll)
      if [ "$#" -lt 2 ]; then
        echo "--dll requires a path" >&2
        exit 1
      fi
      DLL_PATH="$2"
      RUN_BUILD=0
      shift 2
      ;;
    --dll=*)
      DLL_PATH="${1#--dll=}"
      RUN_BUILD=0
      shift
      ;;
    --configuration)
      if [ "$#" -lt 2 ]; then
        echo "--configuration requires a value" >&2
        exit 1
      fi
      CONFIGURATION="$2"
      shift 2
      ;;
    --configuration=*)
      CONFIGURATION="${1#--configuration=}"
      shift
      ;;
    --no-test)
      RUN_TESTS=0
      shift
      ;;
    --no-build)
      RUN_BUILD=0
      shift
      ;;
    --restart)
      RESTART_SERVICE=1
      shift
      ;;
    --no-restart)
      RESTART_SERVICE=0
      shift
      ;;
    --service)
      if [ "$#" -lt 2 ]; then
        echo "--service requires a name" >&2
        exit 1
      fi
      SERVICE_NAME="$2"
      shift 2
      ;;
    --service=*)
      SERVICE_NAME="${1#--service=}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ "$RUN_BUILD" -eq 1 ]; then
  build_args=(--configuration "$CONFIGURATION")
  if [ "$RUN_TESTS" -eq 0 ]; then
    build_args+=(--no-test)
  fi
  "$ROOT_DIR/build.sh" "${build_args[@]}"
fi

if [ -z "$DLL_PATH" ]; then
  DLL_PATH="$ROOT_DIR/src/LifeTools.Emby.VideoSubtitle.Emby/bin/$CONFIGURATION/netstandard2.0/LifeTools.Emby.VideoSubtitle.Emby.dll"
fi

require_file "$DLL_PATH"

DEST="$PLUGINS_DIR/LifeTools.Emby.VideoSubtitle.Emby.dll"
if mkdir -p "$PLUGINS_DIR" 2>/dev/null && install -m 0644 "$DLL_PATH" "$DEST" 2>/dev/null; then
  :
else
  run_privileged mkdir -p "$PLUGINS_DIR"
  run_privileged install -m 0644 "$DLL_PATH" "$DEST"
fi

echo "installed plugin: $DEST"

if [ "$RESTART_SERVICE" -eq 1 ]; then
  run_privileged systemctl restart "$SERVICE_NAME"
  echo "restarted service: $SERVICE_NAME"
else
  echo "restart skipped; restart Emby before using the updated plugin"
fi
