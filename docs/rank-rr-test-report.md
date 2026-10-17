# 段位与 RR 显示测试结果

本文记录早期矩阵验证，其中版本和哈希属于当时产物。v1.0.1 另完成未使用字段、返回值和重复列表转换清理；Go 全套测试、4 项前端检查、静态分析与 Windows 构建通过，并补充空战绩、单页、跨天及三页上限四组回归用例。最终发布内容见 [v1.0.1 更新说明](releases/v1.0.1.md)。

验证日期：2026-10-17。环境：Windows x64、PowerShell 7、Go 1.26.5、Node.js。

## 结论

新增的 1,501 个接口到展示状态的矩阵用例全部通过，前端实际脚本逐一检查这 1,501 份后端生成的状态也全部通过。既有 Go 测试、`go vet ./...` 和 Windows x64 构建通过。

本轮只增加测试数据、测试代码和本报告，没有修改业务逻辑。构建得到的 EXE 与当前根目录 EXE 的 SHA256 一致，无需替换或重启正在运行的程序。

段位和分数作为两个独立的接口值验证。测试不根据 RR 推算段位、不做晋级或降级判断、不检查某个名称和分数组合在游戏中是否能达到。特别验证了接口给出“神话III、1250 RR”时仍显示神话III，给出“辐能战魂、0 RR”时也不自行改成其他段位。

## 实际测试数据

| 分组 | 接口段位名称 | RR 值 | 名称与分数组合数 | 六种来源用例数 |
| --- | --- | --- | ---: | ---: |
| 未定级 | 未定级 | 0 | 1 | 6 |
| 普通段位 | 黑铁、青铜、白银、黄金、铂金、钻石、超凡，各含 I、II、III，共 21 个名称 | 0、1、9、10、11、50、82、99、100 | 189 | 1,134 |
| 神话 | 神话I、神话II、神话III | 0、1、9、10、11、80、99、100、185、350、999 | 33 | 198 |
| 辐能及另一已支持名称 | 辐能战魂、无畏战魂，分别保留接口原名称 | 0、1、9、10、99、100、350、999、1000、1001、1250、1999、9999 | 26 | 156 |
| 段位不由分数推算 | 神话III | 1250 | 1 | 6 |
| 连续刷新后返回未定级 | 未定级 | 0 | 1 个重复状态，用于检查缓存切换 | 1 |
| 合计 | | | | **1,501** |

神话 0 RR 包含 I、II、III 三个接口名称，分别测试六种来源，共 18 例；均显示 `0 RR`，无 RR 缺失告警。本表是测试输入，不代表官方晋级门槛，也不代表这些组合都是服务器正常可达状态。

每个“名称 × RR”组合覆盖六种输入来源：

1. 名片 `card.left_data.title`：`段位 N分`。
2. 名片 `card.left_data.title`：`段位 N RR`。
3. 当前角色 `league_point` 返回 JSON 数字。
4. 当前角色 `league_point` 返回 JSON 字符串。
5. 名片响应顶层 `league_point` 返回 JSON 数字。
6. 名片响应顶层 `league_point` 返回 JSON 字符串。

全部使用已有解析路径；高段位值、`RR` 标题格式、各字段所在位置的组合是明确标记的合成输入，不冒充真实高段位 API 响应。段位数字编码的处理方法另见 [RR 解析与诊断说明](rr-diagnostics.md)。

## 验证范围

- 使用本地模拟 HTTP 服务返回 API envelope，经真正的 `Probe` 请求与 JSON 解码，再运行 `OverlayService.RefreshOnce()` 和 `/api/state` HTTP handler。每例发出主角色、名片、战绩请求各一次，共 4,503 次模拟上游请求。
- 一个服务实例连续执行全部矩阵，包含 9→10、99→100、999→1000、1000→1001、1250→1999 等数值切换，最终从高 RR 返回未定级 0 RR，检查缓存中不残留上次段位或分数。
- 角色列表同时放入其他游戏的神话III/8888 数据，验证不会取错角色。
- 每例同时放入 `82/100` 进度备用文本，确认明确积分优先，包括明确 RR 为 0 或小于 82 的情况。
- 六种来源分别断言 `rr_source`，防止预期来源失败却被其他同值字段掩盖。
- 每例都检查：段位名称、RR、无 warning/error、`stale=false`、今日 1 胜 1 负、上一局 +22、胜利结果及图标 URL。
- 前端通过 Node 的 DOM 替身执行项目中真实的嵌入脚本，消费 Go 测试生成的 `/api/state` JSON，逐例检查名称、RR、无黄点、今日胜负及上一局变化。此检查覆盖显示文本和状态逻辑，不等同于真实 OBS 截图或布局渲染。
- 既有缺失值、无效值、API 故障与恢复、未定级无适用 RR、上一局正负零等测试继续通过。

未请求或修改真实玩家的段位和 RR，未读取或更改真实登录缓存。模拟服务使用构造凭证，并禁止触发真实 QQ 登录。未获得新的真实神话或辐能账号响应，因此不能把矩阵通过表述为已完成这些账号的在线验证。

## 复现与结果文件

```powershell
New-Item -ItemType Directory -Force ./build | Out-Null
$env:VALORANT_OVERLAY_MATRIX_RESULTS = Join-Path (Get-Location).Path 'build/rank-rr-matrix-results.json'
go test -count=1 ./...
go vet ./...
node --test scripts/overlay.test.mjs
Remove-Item Env:VALORANT_OVERLAY_MATRIX_RESULTS
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags '-s -w' -o build/ValorantCNOverlay.matrix-verified.exe ./src/cmd/valorant-overlay
```

- 用例定义：`src/cmd/valorant-overlay/testdata/rank_rr_matrix.json`。
- Go 集成矩阵：`src/cmd/valorant-overlay/rank_rr_matrix_test.go`。
- 前端检查：`scripts/overlay.test.mjs`；本轮 4 项顶层检查全部通过，其中矩阵项检查 1,501 份状态。
- 完整逐例结果：`build/rank-rr-matrix-results.json`，包含输入分组、来源、预期名称和 RR、实际公开状态；仅含合成展示数据。
- 构建产物 SHA256：`e7060eefbda6d0ced6581abb3d1f22b064a8c75b5de2c2574caf7391602e7909`，与当前根目录 EXE 相同。
