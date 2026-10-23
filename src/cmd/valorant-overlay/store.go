// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}

type CredentialStore struct {
	Path      string
	Protector Protector
}

func (s CredentialStore) Save(c Credentials) error {
	if s.Protector == nil {
		return errors.New("credential protection is unavailable")
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	enc, err := s.Protector.Protect(raw)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Write to a sibling temp file and rename, so a crash mid-write cannot
	// leave a truncated credential file that forces a new QR login.
	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(enc); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.Path)
}
func (s CredentialStore) Load() (Credentials, bool, error) {
	if s.Protector == nil {
		return Credentials{}, false, nil
	}
	b, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return Credentials{}, false, nil
	}
	if err != nil {
		return Credentials{}, false, err
	}
	raw, err := s.Protector.Unprotect(b)
	if err != nil {
		return Credentials{}, false, err
	}
	var c Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return Credentials{}, false, err
	}
	return c, true, nil
}
func (s CredentialStore) Delete() error {
	err := os.Remove(s.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
