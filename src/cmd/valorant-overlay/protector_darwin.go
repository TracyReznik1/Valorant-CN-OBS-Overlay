//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	keychainService = "com.valorantcn.overlay"
	keychainAccount = "credentials-key"
	securityPath    = "/usr/bin/security"
)

// darwinProtector stores credentials as AES-GCM ciphertext. The key is kept in
// the login Keychain when available; headless/sandboxed environments fall back
// to a 0600 key file inside the data directory.
type darwinProtector struct {
	keyPath        string
	preferKeychain bool

	mu  sync.Mutex
	key []byte
}

func defaultProtector(dataDir string) Protector {
	return &darwinProtector{
		keyPath:        filepath.Join(dataDir, "credentials.key"),
		preferKeychain: true,
	}
}

func (p *darwinProtector) keyBytes() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.key) == 32 {
		return p.key, nil
	}
	// Once a local fallback key exists, keep using it so a later Keychain
	// recovery does not make previously saved credentials undecryptable.
	if data, err := os.ReadFile(p.keyPath); err == nil {
		if len(data) != 32 {
			return nil, errors.New("local key file has invalid length")
		}
		p.key = data
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if p.preferKeychain {
		if key, err := keychainKey(); err == nil && len(key) == 32 {
			p.key = key
			return key, nil
		}
	}
	key, err := localKey(p.keyPath)
	if err != nil {
		return nil, err
	}
	p.key = key
	return key, nil
}

func keychainKey() ([]byte, error) {
	out, err := exec.Command(securityPath, "find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w").Output()
	if err == nil {
		key, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
		if decErr == nil && len(key) == 32 {
			return key, nil
		}
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	cmd := exec.Command(securityPath, "add-generic-password", "-U", "-s", keychainService, "-a", keychainAccount, "-w", encoded)
	if output, addErr := cmd.CombinedOutput(); addErr != nil {
		return nil, fmt.Errorf("keychain unavailable: %v: %s", addErr, strings.TrimSpace(string(output)))
	}
	return key, nil
}

func localKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) != 32 {
			return nil, errors.New("local key file has invalid length")
		}
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func (p *darwinProtector) Protect(in []byte) ([]byte, error) {
	key, err := p.keyBytes()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, in, nil), nil
}

func (p *darwinProtector) Unprotect(in []byte) ([]byte, error) {
	key, err := p.keyBytes()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(in) < gcm.NonceSize() {
		return nil, errors.New("encrypted credential payload is too short")
	}
	nonce, ciphertext := in[:gcm.NonceSize()], in[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
