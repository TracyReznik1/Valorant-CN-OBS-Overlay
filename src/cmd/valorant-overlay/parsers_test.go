// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func valCardTitle(title string) map[string]any {
	return map[string]any{"card": map[string]any{"left_data": map[string]any{"title": title}}}
}

func TestExtractCurrentRank(t *testing.T) {
	for _, tc := range []struct {
		name  string
		role  map[string]any
		title string
		want  string
	}{
		// Minimal display-only regression input based on local diagnostics:
		// tier_text is empty, tier is numeric 255, card says unranked/0 points.
		{"observedUnrankedEnum", map[string]any{"tier_text": "", "tier": 255.0}, "未定级 0分", "未定级"},
		{"stringEnum", map[string]any{"tier_text": "255", "tier": "255"}, "未定级0分", "未定级"},
		{"namedRole", map[string]any{"tier_text": "铂金I", "tier": 255.0}, "神话II 350分", "铂金I"},
		{"namedAlias", map[string]any{"tier_text": "255", "tier": "神话II"}, "", "神话II"},
		{"missingRole", map[string]any{}, "辐能战魂 1250分", "辐能战魂"},
		{"pointsNotDivision", map[string]any{}, "神话350分", "神话"},
		{"numericTierWithRankedCard", map[string]any{"tier": 17.0}, "铂金II 82分", "铂金II"},
		{"doNotGuess255", map[string]any{"tier": 255.0}, "", ""},
		{"doNotGuessOtherEnum", map[string]any{"tier": 17.0}, "", ""},
		{"invalidObject", map[string]any{"tier_text": map[string]any{"tier": "未定级"}}, "", ""},
		{"unknownText", map[string]any{"tier_text": "invalid"}, "暂无段位信息", ""},
		{"unrankedNoPoints", map[string]any{}, "未定级", "未定级"},
		{"unratedAlias", map[string]any{"tier_text": "未评级"}, "", "未评级"},
		{"noRankAlias", map[string]any{"tier_text": "无段位"}, "", "无段位"},
		{"spacedName", map[string]any{}, " 神话 II 350 RR ", "神话 II"},
		{"unicodeTier", map[string]any{}, "钻石Ⅲ 82分", "钻石Ⅲ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Include a role marker so the existing role parser recognises the map.
			tc.role["game_role_id"] = "test-role"
			if got := extractCurrentRank(parseRoleSnapshot(tc.role), valCardTitle(tc.title)); got != tc.want {
				t.Fatalf("rank=%q, want %q", got, tc.want)
			}
		})
	}
}

