// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scancmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// testApplyLinkURL is the (type,url) pair reused across every applyLink case;
// hoisted to a const so goconst does not trip over the repeated literal.
const testApplyLinkURL = "https://github.com/acme/proj"

// tagsKey is pfmodel's §139 tags key, spelled here because pfmodel keeps its
// own copy private — SetLinkTags is the only writer by design.
const tagsKey = "tags"

// TestApplyLinkGapFillsPreferred guards the rescan path: when an existing
// source-code link lacks Preferred and the scanner resupplies it (origin is
// now marked Preferred), the flag is gap-filled — but an existing
// Preferred:true is never downgraded.
func TestApplyLinkGapFillsPreferred(t *testing.T) {
	t.Run("upgrades_nonpreferred_to_preferred", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{
				{Type: projectfile.LinkSourceCode, URL: testApplyLinkURL},
			},
		}
		applyLink(doc, projectfile.Link{
			Type:      projectfile.LinkSourceCode,
			URL:       testApplyLinkURL,
			Preferred: true,
		}, false)
		require.Len(t, doc.Links, 1)
		assert.True(t, doc.Links[0].Preferred, "existing non-preferred link must be upgraded")
	})

	t.Run("keeps_existing_preferred_when_incoming_not", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{
				{Type: projectfile.LinkSourceCode, URL: testApplyLinkURL, Preferred: true},
			},
		}
		applyLink(doc, projectfile.Link{
			Type:      projectfile.LinkSourceCode,
			URL:       testApplyLinkURL,
			Preferred: false,
		}, false)
		require.Len(t, doc.Links, 1)
		assert.True(t, doc.Links[0].Preferred, "existing preferred must NOT be downgraded")
	})

	t.Run("appends_new_pair_with_preferred", func(t *testing.T) {
		doc := &projectfile.Document{}
		applyLink(doc, projectfile.Link{
			Type:      projectfile.LinkSourceCode,
			URL:       testApplyLinkURL,
			Preferred: true,
		}, false)
		require.Len(t, doc.Links, 1)
		assert.True(t, doc.Links[0].Preferred, "appended link keeps its Preferred")
	})

	t.Run("no_op_when_both_unpreferred", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{
				{Type: projectfile.LinkSourceCode, URL: testApplyLinkURL},
			},
		}
		applyLink(doc, projectfile.Link{
			Type: projectfile.LinkSourceCode,
			URL:  testApplyLinkURL,
		}, false)
		require.Len(t, doc.Links, 1)
		assert.False(t, doc.Links[0].Preferred)
	})
}

// TestApplyLinkGapFillsTags guards the rescan path for capability tags: a
// link the user has never tagged gets the scanner's proposal, and a link that
// already declares tags keeps exactly what it declared. Same rule as
// Preferred — existing values win.
func TestApplyLinkGapFillsTags(t *testing.T) {
	proposed := projectfile.Link{
		Type:  projectfile.LinkSourceCode,
		URL:   testApplyLinkURL,
		Extra: map[string]any{tagsKey: []string{pfmodel.TagPublic, pfmodel.TagBadges}},
	}

	t.Run("fills_untagged_link", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{{Type: projectfile.LinkSourceCode, URL: testApplyLinkURL}},
		}
		applyLink(doc, proposed, false)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, []string{pfmodel.TagPublic, pfmodel.TagBadges}, pfmodel.LinkTags(doc.Links[0]))
	})

	t.Run("keeps_curated_tags", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{{
				Type:  projectfile.LinkSourceCode,
				URL:   testApplyLinkURL,
				Extra: map[string]any{tagsKey: []any{"ci"}},
			}},
		}
		applyLink(doc, proposed, false)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, []string{"ci"}, pfmodel.LinkTags(doc.Links[0]),
			"a curated tag list must survive a rescan untouched")
	})

	t.Run("appended_link_keeps_its_tags", func(t *testing.T) {
		doc := &projectfile.Document{}
		applyLink(doc, proposed, false)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, []string{pfmodel.TagPublic, pfmodel.TagBadges}, pfmodel.LinkTags(doc.Links[0]))
	})
}

// TestApplyLinkForceRetags guards the escape hatch. Gap-fill alone makes the
// first fleet-wide scan irreversible: 152 projectfiles would hold whatever
// vocabulary shipped first, and no later run could correct them. --force is
// how a vocabulary change reaches a fleet that already declared one.
func TestApplyLinkForceRetags(t *testing.T) {
	incoming := projectfile.Link{
		Type:  projectfile.LinkSourceCode,
		URL:   testApplyLinkURL,
		Extra: map[string]any{tagsKey: []string{pfmodel.TagPublic, pfmodel.TagBadgesGitHub}},
	}

	t.Run("overwrites_declared_tags", func(t *testing.T) {
		doc := &projectfile.Document{
			Links: []projectfile.Link{{
				Type:  projectfile.LinkSourceCode,
				URL:   testApplyLinkURL,
				Extra: map[string]any{tagsKey: []any{"badges"}},
			}},
		}
		applyLink(doc, incoming, true)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, []string{pfmodel.TagPublic, pfmodel.TagBadgesGitHub},
			pfmodel.LinkTags(doc.Links[0]))
	})

	t.Run("leaves_label_and_preferred_alone", func(t *testing.T) {
		kept := &projectfile.LocalizedString{Bare: "Curated label"}
		doc := &projectfile.Document{
			Links: []projectfile.Link{{
				Type:      projectfile.LinkSourceCode,
				URL:       testApplyLinkURL,
				Label:     kept,
				Preferred: true,
				Extra:     map[string]any{tagsKey: []any{"badges"}},
			}},
		}
		applyLink(doc, projectfile.Link{
			Type:  projectfile.LinkSourceCode,
			URL:   testApplyLinkURL,
			Label: &projectfile.LocalizedString{Bare: "Source Code on GitHub"},
			Extra: map[string]any{tagsKey: []string{pfmodel.TagPublic}},
		}, true)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, kept, doc.Links[0].Label, "--force must not touch a curated label")
		assert.True(t, doc.Links[0].Preferred, "--force must not touch preferred")
		assert.Equal(t, []string{pfmodel.TagPublic}, pfmodel.LinkTags(doc.Links[0]))
	})
}
