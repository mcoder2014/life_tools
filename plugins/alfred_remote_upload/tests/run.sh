#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOSTS_SCRIPT="$ROOT_DIR/workflow/scripts/hosts.js"
CLIPBOARD_SCRIPT="$ROOT_DIR/workflow/scripts/clipboard.js"
UPLOAD_SCRIPT="$ROOT_DIR/workflow/scripts/upload.sh"
INFO_PLIST="$ROOT_DIR/workflow/info.plist"
BUILD_SCRIPT="$ROOT_DIR/build.sh"
FAKES_DIR="$ROOT_DIR/tests/fakes"
TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/alfred-remote-upload-tests.XXXXXX")"
DATA_DIR="$TEST_TMP/data"

cleanup() {
  rm -rf "$TEST_TMP"
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_eq() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "$expected" != "$actual" ]]; then
    fail "$message: expected '$expected', got '$actual'"
  fi
}

json_get() {
  local json="$1"
  local path="$2"
  printf '%s' "$json" | /usr/bin/plutil -extract "$path" raw -o - -
}

json_item_count() {
  /usr/bin/python3 -c 'import json,sys; print(len(json.load(sys.stdin)["items"]))'
}

run_list() {
  local config="$1"
  local max_size="$2"
  local query="${3:-}"
  env \
    hosts_json="$config" \
    max_size_mb="$max_size" \
    alfred_workflow_data="$DATA_DIR" \
    /usr/bin/osascript -l JavaScript "$HOSTS_SCRIPT" list "$query"
}

mark_used() {
  local host="$1"
  local remote_dir="$2"
  env \
    alfred_workflow_data="$DATA_DIR" \
    /usr/bin/osascript -l JavaScript "$HOSTS_SCRIPT" mark-used "$host" "$remote_dir"
}

test_valid_config_and_filter() {
  local config output
  config='[{"name":"开发机","host":"devbox","remote_dir":"/home/user/tmp_share"},{"name":"生产机","host":"prod","remote_dir":"/srv/share"}]'
  output="$(run_list "$config" 100)"

  assert_eq "2" "$(printf '%s' "$output" | json_item_count)" "valid config should return two hosts"
  assert_eq "开发机" "$(json_get "$output" 'items.0.title')" "config order should be preserved"
  assert_eq "devbox" "$(json_get "$output" 'items.0.variables.upload_host')" "host variable should be passed"
  assert_eq "/home/user/tmp_share" "$(json_get "$output" 'items.0.variables.upload_remote_dir')" "remote directory should be passed"

  output="$(run_list "$config" 100 'prod')"
  assert_eq "生产机" "$(json_get "$output" 'items.0.title')" "query should filter by SSH host"
}

test_mru_sort_and_corrupt_state_fallback() {
  local config changed_config output
  config='[{"name":"开发机","host":"devbox","remote_dir":"/home/user/tmp_share"},{"name":"生产机","host":"prod","remote_dir":"/srv/share"}]'

  mark_used "prod" "/srv/share"
  output="$(run_list "$config" 100)"
  assert_eq "生产机" "$(json_get "$output" 'items.0.title')" "most recently used host should be first"

  changed_config='[{"name":"开发机","host":"devbox","remote_dir":"/home/user/tmp_share"},{"name":"测试机","host":"test","remote_dir":"/tmp/share"}]'
  output="$(run_list "$changed_config" 100)"
  assert_eq "开发机" "$(json_get "$output" 'items.0.title')" "removed MRU hosts should not change configured order"

  printf '{broken' > "$DATA_DIR/recent_hosts.json"
  output="$(run_list "$config" 100)"
  assert_eq "开发机" "$(json_get "$output" 'items.0.title')" "corrupt state should fall back to config order"
}

assert_config_error() {
  local config="$1"
  local max_size="$2"
  local expected_message="$3"
  local output
  output="$(run_list "$config" "$max_size")"
  assert_eq "false" "$(json_get "$output" 'items.0.valid')" "invalid config result must not be actionable"
  [[ "$(json_get "$output" 'items.0.subtitle')" == *"$expected_message"* ]] || fail "expected config error containing '$expected_message'"
}

