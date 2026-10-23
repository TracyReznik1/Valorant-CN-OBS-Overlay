//go:build !windows && !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

type instanceLock struct{}

func acquireInstanceLock() (*instanceLock, bool, error) {
	return acquireNamedLock("valorant-cn-obs-overlay")
}
func acquireNamedLock(string) (*instanceLock, bool, error) { return &instanceLock{}, true, nil }
func (l *instanceLock) Release()                           {}
