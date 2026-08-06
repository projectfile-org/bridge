// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package contributing_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/contributing"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	testRecommendToStar = "recommend-to-star"
	testMastodon        = "mastodon"
	testMastodonURL     = "https://m.to/@a"
	testGitHub          = "github"
	testAuthor          = "testauthor"
)

func docWithSourceCodeLinks(urls ...string) *projectfile.Document {
	links := make([]projectfile.Link, 0, len(urls))
	for _, u := range urls {
		links = append(links, projectfile.Link{Type: "source-code", URL: u})
	}
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "multi-forge-proj"},
		Links:    links,
	}
}

// ── Forge star block ───────────────────────────────────────────────────────

// TestRenderStarLinePerForge is the regression for the bug where the
// "Star the project" appreciation block named only the primary forge. A
// project mirrored to several forges must list one star line per
// links[type=source-code] entry. recommend-to-star must be opted in.
func TestRenderStarLinePerForge(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithSourceCodeLinks(
		"https://codeberg.org/projectfile/cli",
		"https://github.com/damian-buho/projectfile-cli",
		"https://kiota.ch/projectfile/cli",
	), map[string]any{testRecommendToStar: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "Star the project on [codeberg.org](https://codeberg.org/projectfile/cli)")
	assert.Contains(t, body, "Star the project on [github.com](https://github.com/damian-buho/projectfile-cli)")
	assert.Contains(t, body, "Star the project on [kiota.ch](https://kiota.ch/projectfile/cli)")
}

// TestRenderStarEnabledNoForgesBareLine confirms the template degrades to a
// bare "Star the project" line (no link) when stars are enabled but no
// source-code link is declared.
func TestRenderStarEnabledNoForgesBareLine(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(&projectfile.Document{Identity: projectfile.Identity{Name: "no-forge"}},
		map[string]any{testRecommendToStar: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "> - Star the project")
	assert.NotContains(t, body, "Star the project on")
}

// TestRenderStarDisabledNoBlock confirms that when recommend-to-star is off
// (here: unset) the bare "Star the project" line does NOT render either.
func TestRenderStarDisabledNoBlock(t *testing.T) {
	b := contributing.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "no-forge"}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.NotContains(t, body, "Star the project",
		"unset recommend-to-star must suppress the whole star block")
}

// TestRenderStarLineDedup guards against duplicate star lines when several
// includes merge in the same repository URL.
func TestRenderStarLineDedup(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithSourceCodeLinks(
		"https://codeberg.org/projectfile/cli",
		"https://codeberg.org/projectfile/cli",
	), map[string]any{testRecommendToStar: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Equal(t, 1, strings.Count(body, "Star the project on [codeberg.org]"),
		"duplicate source-code URLs must collapse to one star line")
}

// ── Bridge identity ─────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	assert.Equal(t, "CONTRIBUTING.md", contributing.Bridge{}.Filename())
}

func TestBridgePolicyScaffoldOnce(t *testing.T) {
	assert.True(t, contributing.Bridge{}.Policy().ScaffoldOnce,
		"CONTRIBUTING.md must use ScaffoldOnce policy")
}

// withContributing attaches an org.projectfile.contributing extension to pf.
func withContributing(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.ContributingExtensionNS, ext)
	}
	return pf
}

func docWithAuthor(handles map[string]any) *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "follow-proj"},
		People: []projectfile.Person{{
			GivenNames:  "Test",
			FamilyNames: "Author",
			Roles:       []string{"author"},
			Handles:     handles,
		}},
	}
}

// ── recommend-to-star toggle (default: no recommendation) ───────────────────

func TestStarToggleUnsetHidesAll(t *testing.T) {
	b := contributing.Bridge{}
	pf := docWithSourceCodeLinks("https://codeberg.org/o/r", "https://github.com/o/r")
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.NotContains(t, body, "Star the project on",
		"unset recommend-to-star must show no star lines")
}

func TestStarToggleBoolTrueShowsAll(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithSourceCodeLinks(
		"https://codeberg.org/o/r", "https://github.com/o/r"),
		map[string]any{testRecommendToStar: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "Star the project on [codeberg.org]")
	assert.Contains(t, body, "Star the project on [github.com]")
}

func TestStarToggleBoolFalseHidesAll(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithSourceCodeLinks(
		"https://codeberg.org/o/r", "https://github.com/o/r"),
		map[string]any{testRecommendToStar: false})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.NotContains(t, body, "Star the project on")
}

func TestStarToggleMapAllowlist(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithSourceCodeLinks(
		"https://codeberg.org/o/r",
		"https://github.com/o/r",
		"https://kiota.ch/o/r"),
		map[string]any{"recommend-to-star": map[string]any{
			"github.com":   true,
			"kiota.ch":     false,
			"codeberg.org": true,
		}})

	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "Star the project on [github.com]")
	assert.Contains(t, body, "Star the project on [codeberg.org]")
	assert.NotContains(t, body, "Star the project on [kiota.ch]", "false key must be hidden")
}

// ── recommend-to-follow toggle (default: no contacts) ───────────────────────

func TestFollowToggleUnsetHidesAll(t *testing.T) {
	b := contributing.Bridge{}
	pf := docWithAuthor(map[string]any{
		testMastodon: testMastodonURL,
		testGitHub:   testAuthor,
	})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.NotContains(t, body, "Follow [author")
}

func TestFollowToggleBoolTrueShowsAll(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithAuthor(map[string]any{
		testMastodon: testMastodonURL,
		testGitHub:   testAuthor,
	}), map[string]any{"recommend-to-follow": true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "Follow [author (Test Author) on Mastodon]")
	assert.Contains(t, body, "Follow [author (Test Author) on GitHub]")
}

func TestFollowToggleMapAllowlist(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(docWithAuthor(map[string]any{
		testMastodon: testMastodonURL,
		testGitHub:   testAuthor,
		"codeberg":   testAuthor,
	}), map[string]any{"recommend-to-follow": map[string]any{
		testMastodon: true,
		testGitHub:   false,
	}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "Follow [author (Test Author) on Mastodon]")
	assert.NotContains(t, body, "on GitHub]", "false key must be hidden")
	assert.NotContains(t, body, "on Codeberg]", "absent key must be hidden in allowlist mode")
}
