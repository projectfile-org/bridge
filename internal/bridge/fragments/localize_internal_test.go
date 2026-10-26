// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// locProject writes a projectfile declaring one document (given as a YAML
// block) and the passed languages.
func locProject(t *testing.T, dir string, langs []string, document string) *projectfile.Document {
	t.Helper()
	body := "---\n" +
		"# SPDX-FileCopyrightText: 2026 Tester\n#\n# SPDX-License-Identifier: MIT\n" +
		"$schema: https://projectfile.org/schema/v1.json\n" +
		"identity:\n  name: " + filepath.Base(dir) + "\n" +
		"org:\n  projectfile:\n" +
		"    i18n:\n      languages:\n"
	for _, lang := range langs {
		body += "        - " + lang + "\n"
	}
	body += "    fragments:\n      documents:\n" + document
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(body), 0o644))
	pfPath, err := projectfile.DetectPath(dir)
	require.NoError(t, err)
	doc, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	return doc
}

// TestRenderWarnsOnUnreadableLocalizedFragmentDir covers the variant loader's
// only failure: a locale dir that cannot be read is warned and skipped, so one
// broken translation drops its own language and never the canonical file.
func TestRenderWarnsOnUnreadableLocalizedFragmentDir(t *testing.T) {
	dir := t.TempDir()
	doc := locProject(t, dir, []string{"es"},
		"        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	itestFrag(t, dir, "docs/es/features.d", "Característica Propia", "Cuerpo propio.")
	// A .md entry that is a directory matches the glob but cannot be read.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs/es/features.d", "blocked.md"), 0o755))

	var log strings.Builder
	genlog.SetOutput(&log)
	defer genlog.SetOutput(os.Stderr)

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, out.Files, outFeaturesName, "the canonical file survives")
	assert.NotContains(t, out.Files, "docs/es/FEATURES.md")
	assert.Contains(t, log.String(), "unreadable localized fragment dir")
	assert.Contains(t, log.String(), "docs/es/features.d")
}

// TestRenderLocalizesStemMissingFromCatalog verifies a document whose stem has
// no catalog entry keeps its declared title in every language — a half
// translated catalog degrades to the declaration, never to an empty heading.
func TestRenderLocalizesStemMissingFromCatalog(t *testing.T) {
	dir := t.TempDir()
	doc := locProject(t, dir, []string{"es"},
		"        - {dir: docs/changelog.d, out: CHANGELOG.md, title: Changelog}")
	itestFrag(t, dir, "docs/changelog.d", "Change One", "Body.")
	itestFrag(t, dir, "docs/es/changelog.d", "Cambio Uno", "Cuerpo.")

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["CHANGELOG.md"]), "# Changelog")
	assert.Contains(t, string(out.Files["docs/es/CHANGELOG.md"]), "# Changelog",
		"an untranslated stem falls back to the declared title")
}

// TestRenderLocalizesDotlessOutStem pins the stem derivation: a document
// declared without an extension still resolves its catalog title, which is
// what the title lookup is keyed on.
func TestRenderLocalizesDotlessOutStem(t *testing.T) {
	dir := t.TempDir()
	doc := locProject(t, dir, []string{"es"},
		"        - {dir: docs/usage.d, out: USAGE, title: Usage}")
	itestFrag(t, dir, "docs/usage.d", "Demo", "Help body.")
	itestFrag(t, dir, "docs/es/usage.d", "Demo ES", "Cuerpo de ayuda.")

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["USAGE"]), "# Usage")
	assert.Contains(t, string(out.Files["docs/es/USAGE"]), "# Uso")
}

// TestOnlineUnpublishedParentRendersNothingInherited: a parent that publishes
// no such document is an absence, so the document renders with its own
// fragments only and no parent is warned about.
func TestOnlineUnpublishedParentRendersNothingInherited(t *testing.T) {
	dir := t.TempDir()
	doc := locProject(t, dir, []string{"es"},
		"        - dir: docs/features.d\n          out: FEATURES.md\n          title: Features\n"+
			"          parents:\n            - {url: "+itestURL+"}\n")
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	itestFrag(t, dir, "docs/es/features.d", "Característica Propia", "Cuerpo propio.")
	itestStub(t, func(pfmodel.FragmentParent, string, []string) (inheritedCopy, map[string]inheritedCopy, error) {
		return inheritedCopy{}, nil, errNotPublished
	})

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[outFeaturesName]), "Inherited")
	assert.Contains(t, out.Files, "docs/es/FEATURES.md")
}
