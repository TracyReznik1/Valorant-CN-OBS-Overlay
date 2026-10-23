// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "testing"

func TestParseOptionsDefaults(t *testing.T) {
	o, err := parseOptions([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if o.Port != 8787 || o.RefreshSeconds != 30 || o.NoOpen || o.ResetLogin {
		t.Fatalf("options=%#v", o)
	}
}
func TestParseOptionsRejectsInvalidPort(t *testing.T) {
	if _, err := parseOptions([]string{"--port", "70000"}); err == nil {
		t.Fatal("expected invalid port")
	}
}
func TestParseOptionsResetLogin(t *testing.T) {
	o, err := parseOptions([]string{"--reset-login", "--no-open", "--refresh", "60"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.ResetLogin || !o.NoOpen || o.RefreshSeconds != 60 {
		t.Fatalf("options=%#v", o)
	}
}
func TestParseOptionsDiagnose(t *testing.T) {
	o, err := parseOptions([]string{"--diagnose"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Diagnose {
		t.Fatalf("options=%#v", o)
	}
}
func TestParseOptionsDataDir(t *testing.T) {
	o, err := parseOptions([]string{"--data-dir", "./profile"})
	if err != nil {
		t.Fatal(err)
	}
	if o.DataDir != "./profile" {
		t.Fatalf("options=%#v", o)
	}
}
func TestParseOptionsRejectsHugeRefresh(t *testing.T) {
	if _, err := parseOptions([]string{"--refresh", "3601"}); err == nil {
		t.Fatal("expected refresh upper bound")
	}
}
func TestResolveDataDirExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveDataDir(dir, "/app")
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got %q want %q", got, dir)
	}
}
func TestEnsureWritableDir(t *testing.T) {
	dir := t.TempDir() + "/nested"
	if err := ensureWritableDir(dir); err != nil {
		t.Fatal(err)
	}
}
