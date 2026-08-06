// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"embed"

	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

//go:embed all:templates
var templatesFS embed.FS

func init() {
	core.RegisterTemplateTree(templatesFS, "templates")
	bridge.Register(Bridge{})
}
