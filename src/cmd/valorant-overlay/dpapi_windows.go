//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}
type dpapiProtector struct{}

var crypt32 = syscall.NewLazyDLL("crypt32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var procProtect = crypt32.NewProc("CryptProtectData")
var procUnprotect = crypt32.NewProc("CryptUnprotectData")
var procLocalFree = kernel32.NewProc("LocalFree")

func bytesBlob(b []byte) (dataBlob, *byte) {
	if len(b) == 0 {
		return dataBlob{}, nil
	}
	return dataBlob{uint32(len(b)), &b[0]}, &b[0]
}
func blobBytes(b dataBlob) []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	src := unsafe.Slice(b.pbData, int(b.cbData))
	out := make([]byte, len(src))
	copy(out, src)
	return out
}
func (dpapiProtector) Protect(in []byte) ([]byte, error) {
	ib, keep := bytesBlob(in)
	_ = keep
	var out dataBlob
	r, _, e := procProtect.Call(uintptr(unsafe.Pointer(&ib)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(in)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData: %v", e)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobBytes(out), nil
}
func (dpapiProtector) Unprotect(in []byte) ([]byte, error) {
	ib, keep := bytesBlob(in)
	_ = keep
	var out dataBlob
	r, _, e := procUnprotect.Call(uintptr(unsafe.Pointer(&ib)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(in)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %v", e)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobBytes(out), nil
}
func defaultProtector() Protector { return dpapiProtector{} }
