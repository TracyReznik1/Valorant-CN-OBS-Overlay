//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

type instanceLock struct{ handle syscall.Handle }

// acquireInstanceLock uses a named kernel mutex so a second double-click does
// not run another full refresh/login before failing on the HTTP port.
func acquireInstanceLock() (*instanceLock, bool, error) {
	return acquireNamedLock(`Local\ValorantCNOverlay`)
}

func acquireNamedLock(name string) (*instanceLock, bool, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}
	r, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	if r == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return nil, false, callErr
		}
		return nil, false, errors.New("CreateMutexW failed")
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		procCloseHandle.Call(r)
		if callErr == syscall.ERROR_ALREADY_EXISTS {
			return nil, false, nil
		}
		return nil, false, callErr
	}
	return &instanceLock{handle: syscall.Handle(r)}, true, nil
}

func (l *instanceLock) Release() {
	if l == nil || l.handle == 0 {
		return
	}
	procCloseHandle.Call(uintptr(l.handle))
	l.handle = 0
}
