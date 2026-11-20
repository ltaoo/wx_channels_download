# 视频号 feed.js 新增代码与播放验证

2026-11-20，已在正式 `/Applications/WeChat.app` 的原生 LiteApp 视频页面中验证 MITM 追加 JavaScript：`handleFinder=true`，收到 `WXMITM_LITEAPP_H` 执行与标记回执，标记显示时原 Vue 实例保留；用户确认黄色标记可见且视频正常播放。入口是远程 libpag SDK 响应，SDK 原文保留，后面追加独立用户脚本加载器；不使用内存 Hook，也不修改签名包中的 `feed.js`。当前验证适用于本机微信与现有远程 SDK，不保证未来所有版本或 Windows 可用。

下文先保留 WechatNode 副本上的 G 轮与 H5 回退实验记录，正式版原生 LiteApp 的成功结果见文末“正式版成功验证”。

## 范围与授权

见 [scope.md](wxchannels-feed-verification/scope.md)。本记录为本机运行验证，报告类型为技术诊断，`flavor = null`；不作漏洞认定。

## 实现与运行条件

- 覆盖文件：读取 `/private/tmp/wxchannels-effective-feed/runtime-round.json` 的 `source_path` 字段定位；文件 SHA-256 为 `29a1a8f948e0b6aa95c4e73fe5e1c2c14d7651f6054f45a33d8006eccefe9265`。
- 监听脚本：`/private/tmp/wxchannels-effective-feed/watch-additions.cjs`；每两秒读取覆盖文件，提取原始 bundle 前后新增的代码。
- 页面执行：`bootstrap-additions.cjs` 限定目标视频页面、等待 Vue 挂载、避免同一段新增代码重复执行，并记录注入前后的 Vue 实例信息。不会再次执行完整原始 bundle。
- G 版标记只渲染带 `data-n="wxchannels-effective-feed-marker"` 的元素；不调用阻塞弹窗。
- 依赖：WechatNode 管理的微信副本、PageSpy 服务、监听脚本保持运行；视频走 EAPI 时需要保留已修复的 CDN 路由。

查看实际覆盖文件路径：

```bash
python3 -c 'import json; print(json.load(open("/private/tmp/wxchannels-effective-feed/runtime-round.json"))["source_path"])'
```

## Evidence

| ID | source_ref | 观察与复现方式 | content_hash |
|---|---|---|---|
| E-001 | 会话中代理规则查询和 curl 响应 | 修复前访问 CDN 命中 `mock_1786417568453082000`，返回 502；其上游为未监听的 `127.0.0.1:2023`。旧状态已被高优先级规则覆盖，不为复现故障而恢复。 | n/a，命令输出 |
| E-002 | `/private/tmp/wxchannels-effective-feed/cdn-range-verification.json` | 修复后的真实视频请求返回 206，读取 16 字节，Content-Range 为 `bytes 0-15/1743715`。使用的带令牌视频 URL 不写入报告。 | n/a，运行记录 |
| E-003 | `/private/tmp/wxchannels-effective-feed/receipts.jsonl` | 17:29:43 和 17:29:44 收到 G 标记的 `executed`、`badge_added` 回执；可用下方命令读取。 | n/a，追加日志 |
| E-004 | 用户本次人工反馈 | 用户确认视频正常播放，同时看到黄色 G 标记；需在对应本机环境由人工复现。 | n/a，人工观察 |
| E-005 | `/private/tmp/wxchannels-effective-feed/test-additions.cjs` | 自检通过：原 bundle 提取、语法校验、目标页面限制、幂等执行、Vue 实例保留、错误处理。 | n/a，自检输出 |

```bash
python3 -c 'import json; rows=(json.loads(x) for x in open("/private/tmp/wxchannels-effective-feed/receipts.jsonl")); print(json.dumps([r for r in rows if r.get("query",{}).get("marker")==["WXFEED_20261120_G"]],ensure_ascii=False,indent=2))'
node /private/tmp/wxchannels-effective-feed/test-additions.cjs
curl -sS -I --max-time 15 --proxy http://127.0.0.1:8899 https://finder.video.qq.com/
```

最后一条命令请求 CDN 根路径，当前预期返回真实上游 404，并命中下方新规则；它只验证路由，不等价于验证视频播放。

## Findings

| ID | status | severity | evidence_ids | location | 结论与置信度 |
|---|---|---|---|---|---|
| F-001 | validated | n/a_re | E-001、E-002 | EAPI 视频 CDN 路由 | 通用 `*.qq.com` 代理规则会阻断视频请求。新增仅覆盖 `https://finder.video.qq.com/*` 的 HTTPS 直连转发规则后，真实视频请求可返回分段数据；置信度高。 |
| F-002 | validated | n/a_re | E-003、E-004、E-005 | 独立覆盖文件与 PageSpy 执行链 | G 版新增代码执行并显示标记时，视频仍正常播放；置信度高。完整 bundle 替换及原版微信独立生效仍不在此结论内。 |

