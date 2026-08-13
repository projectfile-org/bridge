// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// testTitle is the proper-noun subject used across the compose/promote tests.
const testTitle = "Projectfile Bridges"

// i18nDoc builds a document declaring default-language plus extra languages.
func i18nDoc(def string, langs ...string) *projectfile.Document {
	m := map[string]any{}
	if def != "" {
		m["default-language"] = def
	}
	if len(langs) > 0 {
		vals := make([]any, 0, len(langs))
		for _, l := range langs {
			vals = append(vals, l)
		}
		m["languages"] = vals
	}
	return &projectfile.Document{Extensions: map[string]any{pfmodel.I18NExtensionNS: m}}
}

// TestComposeOnLabelSingleLanguageIsBare pins the no-churn rule: a project
// with no extra languages keeps the label Bare, so a single-language doc does
// not gain a Langs map on re-derive.
func TestComposeOnLabelSingleLanguageIsBare(t *testing.T) {
	label := pfmodel.ComposeOnLabel(&projectfile.Document{}, nil, "Codeberg")
	require.NotNil(t, label)
	assert.Equal(t, "on Codeberg", label.Bare)
	assert.Empty(t, label.Langs)
}

// TestComposeOnLabelMultiLanguageBuildsLangs verifies the connector translates
// per language and Bare is CLEARED — the load-bearing gotcha: a set Bare would
// mask the Langs map at render time (core ExtractLocalizedStringForLang returns
// Bare first).
func TestComposeOnLabelMultiLanguageBuildsLangs(t *testing.T) {
	pf := i18nDoc("en", "es", "uk")
	title := &projectfile.LocalizedString{Bare: testTitle}
	label := pfmodel.ComposeOnLabel(pf, title, "Codeberg")
	require.NotNil(t, label)
	assert.Empty(t, label.Bare, "Bare must be cleared or it masks the Langs map")
	assert.Equal(t, map[string]string{
		"en": "Projectfile Bridges on Codeberg",
		"es": "Projectfile Bridges en Codeberg",
		"uk": "Projectfile Bridges на Codeberg",
	}, label.Langs)
}

// TestComposeOnLabelNilForgeReturnsNil guards the producers: no forge means no
// label is written (the link keeps whatever label it had).
func TestComposeOnLabelNilForgeReturnsNil(t *testing.T) {
	assert.Nil(t, pfmodel.ComposeOnLabel(i18nDoc("en", "es"), nil, ""))
}

// TestComposeOnLabelDefaultLanguageIsFirst verifies a non-en default language
// anchors the Bare form and leads the Langs map.
func TestComposeOnLabelDefaultLanguageIsFirst(t *testing.T) {
	pf := i18nDoc("es", "uk")
	label := pfmodel.ComposeOnLabel(pf, &projectfile.LocalizedString{Bare: "Proyecto"}, "GitHub")
	require.NotNil(t, label)
	assert.Empty(t, label.Bare)
	assert.Equal(t, "Proyecto en GitHub", label.Langs["es"])
	assert.Equal(t, "Proyecto на GitHub", label.Langs["uk"])
}

// TestNounLabelTranslatesSubjects verifies the type-noun subjects localise
// (the connector and noun are independent tables, so "Incidencias" and "en"
// both land in the es entry).
func TestNounLabelTranslatesSubjects(t *testing.T) {
	pf := i18nDoc("en", "es", "uk")
	assert.Equal(t, map[string]string{
		"en": "Issues", "es": "Incidencias", "uk": "Issues",
	}, pfmodel.NounLabel(pf, pfmodel.NounIssues).Langs)
	assert.Equal(t, map[string]string{
		"en": "Packages", "es": "Paquetes", "uk": "Пакунки",
	}, pfmodel.NounLabel(pf, pfmodel.NounPackages).Langs)
	assert.Equal(t, map[string]string{
		"en": "Source Code", "es": "Código fuente", "uk": "Вихідний код",
	}, pfmodel.NounLabel(pf, pfmodel.NounSourceCode).Langs)
}

// TestComposeOnLabelWithNounSubject checks the full Issues/Packages composition
// the forges/registries derivers emit.
func TestComposeOnLabelWithNounSubject(t *testing.T) {
	pf := i18nDoc("en", "es", "uk")
	label := pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounIssues), "Codeberg")
	require.NotNil(t, label)
	assert.Empty(t, label.Bare)
	assert.Equal(t, map[string]string{
		"en": "Issues on Codeberg",
		"es": "Incidencias en Codeberg",
		"uk": "Issues на Codeberg",
	}, label.Langs)
}

