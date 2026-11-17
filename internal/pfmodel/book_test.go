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

// TestPageCollectionsSortByName pins the order a book lists collections in, whatever order the map arrived in.
func TestPageCollectionsSortByName(t *testing.T) {
	got, err := GetPageCollections(extDoc(PagesExtensionNS, map[string]any{
		"rules":   map[string]any{keyTitle: map[string]any{"en": "Rules", "es": "Reglas"}},
		"presets": map[string]any{keyTitle: "Presets", "command": []any{fxTool, "list-presets"}},
	}))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "presets", got[0].Name)
	assert.Equal(t, []string{fxTool, "list-presets"}, got[0].Command)
	assert.Equal(t, &projectfile.LocalizedString{Bare: "Presets"}, got[0].Title)
	assert.Equal(t, "rules", got[1].Name)
	assert.Nil(t, got[1].Command, "an absent command stays nil, so the default applies")
	assert.Equal(t, "Reglas", got[1].Title.Langs["es"])
}

// TestPageCollectionsAbsentIsNil pins the no-collections case every project without pages takes.
func TestPageCollectionsAbsentIsNil(t *testing.T) {
	got, err := GetPageCollections(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestPageCollectionsRejectANonMapEntry pins the one malformed shape a reader cannot skip silently.
func TestPageCollectionsRejectANonMapEntry(t *testing.T) {
	_, err := GetPageCollections(extDoc(PagesExtensionNS, map[string]any{"rules": "Rules"}))
	assert.ErrorContains(t, err, "org.projectfile.pages.rules is not a map")
}

// TestBookPartsKeepDeclarationOrder pins that parts render in the order the conventions include lists them.
func TestBookPartsKeepDeclarationOrder(t *testing.T) {
	got, err := GetBookParts(extDoc(ConventionsExtensionNS, map[string]any{"book": map[string]any{"parts": []any{
		map[string]any{keyTitle: fxFeatures, keyPages: []any{outFeatures}},
		junkRow,
		map[string]any{keyTitle: "Reference", keyPages: []any{"docs/USAGE.md"}, "collections": true},
	}}}))
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, []string{outFeatures}, got[0].Pages)
	assert.False(t, got[0].Collections)
	assert.True(t, got[1].Collections)
}

// TestBookPartsAbsentIsNil pins both absences: no conventions namespace, and conventions without a book key.
func TestBookPartsAbsentIsNil(t *testing.T) {
	got, err := GetBookParts(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, got)
	got, err = GetBookParts(extDoc(ConventionsExtensionNS, map[string]any{keyFragments: map[string]any{}}))
	require.NoError(t, err)
	assert.Nil(t, got)
}
