# 简介

咸鱼程序员在工作之余（如果有的话）可能用到的小工具。

## 安装

支持 Linux 和 macOS。仓库根目录执行下面命令，默认安装稳定工具：`renameV1`、`check_keywords`、`retry_exec`、`codex_hook_notify`、`video_subtitle`。

```bash
./install.sh
```

只安装指定工具：

```bash
./install.sh --tool retry_exec
./install.sh --tool renameV1 --tool check_keywords
./install.sh --tools retry_exec,codex_hook_notify
```

安装到自定义前缀和配置目录：

```bash
./install.sh --prefix "$HOME/.local" --config-dir "$HOME/.config/life_tools"
```

`video_subtitle` 的 Python 依赖默认不会自动安装，需要脚本安装依赖时显式执行：

```bash
./install.sh --tool video_subtitle --with-python-deps
```

`dav` 是实验工具，当前源码里有硬编码 BasicAuth，不在默认安装范围内。确实需要时再显式执行：

```bash
./install.sh --tool dav
```

完整安装说明和 AI Agent 快速安装步骤见：[docs/install.md](docs/install.md)。

## 哪些工具？

## retry_exec

一个简单的重试工具，可以套在 crontab 等地方，避免程序单次执行失败。

![使用方法](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261022_98943128e13fed75.jpg)

这个程序有什么能力呢？
1. 配置后使用简单，`retry_exec [your command] [your command args] [your command args] ...` 只要在你的命令执行前加上`retry_exec` 即可。
2. 帮你重试，重试次数可以自定义；
3. 重试时逐渐延长间隔时间，每次延长 2 倍，最小间隔时间、最大间隔时间可以自己设置；
4. 如果程序最终执行失败，可以通过飞书自定义机器人、企业微信自定义机器人等渠道给自己发送一个提醒；
5. 失败提醒会附带本次最终失败执行输出的末尾内容：优先取 `stderr`，如果 `stderr` 为空则取 `stdout`，最多 4KB，方便直接看到真实报错。

![飞书提醒](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261056_eba2bb5ee530ca60.jpg)

![企微提醒](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261058_42e17078663144a3.jpg)


**安装与配置**

```bash
./install.sh --tool retry_exec
```

脚本会安装 `retry_exec`，并在 `/etc/life_tools/retry_exec.json` 不存在时写入示例配置，还会创建 `/var/log/retry_exec`。安装后编辑配置里的提醒机器人地址和日志路径。


![使用照片](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261103_2f53b1735abdb443.jpg)


## renameV1

命令行批量重命名文件名的工具。我在家里有一台 NAS，安装了 emby，但每次下载动漫时，因为文件名命名不规范，导致 emby 无法识别，修改文件名过于繁琐，
所以实现了一个粗糙版本的文件名批量命名工具，根据顺序排序重命名。

安装方式，需要先配置 Golang 的运行环境，在工程根路径下执行：

```bash
./install.sh --tool renameV1
```

只想构建不安装时，可以执行：

```bash
./build.sh
```

### 使用方法

1. 先在需要重命名的文件夹中输出一个配置文件；
2. 修改配置文件中筛选文件的后缀名，重命名规则等，用于分别重命名外挂字幕和视频内容；
3. 更新执行程序，更新配置文件；
4. 手动修改配置文件中的文件顺序，文件名 `skip` 表示占位，用于下载的视频缺了一两集的情况，序号会跳过 1；
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

## check_keywords

程序员难免在闲暇时写点小的脚本和小工具，如果不小心引用了公司的依赖，或者用了公司的内部代码，可能会导致很严重的后果。

利用 git hook 进行敏感信息检查，检查新增的行中是否包含敏感词。比如将公司的内部 git 地址作为敏感词，
每次 git commit 时检测到新增的行中包含敏感词，就会拒绝提交。

1. 检测敏感词；
2. 检测正则表达式；

## codex_hook_notify

Codex 生命周期提醒工具。程序从 stdin 读取 Codex hook JSON，根据事件类型把提醒发送到飞书自定义机器人。当前处理：

1. `Stop`：Codex 本轮完成或停止。工具会优先读取本机 `CODEX_HOME/sessions`（默认 `~/.codex/sessions`）里的 transcript，推送最后一轮 Codex 用户可见输出；如果读不到 transcript，就回退到 hook 输入里的 `last_assistant_message`。
2. `PermissionRequest`：Codex 等待用户批准执行命令或工具。

所有通知都会带 `Machine:` 字段。配置文件顶层 `machine_name` 非空时优先使用它；未配置或为空时使用操作系统 hostname；仍取不到时显示 `unknown`。

`Stop` 内容较长时会拆成多条飞书 text 消息发送。推送内容按 Codex 输出原样发送，不做敏感信息脱敏；不要把真实 webhook 配到不可信群里。

配置文件默认路径：`/etc/life_tools/codex_hook_notify.json`。示例配置在 `sample/life_tools/codex_hook_notify.json`。完整使用说明见 `docs/codex_hook_notify.md`。

```json
{
  "machine_name": "home-nas",
  "routes": [
    {
      "events": ["Stop"],
      "feishu_custom_robot_urls": ["https://open.feishu.cn/open-apis/bot/v2/hook/xxx"]
    },
    {
      "events": ["PermissionRequest"],
      "feishu_custom_robot_urls": ["https://open.feishu.cn/open-apis/bot/v2/hook/yyy"]
    }
  ]
}
```

安装：

```bash
./install.sh --tool codex_hook_notify
```

默认安装只会构建程序、安装 `/usr/local/bin/codex_hook_notify`、创建配置样例和日志目录，不会修改 Codex 全局 hook。确认配置后，显式执行下面命令安装 `Stop` hook：

```bash
./install.sh --tool codex_hook_notify --install-codex-hook
```

`PermissionRequest` 在 approval 模式下提醒很多，默认不安装；如果旧版本已经安装过本工具的 `PermissionRequest` hook，默认安装命令会把它移除。确实需要审批提醒时再显式安装：

```bash
./install.sh --tool codex_hook_notify --install-codex-hook --with-permission-request
```

日志目录按系统选择：Linux 使用 `/var/log/codex_hook_notify`，macOS 使用 `~/Library/Logs/codex_hook_notify`，其他系统使用 `~/.codex_hook_notify/logs`。发送失败只写 stderr 和日志，程序仍返回 0，不阻塞 Codex。

## video_subtitle

给单个视频生成简体中文字幕的 Python 工具。它会抽取音频、调用火山 ASR、用 LLM 做字幕切分和翻译，并在视频同目录输出 `.zh-CN.srt`。

完整使用说明、工作流程、项目结构和排障方法见：[docs/video_subtitle.md](docs/video_subtitle.md)。

安装：

```bash
./install.sh --tool video_subtitle
```

安装 Python 依赖：

```bash
python3 -m pip install -r video_subtitle/requirements.txt
```

也可以让安装脚本顺手安装 Python 依赖：

```bash
./install.sh --tool video_subtitle --with-python-deps
```

配置文件示例在 `sample/life_tools/video_subtitle.json`，真实配置默认放到 `/etc/life_tools/video_subtitle.json`。

生成字幕：

```bash
video_subtitle --input /path/to/video.mp4
```

常用调试命令：

```bash
# 只重跑翻译，复用 ASR 和切分缓存
video_subtitle --input /path/to/video.mp4 --force-translate

# 跳过 ASR，强制从 ASR words 的 LLM 切分和翻译开始
video_subtitle --input /path/to/video.mp4 --force-split --force-translate
```
