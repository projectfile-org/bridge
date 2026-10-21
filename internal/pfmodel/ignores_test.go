// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIgnoresParsesEveryTarget pins the whole target vocabulary, and the shorthand that a bare list is include-only.
func TestIgnoresParsesEveryTarget(t *testing.T) {
	ext, err := GetIgnoresExtension(extDoc(IgnoresExtensionNS, map[string]any{
		keyGenerate:     []any{targetGit, targetContainer},
		keyExtra:        []any{patternDist},
		targetGit:       []any{patternDist, "*.log"},
		targetDocker:    map[string]any{keyInclude: []any{"a"}, keyExclude: []any{"b"}},
		targetNpm:       []any{"node_modules"},
		"claude":        []any{".claude"},
		targetContainer: []any{".container"},
		"yamllint":      []any{".yamllint"},
		"textlint":      []any{".textlintignore"},
		"fd":            []any{".fdignore"},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{targetGit, targetContainer}, ext.Generate)
	assert.Equal(t, []string{patternDist}, ext.Extra)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{patternDist, "*.log"}}, ext.Git)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{"a"}, Exclude: []string{"b"}}, ext.Docker)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{"node_modules"}}, ext.Npm)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{".claude"}}, ext.Claude)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{".container"}}, ext.Container)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{".yamllint"}}, ext.Yamllint)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{".textlintignore"}}, ext.Textlint)
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{".fdignore"}}, ext.Fd)
}

// TestIgnoreOverrideShorthandAndRefusal pins both accepted shapes and the drop of anything else: a scalar target is no override.
func TestIgnoreOverrideShorthandAndRefusal(t *testing.T) {
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{"a", "b"}}, parseIgnoreTargetOverride([]any{"a", "b"}))
	assert.Equal(t, &IgnoreTargetOverride{Include: []string{"a"}, Exclude: []string{"b"}},
		parseIgnoreTargetOverride(map[string]any{keyInclude: []any{"a"}, keyExclude: []any{"b"}}))
	assert.Nil(t, parseIgnoreTargetOverride(notAMapping))
}

// TestEditorsReadsTheUseList.
func TestEditorsReadsTheUseList(t *testing.T) {
	ext, err := GetEditorsExtension(extDoc(EditorsExtensionNS, map[string]any{"use": []any{"vscode"}}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"vscode"}, ext.Use)
}

// TestVulnerabilitiesAcceptsBothSuppressForms pins the shorthand and the structured form, and the drop of an entry with no ID.
func TestVulnerabilitiesAcceptsBothSuppressForms(t *testing.T) {
	ext, err := GetVulnerabilitiesExtension(extDoc(VulnerabilitiesExtensionNS, map[string]any{
		keyGenerate: []any{"trivy"},
		"suppress": []any{
			"CVE-2024-0001",
			map[string]any{"id": "CVE-2024-0002", "reason": "blocked upstream"},
			map[string]any{"reason": "no id"},
			42,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"trivy"}, ext.Generate)
	assert.Equal(t, []VulnerabilitySuppress{
		{ID: "CVE-2024-0001"},
		{ID: "CVE-2024-0002", Reason: "blocked upstream"},
	}, ext.Suppress)

	ext, err = GetVulnerabilitiesExtension(extDoc(VulnerabilitiesExtensionNS, map[string]any{"suppress": notAMapping}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Suppress, "a suppress list that is not a list carries nothing")
}

// TestVulnerabilitiesParseSuppressListIsEmptyOnNothing pins the empty slice a consumer can range over directly.
func TestVulnerabilitiesParseSuppressListIsEmptyOnNothing(t *testing.T) {
	assert.Empty(t, parseSuppressList(nil))
	assert.Empty(t, parseSuppressList([]any{}))
}

// TestIgnoresGeneratesEveryTargetWhenUndeclared matches the generator's default: no project has to enumerate the files it already gets.
func TestIgnoresGeneratesEveryTargetWhenUndeclared(t *testing.T) {
	doc := extDoc(IgnoresExtensionNS, map[string]any{keyExtra: []any{"x"}})
	ext, err := GetIgnoresExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	for _, target := range []string{targetGit, targetDocker, targetNpm, "claude", targetContainer, "yamllint", "textlint", "fd"} {
		assert.True(t, ext.Generates(target), "target %q", target)
	}

	selected, err := GetIgnoresExtension(extDoc(IgnoresExtensionNS, map[string]any{keyGenerate: []any{targetGit}}))
	require.NoError(t, err)
	assert.True(t, selected.Generates(targetGit))
	assert.False(t, selected.Generates(targetDocker), "a target the list omits is not generated")
	assert.False(t, selected.Generates("unknown"))
}
