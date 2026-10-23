<div align="center">
  <img src="docs/images/hero.png" width="100%" alt="Valorant CN OBS Overlay">
</div>

<h1 align="center">VALORANT CN OBS OVERLAY</h1>

<p align="center">
  <b>国服《无畏契约》段位直播 HUD</b><br>
  Windows 10/11 · macOS 12+ · OBS Browser Source
</p>

<p align="center">
  <a href="../../releases/latest"><img src="https://img.shields.io/badge/下载-v1.1.0-8fb8ff?style=for-the-badge" alt="下载 v1.1.0"></a>
  <img src="https://img.shields.io/badge/Windows-10%2F11-0078D6?style=flat-square&logo=windows&logoColor=white" alt="Windows">
  <img src="https://img.shields.io/badge/macOS-12%2B-111111?style=flat-square&logo=apple&logoColor=white" alt="macOS">
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/License-AGPL--3.0-3da639?style=flat-square" alt="AGPL-3.0">
</p>

<p align="center">
  <code>420×120</code> · <code>127.0.0.1</code> · <code>STREAM-READY</code> · <code>LOCAL-FIRST</code>
</p>

> 非官方社区工具，与 Riot Games、腾讯及《无畏契约》官方无隶属关系。掌瓦接口为非公开接口，官方更新后可能失效。

---

## 01 / LIVE HUD

<p align="center">
  <img src="docs/images/obs-scene.png" width="1000" alt="OBS 实机场景：420×120 浏览器源位于左上角">
</p>

<p align="center"><sub>OBS 场景示意 · 420×120 · UI 左上角 · 实机画面：Riot Games / <a href="https://wiki.playvalorant.com/en-us/File:Gameplay_1.jpeg">VALORANT Wiki</a> · 仅作 UI 合成示意</sub></p>

**STREAM-READY HUD**：段位图标、RR、今日战绩、上一局变化、连胜/连败，一块卡全收。

| 参数 | 值 |
|:--|:--|
| Browser Source | <code>http://127.0.0.1:8787/overlay</code> |
| Size | <code>420 × 120</code> |
| Position | 左上 / 任意角 |
| Background | 透明 |

## 02 / UI SYSTEM

<p align="center">
  <img src="docs/images/overlay-states-grid.png" width="1000" alt="四种 Overlay 状态双排横轴">
</p>

<p align="center"><sub>双排横轴：神话II · 3 连胜 / 辐能 · 5 连败 / 神话II · 5 连败 / 辐能 · 3 连胜</sub></p>

### STATE × COPY

| 实战状态 | UI 文案 | 视觉反馈 |
|:--|:--|:--|
| 神话II · 3 连胜 | 今日 3 胜 / 1 负 · 上一局 +18 分 | 鎏金呼吸 |
| 辐能 · 5 连败 | 今日 0 胜 / 0 负 · 上一局 -22 分 | 红黑警报 |
| 神话II · 5 连败 | 今日 0 胜 / 0 负 · 上一局 -22 分 | 红黑警报 |
| 辐能 · 3 连胜 | 今日 3 胜 / 1 负 · 上一局 +18 分 | 鎏金呼吸 |

~~~text
神话II · 350分 · 3连胜 · 今日 3-1
辐能 · 500分 · 5连败 · 今日 0-0
~~~

### ALL RANKS

<p align="center">
  <img src="docs/images/rank-gallery.png" width="1000" alt="全段位展示：黑铁至辐能">
</p>

<p align="center"><sub>黑铁 → 辐能 · 官方图标 / 主色 / 层级 · 段位色会直接影响卡片背景</sub></p>

### UI ANATOMY

<p align="center">
  <img src="docs/images/ui-anatomy.png" width="1000" alt="UI 组件框选说明">
</p>

<p align="center"><sub>① 段位图标 · ② 段位 + RR · ③ 连胜/连败徽章 · ④ 今日战绩 · ⑤ 上一局变化 · ⑥ 段位色边框</sub></p>

### DESIGN SYSTEM

<p align="center">
  <img src="docs/images/ui-board.png" width="1000" alt="UI 设计系统：神话 / 辐能、鎏金连胜、红黑连败">
</p>

<p align="center"><sub>段位主色 / 鎏金连胜 / 红黑连败 / 徽章与 2.4s 同频呼吸</sub></p>

### STATUS PAGE

<p align="center">
  <img src="docs/images/status-page.png" width="720" alt="辐能状态页">
</p>

<p align="center"><sub>本机状态页：辐能 · 500 分 · 3 连胜 · 今日 3 胜 / 1 负</sub></p>

> **UI 细节**：段位主色跟着段位走（神话粉红、辐能鎏金），再叠加上一局胜负色；连胜鎏金、连败红黑。徽章与整卡同频呼吸，达到 2 场才触发。

## 03 / FEATURES

<p align="center">
  <img src="docs/images/features.png" width="1000" alt="特性效果图">
</p>

<p align="center"><sub>游戏内段位图标 · 段位/胜负动态背景 · 同频呼吸 · Windows / macOS · 本机加密 · 状态页与诊断</sub></p>

## 04 / LOCAL-FIRST SECURITY

<p align="center">
  <img src="docs/images/security.png" width="1000" alt="LOCAL-FIRST 安全效果图">
</p>

<p align="center"><sub>本机加密 · 数据不出机 · 仅 127.0.0.1 · NO CLOUD / NO UPLOAD</sub></p>

