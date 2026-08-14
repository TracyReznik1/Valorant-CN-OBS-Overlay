//go:build windows

package main

import (
	"bytes"
	"testing"
)

func TestDPAPIProtectorRoundTrip(t *testing.T) {
	p := dpapiProtector{}
	plain := []byte("valorant-overlay-secret-test")
	encrypted, err := p.Protect(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, plain) {
		t.Fatal("DPAPI returned plaintext")
	}
	decrypted, err := p.Unprotect(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("roundtrip mismatch: %q", decrypted)
	}
}
