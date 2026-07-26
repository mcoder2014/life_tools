#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CLIPBOARD_SCRIPT="$ROOT_DIR/workflow/scripts/clipboard.js"
FIXTURE_SCRIPT="$ROOT_DIR/tests/clipboard_fixture.js"
TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/alfred-remote-upload-clipboard.XXXXXX")"
SNAPSHOT="$TEST_TMP/pasteboard.json"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

restore_clipboard() {
  local status=$?
  trap - EXIT
  if [[ -f "$SNAPSHOT" ]]; then
    if ! /usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" restore "$SNAPSHOT" >/dev/null 2>&1; then
      echo "FAIL: unable to restore clipboard snapshot" >&2
      status=1
    fi
  fi
  rm -rf "$TEST_TMP"
  exit "$status"
}
trap restore_clipboard EXIT

read_clipboard() {
  /usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" read "$TEST_TMP/output-$1"
}

assert_result() {
  local result="$1"
  local expected_kind="$2"
  local expected_extension="$3"
  IFS=$'\x1f' read -r result_kind result_path result_extension result_temporary <<< "$result"
  [[ "$result_kind" == "$expected_kind" ]] || fail "expected kind $expected_kind, got $result_kind"
  [[ "$result_extension" == "$expected_extension" ]] || fail "expected extension $expected_extension, got $result_extension"
  [[ -f "$result_path" ]] || fail "clipboard output file does not exist: $result_path"
}

/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" snapshot "$SNAPSHOT"
chmod 0600 "$SNAPSHOT"

png_path="$TEST_TMP/source.png"
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' | /usr/bin/base64 -D > "$png_path"

/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-png "$png_path" >/dev/null
png_result="$(read_clipboard png)"
assert_result "$png_result" image png
IFS=$'\x1f' read -r _ preserved_path _ _ <<< "$png_result"
/usr/bin/cmp "$png_path" "$preserved_path" || fail "PNG clipboard bytes should be preserved"

/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-tiff "$png_path" >/dev/null
tiff_result="$(read_clipboard tiff)"
assert_result "$tiff_result" image png
IFS=$'\x1f' read -r _ converted_path _ _ <<< "$tiff_result"
[[ "$(/usr/bin/xxd -p -l 8 "$converted_path")" == "89504e470d0a1a0a" ]] || fail "TIFF clipboard should be converted to PNG"

finder_file="$TEST_TMP/Finder 文件.txt"
printf 'finder file' > "$finder_file"
files_json="$(/usr/bin/python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$finder_file")"
/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-files "$files_json" >/dev/null
file_result="$(read_clipboard file)"
IFS=$'\x1f' read -r file_kind file_path file_extension file_temporary <<< "$file_result"
[[ "$file_kind" == "file" && "$file_path" == "$finder_file" ]] || fail "single Finder file should be returned unchanged"
[[ -z "$file_extension" && "$file_temporary" == "0" ]] || fail "Finder file metadata should mark a non-temporary file"

second_file="$TEST_TMP/second.txt"
printf 'second' > "$second_file"
if [[ "${ALFRED_SKIP_MULTI_FILE_PASTEBOARD_TEST:-0}" == "1" ]]; then
  echo "SKIP: hosted runner does not preserve multiple Finder URLs on NSPasteboard"
else
  files_json="$(/usr/bin/python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$finder_file" "$second_file")"
  /usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-files "$files_json" >/dev/null
  if read_clipboard multiple >/dev/null 2>&1; then
    fail "multiple Finder files should be rejected"
  fi
fi

files_json="$(/usr/bin/python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$TEST_TMP")"
/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-files "$files_json" >/dev/null
if read_clipboard directory >/dev/null 2>&1; then
  fail "Finder directory should be rejected"
fi

/usr/bin/osascript -l JavaScript "$FIXTURE_SCRIPT" set-text "plain text" >/dev/null
if read_clipboard text >/dev/null 2>&1; then
  fail "plain text clipboard should be rejected"
fi

echo "PASS: Alfred remote upload clipboard integration"
