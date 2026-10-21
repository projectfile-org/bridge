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

// TestCitationAbsentAndNotAMap pins both answers: an absent namespace is no citation, a non-map one is a parse error.
func TestCitationAbsentAndNotAMap(t *testing.T) {
	ext, err := GetCitationExtension(nil)
	require.NoError(t, err)
	assert.Nil(t, ext)

	ext, err = GetCitationExtension(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, ext, "no extensions at all is no citation")

	ext, err = GetCitationExtension(&projectfile.Document{Extensions: map[string]any{}})
	require.NoError(t, err)
	assert.Nil(t, ext)

	_, err = GetCitationExtension(extDoc(CitationExtensionNS, []any{junkWord, "a", notAList}))
	require.Error(t, err)
}

// TestCitationReadsDOIAndPreferredForm pins both levels of the namespace, including the volume and issue integers the CFF schema requires.
func TestCitationReadsDOIAndPreferredForm(t *testing.T) {
	ext, err := GetCitationExtension(extDoc(CitationExtensionNS, map[string]any{
		keyDOI:    testDOI,
		"message": "If you use this, cite it.",
		"preferred": map[string]any{
			"type": citationArticle, keyTitle: testTitle, "journal": "JOSS", "volume": 3,
			"issue": 2, "pages": "1-10", keyYear: 2026, keyDOI: "10.5678/cite",
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, testDOI, ext.DOI)
	assert.Equal(t, "If you use this, cite it.", ext.Message)
	require.NotNil(t, ext.Preferred)
	assert.Equal(t, PreferredCitation{
		Type: citationArticle, Title: testTitle, Journal: "JOSS", Volume: 3,
		Issue: 2, Pages: "1-10", Year: 2026, DOI: "10.5678/cite",
	}, *ext.Preferred)
}

// TestCitationPreferredThatIsNotAMapLeavesNoPreferred pins the silent drop: a malformed preferred block yields the plain DOI/message citation.
func TestCitationPreferredThatIsNotAMapLeavesNoPreferred(t *testing.T) {
	ext, err := GetCitationExtension(extDoc(CitationExtensionNS, map[string]any{
		keyDOI: testDOI, "preferred": citationArticle,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, testDOI, ext.DOI)
	assert.Nil(t, ext.Preferred)
}

// TestCitationReadsTheFlatKeyOnly pins the reader the namespace has always used: there is no TOML dotted-header legacy to resolve here.
func TestCitationReadsTheFlatKeyOnly(t *testing.T) {
	doc := &projectfile.Document{Extensions: map[string]any{
		"org": map[string]any{authorityPF: map[string]any{"citation": map[string]any{keyDOI: testDOI}}},
	}}
	ext, err := GetCitationExtension(doc)
	require.NoError(t, err)
	assert.Nil(t, ext, "the flat key is the only spelling this reader knows")
}
