// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if !ok || pe.Stage != "matches" || !strings.Contains(pe.Error(), "登录过期") {
		t.Fatalf("err=%T %#v", err, err)
	}
}