| ENCRYPTED AT REST | LOCAL ONLY | MINIMAL DATA |
|:--|:--|:--|
| Windows DPAPI<br>macOS Keychain + AES-GCM<br>本地 0600 回退 | 只监听 <code>127.0.0.1</code><br>不开放局域网<br>无云同步 | <code>/api/state</code> 不含 Token<br>不返回原始响应<br><code>--reset-login</code> 可清除 |

- Windows 凭证：<code>%LOCALAPPDATA%\ValorantCNOverlay\credentials.bin</code>
- macOS 凭证：<code>~/Library/Application Support/ValorantCNOverlay/credentials.bin</code>
- 凭证文件本身不是明文 Token；不要把数据目录上传网盘或发给他人。

## 05 / QUICK START

### Windows

1. 从 [Releases](../../releases/latest) 下载 **Windows x64 ZIP**。
2. 完整解压，双击 <code>启动段位Overlay.bat</code>。
3. QQ 扫码授权。
4. 浏览器打开预览，OBS 添加 Browser Source。

> [!NOTE]
> 不需要 Python、pip 或管理员权限。SmartScreen 可能提示未知发布者，请核对 Release 与 <code>CHECKSUMS.sha256</code>。

### macOS

1. 从 [Releases](../../releases/latest) 下载 **macOS ZIP**。
2. 完整解压，右键 <code>启动段位Overlay.command</code> → 打开。
3. QQ 扫码授权；Keychain 弹窗选择“允许”。
4. 保持终端运行，OBS 添加 Browser Source。

如果 Gatekeeper 拦截：

~~~bash
xattr -dr com.apple.quarantine /路径/到/解压目录
~~~

> [!NOTE]
> Keychain 不可用时回退到数据目录内 <code>0600</code> 本地密钥，凭证仍不是明文。

## 06 / OBS SETUP

OBS → **来源** → **+** → **浏览器**：

| 设置 | 值 |
|:--|:--|
| URL | <code>http://127.0.0.1:8787/overlay</code> |
| 宽度 | <code>420</code> |
| 高度 | <code>120</code> |

先启动程序，确认 Overlay 预览正常，再加 OBS。

## 07 / STATUS & DIAGNOSTICS

| 功能 | 使用方式 |
|:--|:--|
| 状态页 | <code>http://127.0.0.1:8787/status</code> |
| 健康检查 | <code>http://127.0.0.1:8787/health</code> |
| 重新登录 | 状态页按钮 / <code>重新登录.bat</code> / <code>重新登录.command</code> |
| 本机诊断 | <code>--diagnose</code> |

状态页只读本机缓存，不额外请求掌瓦。运行中登录失效不会自动弹二维码，需手动重新登录。

## 08 / CHANGELOG

- macOS 12+ 支持：Apple Silicon / Intel 双架构。
- 登录可靠性：网络抖动不再误删登录态；后台刷新不弹二维码。
- 二维码状态码补全：<code>65</code> 失效 / <code>53</code> 取消 / <code>66/67</code> 继续轮询。
- 状态页、<code>--diagnose</code>、显式重新登录。
- 上一局 / 连胜连败只统计本次启动后的竞技对局，重启归零。
- 完整说明：[v1.1.0 发布说明](docs/releases/v1.1.0.md)

## 09 / FAQ

<details>
<summary><b>二维码过期 / 登录失效</b></summary>

二维码真正过期会自动刷新；程序退出后重新启动即可。运行中登录失效不会自动弹二维码，请用状态页“重新登录”。

</details>

<details>
<summary><b>OBS 页面空白</b></summary>

先访问 <code>http://127.0.0.1:8787/health</code>，正常返回：

~~~json
{"ok":true}
~~~

再检查 Browser Source URL。

</details>

<details>
<summary><b>数据停止更新</b></summary>

Overlay 保留最后一次数据并显示黄色状态点。先看 <code>/status</code> 的错误/告警；登录失效就重新登录。

</details>

<details>
<summary><b>端口 8787 被占用</b></summary>

~~~powershell
.\ValorantCNOverlay.exe --port 8788
~~~

~~~bash
./ValorantCNOverlay-macos-arm64 --port 8788
~~~

OBS URL 改为 <code>http://127.0.0.1:8788/overlay</code>。

</details>

<details>
<summary><b>命令行参数</b></summary>

| 参数 | 说明 |
|:--|:--|
| <code>--port 8787</code> | 本地端口 |
| <code>--refresh 30</code> | 掌瓦刷新间隔 5–3600 秒 |
| <code>--no-open</code> | 不自动打开浏览器/二维码 |
| <code>--reset-login</code> | 删除登录态并重新扫码 |
| <code>--data-dir 目录</code> | 自定义数据目录 |
| <code>--diagnose</code> | 数据目录/安全存储/端口/连通性 |
| <code>--version</code> | 版本和 commit |

同一时间只允许一个实例。

</details>

## 10 / BUILD

~~~bash
go test ./...
go vet ./...

# macOS 双架构
./scripts/build-macos.sh

# Windows
go build -trimpath -ldflags "-s -w -X main.version=1.1.0" -o ValorantCNOverlay.exe ./src/cmd/valorant-overlay
~~~

GitHub Actions 运行测试并产出 Windows / macOS ZIP；<code>v*</code> tag 创建 Release。

## 11 / DOCS & LICENSE

- [RR 解析与诊断](docs/rr-diagnostics.md)
- [v1.1.0 发布说明](docs/releases/v1.1.0.md)
- [第三方许可](THIRD_PARTY_NOTICES.md)

本项目以 **AGPL-3.0-or-later** 发布，完整条款见 [LICENSE](LICENSE)。

<p align="center"><sub>Made for the CN Valorant community · Not affiliated with Riot Games / Tencent</sub></p>
