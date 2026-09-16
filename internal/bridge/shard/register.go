// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

import "projectfile.org/projectfile/bridge/internal/bridge"

func init() {
	bridge.Register(Bridge{})
}
