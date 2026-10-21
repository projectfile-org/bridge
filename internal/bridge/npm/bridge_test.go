// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

func TestBridgeIdentity(t *testing.T) {
	b := Bridge{}
	ext, pf := b.Labels()
	assert.Equal(t, "npm", b.Name())
	assert.Equal(t, "package.json", b.Filename())
	assert.Empty(t, b.Aliases())
	assert.Equal(t, "package.json", ext, "first label is the external file")
	assert.Equal(t, "projectfile", pf, "second label is the document")
	assert.Equal(t, core.Policy{}, b.Policy())
	assert.Equal(t, []string{fxEngineName}, b.Stacks())
}

func TestBridgeExistsAndFullPath(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p"}`)

	assert.True(t, Bridge{}.Exists(dir), "package.json is present")
	assert.False(t, Bridge{}.Exists(t.TempDir()), "an empty dir carries no package.json")
	assert.Equal(t, filepath.Join(dir, "package.json"), Bridge{}.FullPath(dir, nil))
}

func TestBridgeReadNewEmptyAndClone(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","keywords":["a"]}`)

	ext, err := Bridge{}.Read(dir)
	require.NoError(t, err)
	doc, ok := ext.(*Document)
	require.True(t, ok, "Read returns the typed document")
	assert.Equal(t, "p", doc.Name)

	empty := Bridge{}.NewEmpty()
	require.IsType(t, &Document{}, empty)

	clone := Bridge{}.Clone(doc)
	assert.Equal(t, doc, clone, "Clone returns an equal document")
	assert.NotSame(t, doc, clone, "Clone must not alias the source")

	nilClone := Bridge{}.Clone((*Document)(nil))
	assert.Nil(t, nilClone, "a nil document clones to nil")

	wrong := Bridge{}.Clone("not a document")
	assert.Equal(t, "not a document", wrong, "a foreign type is handed back untouched")
}

func TestBridgeWriteRejectsWrongType(t *testing.T) {
	err := Bridge{}.Write(t.TempDir(), "not a document")
	require.Error(t, err, "a foreign document type must be refused, not silently written")
}

func TestBridgeBuildMappersNilDocument(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "tool"}}
	mappers := Bridge{}.BuildMappers(nil, pf)
	require.NotEmpty(t, mappers, "a nil external document still builds the mapper list")
}

func TestBridgeBuildMappersWrongType(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "tool"}}
	mappers := Bridge{}.BuildMappers(struct{}{}, pf)
	require.NotEmpty(t, mappers)
}

func TestFilenameAndExistsOnMissingDir(t *testing.T) {
	assert.Equal(t, "package.json", Filename)
	assert.False(t, Exists(filepath.Join(t.TempDir(), "nowhere")))
}

func TestFullPathFallsBackWhenCwdIsGone(t *testing.T) {
	// filepath.Abs fails only when the working directory was unlinked, which is
	// the one case FullPath has a relative fallback for.
	orig, err := os.Getwd()
	require.NoError(t, err)
	gone := t.TempDir()
	require.NoError(t, os.Chdir(gone))
	require.NoError(t, os.RemoveAll(gone))
	defer func() { assert.NoError(t, os.Chdir(orig)) }()

	assert.Equal(t, Filename, FullPath("."), "with no cwd the relative name is returned")
}
