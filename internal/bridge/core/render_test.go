// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
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

// Plural is the one template helper every bridge shares: 1 commit, 2 commits.
func TestPluralPicksTheNounByCount(t *testing.T) {
	assert.Equal(t, "1 commit", core.Plural(1, "commit", "commits"))
	assert.Equal(t, "0 commits", core.Plural(0, "commit", "commits"))
	assert.Equal(t, "3 commits", core.Plural(3, "commit", "commits"))
}

// An unregistered name is an error naming it, never a silent empty render.
func TestRenderUnregisteredTemplateErrors(t *testing.T) {
	_, err := core.Render("", "NOPE_MISSING.tmpl", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

// A template that fails executing reports which file failed.
func TestRenderExecErrorNamesTheTemplate(t *testing.T) {
	const name = "EXECFAIL.md.tmpl"
	core.RegisterTemplates(name, func(string) ([]byte, error) { return []byte("{{.Deep.Missing}}"), nil })

	_, err := core.Render("", name, map[string]string{"Deep": "flat"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), name)
}

// A project-local template is parsed on first use, so a syntax error surfaces from that render.
func TestRenderLocalParseErrorNamesTheTemplate(t *testing.T) {
	const name = "LOCALPARSE.md.tmpl"
	dir := t.TempDir()
	local := filepath.Join(dir, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(local, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(local, name), []byte("{{ broken"), 0o644))

	_, err := core.Render(dir, name, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), name)
}

// RegisterTemplateTree walks a bridge's embedded FS flat: *.tmpl only, no directories.
func TestRegisterTemplateTreeRegistersFlatFiles(t *testing.T) {
	root := t.TempDir()
	const names = "TREEA.md.tmpl"
	const nameB = "TREEB.md.tmpl"
	for name, body := range map[string]string{names: "a", nameB: "b"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("skip me"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "nested", "TREEC.md.tmpl"), []byte("c"), 0o644))

	core.RegisterTemplateTree(treeFS{root}, ".")

	out, err := core.Render("", "TREEA.md.tmpl", nil)
	require.NoError(t, err)
	assert.Equal(t, "a", string(out))

	_, err = core.Render("", "notes.txt", nil)
	assert.Error(t, err, "a non-template file never registers")
	_, err = core.Render("", "nested/TREEC.md.tmpl", nil)
	assert.Error(t, err, "nested directories are skipped")
}

// Re-registering a name panics: a cross-bridge collision fails at startup.
func TestRegisterTemplatesPanicsOnDuplicate(t *testing.T) {
	const name = "DUPLICATE.md.tmpl"
	core.RegisterTemplates(name, func(string) ([]byte, error) { return nil, nil })

	assert.Panics(t, func() {
		core.RegisterTemplates(name, func(string) ([]byte, error) { return nil, nil })
	})
}

// treeFS adapts a directory to the minimal FS RegisterTemplateTree reads.
type treeFS struct{ root string }

func (f treeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(filepath.Join(f.root, name))
}

func (f treeFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(f.root, name))
}
