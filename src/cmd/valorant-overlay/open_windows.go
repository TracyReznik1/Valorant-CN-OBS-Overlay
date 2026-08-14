//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import "os/exec"

func openPath(target string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}
