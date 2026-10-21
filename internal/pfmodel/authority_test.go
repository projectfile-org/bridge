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

// TestLookupAuthorityTriesEverySpelling pins the three key spellings in order, and the dotted walk that lets one override sit under another.
func TestLookupAuthorityTriesEverySpelling(t *testing.T) {
	noAuthority := func(doc *projectfile.Document, bridge, extKey, pfKey string) {
		v, ok := lookupAuthority(doc, bridge, extKey, pfKey)
		assert.Empty(t, v)
		assert.False(t, ok)
	}
	noAuthority(nil, bridgePyproject, "requires-python", "requirements.runtime.python")
	noAuthority(&projectfile.Document{}, bridgePyproject, "requires-python", "requirements.runtime.python")

	// A namespace that is not a map is no authority map.
	noAuthority(extDoc(BridgeExtensionNS, notAMapping), bridgePyproject, "requires-python", "requirements.runtime.python")

	// A bridge namespace with no authority sub-map.
	noAuthority(extDoc(BridgeExtensionNS, map[string]any{"other": map[string]any{}}),
		bridgePyproject, "requires-python", "requirements.runtime.python")
}

// TestLookupAuthorityWalksDottedKeys pins the nested form one override can take.
func TestLookupAuthorityWalksDottedKeys(t *testing.T) {
	doc := extDoc(BridgeExtensionNS, map[string]any{keyAuthority: map[string]any{
		bridgePyproject: map[string]any{"requires-python": bridgePyproject},
	}})
	v, ok := lookupAuthority(doc, bridgePyproject, "requires-python", "")
	require.True(t, ok)
	assert.Equal(t, bridgePyproject, v)

	// A partial dotted walk resolves nothing.
	v, ok = lookupAuthority(doc, bridgePyproject, "other", "")
	assert.False(t, ok)
	assert.Empty(t, v)

	// A dotted walk that stops at a scalar resolves nothing.
	doc = extDoc(BridgeExtensionNS, map[string]any{keyAuthority: map[string]any{
		bridgePyproject: notAMapScalar,
	}})
	_, ok = lookupAuthority(doc, bridgePyproject, "requires-python", "")
	assert.False(t, ok)
}

// TestAuthorityValuesAreCaseAndSpaceInsensitive pins the normalisation, so `PyProject` and `  pyproject  ` both resolve to the file-wins side.
func TestAuthorityValuesAreCaseAndSpaceInsensitive(t *testing.T) {
	assert.True(t, isExternalValue(" PyProject ", bridgePyproject, "pyproject.toml"))
	assert.True(t, isExternalValue("PYPROJECT.TOML", bridgePyproject, "pyproject.toml"))
	assert.True(t, isExternalValue("  External ", bridgePyproject, "pyproject.toml"))
	assert.False(t, isExternalValue("someone-else", bridgePyproject, "pyproject.toml"))
	assert.False(t, isExternalValue("", bridgePyproject, "pyproject.toml"))

	assert.True(t, isProjectfileValue(" Projectfile "))
	assert.True(t, isProjectfileValue("PF"))
	assert.True(t, isProjectfileValue("SPEC"))
	assert.False(t, isProjectfileValue("external"))
	assert.False(t, isProjectfileValue("someone-else"))
	assert.False(t, isProjectfileValue(""))
}

// TestAuthorityLookupSkipsAnEmptyKey pins the guard that keeps a bridge with no filename from matching an empty override key.
func TestAuthorityLookupSkipsAnEmptyKey(t *testing.T) {
	doc := extDoc(BridgeExtensionNS, map[string]any{keyAuthority: map[string]any{
		"":          bridgePyproject,
		".":         bridgePyproject,
		"unrelated": bridgePyproject,
	}})
	v, ok := lookupAuthority(doc, "", "", "")
	assert.False(t, ok, "an empty key and a bare dot are not spellings")
	assert.Empty(t, v)
}
