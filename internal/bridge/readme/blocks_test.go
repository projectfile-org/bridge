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

// TestDefaultBlocksOrder pins the canonical README composition. Adding or
// reordering a block is an intentional contract change — fail loudly here so
// the docs/readme-generator.md table stays in sync.
func TestDefaultBlocksOrder(t *testing.T) {
	assert.Equal(t, []string{
		blockLanguages, blockLogo, blockBasics, blockBadges, blockScreenshots,
		blockFeatures, blockBenchmarks, blockQuickStart, blockRequirements,
		blockArtifacts, blockInstallation, blockUsage, blockConfiguration, blockBuilding,
		blockDocumentation, blockFAQ, blockRoadmap,
		blockPolicies, blockLinks, blockFunding, blockLicense,
	}, defaultBlocks)
}

// minimalDoc builds a projectfile Document with just enough identity + license
// for the basics/license blocks to render. Tests add Extensions as needed.
func minimalDoc(t *testing.T) *projectfile.Document {
	t.Helper()
	title := projectfile.LocalizedString{Bare: "demo"}
	summary := projectfile.LocalizedString{Bare: "a demo project"}
	return &projectfile.Document{
		Identity: projectfile.Identity{
			Name:    "demo",
			Title:   &title,
			Summary: &summary,
		},
		License: &projectfile.License{Spdx: "MIT"},
	}
}

// modeWrite is the core.Options.Mode value for the write direction. Declared
// once so the renderDoc/renderMulti helpers and goconst agree.
const modeWrite = "write"

// readmeNS is the org.projectfile.readme namespace key tests set on the
// document's Extensions map. Pulled out so goconst sees one canonical home.
const readmeNS = "org.projectfile.readme"

// renderDoc renders a document into a single string (default language only),
// suitable for substring assertions on the output.
func renderDoc(t *testing.T, dir string, pf *projectfile.Document) string {
	t.Helper()
	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	require.Len(t, out.Files, 1, "no languages configured → exactly README.md")
	return string(out.Files[filenameReadme])
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

// TestProbeFeaturesLinkPresent verifies the single-link probe populates the
// field when FEATURES.md exists. With no H3 feature titles it renders the
// heading + link with no bullets.
func TestProbeFeaturesLinkPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "FEATURES.md", "# Features\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "## Features")
	assert.Contains(t, body, "[Features](FEATURES.md)")
}

// TestProbeFeaturesLinkAbsent verifies the block is silently omitted when
// FEATURES.md is missing.
func TestProbeFeaturesLinkAbsent(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.NotContains(t, body, "## Features")
}

// TestFeaturesBlockListsHeadings verifies the features block extracts the H3
// feature titles from FEATURES.md into a bullet list between the heading and
// the FEATURES.md link. The structural H1/H2 are skipped.
func TestFeaturesBlockListsHeadings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "FEATURES.md",
		"# Features\n\n## Project features\n\n### Persistent APT cache\n\n### Non-root by default\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)

	assert.Contains(t, body, "## Features")
	assert.Contains(t, body, "- Persistent APT cache")
	assert.Contains(t, body, "- Non-root by default")
	assert.Contains(t, body, "[Features](FEATURES.md)")
	assert.NotContains(t, body, "- Features\n",
		"the H1 document title must not appear as a bullet")
	assert.NotContains(t, body, "- Project features",
		"the H2 section heading must not appear as a bullet")
}

// TestExtractFirstHeading pins the documentation link label source: the first
// ATX heading of any level, with empty results on no-heading and read error.
func TestExtractFirstHeading(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "preamble\n## Second Section\n")
	writeFile(t, dir, "b.md", "no heading here\n")
	assert.Equal(t, "Second Section", extractFirstHeading(dir, "a.md"))
	assert.Equal(t, "", extractFirstHeading(dir, "b.md"), "no heading → empty")
	assert.Equal(t, "", extractFirstHeading(dir, "missing.md"), "unreadable → empty")
}

