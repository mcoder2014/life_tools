# Ulanzi D200X 命令执行器安装说明

本文说明如何在 macOS 上构建、安装、升级和验证 `command_executor`。插件只支持 Ulanzi D200X 普通按键，不支持 Windows 或旋钮。

## 1. 运行要求

| 项目 | 要求 |
|---|---|
| 操作系统 | macOS 10.15 或更高 |
| 设备 | Ulanzi D200X |
| Ulanzi Studio | 3.0.11 或更高 |
| Node.js | 构建时使用 Node.js 20 或更高 |
| 插件目录名 | `com.ulanzi.commandexecutor.ulanziPlugin` |

本机实测环境是 macOS 14.7.8、Ulanzi Studio 3.1.9 和 D200X。插件已经安装并由实体按键执行成功。

## 2. 获取安装包

### 2.1 从 GitHub Release 下载

优先从 [GitHub Releases](https://github.com/mcoder2014/life_tools/releases) 下载与版本 tag 同名的安装包：

```text
life_tools_ulanzi_d200x_command_executor_<tag>.zip
```

例如，发布版本是 `v0.0.8` 时，文件名是：

```text
life_tools_ulanzi_d200x_command_executor_v0.0.8.zip
```

也可以使用 GitHub CLI 下载：

```bash
tag="v0.0.8"
gh release download "$tag" \
  --pattern "life_tools_ulanzi_d200x_command_executor_${tag}.zip"
```

已经发布的历史版本不会因发布流程更新而自动补齐资产。如果对应 Release 没有该 zip，请使用包含此发布逻辑的新版本，或按下一节从源码构建。

### 2.2 从源码构建

在仓库根目录执行：

```bash
cd plugins/unlanzi_d200x/command_executor
npm ci
npm test
./build.sh
```

构建成功后生成：

```text
output/com.ulanzi.commandexecutor.ulanziPlugin/
output/life_tools_ulanzi_d200x_command_executor.zip
```

安装前可以再次校验目录包：

```bash
node scripts/validate-package.mjs \
  output/com.ulanzi.commandexecutor.ulanziPlugin
unzip -t output/life_tools_ulanzi_d200x_command_executor.zip
```

## 3. 安装

先完全退出 Ulanzi Studio。目标目录是：

```text
~/Library/Application Support/Ulanzi/UlanziDeck/Plugins
```

以下两种安装命令只用于目标插件目录不存在的首次安装。如果同名目录已经存在，先按“升级与回滚”移动旧目录；不要直接合并覆盖，否则旧版本残留文件可能继续留在安装包中。

### 3.1 安装构建目录

在 `plugins/unlanzi_d200x/command_executor` 下执行：

```bash
plugin_root="$HOME/Library/Application Support/Ulanzi/UlanziDeck/Plugins"
plugin_name="com.ulanzi.commandexecutor.ulanziPlugin"
mkdir -p "$plugin_root"
/usr/bin/ditto "output/$plugin_name" "$plugin_root/$plugin_name"
```

`ditto` 会把 `.ulanziPlugin` 目录内容复制到目标位置。不要复制 `output/` 本身，也不要把 `node_modules/` 放入插件目录。

### 3.2 安装 zip

zip 的最外层已经是 `.ulanziPlugin` 目录，可直接解压到插件根目录：

```bash
plugin_root="$HOME/Library/Application Support/Ulanzi/UlanziDeck/Plugins"
archive="life_tools_ulanzi_d200x_command_executor_v0.0.8.zip"
mkdir -p "$plugin_root"
/usr/bin/ditto -x -k \
  "$archive" \
  "$plugin_root"
```

从源码构建时，把 `archive` 改为 `output/life_tools_ulanzi_d200x_command_executor.zip`。

安装后应存在：

```text
~/Library/Application Support/Ulanzi/UlanziDeck/Plugins/
└── com.ulanzi.commandexecutor.ulanziPlugin/
    ├── manifest.json
    ├── dist/app.js
    ├── plugin/
    ├── property-inspector/
    └── libs/
```

## 4. 升级与回滚

升级前先退出 Studio，并把当前目录移动为带时间戳的备份：

```bash
plugin_root="$HOME/Library/Application Support/Ulanzi/UlanziDeck/Plugins"
plugin_name="com.ulanzi.commandexecutor.ulanziPlugin"
backup_name="$plugin_name.backup-$(date +%Y%m%d-%H%M%S)"
mv "$plugin_root/$plugin_name" "$plugin_root/$backup_name"
/usr/bin/ditto "output/$plugin_name" "$plugin_root/$plugin_name"
```

如果新版本异常，退出 Studio，把新目录移走，再把备份目录改回 `com.ulanzi.commandexecutor.ulanziPlugin`。不要删除其他插件目录。

## 5. 启动与验收

正常启动 Studio，或在需要日志和 Property Inspector 调试时执行：

```bash
open "/Applications/Ulanzi Studio.app" --args --log --webRemoteDebug
```

按以下顺序验收：

1. D200X 在 Studio 中显示“已连接”。
2. 右侧插件列表出现“命令执行器”，其下有“执行命令”。
3. 把“执行命令”拖到普通按键，选中按键后能看到标题、命令、工作目录、环境变量和执行预览。
4. 填入 `pwd` 之类的安全命令，等待约 200 ms 保存配置。
5. 按实体 D200X 按键，确认按键出现运行/成功状态，并核对命令结果。
6. 完全重启 Studio，确认每个按键仍恢复自己的配置。

![Studio 已加载命令执行器](assets/command_executor/installed-plugin.jpg)

## 6. 日志与调试

本机实测插件日志目录是：

```text
~/Library/Application Support/Ulanzi/UlanziDeck/logs/
└── com.ulanzi.ulanzistudio.commandexecutor/
```

查找最新日志：

```bash
find "$HOME/Library/Application Support/Ulanzi/UlanziDeck/logs/com.ulanzi.ulanzistudio.commandexecutor" \
  -type f \
  -name '*.log' \
  -print
```

使用 `--webRemoteDebug` 启动后，Property Inspector 调试入口监听 `127.0.0.1:9292`。主服务 WebSocket 监听 `127.0.0.1:3906`。这两个端口只用于本机调试，不要暴露到局域网或公网。

官方 SDK 会打印完整 WebSocket payload，日志可能包含命令、工作目录和环境变量。日志有助于本机排障，但不要把原文直接上传到 issue 或 PR。

## 7. 卸载

先退出 Ulanzi Studio，再把插件目录移动到备份位置：

```bash
plugin_root="$HOME/Library/Application Support/Ulanzi/UlanziDeck/Plugins"
plugin_name="com.ulanzi.commandexecutor.ulanziPlugin"
mv "$plugin_root/$plugin_name" "$plugin_root/$plugin_name.disabled"
```

重新打开 Studio 后，“命令执行器”不再出现。确认不再需要回滚后，再由用户自行处理 `.disabled` 目录。
