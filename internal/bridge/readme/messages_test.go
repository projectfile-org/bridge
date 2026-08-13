// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// langES / langUK are the shipped translations; declared once so the catalog
// tests and the render tests name them the same way (goconst-clean).
const (
	langES = "es"
	langUK = "uk"
)

// TestCatalogsCoverDefaultKeys is the completeness gate: every shipped
// translation must answer every key the canonical catalog defines. A partial
// catalog silently falls back to English at render time, which is exactly the
// half-translated readme this feature exists to eliminate.
func TestCatalogsCoverDefaultKeys(t *testing.T) {
	catalogsOnce.Do(loadCatalogs)
	require.NotEmpty(t, catalogs[defaultCatalog], "the canonical catalog must load")

	for lang, m := range catalogs {
		if lang == defaultCatalog {
			continue
		}
		for key := range catalogs[defaultCatalog] {
			assert.Containsf(t, m, key, "messages/%s.yaml is missing %q", lang, key)
		}
		for key := range m {
			assert.Containsf(t, catalogs[defaultCatalog], key,
				"messages/%s.yaml defines %q, which messages/%s.yaml does not",
				lang, key, defaultCatalog)
		}
	}
}

// TestCatalogBodyPlaceholders pins the substitution contract for every *.body
// message in every language. license.body interpolates the SPDX id once (%s);
// every other body carries %[1]s (the bare filename, link text) and %[2]s
// (its rebased target) exactly once each. A translation that drops, doubles,
// or swaps the forms renders "%!s(MISSING)" into a published readme.
func TestCatalogBodyPlaceholders(t *testing.T) {
	catalogsOnce.Do(loadCatalogs)
	for lang, m := range catalogs {
		for key, msg := range m {
			if !strings.HasSuffix(key, ".body") {
				continue
			}
			if key == "license.body" {
				assert.Equalf(t, 1, strings.Count(msg, "%s"),
					"messages/%s.yaml: %q must carry exactly one %%s", lang, key)
				continue
			}
			assert.Equalf(t, 1, strings.Count(msg, "%[1]s"),
				"messages/%s.yaml: %q must carry exactly one %%[1]s", lang, key)
			assert.Equalf(t, 1, strings.Count(msg, "%[2]s"),
				"messages/%s.yaml: %q must carry exactly one %%[2]s", lang, key)
			assert.NotContainsf(t, msg, "%s",
				"messages/%s.yaml: %q must use %%[1]s/%%[2]s, not a bare %%s", lang, key)
		}
	}
}

// TestTranslateFallsBackToDefault covers the three-step resolution: the
// language's own catalog, then English, then the key itself.
func TestTranslateFallsBackToDefault(t *testing.T) {
	assert.Equal(t, "Licencia", translate(langES, "license.title"))
	assert.Equal(t, "License", translate("", "license.title"))
	assert.Equal(t, "License", translate("fr", "license.title"),
		"a language with no catalog falls back to the default one")
	assert.Equal(t, "no.such.key", translate(langES, "no.such.key"),
		"an unknown key echoes itself so it is visible and greppable")
}

// TestLookupMessageReportsMisses guards the bool contract the Go-side callers
// depend on for their own fallbacks (a bare filename, a raw link type).
func TestLookupMessageReportsMisses(t *testing.T) {
	v, ok := lookupMessage(langUK, "policy.security")
	assert.True(t, ok)
	assert.Equal(t, "Політика безпеки", v)

	_, ok = lookupMessage(langUK, keyPrefixLinkType+"not-a-type")
	assert.False(t, ok)
}

// TestHealthFileLabelDerivesKey pins the filename → catalog-key convention
// that keeps a new health file from needing a Go edit.
func TestHealthFileLabelDerivesKey(t *testing.T) {
	assert.Equal(t, "Code of Conduct", healthFileLabel(fileCodeOfConduct, ""))
	assert.Equal(t, "Código de conducta", healthFileLabel(fileCodeOfConduct, langES))
	assert.Equal(t, "UNKNOWN.md", healthFileLabel("UNKNOWN.md", langES),
		"a file with no catalog entry falls back to its own name")
}

// TestRenderLocalizedSections is the end-to-end proof that the documented gap
// is closed: README.<lang>.md carries translated headings, probe labels and
// link-group headings — not English ones under a localized filename.
func TestRenderLocalizedSections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileContributing, "# c\n")
	writeFile(t, dir, "INSTALL.md", "# i\n")

	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{{Type: linkTypeSourceCode, URL: urlExampleRepo}}
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{keyLanguages: []any{langES, langUK}},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	require.Len(t, out.Files, 3)

	es := string(out.Files["docs/es/README.md"])
	assert.Contains(t, es, "## Instalación")
	// Body doc-link uses the filename as text, rebased from docs/es/.
	assert.Contains(t, es, "Consulta [INSTALL.md](../../INSTALL.md)")
	assert.Contains(t, es, "## Políticas")
	// Health-file link keeps its localized label, rebased from docs/es/.
	assert.Contains(t, es, "[Cómo contribuir](../../CONTRIBUTING.md)")
	// A single-group links section renders no ### subheading — the heading
	// would only repeat "## Enlaces". The localized link still renders.
	assert.NotContains(t, es, "### Proyecto")
	assert.Contains(t, es, "[Código fuente](https://example.com/repo)")
	assert.NotContains(t, es, "## Installation")

	uk := string(out.Files["docs/uk/README.md"])
	assert.Contains(t, uk, "## Встановлення")
	assert.Contains(t, uk, "## Політики")
	assert.NotContains(t, uk, "## Policies")

	en := string(out.Files[filenameReadme])
	assert.Contains(t, en, "## Installation")
	assert.NotContains(t, en, "### Project", "single-group links render no subheading")
}

// TestRenderPrefersLanguageTemplate covers the per-language template tier: a
// block a project wants to restructure (not merely translate) in one language
// ships <block>.<lang>.tmpl, and the neutral template still serves the rest.
func TestRenderPrefersLanguageTemplate(t *testing.T) {
	dir := t.TempDir()
	overrideDir := filepath.Join(dir, core.LocalTemplatesDir, readmeBlockDir)
	require.NoError(t, os.MkdirAll(overrideDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(overrideDir, "license.es.tmpl"),
		[]byte("SOLO-EN-ESPAÑOL\n"), 0o644))

	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{keyLanguages: []any{langES, langUK}},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)

	assert.Contains(t, string(out.Files["docs/es/README.md"]), "SOLO-EN-ESPAÑOL")
	assert.NotContains(t, string(out.Files["docs/es/README.md"]), "## Licencia")
	assert.Contains(t, string(out.Files["docs/uk/README.md"]), "## Ліцензія",
		"a sibling language keeps the neutral template")
	assert.Contains(t, string(out.Files[filenameReadme]), "## License")
}
