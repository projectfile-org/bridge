// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package stack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/scanners/stack"
)

func touch(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// ── Scan ─────────────────────────────────────────────────────────────────────

func TestScanGoMod(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.Contains(t, tags, "go")
}

func TestScanPackageJSON(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "package.json"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.Contains(t, tags, "node")
}

func TestScanPythonRequirements(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "requirements.txt"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.Contains(t, tags, "python")
}

func TestScanMultipleStacks(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	touch(t, filepath.Join(dir, "package.json"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.Contains(t, tags, "go")
	assert.Contains(t, tags, "node")
}

func TestScanEmptyDirNoTags(t *testing.T) {
	dir := t.TempDir()
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, tags, "empty directory must produce no stack tags")
}

func TestScanTagsAreSorted(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	touch(t, filepath.Join(dir, "package.json"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	for i := 1; i < len(tags); i++ {
		assert.LessOrEqual(t, tags[i-1], tags[i], "tags must be sorted alphabetically")
	}
}

func TestScanNodeModulesIgnored(t *testing.T) {
	dir := t.TempDir()
	// A Cargo.toml inside node_modules must not trigger a "rust" tag.
	touch(t, filepath.Join(dir, "node_modules", "somemod", "Cargo.toml"))
	tags, _, err := stack.Scan(dir)
	require.NoError(t, err)
	assert.NotContains(t, tags, "rust", "Cargo.toml inside node_modules must be ignored")
}

func TestScanHitsRecordRulePath(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "go.mod"))
	_, hits, err := stack.Scan(dir)
	require.NoError(t, err)
	require.NotEmpty(t, hits, "at least one hit must be recorded for go.mod")
	for _, h := range hits {
		assert.NotEmpty(t, h.Rule, "each hit must have a non-empty Rule")
	}
}

// ── DetectableTags ────────────────────────────────────────────────────────────

func TestDetectableTagsNonEmpty(t *testing.T) {
	tags := stack.DetectableTags()
	assert.NotEmpty(t, tags, "DetectableTags must return the known tag vocabulary")
	assert.Contains(t, tags, "go")
	assert.Contains(t, tags, "node")
	assert.Contains(t, tags, "python")
}

// ── IsIgnored ─────────────────────────────────────────────────────────────────

func TestIsIgnored(t *testing.T) {
	assert.True(t, stack.IsIgnored("node_modules"))
	assert.True(t, stack.IsIgnored(".git"))
	assert.True(t, stack.IsIgnored("vendor"))
	assert.False(t, stack.IsIgnored("src"))
	assert.False(t, stack.IsIgnored("internal"))
}
