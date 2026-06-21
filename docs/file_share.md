# file_share 使用说明

`file_share` 用来临时分享本机文件或目录。它启动一个只读 HTTP 服务，访问者只能浏览程序启动时选择的共享范围。

## 安装

```bash
./install.sh --tool file_share
```

只构建不安装时执行：

```bash
./build.sh
```

构建产物是：

```text
output/file_share
```

## 简单用法

分享一个文件或目录：

```bash
file_share /path/to/file-or-dir
```

分享多个路径：

```bash
file_share /path/to/file /path/to/dir
```

指定监听地址：

```bash
file_share -addr 127.0.0.1:9000 /path/to/dir
```

`-addr` 默认是 `0.0.0.0:8080`。

## JSON 配置

复杂共享范围可以写到 JSON 文件：

```json
{
  "entries": [
    {
      "path": "/tmp",
      "name": "tmp"
    }
  ]
}
```

启动：

```bash
file_share -config /etc/life_tools/file_share.json
```

字段说明：

| 字段 | 必填 | 说明 |
|---|---|---|
| `entries[].path` | 是 | 要分享的文件或目录路径 |
| `entries[].name` | 否 | 页面显示名；为空时使用路径 basename |

`-config` 和命令行路径互斥，不能同时使用。JSON 里的相对路径按启动时当前工作目录解析。

## 页面能力

- 首页显示所有共享条目。
- 目录页显示名称、类型、大小和修改时间。
- 文件名默认使用浏览器原生方式打开，旁边提供下载链接。
- 目录提供 zip 下载。
- HTTP 服务只支持 `GET` 和 `HEAD`。

## 安全边界

这个工具是个人临时分享工具，不是安全网盘：

- 默认没有认证。
- 隐藏文件会显示和分享。
- 符号链接会被跟随；目录 zip 也会打包链接目标内容。
- 目录 zip 没有默认大小或文件数上限。

不要把含 `.env`、密钥、私有仓库、敏感挂载点或危险符号链接的目录暴露到公网或不可信网络。需要限制暴露范围时，优先绑定到本机或内网地址，例如：

```bash
file_share -addr 127.0.0.1:8080 /path/to/dir
```

## 验证

安装后检查命令是否存在：

```bash
command -v file_share
file_share -h
```
