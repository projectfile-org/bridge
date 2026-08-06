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
	pf := docWithI18N(map[string]any{"languages": []any{"uk", "", "es", "uk"}})
	assert.Equal(t, []string{"uk", "es"}, pfmodel.Languages(pf))
}

// TestLanguagesMalformedDegradesGracefully verifies a broken namespace never
// blocks the canonical file — a locale list is not worth failing a render.
func TestLanguagesMalformedDegradesGracefully(t *testing.T) {
	assert.Empty(t, pfmodel.Languages(docWithI18N("not-a-map")))
	assert.Empty(t, pfmodel.Languages(docWithI18N(map[string]any{"languages": "es"})))
}
