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

// TestLinkByTypeAppliesTheSelectionRule pins spec §5.11: the preferred entry wins, else the sole entry, else the first in document order.
func TestLinkByTypeAppliesTheSelectionRule(t *testing.T) {
	assert.Nil(t, LinkByType(nil, LinkSourceCode), "no document addresses no link")
	assert.Nil(t, LinkByType(&projectfile.Document{}, ""), "an empty type addresses no link")
	assert.Nil(t, LinkByType(&projectfile.Document{}, LinkSourceCode), "no entry of that type")

	doc := &projectfile.Document{Links: []projectfile.Link{
		{Type: linkTypeDocumentation, URL: "https://docs.test"},
		{Type: LinkSourceCode, URL: "https://mirror.test"},
		{Type: LinkSourceCode, URL: urlOrigin, Preferred: true},
	}}
	l := LinkByType(doc, LinkSourceCode)
	require.NotNil(t, l)
	assert.Equal(t, urlOrigin, l.URL, "preferred wins over document order")

	unpreferred := &projectfile.Document{Links: []projectfile.Link{
		{Type: LinkSourceCode, URL: "https://first.test"},
		{Type: LinkSourceCode, URL: "https://second.test"},
	}}
	assert.Equal(t, "https://first.test", LinkByType(unpreferred, LinkSourceCode).URL,
		"no preferred entry takes the first")

	sole := &projectfile.Document{Links: []projectfile.Link{{Type: LinkSourceCode, URL: "https://only.test"}}}
	assert.Equal(t, "https://only.test", LinkByType(sole, LinkSourceCode).URL)
	assert.Equal(t, "https://only.test", LinkURL(sole, LinkSourceCode))
	assert.Empty(t, LinkURL(nil, LinkSourceCode), "no entry is no URL")
}

// TestLinksByTypeAndByTagListEveryMatch pins the listing readers, which must keep document order and report nothing rather than a guess.
func TestLinksByTypeAndByTagListEveryMatch(t *testing.T) {
	doc := &projectfile.Document{Links: []projectfile.Link{
		{Type: "chat", URL: "https://a.test", Extra: map[string]any{keyTags: []any{TagSupport}}},
		{Type: "chat", URL: "https://b.test"},
	}}
	require.Len(t, LinksByType(doc, "chat"), 2)
	assert.Equal(t, "https://a.test", LinksByType(doc, "chat")[0].URL)
	assert.Nil(t, LinksByType(nil, "chat"), "no document lists nothing")
	assert.Nil(t, LinksByType(doc, ""), "an empty type lists nothing")

	got := LinksByTag(doc, TagSupport)
	require.Len(t, got, 1)
	assert.Equal(t, "https://a.test", got[0].URL)
	assert.Nil(t, LinksByTag(nil, TagSupport))
	assert.Nil(t, LinksByTag(doc, ""), "an empty tag lists nothing")
	assert.Nil(t, LinksByTag(doc, TagPublic), "no link carrying it lists nothing")
}

// TestSetLinkHonoursGapFillAndForce pins the three answers a write can give: no change, a refusal, and a write.
func TestSetLinkHonoursGapFillAndForce(t *testing.T) {
	assert.False(t, SetLink(nil, LinkSourceCode, urlXTest, true), "no document mutates nothing")
	assert.False(t, SetLink(&projectfile.Document{}, "", urlXTest, true), "no type mutates nothing")
	assert.False(t, SetLink(&projectfile.Document{}, LinkSourceCode, "", true), "no URL mutates nothing")

	doc := &projectfile.Document{Links: []projectfile.Link{
		{Type: LinkSourceCode, URL: "https://old.test"},
	}}
	assert.False(t, SetLink(doc, LinkSourceCode, "https://old.test", true), "the same URL is not a change")
	assert.False(t, SetLink(doc, LinkSourceCode, "https://new.test", false),
		"a declared URL is not overwritten without force")
	assert.True(t, SetLink(doc, LinkSourceCode, "https://new.test", true), "force overwrites")
	assert.Equal(t, "https://new.test", LinkURL(doc, LinkSourceCode))

	empty := &projectfile.Document{Links: []projectfile.Link{{Type: LinkSourceCode}}}
	assert.True(t, SetLink(empty, LinkSourceCode, "https://fill.test", false), "an empty entry is a gap to fill")
	assert.Equal(t, "https://fill.test", LinkURL(empty, LinkSourceCode))

	fresh := &projectfile.Document{}
	assert.True(t, SetLink(fresh, LinkSourceCode, "https://add.test", false), "no entry appends one")
	require.Len(t, fresh.Links, 1)
	assert.Equal(t, "https://add.test", fresh.Links[0].URL)
}

// TestLinkPriorityReadsThePriorityChannel pins the three numeric shapes a decoder can hand back and the default every other value resolves to.
func TestLinkPriorityReadsThePriorityChannel(t *testing.T) {
	assert.Equal(t, 90, LinkPriority(projectfile.Link{Extra: map[string]any{keyPriority: 90}}))
	assert.Equal(t, 90, LinkPriority(projectfile.Link{Extra: map[string]any{keyPriority: int64(90)}}))
	assert.Equal(t, 90, LinkPriority(projectfile.Link{Extra: map[string]any{keyPriority: float64(90)}}))
	assert.Equal(t, PriorityDefault, LinkPriority(projectfile.Link{}), "an absent key is the default rank")
	assert.Equal(t, PriorityDefault, LinkPriority(projectfile.Link{
		Extra: map[string]any{keyPriority: "90"},
	}), "a numeric string is not a rank")
}

