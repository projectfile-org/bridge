// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package gitlab

import (
	"projectfile.org/projectfile/bridge/v4/internal/forge/core"
	"projectfile.org/projectfile/bridge/v4/internal/forge/hostmatch"
)

func init() {
	core.Register(string(hostmatch.KindGitLab), New)
}
