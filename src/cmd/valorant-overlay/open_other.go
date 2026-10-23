//go:build !windows && !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

func openPath(string) error { return nil }
