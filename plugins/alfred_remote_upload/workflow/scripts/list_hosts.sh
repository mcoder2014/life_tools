#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec /usr/bin/osascript -l JavaScript "$SCRIPT_DIR/hosts.js" list "${1:-}"
