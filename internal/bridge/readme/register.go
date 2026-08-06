// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"embed"

	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

//go:embed all:templates
var templatesFS embed.FS

func init() {
	core.RegisterTemplates("README.md.tmpl", func(n string) ([]byte, error) {
		return templatesFS.ReadFile("templates/" + n)
	})
	bridge.Register(Bridge{})
}
