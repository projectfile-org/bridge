// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// GetForgeExtension parses `org.projectfile.forge`. All toggles default to
// true: when the extension is absent every field is pushed to every host;
// when present-but-incomplete, omitted toggles still default to "on" so a
// projectfile that mentions the extension only to disable one host doesn't
// accidentally turn the others off. Returns (nil, nil) when the namespace is
// absent — callers treat nil as "every default applies".
func GetForgeExtension(doc *projectfile.Document) (*ForgeExtension, error) {
	m, present, err := lookupNS(doc, ForgeExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &ForgeExtension{
		// Top-level push toggle defaults to true so a [org.projectfile.forge]
		// block that only carries field-level overrides still allows the
		// push to run at all.
		Push: boolFieldDefaultTrue(m, "push"),
		Fields: ForgeFieldsToggles{
			Description: boolValDefaultTrue(m, "fields", "description"),
			Homepage:    boolValDefaultTrue(m, "fields", "homepage"),
			Topics:      boolValDefaultTrue(m, "fields", "topics"),
		},
	}
	if hostsRaw, ok := m["hosts"].(map[string]any); ok && len(hostsRaw) > 0 {
		ext.Hosts = map[string]bool{}
		for host, v := range hostsRaw {
			b, ok := v.(bool)
			if !ok {
				// Non-bool value here is a user typo (e.g. "true" as a string).
				// Treating it as the safe default (push allowed) avoids
				// silently suppressing a push the user expected to happen.
				ext.Hosts[host] = true
				continue
			}
			ext.Hosts[host] = b
		}
	}
	if kindsRaw, ok := m["kinds"].(map[string]any); ok && len(kindsRaw) > 0 {
		ext.Kinds = map[string]string{}
		for host, v := range kindsRaw {
			// Non-string value is a typo — skip silently rather than coerce.
			// A bad kind ("forfejo") will fail later at driver-lookup with a
			// clear "no driver registered for kind X" warning.
			s, _ := v.(string)
			if s == "" {
				continue
			}
			ext.Kinds[host] = s
		}
	}
	return ext, nil
}

// boolFieldDefaultTrue is the single-level analogue of boolValDefaultTrue
// for a top-level toggle. Used by GetForgeExtension for the namespace-wide
// `push` switch which has no nesting layer.
func boolFieldDefaultTrue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	if !ok {
		return true
	}
	return b
}
