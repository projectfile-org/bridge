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

// testTagLinkURL is the mirror every case tags; hoisted so goconst does not
// trip over the repeated literal.
const testTagLinkURL = "https://codeberg.org/acme/proj"

// TestSetLinkTagsGapFill pins the one rule that decides whether a rescan
// argues with the user: the gap is an ABSENT tags key, not an empty list.
func TestSetLinkTagsGapFill(t *testing.T) {
	proposal := []string{TagPublic, TagBadges}

	t.Run("writes_when_no_tags_key", func(t *testing.T) {
		l := projectfile.Link{Type: LinkSourceCode, URL: testTagLinkURL}
		require.True(t, SetLinkTags(&l, proposal, false))
		assert.Equal(t, proposal, LinkTags(l))
	})

	t.Run("keeps_curated_list", func(t *testing.T) {
		l := projectfile.Link{
			Type:  LinkSourceCode,
			URL:   testTagLinkURL,
			Extra: map[string]any{keyTags: []any{"ci"}},
		}
		assert.False(t, SetLinkTags(&l, proposal, false))
		assert.Equal(t, []string{"ci"}, LinkTags(l), "a declared list must never be widened")
	})

	t.Run("keeps_deliberately_empty_list", func(t *testing.T) {
		l := projectfile.Link{
			Type:  LinkSourceCode,
			URL:   "https://gitlab.internal/acme/proj",
			Extra: map[string]any{keyTags: []any{}},
		}
		assert.False(t, SetLinkTags(&l, proposal, false),
			"tags: [] states this mirror affords nothing — a rescan must not overrule it")
		assert.Empty(t, LinkTags(l))
	})

	t.Run("keeps_malformed_list", func(t *testing.T) {
		l := projectfile.Link{
			Type:  LinkSourceCode,
			URL:   testTagLinkURL,
			Extra: map[string]any{keyTags: "public"},
		}
		assert.False(t, SetLinkTags(&l, proposal, false),
			"a malformed value is still the user's, and overwriting it would hide the mistake")
	})

	t.Run("no_op_on_empty_proposal", func(t *testing.T) {
		l := projectfile.Link{Type: LinkSourceCode, URL: "https://kiota.ch/acme/proj"}
		assert.False(t, SetLinkTags(&l, nil, false))
		assert.Nil(t, l.Extra, "an unknown host must not even create the Extra map")
	})

	t.Run("nil_link", func(t *testing.T) {
		assert.False(t, SetLinkTags(nil, proposal, false))
	})
}