// TestFeatureHeadingsFiltersH3 pins the FEATURES.md title extraction: only
// level-3 headings are returned, H1/H2 are skipped, and an absent file is nil.
func TestFeatureHeadingsFiltersH3(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "FEATURES.md", "# Features\n## Project features\n### One\n#### Nested\n### Two\n")
	assert.Equal(t, []string{"One", "Two"}, featureHeadings(dir))

	assert.Nil(t, featureHeadings(t.TempDir()), "absent FEATURES.md → nil")
}

// TestParseATXHeading pins the shared heading parser: level detection, the
// required space after the marker, and the 1–6 CommonMark bound.
func TestParseATXHeading(t *testing.T) {
	cases := []struct {
		line  string
		level int
		text  string
		ok    bool
	}{
		{"# Title", 1, "Title", true},
		{"###  Trimmed  ", 3, "Trimmed", true},
		{"###### Deep", 6, "Deep", true},
		{"####### Too many", 0, "", false},
		{"NoMarker", 0, "", false},
		{"#NoSpace", 0, "", false},
		{"", 0, "", false},
	}
	for _, c := range cases {
		level, text, ok := parseATXHeading(c.line)
		assert.Equalf(t, c.ok, ok, "ok mismatch for %q", c.line)
		assert.Equalf(t, c.level, level, "level mismatch for %q", c.line)
		assert.Equalf(t, c.text, text, "text mismatch for %q", c.line)
	}
}

// TestPoliciesLinksAreHumanReadable guards the user-visible contract: the
// policies block must use human-readable labels, not bare filenames.
func TestPoliciesLinksAreHumanReadable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileContributing, "# Contributing\n")
	writeFile(t, dir, fileSecurity, "# Security\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)

	assert.Contains(t, body, "[How to contribute]("+fileContributing+")")
	assert.Contains(t, body, "[Security policy]("+fileSecurity+")")
	assert.NotContains(t, body, "["+fileContributing+"]("+fileContributing+")",
		"policies must not use the bare filename as link text")
}

// TestPoliciesIncludesCodeOfConduct verifies CODE_OF_CONDUCT.md is now part of
// the policies probe (it was not before this change).
func TestPoliciesIncludesCodeOfConduct(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileCodeOfConduct, "# CoC\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "[Code of Conduct]("+fileCodeOfConduct+")")
}

// TestScreenshotsBlockFromDocsScreenshots verifies the screenshots block
// renders an inline image for every image in docs/screenshots.
func TestScreenshotsBlockFromDocsScreenshots(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/screenshots/demo.png", "binary-placeholder\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "## Screenshots")
	assert.Contains(t, body, "![demo.png](docs/screenshots/demo.png)")
}

// TestDocumentationBlockListsDocsMarkdown verifies the documentation block
// lists every docs/*.md file EXCEPT the excluded ones (the meta-doc
// readme-generator.md and the Makefile reference the building block owns),
// labelling each link with the file's first Markdown heading (falling back to
// the humanized filename when a doc carries no heading).
func TestDocumentationBlockListsDocsMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/MAKEFILE.md", "# Makefile Targets\n")
	writeFile(t, dir, "docs/readme-generator.md", "# meta\n")
	writeFile(t, dir, "docs/architecture.md", "# Architecture Overview\n")
	writeFile(t, dir, "docs/deploy.md", "no heading here\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)

	assert.Contains(t, body, "## Documentation")
	assert.NotContains(t, body, "[Makefile Targets](docs/MAKEFILE.md)",
		"the Makefile reference belongs to the building block, not here")
	assert.Contains(t, body, "[Architecture Overview](docs/architecture.md)")
	assert.Contains(t, body, "[Deploy](docs/deploy.md)",
		"a doc without a heading falls back to the humanized filename")
	assert.NotContains(t, body, "readme-generator",
		"the bridge's own meta-doc must not be advertised as user documentation")
}

// TestLogoBlockFromDocsLogo verifies the logo block renders when an image
// named logo.<ext> lives under docs/.
func TestLogoBlockFromDocsLogo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/logo.png", "binary-placeholder\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, `<img src="docs/logo.png"`)
}

