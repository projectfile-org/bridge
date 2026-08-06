// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

const testValue = "Value"

// TestRenderLocalOverride verifies that a template in
// .projectfile/templates/ takes precedence over the embedded one.
func TestRenderLocalOverride(t *testing.T) {
	const tmplName = "TEST_OVERRIDE.tmpl"

	// Register a "built-in" template that produces "embedded".
	core.RegisterTemplates(tmplName, func(string) ([]byte, error) {
		return []byte("embedded: {{.Value}}"), nil
	})

	t.Run("uses embedded when no local override", func(t *testing.T) {
		dir := t.TempDir()
		out, err := core.Render(dir, tmplName, map[string]string{testValue: "yes"})
		require.NoError(t, err)
		assert.Equal(t, "embedded: yes", string(out))
	})

	t.Run("uses local override when present", func(t *testing.T) {
		dir := t.TempDir()
		localDir := filepath.Join(dir, core.LocalTemplatesDir)
		require.NoError(t, os.MkdirAll(localDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(localDir, tmplName),
			[]byte("local: {{.Value}}"),
			0o644,
		))

		out, err := core.Render(dir, tmplName, map[string]string{testValue: "overridden"})
		require.NoError(t, err)
		assert.Equal(t, "local: overridden", string(out))
	})

	t.Run("falls back when local file is missing", func(t *testing.T) {
		dir := t.TempDir()
		localDir := filepath.Join(dir, core.LocalTemplatesDir)
		require.NoError(t, os.MkdirAll(localDir, 0o755))

		out, err := core.Render(dir, tmplName, map[string]string{testValue: "fallback"})
		require.NoError(t, err)
		assert.Equal(t, "embedded: fallback", string(out))
	})

	t.Run("empty dir skips local lookup", func(t *testing.T) {
		out, err := core.Render("", tmplName, map[string]string{testValue: "nodir"})
		require.NoError(t, err)
		assert.Equal(t, "embedded: nodir", string(out))
	})
}

// TestRenderLocalOverrideCaching verifies the local template is cached
// per (dir, name) and that different dirs get independent caches.
func TestRenderLocalOverrideCaching(t *testing.T) {
	const tmplName = "TEST_CACHE.tmpl"

	core.RegisterTemplates(tmplName, func(string) ([]byte, error) {
		return []byte("embedded: {{.V}}"), nil
	})

	dir1 := t.TempDir()
	localDir1 := filepath.Join(dir1, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(localDir1, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localDir1, tmplName),
		[]byte("dir1: {{.V}}"),
		0o644,
	))

	dir2 := t.TempDir()
	localDir2 := filepath.Join(dir2, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(localDir2, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(localDir2, tmplName),
		[]byte("dir2: {{.V}}"),
		0o644,
	))

	out1, err := core.Render(dir1, tmplName, map[string]string{"V": "x"})
	require.NoError(t, err)
	assert.Equal(t, "dir1: x", string(out1))

	out2, err := core.Render(dir2, tmplName, map[string]string{"V": "y"})
	require.NoError(t, err)
	assert.Equal(t, "dir2: y", string(out2))
}
