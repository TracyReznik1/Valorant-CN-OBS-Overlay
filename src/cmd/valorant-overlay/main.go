// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"syscall"
	"time"
)

var version = "1.1.0"

type Options struct {
	Port           int
	RefreshSeconds int
	NoOpen         bool
	ResetLogin     bool
	ShowVersion    bool
	Diagnose       bool
	DataDir        string
}

func parseOptions(args []string) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("ValorantCNOverlay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&o.Port, "port", 8787, "本地 Overlay HTTP 端口")
	fs.IntVar(&o.RefreshSeconds, "refresh", 30, "掌瓦后台刷新间隔（秒）")
	fs.BoolVar(&o.NoOpen, "no-open", false, "启动后不自动打开预览页或二维码")
	fs.BoolVar(&o.ResetLogin, "reset-login", false, "清除登录缓存并重新扫码")
	fs.BoolVar(&o.ShowVersion, "version", false, "显示版本")
	fs.BoolVar(&o.Diagnose, "diagnose", false, "运行本机诊断（数据目录、DPAPI、端口、网络）")
	fs.StringVar(&o.DataDir, "data-dir", "", "登录缓存与临时文件目录（默认使用系统应用数据目录）")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.Port < 1 || o.Port > 65535 {
		return o, fmt.Errorf("端口必须在 1-65535 之间")
	}
	if o.RefreshSeconds < 5 {
		return o, fmt.Errorf("刷新间隔不能小于 5 秒")
	}
	if o.RefreshSeconds > 3600 {
		return o, fmt.Errorf("刷新间隔不能大于 3600 秒")
	}
	return o, nil
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resolveDataDir defaults to LocalAppData on Windows and Application Support
// on macOS, so credentials survive running the binary from a read-only or
// synced folder. Existing portable .data installs keep working.
func resolveDataDir(explicit, appDir string) (string, error) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	base, err := userDataBase()
	if err != nil {
		return filepath.Join(appDir, ".data"), nil
	}
	preferred := filepath.Join(base, "ValorantCNOverlay")
	if fileExists(filepath.Join(preferred, "credentials.bin")) {
		return preferred, nil
	}
	legacy := filepath.Join(appDir, ".data")
	if fileExists(filepath.Join(legacy, "credentials.bin")) {
		return legacy, nil
	}
	return preferred, nil
}

func userDataBase() (string, error) {
	if runtime.GOOS == "windows" {
		return os.UserCacheDir()
	}
	return os.UserConfigDir()
}

func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func printVersion() {
	fmt.Println("Valorant CN OBS Overlay v" + version)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				fmt.Println("commit:", setting.Value)
			}
		}
	}
}

