#!/bin/bash
set -euo pipefail
PLUGIN_DIR="$(cd "$(dirname "$0")" && pwd)"
python_bin="${PYTHON:-python3}"
exec "$python_bin" "$PLUGIN_DIR/native/install.py" "$@"
