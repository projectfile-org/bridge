// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// fixtureMimicsM11s is the m11s root layout in miniature: pyproject carries requires-python while the projectfile models no python floor.
const fixtureMimicsM11s = "[project]\nname = 'm11s'\nversion = '0.1.0'\nrequires-python = '>=3.14'\n"

func writePyproject(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(FullPath(dir), []byte(content), 0o644))
	return dir
}

func readPyproject(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(FullPath(dir))
	require.NoError(t, err)
	return string(data)
}

// TestSyncPreservesRequiresPythonByDefault is the issue scenario end to end: a default sync must not strip a requires-python the projectfile does not model.
func TestSyncPreservesRequiresPythonByDefault(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)
	assert.False(t, res.ExtChanged, "a default sync must not touch a file value pf lacks")
	assert.Contains(t, readPyproject(t, dir), "requires-python = '>=3.14'")
}

// TestWritePreservesRequiresPython: even an explicit `to` keeps an unmodeled file value; only an authority opt-in clears (next test).
func TestWritePreservesRequiresPython(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	assert.False(t, res.ExtChanged, "no-destroy rule holds on an explicit push too")
	assert.Contains(t, readPyproject(t, dir), "requires-python = '>=3.14'")
}

// TestWriteAuthorityProjectfileRestoresClear proves the escape hatch on the explicit path: authority plus `to` propagates the removal.
func TestWriteAuthorityProjectfileRestoresClear(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, testAuthorityNS, map[string]any{testAuthorityKey: map[string]any{testRequiresPythonField: testProjectfileOwner}})

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	assert.True(t, res.ExtChanged)
	got := readPyproject(t, dir)
	assert.NotContains(t, got, "requires-python")
	assert.True(t, strings.Contains(got, "name = 'm11s'"), "the push must keep what pf models:\n%s", got)
}

// TestSyncAuthorityProjectfileRestoresClear opts one field back into the old behaviour without changing the default for everything else.
func TestSyncAuthorityProjectfileRestoresClear(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, testAuthorityNS, map[string]any{testAuthorityKey: map[string]any{testRequiresPythonField: testProjectfileOwner}})

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)
	assert.True(t, res.ExtChanged)
	assert.NotContains(t, readPyproject(t, dir), "requires-python")
}
