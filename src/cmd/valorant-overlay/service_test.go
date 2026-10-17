// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeProbe struct {
	role       any
	card       any
	pages      map[int]any
	login      Credentials
	loginCalls int
	roleErr    error
	cardErr    error
	matchErr   error
	loginErr   error
}

func (f *fakeProbe) QQLogin() (Credentials, error)                       { f.loginCalls++; return f.login, f.loginErr }
func (f *fakeProbe) FetchMainRole(Credentials) (any, error)              { return f.role, f.roleErr }
func (f *fakeProbe) FetchValCard(Credentials, RoleSnapshot) (any, error) { return f.card, f.cardErr }
func (f *fakeProbe) FetchRecentMatches(_ Credentials, _ RoleSnapshot, page, size int) (any, error) {
	return f.pages[page], f.matchErr
}

func testRefreshService(t *testing.T) (*OverlayService, *fakeProbe) {
	t.Helper()
	fp := &fakeProbe{
		role: map[string]any{"list": []any{
			map[string]any{"gameId": "codev", "rr": 99},
			map[string]any{"gameId": "agame", "tier_text": "神话II", "game_role_id": "pc", "scene": "test-scene"},
		}},
		// Hypothetical high-rank title using the existing card path.
		card:  valCardTitle("神话II 350分"),
		pages: map[int]any{1: map[string]any{"battle_list": []any{}}},
		login: Credentials{UserID: "test-user", TID: "test-tid"},
	}
	svc := &OverlayService{
		State:    NewOverlayState(),
		Store:    CredentialStore{Path: filepath.Join(t.TempDir(), "credentials.bin"), Protector: prefixProtector{}},
		NewProbe: func() (ProbeAPI, error) { return fp, nil },
		Now:      func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)) },
	}
	return svc, fp
}

func TestFetchMatchPagesKeepsSummaryAndPagination(t *testing.T) {
	svc, fp := testRefreshService(t)
	now := svc.now()
	fullPage := make([]any, 10)
	for i := range fullPage {
		fullPage[i] = map[string]any{
			"game_enum": "competitive", "result_title": "胜利", "ts": now.Unix(),
			"competitive_tier": map[string]any{"competitive_point_change": 22},
		}
	}
	loss := map[string]any{
		"game_enum": "competitive", "result_title": "失败", "ts": now.Unix(),
		"competitive_tier": map[string]any{"competitive_point_change": -18},
	}
	yesterday := map[string]any{"game_enum": "competitive", "result_title": "胜利", "ts": now.AddDate(0, 0, -1).Unix()}
	for _, tc := range []struct {
		name       string
		pages      map[int]any
		count      int
		partial    bool
		summary    TodaySummary
		lastChange string
	}{
		{"empty", map[int]any{1: []any{}}, 0, false, TodaySummary{}, ""},
		{"shortPage", map[int]any{1: []any{loss}}, 1, false, TodaySummary{Losses: 1, Games: 1}, "-18"},
		{"crossMidnight", map[int]any{1: fullPage, 2: []any{loss, yesterday}}, 12, false, TodaySummary{Wins: 10, Losses: 1, Games: 11}, "+22"},
		{"threePageLimit", map[int]any{1: fullPage, 2: fullPage, 3: fullPage}, 30, true, TodaySummary{Wins: 30, Games: 30}, "+22"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fp.pages = tc.pages
			matches, partial, err := svc.fetchMatchPages(fp, Credentials{}, RoleSnapshot{})
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != tc.count || partial != tc.partial {
				t.Fatalf("count=%d partial=%t, want %d/%t", len(matches), partial, tc.count, tc.partial)
			}
			if got := summarizeToday(matches, now); got != tc.summary {
				t.Fatalf("summary=%+v, want %+v", got, tc.summary)
			}
			if got := extractRRChange(findLatestCompetitiveMatch(matches)); got != tc.lastChange {
				t.Fatalf("last change=%q, want %q", got, tc.lastChange)
			}
		})
	}
}

func TestRefreshFailureRetainsSnapshotAndRecovers(t *testing.T) {
	for _, stage := range []string{"role_info", "val_card", "matches", "qq_login", "new_probe"} {
		t.Run(stage, func(t *testing.T) {
			svc, fp := testRefreshService(t)
			before, err := svc.RefreshOnce()
			if err != nil {
				t.Fatal(err)
			}
			if before.Stale || before.Warning != "" || before.RR != "350" {
				t.Fatalf("before=%+v", before)
			}
			failure := &ProbeError{stage, "测试请求失败"}
			switch stage {
			case "role_info":
				fp.roleErr = failure
			case "val_card":
				fp.cardErr = failure
			case "matches":
				fp.matchErr = failure
			case "qq_login":
				fp.roleErr = errors.New("test expired")
				fp.loginErr = failure
			case "new_probe":
				svc.NewProbe = func() (ProbeAPI, error) { return nil, failure }
			}
			if _, err := svc.RefreshOnce(); err == nil {
				t.Fatal("expected failure")
			}
			stale := svc.State.Get()
			if !stale.Stale || !strings.Contains(stale.Error, "["+stage+"]") {
				t.Fatalf("stale=%+v", stale)
			}
			stale.Stale = false
			stale.Error = ""
			if stale != before {
				t.Fatalf("cached snapshot changed: %+v", stale)
			}
			fp.roleErr, fp.cardErr, fp.matchErr, fp.loginErr = nil, nil, nil, nil
			svc.NewProbe = func() (ProbeAPI, error) { return fp, nil }
			fp.card = valCardTitle("神话II 351分")
			recovered, err := svc.RefreshOnce()
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Stale || recovered.Error != "" || recovered.Warning != "" || recovered.RR != "351" {
				t.Fatalf("not recovered: %+v", recovered)
			}
		})
	}
}

