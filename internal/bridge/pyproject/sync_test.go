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
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
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
// ExtChanged IS true here, and only for the privacy trove: identity.visibility is a MODELLED field whose absent value
// resolves to private (spec §4.3), so the marker is a convergence, not a value the projectfile failed to model.
func TestSyncPreservesRequiresPythonByDefault(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)
	assert.Equal(t, []core.FieldChange{{Key: "classifiers", Value: classifierPrivate, From: "projectfile", To: "pyproject.toml"}}, res.ExtFields)
	assert.Contains(t, readPyproject(t, dir), "requires-python = '>=3.14'")
}

// TestWritePreservesRequiresPython: even an explicit `to` keeps an unmodeled file value; only an authority opt-in clears (next test).
func TestWritePreservesRequiresPython(t *testing.T) {
	dir := writePyproject(t, fixtureMimicsM11s)
	pf := &projectfile.Document{}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	assert.Len(t, res.ExtFields, 1, "the privacy trove is the only write an undeclared visibility makes")
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

// TestVisibilityProjectsThePrivateTrove pins the three spec states end to end: private and undeclared write the
// PEP 753 marker, public drops it again even when it was hand-written.
func TestVisibilityProjectsThePrivateTrove(t *testing.T) {
	states := map[string]struct {
		declared string
		want     bool
	}{
		"public":     {declared: projectfile.VisibilityPublic, want: false},
		"private":    {declared: projectfile.VisibilityPrivate, want: true},
		"undeclared": {declared: "", want: true},
	}
	for name, tc := range states {
		t.Run(name, func(t *testing.T) {
			dir := writePyproject(t, "[project]\nname = 'demo'\nversion = '0.1.0'\nclassifiers = ['Programming Language :: Python :: 3', '"+classifierPrivate+"']\n")
			pf := &projectfile.Document{Identity: projectfile.Identity{Namespace: "com.acme", Name: "demo", Visibility: tc.declared}}

			_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
			require.NoError(t, err)
			assert.Equal(t, tc.want, strings.Contains(readPyproject(t, dir), classifierPrivate))
			assert.Contains(t, readPyproject(t, dir), "Programming Language :: Python :: 3", "the other classifiers survive")
		})
	}
}

// TestPrivateTroveTeachesTheDocument pins the discovery direction: only a guarded pyproject writes the field back.
func TestPrivateTroveTeachesTheDocument(t *testing.T) {
	teaches := func(t *testing.T, classifiers []string, declared string) string {
		t.Helper()
		py := &Document{Project: Project{Classifiers: classifiers}}
		pf := &projectfile.Document{Identity: projectfile.Identity{Visibility: declared}}
		desc := mapVisibility(py, pf).ToPF(false)
		if desc == "" {
			t.Log("mapper reported no change")
		}
		return pf.Identity.Visibility
	}
	assert.Equal(t, projectfile.VisibilityPrivate, teaches(t, []string{classifierPrivate}, ""))
	assert.Equal(t, "", teaches(t, []string{"Programming Language :: Python :: 3"}, ""), "an unguarded pyproject says nothing about publicity")
	assert.Equal(t, projectfile.VisibilityPublic, teaches(t, []string{classifierPrivate}, projectfile.VisibilityPublic), "a declared value is not overwritten by the file")
}
