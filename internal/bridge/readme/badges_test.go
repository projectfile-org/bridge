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
	got := buildBadges(nil, ext, "")
	require.Len(t, got, 2)
	assert.Equal(t, "dockerhub pulls", got[0].Alt, "alt falls back to name")
	assert.Equal(t, "Go Report", got[1].Alt, "explicit alt preserved")
}

// No ext (readme extension absent entirely) → no badges, never a nil panic.
func TestBuildBadgesNilExtension(t *testing.T) {
	assert.Nil(t, buildBadges(nil, nil, ""))
	assert.Nil(t, buildBadges(nil, &pfmodel.ReadmeExtension{}, ""))
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

	got := buildBadges(pf, ext, "")

	require.Len(t, got, 1, "the codeberg badge has no mirror to resolve against")
	assert.Equal(t, blockLicense, got[0].Alt)
	assert.Equal(t, "https://own.example/license", got[0].Img, "the base document's redeclaration wins")
}

// The m6e fleet's row names, used across the row tests as both the declared
// value and the expected grouping key, plus the one badge name that recurs
// across them.
const (
	rowStatic      = "static"
	rowDynamic     = "dynamic"
	rowEcosystem   = "ecosystem"
	nameStatus     = "status"
	nameReuse      = "reuse"
	nameNpm        = "npm"
	nameSupportUkr = "support-ukraine"
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
			{Name: nameReuse, Img: "i3", Row: rowStatic},
			{Name: nameNpm, Img: "i4", Row: rowEcosystem},
		},
	}

	rows := buildBadgeRows(nil, ext, "")

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

	rows := buildBadgeRows(nil, ext, "")

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

	rows := buildBadgeRows(nil, ext, "")

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

// Priority orders badges WITHIN a row: higher renders first. This is the
// support-ukraine case — a fleet-wide badge pinned to the head of the static
// row from one entry, without reordering the rows themselves.
func TestBuildBadgeRowsPriorityOrdersWithinRow(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "i1", Row: rowStatic},
			{Name: nameSupportUkr, Img: "i2", Row: rowStatic, Priority: 300},
			{Name: nameReuse, Img: "i3", Row: rowStatic},
		},
	}

	rows := buildBadgeRows(nil, ext, "")

	require.Len(t, rows, 1)
	got := []string{rows[0].Badges[0].Alt, rows[0].Badges[1].Alt, rows[0].Badges[2].Alt}
	assert.Equal(t, []string{nameSupportUkr, blockLicense, nameReuse}, got,
		"priority 300 rises to the head; the two default-50 badges keep declaration order")
}

// Priority is stable: equal priorities (including every unset one at
// PriorityDefault) keep declaration order, so a project that never sets
// priority renders byte-identical to the pre-priority layout.
func TestBuildBadgeRowsPriorityStableForTies(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: "first", Img: "i1", Row: rowStatic},
			{Name: "second", Img: "i2", Row: rowStatic},
			{Name: "third", Img: "i3", Row: rowStatic},
		},
	}

	rows := buildBadgeRows(nil, ext, "")

	require.Len(t, rows, 1)
	got := []string{rows[0].Badges[0].Alt, rows[0].Badges[1].Alt, rows[0].Badges[2].Alt}
	assert.Equal(t, []string{"first", "second", "third"}, got,
		"all-default badges keep declaration order")
}

// Priority only reorders peers INSIDE a row; row order and first-appearance
// grouping are untouched. A pinned badge in the ecosystem row does not leap
// into the static row.
func TestBuildBadgeRowsPriorityStaysWithinRow(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "i1", Row: rowStatic},
			{Name: nameNpm, Img: "i2", Row: rowEcosystem, Priority: 999},
		},
	}

	rows := buildBadgeRows(nil, ext, "")

	require.Len(t, rows, 2)
	assert.Equal(t, []string{rowStatic, rowEcosystem}, []string{rows[0].Name, rows[1].Name},
		"row order unchanged even when a later row holds the highest priority")
}

// A shield's href resolves per render language: HrefByLang wins when the
// active language has an entry, so a shared badge (conventional commits,
// semver) can link a Ukrainian render to the Ukrainian spec page instead of
// always the English one. Href stays the fallback for a language the map
// carries no entry for.
func TestBuildBadgesHrefResolvesPerLang(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{
				Name: "commit-style", Img: "i", Href: "https://example.test/en/",
				HrefByLang: map[string]string{"uk": "https://example.test/uk/"},
			},
		},
	}

	assert.Equal(t, "https://example.test/uk/", buildBadges(nil, ext, "uk")[0].Href, "uk entry wins")
	assert.Equal(t, "https://example.test/en/", buildBadges(nil, ext, "es")[0].Href, "no es entry: falls back to Href")
	assert.Equal(t, "https://example.test/en/", buildBadges(nil, ext, "")[0].Href, "default render: falls back to Href")
}

// A project publishing several containers from one matrix (b19/ruby's four
// Ruby series, each its own Docker Hub repository) has no single flatpath to
// badge. A shield naming the `{AXIS}` placeholder fans out to one badge per
// declared series value, img and href paired from the SAME cell — never
// cross-multiplied into wrong pairs.
func TestBuildBadgesFansOutPerSeriesAxis(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyMatrix: map[string]any{keyAxes: map[string]any{
				axisSeries: []any{seriesResolute, seriesNoble},
			}},
		},
	}
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{
				Name: "dockerhub-pulls",
				Img:  "https://img.shields.io/docker/pulls/foo-{" + axisSeries + "}",
				Href: "https://hub.docker.com/r/foo-{" + axisSeries + "}",
			},
		},
	}

	got := buildBadges(pf, ext, "")

	require.Len(t, got, 2, "one badge per declared series")
	assert.Equal(t, "https://img.shields.io/docker/pulls/foo-resolute", got[0].Img)
	assert.Equal(t, "https://hub.docker.com/r/foo-resolute", got[0].Href, "href paired with the same cell as img")
	assert.Equal(t, "https://img.shields.io/docker/pulls/foo-noble", got[1].Img)
	assert.Equal(t, "https://hub.docker.com/r/foo-noble", got[1].Href)
}

// A redeclared shield name MOVES to the row the redeclaration names AND adopts
// the redeclaration's priority — consistent with the existing last-wins dedup.
func TestBuildBadgeRowsRedeclarationAdoptsPriority(t *testing.T) {
	ext := &pfmodel.ReadmeExtension{
		Shields: []pfmodel.Shield{
			{Name: blockLicense, Img: "i1", Row: rowStatic},
			{Name: nameStatus, Img: "i2", Row: rowDynamic, Priority: 10},
			{Name: blockLicense, Img: "i1b", Row: rowStatic, Priority: 400},
		},
	}

	rows := buildBadgeRows(nil, ext, "")

	require.Len(t, rows, 2)
	assert.Equal(t, rowStatic, rows[0].Name)
	require.Len(t, rows[0].Badges, 1, "license is deduped last-wins; the static row holds only the redeclaration")
	assert.Equal(t, blockLicense, rows[0].Badges[0].Alt)
	assert.Equal(t, "i1b", rows[0].Badges[0].Img, "last-wins value preserved")
	assert.Equal(t, 400, rows[0].Badges[0].Priority, "redeclared priority adopted")
}
