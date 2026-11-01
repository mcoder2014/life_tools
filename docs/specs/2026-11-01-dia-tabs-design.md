# Dia Tabs 设计

状态：实现方案已确定；Dia 1.55.1 的 Native Messaging 注册目录、Alfred 实时列表及部分控制页交互已实机验证；剩余状态见使用说明。

## 目标与范围

在 Dia 中保存当前打开网页的原始 `document.title`，避免用户重命名标签后污染复制结果。扩展控制页和 Alfred 使用同一份标题数据；激活已有标签时使用标签 ID，不按 URL 查找，也不新建标签。

仅新增 `plugins/dia_tabs/` 及相关文档、CI。沿用仓库独立 Alfred 插件的打包方式，不修改现有工具、不进入 Go 构建、不读取 Dia 私有数据库。运行时只用浏览器扩展 API、Python 3 标准库和 macOS 剪贴板命令。

## 核心判断与方案比较

值得做。网页标题与浏览器自定义标签名是两种数据，必须从页面 DOM 获取前者；同 URL 的两个标签是不同对象，不能用 URL 代替标签身份。

| 方案 | 成本与限制 | 判断 |
|---|---|---|
| MV3 扩展 + Native Messaging + Unix socket + Alfred | 一个由浏览器管理生命周期的 Python 进程；注册目录已在 Dia 1.55.1 验证 | 推荐。共享一个标题源，无 TCP 监听、无额外守护服务 |
| 扩展 + localhost HTTP/WebSocket 服务 | 需要常驻服务、端口、认证、启动管理和重连 | 不采用，当前需求不需要这些概念 |
| 扩展和 Alfred 分别用浏览器 API、AppleScript | Dia 的 scripting dictionary 有 tab ID，但两端标题缓存分离；执行网页脚本还可能要求开启 Apple Events JavaScript | 不采用为主路径；只有精确激活 API 实测失败时才评估 AppleScript 补充 |

最小数据关系是“一个浏览器会话内，tabId 对应一个当前页面标题”。关闭标签就删除，浏览器重启就重新采集，不建设历史记录库。

## 组件与数据流

| 组件 | 责任 | 不承担的责任 |
|---|---|---|
| MV3 service worker | 查询标签、采集 DOM 标题、维护会话缓存、激活标签 | 不保存浏览历史、不使用 tabs.title 作为网页标题 |
| 扩展控制页 | 显示本 profile 打开的网页、选择与复制 | 不直接访问本机文件 |
| Python native host | 在浏览器 stdio 与 Unix socket 之间转发请求和响应 | 不保存标题快照、不启动 HTTP 服务 |
| Alfred 脚本 | 查询所有活跃 socket，过滤结果，发送激活请求或复制文本 | 不按 URL 猜标签、不扫描 Dia 数据库 |

控制页直接通过 `runtime.sendMessage` 请求 service worker。Alfred 经 Unix socket → native host → `connectNative` port 请求同一个 service worker。每次列表请求重新查询 `tabs.query({})`，与缓存合并，因此列表成员以当前打开标签为准。

每个 native host 连接创建一个随机命名的 socket。不同 profile 自然对应不同 host/连接；不引入中央路由进程。控制页仅展示自身 profile；Alfred 汇总所有能及时响应的连接，附连接短标识用于区分。未安装或禁用扩展的 profile 无法被此工作流列出。

## 标签与标题语义

`active` 定义为 Chrome Tabs API 的 `tab.active`：窗口内当前选中的标签，不要求窗口正处于前台。因此多个窗口可以各有一个 active 标签。这与 `activeTab` 权限不同，后者需要用户显式调用扩展，不能满足自动记录。

