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

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/warn"
)

// Literals the whole core test package shares — goconst counts occurrences
// per package, so a third copy anywhere trips it.
const (
	pfLabel       = "projectfile"
	generatedFile = "GENERATED.md"
	generatedBody = "# generated\n"
)

// fixedRenderer emits one file with fixed content — enough to exercise the
// check path without dragging a real bridge's data requirements in.
type fixedRenderer struct {
	name, body string
}

func (r fixedRenderer) Name() string             { return r.name }
func (r fixedRenderer) Filename() string         { return r.name }
func (fixedRenderer) Aliases() []string          { return nil }
func (r fixedRenderer) Labels() (string, string) { return r.name, pfLabel }
func (fixedRenderer) Policy() core.Policy        { return core.Policy{Marker: true} }
func (r fixedRenderer) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, r.name))
	return err == nil
}

func (r fixedRenderer) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, r.name)
}

func (r fixedRenderer) Render(*projectfile.Document, core.Options) (core.Output, error) {
	return core.Output{Files: map[string][]byte{r.name: []byte(r.body)}}, nil
}

func runCheck(t *testing.T, dir string, r core.Renderer) error {
	t.Helper()
	return core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true})
}

// In sync: the check passes and writes nothing.
func TestCheckPassesWhenFileMatches(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	require.NoError(t, os.WriteFile(filepath.Join(dir, r.name), []byte(r.body), 0o644))

	assert.NoError(t, runCheck(t, dir, r))
}

// Hand-edited: --force would silently overwrite this; --check must report it
// and leave the edit on disk. That difference is the whole point of the flag.
func TestCheckFailsOnDriftAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	edited := "# generated\n\nsomeone added this by hand\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, r.name), []byte(edited), 0o644))

	err := runCheck(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), r.name)
	onDisk, readErr := os.ReadFile(filepath.Join(dir, r.name))
	require.NoError(t, readErr)
	assert.Equal(t, edited, string(onDisk), "the check must never write")
}

// A deleted generated file is drift too — it is exactly the state a sync gate
// exists to catch.
func TestCheckFailsWhenFileMissing(t *testing.T) {
	err := runCheck(t, t.TempDir(), fixedRenderer{name: generatedFile, body: generatedBody})

	require.Error(t, err)
	assert.Contains(t, err.Error(), generatedFile)
}

// WarnOnly keeps the report and drops the verdict: the file is still named, the
// warning ledger still holds it, and the run exits clean. This is the mode that
// lets a commit through while a published bridge fix waits for its image.
func TestCheckWarnOnlyReportsWithoutFailing(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	edited := "# hand-edited\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, r.name), []byte(edited), 0o644))
	warn.Reset()

	err := core.RunRender(r, &projectfile.Document{}, core.Options{
		Dir: dir, Check: true, DryRun: true, WarnOnly: true,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, warn.Count(), "silence is not the point — the ledger must hold it")
	onDisk, readErr := os.ReadFile(filepath.Join(dir, r.name))
	require.NoError(t, readErr)
	assert.Equal(t, edited, string(onDisk), "the check must never write")
}

// The lenient mode must not swallow a real failure: an unreadable target is a
// broken run, not drift, and stays an error whatever the drift severity is.
func TestCheckWarnOnlyStillFailsOnReadError(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	require.NoError(t, os.Mkdir(filepath.Join(dir, r.name), 0o755))
	warn.Reset()

	err := core.RunRender(r, &projectfile.Document{}, core.Options{
		Dir: dir, Check: true, DryRun: true, WarnOnly: true,
	})

	require.Error(t, err)
}
