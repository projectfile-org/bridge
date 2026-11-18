// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

const (
	readmeMD   = "README.md"
	howToAlpha = "docs/how-to/alpha.md"
	langES     = "es"
	keyTitle   = "title"
	keyPages   = "pages"
)

// write creates dir/rel holding body, parents included.
func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
}

// fixture builds a project tree, a rendered collections tree and the document declaring both.
func fixture(t *testing.T) (Input, *projectfile.Document) {
	t.Helper()
	in := Input{Dir: t.TempDir(), Rendered: t.TempDir()}
	for rel, body := range map[string]string{
		readmeMD:                  "# Tool\n",
		"docs/es/README.md":       "# Herramienta\n",
		"docs/FEATURES.md":        "<!-- SPDX -->\n# Features\n",
		"docs/es/FEATURES.md":     "# Características\n",
		howToAlpha:                "# Alpha guide\n",
		"docs/how-to/beta.md":     "no heading\n",
		"docs/how-to/MAKEFILE.md": "# Makefile\n",
		"docs/how-to/image.png":   "png",
		"docs/USAGE.md":           "# Usage\n",
		"CONTRIBUTING.md":         "# Contributing\n",
	} {
		write(t, in.Dir, rel, body)
	}
	for rel, body := range map[string]string{
		"rules/index.md":   "# All rules\n",
		"rules/a-rule.md":  "# A [rule]\n",
		"rules/set/b.md":   "# B rule\n",
		"rules/notes.txt":  "skipped",
		"other/index.md":   "# Undeclared\n",
		"rules/set/c d.md": "# Spaced\n",
	} {
		write(t, in.Rendered, rel, body)
	}
	pf := &projectfile.Document{}
	pf.Identity.Title = &projectfile.LocalizedString{Langs: map[string]string{"en": "Tool", langES: "Herramienta"}}
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{"languages": []any{langES}})
	projectfile.SetExtension(pf, pfmodel.PagesExtensionNS, map[string]any{
		"rules":   map[string]any{keyTitle: map[string]any{"en": "Rules", langES: "Reglas"}},
		"presets": map[string]any{keyTitle: "Presets"},
	})
	projectfile.SetExtension(pf, pfmodel.ConventionsExtensionNS, map[string]any{"book": map[string]any{"parts": []any{
		map[string]any{keyTitle: map[string]any{"en": "Features", langES: "Funciones"}, keyPages: []any{"docs/FEATURES.md"}},
		map[string]any{keyTitle: "Guides", keyPages: []any{"docs/how-to/*.md"}},
		map[string]any{keyTitle: "Roadmap", keyPages: []any{"docs/ROADMAP.md"}},
		map[string]any{keyTitle: "Reference", keyPages: []any{"docs/USAGE.md", "docs/CONFIGURATION.md"}, "collections": true},
		map[string]any{keyTitle: "Development", keyPages: []any{"docs/develop/*.md", "docs/how-to/MAKEFILE.md"}},
		map[string]any{keyTitle: "Project", keyPages: []any{"CONTRIBUTING.md", readmeMD, howToAlpha}},
	}}})
	return in, pf
}

// TestSummaryOfTheDefaultLanguage pins the default SUMMARY.md: part order, drops, a named page claimed out of a glob, the collection tree.
func TestSummaryOfTheDefaultLanguage(t *testing.T) {
	in, pf := fixture(t)
	b, err := Plan(in, pf, "")
	require.NoError(t, err)
	assert.Equal(t, "en", b.Lang)
	assert.Equal(t, "Tool", b.Title)
	assert.Equal(t, `# Summary

[Tool](README.md)

# Features

- [Features](docs/FEATURES.md)

# Guides

- [beta](docs/how-to/beta.md)

# Reference

- [Usage](docs/USAGE.md)
- [Rules](pages/rules/index.md)
    - [A \[rule\]](pages/rules/a-rule.md)
    - [set]()
        - [B rule](pages/rules/set/b.md)
        - [Spaced](<pages/rules/set/c d.md>)

# Development

- [Makefile](docs/how-to/MAKEFILE.md)

# Project

- [Contributing](CONTRIBUTING.md)
- [Alpha guide](docs/how-to/alpha.md)
`, string(Summary(b)))
}