| 状态或事件 | 行为 |
|---|---|
| 打开的 HTTP/HTTPS 标签尚未被采集 | 显示“待访问”，保留 URL，可复制 URL、可激活 |
| 自动采集：active 且 tab.status=complete，未 discarded/frozen | 在顶层 frame 读取 `document.title`、`location.href`、`document.readyState` |
| frozen=true，但 active、complete、未 discarded，且窗口明确 focused=true | 允许有界前台探测；仅接受同 URL、DOM complete/visible 且返回后仍在前台的标题 |
| 初次启动、`onActivated`、窗口获得焦点、`onUpdated` 的 complete/title 变化，或 frozen/discarded 变为 false | 满足上述条件时重新采集，不使用事件中的 title 值；后台恢复不自动采集 |
| 非 active 标签 | 不自动采集；保留其已采集且仍对应当前 URL 的标题 |
| 点击“同步已打开标签” | 手动读取当前 profile 全部 HTTP/HTTPS 已加载网页标题，包含后台标签；不切换、加载或重载页面 |
| 手动同步遇到 loading/discarded 或后台 frozen | 跳过，保持页面状态；符合前台探测条件的 frozen 页面允许探测，单次读取超过 5 秒也跳过 |
| URL 真正改变或正常重载（含同 URL 重载） | 立即作废旧标题，增加该 tab 的采集版本；禁止旧标题与新 URL 配对 |
| 重复上报相同 URL | 只触发满足条件的重新采集，不作废已有标题 |
| 同 URL 标签 frozen/discarded/unloaded，可能同时上报 loading | 保留已采集标题，仍增加采集版本；恢复后也拒绝此前的在途结果 |
| DOM title 变化 | active 时由 `onUpdated` 的 title 事件或顶层 MutationObserver 通知触发重新读取 DOM |
| SPA 改 URL | 作废旧标题，active 且 complete 时重新读取；后续 title 事件继续更新 |
| 页面为空标题 | 显示“无标题”；Markdown 使用 URL 作为链接文本，不回退到 tabs.title |
| 受限页面或执行脚本失败 | 保留 URL，显示“无法读取页面标题”；不得伪装采集成功 |
| 标签关闭或被替换 | 删除旧 tabId 缓存；新 tabId 按新标签处理 |

控制页和 Alfred 默认只列 HTTP/HTTPS 网页。`chrome://`、`dia://`、扩展自身页面、`file://` 不进入列表；不申请 file 权限。隐私窗口明确不支持：manifest 使用 `incognito: "not_allowed"`，并在查询结果再次过滤 incognito。

`frozen` 是 Tabs API 报告值，扩展不主动冻结页面。Dia 1.55.1 已实测前台可交互页面与 `frozen:true` 并存，因此不能仅凭该标记永久拒绝采集。前台探测在注入前重新核对 tab 和窗口焦点，缺少明确 focused=true 时仍跳过；探测失败、无 DOM 结果或 hidden 不覆盖既有标题。控制页使用“浏览器标记冻结”，保留报告值，不将其解释为页面一定停止运行。

标题采集不承诺存在一个永远稳定的标题。页面可以持续修改 title；记录的是最后一次满足条件时读到的 DOM title，不能严格归因到某次用户操作。顶层 MutationObserver 只关注 title 的增删和内容变化，首次变化立即发送无标题 payload 的通知，150 毫秒窗口内的后续变化在窗口结束时合并通知；持续变化不会无限等待稳定，style/meta 变化不触发通知。新版本采样会替换已有页面的 V1 observer，避免必须重载网页。Dia 手动重命名后动态标题的实机行为仍待验证。

## 状态与并发边界

标题缓存放 `chrome.storage.session`，与浏览器会话同生命周期，跨 service worker 重启保留；不放 `storage.sync` 或磁盘 JSON。sessionId 也放在 session storage。浏览器重新启动、扩展重载或升级后重新采集，避免旧 tabId 复用导致错误匹配。

记录至少包含 `tabId, url, title, titleState`。列表输出额外包含实时 `windowId, index, active`。导航版本计数用于拒绝延迟返回，不必对外暴露。

采集前记下 tabId、URL 和版本；异步脚本返回后重新 `tabs.get`，仅在标签仍存在、仍 active、仍 complete、未 discarded/frozen、版本和 URL 均相同、DOM readyState 为 complete 时接受。前台探测可忽略仍为 true 的 frozen，但必须额外检查 DOM visible、窗口仍明确 focused=true；激活或窗口焦点变化同步推进探测版本，即使切走再返回也拒绝此前的探测结果。极快切走可能使自动采样来不及完成，下次激活再补采，不保证每次短暂访问都采到变化后的标题。显式手动同步仅放宽 active 门槛，并在注入前再次读取实时标签状态；导航、关闭和更晚采样校验不变。导航与生命周期暂停事件必须先同步使版本失效，再执行异步工作。关闭后的延迟响应不可重新插回缓存。缓存写入应串行或按 tab key 写，避免两个异步采集互相覆盖整张表。

手动同步与前台探测共享最多 4 个实际未结束的脚本读取，同一 tabId 已有受限执行时不再注入，去重和容量检查不推进采样版本；重复手动同步请求复用同一批次。普通非 frozen 自动采集保持原有语义。受限读取超过 5 秒结束等待并计为跳过；浏览器尚未结束的脚本继续占用额度，直到实际成功或失败，晚到结果只释放额度，不更新标题。导航、关闭不提前释放执行额度，初始化中的探测也不无限阻塞。成功、无法读取、跳过数量来自本轮实际采集结果，不从历史缓存推算。同步响应另含 `skipReasons` 原因计数：`discarded`、`frozen`、`loading`、`unloaded`、`unknown_status`、`stale`、`closed`、`timeout`、`capacity`；已释放、冻结优先于 status 判定。仅返回本轮汇总，不持久化诊断。

