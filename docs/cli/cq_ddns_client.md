# cq_ddns_client

独立的 Cloudflare DDNS 客户端，从 `home_server/client` 迁移而来。运行时直接查询公网 IP 和 Cloudflare API，不需要启动 home_server，也不读取服务端配置。

## 安装和运行

在 life_tools 仓库根目录执行，Go 版本以 `go.mod` 为准：

```bash
./install.sh --tool cq_ddns_client
```

安装后的命令为 `/usr/local/bin/cq_ddns_client`，默认配置为 `/etc/life_tools/cq_ddns_client.json`。安装只生成缺失的示例配置，权限为 `0600`，不会覆盖已有配置或自动启用服务。执行用户需要有配置读取权限。

```bash
# 编辑配置后先预检，只读取 Cloudflare 和公网 IP，输出计划变更
cq_ddns_client -dry-run

# 刷新一次，成功返回 0，任一域名失败返回 1
cq_ddns_client -once

# 常驻：立即刷新，之后每 120 秒刷新
cq_ddns_client
```

没有系统目录写权限时，可以自定义安装位置；运行时显式传入配置路径：

```bash
./install.sh --tool cq_ddns_client --prefix "$HOME/.local" --config-dir "$HOME/.config/life_tools"
"$HOME/.local/bin/cq_ddns_client" -config "$HOME/.config/life_tools/cq_ddns_client.json" -dry-run
```

## 配置

示例见 [cq_ddns_client.json](../../sample/life_tools/cq_ddns_client.json)。

| 字段 | 含义 |
|---|---|
| `cloudflare.api_token` | 对目标 Zone 有 DNS 读取、编辑权限的 API Token。 |
| `cloudflare.zone` | Cloudflare Zone ID，沿用旧字段名；不是域名。 |
| `cloudflare.proxy` | 可选 SOCKS5 地址，例如 `127.0.0.1:1080`，只用于 Cloudflare 请求。 |
| `cloudflare.debug` | 可选应用日志开关，默认 false；不会打开包含认证头的 SDK HTTP dump。 |
| `ddns_config[].domain` | 需要维护的完整域名。 |
| `ddns_config[].ip_version` | `ipv4` 对应 A，`ipv6` 对应 AAAA。 |

同一个域名可以分别配置 IPv4 和 IPv6，同域名同 IP 版本不能重复。Token、Zone、域名或 IP 版本缺失/无效时，启动失败，不发送 DNS 请求。JSON 文件按 JSON 解析，`.yaml`、`.yml` 文件按旧 YAML 解析。

Cloudflare 请求每次最多等待 20 秒，保留原 SDK 重试策略；公网 IP 查询每个地址最多等待 5 秒，并依次尝试以下服务：

| 地址类型 | 查询顺序 |
|---|---|
| IPv4 | `4.ipw.cn` → `api4.ipify.org` → `ipv4.icanhazip.com` → `v4.ident.me` |
| IPv6 | `6.ipw.cn` → `api6.ipify.org` → `ipv6.icanhazip.com` → `v6.ident.me` |

原 `ip.mcoder.cc` 私有查询入口不再是依赖。只有有效且类型正确的 IP 才能用于 DNS 更新；非 200 响应、错误页面和错误类型的 IP 会触发下一个查询源。

## DNS 更新规则

每轮先读取 Zone 中的 DNS 记录，再依次处理配置中的域名。记录按“域名＋A/AAAA 类型”匹配，避免相同域名的不同类型互相覆盖。

| 当前状态 | 行为 |
|---|---|
| 对应类型的记录不存在 | 创建记录，TTL 为 300 秒，关闭 Cloudflare 代理。 |
| IP 没有变化 | 跳过更新，包括 IPv6 等价写法。 |
| IP 发生变化 | 只变更地址，保留原 TTL、Proxied 等记录属性。 |
| 同域名同类型存在多条记录 | 该域名报错，不猜测应该更新哪条记录。 |
| 某个 IP 查询或更新失败 | 记录错误并继续其他域名；常驻模式下一轮重试。 |
| `-dry-run` | 检查一次并输出计划，不创建或更新 DNS，随后退出。 |
| SIGINT / SIGTERM | 取消正在执行的请求并正常退出。 |

日志输出到 stderr，systemd 通过 journal 收集。不输出 Token、完整配置或全 Zone 的 DNS 内容。

## 从 home_client 迁移

先保留旧二进制、旧 YAML 和 `home_client.service`，用新客户端读取同一份配置预检：

```bash
cq_ddns_client -conf /etc/home_server/client_config.yaml -dry-run
```

`-conf` 是 `-config` 的兼容别名。默认路径已改为 `/etc/life_tools/cq_ddns_client.json`，不会自动寻找旧配置。字段名和 IPv4/IPv6、SOCKS5 含义保持不变。

