// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package contributing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Guards covers the defensive contracts Render can never reach: it always gets
// a live document, every follow-block caller asks for defaultAllow=false, and a
// source-code link with an empty URL is skipped before host parsing, so these
// branches exist for every other caller and are driven here directly.
func TestAuthorCollectorsTolerateANilDocument(t *testing.T) {
	assert.Nil(t, resolveAuthorFollows(nil))
	assert.Nil(t, resolveAuthorSites(nil))
	assert.Nil(t, resolveProjectSocials(nil))
	assert.Nil(t, resolveForgeStars(nil))
}

// TestFilterFollowsRestsOnTheCallerDefault pins the unset-toggle contract: the
// caller owns its resting state, so nothing is dropped when it allows all.
func TestFilterFollowsRestsOnTheCallerDefault(t *testing.T) {
	in := []followLink{{Key: "site", URL: "https://dbuho.me"}}
	got := filterFollows(in, projectfile.Toggle{}, true)
	assert.Equal(t, in, got)
	assert.Nil(t, filterFollows(in, projectfile.Toggle{}, false))
}

// TestForgeHostFromURLEmptyInput pins the empty-URL guard before url.Parse.
func TestForgeHostFromURLEmptyInput(t *testing.T) {
	assert.Empty(t, forgeHostFromURL(""))
	assert.Equal(t, "dbuho.me", forgeHostFromURL("https://dbuho.me/x"))
}
