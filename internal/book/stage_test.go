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
)

// read returns dir/rel as a string, failing the test when it is absent.
func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(b)
}

// TestStageWritesEveryLanguage pins the staged tree: book.toml, SUMMARY.md, pages at their own paths, the note, assets and collections.
func TestStageWritesEveryLanguage(t *testing.T) {
	in, pf := fixture(t)
	write(t, in.Dir, readmeMD, "# Tool\n\n![shot](docs/shot.png) [up](../outside.png) [web](https://x.test/a.png) [guide](docs/how-to/beta.md#top)\n")
	write(t, in.Dir, "docs/shot.png", "png")
	rendered := t.TempDir()
	write(t, rendered, "en/rules/index.md", "# All rules\n")
	write(t, rendered, "es/rules/index.md", "# Todas las reglas\n")
	out := filepath.Join(t.TempDir(), "book")

	langs, err := Stage(in.Dir, rendered, out, pf)
	require.NoError(t, err)
	assert.Equal(t, []string{"en", langES}, langs)

	assert.Contains(t, read(t, out, "en/book.toml"), "language = 'en'")
	assert.Contains(t, read(t, out, "en/src/SUMMARY.md"), "[Tool](README.md)")
	assert.Equal(t, "png", read(t, out, "en/src/docs/shot.png"), "a linked local asset is copied at its path")
	assert.NoFileExists(t, filepath.Join(out, "en/src/docs/how-to/image.png"), "an unlinked file is not copied")
	assert.Equal(t, "# All rules\n", read(t, out, "en/src/pages/rules/index.md"))
	assert.Equal(t, "no heading\n", read(t, out, "en/src/docs/how-to/beta.md"), "the default book carries no note")

	assert.Contains(t, read(t, out, "es/src/SUMMARY.md"), "[Herramienta](docs/es/README.md)")
	assert.Equal(t, "# Herramienta\n", read(t, out, "es/src/docs/es/README.md"))
	assert.Equal(t, "> Esta página aún no está traducida.\n\nno heading\n", read(t, out, "es/src/docs/how-to/beta.md"))
	assert.Equal(t, "# Todas las reglas\n", read(t, out, "es/src/pages/rules/index.md"))
}

// TestStageReplacesThePreviousBook pins that a re-run drops a page the project no longer has.
func TestStageReplacesThePreviousBook(t *testing.T) {
	in, pf := fixture(t)
	out := t.TempDir()
	_, err := Stage(in.Dir, t.TempDir(), out, pf)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(in.Dir, "CONTRIBUTING.md")))
	_, err = Stage(in.Dir, t.TempDir(), out, pf)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(out, "en/src/CONTRIBUTING.md"))
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temporary directory is left behind")
}

// TestStageRefusesAForeignDirectory pins that a target without book.toml is never deleted.
func TestStageRefusesAForeignDirectory(t *testing.T) {
	in, pf := fixture(t)
	out := t.TempDir()
	write(t, out, "en/keep.txt", "mine")
	_, err := Stage(in.Dir, t.TempDir(), out, pf)
	require.ErrorContains(t, err, "is not a staged book")
	assert.Equal(t, "mine", read(t, out, "en/keep.txt"))
}

// TestStageAppendsExtras pins docs/<name>.d/<id>.md after the rendered page, the language variant first, and never on index.md.
func TestStageAppendsExtras(t *testing.T) {
	in, pf := fixture(t)
	rendered := t.TempDir()
	for _, lang := range []string{"en", langES} {
		write(t, rendered, lang+"/rules/index.md", "# Rules\n")
		write(t, rendered, lang+"/rules/web/one.md", "# One\n\n\n")
		write(t, rendered, lang+"/rules/two.md", "# Two\n")
		write(t, rendered, lang+"/rules/three.md", "# Three\n")
	}
	write(t, in.Dir, "docs/rules.d/three.md", "Canonical.\n")
	write(t, in.Dir, "docs/rules.d/web/one.md", "Example.\n")
	write(t, in.Dir, "docs/es/rules.d/web/one.md", "Ejemplo.\n")
	write(t, in.Dir, "docs/rules.d/index.md", "Never.\n")
	out := filepath.Join(t.TempDir(), "book")

	_, err := Stage(in.Dir, rendered, out, pf)
	require.NoError(t, err)

	assert.Equal(t, "# One\n\nExample.\n", read(t, out, "en/src/pages/rules/web/one.md"))
	assert.Equal(t, "# One\n\nEjemplo.\n", read(t, out, "es/src/pages/rules/web/one.md"))
	assert.Equal(t, "# Three\n\nCanonical.\n", read(t, out, "es/src/pages/rules/three.md"), "an untranslated extras page falls back to the canonical one")
	assert.Equal(t, "# Two\n", read(t, out, "en/src/pages/rules/two.md"), "a page without extras is unchanged")
	assert.Equal(t, "# Rules\n", read(t, out, "en/src/pages/rules/index.md"), "index.md names no item")
}
