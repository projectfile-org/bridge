// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// execBadgesTemplate parses and executes the embedded badges.tmpl through the
// real FuncMap path (execBlockTemplate), matching how renderBlock runs every
// block. Pass the readme extension to drive the `badgeRows` FuncMap.
func execBadgesTemplate(t *testing.T, ext *pfmodel.ReadmeExtension) string {
	t.Helper()
	body, err := templatesFS.ReadFile("templates/readme.md/badges.tmpl")
	require.NoError(t, err)
	data := readmeView{}
	rendered, err := execBlockTemplate("badges", body, data, "", ext)
	require.NoError(t, err)
	return string(rendered)
}

// A badge with explicit alt renders [![alt](img)](href); a badge without alt
// falls back to its name. This is the user-visible contract.
func TestBadgesTemplateRendersShields(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: "Docker Hub pulls", Alt: "Docker Hub pulls", Img: "https://img.shields.io/docker/pulls/foo", Href: "https://hub.docker.com/r/foo"},
			{Name: "go report", Alt: "Go Report", Img: "https://goreportcard.com/badge/foo", Href: "https://goreportcard.com/report/foo"},
		},
	}
	out := execBadgesTemplate(t, ext)
	assert.Contains(t, out, "[![Docker Hub pulls](https://img.shields.io/docker/pulls/foo)](https://hub.docker.com/r/foo)")
	assert.Contains(t, out, "[![Go Report](https://goreportcard.com/badge/foo)](https://goreportcard.com/report/foo)")
}

// buildBadges must default Alt to Name so a missing alt never yields an
// empty `![ ](...)`, which would render as a broken/empty alt badge.
func TestBuildBadgesAltFallsBackToName(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: "dockerhub pulls", Img: "i", Href: "h"}, // alt omitted
			{Name: "go report", Alt: "Go Report", Img: "i2", Href: "h2"},
		},
	}
	got := buildBadges(nil, ext)
	require.Len(t, got, 2)
	assert.Equal(t, "dockerhub pulls", got[0].Alt, "alt falls back to name")
	assert.Equal(t, "Go Report", got[1].Alt, "explicit alt preserved")
}

// No ext (readme extension absent entirely) → no badges, never a nil panic.
func TestBuildBadgesNilExtension(t *testing.T) {
	assert.Nil(t, buildBadges(nil, nil))
	assert.Nil(t, buildBadges(nil, &pfmodel.ReadmeExtension{}))
}

// A shared fragment declares badges for forges a given project may not have.
// A shield whose URL still carries an unresolved ${…} is dropped rather than
// published as a broken image; the resolvable ones survive.
func TestBuildBadgesDropsUnresolvedAndDedupes(t *testing.T) {
	pf := minimalDoc(t)
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "https://badges.example/license-${license.spdx}", Href: "https://example.test/LICENSE"},
			{Name: "codeberg", Img: "${org.projectfile.forge.remotes.codeberg.url}/badge", Href: "https://example.test"},
			{Name: blockLicense, Img: "https://own.example/license", Href: "https://own.example/LICENSE"},
		},
	}

	got := buildBadges(pf, ext)

	require.Len(t, got, 1, "the codeberg badge has no mirror to resolve against")
	assert.Equal(t, blockLicense, got[0].Alt)
	assert.Equal(t, "https://own.example/license", got[0].Img, "the base document's redeclaration wins")
}

// The m6e fleet's row names, used across the row tests as both the declared
// value and the expected grouping key, plus the one badge name that recurs
// across them.
const (
	rowStatic    = "static"
	rowDynamic   = "dynamic"
	rowEcosystem = "ecosystem"
	nameStatus   = "status"
)

// Badges group into rendered LINES by `row`, and the row order is the order
// each name is first seen — not declaration order of the badges themselves.
// The third entry rejoins the first row although a second row opened between
// them, which is what lets an opt-in fragment append a static badge without
// disturbing the layout.
func TestBuildBadgeRowsGroupsByFirstAppearance(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "i1", Row: rowStatic},
			{Name: nameStatus, Img: "i2", Row: rowDynamic},
			{Name: "reuse", Img: "i3", Row: rowStatic},
			{Name: "npm", Img: "i4", Row: rowEcosystem},
		},
	}

	rows := buildBadgeRows(nil, ext)

	require.Len(t, rows, 3)
	assert.Equal(t, []string{rowStatic, rowDynamic, rowEcosystem}, []string{rows[0].Name, rows[1].Name, rows[2].Name})
	require.Len(t, rows[0].Badges, 2, "reuse rejoins the static row")
	assert.Equal(t, "i3", rows[0].Badges[1].Img)
}

// A shield with no `row` groups like any other — one unnamed row. This is the
// pre-rows shape, so a document that never heard of rows renders one line.
func TestBuildBadgeRowsUnnamedRow(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{{Name: "a", Img: "i1"}, {Name: "b", Img: "i2"}},
	}

	rows := buildBadgeRows(nil, ext)

	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].Name)
	assert.Len(t, rows[0].Badges, 2)
}

// Grouping runs after the name dedup, so redeclaring a name in a later
// fragment also MOVES the badge to the row that redeclaration names — the only
// way to re-file an inherited badge, since includes cannot delete.
func TestBuildBadgeRowsRedeclarationMovesRow(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "i1", Row: rowStatic},
			{Name: nameStatus, Img: "i2", Row: rowDynamic},
			{Name: nameStatus, Img: "i2b", Row: rowStatic},
		},
	}

	rows := buildBadgeRows(nil, ext)

	require.Len(t, rows, 1, "nothing is left in the dynamic row")
	assert.Equal(t, rowStatic, rows[0].Name)
	assert.Equal(t, "i2b", rows[0].Badges[1].Img)
}

// Rows render as separate markdown PARAGRAPHS. A single newline would be a
// soft break — the two rows would collapse onto one visual line — so the blank
// line between them is the user-visible contract.
func TestBadgesTemplateSeparatesRowsWithBlankLine(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Alt: "License", Img: "https://example.test/l.svg", Href: "LICENSE", Row: rowStatic},
			{Name: nameStatus, Alt: "Status", Img: "https://example.test/s.svg", Row: rowDynamic},
		},
	}

	out := execBadgesTemplate(t, ext)

	assert.Equal(t,
		"[![License](https://example.test/l.svg)](LICENSE)\n\n![Status](https://example.test/s.svg)\n",
		out)
}

// No badges → empty render → the block is silently dropped by renderBlock's
// TrimSpace check. Guard that contract so future template edits don't emit
// stray whitespace for badge-less projects.
func TestBadgesTemplateEmptyWhenNoShields(t *testing.T) {
	out := execBadgesTemplate(t, nil)
	assert.Empty(t, strings.TrimSpace(out))
}
