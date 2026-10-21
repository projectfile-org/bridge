// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForgeTogglesDefaultToOn pins the rule that makes a partial block safe: naming one field disables only that one.
func TestForgeTogglesDefaultToOn(t *testing.T) {
	ext, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.True(t, ext.Push, "the namespace-wide switch is on unless switched off")
	assert.True(t, ext.Fields.Description)
	assert.True(t, ext.Fields.Homepage)
	assert.True(t, ext.Fields.Topics)
	assert.True(t, ext.Fields.Protection)
	assert.True(t, ext.Fields.DefaultBranch)
	assert.Nil(t, ext.Hosts, "no host table means every host")
	assert.Nil(t, ext.Kinds)
}

// TestForgePartialBlockDisablesOnlyWhatItNames pins the nested and top-level override paths together.
func TestForgePartialBlockDisablesOnlyWhatItNames(t *testing.T) {
	ext, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{
		keyPush: false,
		"fields": map[string]any{
			keyDescription: false, "default-branch": false,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.False(t, ext.Push)
	assert.False(t, ext.Fields.Description)
	assert.True(t, ext.Fields.Homepage, "an untouched field stays on")
	assert.True(t, ext.Fields.Topics, "an untouched field stays on")
	assert.True(t, ext.Fields.Protection, "an untouched field stays on")
	assert.False(t, ext.Fields.DefaultBranch)
}

// TestForgeHostTableReadsBoolsAndRecoversFromATypo pins the honoured bool and the mistyped value falling back to the safe default.
func TestForgeHostTableReadsBoolsAndRecoversFromATypo(t *testing.T) {
	ext, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{
		keyHosts: map[string]any{hostGitHubCom: true, hostCodeberg: false, "gitlab.com": valueYes},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Hosts, 3)
	assert.True(t, ext.Hosts[hostGitHubCom])
	assert.False(t, ext.Hosts[hostCodeberg])
	assert.True(t, ext.Hosts["gitlab.com"], "a non-bool is a typo, not a refusal")

	empty, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{keyHosts: map[string]any{}}))
	require.NoError(t, err)
	require.NotNil(t, empty)
	assert.Nil(t, empty.Hosts, "an empty table is no table")

	flat, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{keyHosts: hostGitHubCom}))
	require.NoError(t, err)
	require.NotNil(t, flat)
	assert.Nil(t, flat.Hosts)
}

// TestForgeKindsSkipsAMistypedKind pins the deliberate silence: a bad kind fails later at driver lookup, where it is named.
func TestForgeKindsSkipsAMistypedKind(t *testing.T) {
	ext, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{
		"kinds": map[string]any{hostGitHubCom: forgeGitHub, hostCodeberg: "forgejo", "gitlab.com": ""},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, map[string]string{hostGitHubCom: forgeGitHub, hostCodeberg: "forgejo"}, ext.Kinds)

	flat, err := GetForgeExtension(extDoc(ForgeExtensionNS, map[string]any{"kinds": forgeGitHub}))
	require.NoError(t, err)
	require.NotNil(t, flat)
	assert.Nil(t, flat.Kinds, "a kinds table that is not a map declares no kinds")
}

// TestBoolFieldDefaultTrue pins the single-level toggle reader.
func TestBoolFieldDefaultTrue(t *testing.T) {
	assert.True(t, boolFieldDefaultTrue(map[string]any{}, keyPush), "an absent key is on")
	assert.True(t, boolFieldDefaultTrue(map[string]any{keyPush: "false"}, keyPush), "a mistyped value is on")
	assert.True(t, boolFieldDefaultTrue(map[string]any{keyPush: true}, keyPush))
	assert.False(t, boolFieldDefaultTrue(map[string]any{keyPush: false}, keyPush))
}
