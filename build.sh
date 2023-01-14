#!/bin/bash

mkdir -p bin

# 文件批量重命名
go build -v -o ./bin/renameV1 ./renameV1/...

# 敏感词检测
go build -v -o ./bin/check_keywords ./save_work/...