func run(args []string) int {
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Println("❌ 参数错误：" + err.Error())
		fmt.Println("可用参数：--port 8787 --refresh 30 --no-open --reset-login --data-dir <目录> --diagnose --version")
		return 2
	}
	if opts.ShowVersion {
		printVersion()
		return 0
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		fmt.Println("❌ 此发布版仅支持 Windows 10/11 与 macOS 12+")
		return 2
	}

	appDir := executableDir()
	dataDir, err := resolveDataDir(opts.DataDir, appDir)
	if err != nil {
		fmt.Println("❌ 无法确定数据目录：" + err.Error())
		return 3
	}
	if opts.Diagnose {
		return diagnose(opts.Port, dataDir)
	}
	if err := ensureWritableDir(dataDir); err != nil {
		fmt.Println("❌ 数据目录不可写：" + dataDir)
		fmt.Println("   " + err.Error())
		fmt.Println("   可用 --data-dir 指定其他目录。")
		return 3
	}

	lock, acquired, err := acquireInstanceLock()
	if err != nil {
		fmt.Println("❌ 无法创建单实例锁：" + err.Error())
		return 3
	}
	if !acquired {
		fmt.Println("已有 Overlay 实例在运行。请直接使用现有窗口；如需重启，请先关闭旧实例。")
		return 5
	}
	defer lock.Release()

	store := CredentialStore{Path: filepath.Join(dataDir, "credentials.bin"), Protector: defaultProtector(dataDir)}
	if store.Protector == nil {
		fmt.Println("❌ 系统安全存储不可用，拒绝以明文保存登录凭证")
		return 3
	}
	if opts.ResetLogin {
		if err := store.Delete(); err != nil {
			fmt.Println("❌ 清除登录缓存失败：" + err.Error())
			return 3
		}
		if runtime.GOOS == "darwin" {
			_ = os.Remove(filepath.Join(dataDir, "credentials.key"))
		}
		fmt.Println("已清除本机登录缓存，将重新 QQ 扫码。")
	}

	state := NewOverlayState()
	reloginCh := make(chan struct{}, 1)
	svc := &OverlayService{
		State:           state,
		Store:           store,
		SessionStart:    time.Now(),
		RefreshInterval: time.Duration(opts.RefreshSeconds) * time.Second,
		RoleCacheTTL:    5 * time.Minute,
		ReloginCh:       reloginCh,
		Now:             time.Now,
		NewProbe: func() (ProbeAPI, error) {
			return newProbe(ProbeConfig{AutoOpen: !opts.NoOpen, LoginTimeout: 50 * time.Second, TempDir: filepath.Join(dataDir, "tmp")})
		},
	}
	var canRelogin atomic.Bool
	handler := NewOverlayHandler(state, func() bool {
		if !canRelogin.Load() {
			return false
		}
		return svc.RequestRelogin()
	})

	addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)
	fmt.Println("[1/3] 正在启动本地 Overlay 服务…")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("❌ 无法监听 " + addr + "：" + err.Error())
		return 4
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("⚠️ 本地服务异常：" + err.Error())
		}
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/overlay", opts.Port)
	fmt.Println("[2/3] 本地 Overlay 已启动")
	fmt.Println("      URL: " + url)
	fmt.Println("      状态/重新登录: " + fmt.Sprintf("http://127.0.0.1:%d/status", opts.Port))
	fmt.Println("[3/3] OBS → 来源 → 浏览器 → URL 填上面地址")
	fmt.Println("      建议宽度 420，高度 120")
	fmt.Println("      数据目录: " + dataDir)
	if opts.NoOpen {
		fmt.Println("      首次运行或登录失效时会生成 QQ 二维码并打印路径；关闭窗口或按 Ctrl+C 可停止。")
	} else {
		fmt.Println("      首次运行或登录失效时会弹出 QQ 二维码；关闭窗口或按 Ctrl+C 可停止。")
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	stopRefresh := make(chan struct{})
	startResult := make(chan error, 1)
	go func() {
		startResult <- svc.Start(stopRefresh)
	}()

	shutdown := func() {
		canRelogin.Store(false)
		close(stopRefresh)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = os.RemoveAll(filepath.Join(dataDir, "tmp"))
	}

	select {
	case <-signals:
		fmt.Println()
		fmt.Println("正在停止 Overlay…")
		shutdown()
		return 0
	case startErr := <-startResult:
		if startErr != nil {
			var pe *ProbeError
			if errors.As(startErr, &pe) {
				fmt.Println("❌ [" + pe.Stage + "] " + pe.Message)
			} else {
				fmt.Println("❌ 启动失败：" + startErr.Error())
			}
			shutdown()
			return 3
		}
	}

	snapshot := state.Get()
	rrText := snapshot.RR
	if rrText != "--" {
		rrText += " 分"
	}
	lastText := snapshot.LastRRChange
	if lastText != "--" {
		lastText += " 分"
	}
	fmt.Println()
	fmt.Println("      当前：" + snapshot.Rank + " " + rrText + " | " +
		fmt.Sprintf("今日 %d 胜 / %d 负", snapshot.TodayWins, snapshot.TodayLosses) +
		" | 上一局 " + lastText)
	if !opts.NoOpen {
		_ = openPath(url)
	}

	canRelogin.Store(true)
	go svc.Run(stopRefresh)
	<-signals
	fmt.Println()
	fmt.Println("正在停止 Overlay…")
	shutdown()
	return 0
}

func main() { os.Exit(run(os.Args[1:])) }
