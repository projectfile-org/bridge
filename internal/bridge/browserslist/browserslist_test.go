// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package browserslist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/browserslist"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

func docWithBrowsers(browsers any) *projectfile.Document {
	return &projectfile.Document{
		Identity:     projectfile.Identity{Name: "test-proj"},
		Requirements: &projectfile.Requirements{Browsers: browsers},
	}
}

// Sequence form: one query line per entry, marker on top.
func TestRenderSequence(t *testing.T) {
	b := browserslist.Bridge{}
	pf := docWithBrowsers([]string{"last 2 versions", "> 0.5%", "not dead"})
	out, err := b.Render(pf, core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	body := string(out.Files[".browserslistrc"])
	assert.Contains(t, body, core.Marker)
	assert.Contains(t, body, "last 2 versions\n")
	assert.Contains(t, body, "> 0.5%\n")
	assert.Contains(t, body, "not dead\n")
}

// Single-string form must round-trip into a one-line body.
func TestRenderSingleString(t *testing.T) {
	b := browserslist.Bridge{}
	pf := docWithBrowsers("defaults")
	out, err := b.Render(pf, core.Options{})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[".browserslistrc"]), "defaults\n")
}

// Empty / absent list is "unconstrained" (spec §4.8): no file, no error — the
// bridge is default-on in projects that legitimately target no browser.
func TestRenderEmptyIsNoOp(t *testing.T) {
	b := browserslist.Bridge{}
	for _, pf := range []*projectfile.Document{
		docWithBrowsers(nil),
		docWithBrowsers([]string{}),
		{Identity: projectfile.Identity{Name: "x"}}, // no Requirements at all
	} {
		out, err := b.Render(pf, core.Options{})
		require.NoError(t, err, "absent browsers must not fail the run")
		assert.Empty(t, out.Files, "absent browsers must emit no file")
	}
}

// The per-environment mapping form is valid spec we do not render yet — it must
// stay loud rather than degrade into the silent no-op above.
func TestRenderEnvMapErrors(t *testing.T) {
	b := browserslist.Bridge{}
	_, err := b.Render(docWithBrowsers(map[string]any{"production": []string{"defaults"}}), core.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "per-environment mapping")
}

// TestBridgePolicyMarker pins the marker policy the pure-data .gitignore family shares.
func TestBridgePolicyMarker(t *testing.T) {
	assert.True(t, browserslist.Bridge{}.Policy().Marker, "a pure-data file must be marker-managed")
}

// TestBridgeIdentitySurface pins the registry-facing identity argv dispatch reads.
func TestBridgeIdentitySurface(t *testing.T) {
	b := browserslist.Bridge{}
	assert.Equal(t, "browserslist", b.Name())
	label, kind := b.Labels()
	assert.Equal(t, b.Filename(), label, "the label is the managed filename")
	assert.Equal(t, "projectfile", kind)
}

// TestBridgeStacks pins the node-stack ecosystems the default-on bridge claims.
func TestBridgeStacks(t *testing.T) {
	assert.Equal(t, []string{"node", "javascript", "typescript"}, browserslist.Bridge{}.Stacks())
}

// TestBridgePathAndExists drives Exists across the absent and the written file.
func TestBridgePathAndExists(t *testing.T) {
	b := browserslist.Bridge{}
	dir := t.TempDir()
	filename := b.Filename()
	assert.False(t, b.Exists(dir), "an empty directory holds no config")
	assert.Equal(t, filepath.Join(dir, filename), b.FullPath(dir, nil))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte("last 1 version\n"), 0o644))
	assert.True(t, b.Exists(dir), "the written file must be detected")
}
