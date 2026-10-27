# RR 解析与状态诊断

本文包含早期修复的技术记录，以及 2026-10-27 更新的名片优先规则。本次变更见 [v1.0.2 更新说明](releases/v1.0.2.md)。下文历史构建版本、哈希及 [v1.0.1 更新说明](releases/v1.0.1.md) 对应当时的产物。

## 架构与原始数据流

基线版本：`ad62f19036b4594f02134ea828431a7c81cd966a`（v1.0.0 发布准备）。运行时仅依赖 Go 标准库。

1. `main.go` 读取选项，在首次刷新成功后启动仅监听 `127.0.0.1` 的 HTTP 服务。默认后台刷新 30 秒，最低可配置为 5 秒。
2. `service.go` 从 DPAPI 加密文件加载凭证，调用 `FetchMainRole()` 验证；失效或角色信息无效时按原逻辑清除缓存并 QQ 扫码。角色选择优先 `gameId=agame`，否则延续原来的首个角色回退规则。
3. `probe.go` POST `/go/account/get_main_role_raw`，返回 JSON envelope 的 `data`；`game_role_id`、`scene` 用于后续请求。角色中的有效段位名称和 RR 作为名片缺少有效值时的备用来源，数值 `tier` 不直接展示或猜测映射。
4. `FetchValCard()` POST `/go/mine/card/val_card`，提供当前段位卡片；`card.left_data.image_url` 用作段位图标，`card.left_data.title` 是已有的 RR 文本来源。
5. `FetchRecentMatches()` POST `/go/agame/career/record/list`，每页 10 局，最多 3 页，遇到早于本地当天的数据或不足一页即停止。今日胜负只统计当地日期内的竞技对局；上一局变化来自最近竞技对局的 `competitive_tier` 或对局顶层变化字段。
6. `OverlayState` 用锁保存内存快照。`/api/state` 暴露展示字段；嵌入式 HTML 每 3 秒读取本机缓存，不直接请求掌瓦。

上一局可以发生在昨天，因此 `0 Win / 0 Loss` 与 `Last Match: +22pts` 可以同时成立。没有现场响应，不能将此组合直接判为错误；缺失时间戳、模式变化等也会影响统计，尚未现场验证。

## 已确认问题与证据边界

- 原文本备用解析对标题、`rank_detail`、`season_rank_detail` 均限制 `n <= 100`，因此 `185分`、`350分` 无法得到当前 RR。
- 原正则限定 1～3 位且没有数字边界，四位以上数字可能被截取尾部。例如 `10050分` 可被当作 `050分`，再错误显示 `50 RR`。这比缺失值更危险。
- 原结构化字段已经允许高于 100 的值，所以并非所有高段位都必然失败。
- 原任意递归搜索会扫描其他游戏角色、历史赛季或其他玩家对象，Go map 遍历顺序也不稳定。无效文本还会直接成为 RR，或阻断有效备用来源。
- 原 `apiPost()` 把缺失或无法解析的 `result` 当作零，也没有验证有效 `data`；异常 envelope 可能被误判成功。
- 原战绩列表解析将未知结构与真正的空列表一并处理，可能把缓存战绩覆盖为 0/0。现在只接受已支持的列表容器和对象条目；结构变化触发 `matches_parse` 并保留快照，真正空列表仍有效。
- 原 RR 缺失被替换成 `--`，随后仍执行 `Update()` 清除 stale，缺少独立解析诊断。前端 HTTP 非 2xx 直接返回，未显示失败状态。

本仓库无真实 API 响应样本，原测试都是构造数据。复核项目已署名的公开研究：

