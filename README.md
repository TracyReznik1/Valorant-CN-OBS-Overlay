# Valorant CN OBS Rank Overlay（Windows / macOS）

国服《无畏契约》OBS 段位 Overlay。通过 QQ 扫码登录掌上无畏契约数据链路，在本机显示当前段位、RR、今日排位胜负和上一局 RR 变化。

> 本项目为非官方社区工具，与 Riot Games、腾讯及《无畏契约》官方无隶属或授权关系。所使用的掌瓦接口属于非公开接口，官方更新后可能失效。

## 下载后怎么用

### 系统要求

- Windows 10 / Windows 11（x64）
- 或 macOS 12 及以上（Apple Silicon / Intel）
- OBS Studio
- 能正常访问 QQ 登录与掌上无畏契约相关服务

**Windows 不需要安装 Python、pip 或管理员权限；macOS 不需要 Homebrew、Python 或管理员权限。**

### 第一次启动（Windows）

1. 从 GitHub Release 下载 Windows x64 ZIP。
2. **完整解压**到普通文件夹，不要直接在压缩包里运行。
3. 双击 `启动段位Overlay.bat`。
4. 首次运行会打开 QQ 登录二维码，用 QQ 扫码并确认授权。
5. 登录成功后浏览器会打开 Overlay 预览页。
6. 保持程序窗口运行；关闭窗口或按 `Ctrl+C` 会停止 Overlay。

### 第一次启动（macOS）

1. 从 GitHub Release 下载 macOS ZIP。
2. **完整解压**到普通文件夹，不要直接在压缩包里运行。
3. 第一次打开时，右键 `启动段位Overlay.command` → **打开**；如果系统仍拦截，在终端执行：
   ```bash
   xattr -dr com.apple.quarantine /路径/到/解压目录
   ```
4. 终端会显示服务地址；首次运行会生成 QQ 二维码并用预览打开。
5. 用 QQ 扫码并确认授权；登录成功后浏览器会打开 Overlay 预览页。
6. 保持终端窗口运行；按 `Ctrl+C` 或关闭终端会停止 Overlay。

macOS 首次访问 Keychain 时可能弹窗，请选择“允许”。如果拒绝或 Keychain 不可用，程序会回退到数据目录内的 `0600` 本地密钥文件，凭证仍然不是明文。

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
[游戏段位图标]  铂金I  82 分
                 今日 2 胜 / 1 负   上一局 -18 分
```

左侧徽章使用掌瓦段位卡片返回的游戏内段位图标（`card.left_data.image_url`），不额外加背景圆圈；徽章不添加实体底座，只使用按当前段位主色与上一局胜负渐变的柔和背景，边缘自然淡出：胜利偏绿、失败偏红、平局/未知偏中性。图标暂时缺失或加载失败时显示中性占位标记，不会用文字冒充段位图标。连胜/连败只统计**本次软件启动后**开始的竞技对局，软件重启后从头计算；达到 2 场时显示 `N 连胜` / `N 连败` 徽章，并触发整卡呼吸光晕动画。连胜为绿色、连败为红色。上一局同样只显示本次启动后的最近一场竞技对局，重启后回到“上一局 --”。

数据后台默认每 30 秒刷新一次；页面本身每 3 秒读取本机缓存状态，不会每 3 秒请求掌瓦。

## 登录缓存与隐私

- **Windows**：登录凭证默认保存在 `%LOCALAPPDATA%\ValorantCNOverlay\credentials.bin`，使用 **Windows DPAPI** 加密并绑定当前 Windows 用户环境。
- **macOS**：默认保存在 `~/Library/Application Support/ValorantCNOverlay/credentials.bin`，使用 **AES-GCM** 加密；密钥优先放在登录 Keychain，Keychain 不可用时回退到同目录的 `0600` `credentials.key`。
- 旧版便携目录的 `.data/credentials.bin` 会被自动沿用。
- 两种平台的凭证文件本身都不是明文 Token。
- 本地 HTTP 服务只监听 `127.0.0.1`，不会主动开放到局域网。
- `/api/state` 只提供段位、RR、胜负等展示数据，不提供 QQ/掌瓦登录 Token。
- 不要把数据目录上传到 GitHub、网盘或发给其他人，也不要放到会被云盘同步的文件夹。

需要重新授权时：

- Windows：双击 `重新登录.bat`。
- macOS：双击 `重新登录.command`。
- 两个平台也可以打开 `http://127.0.0.1:8787/status`，点击“重新登录（弹出二维码）”。

## 常见问题

### 1. 双击一闪而过 / Windows SmartScreen / macOS Gatekeeper

