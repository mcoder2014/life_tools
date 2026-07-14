#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CLIPBOARD_SCRIPT="$SCRIPT_DIR/clipboard.js"
REMOTE_HELPER_SCRIPT="$SCRIPT_DIR/remote_helper.sh"
HOSTS_SCRIPT="${HOSTS_SCRIPT:-$SCRIPT_DIR/hosts.js}"
SSH_BIN="${SSH_BIN:-/usr/bin/ssh}"
SCP_BIN="${SCP_BIN:-/usr/bin/scp}"
PBCOPY_BIN="${PBCOPY_BIN:-/usr/bin/pbcopy}"
NOTIFY_BIN="${NOTIFY_BIN:-}"
SSH_OPTIONS=(-o BatchMode=yes -o NumberOfPasswordPrompts=0 -o StrictHostKeyChecking=yes -o ConnectTimeout=10)
SCP_OPTIONS=(-s "${SSH_OPTIONS[@]}")

temp_dir=""
remote_path=""
reservation_path=""
remote_pending=0

shell_quote() {
  printf "'"
  printf '%s' "$1" | /usr/bin/sed "s/'/'\"'\"'/g"
  printf "'"
}

notify() {
  local title="$1"
  local message="$2"
  if [[ -n "$NOTIFY_BIN" ]]; then
    "$NOTIFY_BIN" "$title" "$message"
    return
  fi
  ARU_NOTIFY_TITLE="$title" ARU_NOTIFY_MESSAGE="$message" \
    /usr/bin/osascript -l JavaScript -e '
      ObjC.import("Foundation");
      const env = $.NSProcessInfo.processInfo.environment;
      const app = Application.currentApplication();
      app.includeStandardAdditions = true;
      app.displayNotification(ObjC.unwrap(env.objectForKey("ARU_NOTIFY_MESSAGE")), {
        withTitle: ObjC.unwrap(env.objectForKey("ARU_NOTIFY_TITLE"))
      });
    ' >/dev/null
}

run_remote() {
  local operation="$1"
  {
    printf 'action=%s\n' "$(shell_quote "$operation")"
    printf 'source_kind=%s\n' "$(shell_quote "${source_kind:-}")"
    printf 'source_name=%s\n' "$(shell_quote "${source_name:-}")"
    printf 'source_extension=%s\n' "$(shell_quote "${source_extension:-}")"
    printf 'remote_dir=%s\n' "$(shell_quote "${upload_remote_dir:-}")"
    printf 'upload_date=%s\n' "$(shell_quote "${upload_date:-}")"
    printf 'remote_path=%s\n' "$(shell_quote "${remote_path:-}")"
    printf 'reservation_path=%s\n' "$(shell_quote "${reservation_path:-}")"
    /bin/cat "$REMOTE_HELPER_SCRIPT"
  } | "$SSH_BIN" "${SSH_OPTIONS[@]}" "$upload_host" /bin/sh
}

cleanup() {
  if [[ "$remote_pending" == "1" ]]; then
    run_remote cleanup >/dev/null 2>&1 || true
  fi
  if [[ -n "$temp_dir" ]]; then
    rm -rf "$temp_dir"
  fi
}
trap cleanup EXIT

fail_upload() {
  local message="$1"
  notify "Remote Upload 失败" "$message" >/dev/null 2>&1 || true
  echo "$message" >&2
  exit 1
}

