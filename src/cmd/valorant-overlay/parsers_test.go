// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"testing"
	"time"
)

func TestCalcPTQRTokenKnownValue(t *testing.T) {
	if got := calcPTQRToken("abc"); got != 108966 {
		t.Fatalf("calcPTQRToken(abc) = %d", got)
	}
}

func TestParsePTUICallback(t *testing.T) {
	got, ok := parsePTUICallback(`ptuiCB('0','0','https:\/\/example.test\/check_sig?x=1\x26y=2','0','登录成功')`)
	if !ok {
		t.Fatal("callback not parsed")
	}
	if got.Code != "0" {
		t.Fatalf("code=%q", got.Code)
	}
	if got.RedirectURL != `https://example.test/check_sig?x=1&y=2` {
		t.Fatalf("redirect=%q", got.RedirectURL)
	}
}

func TestExtractLoginDataFromNestedAuthURL(t *testing.T) {
	raw := `https://example.test/?u1=` + urlQueryEscape(`auth://tauth.qq.com/#access_token=tok123&openid=oid456`)
	got := extractLoginData(raw)
	if got["access_token"] != "tok123" || got["openid"] != "oid456" {
		t.Fatalf("unexpected login data: %#v", got)
	}
}

func TestParseRoleSnapshotPrefersPCAGame(t *testing.T) {
	raw := map[string]any{"list": []any{
		map[string]any{"gameId": "codev", "role_name": "手游", "tier_text": "未定级", "game_role_id": "mobile"},
		map[string]any{"gameId": "agame", "role_name": "测试玩家", "tier_text": "铂金I", "game_role_id": "pc", "scene": "secret", "role_level": 278.0},
	}}
	role := parseRoleSnapshot(raw)
	if role.RoleName != "测试玩家" || role.TierText != "铂金I" || role.GameRoleID != "pc" {
		t.Fatalf("wrong role selected: %#v", role)
	}
}

func TestExtractCurrentRRFromValCardTitle(t *testing.T) {
	card := map[string]any{"card": map[string]any{"left_data": map[string]any{"title": "铂金I 82分"}}}
	if got := extractCurrentRR(card); got != "82" {
		t.Fatalf("rr=%q", got)
	}
}

func TestExtractRRChangeAndLatestCompetitive(t *testing.T) {
	raw := map[string]any{"battle_list": []any{
		map[string]any{"game_enum": "unrated", "competitive_tier": map[string]any{"competitive_point_change": "+0"}},
		map[string]any{"game_enum": "competitive", "result_title": "失败", "competitive_tier": map[string]any{"competitive_point_change": -18.0}},
	}}
	latest := findLatestCompetitiveMatch(raw)
	if latest == nil {
		t.Fatal("no competitive match")
	}
	if got := extractRRChange(latest); got != "-18" {
		t.Fatalf("change=%q", got)
	}
	if got := normalizedMatchResult(latest); got != "失败" {
		t.Fatalf("result=%q", got)
	}
}

func TestSummarizeTodayOnlyCompetitiveAndLocalDate(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	now := time.Date(2026, 8, 12, 22, 0, 0, 0, loc)
	todayWin := time.Date(2026, 8, 12, 18, 0, 0, 0, loc).Unix()
	todayLoss := time.Date(2026, 8, 12, 19, 0, 0, 0, loc).Unix()
	yesterday := time.Date(2026, 8, 11, 23, 0, 0, 0, loc).Unix()
	raw := []any{
		map[string]any{"game_enum": "competitive", "result_title": "胜利", "ts": float64(todayWin)},
		map[string]any{"game_enum": "competitive", "result_title": "失败", "ts": float64(todayLoss)},
		map[string]any{"game_enum": "competitive", "result_title": "胜利", "ts": float64(yesterday)},
		map[string]any{"game_enum": "unrated", "result_title": "胜利", "ts": float64(todayWin)},
	}
	s := summarizeToday(raw, now)
	if s.Wins != 1 || s.Losses != 1 || s.Games != 2 {
		t.Fatalf("summary=%#v", s)
	}
}

func TestCandidateMatchListNestedDataDoesNotPanic(t *testing.T) {
	raw := map[string]any{"data": map[string]any{"battle_list": []any{map[string]any{"game_enum": "competitive"}}}}
	got := candidateMatchList(raw)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
}
