// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodeOwnersReadsEntriesInDeclarationOrder pins the order-significance: last-match-wins per path forbids any parser-side reorder.
func TestCodeOwnersReadsEntriesInDeclarationOrder(t *testing.T) {
	ext, err := GetCodeOwnersExtension(extDoc(CodeOwnersExtensionNS, map[string]any{
		"entries": []any{
			map[string]any{keyPattern: "*", keyOwners: []any{ownerGlobal}},
			map[string]any{keyPattern: pathDocs, keyOwners: []any{"@docs", ownerGlobal}},
			map[string]any{keyPattern: pathDocs, keyOwners: []any{"@again"}},
			junkRow,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Entries, 3, "a non-map entry costs that entry only")
	assert.Equal(t, CodeOwnersEntry{Pattern: "*", Owners: []string{ownerGlobal}}, ext.Entries[0])
	assert.Equal(t, CodeOwnersEntry{Pattern: pathDocs, Owners: []string{"@docs", ownerGlobal}}, ext.Entries[1])
	assert.Equal(t, CodeOwnersEntry{Pattern: pathDocs, Owners: []string{"@again"}}, ext.Entries[2],
		"a later duplicate pattern stays later — that is what last-match-wins means")
}

// TestCodeOwnersEntriesAbsentOrMalformed degrade to empty rather than an error.
func TestCodeOwnersEntriesAbsentOrMalformed(t *testing.T) {
	ext, err := GetCodeOwnersExtension(extDoc(CodeOwnersExtensionNS, map[string]any{"entries": notAMapping}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Entries)

	ext, err = GetCodeOwnersExtension(extDoc(CodeOwnersExtensionNS, map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Entries)
}