配置转换只改变 YAML/JSON 格式，保留所有字段。可以参照示例手工转换；下面命令需要 Python 3 和 PyYAML，并拒绝覆盖已有目标文件：

```bash
sudo python3 - <<'PY'
import json
import os
import yaml

with open('/etc/home_server/client_config.yaml') as source:
    config = yaml.safe_load(source)
os.makedirs('/etc/life_tools', exist_ok=True)
dest = '/etc/life_tools/cq_ddns_client.json'
fd = os.open(dest, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as target:
    json.dump(config, target, indent=2)
    target.write('\n')
PY
```

如果安装脚本已生成示例 JSON，先确认它仍是未填写的示例并将它另存为备份，再转换；已有真实配置应直接核对使用。转换后验证：

```bash
sudo cq_ddns_client -config /etc/life_tools/cq_ddns_client.json -dry-run
sudo cq_ddns_client -config /etc/life_tools/cq_ddns_client.json -once
```

## Linux 常驻服务和回滚

[systemd 示例](../../sample/systemd/cq_ddns_client.service) 需要 systemd 247+，使用 DynamicUser 和 LoadCredential：配置由 root 持有 `0600` 权限，systemd 将只读凭证副本交给动态服务用户。路径使用 [systemd v247 文档](https://raw.githubusercontent.com/systemd/systemd/v247/man/systemd.exec.xml) 中的 `${CREDENTIALS_DIRECTORY}` 写法。凭证中的配置必须为 JSON。

确认新配置可用后再切换。不要让两个客户端长期同时维护同一批记录：

```bash
sudo chown root:root /etc/life_tools/cq_ddns_client.json
sudo chmod 600 /etc/life_tools/cq_ddns_client.json
sudo install -m 644 sample/systemd/cq_ddns_client.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl stop home_client.service
sudo systemctl start cq_ddns_client.service
sudo journalctl -u cq_ddns_client.service -n 60 --no-pager
```

检查首次刷新和至少一个 120 秒周期都成功后，再调整开机启动：

```bash
sudo systemctl disable home_client.service
sudo systemctl enable cq_ddns_client.service
```

新服务启动或刷新失败时，立即回滚：

```bash
sudo systemctl disable --now cq_ddns_client.service
sudo systemctl enable --now home_client.service
```

## 开发验证

在测试主机上的仓库副本中执行：

```bash
go test ./...
./build.sh
go test -race ./cli/cq_ddns_client
```

自动化测试使用 loopback HTTP 服务，覆盖 JSON/YAML 配置、IP 查询兜底、A/AAAA 同名记录、创建、更新属性保留、跳过、预检不写入、失败和重复记录。实机测试使用当前配置执行 `-dry-run`、`-once`，再验证一个常驻周期。只读预检和 IP 未变化的单次执行不能证明 Cloudflare 实际写权限；创建/更新的真实网络验证应使用专门的测试域名。

### 2026-09-13 实机验证记录

目标为 SSH 别名 `pi`，实际运行 Ubuntu 24.04.4、Linux amd64、Go 1.21.12、systemd 255。此记录不代表在 ARM 硬件上运行过。

| 验证项 | 结果 |
|---|---|
| life_tools 全仓测试、根构建脚本 | 通过。 |
| 新工具发布目标构建 | Linux/macOS 的 amd64、arm64 共 4 个目标均通过；仅 Linux amd64 执行实机测试。 |
| DDNS race 检查 | 通过，包含 Cloudflare 传输断连、请求取消及后续刷新恢复。 |
| home_server 移除 client 后的构建 | 根构建脚本及 `go build ./...` 通过。 |
| 隔离安装两次 | 命令可执行，配置权限 `0600`，第二次不覆盖配置。 |
| 旧 YAML 与转换后的 JSON | 字段完全一致；真实配置的 YAML 预检和 JSON 单次刷新均返回 0。 |
| systemd 动态用户及 LoadCredential | 能读取 JSON 凭证；两轮均完成 IPv4、IPv6 检查。 |
| 120 秒刷新周期 | UTC+8 14:11:46、14:13:46 启动两轮，重启次数 0。 |
| 停止及日志 | 测试服务停止后 `Result=success`、退出码 0；Token 和认证头未出现在日志。 |

部分 IP 查询源出现超时、拒绝连接或解析失败，后续查询源成功，真实配置中的两条 DNS 地址均未发生变化。因此真实网络验证覆盖读取、地址比对和常驻运行；DNS 创建、修改与失败分支由 loopback 集成用例验证，没有通过修改现有 DNS 记录验证写权限。旧常驻服务保持运行，临时测试服务已停止。