// TestTranslatedBookUsesCanonicalPaths pins the language book: every page at its canonical path, read from its docs/<lang>/ variant or from the canonical page as a fallback.
func TestTranslatedBookUsesCanonicalPaths(t *testing.T) {
	in, pf := fixture(t)
	b, err := Plan(in, pf, langES)
	require.NoError(t, err)
	assert.Equal(t, "Herramienta", b.Title)
	require.NotNil(t, b.Intro)
	assert.Equal(t, readmeMD, b.Intro.Path)
	assert.Equal(t, "docs/es/README.md", b.Intro.Origin)
	assert.Equal(t, filepath.Join(in.Dir, "docs", langES, readmeMD), b.Intro.Source)
	assert.False(t, b.Intro.Fallback)
	assert.Equal(t, "Funciones", b.Parts[0].Title)
	assert.Equal(t, "Características", b.Parts[0].Chapters[0].Title)
	guide := b.Parts[1].Chapters[0]
	assert.Equal(t, "docs/how-to/beta.md", guide.Path)
	assert.Equal(t, "docs/how-to/beta.md", guide.Origin)
	assert.Equal(t, filepath.Join(in.Dir, "docs", "how-to", "beta.md"), guide.Source)
	assert.True(t, guide.Fallback, "an untranslated page stands in for its translation")
	assert.Equal(t, "Reglas", b.Parts[2].Chapters[1].Title)
	assert.Empty(t, b.Parts[2].Chapters[1].Origin, "a rendered page has no project source")
}

// TestBookWithoutPartsIsItsIntroduction pins the smallest book: a README and no conventions.
func TestBookWithoutPartsIsItsIntroduction(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, readmeMD, "# Solo\n")
	b, err := Plan(Input{Dir: dir}, &projectfile.Document{}, "")
	require.NoError(t, err)
	assert.Equal(t, "# Summary\n\n[Solo](README.md)\n", string(Summary(b)))
}

// TestPagesOutsideTheProjectAreSkipped pins the traversal edge: an existing file one level above the project never enters the book.
func TestPagesOutsideTheProjectAreSkipped(t *testing.T) {
	parent := t.TempDir()
	write(t, parent, "secret.md", "# Secret\n")
	dir := filepath.Join(parent, "project")
	write(t, dir, readmeMD, "# Solo\n")
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.ConventionsExtensionNS, map[string]any{"book": map[string]any{"parts": []any{
		map[string]any{keyTitle: "Leak", keyPages: []any{"../secret.md", "docs/../../secret.md"}},
	}}})
	b, err := Plan(Input{Dir: dir}, pf, "")
	require.NoError(t, err)
	assert.Empty(t, b.Parts)
}

// TestPlanRejectsMalformedPages pins that a broken collection declaration fails the plan instead of dropping the part.
func TestPlanRejectsMalformedPages(t *testing.T) {
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.PagesExtensionNS, map[string]any{"rules": "Rules"})
	_, err := Plan(Input{Dir: t.TempDir()}, pf, "")
	assert.Error(t, err)
}

// TestFallbackNoteFallsBackToEnglish pins the note for a known language and for one without a string.
func TestFallbackNoteFallsBackToEnglish(t *testing.T) {
	assert.Equal(t, "> Esta página aún no está traducida.\n\n", FallbackNote(langES))
	assert.Equal(t, "> This page is not translated yet.\n\n", FallbackNote("pt-BR"))
}

// TestTomlNamesTheBookAndItsSources pins book.toml: title, language, source root and repository link.
func TestTomlNamesTheBookAndItsSources(t *testing.T) {
	pf := &projectfile.Document{Repositories: []projectfile.Repository{{URL: "https://kiota.ch/o/tool"}}}
	got, err := Toml(Book{Lang: langES, Title: `Herra "mienta"`}, pf, "src")
	require.NoError(t, err)
	assert.Equal(t, `[book]
title = 'Herra "mienta"'
language = 'es'
src = 'src'

[output]
[output.html]
git-repository-url = 'https://kiota.ch/o/tool'
`, string(got))
}