service worker 的事件监听器在模块顶层同步注册。初始化载入 session 后，重新查询当前标签并清理不存在或 URL 不符的记录；初始化期间暂存的 URL、加载和生命周期事件随后按顺序应用，不包含 title。已关闭或被替换的标签不恢复；被早期事件作废的查询快照不用于初次采样，后续事件按实时状态补采。不能只依赖 `onStartup`：休眠后重启 worker 同样要初始化。

`connectNative` 保持连接以支持 Alfred 主动请求。断线后用有限重试与 `alarms` 恢复，重试间隔不低于数秒，长期失败最多每分钟一次；不得循环立即重连。控制页应显示本机桥接是否连接。即使 native host 不可用，扩展自身列表和复制仍正常工作。

## 精确激活与请求协议

每条 Alfred 结果携带 `{connectionId, sessionId, tabId}`。connectionId 是 socket 文件名中的随机 ID；sessionId 由扩展会话产生。URL 与标题只作为展示和复制数据，不能参与查找目标。

激活顺序：验证 sessionId → `tabs.get(tabId)` → `tabs.update(tabId, {active:true})` → 用实时 windowId 调用 `windows.update(windowId, {focused:true})`。最小化窗口需要先恢复正常状态；已经最大化的窗口不要强行恢复。标签在列表显示后关闭时返回“标签已关闭”，不得通过打开 URL 兜底。移动窗口后使用新的 windowId。

Native host 名称固定为 `com.mcoder2014.life_tools.dia_tabs`。协议使用 JSON，固定版本 `v:1` 和字符串请求 `id`。只接受 `list`、`activate` 两个操作，拒绝任意脚本和任意文件路径。控制页 runtime message 与 native 请求共用此协议。

手动同步使用控制页专属 runtime 消息 `{type:"dia-tabs-sync"}`，只在验证扩展自身 sender 后处理，不加入 native/Alfred 协议；网站 content script 不可调用。

```json
{"v":1,"id":"request-uuid","op":"list"}
{"v":1,"id":"request-uuid","ok":true,"sessionId":"session-uuid","tabs":[]}
{"v":1,"id":"request-uuid","op":"activate","sessionId":"session-uuid","tabId":123}
{"v":1,"id":"request-uuid","ok":false,"error":"TAB_CLOSED"}
```

`tabs` 每项字段固定为 `{tabId:number, windowId:number, index:number, url:string, title:string, titleState:"captured"|"pending"|"unavailable", active:boolean}`。列表另附实时 `status`（缺失为 `unknown`）、`discarded` 和 `frozen`（缺失为 `null`），用于控制页解释当前状态，不改变标题缓存。空 title 且 captured 表示无标题；pending 表示未采集。成功激活响应为 `{v:1,id,ok:true}`。失败允许 `TAB_CLOSED`、`STALE_SESSION`、`BAD_REQUEST`、`INTERNAL_ERROR`，可附 `message:string`；不得把内部 traceback 发给页面。native host 可以在连接/超时时补 `BRIDGE_UNAVAILABLE`、`TIMEOUT` 错误。

Unix socket 一次连接一个请求，使用换行分隔 JSON，设置读取超时与消息上限。native stdio 按 Chrome 协议使用 4 字节本机字节序长度 + UTF-8 JSON；stdout 只能写协议，诊断走 stderr，不记录真实 title/URL。host 为并发客户端分配内部请求 ID，不能直接信任客户端 id 的全局唯一性。超时或断线必须解除等待，不遗留 pending 请求。

socket 放在用户目录的私有短路径，例如 `~/.local/share/life-tools-dia-tabs/run/<uuid>.sock`，父目录 `0700`，socket `0600`；绑定前检查完整路径满足 macOS Unix socket 路径长度限制。native host 读到 stdin EOF 即关闭 socket 并退出。Alfred 只展示当前能响应 list 的连接，忽略崩溃遗留 socket；不依赖 PID 文件或磁盘快照判断浏览器在线。普通关闭浏览器后即使有遗留文件，也不会出现旧列表。

## 控制页和 Alfred 交互

