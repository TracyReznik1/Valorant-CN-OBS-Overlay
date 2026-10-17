// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBuildAuthCookieRealToken(t *testing.T) {
	creds := Credentials{UserID: "u1", TID: "t1", OpenID: "o1", UIN: 123, AccessToken: "a1"}
	got := buildAuthCookie(creds, true)
	for _, want := range []string{"clientType=9", "uin=o123", "openid=o1", "access_token=a1", "userId=u1", "tid=t1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("cookie %q missing %q", got, want)
		}
	}
}

func TestAPIPostReturnsData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if !strings.Contains(r.Header.Get("Cookie"), "access_token=a1") {
			t.Fatalf("cookie=%q", r.Header.Get("Cookie"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": 0, "data": map[string]any{"ok": true}})
	}))
	defer srv.Close()
	p, err := newProbe(ProbeConfig{APIBase: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.apiPost("/x", Credentials{UserID: "u1", TID: "t1", OpenID: "o1", UIN: 1, AccessToken: "a1"}, map[string]any{"x": 1}, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	m := mapValue(got)
	if m == nil || m["ok"] != true {
		t.Fatalf("data=%#v", got)
	}
}

func TestAPIPostReturnsStageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": 1001, "msg": "登录过期"})
	}))
	defer srv.Close()
	p, _ := newProbe(ProbeConfig{APIBase: srv.URL})
	_, err := p.apiPost("/x", Credentials{}, map[string]any{}, "matches", false)
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ProbeError)
	if !ok || pe.Stage != "matches" || pe.Error() != "掌瓦接口错误码 +1001" {
		t.Fatalf("err=%T %#v", err, err)
	}
}

func TestAPIPostRejectsBrokenEnvelopesWithoutLeakingMessages(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"result":"broken","data":{}}`,
		`{"result":0.5,"data":{}}`, `{"result":null,"data":{}}`,
		`{"result":0}`, `{"result":0,"data":null}`, `{"result":0,"data":"unexpected"}`,
		`{"result":1001,"msg":"sensitive-marker","errMsg":"sensitive-marker"}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		p, err := newProbe(ProbeConfig{APIBase: srv.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.FetchValCard(Credentials{}, RoleSnapshot{GameRoleID: "pc", Scene: "test-scene"})
		srv.Close()
		pe, ok := err.(*ProbeError)
		if !ok || pe.Stage != "val_card" {
			t.Fatalf("expected stage error for envelope, got %v", err)
		}
		if strings.Contains(err.Error(), "sensitive-marker") {
			t.Fatal("server message leaked")
		}
	}
}

func TestAPIHTTPFailureDoesNotExposeBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("sensitive-marker"))
	}))
	defer srv.Close()
	p, _ := newProbe(ProbeConfig{APIBase: srv.URL})
	_, err := p.FetchMainRole(Credentials{})
	if err == nil || strings.Contains(err.Error(), "sensitive-marker") || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("unsafe or missing status error: %v", err)
	}
}

func TestTransportFailureDoesNotExposeURLOrMessage(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.test/?access_token=sensitive-marker", Err: fmt.Errorf("sensitive-marker")}
	if got := safeRequestError(err); strings.Contains(got, "sensitive-marker") || strings.Contains(got, "https:") {
		t.Fatal("transport error leaked sensitive data")
	}
}

// Synthetic endpoint envelopes, not live player captures. Paths/keys are from
// the existing client; high-rank values test decoding and parser composition.
func TestCurrentRRFromAPIDecodedData(t *testing.T) {
	for _, tc := range []struct {
		point string
		want  string
	}{
		{"82", "82"}, {`"185"`, "185"}, {"350", "350"}, {`"1250"`, "1250"},
		{"null", "1250"}, {`"invalid"`, "1250"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/go/account/get_main_role_raw":
				_, _ = fmt.Fprintf(w, `{"result":0,"data":{"list":[{"gameId":"codev","rr":99},{"gameId":"agame","game_role_id":"pc","scene":"test-scene","league_point":%s}]}}`, tc.point)
			case "/go/mine/card/val_card":
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request["game_role_id"] != "pc" || request["scene"] != "test-scene" {
					t.Error("wrong role in card request")
				}
				_, _ = w.Write([]byte(`{"result":"0","data":{"card":{"left_data":{"title":"辐能战魂 1250分"}}}}`))
			default:
				t.Error("unexpected endpoint")
				w.WriteHeader(404)
			}
		}))
		p, _ := newProbe(ProbeConfig{APIBase: srv.URL})
		roleRaw, err := p.FetchMainRole(Credentials{})
		if err != nil {
			t.Fatal(err)
		}
		card, err := p.FetchValCard(Credentials{}, parseRoleSnapshot(roleRaw))
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got := extractCurrentRR(roleRaw, card); got != tc.want {
			t.Fatalf("RR=%q, want %q", got, tc.want)
		}
	}
}
