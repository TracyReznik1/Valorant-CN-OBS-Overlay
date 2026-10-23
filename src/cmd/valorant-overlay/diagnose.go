// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func supportedPlatform() bool {
	switch runtime.GOOS {
	case "windows":
		return runtime.GOARCH == "amd64"
	case "darwin":
		return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64"
	default:
		return false
	}
}

// diagnose performs read-only (or directory-creating) checks without touching
// the running instance lock, so it can be used while the Overlay is open.
func diagnose(port int, dataDir string) int {
	var results []bool
	add := func(name string, ok bool, detail string) {
		mark := "✅"
		if !ok {
			mark = "❌"
		}
		fmt.Println(mark + " " + name + "：" + detail)
		results = append(results, ok)
	}

	add("运行平台", supportedPlatform(), runtime.GOOS+"/"+runtime.GOARCH)
	add("版本", true, version)

	if err := ensureWritableDir(dataDir); err != nil {
		add("数据目录", false, dataDir+"；"+err.Error())
	} else {
		add("数据目录", true, dataDir)
	}

	protector := defaultProtector(dataDir)
	protectorName := "系统安全存储"
	if runtime.GOOS == "windows" {
		protectorName = "Windows DPAPI"
	} else if runtime.GOOS == "darwin" {
		protectorName = "macOS Keychain + AES-GCM"
	}
	if protector == nil {
		add("凭证保护", false, protectorName+"不可用")
	} else {
		testValue := []byte("valorant-overlay-diagnose")
		encrypted, err := protector.Protect(testValue)
		if err != nil {
			add("凭证保护", false, "加密自检失败："+err.Error())
		} else if decrypted, err := protector.Unprotect(encrypted); err != nil || !bytes.Equal(decrypted, testValue) {
			add("凭证保护", false, "解密自检失败")
		} else {
			add("凭证保护", true, protectorName+"（自检通过）")
		}
	}

	credentialPath := filepath.Join(dataDir, "credentials.bin")
	if _, err := os.Stat(credentialPath); os.IsNotExist(err) {
		add("登录缓存", true, "尚未登录（首次运行正常）")
	} else if err != nil {
		add("登录缓存", false, err.Error())
	} else if protector == nil {
		add("登录缓存", false, "文件存在，但 DPAPI 不可用")
	} else {
		store := CredentialStore{Path: credentialPath, Protector: protector}
		if _, ok, err := store.Load(); err != nil {
			add("登录缓存", false, "无法解密："+err.Error())
		} else if !ok {
			add("登录缓存", false, "文件存在但无法识别")
		} else {
			add("登录缓存", true, "已加密保存")
		}
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if listener, err := net.Listen("tcp", addr); err != nil {
		add("端口", true, addr+" 被占用（若 Overlay 正在运行则正常）")
	} else {
		_ = listener.Close()
		add("端口", true, addr+" 可用")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodHead, defaultAPIBase+"/", nil)
	if err != nil {
		add("掌瓦接口连通性", false, "请求构造失败")
	} else {
		resp, err := client.Do(req)
		if err != nil {
			add("掌瓦接口连通性", false, safeRequestError(err))
		} else {
			_ = resp.Body.Close()
			add("掌瓦接口连通性", true, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
	}

	failed := 0
	for _, ok := range results {
		if !ok {
			failed++
		}
	}
	fmt.Println()
	if failed > 0 {
		fmt.Printf("诊断完成：%d 项通过，%d 项失败。\n", len(results)-failed, failed)
		return 1
	}
	fmt.Printf("诊断完成：%d 项全部通过。\n", len(results))
	return 0
}
