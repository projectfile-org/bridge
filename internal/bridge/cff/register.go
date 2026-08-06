// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import "projectfile.org/projectfile/bridge/internal/bridge"

// init self-registers the CFF bridge. cmd/bridge.go blank-imports this
// package so the registration runs at process start.
func init() {
	bridge.Register(Bridge{})
}
