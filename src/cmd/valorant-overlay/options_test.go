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