if [[ ! "${upload_host:-}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  fail_upload "SSH Host 配置无效"
fi
if [[ "${upload_remote_dir:-}" != /* ]] || [[ "$upload_remote_dir" =~ [[:cntrl:]] ]]; then
  fail_upload "远端目录必须是无控制字符的绝对路径"
fi
mru_remote_dir="$upload_remote_dir"
upload_remote_dir="${upload_remote_dir%/}"
if [[ -z "$upload_remote_dir" ]]; then
  upload_remote_dir="/"
fi

max_size_mb="${max_size_mb:-100}"
max_safe_mib="8796093022207"
if [[ ! "$max_size_mb" =~ ^[1-9][0-9]*$ ]] ||
  (( ${#max_size_mb} > ${#max_safe_mib} )) ||
  { (( ${#max_size_mb} == ${#max_safe_mib} )) && [[ "$max_size_mb" > "$max_safe_mib" ]]; }; then
  fail_upload "max_size_mb 必须是不会导致大小计算溢出的正整数"
fi

cache_root="${alfred_workflow_cache:-${TMPDIR:-/tmp}}"
mkdir -p "$cache_root"
temp_dir="$(mktemp -d "$cache_root/upload.XXXXXX")"

if [[ -n "${CLIPBOARD_HELPER:-}" ]]; then
  if ! clipboard_result="$("$CLIPBOARD_HELPER" read "$temp_dir/clipboard")"; then
    fail_upload "剪贴板中没有可上传的单个文件或图片"
  fi
else
  if ! clipboard_result="$(/usr/bin/osascript -l JavaScript "$CLIPBOARD_SCRIPT" read "$temp_dir/clipboard")"; then
    fail_upload "剪贴板中没有可上传的单个文件或图片"
  fi
fi

IFS=$'\x1f' read -r source_kind source_path source_extension source_temporary <<< "$clipboard_result"
if [[ "$source_kind" != "file" && "$source_kind" != "image" ]]; then
  fail_upload "剪贴板数据类型无效"
fi
if [[ ! -f "$source_path" ]]; then
  fail_upload "剪贴板文件不存在或不是普通文件"
fi
if [[ "$source_path" =~ [[:cntrl:]] ]]; then
  fail_upload "暂不支持路径中包含控制字符的文件"
fi

file_size="$(/usr/bin/stat -f '%z' "$source_path")"
limit_bytes=$((max_size_mb * 1024 * 1024))
if (( file_size > limit_bytes )); then
  fail_upload "文件大小超过 ${max_size_mb} MiB 限制"
fi

source_name="$(/usr/bin/basename "$source_path")"
if [[ "$source_kind" == "image" ]]; then
  if [[ ! "$source_extension" =~ ^[a-z0-9]+$ ]]; then
    fail_upload "剪贴板图片后缀无效"
  fi
else
  source_extension=""
fi
upload_date="${ALFRED_REMOTE_UPLOAD_DATE:-$(/bin/date +%Y%m%d)}"
if [[ ! "$upload_date" =~ ^[0-9]{8}$ ]]; then
  fail_upload "上传日期格式无效"
fi

if ! reserve_result="$(run_remote reserve)"; then
  fail_upload "无法在远端创建上传路径"
fi
protocol_prefix=$'ARU1\x1f'
field_separator=$'\x1f'
if [[ "$reserve_result" == *$'\n'* || "$reserve_result" == *$'\r'* ]] ||
  [[ "$reserve_result" != "$protocol_prefix"* ]]; then
  fail_upload "远端返回的上传路径协议无效"
fi
reserve_fields="${reserve_result#"$protocol_prefix"}"
if [[ "$reserve_fields" != *"$field_separator"* ]]; then
  fail_upload "远端返回的上传路径字段不完整"
fi
remote_path="${reserve_fields%%"$field_separator"*}"
reservation_path="${reserve_fields#*"$field_separator"}"
if [[ -z "$remote_path" || "$remote_path" != /* ]] ||
  [[ "$remote_path" =~ [[:cntrl:]] ]] ||
  [[ "$reservation_path" =~ [[:cntrl:]] ]] ||
  [[ "$reservation_path" == *"$field_separator"* ]]; then
  fail_upload "远端返回的上传路径字段无效"
fi

if [[ "$upload_remote_dir" == "/" ]]; then
  remote_prefix="/"
else
  remote_prefix="$upload_remote_dir/"
fi

if [[ "$source_kind" == "image" ]]; then
  if [[ "$remote_path" != "$remote_prefix"* ]]; then
    fail_upload "远端截图路径超出配置目录"
  fi
  relative_path="${remote_path#"$remote_prefix"}"
  image_prefix="${upload_date}_"
  image_suffix=".${source_extension}"
  if [[ "$relative_path" == */* || "$relative_path" != "$image_prefix"*"$image_suffix" ]]; then
    fail_upload "远端截图文件名无效"
  fi
  image_sequence="${relative_path#"$image_prefix"}"
  image_sequence="${image_sequence%"$image_suffix"}"
  if [[ ! "$image_sequence" =~ ^[1-9][0-9]*$ ]] || (( image_sequence > 999999 )); then
    fail_upload "远端截图序号无效"
  fi
  expected_reservation="${remote_prefix}.alfred_remote_upload_${upload_date}_${image_sequence}.reserve"
  if [[ "$reservation_path" != "$expected_reservation" ]]; then
    fail_upload "远端截图 reservation 路径无效"
  fi
else
  file_prefix="${remote_prefix}${upload_date}/"
  if [[ "$remote_path" != "$file_prefix"* ]] || [[ -n "$reservation_path" ]]; then
    fail_upload "远端普通文件路径无效"
  fi
  remote_name="${remote_path#"$file_prefix"}"
  if [[ -z "$remote_name" || "$remote_name" == */* ]]; then
    fail_upload "远端普通文件名无效"
  fi
  if [[ "$remote_name" != "$source_name" ]]; then
    stem="$source_name"
    extension=""
    if [[ "$source_name" == .* && "${source_name#?}" == *.* ]] ||
      [[ "$source_name" != .* && "$source_name" == *.* ]]; then
      stem="${source_name%.*}"
      extension=".${source_name##*.}"
    fi
    collision_prefix="${stem}_"
    if [[ "$remote_name" != "$collision_prefix"*"$extension" ]]; then
      fail_upload "远端普通文件重名后缀无效"
    fi
    collision_sequence="${remote_name#"$collision_prefix"}"
    if [[ -n "$extension" ]]; then
      collision_sequence="${collision_sequence%"$extension"}"
    fi
    if [[ ! "$collision_sequence" =~ ^[1-9][0-9]*$ ]] || (( collision_sequence > 999999 )); then
      fail_upload "远端普通文件重名序号无效"
    fi
  fi
fi
remote_pending=1

if ! "$SCP_BIN" "${SCP_OPTIONS[@]}" -- "$source_path" "${upload_host}:${remote_path}"; then
  fail_upload "SCP 上传失败"
fi
if ! run_remote commit >/dev/null; then
  fail_upload "远端权限设置失败"
fi
remote_pending=0

if ! printf '%s' "$remote_path" | "$PBCOPY_BIN"; then
  fail_upload "远端文件已上传，但写入剪贴板失败：$remote_path"
fi

mru_updated=true
if ! env alfred_workflow_data="${alfred_workflow_data:-}" \
  /usr/bin/osascript -l JavaScript "$HOSTS_SCRIPT" mark-used "$upload_host" "$mru_remote_dir" >/dev/null; then
  mru_updated=false
fi

if [[ "$mru_updated" == "true" ]]; then
  notify "Remote Upload 完成" "$remote_path" >/dev/null 2>&1 || true
else
  notify "Remote Upload 完成" "$remote_path（最近使用排序未更新）" >/dev/null 2>&1 || true
fi
printf '%s\n' "$remote_path"
