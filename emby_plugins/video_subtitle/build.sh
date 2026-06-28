#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOLUTION="$ROOT_DIR/LifeTools.Emby.VideoSubtitle.sln"
CONFIGURATION="Release"
RUN_TESTS=1

usage() {
  cat <<'EOF'
Usage:
  ./build.sh [options]

Options:
  --configuration NAME    Build configuration. Default: Release.
  --no-test               Skip dotnet test and only build the plugin.
  -h, --help              Show this help.

Output:
  src/LifeTools.Emby.VideoSubtitle.Emby/bin/<Configuration>/netstandard2.0/LifeTools.Emby.VideoSubtitle.Emby.dll
EOF
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 command not found" >&2
    exit 1
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
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

require_command dotnet

if [ "$RUN_TESTS" -eq 1 ]; then
  dotnet test "$SOLUTION" --configuration "$CONFIGURATION"
fi

dotnet build "$SOLUTION" --configuration "$CONFIGURATION"

DLL="$ROOT_DIR/src/LifeTools.Emby.VideoSubtitle.Emby/bin/$CONFIGURATION/netstandard2.0/LifeTools.Emby.VideoSubtitle.Emby.dll"
if [ ! -f "$DLL" ]; then
  echo "plugin dll not found after build: $DLL" >&2
  exit 1
fi

echo "plugin dll: $DLL"
