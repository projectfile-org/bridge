// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"embed"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// summaryTemplate renders the summary line; a project overrides it under .projectfile/templates/.
const summaryTemplate = "release-notes-summary.md.tmpl"

//go:embed templates
var templatesFS embed.FS

func init() {
	core.RegisterTemplateTree(templatesFS, "templates")
}
