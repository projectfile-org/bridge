// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// TestFragmentsReadsDeclaredDocumentsParentsFirst pins the explicit override namespace: documents are taken verbatim and each carries its own parents.
func TestFragmentsReadsDeclaredDocumentsParentsFirst(t *testing.T) {
	ext, err := GetFragmentsExtension(extDoc(FragmentsExtensionNS, map[string]any{
		keyDocuments: []any{
			map[string]any{
				keyDir: "docs/features.d", keyOut: outFeatures, keyTitle: "Features",
				keyParents: []any{map[string]any{fxURL: urlKiotaB19Ubuntu, "ref": refV1}},
			},
			junkRow,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Documents, 1)
	assert.Equal(t, "docs/features.d", ext.Documents[0].Dir)
	assert.Equal(t, outFeatures, ext.Documents[0].Out)
	assert.Equal(t, "Features", ext.Documents[0].Title)
	assert.Equal(t, []FragmentParent{{URL: urlKiotaB19Ubuntu, Ref: refV1}}, ext.Documents[0].Parents)
	assert.False(t, ext.Documents[0].Standalone, "the override namespace has no inherit flag")
}

// TestFragmentsDocumentsThatAreNotAListCarryNothing.
func TestFragmentsDocumentsThatAreNotAListCarryNothing(t *testing.T) {
	ext, err := GetFragmentsExtension(extDoc(FragmentsExtensionNS, map[string]any{keyDocuments: outFeatures}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Documents)
}

// TestConventionsFragmentsSplitsShellsFromParents pins the split the include merger depends on: shells from the include, parents from the project.
func TestConventionsFragmentsSplitsShellsFromParents(t *testing.T) {
	ext, err := GetConventionsFragments(extDoc(ConventionsExtensionNS, map[string]any{
		"fragments": map[string]any{
			keyDocuments: []any{
				map[string]any{keyDir: "docs/features.d", keyOut: outFeatures, keyTitle: "Features"},
				map[string]any{keyDir: "docs/roadmap.d", keyOut: "ROADMAP.md", "inherit": false},
			},
			keyParents: []any{map[string]any{fxURL: "https://kiota.ch/m6e/core"}},
		},
		"commit-style": commitStyleConventional,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Documents, 2)
	assert.False(t, ext.Documents[0].Standalone, "a shell inherits unless it says otherwise")
	assert.True(t, ext.Documents[1].Standalone, "inherit: false marks a standalone shell")
	assert.Equal(t, []FragmentParent{{URL: "https://kiota.ch/m6e/core"}}, ext.Parents)
}

// TestConventionsFragmentsAbsentAndWithoutSubKey pins the two nils the renderer distinguishes: no conventions at all, and conventions that declare no fragments.
func TestConventionsFragmentsAbsentAndWithoutSubKey(t *testing.T) {
	ext, err := GetConventionsFragments(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, ext)

	ext, err = GetConventionsFragments(extDoc(ConventionsExtensionNS, map[string]any{"commit-style": commitStyleConventional}))
	require.NoError(t, err)
	assert.Nil(t, ext, "conventions with no fragments sub-key declare no inheritance")

	ext, err = GetConventionsFragments(extDoc(ConventionsExtensionNS, map[string]any{"fragments": notAMapping}))
	require.NoError(t, err)
	assert.Nil(t, ext, "a fragments sub-key that is not a map declares no inheritance")
}

// TestParentsValDropsAnEntryWithNoURL pins the drop: a parent with no resolvable identity is an empty row the fetcher would have to guard against on every call.
func TestParentsValDropsAnEntryWithNoURL(t *testing.T) {
	out := parentsVal(map[string]any{keyParents: []any{
		map[string]any{fxURL: urlKiotaB19Ubuntu},
		map[string]any{"ref": refV1},
		junkRow,
	}}, keyParents)
	assert.Equal(t, []FragmentParent{{URL: urlKiotaB19Ubuntu}}, out)
	assert.Empty(t, parentsVal(map[string]any{}, keyParents))
	assert.Empty(t, parentsVal(map[string]any{keyParents: notAMapping}, keyParents))
}

// TestDocsAnyNarrowsToMapsAndDropsTheRest.
func TestDocsAnyNarrowsToMapsAndDropsTheRest(t *testing.T) {
	out := docsAny([]any{map[string]any{keyDir: "d"}, junkRow, nil})
	require.Len(t, out, 1)
	assert.Equal(t, "d", strVal(out[0], keyDir))
	assert.Nil(t, docsAny(notAMapping), "a list that is not a list is no list of documents")
	assert.Empty(t, docsAny([]any{}))
}
