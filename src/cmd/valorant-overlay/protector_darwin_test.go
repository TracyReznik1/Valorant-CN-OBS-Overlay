//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDarwinProtectorRoundTrip(t *testing.T) {
	p := &darwinProtector{keyPath: filepath.Join(t.TempDir(), "credentials.key")}
	plain := []byte("valorant-overlay-secret-test")
	encrypted, err := p.Protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, plain) {
		t.Fatal("protector returned plaintext")
	}
	decrypted, err := p.Unprotect(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("roundtrip mismatch: %q", decrypted)
	}

	info, err := os.Stat(p.keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("key file permissions are too broad: %v", info.Mode().Perm())
	}

	second := &darwinProtector{keyPath: p.keyPath}
	decrypted2, err := second.Unprotect(encrypted)
	if err != nil || !bytes.Equal(decrypted2, plain) {
		t.Fatalf("second protector failed: %v", err)
	}

	tampered := append([]byte(nil), encrypted...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := p.Unprotect(tampered); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}
