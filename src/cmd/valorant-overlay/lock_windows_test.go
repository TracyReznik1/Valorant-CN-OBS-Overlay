//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"fmt"
	"testing"
	"time"
)

func TestInstanceLockIsExclusive(t *testing.T) {
	name := fmt.Sprintf("Local\\ValorantCNOverlay-test-%d", time.Now().UnixNano())
	first, ok, err := acquireNamedLock(name)
	if err != nil || !ok {
		t.Fatalf("first lock ok=%t err=%v", ok, err)
	}
	second, ok, err := acquireNamedLock(name)
	if err != nil || ok {
		t.Fatalf("second lock ok=%t err=%v", ok, err)
	}
	if second != nil {
		t.Fatal("second lock should be nil")
	}
	first.Release()
	third, ok, err := acquireNamedLock(name)
	if err != nil || !ok {
		t.Fatalf("third lock ok=%t err=%v", ok, err)
	}
	third.Release()
}
