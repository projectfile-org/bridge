// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Localized "{subject} on {forge}" labels for derived links.
//
// scan + derive compose a link label from a subject (the project title for
// source-code links, a type noun for Issues/Packages) and a forge display name
// (a proper noun, never translated). When a project declares
// org.projectfile.i18n.languages the label is emitted as a Langs map so each
// locale gets its own connector word; a single-language project stays Bare so
// nothing churns.
//
// The connector and the type nouns live HERE rather than in the readme message
// catalog: the catalog's link.type.* values are the full standalone type names
// ("Issue tracker", "Package Registry"), which are not the short nouns a
// "{noun} on {forge}" label composes from.

// Noun keys for the three subjects scan/derive label. Exported so the
// producers spell them once; the link type "source-code" doubles as its noun
// key (its placeholder subject before the title rewrite promotes it).
const (
	NounSourceCode = "source-code"
	NounIssues     = "issues"
	NounPackages   = "packages"
)

// onConnector translates the "on" that joins subject and forge, per locale.
// An unknown tag falls back to English (onWord) so an undeclared locale still
// composes a readable label rather than empty.
var onConnector = map[string]string{
	"en": "on",
	"es": "en",
	"uk": "на",
}

// linkNouns translates the "{noun} on {forge}" subject per locale. The noun is
// the short word the label leads with — "Issues", not "Issue tracker" — so it
// is independent of the readme catalog's link.type.* values. A missing entry
// degrades to English then to the bare key (nounWord).
var linkNouns = map[string]map[string]string{
	NounSourceCode: {"en": "Source Code", "es": "Código fuente", "uk": "Вихідний код"},
	NounIssues:     {"en": "Issues", "es": "Incidencias", "uk": "Issues"},
	NounPackages:   {"en": "Packages", "es": "Paquetes", "uk": "Пакунки"},
}

func onWord(lang string) string {
	if w, ok := onConnector[lang]; ok {
		return w
	}
	return onConnector["en"]
}

// nounWord resolves a noun key for a language, falling back to English then to
// the key itself so an unknown combination never yields empty.
func nounWord(nounKey, lang string) string {
	if nouns, ok := linkNouns[nounKey]; ok {
		if w, ok := nouns[lang]; ok && w != "" {
			return w
		}
		if w, ok := nouns["en"]; ok && w != "" {
			return w
		}
	}
	return nounKey
}

// labelLangs is the full set a localized label covers: the default language
// (the canonical root render) plus every declared extra language. Order is
// default-first so a single-element slice signals "single language → Bare".
func labelLangs(pf *projectfile.Document) []string {
	def := DefaultLanguage(pf)
	extras := Languages(pf)
	out := make([]string, 0, 1+len(extras))
	out = append(out, def)
	out = append(out, extras...)
	return out
}

// NounLabel builds the localized subject for a derive noun, one entry per
// declared language. Single-language projects get a Bare string; multi-language
// projects get a Langs map. Returns nil for an unknown noun key with no table
// entry only when both the key and English fallback are empty — otherwise the
// key itself is used so a caller never gets a nil subject to compose with.
func NounLabel(pf *projectfile.Document, nounKey string) *projectfile.LocalizedString {
	langs := labelLangs(pf)
	if len(langs) == 1 {
		return &projectfile.LocalizedString{Bare: nounWord(nounKey, langs[0])}
	}
	m := make(map[string]string, len(langs))
	for _, l := range langs {
		m[l] = nounWord(nounKey, l)
	}
	return &projectfile.LocalizedString{Langs: m}
}

// ComposeOnLabel composes "{subject} {connector} {forge}" per declared
// language. subject is a pre-localized string — the project title for
// source-code links, or NounLabel output for type nouns; a Bare subject (an
// untranslated title) is reused verbatim for every language, since the
// connector is the only part that varies. forge is a proper noun and is never
// translated. Single-language projects get Bare; multi-language get a Langs
// map with Bare cleared (ExtractLocalizedStringForLang returns Bare first, so a
// set Bare would mask the per-language entries — see core helpers.go).
func ComposeOnLabel(pf *projectfile.Document, subject *projectfile.LocalizedString, forge string) *projectfile.LocalizedString {
	if forge == "" {
		return nil
	}
	langs := labelLangs(pf)
	compose := func(l string) string {
		subj := projectfile.ExtractLocalizedStringForLang(subject, l)
		return strings.TrimSpace(subj + " " + onWord(l) + " " + forge)
	}
	if len(langs) == 1 {
		return &projectfile.LocalizedString{Bare: compose(langs[0])}
	}
	m := make(map[string]string, len(langs))
	for _, l := range langs {
		m[l] = compose(l)
	}
	return &projectfile.LocalizedString{Langs: m}
}

// PromoteSourceCodeLabel replaces a scanner placeholder label (the "Source
// Code on {forge}" noun form, in any locale) with one whose subject is title,
// recomposed per declared language. It is the localized successor to the scan
// title-rewrite: the scanner writes the noun placeholder (it sees only the
// base doc), and the rewrite promotes it to the effective title (resolved with
// includes) once the merged doc is available.
//
// Returns the promoted label when the input is still the placeholder, nil
// otherwise — so a curated or already-promoted label is left untouched. The
// detection compares the DEFAULT-language value against the noun prefix, which
// matches both Bare placeholders and Langs placeholders in one check.
func PromoteSourceCodeLabel(pf *projectfile.Document, label, title *projectfile.LocalizedString) *projectfile.LocalizedString {
	if label == nil || title == nil {
		return nil
	}
	def := DefaultLanguage(pf)
	cur := projectfile.ExtractLocalizedStringForLang(label, def)
	prefix := nounWord(NounSourceCode, def) + " " + onWord(def) + " "
	if !strings.HasPrefix(cur, prefix) {
		return nil
	}
	if projectfile.ExtractLocalizedStringForLang(title, def) == "" {
		return nil
	}
	forge := strings.TrimPrefix(cur, prefix)
	return ComposeOnLabel(pf, title, forge)
}