新规则为 `mock_1795166888391524000`，名称 `wxchannels video CDN direct playback`，方法 `ALL`，目标 `https://finder.video.qq.com:443`，优先级 10003。原通用代理规则保持原配置，由此更具体的高优先级规则覆盖视频 CDN。

## Path 与时间线

P-001（`path_type=callflow`）：

1. 17:07，完整脚本在运行中的页面上重新执行，用户看到 F 标记但视频不播放；这一轮不计成功。
2. 17:27，代理测试证实视频 CDN 返回 502（E-001 → F-001）。
3. 17:28，增加 CDN 定向规则；真实视频请求返回 206（E-002 → F-001）。
4. 17:29，只执行覆盖文件新增代码，收到 G 回执（E-003 → F-002）。
5. 随后用户确认标记与正常播放同时存在（E-004 → F-002）。

## 限制

当前只支持在原始 bundle 前后新增代码，修改原 bundle 内部会被监听脚本拒绝。代码新增部分本身仍可能产生业务副作用；本次只验证了标记功能。未证明此前播放失败仅由重复挂载引起，已确认的独立阻断因素是 CDN 502。

收尾的只读 PageSpy 状态查询超时，未取得实时播放器数值或 Vue 实例 ID；本轮运行结果依据执行回执与用户实际播放确认。

监听脚本及辅助记录位于临时目录，重启或清理临时文件后不保证保留；未配置开机自启，也未验证长时间运行或微信升级后的行为。签名包缓存替换与纯 MITM 完整文件替换仍未实现。

## 正式版正常点击卡片的 H5 回退实验

用户随后选择正式微信内的 H5 方案，并要求保留直接点击视频卡片的操作方式。手工追加 `skipLiteApp=1` 只影响普通网页链接，不能覆盖这一操作流程。

2026-11-20 17:49–17:50，读取现有加载器确认：`applyEngineConfig` 将 `featureSwitch.handleFinder` 传到 `refreshFinderState`，再调用 `XWorkerHandleFinder`，参数为 `if_handle`。提取原始 JavaScript 方法进行离线自检，配置为 true/false 时，记录到的原生调用参数分别为 true/false；此测试尚未验证原生客户端实际回退行为。

已将本地 `ilinkres_uzhblidb/engineConfig/10/dist/engineConfig.json` 中的 `handleFinder` 从 true 改为 false，其他字段不变。修改后 SHA-256 为 `4b3f6c767d4f9866de44d682c1ffb15720cadfe5d581740171300163d5d94858`，与本机原有版本 8 的配置一致。该配置包的 `bl.getConfig()` 直接读取 JSON，没有调用本报告前述应用 JavaScript 的 `checkSignature` 路径。

备份与清单位于 `/private/tmp/wxchannels-effective-feed/finder-h5-switch/`；应用、恢复脚本为 `/private/tmp/wxchannels-effective-feed/finder_h5_switch.py`。恢复命令如下，需相应的目录写入权限；配置被外部更新后，脚本会停止以保留更新内容：

```bash
python3 /private/tmp/wxchannels-effective-feed/finder_h5_switch.py restore
```

下一步必须完全退出正式微信、重新启动并直接点击原视频卡片，观察是否产生 H5 HTML/JavaScript 网络请求。此实验尚未运行验证，不能据此宣称正式微信的 feed.js 修改已生效。后续资源更新也可能覆盖此配置。

### 自动查找并关闭开关

在项目根目录执行：

```bash
sh scripts/disable-wechat-liteapp-finder.sh
```

脚本使用 macOS 自带的 `plutil`，自动扫描当前用户微信容器的 `radium/users`，处理各账号、各数字版本目录中的引擎配置；也可将其他 `radium/users` 目录作为唯一参数传入。只将布尔值 `featureSwitch.handleFinder` 改为 false，保留其他 JSON 字段；已为 false 的文件直接跳过。修改前在原目录生成唯一的 `engineConfig.json.bak.*` 备份，使用临时文件校验并替换，写入前检查原文件是否被其他程序改动。脚本不负责重启微信。

自检命令为 `sh scripts/test-disable-wechat-liteapp-finder.sh`，覆盖多账号、多版本、含空格路径、备份内容、其他字段保留、重复执行、非法 JSON、错误值类型和未找到配置的情况。

## 保持 LiteApp 开启的远程 SDK 追加脚本实验

