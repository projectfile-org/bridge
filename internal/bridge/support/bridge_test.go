// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package support_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/support"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// localizedDoc builds a minimal document declaring langs on the shared
// org.projectfile.i18n namespace.
func localizedDoc(langs ...string) *projectfile.Document {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Name:  "widget",
			Title: &projectfile.LocalizedString{Langs: map[string]string{"en": "Widget", "es": "Artilugio"}},
		},
	}
	if len(langs) > 0 {
		items := make([]any, len(langs))
		for i, l := range langs {
			items[i] = l
		}
		projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{"languages": items})
	}
	return pf
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestRenderSingleLanguage is the unlocalized baseline: no i18n namespace
// means exactly the canonical file and no cross-language bar.
func TestRenderSingleLanguage(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc(), core.Options{Offline: true})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	assert.NotContains(t, string(out.Files["SUPPORT.md"]), "](SUPPORT.")
}

// TestRenderEmitsOneFilePerShippedLanguage verifies the declared languages
// drive the multi-file output for a community health file: the canonical file
// at the root, one variant per language under docs/<lang>/.
func TestRenderEmitsOneFilePerShippedLanguage(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es", "uk"), core.Options{Offline: true})
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"SUPPORT.md", "docs/es/SUPPORT.md", "docs/uk/SUPPORT.md"},
		keys(out.Files))
}

// TestRenderResolvesTitlePerLanguage verifies localized-string fields resolve
// in the variant's own language rather than falling back to English.
func TestRenderResolvesTitlePerLanguage(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es"), core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["SUPPORT.md"]), "**Widget**")
	assert.Contains(t, string(out.Files["docs/es/SUPPORT.md"]), "**Artilugio**")
}

// TestRenderTranslatesRowVocabulary verifies the "Where to Ask" question
// column comes from the template, not from Go — the whole reason tableRow
// carries a Kind instead of an English sentence.
func TestRenderTranslatesRowVocabulary(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es"), core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["SUPPORT.md"]), "**Report a security vulnerability**")
	assert.Contains(t, string(out.Files["docs/es/SUPPORT.md"]), "**Informar de una vulnerabilidad de seguridad**")
}

// TestRenderCrossLinksStayInLanguage verifies a localized document links the
// localized sibling, so a reader who arrived in Spanish stays in Spanish.
func TestRenderCrossLinksStayInLanguage(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es"), core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["SUPPORT.md"]), "[SECURITY.md](SECURITY.md)")
	assert.Contains(t, string(out.Files["docs/es/SUPPORT.md"]), "[docs/es/SECURITY.md](docs/es/SECURITY.md)")
}

// TestRenderSkipsUntranslatedLanguage is the missing-translation policy: a
// declared language with no template produces no file at all — never the
// default-language body under a localized name — and never appears in the bar
// of the files that did render.
func TestRenderSkipsUntranslatedLanguage(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es", "zz"), core.Options{Offline: true})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"SUPPORT.md", "docs/es/SUPPORT.md"}, keys(out.Files))
	assert.NotContains(t, string(out.Files["SUPPORT.md"]), "docs/zz/SUPPORT.md",
		"a skipped language must not be advertised in the cross-language bar")
}

// TestRenderCrossLanguageBar verifies every variant links the others and
// never itself.
func TestRenderCrossLanguageBar(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es", "uk"), core.Options{Offline: true})
	require.NoError(t, err)

	en := string(out.Files["SUPPORT.md"])
	assert.Contains(t, en, "[Español](docs/es/SUPPORT.md)")
	assert.Contains(t, en, "[Українська](docs/uk/SUPPORT.md)")

	es := string(out.Files["docs/es/SUPPORT.md"])
	assert.Contains(t, es, "[English](SUPPORT.md)")
	assert.NotContains(t, es, "[Español](docs/es/SUPPORT.md)", "a variant must not link to itself")
}