// TestBuildingBlockIncludesMakefileDoc verifies the building block lists
// docs/MAKEFILE.md when present (BUILD.md is absent here).
func TestBuildingBlockIncludesMakefileDoc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/MAKEFILE.md", "# Makefile\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "## Building")
	assert.Contains(t, body, "[Makefile reference](docs/MAKEFILE.md)")
}

// TestBuildingBlockIncludesBuildDoc verifies the building block lists BUILD.md
// when present at the root.
func TestBuildingBlockIncludesBuildDoc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "BUILD.md", "# Build\n")
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "[How to build](BUILD.md)")
}

// TestRenderSkipsAbsentBlocks verifies that an empty repo yields only basics +
// license — none of the new optional sections leak through as empty headings.
func TestRenderSkipsAbsentBlocks(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)

	assert.Contains(t, body, "# demo")
	assert.Contains(t, body, "## License")
	for _, heading := range []string{
		"## Features", "## Benchmarks", "## Quick Start", "## Requirements",
		"## Installation", "## Usage", "## Building", "## Documentation",
		"## FAQ", "## Policies", "## Funding", "## Screenshots",
	} {
		assert.NotContains(t, body, heading, "absent data must drop the block, not its heading")
	}
}

// TestRenderIncludesAllPresentBlocks is the positive counterpart: with a full
// fixture, every block emits its heading.
func TestRenderIncludesAllPresentBlocks(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{
		fileFeatures, fileBenchmarks, fileQuickStart, fileRequirements,
		fileInstall, fileUsage, buildDocFile, fileFAQ, fileFunding,
		fileContributing, fileSecurity, fileSupport, fileCodeOfConduct,
	} {
		writeFile(t, dir, f, "# "+f+"\n")
	}
	writeFile(t, dir, "docs/MAKEFILE.md", "# Makefile\n")
	writeFile(t, dir, "docs/architecture.md", "# Architecture\n")
	writeFile(t, dir, "docs/screenshots/x.png", "x\n")
	writeFile(t, dir, "docs/logo.png", "logo\n")

	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)

	for _, heading := range []string{
		"## Features", "## Benchmarks", "## Quick Start", "## Requirements",
		"## Installation", "## Usage", "## Building", "## Documentation",
		"## FAQ", "## Policies", "## Funding", "## Screenshots",
	} {
		assert.Contains(t, body, heading, "present data must render the block heading")
	}
}

// keyLanguages is the languages key inside org.projectfile.i18n. Only the
// fixtures need the literal now that pfmodel owns the production lookup.
const keyLanguages = "languages"

// TestMultiLangEmitsOneFilePerLanguage verifies the languages config drives
// the multi-file output: README.md (default) plus one file per declared lang.
func TestMultiLangEmitsOneFilePerLanguage(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{
			keyLanguages: []any{"es", "uk"},
		},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"README.md", "docs/es/README.md", "docs/uk/README.md"},
		keys(out.Files))
}

// summaryEN/ES/UK are the per-language summary fixtures used by the
// multi-lang render test. Declared as constants so the map value and the
// assertions share one canonical literal (goconst-clean).
const (
	summaryEN = "Hello"
	summaryES = "Hola"
	summaryUK = "Привіт"
)

// TestMultiLangResolvesSummaryPerLanguage verifies identity.summary is
// resolved per language into each variant.
func TestMultiLangResolvesSummaryPerLanguage(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Identity.Summary = &projectfile.LocalizedString{
		Langs: map[string]string{"en": summaryEN, "es": summaryES, "uk": summaryUK},
	}
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{
			keyLanguages: []any{"es", "uk"},
		},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)

	assert.Contains(t, string(out.Files["README.md"]), summaryEN)
	assert.Contains(t, string(out.Files["docs/es/README.md"]), summaryES)
	assert.Contains(t, string(out.Files["docs/uk/README.md"]), summaryUK)
}