func TestRRMissingIsDiagnosedSeparatelyFromAPIFailure(t *testing.T) {
	svc, fp := testRefreshService(t)
	fp.card = valCardTitle("神话II 暂无积分")
	partial, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if partial.Stale || partial.Error != "" || partial.RR != "--" || partial.RRSource != "" || !strings.HasPrefix(partial.Warning, "[rr_parse]") {
		t.Fatalf("missing RR not diagnosed: %+v", partial)
	}
	fp.cardErr = &ProbeError{"val_card", "测试请求失败"}
	if _, err := svc.RefreshOnce(); err == nil {
		t.Fatal("expected failure")
	}
	if stale := svc.State.Get(); !stale.Stale || stale.Warning != partial.Warning || stale.RR != "--" {
		t.Fatalf("lost partial cache: %+v", stale)
	}
	fp.cardErr = nil
	fp.card = valCardTitle("神话II 185分")
	full, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if full.Stale || full.Error != "" || full.Warning != "" || full.RR != "185" || full.RRSource != "source[1].card.left_data.title" {
		t.Fatalf("not recovered: %+v", full)
	}
}

func TestUnrankedEnumAndPlacementTransition(t *testing.T) {
	svc, fp := testRefreshService(t)
	pc := mapValue(anyList(mapValue(fp.role)["list"])[1])
	pc["tier_text"], pc["tier"] = "", 255.0
	fp.card = valCardTitle("未定级 0分")
	got, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rank != "未定级" || got.RR != "0" || got.Warning != "" || got.Stale {
		t.Fatalf("unranked snapshot=%+v", got)
	}
	// An explicitly unranked account need not have applicable current RR.
	fp.card = valCardTitle("未定级")
	got, err = svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rank != "未定级" || got.RR != "--" || got.Warning != "" || got.Stale {
		t.Fatalf("inapplicable RR flagged as error: %+v", got)
	}
	// Placement completion must use fresh rank and RR, not a cached unranked flag.
	pc["tier_text"], pc["tier"] = "铂金I", 16.0
	fp.card = valCardTitle("铂金I 82分")
	got, err = svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rank != "铂金I" || got.RR != "82" || got.Warning != "" || got.Stale {
		t.Fatalf("placement transition failed: %+v", got)
	}
	fp.card = valCardTitle("铂金I")
	got, err = svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.RR != "--" || !strings.Contains(got.Warning, "[rr_parse]") {
		t.Fatalf("ranked RR missing without warning: %+v", got)
	}
}

func TestUnknownNumericRankIsDiagnosed(t *testing.T) {
	svc, fp := testRefreshService(t)
	pc := mapValue(anyList(mapValue(fp.role)["list"])[1])
	pc["tier_text"], pc["tier"] = "", 255.0
	fp.card = valCardTitle("0分")
	got, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got.Rank != "--" || !strings.Contains(got.Warning, "[rank_parse]") {
		t.Fatalf("numeric rank was guessed: %+v", got)
	}
}

func TestMalformedMatchListDoesNotOverwriteCachedRecord(t *testing.T) {
	svc, fp := testRefreshService(t)
	fp.pages[1] = map[string]any{"battle_list": []any{map[string]any{
		"game_enum": "competitive", "result_title": "胜利", "ts": float64(svc.now().Unix()),
		"competitive_tier": map[string]any{"competitive_point_change": 22},
	}}}
	before, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if before.TodayWins != 1 || before.LastRRChange != "+22" {
		t.Fatalf("bad setup: %+v", before)
	}
	for _, invalid := range []any{nil, map[string]any{}, map[string]any{"battle_list": "changed"}, map[string]any{"battle_list": []any{nil}}} {
		fp.pages[1] = invalid
		if _, err := svc.RefreshOnce(); err == nil {
			t.Fatal("unrecognised list counted as empty success")
		}
		cached := svc.State.Get()
		if !cached.Stale || !strings.HasPrefix(cached.Error, "[matches_parse]") {
			t.Fatalf("missing error: %+v", cached)
		}
		cached.Stale, cached.Error = false, ""
		if cached != before {
			t.Fatalf("cached record overwritten: %+v", cached)
		}
	}
	fp.pages[1] = map[string]any{"battle_list": []any{}}
	recovered, err := svc.RefreshOnce()
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Stale || recovered.Error != "" || recovered.TodayWins != 0 || recovered.LastRRChange != "--" {
		t.Fatalf("genuine empty list not recovered: %+v", recovered)
	}
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
