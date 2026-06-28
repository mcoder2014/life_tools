# file_share 使用说明

`file_share` 用来临时分享本机文件或目录。它启动一个只读 HTTP 服务，访问者只能浏览程序启动时选择的共享范围。

## 设计目标

| 目标 | 处理方式 |
|---|---|
| 快速分享本机路径 | 直接把命令行路径或 JSON 配置转换成共享条目 |
| 保持只读 | HTTP 只允许 `GET` 和 `HEAD` |
| 支持目录浏览 | 目录页用 HTML 表格展示名称、类型、大小和修改时间 |
| 支持目录打包 | `/zip/` 路由按请求实时创建 zip |
| 避免明显路径穿越 | URL 相对路径包含 `.` 或 `..` 段时直接拒绝 |

## 代码结构

| 路径 | 作用 |
|---|---|
| `cli/file_share/main.go` | CLI 参数、配置加载和 HTTP server 启动 |
| `cli/file_share/config.go` | JSON 配置、共享条目构造和路径 stat |
| `cli/file_share/server.go` | 路由、目录页渲染、文件下载、目录 zip 和访问日志 |
| `sample/life_tools/file_share.json` | 示例配置 |

## 请求流程

```mermaid
flowchart TD
    A["启动 file_share"] --> B{"是否传入 -config?"}
    B -->|是| C["读取 JSON entries"]
    B -->|否| D["读取命令行路径参数"]
    C --> E["BuildEntries 校验路径并分配 ID"]
    D --> E
    E --> F["启动 http.ListenAndServe"]
    F --> G{"请求路径"}
    G -->|"/"| H["渲染共享入口页"]
    G -->|"/browse/{id}/..."| I["浏览目录或内联打开文件"]
    G -->|"/raw/{id}/..."| J["内联返回文件"]
    G -->|"/download/{id}/..."| K["附件下载文件"]
    G -->|"/zip/{id}/..."| L["实时打包目录 zip"]
```

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
- 访问日志输出到 stdout，格式包含 `client_ip`、请求方法、路径、状态码和耗时。

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
