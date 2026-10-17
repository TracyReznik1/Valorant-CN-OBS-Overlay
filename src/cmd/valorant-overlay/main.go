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
	"syscall"
	"time"
)

const version = "1.0.1"

type Options struct {
	Port           int
	RefreshSeconds int
	NoOpen         bool
	ResetLogin     bool
	ShowVersion    bool
}

func parseOptions(args []string) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("ValorantCNOverlay", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&o.Port, "port", 8787, "本地 Overlay HTTP 端口")
	fs.IntVar(&o.RefreshSeconds, "refresh", 30, "掌瓦后台刷新间隔（秒）")
	fs.BoolVar(&o.NoOpen, "no-open", false, "启动后不自动打开预览页")
	fs.BoolVar(&o.ResetLogin, "reset-login", false, "清除登录缓存并重新扫码")
	fs.BoolVar(&o.ShowVersion, "version", false, "显示版本")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.Port < 1 || o.Port > 65535 {
		return o, fmt.Errorf("端口必须在 1-65535 之间")
	}
	if o.RefreshSeconds < 5 {
		return o, fmt.Errorf("刷新间隔不能小于 5 秒")
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

func run(args []string) int {
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Printf("❌ 参数错误：%v\n", err)
		fmt.Println("可用参数：--port 8787 --refresh 30 --no-open --reset-login --version")
		return 2
	}
	if opts.ShowVersion {
		fmt.Printf("Valorant CN OBS Overlay v%s\n", version)
		return 0
	}
	if runtime.GOOS != "windows" {
		fmt.Println("❌ 此发布版仅支持 Windows 10/11 x64")
		return 2
	}
	appDir := executableDir()
	store := CredentialStore{Path: filepath.Join(appDir, ".data", "credentials.bin"), Protector: defaultProtector()}
	if store.Protector == nil {
		fmt.Println("❌ Windows DPAPI 不可用，拒绝以明文保存登录凭证")
		return 3
	}
	if opts.ResetLogin {
		if err := store.Delete(); err != nil {
			fmt.Printf("❌ 清除登录缓存失败：%v\n", err)
			return 3
		}
		fmt.Println("已清除本机登录缓存，将重新 QQ 扫码。")
	}

	state := NewOverlayState()
	svc := &OverlayService{State: state, Store: store, RefreshInterval: time.Duration(opts.RefreshSeconds) * time.Second, Now: time.Now, NewProbe: func() (ProbeAPI, error) {
		return newProbe(ProbeConfig{AutoOpen: true, LoginTimeout: 50 * time.Second, TempDir: filepath.Join(appDir, "tmp")})
	}}
	fmt.Println("[1/3] 正在读取掌瓦数据；首次运行或登录失效时会弹出 QQ 二维码…")
	snapshot, err := svc.RefreshOnce()
	if err != nil {
		var pe *ProbeError
		if errors.As(err, &pe) {
			fmt.Printf("❌ [%s] %s\n", pe.Stage, pe.Message)
		} else {
			fmt.Printf("❌ 启动失败：%v\n", err)
		}
		return 3
	}

	addr := fmt.Sprintf("127.0.0.1:%d", opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("❌ 无法监听 %s：%v\n", addr, err)
		return 4
	}
	server := &http.Server{Handler: NewOverlayHandler(state), ReadHeaderTimeout: 5 * time.Second}
	stopRefresh := make(chan struct{})
	go svc.Run(stopRefresh)
	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("⚠️ 本地服务异常：%v\n", err)
		}
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/overlay", opts.Port)
	fmt.Println("[2/3] 本地 Overlay 已启动")
	fmt.Printf("      URL: %s\n", url)
	fmt.Println("[3/3] OBS → 来源 → 浏览器 → URL 填上面地址")
	fmt.Println("      建议宽度 420，高度 120")
	fmt.Printf("      当前：%s %s RR | %d Win / %d Loss | Last Match %s\n", snapshot.Rank, snapshot.RR, snapshot.TodayWins, snapshot.TodayLosses, snapshot.LastRRChange)
	fmt.Println("      关闭此窗口或按 Ctrl+C 可停止。")
	if !opts.NoOpen {
		_ = openPath(url)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	fmt.Println("\n正在停止 Overlay…")
	close(stopRefresh)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = os.RemoveAll(filepath.Join(appDir, "tmp"))
	return 0
}

func main() { os.Exit(run(os.Args[1:])) }
