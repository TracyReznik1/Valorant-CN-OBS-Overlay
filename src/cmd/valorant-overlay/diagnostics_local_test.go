//go:build windows && localdiagnostics

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in, read-only diagnostic. Credentials are decrypted only in memory;
// neither credentials nor raw responses/arbitrary fields/URLs are logged or saved.
func TestLocalRankDiagnostic(t *testing.T) {
	path := os.Getenv("VALORANT_OVERLAY_DIAGNOSTICS_STORE")
	if path == "" {
		t.Skip("set the local encrypted credential-store path to opt in")
	}
	store := CredentialStore{Path: path, Protector: defaultProtector(filepath.Dir(path))}
	c, ok, err := store.Load()
	if err != nil || !ok {
		t.Fatal("cannot load local encrypted login")
	}
	p, err := newProbe(ProbeConfig{})
	if err != nil {
		t.Fatal("cannot initialise probe")
	}
	raw, err := p.FetchMainRole(c)
	if err != nil {
		t.Fatal("main-role request failed")
	}
	role := parseRoleSnapshot(raw)
	for _, key := range []string{"tier_text", "tier", "competitive_tier", "league_point"} {
		t.Logf("role.%s type=%T integer=%q", key, role.Raw[key], rrInteger(role.Raw[key], false))
	}
	t.Logf("role.tier_text empty=%t", strings.TrimSpace(stringValue(role.Raw["tier_text"])) == "")
	card, err := p.FetchValCard(c, role)
	if err != nil {
		t.Fatal("card request failed")
	}
	left := mapValue(mapValue(mapValue(card)["card"])["left_data"])
	title, _ := left["title"].(string)
	rr := extractCurrentRR(card)
	t.Logf("card.title type=%T rr=%q starts_with_255=%t", left["title"], rr, strings.HasPrefix(strings.TrimSpace(title), "255"))
	for _, name := range []string{"未定级", "未评级", "无段位", "黑铁", "青铜", "白银", "黄金", "铂金", "钻石", "超凡", "神话", "辐能战魂", "无畏战魂"} {
		if strings.Contains(title, name) {
			t.Logf("card.title contains rank keyword=%q", name)
		}
	}
	name := extractCurrentRank(role, card)
	t.Logf("resolved rank=%q", name)
	if expected := os.Getenv("VALORANT_OVERLAY_DIAGNOSTICS_EXPECT_RANK"); expected != "" {
		if validRankName(expected) == "" {
			t.Fatal("expected rank must be a known display name")
		}
		if name != expected {
			t.Fatal("resolved rank does not match expected display name")
		}
	}
}
