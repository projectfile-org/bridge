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
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

const (
	retiredFile    = "RETIRED.txt"
	retiredMarker  = "# pf-cli-managed: yes\n"
	emptyStateBody = "# retired state — nothing is suppressed\n"
)

// A managed copy of a file the render retired: pf's state is the file's ABSENCE — the run must bring the disk to it.
func writeManagedRetired(t *testing.T, dir, body string) string {
	t.Helper()
	target := filepath.Join(dir, retiredFile)
	require.NoError(t, os.WriteFile(target, []byte(retiredMarker+body), 0o644))
	return target
}

func TestRetireDeletesManagedFile(t *testing.T) {
	dir := t.TempDir()
	target := writeManagedRetired(t, dir, "CVE-1\n")

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: false}))

	_, err := os.Stat(target)
	assert.True(t, os.IsNotExist(err), "a managed stale file must be removed, not kept")
}

// A detached copy belongs to its human — retirement skips it, and the run still succeeds, exactly as the overwrite gate leaves it alone.
func TestRetireLeavesDetachedFileAlone(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, retiredFile)
	require.NoError(t, os.WriteFile(target, []byte("hand-adopted, no marker\n"), 0o644))

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: false}))

	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hand-adopted, no marker\n", string(onDisk), "a detached stale file must keep every byte")
}

// --force is the one way a detached copy is retired: it answers "make it match pf" whatever the marker state is.
func TestRetireForceOverridesDetached(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, retiredFile)
	require.NoError(t, os.WriteFile(target, []byte("hand-adopted, no marker\n"), 0o644))

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Force: true, NoCreate: false}))

	_, err := os.Stat(target)
	assert.True(t, os.IsNotExist(err), "--force retires a detached copy like it overwrites one")
}

func TestRetireDryRunKeepsFile(t *testing.T) {
	dir := t.TempDir()
	target := writeManagedRetired(t, dir, "CVE-1\n")

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, DryRun: true}))

	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(onDisk), "CVE-1", "a dry-run must not delete anything")
}

// An absent delete target is already the state pf declares — a no-op, never a
// create.
func TestRetireMissingDeleteTargetIsNoOp(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: false}))
	_, err := os.Stat(filepath.Join(dir, retiredFile))
	assert.True(t, os.IsNotExist(err))
}

// A rewrite entry (non-nil body) is the retirement for files a scanner wrapper requires on disk: stale content replaced, the file survives.
func TestRetireRewritesToEmptyState(t *testing.T) {
	dir := t.TempDir()
	target := writeManagedRetired(t, dir, "CVE-1\n")

	empty := emptyStateBody
	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(empty)}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: false}))

	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, empty, string(onDisk), "a rewrite retirement replaces the stale body, not the file")
}

// A missing rewrite target joins a --create-all run like any other declared file: pf's state for it is present-with-this-body.
func TestRetireRewriteCreateAllCreatesMissing(t *testing.T) {
	dir := t.TempDir()
	empty := emptyStateBody
	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(empty)}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: false}))

	onDisk, err := os.ReadFile(filepath.Join(dir, retiredFile))
	require.NoError(t, err)
	assert.Equal(t, empty, string(onDisk))
}

func TestRetireRewriteNoCreateSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	empty := emptyStateBody
	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(empty)}}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, NoCreate: true}))
	_, err := os.Stat(filepath.Join(dir, retiredFile))
	assert.True(t, os.IsNotExist(err), "without --create-all the missing rewrite target is skipped")
}

// ── the check side ──────────────────────────────────────────────────────────

// A managed stale copy is drift: the gate must name the file and change nothing on disk.
func TestCheckRetireDriftsOnManagedStaleFile(t *testing.T) {
	dir := t.TempDir()
	target := writeManagedRetired(t, dir, "CVE-1\n")

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	err := core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), retiredFile)
	onDisk, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Contains(t, string(onDisk), "CVE-1", "the check must never write")
}

func TestCheckRetirePassesWhenStaleFileIsGone(t *testing.T) {
	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	assert.NoError(t, runCheck(t, t.TempDir(), r))
}

func TestCheckRetirePassesOnDetachedStaleFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, retiredFile), []byte("hand-adopted, no marker\n"), 0o644))

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: nil}}
	assert.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true}))
}

// A managed copy already in the empty state is in sync — the ran-through case after one real generate.
func TestCheckRetirePassesOnRewrittenEmptyState(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, retiredFile), []byte(emptyStateBody), 0o644))

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(emptyStateBody)}}
	assert.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true}))
}

func TestCheckRetireDriftsOnStaleManagedRewrite(t *testing.T) {
	dir := t.TempDir()
	target := writeManagedRetired(t, dir, "CVE-1\n")

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(emptyStateBody)}}
	err := core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), retiredFile)
	onDisk, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Contains(t, string(onDisk), "CVE-1", "the check must never write")
}

// A detached rewrite target is drift-proof in check mode too, whatever its content — the managed-only rule.
func TestCheckRetirePassesOnDetachedManagedContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, retiredFile), []byte("hand-adopted stale body\n"), 0o644))

	r := &stubRenderer{filename: retiredFile, policy: core.Policy{Marker: true}, remove: map[string][]byte{retiredFile: []byte(emptyStateBody)}}
	assert.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Check: true, DryRun: true}))
}
