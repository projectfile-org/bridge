// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import "projectfile.org/projectfile/bridge/internal/scanners/core"

func init() {
	core.Register(authorsScanner{})
	core.Register(remotesScanner{})
	core.Register(datesScanner{})
}
