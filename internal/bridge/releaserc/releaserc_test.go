// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releaserc_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/releaserc"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const testBranches = "branches"

// doc builds a Document with a single repository, an optional forge-kinds
// override, and the release extension populated from a map.
func doc(repoURL string, kinds map[string]string, release map[string]any) *projectfile.Document {
	d := &projectfile.Document{
		Identity:     projectfile.Identity{Name: "ubuntu"},
		Repositories: []projectfile.Repository{{URL: repoURL}},
	}
	if release != nil {
		projectfile.SetExtension(d, pfmodel.ReleaseExtensionNS, release)
	}
	if kinds != nil {
		projectfile.SetExtension(d, pfmodel.ForgeExtensionNS, map[string]any{
			"kinds": toAnyMap(kinds),
		})
	}
	return d
}

func toAnyMap(in map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mainBranch() []any {
	return []any{map[string]any{"pattern": "main", "channel": "latest"}}
}

// renderConfig renders and unmarshals the body back so we assert on structure,
// not on text — the marker line and REUSE header are stripped by the parser.
func renderConfig(t *testing.T, pf *projectfile.Document) (map[string]any, string) {
	t.Helper()
	out, err := releaserc.Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	body := string(out.Files[".releaserc.yaml"])
	assert.True(t, strings.HasPrefix(body, core.YAMLDocStart),
		"output must begin with the YAML document-start marker for yamllint compliance")
	assert.NotContains(t, body, "'@semantic-release/",
		"plugin names must use double quotes (the project's single quote-style), never single")
	assert.Contains(t, body, `"@semantic-release/`,
		"plugin names must be double-quoted")
	assert.Contains(t, body, core.Marker, "ownership marker must be present")
	var cfg map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(body), &cfg), "output must be valid YAML")
	return cfg, body
}

// Self-hosted Forgejo on a bare host (kiota.ch) resolves via the forge-kinds
// override and emits @markwylde/semantic-release-gitea with the instance URL.
func TestForgejoSelfHosted(t *testing.T) {
	pf := doc("https://kiota.ch/b19/ubuntu", map[string]string{"kiota.ch": "forgejo"},
		map[string]any{"tag-format": "v${version}", testBranches: mainBranch()})
	cfg, _ := renderConfig(t, pf)

	assert.Equal(t, "v${version}", cfg["tagFormat"])
	plugins := cfg["plugins"].([]any)
	gitea := plugins[len(plugins)-1].([]any) // last entry is the forge plugin pair
	assert.Equal(t, "@markwylde/semantic-release-gitea", gitea[0])
	assert.Equal(t, "https://kiota.ch", gitea[1].(map[string]any)["giteaUrl"])
}

// Codeberg is also Forgejo but a different instance URL — same plugin, distinct
// giteaUrl. Proves the host (not a hardcoded constant) drives the URL.
func TestCodebergInstanceURL(t *testing.T) {
	pf := doc("https://codeberg.org/me/proj", nil,
		map[string]any{testBranches: mainBranch()})
	cfg, _ := renderConfig(t, pf)
	gitea := cfg["plugins"].([]any)[2].([]any)
	assert.Equal(t, "https://codeberg.org", gitea[1].(map[string]any)["giteaUrl"])
}

// GitHub-primary projects get @semantic-release/github (a plain string, no URL).
func TestGitHubPrimary(t *testing.T) {
	pf := doc("https://github.com/me/proj", nil,
		map[string]any{testBranches: mainBranch()})
	cfg, _ := renderConfig(t, pf)
	plugins := cfg["plugins"].([]any)
	assert.Equal(t, "@semantic-release/github", plugins[len(plugins)-1])
}

// changelog != none injects @semantic-release/changelog; absence omits it.
func TestChangelogGating(t *testing.T) {
	with := doc("https://github.com/me/proj", nil,
		map[string]any{"changelog": "keep-a-changelog", testBranches: mainBranch()})
	cfgWith, _ := renderConfig(t, with)
	assert.Contains(t, cfgWith["plugins"], "@semantic-release/changelog")

	without := doc("https://github.com/me/proj", nil,
		map[string]any{"changelog": "none", testBranches: mainBranch()})
	cfgWithout, _ := renderConfig(t, without)
	assert.NotContains(t, cfgWithout["plugins"], "@semantic-release/changelog")
}

// tag-format defaults to v${version} when the field is omitted.
func TestTagFormatDefault(t *testing.T) {
	pf := doc("https://github.com/me/proj", nil, map[string]any{testBranches: mainBranch()})
	cfg, _ := renderConfig(t, pf)
	assert.Equal(t, "v${version}", cfg["tagFormat"])
}

// Refusals: no release intent, no branches, and no repository URL each error.
func TestRefusals(t *testing.T) {
	b := releaserc.Bridge{}

	_, err := b.Render(doc("https://github.com/me/proj", nil, nil), core.Options{})
	assert.Error(t, err, "absent release extension must refuse")

	_, err = b.Render(doc("https://github.com/me/proj", nil,
		map[string]any{"tag-format": "v${version}"}), core.Options{})
	assert.Error(t, err, "no branches must refuse")

	_, err = b.Render(doc("", nil, map[string]any{testBranches: mainBranch()}), core.Options{})
	assert.Error(t, err, "no repository URL must refuse")
}
