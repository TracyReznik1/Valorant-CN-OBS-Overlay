// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

type rankRRMatrix struct {
	Synthetic bool     `json:"synthetic"`
	Sources   []string `json:"sources"`
	Groups    []struct {
		ID     string   `json:"id"`
		Ranks  []string `json:"ranks"`
		Points []int    `json:"points"`
	} `json:"groups"`
}

type rankRRMatrixResult struct {
	CaseID       string          `json:"case_id"`
	Group        string          `json:"group"`
	Source       string          `json:"source"`
	ExpectedRank string          `json:"expected_rank"`
	ExpectedRR   string          `json:"expected_rr"`
	State        OverlaySnapshot `json:"state"`
}

type matrixProbe struct{ *Probe }

// Never allow a failed mock request to initiate a real QQ login.
func (matrixProbe) QQLogin() (Credentials, error) {
	return Credentials{}, fmt.Errorf("matrix tests must use the synthetic cached login")
}

// This is an integration matrix, not a promotion simulator. Every rank must
// remain exactly the API's label even when RR would suggest a different rank.
// All inputs are synthetic, including the high-rank response variations.
func TestRankRRAPIDisplayMatrix(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "rank_rr_matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	var matrix rankRRMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	if !matrix.Synthetic || len(matrix.Groups) != 4 || len(matrix.Sources) != 6 {
		t.Fatal("incomplete synthetic matrix")
	}

	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	var mu sync.RWMutex
	var roleData, cardData any
	requestCount := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requestCount++
		if r.Method != http.MethodPost {
			t.Error("expected POST")
			w.WriteHeader(405)
			return
		}
		var data any
		switch r.URL.Path {
		case "/go/account/get_main_role_raw":
			data = roleData
		case "/go/mine/card/val_card", "/go/agame/career/record/list":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if body["game_role_id"] != "matrix-pc" || body["scene"] != "matrix-scene" {
				t.Error("wrong selected role in API request")
				w.WriteHeader(400)
				return
			}
			if r.URL.Path == "/go/mine/card/val_card" {
				data = cardData
			} else {
				data = map[string]any{"battle_list": []any{
					map[string]any{"game_enum": "competitive", "result_title": "胜利", "ts": now.Unix(), "competitive_tier": map[string]any{"competitive_point_change": 22}},
					map[string]any{"game_enum": "competitive", "result_title": "失败", "ts": now.Add(-time.Minute).Unix(), "competitive_tier": map[string]any{"competitive_point_change": -18}},
				}}
			}
		default:
			t.Error("unexpected API path")
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"result": 0, "data": data}); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	p, err := newProbe(ProbeConfig{APIBase: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	store := CredentialStore{Path: filepath.Join(t.TempDir(), "synthetic.bin"), Protector: prefixProtector{}}
	if err := store.Save(Credentials{UserID: "matrix-test", TID: "matrix-test"}); err != nil {
		t.Fatal(err)
	}
	svc := &OverlayService{State: NewOverlayState(), Store: store, NewProbe: func() (ProbeAPI, error) { return matrixProbe{p}, nil }, Now: func() time.Time { return now }}
	stateHandler := NewOverlayHandler(svc.State)
	var results []rankRRMatrixResult
	groupCounts := map[string]int{}

	check := func(group, rank string, points int, source string) {
		id := fmt.Sprintf("%s/%s/%d/%s", group, rank, points, source)
		t.Run(id, func(t *testing.T) {
			wantRR := strconv.Itoa(points)
			pc := map[string]any{"gameId": "agame", "game_role_id": "matrix-pc", "scene": "matrix-scene", "tier_text": "", "tier": 255}
			card := valCardTitle(rank)
			left := mapValue(mapValue(card["card"])["left_data"])
			left["image_url"] = "https://example.test/rank.png"
			// A lower-priority progress value must not override explicit RR,
			// even when that explicit RR is zero or smaller than the progress.
			pc["rank_detail"] = "82/100"
			wantSource := ""
			switch source {
			case "title_points":
				left["title"] = rank + " " + wantRR + "分"
				wantSource = "source[1].card.left_data.title"
			case "title_rr":
				left["title"] = rank + " " + wantRR + " RR"
				wantSource = "source[1].card.left_data.title"
			case "role_number":
				pc["tier_text"], pc["league_point"] = rank, points
				wantSource = "source[0].league_point"
			case "role_string":
				pc["tier_text"], pc["league_point"] = rank, wantRR
				wantSource = "source[0].league_point"
			case "card_number":
				card["league_point"] = points
				wantSource = "source[1].league_point"
			case "card_string":
				card["league_point"] = wantRR
				wantSource = "source[1].league_point"
			default:
				t.Fatalf("unsupported fixture source %q", source)
			}
			mu.Lock()
			roleData = map[string]any{"list": []any{map[string]any{"gameId": "codev", "tier_text": "神话III", "rr": 8888}, pc}}
			cardData = card
			mu.Unlock()
			snap, err := svc.RefreshOnce()
			if err != nil {
				t.Fatal(err)
			}
			if snap.Rank != rank || snap.RR != wantRR || snap.RRSource != wantSource {
				t.Fatalf("API rank/RR changed or wrong source: %+v", snap)
			}
			if snap.Stale || snap.Warning != "" || snap.Error != "" {
				t.Fatalf("valid API values flagged as error: %+v", snap)
			}
			if snap.TodayWins != 1 || snap.TodayLosses != 1 || snap.LastRRChange != "+22" || snap.LastResult != "胜利" || snap.RankIcon != "https://example.test/rank.png" {
				t.Fatalf("other overlay fields regressed: %+v", snap)
			}
			recorder := httptest.NewRecorder()
			stateHandler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/state", nil))
			if recorder.Code != 200 {
				t.Fatalf("state HTTP status %d", recorder.Code)
			}
			var exposed OverlaySnapshot
			if err := json.Unmarshal(recorder.Body.Bytes(), &exposed); err != nil {
				t.Fatal(err)
			}
			if exposed != snap {
				t.Fatal("HTTP state differs from refreshed snapshot")
			}
			results = append(results, rankRRMatrixResult{id, group, source, rank, wantRR, exposed})
			groupCounts[group]++
		})
	}
	for _, group := range matrix.Groups {
		for _, rank := range group.Ranks {
			for _, points := range group.Points {
				for _, source := range matrix.Sources {
					check(group.ID, rank, points, source)
				}
			}
		}
	}
	// Explicitly disprove any "RR over 1000 means Radiant" inference.
	for _, source := range matrix.Sources {
		check("independent_rank", "神话III", 1250, source)
	}
	// The same service has moved through every combination to 9999 RR. Return
	// to unranked/0 to verify that neither rank nor RR sticks in the cache.
	check("return_to_unranked", "未定级", 0, "title_points")
	if t.Failed() {
		return
	}
	mu.RLock()
	observedRequests := requestCount
	mu.RUnlock()
	if len(results) != 1501 || observedRequests != 3*len(results) {
		t.Fatalf("incomplete matrix: %d cases, %d requests", len(results), observedRequests)
	}
	t.Logf("synthetic API -> refresh -> HTTP matrix: %d cases, %d requests, groups=%v", len(results), observedRequests, groupCounts)
	if path := os.Getenv("VALORANT_OVERLAY_MATRIX_RESULTS"); path != "" {
		report := struct {
			Synthetic bool                 `json:"synthetic"`
			Count     int                  `json:"count"`
			Cases     []rankRRMatrixResult `json:"cases"`
		}{true, len(results), results}
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