// TestSetLinkPriorityGapFills pins the shared write rule: a declared key wins, the default rank writes nothing, and a nil link mutates nothing.
func TestSetLinkPriorityGapFills(t *testing.T) {
	assert.False(t, SetLinkPriority(nil, 90, false), "no link mutates nothing")
	assert.False(t, SetLinkPriority(&projectfile.Link{}, PriorityDefault, false),
		"the default rank is not a proposal")

	blank := projectfile.Link{}
	assert.True(t, SetLinkPriority(&blank, 90, false), "a link with no Extra map still takes the write")
	assert.Equal(t, 90, LinkPriority(blank))

	l := projectfile.Link{Extra: map[string]any{}}
	assert.True(t, SetLinkPriority(&l, 90, false))
	assert.Equal(t, 90, LinkPriority(l))

	declared := projectfile.Link{Extra: map[string]any{keyPriority: 10}}
	assert.False(t, SetLinkPriority(&declared, 90, false), "a declared key wins without force")
	assert.Equal(t, 10, LinkPriority(declared))
	assert.True(t, SetLinkPriority(&declared, 90, true), "force overrides")
	assert.Equal(t, 90, LinkPriority(declared))
}

// TestTagsFromAcceptsBothEncoderShapes pins the tolerance two readers of one convention must share: absent, wrong type, mixed items and empty strings.
func TestTagsFromAcceptsBothEncoderShapes(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, TagsFrom([]string{"a", "b"}))
	assert.Equal(t, []string{"a", "b"}, TagsFrom([]any{"a", "b"}))
	assert.Equal(t, []string{"a"}, TagsFrom([]any{"a", "", 3, nil}), "an empty or mistyped item is dropped")
	assert.Nil(t, TagsFrom(nil), "an absent value carries no capability")
	assert.Nil(t, TagsFrom("a"), "a scalar is not a tag list")
	assert.Nil(t, TagsFrom(map[string]any{"a": true}))
}

// TestMergeTagUnionsInDeclarationOrder pins the idempotence a proposal needs: re-proposing a tag the link already carries changes nothing.
func TestMergeTagUnionsInDeclarationOrder(t *testing.T) {
	assert.Equal(t, []string{TagPublic}, mergeTag(nil, TagPublic))
	assert.Equal(t, []string{TagPublic, TagBadges}, mergeTag([]string{TagPublic}, TagBadges))
	assert.Equal(t, []string{TagPublic, TagBadges}, mergeTag([]string{TagPublic, TagBadges}, TagPublic),
		"an already-declared tag is not appended twice")
}

// TestSetMainDocumentationURLPinsTheTagSelection pins the single documentation slot: the tag names THE entry, and a project with neither gains one.
func TestSetMainDocumentationURLPinsTheTagSelection(t *testing.T) {
	assert.False(t, SetMainDocumentationURL(nil, "https://docs.test", false), "no document mutates nothing")
	assert.False(t, SetMainDocumentationURL(&projectfile.Document{}, "", false), "no URL mutates nothing")

	// The URL already sits on a documentation entry: it only gains the tag.
	existing := &projectfile.Document{Links: []projectfile.Link{
		{Type: linkTypeDocumentation, URL: "https://docs.test", Extra: map[string]any{keyTags: []any{TagPublic}}},
		{Type: LinkSourceCode, URL: urlOrigin},
	}}
	require.True(t, SetMainDocumentationURL(existing, "https://docs.test", false))
	assert.Equal(t, []string{TagPublic, TagMainDocumentation}, LinkTags(existing.Links[0]),
		"a curated tag list is merged, never replaced")
	assert.Equal(t, "https://docs.test", MainDocumentationURL(existing))
	assert.False(t, SetMainDocumentationURL(existing, "https://docs.test", false), "a second call is a no-op")

	// An untagged documentation entry is invisible to the selector.
	untagged := &projectfile.Document{Links: []projectfile.Link{
		{Type: linkTypeDocumentation, URL: "https://supplementary.test"},
	}}
	assert.Empty(t, MainDocumentationURL(untagged))

	// A main-tagged entry takes the URL under the gap/force rule, and the supplementary documentation links around it are skipped by name.
	blanked := &projectfile.Document{Links: []projectfile.Link{
		{Type: linkTypeDocumentation, URL: "https://supplementary.test"},
		{Type: LinkSourceCode, URL: urlOrigin},
		{Type: linkTypeDocumentation, Extra: map[string]any{keyTags: []any{TagMainDocumentation}}},
	}}
	require.True(t, SetMainDocumentationURL(blanked, "https://docs.test", false))
	assert.Equal(t, "https://docs.test", MainDocumentationURL(blanked))

	declared := &projectfile.Document{Links: []projectfile.Link{
		{Type: linkTypeDocumentation, URL: "https://old.test", Extra: map[string]any{keyTags: []any{TagMainDocumentation}}},
	}}
	assert.False(t, SetMainDocumentationURL(declared, "https://new.test", false), "a declared URL is not moved without force")
	assert.True(t, SetMainDocumentationURL(declared, "https://new.test", true), "force moves it")
	assert.Equal(t, "https://new.test", MainDocumentationURL(declared))

	// Nothing declared at all: a fresh tagged entry appears.
	fresh := &projectfile.Document{}
	require.True(t, SetMainDocumentationURL(fresh, "https://docs.test", false))
	require.Len(t, fresh.Links, 1)
	assert.Equal(t, []string{TagMainDocumentation}, LinkTags(fresh.Links[0]))
}
