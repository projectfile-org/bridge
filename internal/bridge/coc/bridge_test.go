// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package coc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Literals three or more test files repeat, hoisted so goconst stops counting them.
const (
	nestedRel = "docs/CODE_OF_CONDUCT.md"
	keyPath   = "path"
	xdgApp    = "projectfile"
	cliFile   = "cli.json"
	xdgEnv    = "XDG_CONFIG_HOME"
)

// ── identity surface ────────────────────────────────────────────────────────

func TestBridgeIdentity(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "coc", b.Name())
	assert.Equal(t, core.FileCodeOfConduct, b.Filename())
	assert.Nil(t, b.Aliases())
	label, provider := b.Labels()
	assert.Equal(t, core.FileCodeOfConduct, label)
	assert.Equal(t, "projectfile", provider)
	assert.True(t, b.Policy().Marker, "Markdown carries a comment, so the sentinel folds into the header")
}

// pointUserConfigAt aims $XDG_CONFIG_HOME at a temp tree holding ext as cli.json and drops the load cache.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, xdgApp), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, xdgApp, cliFile), body, 0o644))
	}
	t.Setenv(xdgEnv, dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

func TestBridgeExists(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.False(t, Bridge{}.Exists(dir), "an empty directory holds no code of conduct")
	require.NoError(t, os.WriteFile(filepath.Join(dir, core.FileCodeOfConduct), []byte("# Conduct"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the default path is the probed one")
	require.NoError(t, os.Remove(filepath.Join(dir, core.FileCodeOfConduct)))
	assert.False(t, Bridge{}.Exists(dir), "a removed file reads absent again")
}

// The user-config path override relocates the probe, so a nested file is what Exists sees.
func TestBridgeExistsFollowsTheUserConfigPath(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{core.FileCodeOfConduct: map[string]any{keyPath: nestedRel}}}})
	assert.False(t, Bridge{}.Exists(dir))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, nestedRel), []byte("# Conduct"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the override path is the probed one")
}

func TestBridgeFullPathResolvesTheUserConfigPath(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.Equal(t, filepath.Join(dir, core.FileCodeOfConduct), Bridge{}.FullPath(dir, nil))
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{core.FileCodeOfConduct: map[string]any{keyPath: nestedRel}}}})
	assert.Equal(t, filepath.Join(dir, nestedRel), Bridge{}.FullPath(dir, nil), "the user config outranks the built-in default")
}

// ── render ──────────────────────────────────────────────────────────────────

func TestRenderCanonicalConduct(t *testing.T) {
	out, err := Bridge{}.Render(conductDoc(), core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	body := string(out.Files[core.FileCodeOfConduct])
	assert.Contains(t, body, "# Contributor Covenant Code of Conduct")
	assert.Contains(t, body, "[Widget](README.md)", "the same-language README is the rebased sibling link")
	assert.Contains(t, body, "**<"+fxContact+">**", "the security contact is the enforcement mailbox")
	assert.Contains(t, body, "and also applies", "the default scope is project-and-spaces")
	assert.Contains(t, body, core.MarkerInner, "every generated file carries the sentinel")
	assert.NotContains(t, body, "learn how", "the footer is opt-in per namespace")
}

// contactEmail prefers a community person over a maintainer, so a curated moderator wins.
func TestRenderResolvesContactFromTheCommunityRole(t *testing.T) {
	pf := bareDoc()
	pf.People = []projectfile.Person{
		{GivenNames: "Ada", FamilyNames: "Lovelace", Email: fxMaint, Roles: []string{projectfile.RoleMaintainer}},
		{GivenNames: "Mod", FamilyNames: "Erator", Email: fxContact, Roles: []string{projectfile.RoleCommunity}},
	}
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[core.FileCodeOfConduct]), "**<"+fxContact+">**", "the curated moderator wins over the maintainer")
}

// The git config is the last resort, so a bare repository still renders a usable mailbox.
func TestRenderFallsBackToTheGitConfigEmail(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", gitHome(t))
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(bareDoc(), core.Options{Dir: dir})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[core.FileCodeOfConduct]), "**<"+fxGitEmail+">**")
}

func TestRenderFailsWithoutAnyContact(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pointUserConfigAt(t, nil)
	_, err := Bridge{}.Render(bareDoc(), core.Options{})
	require.Error(t, err, "a mailbox nobody declared is not invented")
	assert.Contains(t, err.Error(), "no contact email available")
}

