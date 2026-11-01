#!/bin/bash
set -euo pipefail
exec "${DIA_TABS_HOME:-$HOME/.local/share/life-tools-dia-tabs}/dia-tabs" action "${1:-}"
