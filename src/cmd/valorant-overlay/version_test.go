// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesVersionFile(t *testing.T) {
	raw, err := os.ReadFile("../../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != version {
		t.Fatalf("VERSION=%q but main.version=%q; update both or build with -X main.version", got, version)
	}
}