test_config_validation() {
  local output
  output="$(run_list '[{"name":"A","host":"devbox","remote_dir":"/a"}]' '')"
  assert_eq "A" "$(json_get "$output" 'items.0.title')" "empty max_size_mb should use the default"

  output="$(run_list '[{"name":"toString","host":"devbox","remote_dir":"/a"}]' 100)"
  assert_eq "toString" "$(json_get "$output" 'items.0.title')" "valid names must not collide with Object prototype keys"

  assert_config_error '{bad json' 100 "JSON"
  assert_config_error '[]' 100 "至少配置一台主机"
  assert_config_error '[{"name":"开发机","host":"devbox"}]' 100 "remote_dir"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"/a"},{"name":"A","host":"prod","remote_dir":"/b"}]' 100 "name 不能重复"
  assert_config_error '[{"name":"A","host":"-bad","remote_dir":"/a"}]' 100 "SSH Host"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"relative"}]' 100 "绝对路径"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"/a\tb"}]' 100 "控制字符"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"/a"}]' 0 "max_size_mb"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"/a"}]' abc "max_size_mb"
  assert_config_error '[{"name":"A","host":"devbox","remote_dir":"/a"}]' 999999999999999999 "max_size_mb"
}

test_clipboard_type_policy() {
  local output
  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" classify-types '["public.png","public.tiff"]')"
  assert_eq "png" "$(json_get "$output" 'extension')" "PNG should be preserved"
  assert_eq "true" "$(json_get "$output" 'preserve')" "PNG should not be converted"

  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" classify-types '["public.jpeg"]')"
  assert_eq "jpg" "$(json_get "$output" 'extension')" "JPEG should use jpg extension"

  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" classify-types '["com.compuserve.gif"]')"
  assert_eq "gif" "$(json_get "$output" 'extension')" "GIF should be preserved"

  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" classify-types '["org.webmproject.webp"]')"
  assert_eq "webp" "$(json_get "$output" 'extension')" "WebP should be preserved"

  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" classify-types '["public.tiff"]')"
  assert_eq "png" "$(json_get "$output" 'extension')" "TIFF should fall back to PNG"
  assert_eq "false" "$(json_get "$output" 'preserve')" "TIFF should be converted"
}

file_paths_json() {
  /usr/bin/python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$@"
}

test_clipboard_file_validation() {
  local regular_file="$TEST_TMP/no-extension"
  local directory="$TEST_TMP/clipboard-directory"
  local fifo="$TEST_TMP/clipboard-fifo"
  local output
  printf 'file' > "$regular_file"
  mkdir -p "$directory"
  /usr/bin/mkfifo "$fifo"

  output="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" validate-file-paths "$(file_paths_json "$regular_file")")"
  assert_eq "file" "${output%%$'\x1f'*}" "single regular file should be accepted"
  [[ "$output" == *"$regular_file"* ]] || fail "regular file result should contain its path"

  if /usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" validate-file-paths "$(file_paths_json "$directory")" >/dev/null 2>&1; then
    fail "directories should be rejected"
  fi
  if /usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" validate-file-paths "$(file_paths_json "$regular_file" "$directory")" >/dev/null 2>&1; then
    fail "multiple Finder items should be rejected"
  fi
  if /usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" validate-file-paths "$(file_paths_json "$fifo")" >/dev/null 2>&1; then
    fail "special files should be rejected"
  fi
  if /usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" validate-file-paths "$(file_paths_json "$TEST_TMP/missing")" >/dev/null 2>&1; then
    fail "missing Finder files should be rejected"
  fi
}