// These are synthetic regression inputs using paths already recognised by the
// original code. High-rank values/formats are hypothetical, not API captures.
func TestCurrentRRValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  any
		want string
	}{
		{"normal82", valCardTitle("铂金I 82分"), "82"},
		{"hypotheticalImmortal185", valCardTitle("神话II 185分"), "185"},
		{"hypotheticalImmortal350", valCardTitle("神话III 350分"), "350"},
		{"hypotheticalRadiant1250", valCardTitle("辐能战魂 1250分"), "1250"},
		{"hypotheticalRadiant10050NoSuffixTruncation", valCardTitle("辐能战魂 10050分"), "10050"},
		{"string", map[string]any{"league_point": "185"}, "185"},
		{"number", map[string]any{"league_point": 350.0}, "350"},
		{"jsonNumber", map[string]any{"rr": json.Number("1250")}, "1250"},
		{"zero", map[string]any{"rr": 0}, "0"},
		{"missing", map[string]any{}, ""},
		{"nilSource", nil, ""},
		{"null", map[string]any{"rr": nil}, ""},
		{"empty", map[string]any{"rr": " "}, ""},
		{"unknown", valCardTitle("神话II 暂无积分"), ""},
		{"detailPoints", map[string]any{"rank_detail": "神话II 350 RR"}, "350"},
		{"seasonPoints", map[string]any{"season_rank_detail": "辐能战魂 1250分"}, "1250"},
		{"slash", map[string]any{"rank_detail": "82/100"}, "82"},
		{"slashZero", map[string]any{"rank_detail": "0 / 100"}, "0"},
		{"slash100", valCardTitle("铂金I 100 / 100"), "100"},
		{"slashWithUnit", map[string]any{"rank_detail": "82 / 100 RR"}, "82"},
		{"slashOver100", map[string]any{"rank_detail": "185/100"}, ""},
		{"slashOver100WithUnit", map[string]any{"rank_detail": "185 / 100分"}, ""},
		{"wrongDenominator", map[string]any{"rank_detail": "82/1000"}, ""},
		{"textNegative", valCardTitle("神话II -18分"), ""},
		{"textSpacedNegative", valCardTitle("神话II - 18分"), ""},
		{"textFraction", valCardTitle("神话II 18.5分"), ""},
		{"textGroupedNotSupported", valCardTitle("辐能战魂 1,250分"), ""},
		{"textOverflow", valCardTitle("99999999999999999999999999999分"), ""},
		{"invalidObject", map[string]any{"rr": map[string]any{"value": 350}}, ""},
		{"invalidTextObject", map[string]any{"rank_detail": map[string]any{"title": "350分"}}, ""},
		{"noHistorySearch", map[string]any{"history": map[string]any{"rr": 350}}, ""},
		{"noOtherPlayerSearch", map[string]any{"players": []any{map[string]any{"rr": 350}}}, ""},
		{"noHistoryTextSearch", map[string]any{"history": map[string]any{"rank_detail": "350分"}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractCurrentRR(tc.src); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCurrentRRRejectsInvalidStructuredValues(t *testing.T) {
	for _, value := range []any{"unknown", "NaN", "Inf", "-1", -1, 18.5, "18.5", true, math.NaN(), math.Inf(1), float64(1 << 53), "9223372036854775808"} {
		if got := extractCurrentRR(map[string]any{"league_point": value}); got != "" {
			t.Errorf("invalid %T value produced %q", value, got)
		}
		if got := extractCurrentRR(map[string]any{"league_point": value}, valCardTitle("82分")); got != "82" {
			t.Errorf("invalid %T value blocked valid fallback: %q", value, got)
		}
	}
}

func TestCurrentRRSupportedKeys(t *testing.T) {
	for _, key := range rrKeys {
		for _, value := range []any{" 00185 ", 185.0, 185, int64(185)} {
			if got := extractCurrentRR(map[string]any{key: value}); got != "185" {
				t.Errorf("key %s/%T: got %q", key, value, got)
			}
		}
	}
}

func TestCurrentRRPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		srcs []any
		want string
	}{
		{"structuredOverProgress", []any{map[string]any{"league_point": 350, "rank_detail": "82/100"}}, "350"},
		{"structuredOtherSourceOverProgress", []any{map[string]any{"rank_detail": "82/100"}, map[string]any{"rr": "1250"}}, "1250"},
		{"titleOverProgress", []any{map[string]any{"rank_detail": "82/100"}, valCardTitle("神话II 350分")}, "350"},
		{"detailPointsOverProgress", []any{map[string]any{"rank_detail": "82/100", "season_rank_detail": "350 RR"}}, "350"},
		{"sameTextPointsOverProgress", []any{map[string]any{"rank_detail": "82/100，当前350 RR"}}, "350"},
		{"preserveStructuredPrecedence", []any{map[string]any{"league_point": 185}, valCardTitle("350分")}, "185"},
		{"preserveKeyPrecedence", []any{map[string]any{"league_point": 185, "rr": 350}}, "185"},
		{"invalidFirstKey", []any{map[string]any{"league_point": "invalid", "rr": 350}}, "350"},
		{"zeroIsValid", []any{map[string]any{"league_point": 0}, valCardTitle("350分")}, "0"},
		{"knownCardScope", []any{map[string]any{"card": map[string]any{"rr": 350}}}, "350"},
		{"knownLeftScope", []any{map[string]any{"card": map[string]any{"left_data": map[string]any{"rr": 350}}}}, "350"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractCurrentRR(tc.srcs...); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCurrentRRUsesSelectedRoleOnly(t *testing.T) {
	roles := []any{
		map[string]any{"gameId": "codev", "rr": 99},
		map[string]any{"gameId": "agame", "league_point": 350},
	}
	for _, src := range []any{roles, map[string]any{"list": roles}} {
		if got := extractCurrentRR(src); got != "350" {
			t.Fatalf("selected RR = %q", got)
		}
	}
	delete(roles[1].(map[string]any), "league_point")
	if got := extractCurrentRR(map[string]any{"list": roles}); got != "" {
		t.Fatalf("used another role's RR: %q", got)
	}
}

func TestRRChangeValues(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{{22, "+22"}, {"+22", "+22"}, {-18.0, "-18"}, {"-18", "-18"}, {0, "0"}, {"+0", "0"}, {nil, ""}, {"invalid", ""}, {1.5, ""}} {
		match := map[string]any{"competitive_tier": map[string]any{"competitive_point_change": tc.value}}
		if got := extractRRChange(match); got != tc.want {
			t.Errorf("%T: got %q, want %q", tc.value, got, tc.want)
		}
	}
	if got := extractRRChange(nil); got != "" {
		t.Fatalf("nil match = %q", got)
	}
	if got := extractRRChange(map[string]any{"players": []any{map[string]any{"competitive_point_change": 22}}}); got != "" {
		t.Fatalf("used another player's change: %q", got)
	}
	if got := extractRRChange(map[string]any{"competitive_tier": map[string]any{"competitive_point_change": "bad"}, "rank_rating_change": -18}); got != "-18" {
		t.Fatalf("fallback change = %q", got)
	}
}

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
	matches, ok := parseMatchList(raw)
	if !ok {
		t.Fatal("match list not parsed")
	}
	latest := findLatestCompetitiveMatch(matches)
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
	matches, ok := parseMatchList(raw)
	if !ok {
		t.Fatal("match list not parsed")
	}
	s := summarizeToday(matches, now)
	if s.Wins != 1 || s.Losses != 1 || s.Games != 2 {
		t.Fatalf("summary=%#v", s)
	}
}

func TestParseMatchListNestedData(t *testing.T) {
	raw := map[string]any{"data": map[string]any{"battle_list": []any{map[string]any{"game_enum": "competitive"}}}}
	got, ok := parseMatchList(raw)
	if !ok || len(got) != 1 {
		t.Fatalf("ok=%t len=%d", ok, len(got))
	}
}

func TestParseMatchListKnownContainers(t *testing.T) {
	for _, key := range []string{"battle_list", "list", "records", "matches"} {
		for _, list := range []any{[]any{}, []map[string]any{}, []any{map[string]any{"game_enum": "competitive"}}} {
			if _, ok := parseMatchList(map[string]any{"data": map[string]any{key: list}}); !ok {
				t.Fatalf("valid list %s/%T rejected", key, list)
			}
		}
	}
}