// TestMultiLangCrossLinks verifies the cross-language bar appears at the top
// of each variant, lists every OTHER language, and never links to itself.
func TestMultiLangCrossLinks(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{
			keyLanguages: []any{"es", "uk"},
		},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)

	en := string(out.Files["README.md"])
	es := string(out.Files["docs/es/README.md"])
	uk := string(out.Files["docs/uk/README.md"])

	// The default variant links to es and uk under docs/<lang>/, labelled with
	// each language's endonym, and never to itself.
	assert.Contains(t, en, "[Español](docs/es/README.md)")
	assert.Contains(t, en, "[Українська](docs/uk/README.md)")
	assert.NotContains(t, en, "](README.md)", "default variant must not link to itself")

	// es variant links to the canonical (English) root + uk, never to itself.
	assert.Contains(t, es, "[English](README.md)")
	assert.Contains(t, es, "[Українська](docs/uk/README.md)")
	assert.NotContains(t, es, "](docs/es/README.md)", "es variant must not link to itself")

	// uk variant links to the canonical (English) root + es, never to itself.
	assert.Contains(t, uk, "[English](README.md)")
	assert.Contains(t, uk, "[Español](docs/es/README.md)")
	assert.NotContains(t, uk, "](docs/uk/README.md)", "uk variant must not link to itself")
}

// TestSingleLangHasNoLanguagesBlock verifies that without a languages config,
// only README.md is written and no cross-link bar appears.
func TestSingleLangHasNoLanguagesBlock(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)
	assert.Len(t, out.Files, 1)
	assert.Contains(t, out.Files, filenameReadme)

	body := string(out.Files[filenameReadme])
	// The cross-link bar would render `[EN](README.md)` or similar — none of
	// the language codes should appear as a standalone badge near the top.
	assert.NotContains(t, body, "[EN](README.md)")
}

// TestReadmeLanguagesReadsSharedNamespace verifies README variants are driven
// by the document-wide org.projectfile.i18n list — the same list every
// community health file honours — not a README-local one.
func TestReadmeLanguagesReadsSharedNamespace(t *testing.T) {
	pf := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.I18NExtensionNS: map[string]any{
				keyLanguages: []any{"es", "uk"},
			},
		},
	}
	assert.Equal(t, []string{"es", "uk"}, pfmodel.Languages(pf))
}

// keyDefaultLanguage is the default-language key inside org.projectfile.i18n.
const keyDefaultLanguage = "default-language"

// TestDefaultLanguageReanchorsRootRender verifies that a non-English default
// language makes the root README resolve strings in that language and routes
// English under docs/en/ — the Spanish-first project case.
func TestDefaultLanguageReanchorsRootRender(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Identity.Summary = &projectfile.LocalizedString{
		Langs: map[string]string{"en": summaryEN, "es": summaryES},
	}
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{
			keyDefaultLanguage: "es",
			keyLanguages:       []any{"en"},
		},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)

	// The root README is Spanish (the default language); English moved under
	// docs/en/. The default language is never also a variant, so only two
	// files render.
	assert.ElementsMatch(t, []string{"README.md", "docs/en/README.md"}, keys(out.Files))

	// Root resolves the Spanish summary; the canonical "## License" heading
	// is Spanish too, proving the catalog resolved in the default language.
	root := string(out.Files["README.md"])
	assert.Contains(t, root, summaryES, "root README resolves the default language")
	assert.Contains(t, root, "## Licencia", "root README catalog text is Spanish")
	assert.NotContains(t, root, summaryEN, "root README must not fall through to English")

	// The English copy lives under docs/en/ and carries the cross-language bar
	// back to the Spanish root.
	en := string(out.Files["docs/en/README.md"])
	assert.Contains(t, en, summaryEN)
	assert.Contains(t, en, "[Español](README.md)", "English variant links the Spanish root")
}

// TestReadmeLanguagesAbsentReturnsNil verifies the absence path so a single
// default render is the result.
func TestReadmeLanguagesAbsentReturnsNil(t *testing.T) {
	assert.Nil(t, pfmodel.Languages(minimalDoc(t)))
	assert.Nil(t, pfmodel.Languages(nil))
}

// TestHumanizeFilename pins the doc-filename → label humanization.
func TestHumanizeFilename(t *testing.T) {
	assert.Equal(t, "Makefile", humanizeFilename("MAKEFILE.md"))
	assert.Equal(t, "Api Guide", humanizeFilename("api-guide.md"))
	assert.Equal(t, "Release Notes", humanizeFilename("release_notes.md"))
}

