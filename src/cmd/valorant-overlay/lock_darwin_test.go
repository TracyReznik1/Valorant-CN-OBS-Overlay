//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"path/filepath"
	"testing"
)

func TestInstanceLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overlay.lock")
	first, ok, err := acquireLockFile(path)
	if err != nil || !ok {
		t.Fatalf("first lock ok=%t err=%v", ok, err)
	}
	second, ok, err := acquireLockFile(path)
	if err != nil || ok {
		t.Fatalf("second lock ok=%t err=%v", ok, err)
	}
	if second != nil {
		t.Fatal("second lock should be nil")
	}
	first.Release()
	third, ok, err := acquireLockFile(path)
	if err != nil || !ok {
		t.Fatalf("third lock ok=%t err=%v", ok, err)
	}
	third.Release()
}
