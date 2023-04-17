#!/bin/bash

mkdir -p output

# 文件批量重命名
go build -v -o ./output/renameV1 ./renameV1/...

# 敏感词检测
go build -v -o ./output/check_keywords ./save_work/...

go build -v -o ./output/dav ./webdav/...