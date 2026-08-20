// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// The fragments bridge's structural strings, per language. Fragments are data,
// not prose, so — unlike the health files — there is no translated template:
// one structural template serves every language and only the strings the
// assembler itself contributes (the document title, the section headings)
// localize. A handful of keys does not justify a YAML catalog, so the strings
// live in a Go map, the same shape core's GeneratedFooter footerStrings uses.
//
// Keys whose suffix is derived, never hand-written: `title.<stem>` where stem
// is the document's out filename lowercased without extension (FEATURES.md →
// title.features). A document without an entry keeps its declared title in
// every language — graceful degradation, not an error.

// Catalog key constants — one home per key, used by every language map.
const (
	keyProjectHeading = "project.heading"
	keyInheritedPlain = "inherited.plain"
	keyTitlePrefix    = "title."
)

// fragmentsStrings maps lang → key → format string.
var fragmentsStrings = map[string]map[string]string{
	"en": {
		keyProjectHeading:           "Project %s",
		keyInheritedPlain:           "Inherited from %s",
		keyTitlePrefix + "features": "Features",
		keyTitlePrefix + "roadmap":  "Roadmap",
	},
	"es": {
		keyProjectHeading:           "%s del proyecto",
		keyInheritedPlain:           "Heredado de %s",
		keyTitlePrefix + "features": "Características",
		keyTitlePrefix + "roadmap":  "Hoja de ruta",
	},
	"uk": {
		keyProjectHeading:           "%s проєкту",
		keyInheritedPlain:           "Успадковано від %s",
		keyTitlePrefix + "features": "Можливості",
		keyTitlePrefix + "roadmap":  "Дорожня карта",
	},
}

// fragmentString resolves one structural string in lang, falling back to
// English per key — the same per-key fallback the readme catalog runs, so a
// half-translated language degrades visibly instead of dropping the render.
func fragmentString(lang, key string) (string, bool) {
	if lang != "" && lang != "en" {
		if v, ok := fragmentsStrings[lang][key]; ok {
			return v, true
		}
		genlog.Decision("fragment_string", key, "fragments strings", "untranslated in "+lang)
	}
	v, ok := fragmentsStrings["en"][key]
	return v, ok
}

// localizedTitle resolves the document's H1 title for lang: the catalog entry
// keyed by the out stem when one exists, else the declared title verbatim.
func localizedDocTitle(title, out, lang string) string {
	stem := strings.ToLower(strings.TrimSuffix(out, pathExt(out)))
	if v, ok := fragmentString(lang, keyTitlePrefix+stem); ok {
		return v
	}
	return title
}

// localizedProjectHeading resolves the "## Project Features" heading for lang.
// The pattern is one message (not a prefix) because word order differs per
// language: Spanish puts the noun first ("Características del proyecto").
func localizedProjectHeading(title, out, lang string) string {
	if v, ok := fragmentString(lang, "project.heading"); ok {
		return "## " + fmt.Sprintf(v, localizedDocTitle(title, out, lang))
	}
	return "## Project " + title
}

// localizedInheritedHeading resolves a parent section heading for lang. The
// heading names the parent only — never a version, which would rewrite every
// child document on each parent release. The English shape stays the fallback
// so a copy that resolves nothing still renders a heading.
func localizedInheritedHeading(c inheritedCopy, lang string) string {
	if v, ok := fragmentString(lang, keyInheritedPlain); ok {
		return "## " + fmt.Sprintf(v, c.displayName())
	}
	return c.Heading()
}

// pathExt is filepath.Ext for forward-slash paths (out names are repo-relative).
func pathExt(p string) string {
	if i := strings.LastIndexByte(p, '.'); i >= 0 && i > strings.LastIndexByte(p, '/') {
		return p[i:]
	}
	return ""
}
