// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package support_test

import (
	"strings"
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

// TestRenderDropsMCVEGuideLine: the how-to-ask list stands on its own — the
// old "See an MCVE guide for tips." pointer added nothing the five points
// above it had not said, in every language.
func TestRenderDropsMCVEGuideLine(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc("es", "uk"), core.Options{Offline: true})
	require.NoError(t, err)
	for name, body := range out.Files {
		assert.NotContains(t, string(body), "MCVE", "file="+name)
	}
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
	// The sibling is a co-located file: basename text, same-directory URL —
	// identical from the root and from docs/es/.
	assert.Contains(t, string(out.Files["SUPPORT.md"]), "[SECURITY.md](SECURITY.md)")
	assert.Contains(t, string(out.Files["docs/es/SUPPORT.md"]), "[SECURITY.md](SECURITY.md)")
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
	assert.Contains(t, es, "[English](../../SUPPORT.md)")
	assert.NotContains(t, es, "[Español](docs/es/SUPPORT.md)", "a variant must not link to itself")
}

// TestRenderBeforeLinksPriorityOrdersWithinType: a §139 `priority` on a
// support-tagged links[] entry reorders the "Before You Ask" list (higher
// first). Before You Ask is tag-driven, so the entries carry the `support` tag.
func TestRenderBeforeLinksPriorityOrdersWithinType(t *testing.T) {
	pf := localizedDoc()
	pf.Links = []projectfile.Link{
		{
			Type: projectfile.LinkDocumentation, URL: "https://example.test/docs",
			Label: &projectfile.LocalizedString{Bare: "Docs"},
			Extra: map[string]any{"tags": []any{"support"}},
		},
		{
			Type: projectfile.LinkDocumentation, URL: "https://example.test/guide",
			Label: &projectfile.LocalizedString{Bare: "Pinned Guide"},
			Extra: map[string]any{"priority": 300, "tags": []any{"support"}},
		},
	}

	out, err := support.Bridge{}.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["SUPPORT.md"])

	guideIdx := strings.Index(body, "[Pinned Guide]")
	docsIdx := strings.Index(body, "[Docs]")
	require.NotEqual(t, -1, guideIdx)
	require.NotEqual(t, -1, docsIdx)
	assert.Less(t, guideIdx, docsIdx,
		"higher-priority documentation link renders first in Before You Ask")
}

// TestRenderPaidSupportBlock: a links[type=paid-support] entry is the "paid
// support available" flag — it renders a dedicated end block in each language,
// NOT a "Where to Ask" row, so the link appears once and only inside the block.
func TestRenderPaidSupportBlock(t *testing.T) {
	pf := localizedDoc("es")
	pf.Links = []projectfile.Link{
		{
			Type:  "paid-support",
			URL:   "https://example.test/enterprise",
			Label: &projectfile.LocalizedString{Bare: "Enterprise Support"},
		},
	}

	out, err := support.Bridge{}.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	en := string(out.Files["SUPPORT.md"])
	es := string(out.Files["docs/es/SUPPORT.md"])

	assert.Contains(t, en, "## Paid Support")
	assert.Contains(t, en, "Paid support for Widget is available:")
	assert.Contains(t, es, "## Soporte de pago")
	assert.Contains(t, es, "Hay soporte de pago disponible para Artilugio:")
	assert.Contains(t, en, "- [Enterprise Support](https://example.test/enterprise)")

	// Not a "Where to Ask" row: the URL shows up exactly once, after the block
	// heading rather than in the table above it.
	assert.Equal(t, 1, strings.Count(en, "https://example.test/enterprise"))
	assert.Greater(t, strings.Index(en, "https://example.test/enterprise"),
		strings.Index(en, "## Paid Support"))
	assert.NotContains(t, en, "commercial support / SLA", "removed row label must not render")
}

// TestRenderPaidSupportEmail: a mailto: URL renders as a clickable email link,
// so the same links[type=paid-support] entry covers the email case for free.
func TestRenderPaidSupportEmail(t *testing.T) {
	pf := localizedDoc()
	pf.Links = []projectfile.Link{
		{
			Type:  "paid-support",
			URL:   "mailto:sales@example.test",
			Label: &projectfile.LocalizedString{Bare: "Sales"},
		},
	}

	out, err := support.Bridge{}.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["SUPPORT.md"]), "- [Sales](mailto:sales@example.test)")
}

// TestRenderNoPaidSupportOmitsBlock: no links[type=paid-support] entry means no
// block at all — the flag is the link's presence.
func TestRenderNoPaidSupportOmitsBlock(t *testing.T) {
	out, err := support.Bridge{}.Render(localizedDoc(), core.Options{Offline: true})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files["SUPPORT.md"]), "## Paid Support")
}
