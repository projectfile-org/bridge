// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

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
	return &projectfile.Document{Extensions: map[string]any{I18NExtensionNS: m}}
}

// TestOnWordFallsBackToEnglish pins the connector fallback: an undeclared locale still composes a readable label rather than an empty one.
func TestOnWordFallsBackToEnglish(t *testing.T) {
	assert.Equal(t, "on", onWord("en"))
	assert.Equal(t, "en", onWord("es"))
	assert.Equal(t, "на", onWord("uk"))
	assert.Equal(t, "on", onWord("fr"), "an unknown tag falls back to English")
	assert.Equal(t, "on", onWord(""), "an empty tag is the default language")
}

// TestNounWordFallsBackDownTheLadder pins the three rungs: the locale, English, then the key itself so an unknown combination never yields empty.
func TestNounWordFallsBackDownTheLadder(t *testing.T) {
	assert.Equal(t, "Repository", nounWord(NounSourceCode, "en"))
	assert.Equal(t, "Repositorio", nounWord(NounSourceCode, "es"))
	assert.Equal(t, "Репозиторій", nounWord(NounSourceCode, "uk"))
	assert.Equal(t, "Repository", nounWord(NounSourceCode, "fr"), "an unknown locale takes English")
	assert.Equal(t, "Paquetes", nounWord(NounPackages, "es"))
	assert.Equal(t, "Головна сторінка", nounWord(NounHomepage, "uk"))
	assert.Equal(t, "Documentation", nounWord(NounDocumentation, "en"))
	assert.Equal(t, "Issues", nounWord(NounIssues, "en"))
	assert.Equal(t, "unheard-of", nounWord("unheard-of", "en"),
		"a key with no table entry is its own subject")
	assert.Equal(t, "unheard-of", nounWord("unheard-of", "es"))
}

// TestNounLabelSingleLanguageIsBareAndMultiIsLangs pins the no-churn rule and the load-bearing Bare clear.
func TestNounLabelSingleLanguageIsBareAndMultiIsLangs(t *testing.T) {
	single := i18nDoc("en")
	bare := NounLabel(single, NounIssues)
	require.NotNil(t, bare)
	assert.Equal(t, "Issues", bare.Bare)
	assert.Empty(t, bare.Langs)

	multi := i18nDoc("en", "es", "uk")
	langs := NounLabel(multi, NounIssues)
	require.NotNil(t, langs)
	assert.Empty(t, langs.Bare, "a set Bare would mask every per-language entry")
	assert.Equal(t, map[string]string{
		"en": "Issues", "es": "Incidencias", "uk": "Задачі",
	}, langs.Langs)

	defaulted := NounLabel(&projectfile.Document{}, NounSourceCode)
	require.NotNil(t, defaulted)
	assert.Equal(t, "Repository", defaulted.Bare, "no i18n namespace is a single-language document")
}

// TestPromoteRepositoryLabelIsTheAlias pins the delegation, so the renamed concept and the older callers resolve identically.
func TestPromoteRepositoryLabelIsTheAlias(t *testing.T) {
	doc := &projectfile.Document{}
	label := &projectfile.LocalizedString{Bare: placeholderOnCodeberg}
	title := &projectfile.LocalizedString{Bare: testTitle}
	want := PromoteSourceCodeLabel(doc, label, title)
	got := PromoteRepositoryLabel(doc, label, title)
	require.NotNil(t, want)
	require.NotNil(t, got)
	assert.Equal(t, want, got)
	assert.Equal(t, "Projectfile on Codeberg", got.Bare)
}

// TestPromoteSourceCodeLabelRefusesAnythingButThePlaceholder pins the detection that keeps a curated label untouched.
func TestPromoteSourceCodeLabelRefusesAnythingButThePlaceholder(t *testing.T) {
	doc := i18nDoc("en")
	title := &projectfile.LocalizedString{Bare: testTitle}
	placeholder := &projectfile.LocalizedString{Bare: placeholderOnCodeberg}

	curated := &projectfile.LocalizedString{Bare: "Browse the code on Codeberg"}
	assert.Nil(t, PromoteSourceCodeLabel(doc, curated, title), "a curated label is left alone")
	assert.Nil(t, PromoteSourceCodeLabel(doc, placeholder, &projectfile.LocalizedString{Bare: ""}),
		"a project with no title has nothing to promote to")
	assert.Nil(t, PromoteSourceCodeLabel(doc, nil, title), "no label is no placeholder")
	assert.Nil(t, PromoteSourceCodeLabel(doc, placeholder, nil), "no title is no subject")
}

// TestPromoteSourceCodeLabelLocalizesWithTheDeclaredLanguages pins the per-language recomposition the scanner's placeholder cannot do.
func TestPromoteSourceCodeLabelLocalizesWithTheDeclaredLanguages(t *testing.T) {
	doc := i18nDoc("en", "es", "uk")
	promoted := PromoteSourceCodeLabel(doc,
		&projectfile.LocalizedString{Bare: placeholderOnCodeberg},
		&projectfile.LocalizedString{Bare: testTitle})
	require.NotNil(t, promoted)
	assert.Equal(t, map[string]string{
		"en": "Projectfile on Codeberg",
		"es": "Projectfile en Codeberg",
		"uk": "Projectfile на Codeberg",
	}, promoted.Langs)
	assert.Empty(t, promoted.Bare, "the Bare must be cleared or it masks every entry")
}
