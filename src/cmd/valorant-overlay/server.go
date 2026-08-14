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

type OverlaySnapshot struct {
	Rank         string `json:"rank"`
	RR           string `json:"rr"`
	TodayWins    int    `json:"today_wins"`
	TodayLosses  int    `json:"today_losses"`
	LastResult   string `json:"last_result"`
	LastRRChange string `json:"last_rr_change"`
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

func NewOverlayHandler(state *OverlayState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(state.Get())
	})
	page := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(overlayHTML)
	}
	mux.HandleFunc("/", page)
	mux.HandleFunc("/overlay", page)
	return mux
}
