#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
plugin_name="com.ulanzi.commandexecutor.ulanziPlugin"
source_plugin="$script_dir/$plugin_name"
bundle="$source_plugin/dist/app.js"
output_root="$script_dir/output"
output_plugin="$output_root/$plugin_name"
archive="$output_root/life_tools_ulanzi_d200x_command_executor.zip"

cd "$script_dir"
rm -f "$bundle"
rm -rf "$output_plugin"
rm -f "$archive"
npm run bundle
mkdir -p "$output_root"
/usr/bin/ditto --norsrc "$source_plugin" "$output_plugin"
node scripts/validate-package.mjs "$output_plugin"
(
  cd "$output_root"
  /usr/bin/ditto -c -k --norsrc --keepParent "$plugin_name" "$archive"
)
/usr/bin/unzip -t "$archive"
