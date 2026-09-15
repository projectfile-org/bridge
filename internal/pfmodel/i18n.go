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
// SECURITY, SUPPORT) emits a variant for, plus the project's primary language
// (default-language), which the canonical root-level file is written in.
//
// One list for the whole document rather than one per namespace: a project
// that speaks Spanish speaks it in every community health file, and a single
// list means adding a locale is a one-line edit.
type I18NExtension struct {
	// DefaultLanguage is the project's primary BCP 47 tag — the language the
	// canonical root-level files are written in. Empty means the spec default
	// "en". Resolved through DefaultLanguage() so callers never see the raw
	// empty string.
	DefaultLanguage string
	// Languages are BCP 47 tags (spec §7) — "es", "uk", "pt-BR". The OTHER
	// languages (not including DefaultLanguage) the project publishes. The
	// canonical file (README.md, CONTRIBUTING.md, …) is always rendered in the
	// default language and is NOT named here; these are the extra variants,
	// each placed under docs/<lang>/.
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
	return &I18NExtension{
		DefaultLanguage: strVal(m, "default-language"),
		Languages:       strListVal(m, "languages"),
	}, nil
}

// defaultLanguageSpec is the spec default for org.projectfile.i18n.default-
// language when the field (or namespace) is absent. The canonical root-level
// files are written in this language.
const defaultLanguageSpec = "en"

// DefaultLanguage resolves the project's primary language — the one the
// canonical root-level files are written in. Returns defaultLanguageSpec
// ("en") when the field or namespace is absent, so callers always get a
// concrete tag and never have to special-case the empty string.
//
// A malformed namespace degrades to the spec default with a warning, because a
// broken i18n block must not block the canonical file.
func DefaultLanguage(doc *projectfile.Document) string {
	ext, err := GetI18NExtension(doc)
	if err != nil {
		genlog.Warn("ignoring malformed localization namespace",
			"namespace", I18NExtensionNS, "error", err.Error())
		return defaultLanguageSpec
	}
	if ext == nil || ext.DefaultLanguage == "" {
		return defaultLanguageSpec
	}
	return ext.DefaultLanguage
}

// Languages resolves the document's extra-language list, normalized: empty
// entries dropped, duplicates collapsed, the default language excluded (it is
// the root file, not a variant), declaration order preserved. This is the
// single source every localizable bridge reads — a malformed namespace
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
	def := DefaultLanguage(doc)
	seen := make(map[string]bool, len(ext.Languages))
	out := make([]string, 0, len(ext.Languages))
	for _, lang := range ext.Languages {
		reason := ""
		switch {
		case lang == "" || seen[lang]:
			reason = "dropped (empty or duplicate)"
		case lang == def:
			reason = "dropped (is default-language; rendered at root)"
		}
		if reason != "" {
			genlog.DebugRow("language", lang, I18NExtensionNS+".languages", reason)
			continue
		}
		seen[lang] = true
		out = append(out, lang)
	}
	return out
}