run_upload() {
  local kind="$1"
  local source_path="$2"
  local extension="$3"
  local data_dir="$4"
  local remote_dir="$5"
  local max_size="$6"

  env \
    upload_name="devbox" \
    upload_host="devbox" \
    upload_remote_dir="$remote_dir" \
    max_size_mb="$max_size" \
    alfred_workflow_data="$data_dir" \
    alfred_workflow_cache="$TEST_TMP/cache" \
    ALFRED_REMOTE_UPLOAD_DATE="${ALFRED_REMOTE_UPLOAD_DATE:-20260714}" \
    CLIPBOARD_HELPER="$FAKES_DIR/clipboard" \
    SSH_BIN="$FAKES_DIR/ssh" \
    SCP_BIN="$FAKES_DIR/scp" \
    PBCOPY_BIN="$FAKES_DIR/pbcopy" \
    NOTIFY_BIN="$FAKES_DIR/notify" \
    HOSTS_SCRIPT="$HOSTS_SCRIPT" \
    FAKE_SOURCE_KIND="$kind" \
    FAKE_SOURCE_PATH="$source_path" \
    FAKE_SOURCE_EXTENSION="$extension" \
    FAKE_SOURCE_TEMPORARY="0" \
    FAKE_CLIPBOARD_OUTPUT="$TEST_TMP/clipboard.txt" \
    FAKE_NOTIFICATION_OUTPUT="$TEST_TMP/notifications.txt" \
    "$UPLOAD_SCRIPT"
}

test_image_sequence_and_permissions() {
  local source_png="$TEST_TMP/source.png"
  local source_jpg="$TEST_TMP/source.jpg"
  local remote_dir="$TEST_TMP/remote-images"
  local data_dir="$TEST_TMP/data-images"
  local output
  printf 'png-data' > "$source_png"
  printf 'jpg-data' > "$source_jpg"

  output="$(run_upload image "$source_png" png "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714_1.png" "$output" "first screenshot should use sequence 1"
  /usr/bin/cmp "$source_png" "$remote_dir/20260714_1.png" || fail "uploaded PNG content should match"
  assert_eq "644" "$(/usr/bin/stat -f '%Lp' "$remote_dir/20260714_1.png")" "uploaded screenshot should be mode 0644"
  assert_eq "$output" "$(<"$TEST_TMP/clipboard.txt")" "success should copy remote path"

  output="$(run_upload image "$source_jpg" jpg "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714_2.jpg" "$output" "screenshot sequence should span extensions"
  [[ ! -e "$remote_dir/.alfred_remote_upload_20260714_1.reserve" ]] || fail "reservation should be removed after success"
  assert_eq "devbox" "$(/usr/bin/plutil -extract recent.0.host raw -o - "$data_dir/recent_hosts.json")" "success should update MRU"
}

test_file_name_collisions() {
  local source_dir="$TEST_TMP/source-files"
  local source_path="$source_dir/报告's file.txt"
  local hidden_path="$source_dir/.env"
  local extensionless_path="$source_dir/README"
  local shell_meta_path="$source_dir/literal \$(touch injected) \`echo bad\`.txt"
  local remote_dir="$TEST_TMP/remote-files"
  local data_dir="$TEST_TMP/data-files"
  local output
  mkdir -p "$source_dir"
  printf 'report' > "$source_path"
  printf 'secret' > "$hidden_path"
  printf 'readme' > "$extensionless_path"
  printf 'literal shell characters' > "$shell_meta_path"

  output="$(run_upload file "$source_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/报告's file.txt" "$output" "ordinary file should preserve its name"
  output="$(run_upload file "$source_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/报告's file_1.txt" "$output" "ordinary file collision should add suffix before extension"

  output="$(run_upload file "$hidden_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/.env" "$output" "hidden file should preserve its name"
  output="$(run_upload file "$hidden_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/.env_1" "$output" "hidden file collision should append suffix"

  output="$(run_upload file "$extensionless_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/README" "$output" "extensionless file should preserve its name"
  output="$(run_upload file "$extensionless_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/README_1" "$output" "extensionless collision should append suffix"

  output="$(FAKE_REQUIRE_SFTP=1 run_upload file "$shell_meta_path" '' "$data_dir" "$remote_dir" 100)"
  assert_eq "$remote_dir/20260714/$(basename "$shell_meta_path")" "$output" "SFTP should treat shell metacharacters as literal path characters"
  [[ ! -e "$source_dir/injected" ]] || fail "remote filename must not execute local shell syntax"
}

