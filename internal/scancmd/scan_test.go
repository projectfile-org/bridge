// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scancmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// testApplyLinkURL is the (type,url) pair reused across every applyLink case;
// hoisted to a const so goconst does not trip over the repeated literal.
const testApplyLinkURL = "https://github.com/acme/proj"

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
		})
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
		})
		require.Len(t, doc.Links, 1)
		assert.True(t, doc.Links[0].Preferred, "existing preferred must NOT be downgraded")
	})

	t.Run("appends_new_pair_with_preferred", func(t *testing.T) {
		doc := &projectfile.Document{}
		applyLink(doc, projectfile.Link{
			Type:      projectfile.LinkSourceCode,
			URL:       testApplyLinkURL,
			Preferred: true,
		})
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
		})
		require.Len(t, doc.Links, 1)
		assert.False(t, doc.Links[0].Preferred)
	})
}
