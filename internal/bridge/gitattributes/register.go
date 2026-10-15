// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package gitattributes

import "projectfile.org/projectfile/bridge/v4/internal/bridge"

func init() {
	bridge.Register(Bridge{})
}
