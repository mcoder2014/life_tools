# 简介

平时生活中可能用到的小工具

## 哪些工具？

### renameV1

构建方式，需要先配置 golang 的运行环境。

```bash
mkdir bin && go build -v -o ./bin/renameV1 ./renameV1/...
```

使用方法

1. 先在需要重命名的文件夹中输出一个配置文件；
2. 修改配置文件中筛选文件的后缀名，重命名规则等；
3. 更新执行程序，更新配置文件；
4. 手动修改配置文件中的文件顺序，skip 表示占位，序号会跳过 1；
5. 执行程序，确认更新；

```bash
Usage of ./renameV1:
  -conf_file_name string
        工作配置文件的文件名称 (default "rename_v1.json")
  -create_conf int
        创建配置文件 层级数量
  -dir string
        工作路径 (default "./")
  -rename
        依据配置文件执行重命名操作
  -skip_double_check
        跳过命令行二次确认
  -update_conf
        更新配置文件
```