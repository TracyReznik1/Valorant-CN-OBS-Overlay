# Valorant CN OBS Rank Overlay

国服《无畏契约》OBS 段位 Overlay。通过 QQ 扫码登录掌上无畏契约数据链路，在本机显示当前段位、RR、今日排位胜负和上一局 RR 变化。

> 本项目为非官方社区工具，与 Riot Games、腾讯及《无畏契约》官方无隶属或授权关系。所使用的掌瓦接口属于非公开接口，官方更新后可能失效。

## 下载后怎么用

### 系统要求

- Windows 10 / Windows 11
- 64 位 x86（x64）
- OBS Studio
- 能正常访问 QQ 登录与掌上无畏契约相关服务

**不需要安装 Python，不需要 pip，不需要管理员权限。**

### 第一次启动

1. 从 GitHub Release 下载 Windows x64 ZIP。
2. **完整解压**到普通文件夹，不要直接在压缩包里运行。
3. 双击 `启动段位Overlay.bat`。
4. 首次运行会打开 QQ 登录二维码，用 QQ 扫码并确认授权。
5. 登录成功后浏览器会打开 Overlay 预览页。
6. 保持程序窗口运行；关闭窗口或按 `Ctrl+C` 会停止 Overlay。

默认 Overlay 地址：

```text
http://127.0.0.1:8787/overlay
```

## OBS 设置

OBS → **来源** → **+** → **浏览器**：

- URL：`http://127.0.0.1:8787/overlay`
- 宽度：`420`
- 高度：`120`

Overlay 默认显示：

```text
[段位图标]  铂金I  82 RR
             2 Win / 1 Loss   Last Match: -18pts
```

数据后台默认每 30 秒刷新一次；页面本身每 3 秒读取本机缓存状态，不会每 3 秒请求掌瓦。

当前段位、RR 和图标优先采用名片数据；名片中的段位或 RR 缺失、无效时，分别回退到主角色数据。今日胜负和上一局 RR 变化仍来自战绩接口。段位按接口名称显示，不根据 RR 推算。

## 登录缓存与隐私

- 登录凭证保存在本目录 `.data/credentials.bin`。
- 凭证使用 **Windows DPAPI** 加密，并绑定当前 Windows 用户环境；文件本身不是明文 Token。
- 本地 HTTP 服务只监听 `127.0.0.1`，不会主动开放到局域网。
- `/api/state` 只提供段位、RR、胜负等展示数据，不提供 QQ/掌瓦登录 Token。
- 不要把 `.data` 文件夹上传到 GitHub、网盘或发给其他人。

需要重新授权时，双击：

```text
重新登录.bat
```

## 常见问题

### 1. 双击一闪而过 / Windows SmartScreen 提示

请先运行 `诊断.bat`。本项目当前没有商业代码签名证书，因此 Windows SmartScreen 可能显示“未知发布者”。如果 EXE 被 Windows 安全中心或第三方杀毒软件拦截，请先核对 Release 来源和 `CHECKSUMS.sha256`，再查看拦截记录；不建议盲目关闭安全软件。

### 2. 二维码过期

关闭程序后重新双击 `启动段位Overlay.bat`。

### 3. OBS 页面空白

先在浏览器访问：

```text
http://127.0.0.1:8787/health
```

正常应返回：

```json
{"ok":true}
```

然后检查 OBS 浏览器源 URL 是否为 `http://127.0.0.1:8787/overlay`。

### 4. 数据停止更新但 Overlay 还在

Overlay 会保留最后一次成功数据，并显示黄色状态点。可能是网络波动、登录态失效或掌瓦接口发生变化。登录态失效时程序会尝试重新授权。

当前 RR 缺失或无法解析时显示 `-- RR`，同时以独立解析告警点亮黄点。鼠标悬停黄点可查看原因；也可以在本机查看 `http://127.0.0.1:8787/api/state` 的 `error`、`warning` 和 `rr_source`。这些诊断只包含固定说明、错误码和取值路径，不包含原始接口响应或凭证。后续完整刷新成功后黄点自动消失。

已知字段及文本路径支持超过 100、1000 的累计 RR；`82/100` 进度格式仍限于 0～100。真实高段位响应结构尚需玩家实测，详见 [RR 解析与诊断说明](docs/rr-diagnostics.md)。

### 5. 端口 8787 被占用

开发者/高级用户可以从命令行运行：

```powershell
.\ValorantCNOverlay.exe --port 8788
```

OBS URL 同步改为 `http://127.0.0.1:8788/overlay`。

## 命令行参数

```text
--port 8787          本地 HTTP 端口
--refresh 30         掌瓦刷新间隔（最低 5 秒）
--no-open            启动后不自动打开浏览器
--reset-login        删除本机加密登录态并重新扫码
--version            显示版本
```

## 从源码构建

项目运行时只使用 Go 标准库，无第三方 Go module 依赖。

```powershell
go test ./...
go build -trimpath -ldflags "-s -w" -o ValorantCNOverlay.exe ./src/cmd/valorant-overlay
```

当前发布目标：Windows x64。

GitHub Actions 已包含在 `.github/workflows/build-windows.yml`，推送后会自动运行测试并生成 Windows x64 ZIP；推送 `v*` tag 时还会创建 GitHub Release。

## 安全说明

本工具需要代表用户访问 QQ 授权和掌上无畏契约接口，因此任何同类工具都涉及登录凭证。建议只从你信任的源码构建或项目 Release 下载，并自行核对源码和构建流程。

## 开源许可

本项目以 **GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later)** 发布。完整条款见 `LICENSE`。

实现过程参考/借鉴了社区对国服掌瓦接口及 QQ 授权流程的公开研究。具体署名和第三方许可证见 `THIRD_PARTY_NOTICES.md`。