控制页使用表格，列为选择框、网页标题、URL、状态/窗口和复制 URL 操作。标题和 URL 均可点击，按 tabId 精确激活已有标签；URL 使用链接样式按钮，或在 anchor 上 preventDefault，阻止浏览器默认导航和新建标签。整行作为选择区域：普通点击单选，Ctrl/Cmd 点击切换选择，Shift 点击选择当前显示顺序中的连续区间；带修饰键点击标题或 URL 仍只执行选择，不得激活目标标签。复制按钮阻止事件冒泡。

提供“复制所选 URL”和“复制所选 Markdown”。按当前表格顺序输出，每项一行；不同 tab 即使同 URL 也保留为不同项。刷新时按 tabId 保留仍存在的选择，移除已关闭项。无已采集标题的条目使用 URL 作为 Markdown 文本，不把“待访问”写入结果。

“同步已打开标签”按钮在执行期间禁用并显示忙碌状态，完成后显示本轮成功、无法读取和跳过数量，刷新列表并保留仍存在的选择；失败后恢复按钮并允许重试。

Alfred keyword 为 `dia`，参数先识别 `list`，`dia list` 展示全部，`dia list <查询>` 按 title 和 URL 进行不区分大小写的过滤。无参数时返回可自动补全到 `dia list ` 的入口。使用 Script Filter JSON 的 `mods`：回车激活，Cmd+回车复制 URL，Alt+回车复制 Markdown。所有动作以 JSON 参数通过 argv 传递，不把 title/URL 插进 shell 代码。复制用 `/usr/bin/pbcopy` 的 stdin。

扩展和 Alfred 的 Markdown 转义规则必须一致：标题中的换行归一为空格、转义反斜线及方括号；URL 中会破坏链接语法的空格、括号、尖括号和控制字符使用百分号编码，保留原有合法百分号编码。页面渲染用 textContent，不用网页 title 拼接 HTML。两端共用测试用例数据来验证一致性，无需为此增加公共运行库。

## 权限、文件与安装

扩展权限为 `tabs`、`scripting`、`storage`、`nativeMessaging`、`alarms`、`clipboardWrite`，host permissions 仅 `http://*/*` 和 `https://*/*`。不申请 debugger、history、cookies、webRequest 或 externally_connectable。

建议文件结构：

```text
plugins/dia_tabs/
  extension/manifest.json
  extension/background.js
  extension/control.html
  extension/control.js
  extension/control.css
  native/host.py
  workflow/info.plist
  workflow/scripts/dia_tabs.py
  build.sh
  install.sh
  tests/background.test.mjs
  tests/control.test.mjs
  tests/test_native.py
  tests/test_workflow.py
  tests/markdown_cases.json
docs/plugins/dia_tabs.md
docs/specs/2026-11-01-dia-tabs-design.md
.github/workflows/dia-tabs.yml
```

安装到稳定用户目录，不能让 native manifest 指向临时 worktree。安装脚本解析已安装的 Python 3 绝对路径并生成 launcher；不能假设 Alfred 或 Dia 继承交互 shell 的 PATH。native manifest 的 allowed_origins 只允许本扩展 ID，不允许通配符。扩展建议提供稳定的 manifest 公钥以固定 unpacked ID；公钥不是密钥，不应提交私钥。

