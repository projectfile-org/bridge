// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// GetCodeOwnersExtension parses `org.projectfile.codeowners`. The entries
// array is order-significant — CODEOWNERS pattern resolution is "last match
// wins per path", so reordering changes semantics.
func GetCodeOwnersExtension(doc *projectfile.Document) (*CodeOwnersExtension, error) {
	m, present, err := lookupNS(doc, CodeOwnersExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	entries, _ := m["entries"].([]any)
	ext := &CodeOwnersExtension{}
	for _, e := range entries {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		ext.Entries = append(ext.Entries, CodeOwnersEntry{
			Pattern: strVal(em, "pattern"),
			Owners:  strListVal(em, "owners"),
		})
	}
	return ext, nil
}
