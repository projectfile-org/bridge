// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/fragments"
)

// TestBridgeIdentitySurface pins the contract the dispatcher reads: the one
// bridge registers under a filename that names no single output, reports no
// canonical path (RunRender creates files as the documents declare them), and
// carries an empty policy so a generated artefact is always overwritten.
func TestBridgeIdentitySurface(t *testing.T) {
	b := fragments.Bridge{}
	assert.Equal(t, keyFragments, b.Name())
	assert.Equal(t, keyFragments, b.Filename())
	assert.Empty(t, b.Aliases())
	label, family := b.Labels()
	assert.Equal(t, keyFragments, label)
	assert.Equal(t, "projectfile", family)
	assert.Contains(t, b.Describe(), outFeatures)
	assert.Equal(t, core.Policy{}, b.Policy())
	assert.False(t, b.Exists(filepath.Join("dir", outFeatures)))
	assert.Equal(t, filepath.Join("dir", keyFragments), b.FullPath("dir", nil))
}

// TestRenderFailsOnScalarNamespace verifies a fragments namespace that is not a
// map fails the render naming the document set, rather than reading as an
// absent namespace and silently rendering nothing.
func TestRenderFailsOnScalarNamespace(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    "+keyFragments+": a-scalar-not-a-map")

	_, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "map")
}

// TestRenderFailsOnUnreadableFragment covers the per-document load wrap: one
// fragment entry the reader cannot open fails the whole document, named, so a
// partial file never lands on disk.
func TestRenderFailsOnUnreadableFragment(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	// A .md entry that is a directory matches the glob but cannot be read.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs", "features.d", "blocked.md"), 0o755))

	_, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), outFeatures)
}

// brokenTemplate is a project-local override whose parse fails, so the
// two-tier renderer surfaces the error through the bridge's own wrap.
const brokenTemplate = "{{ .Title "

// variantBlindTemplate renders fine for a view with a project section and
// fails only for one without — the shape that reaches the variant's own
// assembly error wrap.
const variantBlindTemplate = "{{- .REUSEHeader -}}\n# {{.Title}}\n{{ if not .HasProject }}{{ .NoSuchField }}{{ end }}\n"

// writeLocalTemplate installs one project-local template override.
func writeLocalTemplate(t *testing.T, dir, body string) {
	t.Helper()
	full := filepath.Join(dir, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(full, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(full, "fragments.md.tmpl"), []byte(body), 0o644))
}

// TestRenderFailsOnBrokenLocalTemplate verifies a project-local override that
// does not parse fails the render instead of falling back silently to the
// embedded template.
func TestRenderFailsOnBrokenLocalTemplate(t *testing.T) {
	dir := t.TempDir()
	writeProjectfile(t, dir, "    fragments:\n      documents:\n        - {dir: docs/features.d, out: FEATURES.md, title: Features}")
	writeFragment(t, dir, "docs/features.d", "alpha", "Alpha", "Body.")
	writeLocalTemplate(t, dir, brokenTemplate)

	_, err := fragments.Bridge{}.Render(docWithFragments(t, dir), core.Options{Dir: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), outFeatures)
}

// TestRenderFailsOnVariantAssemblyError covers the localized half of the
// assembly wrap: the canonical file renders and the variant's own assembly
// fails, so the error names the localized path rather than the canonical one.
func TestRenderFailsOnVariantAssemblyError(t *testing.T) {
	child := t.TempDir()
	body := i18nBlock("es") +
		"    fragments:\n      documents:\n        - dir: docs/features.d\n" +
		"          out: FEATURES.md\n          title: Features\n" +
		parentsBlock(parentURL)
	writeProjectfile(t, child, body)
	writeFragment(t, child, "docs/features.d", "own", "Own Feature", "Own body.")
	// The variant ships no own fragments but a committed localized document,
	// so it renders — straight into the blind half of the template.
	writeCommittedDoc(t, child, outFeatures, "Features", "## Project Features",
		[]string{ownFeatureBody},
		[][2]string{{headingInherited, parentFeatBody}},
		false)
	writeCommittedDoc(t, child, "docs/es/"+outFeatures, "Características", "## Características del proyecto",
		nil,
		[][2]string{{"## Heredado de b19/ubuntu", parentFeatBody}},
		true)
	writeLocalTemplate(t, child, variantBlindTemplate)

	_, err := fragments.Bridge{}.Render(docWithFragments(t, child), offlineOptions(child))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docs/es/"+outFeatures)
	assert.True(t, strings.Contains(err.Error(), "NoSuchField"), "the variant's own assembly failed: %v", err)
}