// A declared scope narrows the covenant text and how-to-link turns the footer on.
func TestRenderHonoursDeclaredScopeAndHowToLink(t *testing.T) {
	pf := conductDoc()
	setConductNS(pf, map[string]any{"covenant": covenantContributorCovenant, "scope": "project", "how-to-link": true})
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileCodeOfConduct])
	assert.Contains(t, body, "issues, pull")
	assert.NotContains(t, body, "and also applies", "scope=project drops the public-representation clause")
	assert.Contains(t, body, "*Generated from projectfile ([learn how](https://projectfile.org/how-to/code-of-conduct))*")
}

// An unsupported covenant fails loudly instead of rendering CC against the project's intent.
func TestRenderRejectsAnUnsupportedCovenant(t *testing.T) {
	pf := conductDoc()
	setConductNS(pf, map[string]any{"covenant": "citizen-code-of-conduct"})
	_, err := Bridge{}.Render(pf, core.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported covenant")
	assert.Contains(t, err.Error(), covenantContributorCovenant)
}

// A declared language with no shipped template produces no file, never an English body.
func TestRenderSkipsADeclaredLanguageWithoutATemplate(t *testing.T) {
	out, err := Bridge{}.Render(withLangs(conductDoc(), "es", "zz"), core.Options{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{core.FileCodeOfConduct, "docs/es/CODE_OF_CONDUCT.md"}, keys(out.Files))
	assert.NotContains(t, string(out.Files[core.FileCodeOfConduct]), "docs/zz/", "a dropped language leaves the bar")
}

// es and uk ship embedded, so each declared language renders its own prose and rebased links.
func TestRenderLocalizedVariants(t *testing.T) {
	out, err := Bridge{}.Render(withLangs(conductDoc(), "es", "uk"), core.Options{})
	require.NoError(t, err)
	assert.Contains(t, out.Files, core.FileCodeOfConduct)
	assert.Contains(t, string(out.Files["docs/es/CODE_OF_CONDUCT.md"]), "# Código de Conducta del Contributor Covenant")
	assert.Contains(t, string(out.Files["docs/es/CODE_OF_CONDUCT.md"]), "[Widget](README.md)", "the sibling is co-located under docs/es/")
	assert.Contains(t, string(out.Files["docs/es/CODE_OF_CONDUCT.md"]), "**<"+fxContact+">**")
	assert.Contains(t, string(out.Files["docs/es/CODE_OF_CONDUCT.md"]), "[English](../../CODE_OF_CONDUCT.md)")
	assert.Contains(t, string(out.Files["docs/uk/CODE_OF_CONDUCT.md"]), "# Кодекс поведінки Contributor Covenant")
	assert.Contains(t, string(out.Files["docs/uk/CODE_OF_CONDUCT.md"]), "<!-- textlint-disable ")
}

// The See also section is entirely project-authored: one link tagged coc, no others.
func TestRenderAppendsTheTaggedSeeAlsoLinks(t *testing.T) {
	pf := conductDoc()
	pf.Links = []projectfile.Link{{
		Type:  projectfile.LinkDocumentation,
		URL:   "https://example.org/conduct",
		Label: &projectfile.LocalizedString{Bare: "Moderation log"},
		Extra: map[string]any{"tags": []any{"coc"}},
	}}
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileCodeOfConduct])
	assert.Contains(t, body, "## See also")
	assert.Contains(t, body, "- [Moderation log](https://example.org/conduct)")
}

// An untagged document grows no orphan heading, even with links[] declared.
func TestRenderOmitsSeeAlsoWithoutATaggedLink(t *testing.T) {
	pf := conductDoc()
	pf.Links = []projectfile.Link{{Type: projectfile.LinkHomepage, URL: "https://example.org"}}
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[core.FileCodeOfConduct]), "## See also")
}

// A project-local template that does not parse surfaces through the bridge's own error path.
func TestRenderFailsOnABrokenProjectLocalTemplate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, core.LocalTemplatesDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, core.LocalTemplatesDir, core.FileCodeOfConduct+".tmpl"), []byte("{{.NoSuchField"), 0o644))
	_, err := Bridge{}.Render(conductDoc(), core.Options{Dir: dir})
	require.Error(t, err, "a project-local template that does not parse must fail the render")
}

func TestRenderIsByteStable(t *testing.T) {
	first, err := Bridge{}.Render(conductDoc(), core.Options{})
	require.NoError(t, err)
	for range 4 {
		again, err := Bridge{}.Render(conductDoc(), core.Options{})
		require.NoError(t, err)
		assert.Equal(t, first.Files[core.FileCodeOfConduct], again.Files[core.FileCodeOfConduct])
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
