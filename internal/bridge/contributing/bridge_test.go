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
	testSourceCode      = "source-code"
)

func docWithSourceCodeLinks(urls ...string) *projectfile.Document {
	links := make([]projectfile.Link, 0, len(urls))
	for _, u := range urls {
		links = append(links, projectfile.Link{Type: testSourceCode, URL: u})
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

// TestRenderStarLinePriorityOrdersForges: a §139 `priority` on a source-code
// link reorders the "Star the project" lines (higher first). The pinned forge
// rises to the head of the list.
func TestRenderStarLinePriorityOrdersForges(t *testing.T) {
	b := contributing.Bridge{}
	pf := withContributing(&projectfile.Document{
		Identity: projectfile.Identity{Name: "multi-forge-proj"},
		Links: []projectfile.Link{
			{Type: testSourceCode, URL: "https://codeberg.org/projectfile/cli"},
			{
				Type: testSourceCode, URL: "https://kiota.ch/projectfile/cli",
				Extra: map[string]any{"priority": 300},
			},
			{Type: testSourceCode, URL: "https://github.com/damian-buho/projectfile-cli"},
		},
	}, map[string]any{testRecommendToStar: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])

	kiotaIdx := strings.Index(body, "Star the project on [kiota.ch]")
	codebergIdx := strings.Index(body, "Star the project on [codeberg.org]")
	require.NotEqual(t, -1, kiotaIdx)
	require.NotEqual(t, -1, codebergIdx)
	assert.Less(t, kiotaIdx, codebergIdx,
		"higher-priority kiota.ch forge renders before the default-priority codeberg one")
}

// ── Bridge identity ─────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	assert.Equal(t, "CONTRIBUTING.md", contributing.Bridge{}.Filename())
}

func TestBridgePolicyScaffoldOnce(t *testing.T) {
	assert.True(t, contributing.Bridge{}.Policy().ScaffoldOnce,
		"CONTRIBUTING.md must use ScaffoldOnce policy")
}

// TestRenderCarriesTheSentinel pins the rule for a file the write gate never
// overwrites on its own: the sentinel is a warning to the human reader, so a
// scaffold-once artefact carries it exactly like a marker-policy one.
func TestRenderCarriesTheSentinel(t *testing.T) {
	out, err := contributing.Bridge{}.Render(docWithSourceCodeLinks(), core.Options{Offline: true})
	require.NoError(t, err)
	body := out.Files["CONTRIBUTING.md"]
	assert.Contains(t, string(body), core.MarkerInner+"\n-->",
		"the sentinel must be folded into the SPDX header")
	assert.True(t, core.HasMarker(body), "the header must read as managed")
}

// withContributing attaches an org.projectfile.contributing extension to pf.
func withContributing(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.ContributingExtensionNS, ext)
	}
	return pf
}

// withConventions attaches an org.projectfile.conventions extension to pf.
func withConventions(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.ConventionsExtensionNS, ext)
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
		"https://codeberg.org/o/r", "https://github.com/o/r",
	),
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
		"https://codeberg.org/o/r", "https://github.com/o/r",
	),
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
		"https://kiota.ch/o/r",
	),
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

// ── Conventions section ─────────────────────────────────────────────────────

// TestRenderConventionsReplacesOldSections confirms the compact Conventions
// section replaced the old "Your First Code Contribution" and "Style guides"
// blocks in the default section list.
func TestRenderConventionsReplacesOldSections(t *testing.T) {
	b := contributing.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "conv-proj"}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "## Conventions")
	assert.NotContains(t, body, "## Your First Code Contribution", "old section must be gone")
	assert.NotContains(t, body, "## Style guides", "old section must be gone")
}

// TestRenderConventionsVersioningKnown renders the SemVer link for the
// documented semantic value.
func TestRenderConventionsVersioningKnown(t *testing.T) {
	b := contributing.Bridge{}
	pf := withConventions(&projectfile.Document{Identity: projectfile.Identity{Name: "v-proj"}},
		map[string]any{"versioning": "semantic"})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**Versioning:** [Semantic Versioning](https://semver.org/)")
}

// TestRenderConventionsVersioningArbitrary guards the graceful-degradation
// rule: an unknown versioning value must surface as a raw bullet, not vanish.
func TestRenderConventionsVersioningArbitrary(t *testing.T) {
	b := contributing.Bridge{}
	pf := withConventions(&projectfile.Document{Identity: projectfile.Identity{Name: "v-proj"}},
		map[string]any{"versioning": "my-scheme"})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**Versioning:** my-scheme", "unknown value must render raw, not drop")
}

// TestRenderConventionsWorkflowArbitrary guards graceful degradation for an
// unknown workflow: the raw value surfaces and the default branch is NOT
// interpolated (no known sentence template applies).
func TestRenderConventionsWorkflowArbitrary(t *testing.T) {
	b := contributing.Bridge{}
	pf := withConventions(&projectfile.Document{Identity: projectfile.Identity{Name: "w-proj"}},
		map[string]any{"workflow": "weird-flow"})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**Workflow:** uses the `weird-flow` workflow.")
}

// TestRenderConventionsWorkflowKnownInterpolatesBranch confirms a known
// workflow value interpolates the default branch into its sentence.
func TestRenderConventionsWorkflowKnownInterpolatesBranch(t *testing.T) {
	b := contributing.Bridge{}
	pf := withConventions(&projectfile.Document{
		Identity:     projectfile.Identity{Name: "w-proj"},
		Repositories: []projectfile.Repository{{Role: "origin", Branch: "trunk"}},
	}, map[string]any{"workflow": "git-flow"})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**Workflow:** Git Flow — feature branches from `develop`")
	assert.Contains(t, body, "hotfix branches from `trunk`.", "default branch must be interpolated")
}

// TestRenderConventionsLanguageStyles renders one bullet per declared stack
// style-guide-url, labelled by the raw stack tag.
func TestRenderConventionsLanguageStyles(t *testing.T) {
	b := contributing.Bridge{}
	pf := withConventions(&projectfile.Document{
		Identity: projectfile.Identity{Name: "ls-proj"},
		Stack:    []string{"js"},
	}, map[string]any{"js": map[string]any{"style-guide-url": "https://standardjs.com"}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**js style:** <https://standardjs.com>")
}

// TestRenderConventionsCommitsDefault confirms an unset commit-style still
// renders the default Conventional Commits bullet (prior behaviour preserved).
func TestRenderConventionsCommitsDefault(t *testing.T) {
	b := contributing.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "c-proj"}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["CONTRIBUTING.md"])
	assert.Contains(t, body, "**Commits:** [Conventional Commits](https://www.conventionalcommits.org/)")
}
