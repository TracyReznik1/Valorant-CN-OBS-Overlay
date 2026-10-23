// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type prefixProtector struct{}

func (prefixProtector) Protect(b []byte) ([]byte, error) {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ 0xA5
	}
	return out, nil
}
func (prefixProtector) Unprotect(b []byte) ([]byte, error) {
	out := make([]byte, len(b))
	for i := range b {
		out[i] = b[i] ^ 0xA5
	}
	return out, nil
}

func TestCredentialStoreRoundTripIsProtected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.bin")
	s := CredentialStore{Path: path, Protector: prefixProtector{}}
	want := Credentials{UserID: "uid", TID: "tid", OpenID: "secret-openid", UIN: 7, AccessToken: "secret-token"}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "secret-token") {
		t.Fatal("credential file contains plaintext token")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".credentials-") {
			t.Fatalf("temporary credential file left behind: %s", entry.Name())
		}
	}
	got, ok, err := s.Load()
	if err != nil || !ok {
		t.Fatalf("load ok=%v err=%v", ok, err)
	}
	if got.AccessToken != want.AccessToken || got.OpenID != want.OpenID {
		t.Fatalf("got=%#v", got)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}

func TestOverlayHTTPExposesOnlyPublicState(t *testing.T) {
	state := NewOverlayState()
	state.Update(OverlaySnapshot{Rank: "铂金I", RR: "82", RRSource: "source[1].card.left_data.title", TodayWins: 2, TodayLosses: 1, LastRRChange: "-18"})
	srv := httptest.NewServer(NewOverlayHandler(state))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["rank"] != "铂金I" || got["rr"] != "82" || got["rr_source"] != "source[1].card.left_data.title" {
		t.Fatalf("state=%v", got)
	}
	for _, secret := range []string{"access_token", "openid", "tid", "cookie"} {
		if strings.Contains(strings.ToLower(string(b)), secret) {
			t.Fatalf("public API leaked %s: %s", secret, b)
		}
	}
}

func TestOverlayPageIsEmbedded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/overlay", nil)
	w := httptest.NewRecorder()
	NewOverlayHandler(NewOverlayState()).ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "无畏契约") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing security headers: %v", w.Header())
	}
}

func TestStatusPageAndReloginEndpoint(t *testing.T) {
	state := NewOverlayState()
	calls := 0
	handler := NewOverlayHandler(state, func() bool { calls++; return true })

	statusReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), "重新登录") {
		t.Fatalf("status code=%d body=%q", statusRec.Code, statusRec.Body.String())
	}

	reloginReq := httptest.NewRequest(http.MethodPost, "/api/relogin", nil)
	reloginRec := httptest.NewRecorder()
	handler.ServeHTTP(reloginRec, reloginReq)
	if reloginRec.Code != http.StatusOK || calls != 1 {
		t.Fatalf("relogin code=%d calls=%d body=%q", reloginRec.Code, calls, reloginRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/relogin", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET relogin code=%d", getRec.Code)
	}

	busyHandler := NewOverlayHandler(state, func() bool { return false })
	busyRec := httptest.NewRecorder()
	busyHandler.ServeHTTP(busyRec, httptest.NewRequest(http.MethodPost, "/api/relogin", nil))
	if busyRec.Code != http.StatusConflict {
		t.Fatalf("busy relogin code=%d", busyRec.Code)
	}

	noCallback := NewOverlayHandler(state)
	noCallbackRec := httptest.NewRecorder()
	noCallback.ServeHTTP(noCallbackRec, httptest.NewRequest(http.MethodPost, "/api/relogin", nil))
	if noCallbackRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no-callback relogin code=%d", noCallbackRec.Code)
	}
}

func TestOverlayHandlerUnknownPathIs404(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	w := httptest.NewRecorder()
	NewOverlayHandler(NewOverlayState()).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d", w.Code)
	}
}
