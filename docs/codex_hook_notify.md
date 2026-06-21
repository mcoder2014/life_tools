# codex_hook_notify 使用说明

`codex_hook_notify` 用来接收 Codex lifecycle hook 输入，并通过飞书自定义机器人发送提醒。第一版只提醒两个事件：

1. `Stop`：Codex 本轮完成或停止；
2. `PermissionRequest`：Codex 等待用户批准命令或工具调用。

提醒失败不会阻塞 Codex。程序会向 stderr 输出错误，并写入本地日志。

## 默认路径

| 内容 | Linux | macOS |
|---|---|---|
| 可执行文件 | `/usr/local/bin/codex_hook_notify` | `/usr/local/bin/codex_hook_notify` |
| 配置文件 | `/etc/life_tools/codex_hook_notify.json` | `/etc/life_tools/codex_hook_notify.json` |
| 日志目录 | `/var/log/codex_hook_notify` | `~/Library/Logs/codex_hook_notify` |
| Codex hook 配置 | `~/.codex/hooks.json` | `~/.codex/hooks.json` |

其他系统的日志目录是 `~/.codex_hook_notify/logs`。

## 配置飞书机器人

先在飞书群里添加自定义机器人，拿到 webhook URL。不要把真实 webhook 提交到 git。

配置文件默认是：

```bash
/etc/life_tools/codex_hook_notify.json
```

配置格式：

```json
{
  "routes": [
    {
      "events": ["Stop"],
      "feishu_custom_robot_urls": [
        "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
      ]
    },
    {
      "events": ["PermissionRequest"],
      "feishu_custom_robot_urls": [
        "https://open.feishu.cn/open-apis/bot/v2/hook/yyy"
      ]
    }
  ]
}
```

一个事件可以配置多个 webhook。没有匹配 route 的事件会被静默跳过。

## Linux 安装

在仓库根目录执行：

```bash
./install.sh
```

脚本会做这些事：

1. 构建 `output/codex_hook_notify`；
2. 安装到 `/usr/local/bin/codex_hook_notify`；
3. 如果 `/etc/life_tools/codex_hook_notify.json` 不存在，安装示例配置；
4. 创建 `/var/log/codex_hook_notify`，并把目录 owner 设置为当前用户。

填写真实 webhook 后，安装全局 Codex hook：

```bash
./install.sh --install-codex-hook
```

## macOS 安装

在仓库根目录执行：

```bash
./install.sh
```

脚本会把程序安装到 `/usr/local/bin/codex_hook_notify`，配置仍放在 `/etc/life_tools/codex_hook_notify.json`。日志目录会创建在：

```bash
~/Library/Logs/codex_hook_notify
```

填写真实 webhook 后，安装全局 Codex hook：

```bash
./install.sh --install-codex-hook
```

macOS 上如果 `/usr/local/bin` 或 `/etc/life_tools` 需要管理员权限，脚本会通过 `sudo` 请求权限。

## 验证

先做一次手工 `Stop` 事件测试：

```bash
printf '%s\n' '{"hook_event_name":"Stop","session_id":"manual-stop-test","cwd":"/tmp","model":"manual","last_assistant_message":"codex_hook_notify Stop test"}' \
  | /usr/local/bin/codex_hook_notify --config /etc/life_tools/codex_hook_notify.json
```

再做一次 `PermissionRequest` 事件测试：

```bash
printf '%s\n' '{"hook_event_name":"PermissionRequest","session_id":"manual-permission-test","cwd":"/tmp","tool_name":"manual","permission_request":{"reason":"codex_hook_notify PermissionRequest test","command":"manual command"}}' \
  | /usr/local/bin/codex_hook_notify --config /etc/life_tools/codex_hook_notify.json
```

收到飞书消息后，再检查日志。

Linux：

```bash
ls -lt /var/log/codex_hook_notify
```

macOS：

```bash
ls -lt ~/Library/Logs/codex_hook_notify
```

最后启动一次 Codex，让它自然触发 `Stop`。如果 Codex 第一次提示是否信任这个 hook，确认信任后再测一次。

## 卸载 hook

如果只想停用提醒，编辑：

```bash
~/.codex/hooks.json
```

删除 `Stop` 和 `PermissionRequest` 中命令为下面内容的 hook：

```bash
/usr/local/bin/codex_hook_notify --config /etc/life_tools/codex_hook_notify.json
```

不要删除其他 hook，避免影响已有 Codex 集成。
