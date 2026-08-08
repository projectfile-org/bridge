// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// TestLocalizedFilename pins the naming rule every localized artefact uses:
// the locale lives in the directory (docs/<lang>/), and the canonical file
// (empty lang) stays at the root.
func TestLocalizedFilename(t *testing.T) {
	assert.Equal(t, "CONTRIBUTING.md", core.LocalizedFilename("CONTRIBUTING.md", ""))
	assert.Equal(t, "docs/es/CONTRIBUTING.md", core.LocalizedFilename("CONTRIBUTING.md", "es"))
	assert.Equal(t, "docs/pt-BR/README.md", core.LocalizedFilename("README.md", "pt-BR"))
	assert.Equal(t, "docs/uk/CODE_OF_CONDUCT.md", core.LocalizedFilename(core.FileCodeOfConduct, "uk"))
}

// TestLocalizedTemplateName pins that a variant's template keeps the tag infix
// before the extension regardless of the output directory — template lookup
// did not move, only disk output did.
func TestLocalizedTemplateName(t *testing.T) {
	assert.Equal(t, "SUPPORT.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", ""))
	assert.Equal(t, "SUPPORT.es.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", "es"))
}

// supportVariant is the docs/<lang>/ path the language-links tests expect for
// a SUPPORT variant; declared once so goconst sees a single literal.
const supportVariantUK = "docs/uk/SUPPORT.md"

// labelUkrainian is the Ukrainian endonym used in the language-links tests;
// declared once so goconst sees a single literal.
const labelUkrainian = "Українська"

// TestLanguageLinksExcludesActive verifies the cross-language bar lists every
// other variant including the canonical file, points variants at docs/<lang>/,
// and never links to itself. The canonical entry is labelled with the default
// language's endonym.
func TestLanguageLinksExcludesActive(t *testing.T) {
	langs := []string{"es", "uk"}

	en := core.LanguageLinks(core.FileSupport, "", "en", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "es", Label: "Español", Filename: "docs/es/SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: supportVariantUK},
	}, en, "canonical variant lists the translations only")

	es := core.LanguageLinks(core.FileSupport, "es", "en", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "", Label: "English", Filename: "SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: supportVariantUK},
	}, es, "a translation links back to the canonical file and the siblings")
}

// TestLanguageLinksDefaultLanguageLabel verifies that when the project's
// default language is not English, the canonical entry is labelled with that
// language's endonym — a Spanish-first project's root file reads "Español".
func TestLanguageLinksDefaultLanguageLabel(t *testing.T) {
	langs := []string{"en", "uk"}
	en := core.LanguageLinks(core.FileSupport, "en", "es", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "", Label: "Español", Filename: "SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: supportVariantUK},
	}, en, "canonical entry uses the default-language endonym")
}

// TestLanguageLinksSingleLanguage verifies a project with no declared
// languages grows no bar at all.
func TestLanguageLinksSingleLanguage(t *testing.T) {
	assert.Nil(t, core.LanguageLinks(core.FileSupport, "", "en", nil))
}

// TestInsertLanguageBarAboveH1 verifies the bar lands above the title and
// below whatever the template emitted first (the managed marker), so marker
// detection and the reader's first glance both still work.
func TestInsertLanguageBarAboveH1(t *testing.T) {
	body := []byte("<!-- pf-cli-managed: yes -->\n\n# Getting Support\n\nBody.\n")
	out := string(core.InsertLanguageBar(body, core.FileSupport, "", "en", []string{"es"}))
	assert.Equal(t,
		"<!-- pf-cli-managed: yes -->\n\n[Español](docs/es/SUPPORT.md)\n\n# Getting Support\n\nBody.\n",
		out)
}

// TestInsertLanguageBarNoLanguagesIsNoop guards the single-language path: an
// unlocalized project's files must come out byte-identical to before.
func TestInsertLanguageBarNoLanguagesIsNoop(t *testing.T) {
	body := []byte("# Title\n\nBody.\n")
	assert.Equal(t, body, core.InsertLanguageBar(body, core.FileSupport, "", "en", nil))
}

// TestInsertLanguageBarNoHeading covers the degenerate body with no H1: the
// bar still renders, at the top, rather than being silently dropped.
func TestInsertLanguageBarNoHeading(t *testing.T) {
	out := string(core.InsertLanguageBar([]byte("no heading here\n"), core.FileSupport, "es", "en", []string{"es"}))
	assert.Equal(t, "[English](SUPPORT.md)\n\nno heading here\n", out)
}

// TestLocalizedSiblingStaysInLanguage verifies a cross-reference from a
// localized document points at the sibling's matching variant, and that the
// canonical document keeps the canonical link.
func TestLocalizedSiblingStaysInLanguage(t *testing.T) {
	assert.Equal(t, "SECURITY.md", core.LocalizedSibling(core.FileSecurity, ""))
	assert.Equal(t, "docs/es/SECURITY.md", core.LocalizedSibling(core.FileSecurity, "es"))
}

// TestCollapseBlankLines pins the shared whitespace contract every renderer
// now goes through.
func TestCollapseBlankLines(t *testing.T) {
	assert.Equal(t, "a\n\nb\n", string(core.CollapseBlankLines([]byte("a\n\n\n\nb\n\n\n"))))
}