用户明确不接受内存 Hook，继续限定 MITM 与 JavaScript 文件修改。已下载的 `chunks-common.js` 中，模块 52603 初始化应用时调用模块 82993 的加载函数；后者在 `window.libpag` 不存在时通过 script 节点加载远程 `https://res.wx.qq.com/t/wx_fed/cdn_libs/res/libpag/4.3.48/libpag.min.js`。不能仅凭历史 H5 的 SDK 网络请求认定正式 LiteApp 本轮已触发此入口。

项目新增 `pkg/scraper/wxchannels/liteapp.go`，复用 `inject.globalScript` 配置：在目标 SDK 响应末尾追加受页面地址限制的加载器，将用户 JS 作为独立 script 节点插入。SDK 原文保留，原始 feed bundle 不重新执行。辅助标记脚本为 `scripts/wxchannels-liteapp-extra.js`，只允许视频号 LiteApp 地址，等待原 Vue 实例挂载后显示 H 标记并报告执行结果；不在 H5 页面显示。

新增 Go 单元测试与 `scripts/test-wxchannels-liteapp-extra.cjs` 已通过，覆盖原 SDK 保留、脚本修改重读、缺失脚本、无配置与非目标资源、页面范围、重复加载、挂载等待、Vue 实例保留。

本机 EAPI 规则 `mock_1795171362863440000` 将 libpag SDK GET 转发到 `127.0.0.1:18998`；辅助转发程序直接复用项目拦截器，日志在 `/private/tmp/wxchannels-effective-feed/libpag-bridge.log`。通过 EAPI 的 curl 测试证实原始 160,034 字节为修改后响应的完整前缀，修改后共 162,856 字节，带 `Cache-Control: no-store`。原 SDK SHA-256 为 `76e239644ca98c3f765d31e094d08b28b324033fc3fdd086c0b31aa5002de514`，修改后为 `f428e3c2d3bc07ad3fc4bc2d61713cb1ad0905fa302e57cf13433732eff57369`。这只证明代理响应正确，不证明页面执行。

首次加载需要避开已有一年缓存。已按缓存键而非文件内容引用定位到无 query 的 SDK HTTP 缓存 `a9500941d7561aa1_0`；其他页面源码中引用相同地址的缓存不在移动范围。准备计划为 `/private/tmp/wxchannels-effective-feed/native-mitm-cache-plan.json`，执行脚本为同目录 `native_mitm_cache.py`，要求正式微信完全退出后才可应用。

用户确认完全退出后，已核对正式微信进程不存在，应用计划将这一条缓存移至 `/private/tmp/wxchannels-effective-feed/native-sdk-cache-backup/`，并恢复引擎配置的 `handleFinder=true`。备份 SHA-256 与原缓存一致，原缓存位置已不存在；签名包中的 `feed.js` SHA-256 仍为 `2957c47b5147f09bfa4b07e9e96307d5eee499e935d468b81a45688d46d38e2b`。服务与 EAPI 规则均正常，准备状态保存为 `/private/tmp/wxchannels-effective-feed/native-mitm-ready.json`。

该准备阶段随后完成正式版验证，结果见下节。此远程入口依赖微信继续加载该 SDK，不作未来所有版本可用的保证。

## 正式版成功验证

### Evidence

- E-006：正式进程路径为 `/Applications/WeChat.app/Contents/MacOS/WeChat`，核验 PID 为 1681；配置 `handleFinder=true`。18:56:37 SDK 转发日志记录修改后的 libpag 响应，原始 SDK 哈希与前述直连原文一致。18:56:37.896 收到 H 标记的 `badge_added` 回执，页面为 `/a/finder/wxalite28e098851253e34f4777f7ca024236fd/pages/lite/feed`，`app_preserved=true`；同秒收到 `executed` 回执。原始 `feed.js` 哈希仍为上述原值。脱敏核验快照：[native-mitm-success.json](wxchannels-feed-verification/native-mitm-success.json)。原始本机记录位于 `/private/tmp/wxchannels-effective-feed/native-mitm-success.json`、`receipts.jsonl` 和 `libpag-bridge.log`。
- E-007（人工独立观察）：用户本次反馈“现在有注入黄色标签，并且可以正常播放”。此观察对应已恢复 LiteApp 开关、移走单项 SDK HTTP 缓存后的正式微信测试，不沿用此前副本上的 G 结果。

E-006 快照 SHA-256：`4a0255312a8cc5c3bd7f9825a810a3c2a470cd11e1700e8f053301719d53e334`；离线读取复现命令为 `cat docs/wxchannels-feed-verification/native-mitm-success.json`。实时复现需保持上述本机服务、代理规则和目标 SDK 请求入口有效。

`executed` 回执中的 `app_preserved=false` 表示示例模型当时尚未保存原 Vue 实例引用，不是实例被销毁；保存引用并渲染标记后的 `badge_added` 回执为 true。

### Finding 与 Path