test_concurrent_image_reservation() {
  local source_a="$TEST_TMP/concurrent-a.png"
  local source_b="$TEST_TMP/concurrent-b.png"
  local remote_dir="$TEST_TMP/remote-concurrent"
  local output_a="$TEST_TMP/concurrent-a.out"
  local output_b="$TEST_TMP/concurrent-b.out"
  local pid_a pid_b paths
  printf 'image-a' > "$source_a"
  printf 'image-b' > "$source_b"

  run_upload image "$source_a" png "$TEST_TMP/data-concurrent-a" "$remote_dir" 100 > "$output_a" &
  pid_a=$!
  run_upload image "$source_b" png "$TEST_TMP/data-concurrent-b" "$remote_dir" 100 > "$output_b" &
  pid_b=$!
  wait "$pid_a"
  wait "$pid_b"

  paths="$(printf '%s\n%s\n' "$(<"$output_a")" "$(<"$output_b")" | /usr/bin/sort)"
  assert_eq "$remote_dir/20260714_1.png
$remote_dir/20260714_2.png" "$paths" "concurrent uploads should reserve different paths"
  assert_eq "2" "$(find "$remote_dir" -type f -name '20260714_*.png' | wc -l | tr -d ' ')" "both concurrent uploads should remain"
}

test_size_and_failure_cleanup() {
  local source_path="$TEST_TMP/large.bin"
  local remote_dir="$TEST_TMP/remote-failures"
  local data_dir="$TEST_TMP/data-failures"
  local boundary_path="$TEST_TMP/boundary.bin"
  local boundary_remote="$TEST_TMP/remote-boundary"
  local outside_sentinel="$TEST_TMP/outside-sentinel"
  local overflow_error
  /bin/dd if=/dev/zero of="$boundary_path" bs=1048576 count=1 2>/dev/null
  /bin/dd if=/dev/zero of="$source_path" bs=1048576 count=2 2>/dev/null
  printf 'original' > "$TEST_TMP/clipboard.txt"

  run_upload file "$boundary_path" '' "$TEST_TMP/data-boundary" "$boundary_remote" 1 >/dev/null
  [[ -f "$boundary_remote/20260714/boundary.bin" ]] || fail "file exactly at max_size_mb should succeed"
  printf 'original' > "$TEST_TMP/clipboard.txt"

  if overflow_error="$(run_upload file "$boundary_path" '' "$data_dir" "$remote_dir" 999999999999999999 2>&1)"; then
    fail "overflowing max_size_mb should be rejected"
  fi
  [[ "$overflow_error" == *"max_size_mb"* ]] || fail "overflowing max_size_mb should report a configuration error"

  if run_upload file "$source_path" '' "$data_dir" "$remote_dir"$'\tbad' 100 >/dev/null 2>&1; then
    fail "upload should reject control characters in remote_dir"
  fi

  if run_upload file "$source_path" '' "$data_dir" "$remote_dir" 1 >/dev/null 2>&1; then
    fail "oversized file should fail"
  fi
  assert_eq "original" "$(<"$TEST_TMP/clipboard.txt")" "size failure should preserve clipboard"
  [[ ! -d "$remote_dir" ]] || fail "size failure should not create remote directory"

  printf 'small' > "$source_path"
  if ALFRED_REMOTE_UPLOAD_DATE='../escape' run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "invalid upload date override should be rejected"
  fi
  [[ ! -d "$remote_dir" ]] || fail "invalid upload date should not create remote files"

  printf 'sentinel' > "$outside_sentinel"
  if FAKE_TAMPER_RESERVE_PATH="$outside_sentinel" run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "reserve path outside remote_dir should fail the upload"
  fi
  assert_eq "sentinel" "$(<"$outside_sentinel")" "invalid reserve response must not overwrite files outside remote_dir"

  if FAKE_SSH_FAIL=1 run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "SSH reserve failure should fail the upload"
  fi
  [[ ! -d "$remote_dir" ]] || fail "SSH reserve failure should not create remote files"

  if FAKE_RESERVE_CHMOD_FAIL=1 run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "reserve chmod failure should fail the upload"
  fi
  assert_eq "0" "$(find "$remote_dir" -type f 2>/dev/null | wc -l | tr -d ' ')" "reserve chmod failure should clean placeholders and reservations"
  if FAKE_RESERVE_CHMOD_FAIL=1 run_upload file "$source_path" '' "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "ordinary file reserve chmod failure should fail the upload"
  fi
  assert_eq "0" "$(find "$remote_dir" -type f 2>/dev/null | wc -l | tr -d ' ')" "ordinary reserve chmod failure should clean its placeholder"

  if FAKE_SCP_FAIL=1 run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "SCP failure should fail the upload"
  fi
  assert_eq "original" "$(<"$TEST_TMP/clipboard.txt")" "SCP failure should preserve clipboard"
  assert_eq "0" "$(find "$remote_dir" -type f 2>/dev/null | wc -l | tr -d ' ')" "SCP failure should clean placeholders"

  if FAKE_SSH_FAIL_COMMIT=1 run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "chmod failure should fail the upload"
  fi
  assert_eq "0" "$(find "$remote_dir" -type f 2>/dev/null | wc -l | tr -d ' ')" "chmod failure should clean partial files"
}

