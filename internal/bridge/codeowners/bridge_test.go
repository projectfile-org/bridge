// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

func TestBridgeIdentitySurface(t *testing.T) {
	t.Parallel()

	b := Bridge{}
	assert.Equal(t, "codeowners", b.Name())
	assert.Equal(t, Filename, b.Filename())
	assert.Nil(t, b.Aliases(), "CODEOWNERS has no alternative spelling")
	assert.Equal(t, core.Policy{}, b.Policy(), "a synced file carries no overwrite marker")

	ext, label := b.Labels()
	assert.Equal(t, Filename, ext)
	assert.Equal(t, "projectfile", label)
	assert.IsType(t, &Document{}, b.NewEmpty())
}

func TestBridgeExistsAndFullPathFollowTheProbe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	b := Bridge{}
	assert.False(t, b.Exists(dir))
	assert.Equal(t, filepath.Join(dir, Filename), b.FullPath(dir, nil))

	writeProbe(t, dir, testProbeDocs, testPatternDeploy+" "+testOwnerOps+"\n")
	assert.True(t, b.Exists(dir))
	assert.Equal(t, filepath.Join(dir, testProbeDocs), b.FullPath(dir, &projectfile.Document{}),
		"the projectfile never moves the CODEOWNERS probe path")
}

func TestBridgeReadHandsBackThePackagesOwnDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProbe(t, dir, Filename, testPatternSrc+" "+testOwnerAda+"\n")
	got, err := Bridge{}.Read(dir)
	require.NoError(t, err)
	doc, ok := got.(*Document)
	require.True(t, ok, "Read returns the package document so the sync engine can build mappers from it")
	require.Len(t, doc.Lines, 1)
	assert.Equal(t, testPatternSrc, doc.Lines[0].Pattern)
}

func TestBridgeWriteRejectsAForeignDocument(t *testing.T) {
	t.Parallel()

	err := Bridge{}.Write(t.TempDir(), "not-a-document")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected document type")
}

func TestBridgeWriteAcceptsItsOwnDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, Bridge{}.Write(dir, &Document{
		Lines: []Line{{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}}},
	}))
	assert.Equal(t, testPatternSrc+" "+testOwnerAda+"\n", readFile(t, filepath.Join(dir, Filename)))
}

func TestBridgeClonePassesAValueThrough(t *testing.T) {
	t.Parallel()

	assert.Nil(t, Bridge{}.Clone(nil), "a nil document clones to itself")
	assert.Equal(t, "foreign", Bridge{}.Clone("foreign"), "a foreign type is handed back untouched")

	cp, ok := Bridge{}.Clone(&Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}}).(*Document)
	require.True(t, ok)
	require.Len(t, cp.Lines, 1)
	assert.Equal(t, []string{testOwnerAda}, cp.Lines[0].Owners)
}

func TestBridgeBuildMappersAcceptsANilDocument(t *testing.T) {
	t.Parallel()

	var mappers core.MapperList
	require.NotPanics(t, func() { mappers = Bridge{}.BuildMappers(nil, &projectfile.Document{}) })
	assert.Len(t, mappers, 1, "a nil external document builds a fresh one")

	typed, err := Bridge{}.Read(t.TempDir())
	require.NoError(t, err)
	assert.Len(t, Bridge{}.BuildMappers(typed, &projectfile.Document{}), 1)
}

func TestReadWriteRoundTripIsStable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	body := testPatternDeploy + " " + testOwnerOps + "\n"
	writeProbe(t, dir, Filename, body)

	doc, err := Bridge{}.Read(dir)
	require.NoError(t, err)
	require.NoError(t, Bridge{}.Write(dir, doc))
	assert.Equal(t, body, readFile(t, filepath.Join(dir, Filename)), "write→read→write must be byte-stable")

	_, err = os.Stat(filepath.Join(dir, testProbeGithub))
	assert.True(t, os.IsNotExist(err), "a root file is never duplicated under .github/")
}
