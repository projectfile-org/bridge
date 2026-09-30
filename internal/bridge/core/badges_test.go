// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// The badge the document-routing tests move between documents.
const (
	badgeCommits    = "commits"
	badgeCommitsImg = "https://img.test/" + badgeCommits
)

// TestBadgesRenderOnTheirDocument: a shield with no documents stays on the readme, one naming contributing leaves it.
func TestBadgesRenderOnTheirDocument(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: "license", Img: "https://img.test/license"},
		{Name: badgeCommits, Img: badgeCommitsImg, Documents: []string{"contributing"}},
	}
	assert.Equal(t, "![license](https://img.test/license)", BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "")))
	assert.Equal(t, "![commits](https://img.test/commits)", BadgeMarkdown(BadgeRows(nil, shields, "contributing", "")))
}

// TestRedeclaredBadgeMovesDocument: redeclaring an inherited name with documents moves the badge, it does not copy it.
func TestRedeclaredBadgeMovesDocument(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: badgeCommits, Img: badgeCommitsImg},
		{Name: badgeCommits, Img: badgeCommitsImg, Href: "https://cc.test", Documents: []string{"contributing"}},
	}
	assert.Empty(t, Badges(nil, shields, DocumentReadme, ""))
	assert.Equal(t, "[![commits](https://img.test/commits)](https://cc.test)", BadgeMarkdown(BadgeRows(nil, shields, "contributing", "")))
}