- **Windows**：请先运行 `诊断.bat`。本项目当前没有商业代码签名证书，因此 SmartScreen 可能显示“未知发布者”。如果 EXE 被安全软件拦截，请先核对 Release 来源和 `CHECKSUMS.sha256`，再查看拦截记录；不建议盲目关闭安全软件。
- **macOS**：首次打开请右键 `启动段位Overlay.command` → **打开**。如果仍提示“无法验证开发者”，在终端执行 `xattr -dr com.apple.quarantine /路径/到/解压目录`，或从源码自行构建。启动失败时先双击 `诊断.command`。

### 2. 二维码过期

关闭程序后重新双击 `启动段位Overlay.bat`（Windows）或 `启动段位Overlay.command`（macOS）。

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

Overlay 会保留最后一次成功数据，并显示黄色状态点。可能是网络波动、登录态失效或掌瓦接口发生变化。程序启动时会尝试恢复登录；如果运行中登录态失效，**不会自动弹出二维码**，请关闭程序后运行 `重新登录.bat`（Windows）/ `重新登录.command`（macOS），或打开 `/status` 点击重新登录。

当前 RR 缺失或无法解析时显示 `--`，同时以独立解析告警点亮黄点。鼠标悬停黄点可查看原因；也可以在本机查看 `http://127.0.0.1:8787/api/state` 的 `error`、`warning` 和 `rr_source`。这些诊断只包含固定说明、错误码和取值路径，不包含原始接口响应或凭证。后续完整刷新成功后黄点自动消失。

已知字段及文本路径支持超过 100、1000 的累计 RR；`82/100` 进度格式仍限于 0～100。真实高段位响应结构尚需玩家实测，详见 [RR 解析与诊断说明](docs/rr-diagnostics.md)。

### 5. 端口 8787 被占用

开发者/高级用户可以从命令行运行：

Windows：

```powershell
.\ValorantCNOverlay.exe --port 8788
```

macOS：

```bash
./ValorantCNOverlay-macos-arm64 --port 8788
# Intel Mac 使用 ValorantCNOverlay-macos-amd64
```

OBS URL 同步改为 `http://127.0.0.1:8788/overlay`。

## 命令行参数

```text
--port 8787          本地 HTTP 端口
--refresh 30         掌瓦刷新间隔（5–3600 秒）
--no-open            启动后不自动打开浏览器或二维码（会打印二维码路径）
--reset-login        删除本机加密登录态并重新扫码
--data-dir <目录>    自定义登录缓存与临时文件目录
--diagnose           检查数据目录、系统安全存储、端口和掌瓦连通性
--version            显示版本
```

同一时间只允许一个实例运行；重复双击会提示已有实例，而不是再请求一次掌瓦。

本机状态页：`http://127.0.0.1:8787/status`。可查看当前状态、错误信息和 RR 来源，也可以点击“重新登录”让程序清除登录态并重新弹出二维码。OBS 仍然使用 `/overlay`。

## 从源码构建

项目运行时只使用 Go 标准库，无第三方 Go module 依赖。

Windows：

```powershell
go test ./...
$version = (Get-Content VERSION -Raw).Trim()
go build -trimpath -ldflags "-s -w -X main.version=$version" -o ValorantCNOverlay.exe ./src/cmd/valorant-overlay
```

macOS（同时构建 Apple Silicon 和 Intel）：

```bash
./scripts/build-macos.sh
# 或手动：
# GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=1.1.0" -o build/ValorantCNOverlay-macos-arm64 ./src/cmd/valorant-overlay
# GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=1.1.0" -o build/ValorantCNOverlay-macos-amd64 ./src/cmd/valorant-overlay
```

当前发布目标：Windows x64、macOS 12+（Apple Silicon / Intel）。

GitHub Actions 已包含在 `.github/workflows/build-windows.yml`，推送后会运行测试并生成 Windows x64 ZIP 与 macOS ZIP；推送 `v*` tag 时还会创建 GitHub Release。

## 安全说明

本工具需要代表用户访问 QQ 授权和掌上无畏契约接口，因此任何同类工具都涉及登录凭证。建议只从你信任的源码构建或项目 Release 下载，并自行核对源码和构建流程。

- Windows/macOS 发布包目前都没有商业代码签名；macOS 也没有 notarization，Gatekeeper 可能提示未知开发者。
- Windows 使用 DPAPI；macOS 优先使用 Keychain，Keychain 不可用时回退到 `0600` 本地密钥文件。两种方式都只能防止其他本机用户直接读取，不能防御已经以当前用户身份运行的恶意程序。

## 开源许可

本项目以 **GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later)** 发布。完整条款见 `LICENSE`。

实现过程参考/借鉴了社区对国服掌瓦接口及 QQ 授权流程的公开研究。具体署名和第三方许可证见 `THIRD_PARTY_NOTICES.md`。
