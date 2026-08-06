// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package github

import (
	"projectfile.org/projectfile/bridge/internal/forge/core"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
)

// init self-registers the github driver with the forge/core registry.
// Blank-imported from cmd/forge.go.
func init() {
	core.Register(string(hostmatch.KindGitHub), New)
}