- [API_FIELDS.md（固定提交）](https://github.com/wuche-2453198/astrbot_plugin_valorant_analysis/blob/1aae13042590d0ad2c9c94515e7e6ad107d5d887/API_FIELDS.md) 记录主角色、名片、战绩端点，但没有神话/辐能战魂累计积分字段规范。
- [main.py（固定提交）](https://github.com/wuche-2453198/astrbot_plugin_valorant_analysis/blob/1aae13042590d0ad2c9c94515e7e6ad107d5d887/main.py) 的 `_parse_season_data()` 将 `card.left_data.title` 作为段位详情，注释举例“铂金II 46分”。其 `season_rank_detail` 是本地存储映射，不能当作已证实的掌瓦专用字段。

这些材料只能佐证普通段位卡片路径；没有证据证明高段位采用不同字段，也不能证明没有不同字段。本次未新增猜测的“高段位专用字段”，未使用玩家凭证请求实时接口。

## 修复后的取值规则

范围限制为调用方提供的当前角色对象、卡片响应顶层、`card` 和 `card.left_data`。若传入角色列表，沿用角色解析器选择单个角色。不会递归搜索历史、战绩、好友、其他玩家或未知结构；未知结构宁可显示缺失并告警。

段位名称优先解析 `card.left_data.title` 中的明确名称；名片没有可识别名称时，再检查选定角色的 `tier_text`、`tier` 和 `TierText`。例如角色仍返回“神话III”，名片已返回“辐能战魂 1250分”，显示“辐能战魂”；名片明确返回“神话III 1250分”时仍显示神话III，不按积分推算段位。

RR 同样先完整解析名片响应；名片所有已支持路径都没有有效 RR 时，才解析选定角色。段位和 RR 各自回退，合法的 `0` 不会触发回退。诊断路径仍以 `source[0]` 表示选定角色、`source[1]` 表示名片响应；索引不是来源优先级。

每个来源内部按以下顺序取第一个有效值：

1. 明确数值字段，保留原字段顺序：`league_point`、`ranked_rating`、`rank_rating`、`competitive_point`、`rr`；按顶层、`card`、`card.left_data` 顺序检查。
2. 卡片 `card.left_data.title` 的完整 `N分` / `N RR` 文本，其次允许范围内的 `rank_detail`、`season_rank_detail` 同类文本。
3. 上述文本中的 `N/100` 普通进度，且 `0 <= N <= 100`。

同一来源内部，明确数值字段优先于积分文本，积分文本优先于 `/100` 进度。来源之间始终名片优先：名片 `350分` 与角色 `league_point=999` 并存，选择 350；名片 `82/100` 与角色 `league_point=350` 并存，也选择名片的 82。两个明确数值字段冲突时仍按既有字段顺序，不按“较大数值”猜测，也不依据未经证实的段位数字重新解释字段；可通过 `rr_source` 定位选择的路径。该策略解决角色旧值覆盖名片值的问题，不能保证上游名片本身已经同步。

数值接受非负整数的十进制字符串及整数数值，0 是有效值。RR 变化允许带符号整数，正数输出 `+N`，零输出 `0`。拒绝空值、布尔、对象、无效字符串、分数、NaN、无穷和表示溢出。JSON 数值使用现有 float64 解码，只接受安全整数范围；整数文本使用 int64 范围。这些是数值表示限制，并非游戏 RR 上限。字符串不推测千位分隔符、小数或科学计数格式。

## 黄点、缓存与诊断

| 场景 | 本机 API 状态 | 显示与恢复 |
| --- | --- | --- |
| 尚未首次刷新 | `stale=true`，等待首次刷新 | 正常程序在首次 API 刷新失败时退出，不会启动 HTTP 服务 |
| 初始化探针、登录、角色、名片或战绩请求失败 | `stale=true`，`error` 带阶段 | `MarkStale()` 保留整份此前快照及 `updated_at`；后续成功 `Update()` 清除 stale/error |
| API 正常但已定级账号的当前 RR 无法解析 | `stale=false`，`rr=--`，`warning=[rr_parse] ...` | 展示本次其他有效数据并亮黄点，明确属于解析不完整；不把以前 RR 拼到新段位上。恢复可解析 RR 后清除 warning |
| 已明确未定级，当前 RR 不存在 | `stale=false`，`rr=--`，无 RR 解析告警 | RR 不适用；若接口明确提供 0 等有效 RR，仍展示该值，不伪造 0 |
| 所有已知路径均无明确段位名称 | `rank=--`，`warning` 包含 `[rank_parse]` | 保留其他有效字段并亮黄点；不把未知数值编码猜测成未定级 |
| 前端请求本机状态网络失败、HTTP 非 2xx 或 JSON 解析异常 | 页面本地标记 | 保留此前显示并亮黄点，提示本机状态读取失败；不改写后端状态。下一次有效响应恢复 |
| 正常完整刷新 | `stale=false`，无 error/warning | 黄点消失 |

因此原直播黄点不能由原 RR 解析失败单独解释。可能是已有 `-- RR` 快照后发生了 API 错误，也可能是浏览器读取本机状态失败；缺少现场状态和日志，不能归因于某个接口。今日分页达到 30 条时只显示战绩 `+`，不是黄点条件。

本机 `http://127.0.0.1:8787/api/state` 新增：

- `rr_source`：固定字段路径，例如 `source[1].card.left_data.title`，只说明取值位置。
- `warning`：固定解析说明；API 错误仍放在 `error` 中，阶段如 `[val_card]`、`[matches]`。

错误输出不包含原始响应正文、服务端任意消息、带查询参数的 URL 或任意底层错误文本；保留阶段、HTTP 状态码、接口数值错误码和超时说明。登录请求参数、Cookie、DPAPI 存储逻辑均未更改。没有添加原始响应转储功能。

## 测试与现场验证

新增测试全部为合成回归输入，高段位值/格式在注释及用例名中明确标为假设性，不能称作真实接口样本。

- 解析：82、185、350、1250、10050，字符串/数字/0，缺失/空/无效/异常类型，整数表示溢出，`82/100` 语义及其与累计 RR 的优先级，角色选择，拒绝无关嵌套字段。
- 上一局变化：正数、负数、零、无效字段和有效备用值。
- 服务：探针、登录、角色、名片、战绩失败后的缓存完整保留；恢复清除 stale/error；RR 告警和恢复；无效战绩结构不覆盖缓存，真正空列表可恢复为 0/0。
- HTTP：数字和字符串经真实客户端 JSON 解码后解析；无效 envelope 拒绝；敏感标记不会出现在错误中。
- 前端：HTTP 错误、网络错误、无效 JSON、API stale、独立 RR 告警及恢复；轮询间隔仍为 3000 毫秒。

验证命令：

```powershell
go test ./...
go vet ./...
node --test scripts/overlay.test.mjs  # 可选，Node 标准库，不增加运行时依赖
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go build -trimpath -ldflags "-s -w" -o build/ValorantCNOverlay.exe ./src/cmd/valorant-overlay
```

根目录的 `启动段位Overlay.bat`、`重新登录.bat`、`诊断.bat` 要求 EXE 与脚本位于同一目录。使用上述 `build/` 输出时，需将 `build/ValorantCNOverlay.exe` 复制到项目根目录；也可直接运行 `./scripts/build-windows.ps1`，它会生成根目录 EXE。当前修复版已补齐根目录 EXE 并通过 `诊断.bat` 检查。

2026-10-17 本地验证（PowerShell 7.6.3、Go 1.26.5、Windows/amd64）：

- `go test ./...` 及最终 `go test -count=1 -coverprofile build/coverage.out ./...` 均通过，包含真实 Windows DPAPI 往返测试；整体语句覆盖率 57.3%。
- `go vet ./...` 通过；可选前端 Node 检查 3 项全部通过。
- 显式设置 `GOOS=windows`、`GOARCH=amd64`、`CGO_ENABLED=0` 后构建通过。这是在 Windows 上验证目标构建，并非跨系统交叉编译。
- `build/ValorantCNOverlay.exe --version` 成功运行，输出 `v1.0.0`；这是版本入口启动检查，不代表已经 QQ 登录或成功请求真实掌瓦 API。
- 最终 EXE SHA256：`a65e32b9506cf5d7e14e781b618a613429e0041780dd51a5999b16b10841f52d`。
- 未执行真实 QQ 扫码、高段位玩家接口请求或 OBS 直播验证；未修改 DPAPI、认证参数、刷新周期或界面布局。

仍需在神话、辐能战魂玩家环境运行修复版，对照掌瓦界面核对当前积分及变化后的积分。优先查看安全状态的 `rr_source`、`warning`、`error`。若结构仍不匹配，应由玩家在本地仅提取白名单展示路径和 RR 值，不分享完整响应、`.data`、请求头、Cookie、QQ Token、OpenID、TID、Access Token 或 scene 等凭证。得到脱敏且可追溯的样本后再扩充明确路径和优先级。

## 未定级数字段位编码的修复

当角色 `tier_text` 为空、备用 `tier` 返回数值枚举，而名片标题包含明确段位名称时，直接把 `tier` 转成字符串会显示数字编码。公开回归用例使用构造的角色标识与最小字段，不包含玩家标识、战绩或原始响应。

根因是原先 `first("tier_text", "tier")` 将数值枚举转换成字符串，服务未经名称验证便直接显示。修复后只接受有效段位名称，不对数字枚举建立未经证实的全局映射。2026-10-27 又将来源顺序改为优先从 `card.left_data.title` 提取明确段位名称，缺失时回退到角色名称。

回归覆盖：数字/字符串枚举、名片与角色名称冲突、名片缺失时的角色回退、缺少明确名称时告警、未定级 0 RR、无适用 RR、完成定级切换，以及高 RR 标题中的数字不会混入段位名称。

公开报告只保留实现方法与测试结论；本地账号状态、个人战绩和操作记录不随源码或发布包分发。版本构建校验以对应发布包的 `CHECKSUMS.sha256` 为准。

安全诊断默认不参与普通测试。仅在本机需要排查时显式启用：

```powershell
$env:VALORANT_OVERLAY_DIAGNOSTICS_STORE = (Resolve-Path ./.data/credentials.bin).Path
$env:VALORANT_OVERLAY_DIAGNOSTICS_EXPECT_RANK = '未定级' # 可选断言
go test -tags localdiagnostics -run '^TestLocalRankDiagnostic$' -count=1 -v ./src/cmd/valorant-overlay
Remove-Item Env:VALORANT_OVERLAY_DIAGNOSTICS_STORE
Remove-Item Env:VALORANT_OVERLAY_DIAGNOSTICS_EXPECT_RANK -ErrorAction SilentlyContinue
```

诊断直接读取现有 DPAPI 缓存并在内存解密，只请求当前主角色和名片，不执行扫码、不删除或重写登录缓存；默认不记录任意响应文本。未定级账号的实测不能替代真实神话、辐能战魂数据验证。