test_pbcopy_failure_keeps_remote_file_without_mru() {
  local source_path="$TEST_TMP/pbcopy.png"
  local remote_dir="$TEST_TMP/remote-pbcopy"
  local data_dir="$TEST_TMP/data-pbcopy"
  printf 'small' > "$source_path"
  printf 'original' > "$TEST_TMP/clipboard.txt"

  if FAKE_PBCOPY_FAIL=1 run_upload image "$source_path" png "$data_dir" "$remote_dir" 100 >/dev/null 2>&1; then
    fail "pbcopy failure should fail the workflow"
  fi
  [[ -f "$remote_dir/20260714_1.png" ]] || fail "pbcopy failure should keep completed remote upload"
  assert_eq "original" "$(<"$TEST_TMP/clipboard.txt")" "pbcopy failure should preserve clipboard"
  [[ ! -e "$data_dir/recent_hosts.json" ]] || fail "pbcopy failure should not update MRU"
}

test_workflow_manifest_and_package() {
  local artifact="$TEST_TMP/Remote Upload.alfredworkflow"
  [[ -f "$INFO_PLIST" ]] || fail "workflow/info.plist is missing"
  /usr/bin/plutil -lint "$INFO_PLIST" >/dev/null || fail "workflow/info.plist is invalid"
  /usr/bin/python3 - "$INFO_PLIST" <<'PY'
import plistlib
import sys

with open(sys.argv[1], "rb") as source:
    workflow = plistlib.load(source)

assert workflow["name"] == "Remote Upload"
assert workflow["bundleid"] == "com.mcoder2014.life-tools.alfred-remote-upload"
configuration = {item["variable"]: item for item in workflow["userconfigurationconfig"]}
assert configuration["hosts_json"]["config"]["required"] is True
assert configuration["max_size_mb"]["config"]["default"] == "100"

objects = {item["uid"]: item for item in workflow["objects"]}
filters = [item for item in objects.values() if item["type"] == "alfred.workflow.input.scriptfilter"]
assert len(filters) == 1
assert filters[0]["config"]["keyword"] == "up"
assert filters[0]["config"]["type"] == 8
assert filters[0]["config"]["scriptfile"] == "scripts/list_hosts.sh"
connections = workflow["connections"][filters[0]["uid"]]
assert len(connections) == 1
action = objects[connections[0]["destinationuid"]]
assert action["type"] == "alfred.workflow.action.script"
assert action["config"]["type"] == 8
assert action["config"]["scriptfile"] == "scripts/upload.sh"
PY

  [[ -x "$BUILD_SCRIPT" ]] || fail "build.sh is missing or not executable"
  "$BUILD_SCRIPT" "$artifact" >/dev/null
  /usr/bin/unzip -t "$artifact" >/dev/null || fail "workflow package is invalid"
  /usr/bin/unzip -Z1 "$artifact" | /usr/bin/grep -qx 'info.plist' || fail "package must contain info.plist at root"
  if /usr/bin/unzip -Z1 "$artifact" | /usr/bin/grep -Eq '(^|/)(prefs\.plist|recent_hosts\.json)$'; then
    fail "package must not contain user configuration or MRU state"
  fi
}

test_valid_config_and_filter
test_mru_sort_and_corrupt_state_fallback
test_config_validation
test_clipboard_type_policy
test_clipboard_file_validation
test_image_sequence_and_permissions
test_file_name_collisions
test_concurrent_image_reservation
test_size_and_failure_cleanup
test_pbcopy_failure_keeps_remote_file_without_mru
test_workflow_manifest_and_package

echo "PASS: Alfred remote upload tests"
