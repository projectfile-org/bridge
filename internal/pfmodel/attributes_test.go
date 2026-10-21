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

// TestAttributesNamesAreSortedAndOpen pins the byte-stability requirement: randomised map order needs one sorted name list.
func TestAttributesNamesAreSortedAndOpen(t *testing.T) {
	ext := &AttributesExtension{Rules: map[string]*AttributeRule{
		attrLinguistGenerated: {Include: []string{patternLock}},
		attrExportIgnore:      {Exclude: []string{pathDocs}},
		keyShell:              {Include: []string{patternSh}, Exclude: []string{pathVendor}},
	}}
	assert.Equal(t, []string{attrExportIgnore, attrLinguistGenerated, keyShell}, ext.AttributeNames())
	assert.Nil(t, (*AttributesExtension)(nil).AttributeNames(), "no extension declares no rules")
	assert.Empty(t, (&AttributesExtension{}).AttributeNames())
}

// TestGetAttributesExtensionParsesShorthandAndFullForms pins the two accepted shapes and the drop of a value that is neither.
func TestGetAttributesExtensionParsesShorthandAndFullForms(t *testing.T) {
	ext, err := GetAttributesExtension(extDoc(AttributesExtensionNS, map[string]any{
		keyGenerate:           []any{keyShell},
		keyExtra:              []any{attrExportIgnore},
		attrLinguistGenerated: []any{patternLock},
		keyShell:              map[string]any{keyInclude: []any{patternSh}, keyExclude: []any{pathVendor}},
		attrExportIgnore:      []any{pathDocs},
		"nonsense":            notAMapScalar,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{keyShell}, ext.Generate)
	assert.Equal(t, []string{attrExportIgnore}, ext.Extra)
	require.Len(t, ext.Rules, 3, "a value that is neither a list nor a map is no rule")
	assert.Equal(t, &AttributeRule{Include: []string{patternLock}}, ext.Rules[attrLinguistGenerated])
	assert.Equal(t, &AttributeRule{Include: []string{patternSh}, Exclude: []string{pathVendor}}, ext.Rules[keyShell])
	assert.Equal(t, &AttributeRule{Include: []string{pathDocs}}, ext.Rules[attrExportIgnore])
	assert.Equal(t, []string{attrExportIgnore, attrLinguistGenerated, keyShell}, ext.AttributeNames())
}

// TestParseAttributeRuleAcceptsOnlyTheTwoShapes pins the shorthand, the full mapping and the refusal.
func TestParseAttributeRuleAcceptsOnlyTheTwoShapes(t *testing.T) {
	assert.Equal(t, &AttributeRule{Include: []string{"a"}}, parseAttributeRule([]any{"a"}))
	assert.Equal(t, &AttributeRule{Include: []string{"a"}, Exclude: []string{"b"}},
		parseAttributeRule(map[string]any{keyInclude: []any{"a"}, keyExclude: []any{"b"}}))
	assert.Nil(t, parseAttributeRule(notAMapping))
	assert.Nil(t, parseAttributeRule(nil))
}

// TestGetAttributesExtensionAbsentIsBuiltInVocabularyOnly.
func TestGetAttributesExtensionAbsentIsBuiltInVocabularyOnly(t *testing.T) {
	ext, err := GetAttributesExtension(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, ext)
}
