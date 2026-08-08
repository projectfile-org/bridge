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
// the BCP 47 tag goes before the extension, and the canonical file (empty
// lang) passes through untouched.
func TestLocalizedFilename(t *testing.T) {
	assert.Equal(t, "CONTRIBUTING.md", core.LocalizedFilename("CONTRIBUTING.md", ""))
	assert.Equal(t, "CONTRIBUTING.es.md", core.LocalizedFilename("CONTRIBUTING.md", "es"))
	assert.Equal(t, "README.pt-BR.md", core.LocalizedFilename("README.md", "pt-BR"))
	assert.Equal(t, "CODE_OF_CONDUCT.uk.md", core.LocalizedFilename(core.FileCodeOfConduct, "uk"))
}

// TestLocalizedTemplateName pins that a variant's template carries the tag in
// the same position as the file it renders — the rule that lets a translator
// add a locale by dropping in one file.
func TestLocalizedTemplateName(t *testing.T) {
	assert.Equal(t, "SUPPORT.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", ""))
	assert.Equal(t, "SUPPORT.es.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", "es"))
}

// TestLanguageLinksExcludesActive verifies the cross-language bar lists every
// other variant including the canonical file, and never links to itself.
func TestLanguageLinksExcludesActive(t *testing.T) {
	langs := []string{"es", "uk"}

	en := core.LanguageLinks(core.FileSupport, "", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "es", Label: "Español", Filename: "SUPPORT.es.md"},
		{Code: "uk", Label: "Українська", Filename: "SUPPORT.uk.md"},
	}, en, "canonical variant lists the translations only")

	es := core.LanguageLinks(core.FileSupport, "es", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "", Label: "English", Filename: "SUPPORT.md"},
		{Code: "uk", Label: "Українська", Filename: "SUPPORT.uk.md"},
	}, es, "a translation links back to the canonical file and the siblings")
}

// TestLanguageLinksSingleLanguage verifies a project with no declared
// languages grows no bar at all.
func TestLanguageLinksSingleLanguage(t *testing.T) {
	assert.Nil(t, core.LanguageLinks(core.FileSupport, "", nil))
}

// TestInsertLanguageBarAboveH1 verifies the bar lands above the title and
// below whatever the template emitted first (the managed marker), so marker
// detection and the reader's first glance both still work.
func TestInsertLanguageBarAboveH1(t *testing.T) {
	body := []byte("<!-- pf-cli-managed: yes -->\n\n# Getting Support\n\nBody.\n")
	out := string(core.InsertLanguageBar(body, core.FileSupport, "", []string{"es"}))
	assert.Equal(t,
		"<!-- pf-cli-managed: yes -->\n\n[Español](SUPPORT.es.md)\n\n# Getting Support\n\nBody.\n",
		out)
}

// TestInsertLanguageBarNoLanguagesIsNoop guards the single-language path: an
// unlocalized project's files must come out byte-identical to before.
func TestInsertLanguageBarNoLanguagesIsNoop(t *testing.T) {
	body := []byte("# Title\n\nBody.\n")
	assert.Equal(t, body, core.InsertLanguageBar(body, core.FileSupport, "", nil))
}

// TestInsertLanguageBarNoHeading covers the degenerate body with no H1: the
// bar still renders, at the top, rather than being silently dropped.
func TestInsertLanguageBarNoHeading(t *testing.T) {
	out := string(core.InsertLanguageBar([]byte("no heading here\n"), core.FileSupport, "es", []string{"es"}))
	assert.Equal(t, "[English](SUPPORT.md)\n\nno heading here\n", out)
}

// TestLocalizedSiblingStaysInLanguage verifies a cross-reference from a
// localized document points at the sibling's matching variant, and that the
// canonical document keeps the canonical link.
func TestLocalizedSiblingStaysInLanguage(t *testing.T) {
	assert.Equal(t, "SECURITY.md", core.LocalizedSibling(core.FileSecurity, ""))
	assert.Equal(t, "SECURITY.es.md", core.LocalizedSibling(core.FileSecurity, "es"))
}

// TestCollapseBlankLines pins the shared whitespace contract every renderer
// now goes through.
func TestCollapseBlankLines(t *testing.T) {
	assert.Equal(t, "a\n\nb\n", string(core.CollapseBlankLines([]byte("a\n\n\n\nb\n\n\n"))))
}
