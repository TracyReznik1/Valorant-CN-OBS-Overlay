// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"path/filepath"
	"testing"
	"time"
)

type fakeProbe struct {
	role       any
	card       any
	pages      map[int]any
	login      Credentials
	loginCalls int
}

func (f *fakeProbe) QQLogin() (Credentials, error)                       { f.loginCalls++; return f.login, nil }
func (f *fakeProbe) FetchMainRole(Credentials) (any, error)              { return f.role, nil }
func (f *fakeProbe) FetchValCard(Credentials, RoleSnapshot) (any, error) { return f.card, nil }
func (f *fakeProbe) FetchRecentMatches(_ Credentials, _ RoleSnapshot, page, size int) (any, error) {
	return f.pages[page], nil
}

func TestOverlayServiceRefreshBuildsSnapshot(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 8, 12, 22, 0, 0, 0, loc)
	ts := time.Date(2026, 8, 12, 20, 0, 0, 0, loc).Unix()
	fp := &fakeProbe{
		role:  map[string]any{"list": []any{map[string]any{"gameId": "agame", "role_name": "测试玩家", "tier_text": "铂金I", "game_role_id": "pc", "scene": "s"}}},
		card:  map[string]any{"card": map[string]any{"left_data": map[string]any{"title": "铂金I 82分", "image_url": "https://example/icon.png"}}},
		pages: map[int]any{1: map[string]any{"battle_list": []any{map[string]any{"game_enum": "competitive", "result_title": "失败", "ts": float64(ts), "competitive_tier": map[string]any{"competitive_point_change": -18.0}}}}},
		login: Credentials{UserID: "u", TID: "t", OpenID: "o", AccessToken: "a"},
	}
	store := CredentialStore{Path: filepath.Join(t.TempDir(), "c.bin"), Protector: prefixProtector{}}
	state := NewOverlayState()
	svc := OverlayService{State: state, Store: store, NewProbe: func() (ProbeAPI, error) { return fp, nil }, Now: func() time.Time { return now }}
	got, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rank != "铂金I" || got.RR != "82" || got.TodayLosses != 1 || got.LastRRChange != "-18" || got.RankIcon == "" {
		t.Fatalf("snapshot=%#v", got)
	}
	if fp.loginCalls != 1 {
		t.Fatalf("loginCalls=%d", fp.loginCalls)
	}
	_, ok, err := store.Load()
	if err != nil || !ok {
		t.Fatalf("credentials not cached ok=%v err=%v", ok, err)
	}
}
