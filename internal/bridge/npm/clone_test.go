// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CloneOfNil pins the nil receiver contract the sync dry-run path relies on:
// cloning a missing document must not panic and yields no document.
func TestCloneOfNil(t *testing.T) {
	var doc *Document
	assert.Nil(t, doc.Clone())
}

// TestCloneDeepCopiesEveryContainer pins that no slice, map or nested raw value
// is shared with the source: a dry-run planner mutating the copy must never
// reach the caller's document.
func TestCloneDeepCopiesEveryContainer(t *testing.T) {
	src := &Document{
		Name:            fxPkgDir,
		Keywords:        []string{"a", "b"},
		Dependencies:    map[string]string{"dep": "^1.0.0"},
		DevDependencies: map[string]string{"dev": "^2.0.0"},
		OS:              []string{fxLinux},
		CPU:             []string{fxX64},
		Contributors:    []any{map[string]any{npmKeyName: "Alice"}},
		Maintainers:     []any{map[string]any{npmKeyName: "Bob"}},
		Author:          map[string]any{npmKeyName: "Carol", fxKeyEmail: "carol@example.test"},
		Repository:      map[string]any{fxKeyURL: fxPkgURL, fxKeyType: repoTypeGit},
		Bugs:            map[string]any{"url": fxIssuesURL},
		Funding:         []any{map[string]any{"url": fxFundURL}},
	}
	cp := src.Clone()

	assert.Equal(t, src, cp, "the copy must be equal to the source")
	require.NotSame(t, src, cp)

	cp.Keywords[0] = fxMutated
	cp.Dependencies["dep"] = fxMutated
	cp.DevDependencies["dev"] = fxMutated
	cp.OS[0] = fxMutated
	cp.CPU[0] = fxMutated
	cp.Contributors[0].(map[string]any)[npmKeyName] = fxMutated
	cp.Maintainers[0].(map[string]any)[npmKeyName] = fxMutated
	cp.Author.(map[string]any)[npmKeyName] = fxMutated
	cp.Repository.(map[string]any)["url"] = fxMutated
	cp.Bugs.(map[string]any)["url"] = fxMutated
	cp.Funding.([]any)[0].(map[string]any)["url"] = fxMutated

	assert.Equal(t, []string{"a", "b"}, src.Keywords, "the source slice must not be aliased")
	assert.Equal(t, "a", src.Keywords[0])
	assert.Equal(t, "^1.0.0", src.Dependencies["dep"], "the source map must not be aliased")
	assert.Equal(t, "^2.0.0", src.DevDependencies["dev"])
	assert.Equal(t, []string{fxLinux}, src.OS)
	assert.Equal(t, []string{fxX64}, src.CPU)
	assert.Equal(t, "Alice", src.Contributors[0].(map[string]any)[npmKeyName])
	assert.Equal(t, "Bob", src.Maintainers[0].(map[string]any)[npmKeyName])
	assert.Equal(t, "Carol", src.Author.(map[string]any)[npmKeyName])
	assert.Equal(t, fxPkgURL, src.Repository.(map[string]any)["url"])
	assert.Equal(t, fxIssuesURL, src.Bugs.(map[string]any)["url"])
	assert.Equal(t, fxFundURL, src.Funding.([]any)[0].(map[string]any)["url"])
	assert.Equal(t, fxPkgDir, src.Name, "scalar fields are copied by value")
}

// TestCloneOfEmptyContainers pins that a nil container stays nil rather than
// becoming an empty one, so the writer keeps omitting absent keys.
func TestCloneOfEmptyContainers(t *testing.T) {
	cp := (&Document{Name: fxPkgDir}).Clone()
	assert.Nil(t, cp.Keywords)
	assert.Nil(t, cp.Dependencies)
	assert.Nil(t, cp.OS)
	assert.Nil(t, cp.CPU)
	assert.Nil(t, cp.Contributors)
	assert.Nil(t, cp.Maintainers)
	assert.Nil(t, cp.Author)
	assert.Nil(t, cp.Repository)
	assert.Nil(t, cp.Bugs)
	assert.Nil(t, cp.Funding)
}

// TestCloneCopiesScalarFields pins that a scalar without a container is carried
// across without any of the clone helpers dereferencing it.
func TestCloneCopiesScalarFields(t *testing.T) {
	src := &Document{
		Name:        fxPkgDir,
		Version:     "1.2.3",
		Description: "desc",
		License:     fxLicenseMIT,
		Homepage:    fxURLExampleTest,
		Private:     true,
		Author:      "Alice <alice@example.test>",
	}
	cp := src.Clone()
	assert.Equal(t, "Alice <alice@example.test>", cp.Author, "a scalar any is cloned as itself")
	assert.Equal(t, "1.2.3", cp.Version)
	assert.Equal(t, "desc", cp.Description)
	assert.Equal(t, fxLicenseMIT, cp.License)
	assert.Equal(t, fxURLExampleTest, cp.Homepage)
	assert.True(t, cp.Private)
}
