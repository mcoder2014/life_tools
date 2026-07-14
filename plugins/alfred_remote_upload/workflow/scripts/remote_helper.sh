#!/bin/sh

set -eu

case "${action:-}" in
  reserve)
    if [ "$remote_dir" = "/" ]; then
      remote_base=""
    else
      remote_base="$remote_dir"
    fi
    if [ "$source_kind" = "image" ]; then
      mkdir -p "$remote_dir"
      sequence=1
      while [ "$sequence" -le 999999 ]; do
        reservation_path="$remote_base/.alfred_remote_upload_${upload_date}_${sequence}.reserve"
        if (set -C; : > "$reservation_path") 2>/dev/null; then
          existing=false
          for existing_path in "$remote_base/${upload_date}_${sequence}".*; do
            if [ -e "$existing_path" ]; then
              existing=true
              break
            fi
          done
          if [ "$existing" = true ]; then
            rm -f "$reservation_path"
            sequence=$((sequence + 1))
            continue
          fi

          remote_path="$remote_base/${upload_date}_${sequence}.${source_extension}"
          if (set -C; : > "$remote_path") 2>/dev/null; then
            if ! chmod 0600 "$remote_path"; then
              rm -f "$remote_path" "$reservation_path" || true
              echo "无法设置截图占位文件权限" >&2
              exit 1
            fi
            printf 'ARU1\037%s\037%s\n' "$remote_path" "$reservation_path"
            exit 0
          fi
          rm -f "$reservation_path"
        fi
        sequence=$((sequence + 1))
      done
      echo "无法为当天截图分配文件名" >&2
      exit 1
    fi

    target_dir="$remote_base/$upload_date"
    mkdir -p "$target_dir"
    stem="$source_name"
    extension=""
    case "$source_name" in
      .*)
        case "${source_name#?}" in
          *.*)
            stem="${source_name%.*}"
            extension=".${source_name##*.}"
            ;;
        esac
        ;;
      *.*)
        stem="${source_name%.*}"
        extension=".${source_name##*.}"
        ;;
    esac

    suffix=0
    while [ "$suffix" -le 999999 ]; do
      if [ "$suffix" -eq 0 ]; then
        candidate="$target_dir/$source_name"
      else
        candidate="$target_dir/${stem}_${suffix}${extension}"
      fi
      if (set -C; : > "$candidate") 2>/dev/null; then
        if ! chmod 0600 "$candidate"; then
          rm -f "$candidate" || true
          echo "无法设置普通文件占位权限" >&2
          exit 1
        fi
        printf 'ARU1\037%s\037\n' "$candidate"
        exit 0
      fi
      suffix=$((suffix + 1))
    done
    echo "无法为普通文件分配文件名" >&2
    exit 1
    ;;
  commit)
    chmod 0644 "$remote_path"
    if [ -n "${reservation_path:-}" ]; then
      rm -f "$reservation_path"
    fi
    ;;
  cleanup)
    if [ -n "${remote_path:-}" ]; then
      rm -f "$remote_path"
    fi
    if [ -n "${reservation_path:-}" ]; then
      rm -f "$reservation_path"
    fi
    ;;
  *)
    echo "未知远端操作" >&2
    exit 1
    ;;
esac
