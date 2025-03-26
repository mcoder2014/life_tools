# 简介

咸鱼程序员在工作之余（如果有的话）可能用到的小工具。

## 哪些工具？

## retry_exec

一个简单的重试工具，可以套在 crontab 等地方，避免程序单次执行失败。

![使用方法](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261022_98943128e13fed75.jpg)

这个程序有什么能力呢？
1. 配置后使用简单，`retry_exec [your command] [your command args] [your command args] ...` 只要在你的命令执行前加上`retry_exec` 即可。
2. 帮你重试，重试次数可以自定义；
3. 重试时逐渐延长间隔时间，每次延长 2 倍，最小间隔时间、最大间隔时间可以自己设置；
4. 如果程序最终执行失败，可以通过飞书自定义机器人、企业微信自定义机器人等渠道给自己发送一个提醒

![飞书提醒](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261056_eba2bb5ee530ca60.jpg)

![企微提醒](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261058_42e17078663144a3.jpg)


**配置**

1. 将 sample/life_tools 下的示例配置文件放在在`/etc/life_tools/retry_exec.json`这里，确保有读权限；
`sudo chmod a+r /etc/life_tools/retry_exec.json`
2. 提前创建好日志路径，确保有读写权限 `mkdir -p /var/log/retry_exec`;
3. 然后就可以正常使用啦；


![使用照片](https://mcoder-image-storage.oss-cn-hangzhou.aliyuncs.com/image/25261103_2f53b1735abdb443.jpg)


## renameV1

命令行批量重命名文件名的工具。我在家里有一台 NAS，安装了 emby，但每次下载动漫时，因为文件名命名不规范，导致 emby 无法识别，修改文件名过于繁琐，
所以实现了一个粗糙版本的文件名批量命名工具，根据顺序排序重命名。

构建方式，需要先配置 golang 的运行环境，在工程根路径下执行 `./build.sh` 命令。

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