//go:build !windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

func defaultProtector() Protector { return nil }
