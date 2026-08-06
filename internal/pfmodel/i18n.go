// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// I18NExtension is the document-level localization declaration: the set of
// languages every localizable renderer (README, CONTRIBUTING, CODE_OF_CONDUCT,
// SECURITY, SUPPORT) emits a variant for, on top of the canonical
// default-language file.
//
// One list for the whole document rather than one per namespace: a project
// that speaks Spanish speaks it in every community health file, and a single
// list means adding a locale is a one-line edit.
type I18NExtension struct {
	// Languages are BCP 47 tags (spec §7) — "es", "uk", "pt-BR". The
	// canonical file (README.md, CONTRIBUTING.md, …) is always rendered and
	// is NOT named here; these are the extra variants.
	Languages []string
}

// GetI18NExtension parses `org.projectfile.i18n`. Returns (nil, nil) when the
// namespace is absent — the document is single-language and every renderer
// emits only its canonical file.
func GetI18NExtension(doc *projectfile.Document) (*I18NExtension, error) {
	m, present, err := lookupNS(doc, I18NExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &I18NExtension{Languages: strListVal(m, "languages")}, nil
}

// Languages resolves the document's extra-language list, normalized: empty
// entries dropped, duplicates collapsed, declaration order preserved. This is
// the single source every localizable bridge reads — a malformed namespace
// degrades to "no extra languages" with a warning rather than failing the
// render, because a broken locale list must not block the canonical file.
func Languages(doc *projectfile.Document) []string {
	ext, err := GetI18NExtension(doc)
	if err != nil {
		genlog.Warn("ignoring malformed localization namespace",
			"namespace", I18NExtensionNS, "error", err.Error())
		return nil
	}
	if ext == nil {
		return nil
	}
	seen := make(map[string]bool, len(ext.Languages))
	out := make([]string, 0, len(ext.Languages))
	for _, lang := range ext.Languages {
		if lang == "" || seen[lang] {
			genlog.Decision("language", lang, I18NExtensionNS+".languages", "dropped (empty or duplicate)")
			continue
		}
		seen[lang] = true
		out = append(out, lang)
	}
	return out
}
