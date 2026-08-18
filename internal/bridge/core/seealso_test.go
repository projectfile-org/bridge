// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

const (
	testTagsKey         = "tags"
	testTagContributing = "contributing"
	testTypeDocs        = "docs"
	testURLB            = "https://example.org/b"
)

func TestRenderSeeAlsoEmptyIsNil(t *testing.T) {
	assert.Nil(t, core.RenderSeeAlso(nil), "no links must grow no orphan heading")
	assert.Nil(t, core.RenderSeeAlso([]core.SeeAlsoLink{}))
}

func TestRenderSeeAlsoBulletsPerLink(t *testing.T) {
	out := core.RenderSeeAlso([]core.SeeAlsoLink{
		{Label: "Support channels", URL: "https://example.org/support"},
		{Label: "Security policy", URL: "https://example.org/security"},
	})
	assert.Equal(t,
		"## See also\n\n"+
			"- [Support channels](https://example.org/support)\n"+
			"- [Security policy](https://example.org/security)\n",
		string(out))
}

// ── SeeAlsoFor: tag-gated resolution of project-authored links[] ───────────

func TestSeeAlsoForReturnsNilWhenNoLinkCarriesTheTag(t *testing.T) {
	pf := &projectfile.Document{Links: []projectfile.Link{
		{Type: "chat", URL: "https://example.org/chat", Extra: map[string]any{testTagsKey: []any{"support"}}},
	}}
	assert.Nil(t, core.SeeAlsoFor(pf, testTagContributing, ""),
		"a link tagged for a different document must not leak into this one")
}

func TestSeeAlsoForReturnsNilOnNoLinksDeclared(t *testing.T) {
	assert.Nil(t, core.SeeAlsoFor(&projectfile.Document{}, testTagContributing, ""))
}

func TestSeeAlsoForFiltersByTag(t *testing.T) {
	pf := &projectfile.Document{Links: []projectfile.Link{
		{Type: "chat", URL: "https://example.org/chat", Extra: map[string]any{testTagsKey: []any{"support"}}},
		{
			Type:  testTypeDocs,
			URL:   "https://example.org/style-guide",
			Label: &projectfile.LocalizedString{Bare: "Style guide"},
			Extra: map[string]any{testTagsKey: []any{testTagContributing}},
		},
	}}
	got := core.SeeAlsoFor(pf, testTagContributing, "")
	assert.Equal(t, []core.SeeAlsoLink{{Label: "Style guide", URL: "https://example.org/style-guide"}}, got)
}

func TestSeeAlsoForLabelFallsBackToTypeThenURL(t *testing.T) {
	pf := &projectfile.Document{Links: []projectfile.Link{
		{Type: testTypeDocs, URL: "https://example.org/a", Extra: map[string]any{testTagsKey: []any{testTagContributing}}},
		{Type: "", URL: testURLB, Extra: map[string]any{testTagsKey: []any{testTagContributing}}},
	}}
	got := core.SeeAlsoFor(pf, testTagContributing, "")
	assert.Equal(t, []core.SeeAlsoLink{
		{Label: testTypeDocs, URL: "https://example.org/a"},
		{Label: testURLB, URL: testURLB},
	}, got)
}

func TestSeeAlsoForOrdersByPriorityThenDeclarationOrder(t *testing.T) {
	pf := &projectfile.Document{Links: []projectfile.Link{
		{
			Type: "default-priority", URL: "https://example.org/low",
			Extra: map[string]any{testTagsKey: []any{testTagContributing}},
		},
		{
			Type: "boosted", URL: "https://example.org/high",
			Extra: map[string]any{testTagsKey: []any{testTagContributing}, "priority": 90},
		},
	}}
	got := core.SeeAlsoFor(pf, testTagContributing, "")
	assert.Equal(t, "https://example.org/high", got[0].URL, "higher priority (90 > default 50) sorts first")
	assert.Equal(t, "https://example.org/low", got[1].URL)
}
