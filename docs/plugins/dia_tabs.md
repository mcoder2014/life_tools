# Dia 网页标题与标签

Dia Tabs 保存打开网页的原始 `document.title`，在扩展控制页和 Alfred 中共用同一份数据。Dia 中的自定义标签名不会被当作网页标题；选择已有标签时按标签 ID 激活，不按 URL 查找，也不会另开一个相同页面。

适用环境：macOS、支持 MV3 扩展与 Native Messaging 的 Dia、Alfred 5（Workflow 需要 Powerpack）、Python 3.9+。运行时只使用浏览器 API、Python 标准库和 macOS `pbcopy`；Node 24 仅用于开发测试。

## 能做什么

| 入口 | 操作 | 结果 |
|---|---|---|
| 扩展控制页 | 点击标签标题或 URL | 激活该已有标签及其窗口 |
| 扩展控制页 | 同步已打开标签 | 读取当前 profile 已加载网页的原始标题，包含后台标签；显示本轮成功、无法读取和跳过数量 |
| 扩展控制页 | 单击行、Cmd/Ctrl 切换、Shift 连选 | 选择当前列表中的一项或多项 |
| 扩展控制页 | 复制所选 URL / Markdown | 按列表顺序输出，一项一行；同 URL 的多个标签保留多项 |
| Alfred | `dia list` | 汇总已连接扩展的各 profile 当前网页 |
| Alfred | `dia list 关键词` | 按原始标题和 URL 搜索，不区分大小写 |
| Alfred | 回车 | 精确激活这条结果对应的标签 |
| Alfred | Cmd+回车 / Alt+回车 | 分别复制 URL / Markdown |

控制页只显示当前 profile。Alfred 每条结果附连接短标识以区分 profile；只展示能够实时响应的连接，不保存历史列表。

控制页的搜索、复制和结果反馈集中在列表上方，长列表的表头保持可见。标题和 URL 以单行省略显示，悬停可查看完整内容；复制仍使用完整原始值。状态标签同时显示标题采集结果、页面当前状态和窗口编号，浅色与系统深色外观均可使用。

## 安装

先构建安装文件，再将运行文件安装到稳定目录。不要从临时 worktree 直接注册 native host。

```bash
plugins/dia_tabs/build.sh
plugins/dia_tabs/install.sh
```

安装器默认使用当前 `python3` 的绝对路径生成 launcher。Alfred 和 Dia 无需加载交互 shell 配置。也可以明确指定 Python 和 Alfred 的 workflows 父目录：

```bash
PYTHON=/absolute/path/to/python3 plugins/dia_tabs/install.sh \
  --python /absolute/path/to/python3 \
  --workflow-dir "$HOME/Library/Application Support/Alfred/Alfred.alfredpreferences/workflows"
```

若 Alfred 设置了同步目录，`--workflow-dir` 应指向同步目录下的 `Alfred.alfredpreferences/workflows`。只会创建或更新 `user.workflow.life-tools-dia-tabs`，不会替换其他 Workflow。省略此参数时，在稳定 runtime 安装后双击构建生成的 `.alfredworkflow` 导入。

安装器不运行 sudo、不重启 Dia、不自动打开浏览器页面，也不自动加载扩展。

| 路径 | 用途 |
|---|---|
| `~/.local/share/life-tools-dia-tabs/extension` | 在 Dia 中加载的解压扩展目录 |
| `~/.local/share/life-tools-dia-tabs/native-host` | Native Messaging launcher，使用固定 Python 绝对路径 |
| `~/.local/share/life-tools-dia-tabs/dia-tabs` | Alfred 调用的命令入口 |
| `~/.local/share/life-tools-dia-tabs/run/*.sock` | 活跃连接的私有 Unix socket |
| `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.mcoder2014.life_tools.dia_tabs.json` | 默认 native host 注册文件，已在 Dia 1.55.1 验证 |

