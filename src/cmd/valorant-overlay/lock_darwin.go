//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type instanceLock struct{ file *os.File }

func acquireInstanceLock() (*instanceLock, bool, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("valorant-cn-overlay-%d.lock", os.Getuid()))
	return acquireLockFile(path)
}

func acquireLockFile(path string) (*instanceLock, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &instanceLock{file: file}, true, nil
}

func (l *instanceLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}
