#!/bin/bash

mkdir -p output

# 文件批量重命名
go build -v -o ./output/renameV1 ./cli/renameV1/...

# 敏感词检测
go build -v -o ./output/check_keywords ./cli/check_keywords/...

# retry 包装壳子
go build -v -o ./output/retry_exec ./cli/retry_exec/...

# Codex hook 飞书提醒
go build -v -o ./output/codex_hook_notify ./cli/codex_hook_notify/...

# HTTP 文件快速分享
go build -v -o ./output/file_share ./cli/file_share/...