2026-11-01 在本机 Dia 1.55.1 实测：注册到 `Dia/User Data/NativeMessagingHosts` 时返回 `Specified native messaging host not found`；改为上述 `Google/Chrome/NativeMessagingHosts` 后成功连接，Alfred 可以实时列出网页。因此默认采用本机已验证的位置。[AdGuard 的浏览器兼容性配置](https://github.com/AdguardTeam/CompatibilityIssues/blob/master/mac/browsers.json) 将 Dia 列为前一种目录，与本机结果不一致；以目标 Dia 的实际连通结果为准。不同 Dia 版本若使用其他目录，用 `--native-host-dir` 明确覆盖。安装器每次只写一个目录，不向多个浏览器目录散装 manifest。

```bash
plugins/dia_tabs/install.sh \
  --native-host-dir "$HOME/path/to/NativeMessagingHosts"
```

在 Dia 的扩展管理页打开开发者模式，加载安装器输出的稳定 `extension` 目录。固定扩展 ID 为 `mcalepoloegbbciipaklojhcapcghdpd`；native manifest 的 `allowed_origins` 仅允许此 ID。点击工具栏扩展按钮打开控制页，检查“Alfred 连接就绪”；必要时点击“重新连接”。连接失败时，状态下方会显示后端返回的错误详情，恢复后自动清除；连接不可用时，控制页列表和复制仍可独立使用。

默认 `--prefix` 为上述稳定目录。自定义时路径必须足够短，完整 socket 路径（UTF-8 字节）小于 104 字节；安装器会在写入前检查。使用自定义 prefix 时，同时指定 `--workflow-dir` 以写入正确 runtime 路径，或在 Alfred Workflow 的变量 `DIA_TABS_HOME` 中填写该绝对路径。

## 标题何时更新

自动采集只在标签是所在窗口的 active 标签、且加载完成时读取顶层网页 DOM；首次安装不会遍历访问所有后台网页。多个窗口各自选中的标签都可以采集。缓存存于 `chrome.storage.session`，浏览器重启或扩展重载后重新采集，不建立网页历史数据库。

点击“同步已打开标签”会尝试读取本 profile 已打开 HTTP/HTTPS 网页的 `document.title`，包括后台及浏览器标记冻结（frozen）的标签，不要求页面可见或窗口聚焦；不主动切换、加载或重载页面。加载中、已释放（discarded）、尚未加载或读取超过 5 秒的页面会跳过，受限页面计为“无法读取”；采集期间导航、关闭或被更新采样替代的结果也计为跳过。手动读取最多占用 3 个未结束的执行，与自动前台探测合计最多 4 个，同一标签不重复发起受限读取。真正冻结的页面可能不执行脚本而超时；超时执行仍占用额度，直到浏览器实际结束。已有 3 个受限执行未结束时，其余手动读取计为额度不足并跳过，不保证所有标签都实际注入或同步成功。单页读取失败或超时保留同 URL 的已采集标题；真正导航仍清空。完成后刷新列表并保留仍存在的选择，Alfred 后续查询共用更新后的标题；“刷新列表”保持原有自动采集规则。

`frozen` 来自浏览器 Tabs API，扩展不会主动冻结页面。2026-11-01 在 Dia 1.55.1 实测存在前台页面可正常交互，但 API 仍报告 `frozen:true` 的情况。自动采集仅对当前聚焦窗口内的 active、complete、未释放页面进行最多 5 秒的前台探测；DOM 必须返回同 URL、complete、visible，返回后仍满足前台条件，才接受原始标题。失焦、切走或探测失败保留已有标题；真正导航仍清空。窗口重新获得焦点时补采当前标签，自动采集不探测后台冻结页面；显式手动同步可尝试读取。

| 状态 | 列表显示 | 复制 Markdown 的链接文字 |
|---|---|---|
| 已采集原始标题 | 网页原始标题 | 原始标题 |
| 还未访问 | 待访问 | URL |
| 网页原始标题为空 | 无标题 | URL |
| 受限页面或读取失败 | 无法读取页面标题 | URL |

| 页面变化 | 标题处理 |
|---|---|
| URL 真正改变或正常重载（含同 URL 重载） | 立即作废旧标题；后台导航同样清空 |
| 重复上报相同 URL、浏览器自定义标签名变化 | 不直接清空或使用浏览器标签名；active 且加载完成时重新读取 DOM |
| 同 URL 页面冻结、释放或尚未加载，可能同时上报 loading | 保留已采集标题，拒绝跨越该状态的旧采样结果 |
| 解除冻结、恢复已释放页面或窗口恢复前台 | 满足采集条件时补采；后台标签继续保留已采集标题 |
| active 页面修改 DOM title | 首次变化立即通知重新读取；150 毫秒内的后续变化合并为一次通知，不传递标题内容 |

这里只根据 DOM 标题、active 和页面生命周期判断是否更新，不能严格判定每次标题变化是否由用户操作引起。极快切走时，尚未完成的自动采样会被拒绝，变化后的标题可能要等下次激活才采集。关闭标签后从列表移除；关闭后的延迟采集响应不会恢复旧记录。

仅列出 HTTP/HTTPS 网页。不支持隐私窗口，不读取 `file://`、`chrome://`、`dia://` 或扩展页面。Markdown 转义反斜线和方括号，标题换行归为空格；URL 中的空格、控制字符、括号、尖括号和反斜线做百分号编码，保留已有百分号编码。

同步完成后显示本轮跳过原因及数量：已释放、浏览器标记冻结、加载中、尚未加载、状态未知、页面已变化、已关闭、读取超时或先前读取尚未结束。列表状态列显示浏览器报告的状态；“浏览器标记冻结”可与已记录标题同时显示，不能据此判定页面无法交互。浏览器没有提供的状态显示为未知，不推断为加载完成。诊断只用于当前展示，不单独保存或记录网页 URL。

## 本机桥接与排障

每个扩展 native 连接创建一个随机 socket，目录权限 `0700`、socket 权限 `0600`。host 只转发当前请求，不保存网页快照；标准输入断开即移除自身 socket。崩溃遗留的 socket 不会被当作历史数据展示。最多并发 32 个请求，单消息上限 1 MiB，默认 host 响应超时 2 秒；Alfred 并行查询连接，忽略不可用连接。

| 现象 | 检查与处理 |
|---|---|
| 控制页正常，Alfred 显示桥接未连接 | 检查 native manifest 目录与 `path`；确认安装时指定的 Python 仍存在，再点“重新连接” |
| 扩展显示 native host 未找到 | 核对注册目录是否适用于本机 Dia；不要同时向多个目录重复注册 |
| Alfred 查询不到某个 profile | 在该 profile 启用扩展并确认桥接连接；隐私窗口不支持 |
| 结果显示“待访问” | 点击“同步已打开标签”；加载中、已释放或冻结的页面需正常打开并加载完成后再试 |
| 回车后标签未激活 | 标签可能已关闭，或浏览器会话已变化；重新执行 `dia list`，不会用 URL 新开标签兜底 |
| 复制失败 | 查看 Alfred Workflow debugger；`pbcopy` 失败返回非零状态，不报告复制成功 |
| 移动或删除了安装时的 Python | 使用新的 `--python` 路径重新运行安装器 |

可以在终端只读查询桥接，不需要启动 Alfred：

```bash
"$HOME/.local/share/life-tools-dia-tabs/dia-tabs" list 'list'
```

更新时重新构建和运行安装器，再在 Dia 扩展管理页重新加载扩展。卸载时先在 Dia 移除该扩展，在 Alfred 中删除 Dia Tabs Workflow，再删除安装器输出的本工具 native manifest 和稳定安装目录。只删除 `com.mcoder2014.life_tools.dia_tabs.json`，不要清空整个 NativeMessagingHosts 或 workflows 目录。`install-state.json` 记录本工具本次的安装路径，可用于核对归属。

## 构建、验证和发布

```bash
node --test plugins/dia_tabs/tests/*.test.mjs
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s plugins/dia_tabs/tests -p 'test_*.py' -v
bash -n plugins/dia_tabs/build.sh plugins/dia_tabs/install.sh plugins/dia_tabs/workflow/scripts/*.sh
plutil -lint plugins/dia_tabs/workflow/info.plist
plugins/dia_tabs/build.sh /tmp/dia-tabs-build dev
for file in /tmp/dia-tabs-build/*; do unzip -t "$file"; done
```

Python 集成测试需要在临时目录创建 Unix socket，不操作真实浏览器或系统剪贴板；复制测试拦截 `pbcopy` 并检查 stdin。自动化测试覆盖并发请求 ID 隔离、分片帧、UTF-8、超时、EOF 清理、陈旧 socket、关键词和修饰键参数、两端相同的 Markdown fixtures，以及临时安装与 zip 内容。

| 产物 | 内容 |
|---|---|
| `life_tools_dia_tabs_extension_<版本>.zip` | 解压扩展，内含根目录 `manifest.json` |
| `life_tools_dia_tabs_<版本>.alfredworkflow` | 可导入 Alfred 的工作流；需要先安装稳定 runtime |
| `life_tools_dia_tabs_source_<版本>.zip` | 扩展、本机 Python 源码、工作流、构建/安装脚本和本说明 |

构建包不包含 socket、用户配置、浏览器列表或标题缓存。解压 source 包后可直接运行其 `install.sh`。`.github/workflows/dia-tabs.yml` 独立执行 macOS 上的 Node/Python/shell/plist/包校验；PR 只上传验证包，`v*` tag 通过后附加以上 3 个版本化资产，不修改现有 Go 发布包。

离线验证不能替代 Dia/Alfred 实机验收。2026-11-01 本机 Dia 1.55.1、Alfred 5 的状态如下：

| 范围 | 实机状态 |
|---|---|
| Native 桥接与 Alfred 列表 | Chrome 用户目录注册成功；Alfred 搜索可以显示演示网页 |
| DOM 标题与控制页 | 动态标题由 Alpha 更新为包含方括号和中文的标题；点击 URL 返回同一标签；所选单项 Markdown 在真实文本框粘贴准确；关闭演示标签后列表由 1 项变为 0 项 |
| 前台冻结标记兼容 | 已在真实网页复现 active、complete、可交互而 frozen=true；更新扩展后，激活原标签即自动采集 DOM 标题，控制页显示已记录，API 仍报告 frozen=true；未刷新网页或点击手动同步 |
| 手动同步 | 测试页转到后台后修改 DOM title，控制页先保持旧标题；点击同步后更新为新标题，测试页未被激活、标签数量未增加。已释放页面的跳过原因和数量已实测，Alfred 共用缓存数量一致 |
| 待验证 | 手动重命名后动态标题、跨窗口/最小化、浏览器退出后的清理、Alfred 回车实际切换及 Cmd/Alt+回车实际复制 |

Alfred 修饰键复制已通过参数和 `pbcopy` 的离线测试；组合键的实机交互尚未确认，不能据此声称验收完成。
