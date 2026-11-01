#!/bin/bash
set -euo pipefail
PLUGIN_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "${PYTHON:-python3}" "$PLUGIN_DIR/native/build.py" "${1:-$PLUGIN_DIR/dist}" "${2:-dev}"