// TestPromoteSourceCodeLabelReplacesNounWithTitle verifies the scan
// title-rewrite turns a scanner placeholder into the title, localised — in
// every language at once, not just the default.
func TestPromoteSourceCodeLabelReplacesNounWithTitle(t *testing.T) {
	pf := i18nDoc("en", "es", "uk")
	title := &projectfile.LocalizedString{Bare: testTitle}
	// Placeholder the scanner would write (localized noun subject).
	placeholder := pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounSourceCode), "Codeberg")
	promoted := pfmodel.PromoteSourceCodeLabel(pf, placeholder, title)
	require.NotNil(t, promoted)
	assert.Empty(t, promoted.Bare)
	assert.Equal(t, "Projectfile Bridges on Codeberg", promoted.Langs["en"])
	assert.Equal(t, "Projectfile Bridges en Codeberg", promoted.Langs["es"])
}

// TestPromoteSourceCodeLabelLeavesCuratedAlone verifies a hand-written or
// already-promoted label is NOT rewritten — the detection only fires on the
// scanner placeholder.
func TestPromoteSourceCodeLabelLeavesCuratedAlone(t *testing.T) {
	pf := i18nDoc("en", "es", "uk")
	title := &projectfile.LocalizedString{Bare: testTitle}
	curated := &projectfile.LocalizedString{Bare: "My Custom Label"}
	assert.Nil(t, pfmodel.PromoteSourceCodeLabel(pf, curated, title))
	// Already-promoted (title subject) is also a no-op.
	promoted := pfmodel.ComposeOnLabel(pf, title, "Codeberg")
	assert.Nil(t, pfmodel.PromoteSourceCodeLabel(pf, promoted, title))
}

// TestPromoteSourceCodeLabelBarePlaceholder verifies a Bare placeholder (a
// single-language scanner result) is still promoted, so the rewrite works for
// docs that declare no extra languages.
func TestPromoteSourceCodeLabelBarePlaceholder(t *testing.T) {
	pf := &projectfile.Document{}
	title := &projectfile.LocalizedString{Bare: testTitle}
	placeholder := pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounSourceCode), "Codeberg")
	promoted := pfmodel.PromoteSourceCodeLabel(pf, placeholder, title)
	require.NotNil(t, promoted)
	assert.Equal(t, "Projectfile Bridges on Codeberg", promoted.Bare)
	assert.Empty(t, promoted.Langs)
}

// TestSetLinkLabelGapFill verifies an existing label wins and force overrides —
// the same gap contract SetLinkTags uses, applied to the label field.
func TestSetLinkLabelGapFill(t *testing.T) {
	doc := &projectfile.Document{Links: []projectfile.Link{
		{Type: "bugs", URL: "https://example.com/issues"},
	}}
	derived := &projectfile.LocalizedString{Bare: "Issues on Example"}

	// First write gap-fills the empty slot.
	assert.True(t, pfmodel.SetLinkLabel(doc, "bugs", "https://example.com/issues", derived, false))
	assert.Equal(t, derived, doc.Links[0].Label)

	// Second write is a no-op without force: the curated label survives.
	curated := &projectfile.LocalizedString{Bare: "Curated"}
	assert.False(t, pfmodel.SetLinkLabel(doc, "bugs", "https://example.com/issues", curated, false))
	assert.Equal(t, derived, doc.Links[0].Label)

	// Force overwrites.
	assert.True(t, pfmodel.SetLinkLabel(doc, "bugs", "https://example.com/issues", curated, true))
	assert.Equal(t, curated, doc.Links[0].Label)
}

// TestSetLinkLabelNoMatchingLink verifies a write against a (type, url) that is
// not present writes nothing and reports false.
func TestSetLinkLabelNoMatchingLink(t *testing.T) {
	doc := &projectfile.Document{Links: []projectfile.Link{
		{Type: "bugs", URL: "https://example.com/issues"},
	}}
	label := &projectfile.LocalizedString{Bare: "x"}
	assert.False(t, pfmodel.SetLinkLabel(doc, "bugs", "https://other.example.com/issues", label, true))
	assert.Nil(t, doc.Links[0].Label)
}

// TestSetLinkLabelNilArgsIsNoop pins the defensive guards so a nil doc, label
// or empty selector never panics or writes.
func TestSetLinkLabelNilArgsIsNoop(t *testing.T) {
	label := &projectfile.LocalizedString{Bare: "x"}
	assert.False(t, pfmodel.SetLinkLabel(nil, "bugs", "u", label, true))
	assert.False(t, pfmodel.SetLinkLabel(&projectfile.Document{}, "bugs", "u", nil, true))
	assert.False(t, pfmodel.SetLinkLabel(&projectfile.Document{}, "", "u", label, true))
}
