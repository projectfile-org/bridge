// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetReadmeExtensionSectionsAreAListOfGroups pins the list shape: install alternatives are not steps, so each gets its own fence.
func TestGetReadmeExtensionSectionsAreAListOfGroups(t *testing.T) {
	ext, err := GetReadmeExtension(extDoc(ReadmeExtensionNS, map[string]any{
		"build-doc": true,
		sectionInstallation: []any{
			map[string]any{
				keyName:     targetDocker,
				keyTitle:    map[string]any{"en": titleWithDocker, "es": titleConDocker},
				"prefix":    map[string]any{"en": textPullIt},
				keyCommands: []any{"docker pull org/name"},
				"postfix":   map[string]any{"en": textThenRunIt},
				"syntax":    "sh", "priority": 90, "per-cell": true,
				"cells": "{GOOS}/{GOARCH}", keyFile: "scripts/install.sh",
			},
			map[string]any{
				keyName:     targetDocker,
				keyCommands: []any{"docker pull org/name:latest"},
			},
			junkRow,
		},
		"usage": []any{map[string]any{keyName: ""}},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.True(t, ext.BuildDoc)

	require.Contains(t, ext.Sections, sectionInstallation)
	groups := ext.Sections[sectionInstallation]
	require.Len(t, groups, 1, "a redeclared group name REPLACES the inherited one")
	assert.Equal(t, targetDocker, groups[0].Name)
	assert.Equal(t, []string{"docker pull org/name:latest"}, groups[0].Commands, "LAST wins")
	assert.Empty(t, groups[0].Title, "an override replaces the whole group, fields it omits included")
	assert.Empty(t, groups[0].Cells, "an override carries no inherited fence it did not restate")

	// A group that is not overridden keeps everything it declared.
	ext, err = GetReadmeExtension(extDoc(ReadmeExtensionNS, map[string]any{
		sectionInstallation: []any{
			map[string]any{
				keyName:     targetDocker,
				keyTitle:    map[string]any{"en": titleWithDocker, "es": titleConDocker},
				"prefix":    map[string]any{"en": textPullIt},
				keyCommands: []any{"docker pull org/name"},
				"postfix":   map[string]any{"en": textThenRunIt},
				"syntax":    "sh", "priority": 90, "per-cell": true,
				"cells": "{GOOS}/{GOARCH}", keyFile: "scripts/install.sh",
			},
		},
	}))
	require.NoError(t, err)
	groups = ext.Sections[sectionInstallation]
	require.Len(t, groups, 1)
	assert.Equal(t, titleWithDocker, groups[0].Title)
	assert.Equal(t, map[string]string{"en": titleWithDocker, "es": titleConDocker}, groups[0].TitleByLang)
	assert.Equal(t, textPullIt, groups[0].Prefix)
	assert.Equal(t, map[string]string{"en": textPullIt}, groups[0].PrefixByLang)
	assert.Equal(t, textThenRunIt, groups[0].Postfix)
	assert.Equal(t, map[string]string{"en": textThenRunIt}, groups[0].PostfixByLang)
	assert.Equal(t, "sh", groups[0].Syntax)
	assert.Equal(t, 90, groups[0].Priority)
	assert.True(t, groups[0].PerCell)
	assert.Equal(t, "{GOOS}/{GOARCH}", groups[0].Cells)
	assert.Equal(t, "scripts/install.sh", groups[0].File)

	assert.NotContains(t, ext.Sections, "usage", "a group with no commands, file or prose renders nothing")
}

// TestParseReadmeSectionDropsAnEmptyGroup pins the drop rule: an unresolvable group disappears whole, lead-in sentence included.
func TestParseReadmeSectionDropsAnEmptyGroup(t *testing.T) {
	out := parseReadmeSection(map[string]any{}, sectionInstallation)
	assert.Nil(t, out, "an absent section is no section")
	out = parseReadmeSection(map[string]any{sectionInstallation: notAMapping}, sectionInstallation)
	assert.Nil(t, out, "a section that is not a list is no section")
	out = parseReadmeSection(map[string]any{sectionInstallation: []any{junkRow, map[string]any{keyName: "x"}}}, sectionInstallation)
	assert.Nil(t, out, "every entry empty falls through to the file-probe fallback")

	unnamed := parseReadmeSection(map[string]any{sectionInstallation: []any{
		map[string]any{keyCommands: []any{"a"}},
		map[string]any{keyCommands: []any{"b"}},
	}}, sectionInstallation)
	require.Len(t, unnamed, 2, "unnamed groups never collide — each is kept as declared")
}

// TestParseReadmeSectionKeepsFirstPositionOnOverride pins the position rule: an override never reorders the section, it replaces in place.
func TestParseReadmeSectionKeepsFirstPositionOnOverride(t *testing.T) {
	out := parseReadmeSection(map[string]any{sectionInstallation: []any{
		map[string]any{keyName: targetDocker, keyCommands: []any{"a"}},
		map[string]any{keyName: targetNpm, keyCommands: []any{"b"}},
		map[string]any{keyName: targetDocker, keyCommands: []any{"c"}},
	}}, sectionInstallation)
	require.Len(t, out, 2)
	assert.Equal(t, targetDocker, out[0].Name, "the first position is kept")
	assert.Equal(t, []string{"c"}, out[0].Commands)
	assert.Equal(t, targetNpm, out[1].Name)
}

// TestGetReadmeExtensionExtrasSkipsMalformedEntries matches the shields behaviour: one bad entry costs that entry only.
func TestGetReadmeExtensionExtrasSkipsMalformedEntries(t *testing.T) {
	ext, err := GetReadmeExtension(extDoc(ReadmeExtensionNS, map[string]any{
		"extras": []any{junkRow, nil, map[string]any{keyName: "notice", "content": keyText}},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Extras, 1)
	assert.Equal(t, "notice", ext.Extras[0].Name)
	assert.Equal(t, keyText, ext.Extras[0].Content)

	none, err := GetReadmeExtension(extDoc(ReadmeExtensionNS, map[string]any{"extras": notAMapping}))
	require.NoError(t, err)
	require.NotNil(t, none)
	assert.Empty(t, none.Extras)
}

// TestExtractLocalizedValPrefersEnglishThenAnyNonEmpty pins the default resolution a localized value collapses to.
func TestExtractLocalizedValPrefersEnglishThenAnyNonEmpty(t *testing.T) {
	assert.Equal(t, "Hi", extractLocalizedVal(map[string]any{"k": map[string]any{"en": "Hi", "es": msgHola}}, "k"))
	assert.Equal(t, msgHola, extractLocalizedVal(map[string]any{"k": map[string]any{"es": msgHola, "": ""}}, "k"),
		"an absent English entry takes any non-empty translation")
	assert.Equal(t, keyText, extractLocalizedVal(map[string]any{"k": keyText}, "k"))
	assert.Empty(t, extractLocalizedVal(map[string]any{}, "k"))
	assert.Empty(t, extractLocalizedVal(map[string]any{"k": 7}, "k"), "a number is not prose")
	assert.Empty(t, extractLocalizedVal(map[string]any{"k": map[string]any{}}, "k"))
	assert.Empty(t, extractLocalizedVal(map[string]any{"k": map[string]any{"en": ""}}, "k"))
}

// TestLocalizedTextFallsBackToTheDefault pins the two-step read a language-aware consumer performs.
func TestLocalizedTextFallsBackToTheDefault(t *testing.T) {
	m := map[string]any{"k": map[string]any{"en": "Hi", "es": msgHola}}
	assert.Equal(t, msgHola, LocalizedText(m, "k", "es"))
	assert.Equal(t, "Hi", LocalizedText(m, "k", "uk"), "an unrendered language takes the default")
	assert.Equal(t, "Hi", LocalizedText(m, "k", ""))
	assert.Equal(t, "bare", LocalizedText(map[string]any{"k": "bare"}, "k", "es"), "a bare string has no variants")
	assert.Empty(t, LocalizedText(map[string]any{}, "k", "es"))
}

// TestExtractLocalizedMapReturnsNilForAnythingButALangMap pins the nil a bridge reads as "single-language, no per-lang render".
func TestExtractLocalizedMapReturnsNilForAnythingButALangMap(t *testing.T) {
	assert.Equal(t, map[string]string{"en": "Hi"}, extractLocalizedMap(map[string]any{"k": map[string]any{"en": "Hi"}}, "k"))
	assert.Nil(t, extractLocalizedMap(map[string]any{"k": keyText}, "k"), "a bare string carries no lang map")
	assert.Nil(t, extractLocalizedMap(map[string]any{}, "k"), "an absent key carries no lang map")
	assert.Nil(t, extractLocalizedMap(map[string]any{"k": 7}, "k"), "a number carries no lang map")
	assert.Nil(t, extractLocalizedMap(map[string]any{"k": map[string]any{"en": ""}}, "k"), "an all-empty map carries nothing")
}
