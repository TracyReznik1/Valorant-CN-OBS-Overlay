// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"sync"
)

//go:embed static/overlay.html
var overlayHTML []byte

//go:embed static/status.html
var statusHTML []byte

type OverlaySnapshot struct {
	Rank         string `json:"rank"`
	RR           string `json:"rr"`
	RRSource     string `json:"rr_source,omitempty"`
	Warning      string `json:"warning,omitempty"`
	TodayWins    int    `json:"today_wins"`
	TodayLosses  int    `json:"today_losses"`
	LastResult   string `json:"last_result"`
	LastRRChange string `json:"last_rr_change"`
	StreakType   string `json:"streak_type,omitempty"`
	StreakCount  int    `json:"streak_count,omitempty"`
	RankIcon     string `json:"rank_icon"`
	UpdatedAt    string `json:"updated_at"`
	Stale        bool   `json:"stale"`
	Error        string `json:"error"`
	TodayPartial bool   `json:"today_partial"`
}

type OverlayState struct {
	mu       sync.RWMutex
	snapshot OverlaySnapshot
}

func NewOverlayState() *OverlayState {
	return &OverlayState{snapshot: OverlaySnapshot{Rank: "--", RR: "--", LastResult: "--", LastRRChange: "--", Stale: true, Error: "等待首次刷新"}}
}
func (s *OverlayState) Update(v OverlaySnapshot) {
	s.mu.Lock()
	v.Stale = false
	v.Error = ""
	s.snapshot = v
	s.mu.Unlock()
}
func (s *OverlayState) MarkStale(err string) {
	s.mu.Lock()
	v := s.snapshot
	v.Stale = true
	v.Error = err
	s.snapshot = v
	s.mu.Unlock()
}
func (s *OverlayState) Get() OverlaySnapshot { s.mu.RLock(); defer s.mu.RUnlock(); return s.snapshot }

func secureHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func NewOverlayHandler(state *OverlayState, relogin ...func() bool) http.Handler {
	var onRelogin func() bool
	if len(relogin) > 0 {
		onRelogin = relogin[0]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(state.Get())
	})
	mux.HandleFunc("/api/relogin", func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "message": "只支持 POST 请求"})
			return
		}
		if onRelogin == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "message": "当前实例未启用重新登录"})
			return
		}
		if !onRelogin() {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "程序正在初始化或已有重新登录请求，请稍后再试"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "已提交重新登录请求，请查看程序窗口或二维码"})
	})

	page := func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src http: https: data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'")
		_, _ = w.Write(overlayHTML)
	}
	status := func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'")
		_, _ = w.Write(statusHTML)
	}
	mux.HandleFunc("/overlay", page)
	mux.HandleFunc("/status", status)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(w)
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page(w, r)
	})
	return mux
}
