// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

func docWithI18N(v any) *projectfile.Document {
	return &projectfile.Document{Extensions: map[string]any{pfmodel.I18NExtensionNS: v}}
}

// keyLanguages is the languages key inside org.projectfile.i18n; declared once
// so goconst sees a single canonical literal across the fixtures.
const keyLanguages = "languages"

// TestLanguagesAbsentIsSingleLanguage pins the resting state: no namespace
// means every renderer emits only its canonical file.
func TestLanguagesAbsentIsSingleLanguage(t *testing.T) {
	assert.Nil(t, pfmodel.Languages(nil))
	assert.Nil(t, pfmodel.Languages(&projectfile.Document{}))
}

// TestLanguagesNormalizes verifies the list is cleaned once, centrally:
// empties dropped, duplicates collapsed, declaration order preserved (it is
// the order of the cross-language bar).
func TestLanguagesNormalizes(t *testing.T) {
	pf := docWithI18N(map[string]any{keyLanguages: []any{"uk", "", "es", "uk"}})
	assert.Equal(t, []string{"uk", "es"}, pfmodel.Languages(pf))
}

// TestLanguagesMalformedDegradesGracefully verifies a broken namespace never
// blocks the canonical file — a locale list is not worth failing a render.
func TestLanguagesMalformedDegradesGracefully(t *testing.T) {
	assert.Empty(t, pfmodel.Languages(docWithI18N("not-a-map")))
	assert.Empty(t, pfmodel.Languages(docWithI18N(map[string]any{keyLanguages: "es"})))
}

// TestDefaultLanguageDefaultsToEN pins the resting state: no namespace or no
// field means the canonical files are written in English.
func TestDefaultLanguageDefaultsToEN(t *testing.T) {
	assert.Equal(t, "en", pfmodel.DefaultLanguage(nil))
	assert.Equal(t, "en", pfmodel.DefaultLanguage(&projectfile.Document{}))
	assert.Equal(t, "en", pfmodel.DefaultLanguage(docWithI18N(map[string]any{})))
}

// TestDefaultLanguageHonoured verifies an explicit declaration flows through.
func TestDefaultLanguageHonoured(t *testing.T) {
	pf := docWithI18N(map[string]any{"default-language": "es"})
	assert.Equal(t, "es", pfmodel.DefaultLanguage(pf))
}

// TestDefaultLanguageMalformedDegradesGracefully verifies a broken namespace
// never blocks the canonical file with an error — it falls back to the spec
// default, like Languages does.
func TestDefaultLanguageMalformedDegradesGracefully(t *testing.T) {
	assert.Equal(t, "en", pfmodel.DefaultLanguage(docWithI18N("not-a-map")))
}

// TestLanguagesExcludesDefaultLanguage verifies the default language is never
// also a variant: it is the root file, so listing it in `languages` would
// double-render. It is dropped (with a diagnostic) instead.
func TestLanguagesExcludesDefaultLanguage(t *testing.T) {
	pf := docWithI18N(map[string]any{
		"default-language": "es",
		keyLanguages:       []any{"es", "uk"},
	})
	assert.Equal(t, []string{"uk"}, pfmodel.Languages(pf))
}