F-003：`status=validated`，`severity=n/a_re`，`confidence=high`，`evidence_ids=[E-006,E-007]`，位置为 `pkg/scraper/wxchannels/liteapp.go` 与当前远程 SDK 请求。正式版原生视频号已能通过 MITM 执行附加 JS，且本轮正常播放；未修改或替换签名包中的完整 feed bundle。

P-002（`path_type=callflow`）：LiteApp 初始化加载远程 libpag SDK → EAPI 转发到本地 18998 服务 → 项目拦截器保留 SDK 并追加脚本加载器 → 仅在目标 LiteApp 地址插入附加 JS → 收到 H 执行与保留 Vue 实例的标记回执（E-006）→ 用户确认播放正常（E-007），形成 F-003。

### 使用与边界

本机可编辑的附加代码为 `scripts/wxchannels-liteapp-extra.js`；代理在下一次 SDK 请求时读取文件，已有页面不自动热更新。项目正式接口复用 `inject.globalScript`，说明见 [用户脚本](config/script.md)。

当前运行验证仍依赖 EAPI 8899 与本地临时 SDK 转发服务 18998，服务需持续运行；现有已启动的旧版本下载器二进制尚未更新。临时缓存备份、辅助服务及原始日志位于 `/private/tmp`，不保证重启或清理后保留。Windows 注入、开机自启、未来 SDK 移除或完全本地化的情况未验证。


## 复用项目业务 JS 重写（待正式页面验证）

用户补充目标是让 `interceptor.go` 的业务重写真正生效，黄色标签不作为这一目标的完成证明。

已将原 H5 API 重写代码原样提取为 `api_rewrite.go` 的 `rewrite_channels_api_js`，原 H5 调用保留 exports 事件处理；原生模块复用相同方法重写，跳过 ESM export 处理。远程 SDK 加载器取得当前页面 Webpack API 类模块的源码，通过本机 MITM 的 compile 接口改写，随后以内容哈希路径和 query 加载真实 `.js` 文件。仅把改写方法安装到原类原型，不创建新的服务实例、不重跑完整 feed bundle；现有请求适配器、凭据状态、Vue 与播放器实例保留。

同一加载器复用项目原有 H5 注入清单，加载事件总线、工具、详情页脚本和可选用户脚本；共享资源通过 `res.wx.qq.com/__assets/*` 返回。业务模块内容哈希 URL 与所有相关响应的 no-store 配合，避免代理收到请求后继续使用旧脚本。SDK 若仍命中此前长期缓存，仍需先促成一次真实 SDK 网络请求。

SDK 加载晚于首屏请求，因此安装后通过改写后的 API 补做初始化；取得路由参数或现有 Pinia feed 数据时补取详情。诊断接口 `/__assets/wxchannels/liteapp/status` 记录替换方法数、实际 Init/详情事件数、内容版本和 Vue 实例保留状态，不记录视频对象或认证参数。

E-008：真实已下载的原生模块 92198 已经重写并通过语法校验，改写模块 10,142 字节，版本 `1211356866517f4794e85a79a3c6d5a99990c029bc680bc469075907c77239f8`。离线使用原模块与改写模块，保留相同依赖接口与请求返回夹具，验证初始化和详情请求的参数/返回值一致、已有状态保留，改写版本发出真实调用结果对应的 `channels:Init`、`channels:OnFeedProfileLoaded`。该检查的网络请求适配器使用夹具，不能充当正式微信内执行的证明。

E-009：Go 包测试（开发资源模式与 embed_inject 模式）、Node 原生运行时检查通过，覆盖内容变化导致哈希 URL 变化、API 方法实际执行、原实例和 Vue 保留、首屏 store 详情补取、页面范围、重复安装与编译失败。EAPI 链路已验证 22 项项目注入资源可下载，SDK 原文为改写响应完整前缀。脱敏准备快照见 [business-rewrite-ready.json](wxchannels-feed-verification/business-rewrite-ready.json)。

本机 `18998` 转发服务已更新；新增 EAPI 规则 `mock_1795173899056435000` 转发 `https://res.wx.qq.com/__assets/*`，原 SDK 规则继续使用。当前服务日志为 `/private/tmp/wxchannels-effective-feed/libpag-bridge-business.log`。

F-004：`status=needs_validation`，`confidence=medium`，`evidence_ids=[E-008,E-009]`。当前已实现“业务源码重写 → 哈希 URL 加载 → 原实例方法替换”的链路并完成离线/代理检查，正式微信需要用户再次完全退出并重新打开原生视频页面，核对本轮业务事件和播放结果。此前 E-006/E-007 仅证明额外标记脚本入口。

当前适配范围为导出详情 API 类的模块；其他原生单例模块中的首页切换、直播等功能尚未完整适配，不能声称 H5 的全部功能已经移植。
