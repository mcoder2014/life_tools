#!/bin/bash

set -euo pipefail

PLUGIN_DIR="$(cd "$(dirname "$0")" && pwd)"
WORKFLOW_DIR="$PLUGIN_DIR/workflow"
requested_output="${1:-$PLUGIN_DIR/dist/Remote Upload.alfredworkflow}"
output_dir="$(dirname "$requested_output")"
output_name="$(basename "$requested_output")"

mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
output_path="$output_dir/$output_name"
/bin/rm -f "$output_path"

(
  cd "$WORKFLOW_DIR"
  /usr/bin/zip -qry "$output_path" . \
    -x '*.DS_Store' 'prefs.plist' 'recent_hosts.json' '*.alfredpreferences'
)

printf '%s\n' "$output_path"
