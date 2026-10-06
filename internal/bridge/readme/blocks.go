// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Blocks renders the named README blocks in the default language, without the managed header.
func Blocks(pf *projectfile.Document, dir string, names ...string) (string, error) {
	ext, _ := pfmodel.GetReadmeExtension(pf)
	view := newReadmeView(pf, "", pfmodel.Languages(pf))
	var parts []string
	for _, name := range names {
		body, err := renderBlock(dir, name, view, ext, "", templatesFS)
		if err != nil {
			return "", err
		}
		if body != nil {
			parts = append(parts, strings.TrimSpace(string(body)))
		}
	}
	return strings.Join(parts, "\n\n"), nil
}
