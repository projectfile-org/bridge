// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// GetCitationExtension parses `org.projectfile.citation`. Returns (nil, nil)
// when the namespace is absent. Reads the flat key directly — matches the
// pre-core-2.0 behaviour (this namespace has no TOML dotted-header legacy).
func GetCitationExtension(doc *projectfile.Document) (*CitationExtension, error) {
	if doc == nil || doc.Extensions == nil {
		return nil, nil
	}
	raw, ok := doc.Extensions["org.projectfile.citation"]
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("org.projectfile.citation is not a map")
	}

	ext := &CitationExtension{
		DOI:     strVal(m, "doi"),
		Message: strVal(m, "message"),
	}
	if pref, ok := m["preferred"].(map[string]any); ok {
		ext.Preferred = &PreferredCitation{
			Type:    strVal(pref, "type"),
			Title:   strVal(pref, "title"),
			Journal: strVal(pref, "journal"),
			Volume:  intVal(pref, "volume"),
			Issue:   intVal(pref, "issue"),
			Pages:   strVal(pref, "pages"),
			Year:    intVal(pref, "year"),
			DOI:     strVal(pref, "doi"),
		}
	}
	return ext, nil
}
