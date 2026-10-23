//go:build !windows && !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

func defaultProtector(string) Protector { return nil }