// keys returns the sorted keys of a map (test helper for ElementsMatch).
func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// extrasDefaultContent is the per-test fixture for the default extras content.
// Declared as a constant so goconst sees one canonical home for the literal
// shared with the matching ContentByLang entry below.
const extrasDefaultContent = "Hello"

// TestExtraContentForLangResolution verifies extras content is resolved
// per-language when ContentByLang is set, falling back to Content otherwise.
func TestExtraContentForLangResolution(t *testing.T) {
	// With a lang map, the requested lang wins; missing lang falls back.
	multi := pfmodel.ReadmeExtra{
		Name:          "notice",
		Content:       extrasDefaultContent,
		ContentByLang: map[string]string{"en": extrasDefaultContent, "es": "Hola"},
	}
	assert.Equal(t, "Hola", extraContentForLang(multi, "es"))
	assert.Equal(t, extrasDefaultContent, extraContentForLang(multi, "en"))
	assert.Equal(t, extrasDefaultContent, extraContentForLang(multi, "fr"),
		"unknown lang falls back to default Content")

	// Bare string (no lang map) always yields Content regardless of lang.
	bare := pfmodel.ReadmeExtra{Name: "notice", Content: "Just a string"}
	assert.Equal(t, "Just a string", extraContentForLang(bare, "es"))
}

// TestRenderUsesExtraContentPerLang is an end-to-end check that an extras
// block with a lang map renders the right variant in each README file.
func TestRenderUsesExtraContentPerLang(t *testing.T) {
	dir := t.TempDir()
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		pfmodel.I18NExtensionNS: map[string]any{keyLanguages: []any{"es"}},
		readmeNS: map[string]any{
			"extras": []any{
				map[string]any{
					"name": "special-psa",
					"content": map[string]any{
						"en": "Important notice in English",
						"es": "Aviso importante en español",
					},
				},
			},
			keyBlocks: []any{blockLanguages, "special-psa", blockLicense},
		},
	}

	out, err := Bridge{}.Render(pf, core.Options{Dir: dir, Mode: modeWrite, Force: true})
	require.NoError(t, err)

	enBody := string(out.Files["README.md"])
	esBody := string(out.Files["docs/es/README.md"])

	assert.Contains(t, enBody, "Important notice in English")
	assert.NotContains(t, enBody, "Aviso importante")
	assert.Contains(t, esBody, "Aviso importante en español")
	assert.NotContains(t, esBody, "Important notice in English")

	// Both files still carry the license section — each with its own heading.
	assert.Contains(t, enBody, "## License")
	assert.Contains(t, esBody, "## Licencia")
}

// TestRenderLocalOverrideWins is a regression guard for the 4-tier resolver:
// a project-local template still wins over the embedded one. Uses logo as
// the canary since the embedded version is non-trivial now.
func TestRenderLocalOverrideWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docs/logo.png", "x\n")
	// Project-local override that emits a marker line.
	overrideDir := filepath.Join(dir, core.LocalTemplatesDir, "readme.md")
	require.NoError(t, os.MkdirAll(overrideDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(overrideDir, "logo.tmpl"),
		[]byte("LOCAL-LOGO-OVERRIDE\n"),
		0o644,
	))

	pf := minimalDoc(t)
	body := renderDoc(t, dir, pf)
	assert.Contains(t, body, "LOCAL-LOGO-OVERRIDE")
	assert.NotContains(t, body, `<img src="docs/logo.png"`,
		"local override must replace the embedded template entirely")
}

// TestCollapseBlankLines is a small guard for the whitespace normalizer so
// future regex tweaks keep the user-visible blank-line contract.
func TestCollapseBlankLines(t *testing.T) {
	in := []byte("a\n\n\n\nb\n\n\n")
	out := core.CollapseBlankLines(in)
	assert.Equal(t, "a\n\nb\n", string(out))
	assert.False(t, strings.HasSuffix(string(out), "\n\n"),
		"trailing blanks must collapse to a single newline")
}
