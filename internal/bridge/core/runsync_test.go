// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

const (
	testStubJSON = "stub.json"
	testAppName  = "my-app"
)

// ── Stub Syncer ──────────────────────────────────────────────────────────────

// memDoc is the in-memory "external document" for stub Syncer tests.
type memDoc struct {
	fields map[string]string
}

type stubSyncer struct {
	filename string
	exists   bool
	written  int // incremented on each Write call
}

func (s *stubSyncer) Name() string             { return "stub-syncer" }
func (s *stubSyncer) Filename() string         { return s.filename }
func (s *stubSyncer) Aliases() []string        { return nil }
func (s *stubSyncer) Labels() (string, string) { return s.filename, pfLabel }
func (s *stubSyncer) Policy() core.Policy      { return core.Policy{} }

func (s *stubSyncer) Exists(_ string) bool { return s.exists }

func (s *stubSyncer) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, s.filename)
}

func (s *stubSyncer) NewEmpty() any {
	return &memDoc{fields: map[string]string{}}
}

func (s *stubSyncer) Clone(doc any) any {
	d := doc.(*memDoc)
	cp := &memDoc{fields: make(map[string]string, len(d.fields))}
	for k, v := range d.fields {
		cp.fields[k] = v
	}
	return cp
}

func (s *stubSyncer) Read(_ string) (any, error) {
	return &memDoc{fields: map[string]string{"name": "from-ext"}}, nil
}

func (s *stubSyncer) Write(_ string, _ any) error {
	s.written++
	return nil
}

func (s *stubSyncer) BuildMappers(extDoc any, pf *projectfile.Document) core.MapperList {
	d := extDoc.(*memDoc)
	return core.MapperList{
		{
			ExtKey: "name",
			PFKey:  "identity.name",
			FromPF: func(force bool) string {
				if pf.Identity.Name == "" {
					return ""
				}
				if d.fields["name"] != "" && !force {
					return ""
				}
				d.fields["name"] = pf.Identity.Name
				return pf.Identity.Name
			},
			ToPF: func(force bool) string {
				v := d.fields["name"]
				if v == "" {
					return ""
				}
				if pf.Identity.Name != "" && !force {
					return ""
				}
				pf.Identity.Name = v
				return v
			},
		},
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func writePFFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestRunSyncCreateExt(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: my-app\n")
	syn := &stubSyncer{filename: testStubJSON, exists: false}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: testAppName}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeSync}

	res, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, 1, syn.written, "Write must be called once to create the ext file")
}

func TestRunSyncDryRunSkipsWrite(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: my-app\n")
	syn := &stubSyncer{filename: testStubJSON, exists: false}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: testAppName}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeSync, DryRun: true}

	res, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Equal(t, 0, syn.written, "dry-run must not call Write")
}

func TestRunSyncModeWritePushesToExt(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: from-pf\n")
	// ext file must exist for Syncer.Read to be called; Exists() is stubbed to true.
	syn := &stubSyncer{filename: testStubJSON, exists: true}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "from-pf"}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeWrite}

	res, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	assert.True(t, res.ExtChanged, "ModeWrite must mark ExtChanged")
	assert.Equal(t, 1, syn.written)
}

func TestRunSyncModeReadPullsFromExt(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: \"\"\n")
	syn := &stubSyncer{filename: testStubJSON, exists: true}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: ""}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead}

	_, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	// ToPF(force=true) copies "from-ext" → pf.Identity.Name
	assert.Equal(t, "from-ext", pf.Identity.Name)
}

// ModeSync is projectfile-authoritative: a set pf field overwrites the ext
// value even when the ext file is the newer one on disk.
func TestRunSyncPFWinsRegardlessOfMtime(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: from-pf\n")
	extPath := filepath.Join(dir, testStubJSON)
	require.NoError(t, os.WriteFile(extPath, []byte("{}"), 0o644))
	newer := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(extPath, newer, newer))
	syn := &stubSyncer{filename: testStubJSON, exists: true}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "from-pf"}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeSync}

	res, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	assert.True(t, res.ExtChanged, "the newer ext file must still take the pf value")
	assert.False(t, res.PFChanged, "a set pf field must never be overwritten from ext")
	assert.Equal(t, "from-pf", pf.Identity.Name)
}

// ModeSync gap-fills: an empty pf field takes the ext value (the backfill case).
func TestRunSyncEmptyPFFieldGapFillsFromExt(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: \"\"\n")
	syn := &stubSyncer{filename: testStubJSON, exists: true}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: ""}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeSync}

	res, err := core.RunSync(syn, pf, opts)
	require.NoError(t, err)
	assert.True(t, res.PFChanged, "an empty pf field must gap-fill from ext")
	assert.Equal(t, "from-ext", pf.Identity.Name)
}

func TestRunSyncNoCreateRefuses(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: my-app\n")
	syn := &stubSyncer{filename: testStubJSON, exists: false}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: testAppName}}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeSync, NoCreate: true}

	_, err := core.RunSync(syn, pf, opts)
	assert.Error(t, err, "--no-create must return an error when ext file is absent")
}

// TestRunSyncWritesBaseNotMerged is the regression test for the include-leak
// bug.  The base file declares an include that contributes license.spdx.  The
// merged doc the caller passes to RunSync has BOTH the base name (empty) and
// the include license.  After sync pushes "from-ext" into identity.name, the
// on-disk base file must contain the synced name but NOT the include-sourced
// license.
func TestRunSyncWritesBaseNotMerged(t *testing.T) {
	dir := t.TempDir()

	// Write the include file with license data.
	includePath := filepath.Join(dir, "include.yaml")
	require.NoError(t, os.WriteFile(includePath,
		[]byte("---\nlicense:\n  spdx: MIT\n"), 0o644))

	// Write the base projectfile that references the include.
	pfPath := writePFFile(t, dir,
		"---\nidentity:\n  name: \"\"\nincludes:\n  - include.yaml\n")

	// Read the MERGED doc — this is what the bridge command passes.
	merged, _, err := projectfile.ReadWithOptions(dir, projectfile.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, "MIT", merged.License.Spdx,
		"merged doc must see include license")

	// Run sync: ModeRead pushes "from-ext" into identity.name.
	syn := &stubSyncer{filename: testStubJSON, exists: true}
	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead}
	res, err := core.RunSync(syn, merged, opts)
	require.NoError(t, err)
	assert.True(t, res.PFChanged)

	// Re-read the BASE file from disk (no include resolution).
	base, err := projectfile.ReadBaseFromPath(pfPath)
	require.NoError(t, err)

	assert.Equal(t, "from-ext", base.Identity.Name,
		"synced name must land in base")
	assert.Nil(t, base.License,
		"include-sourced license must NOT be materialised into base")
}