2026-11-01 在本机 Dia 1.55.1 实测确认 native host 使用 `~/Library/Application Support/Google/Chrome/NativeMessagingHosts`：注册到 `Dia/User Data/NativeMessagingHosts` 时返回 `Specified native messaging host not found`，改为 Chrome 用户目录后连接成功，Alfred 可以实时列出网页。安装器默认采用已实测目录，保留 `--native-host-dir` 覆盖。[AdGuard 官方仓库的浏览器兼容表](https://github.com/AdguardTeam/CompatibilityIssues/blob/master/mac/browsers.json)将 Dia 配置为前一种目录，与本机结果冲突；该表是 AdGuard 的适配实现，目标机器连通证据优先，不推断其他 Dia 版本的查找顺序。只保留本工具在已验证目录中的 manifest，删除测试产生的本工具冗余 manifest；不得改动已有浏览器配置或写系统目录。构建产物包含 unpacked 扩展、本机安装文件和 `.alfredworkflow`，不包含用户列表、socket、配置副本或缓存。

安装顺序：构建及离线验证 → 安装稳定文件/native host → 在 Dia 加载 unpacked 扩展 → 检查桥接连通 → 导入 Alfred Workflow → 集中进行实机验收。用户使用 Dia 时不进行分散 UI 探查，不重启或抢占浏览器。卸载仅删除本工具已安装文件和已确认归属的 manifest，不批量清理用户目录。

## 验证与完成标准

| 层次 | 必须覆盖 |
|---|---|
| 扩展状态测试 | active/complete 门槛；导航失效；后台保留；关闭清理；延迟响应；同 URL 双标签；SW 初始化恢复；tabs.title 不能成为内容来源 |
| 控制页测试 | 普通/Ctrl/Cmd/Shift 选择；按钮不冒泡；关闭后选择清理；输出顺序；特殊字符转义 |
| Python 协议测试 | 分片帧、UTF-8 字节长度、超限、无效 JSON、并发请求、超时、EOF 退出、陈旧 socket、错误 session |
| Alfred 测试 | `dia list` 解析和过滤；mods 参数；不拼 shell；断连空态；复制失败不能报成功 |
| 包检查 | JS 语法与 Node 测试、Python unittest、shell 语法、plutil、zip 内容和解压检查 |
| Dia 实机 | 手动重命名仍读 DOM title；已改名页继续动态改 title；active 加载完成采集；后台导航清空；两个同 URL 标签精确激活；跨窗口/最小化；关闭清理；native 重连；浏览器关闭后 Alfred 无旧结果 |
| Alfred 实机 | 输入 `dia list` 展示与搜索；回车激活已有标签；Cmd/Alt 复制；本机真实剪贴板内容；无额外新标签 |

离线测试不能替代 Dia 实机验证。若某项因用户正在使用浏览器无法完成，交付应列出未验证项，不能声称全部可用。当前环境已确认 Dia 1.55.1、Alfred 5、Python 3 和 Node 可用；具体 Chromium 内核版本、Native Messaging 路径、标签 API 与上述行为的兼容性仍需最终核对。

## 落地顺序与验证命令

1. 扩展实现 manifest、标题缓存/事件处理、控制页及 Node 测试；本机端实现 host、Alfred、安装打包及 Python 测试。两端严格遵守上述固定协议。
2. 合并后完成独立 CI、README/AGENTS/使用说明入口。构建产物进入本插件 dist；检查现有 release 约定，避免影响其他资产。
3. 跑离线检查并审查生成包；安装稳定路径文件后，再集中进行 Dia 和 Alfred 实机验收。所有测试网页、截图和提交附件仅使用无敏感演示数据。
4. 根据实机证据修正注册目录与兼容性说明，再提交 PR；不把待验证事实写成保证。

建议统一提供 `plugins/dia_tabs/tests/run.sh` 封装离线检查，底层至少执行以下命令（具体文件名与实现保持一致）：

```bash
node --test plugins/dia_tabs/tests/*.test.mjs
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s plugins/dia_tabs/tests -p 'test_*.py'
bash -n plugins/dia_tabs/build.sh plugins/dia_tabs/install.sh
plutil -lint plugins/dia_tabs/workflow/info.plist
plugins/dia_tabs/build.sh
git diff --check
```

扩展含 JS 模块时用 module 方式检查语法，避免以 CommonJS 模式检查产生伪错误。打包脚本输出实际产物路径，随后逐个执行 `unzip -t` 并检查包中没有用户状态；此插件不运行仓库根目录 Go build/test。

## 依据

- 项目现有 `AGENTS.md`、`README.MD`、`plugins/alfred_remote_upload/build.sh`、Workflow plist 与 `.github/workflows/alfred-workflow.yml`：独立插件、源码打包、离线测试与 macOS 实机验证约定。
- [Chrome Tabs API](https://developer.chrome.com/docs/extensions/reference/api/tabs)：active 是窗口内选中状态；activeTab 权限需用户操作；激活 tab 不等于聚焦 window。
- [Chrome Scripting API](https://developer.chrome.com/docs/extensions/reference/api/scripting)：运行时注入和顶层 frame 的脚本结果。
- [Chrome Storage API](https://developer.chrome.com/docs/extensions/reference/api/storage)：session storage 在浏览器重启及扩展重载后清除，可用于 service worker 状态。
- [Chrome service worker 生命周期](https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle)：worker 会终止，全局变量不能持久保存；Chrome 105 起 Native Messaging 连接可保持 worker 存活。
- [Chrome Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging)：stdio 帧格式、host manifest、严格 allowed_origins；浏览器之间的注册目录存在差异，Chrome 路径不能自动视为 Dia 路径。
- [Alfred Script Filter JSON](https://www.alfredapp.com/help/workflows/inputs/script-filter/json/)：items、arg、mods、valid 等交互字段。
- [Dia 1.17.0 发布说明](https://www.diabrowser.com/changelog/mac/1-17-0)：Manifest V2 扩展已禁用，新增扩展采用 MV3。
