// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forges_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/forges"
)

// owner is the repository owner every b19 mirror shares in these fixtures.
const (
	owner = "b19"
	repo  = "ubuntu"
	// codebergURL is the web page every fixture mirror resolves to, and the
	// value the issues alias must reach from an ssh clone URL.
	codebergURL = "https://codeberg.org/b19/ubuntu"
	kindForgejo = "forgejo"
	hostKiota   = "kiota.ch"
	slugGit     = "git"
	keyTags     = "tags"
	keyReleases = "releases"
	kiotaSSH    = "ssh://git@kiota.ch/b19/ubuntu.git"
	codebergSCP = "git@codeberg.org:b19/ubuntu.git"
)

func sourceCode(urls ...string) *projectfile.Document {
	doc := &projectfile.Document{}
	for _, u := range urls {
		doc.Links = append(doc.Links, projectfile.Link{Type: projectfile.LinkSourceCode, URL: u})
	}
	return doc
}

// The b19/ubuntu shape: three mirrors, one of them a self-hosted Forgejo on a
// bare hostname that only org.projectfile.forge.kinds can classify.
func TestRemotesAcrossMirrors(t *testing.T) {
	doc := sourceCode(
		codebergURL,
		"https://github.com/damian-buho/b19-ubuntu",
		"https://kiota.ch/b19/ubuntu",
	)

	got := forges.Remotes(doc, map[string]string{hostKiota: kindForgejo})

	require.Len(t, got, 3)
	assert.Equal(t, map[string]any{
		forges.KeyHost: "codeberg.org", forges.KeyOwner: owner, forges.KeyRepo: repo,
		forges.KeyURL: codebergURL, forges.KeyKind: kindForgejo,
	}, got["codeberg"])
	assert.Equal(t, map[string]any{
		forges.KeyHost: "github.com", forges.KeyOwner: "damian-buho", forges.KeyRepo: "b19-ubuntu",
		forges.KeyURL: "https://github.com/damian-buho/b19-ubuntu", forges.KeyKind: "github",
	}, got["github"])
	assert.Equal(t, kindForgejo, got["kiota"].(map[string]any)[forges.KeyKind],
		"a bare-hostname instance is classified only by the user's kinds map")
}

func TestRemotesShapes(t *testing.T) {
	cases := []struct {
		name, url, slug, owner, repo string
	}{
		{"trailing .git", "https://codeberg.org/b19/ubuntu.git", "codeberg", owner, repo},
		{"trailing slash", "https://codeberg.org/b19/ubuntu/", "codeberg", owner, repo},
		{"gitlab subgroup", "https://gitlab.com/group/sub/proj", "gitlab", "group/sub", "proj"},
		{"sourcehut tilde", "https://git.sr.ht/~user/repo", slugGit, "~user", "repo"},
		{"self-hosted", "https://gitlab.example.com/team/app", "gitlab", "team", "app"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := forges.Remotes(sourceCode(tc.url), nil)
			remote, ok := got[tc.slug].(map[string]any)
			require.True(t, ok, "expected slug %q in %v", tc.slug, got)
			assert.Equal(t, tc.owner, remote[forges.KeyOwner])
			assert.Equal(t, tc.repo, remote[forges.KeyRepo])
		})
	}
}

// Nothing addressable → nothing derived, so a badge referencing a remote is
// dropped rather than rendered against a half-built URL.
func TestRemotesSkipsUnusable(t *testing.T) {
	assert.Nil(t, forges.Remotes(nil, nil))
	assert.Nil(t, forges.Remotes(sourceCode(), nil))
	assert.Nil(t, forges.Remotes(sourceCode(kiotaSSH), nil),
		"an ssh clone URL has no scheme a badge endpoint can use")
	assert.Nil(t, forges.Remotes(sourceCode("https://codeberg.org/"), nil), "no owner/repo")
}

// Two mirrors on one host would fight over the slug; first in document order
// wins so the rendered badge row is stable.
func TestRemotesFirstMirrorWinsTheSlug(t *testing.T) {
	got := forges.Remotes(sourceCode(
		"https://github.com/one/project",
		"https://github.com/two/project",
	), nil)

	require.Len(t, got, 1)
	assert.Equal(t, "one", got["github"].(map[string]any)[forges.KeyOwner])
}

// tagged builds a source-code link carrying capability tags on the §139
// additional-key channel, the shape a YAML decoder produces.
func tagged(url string, preferred bool, tags ...string) projectfile.Link {
	l := projectfile.Link{Type: projectfile.LinkSourceCode, URL: url, Preferred: preferred}
	if len(tags) > 0 {
		items := make([]any, 0, len(tags))
		for _, t := range tags {
			items = append(items, t)
		}
		l.Extra = map[string]any{keyTags: items}
	}
	return l
}

// The whole point of the pass: a fragment addresses `remotes.badges` and gets
// whichever mirror THIS project tagged, with the slug still reachable.
func TestRemotesAliasesByCapability(t *testing.T) {
	doc := &projectfile.Document{Links: []projectfile.Link{
		tagged(codebergURL, false, "public", "badges"),
		tagged("https://github.com/damian-buho/b19-ubuntu", false, "public", "ci"),
		tagged("https://kiota.ch/b19/ubuntu", true, "ci"),
	}}

	got := forges.Remotes(doc, map[string]string{hostKiota: kindForgejo})

	assert.Equal(t, got["codeberg"], got["badges"], "an alias IS the remote, not a copy")
	assert.Equal(t, got["codeberg"], got["public"], "first in document order wins the alias")
	assert.Equal(t, got["github"], got["ci"], "github is the first ci-tagged mirror")
	assert.Equal(t, got["kiota"], got[forges.AliasPreferred],
		"links[].preferred yields its alias with no tag declared")
}

// A fragment's claim must hold however a project orders its includes: the
// highest `priority` wins the alias, not the first in document order.
func TestRemotesAliasByPriority(t *testing.T) {
	kiota := tagged("https://kiota.ch/b19/ubuntu", true, "public", "badges")
	kiota.Extra["priority"] = 10
	codeberg := tagged(codebergURL, false, "public", "badges")
	codeberg.Extra["priority"] = 90
	doc := &projectfile.Document{Links: []projectfile.Link{kiota, codeberg}}

	got := forges.Remotes(doc, map[string]string{hostKiota: kindForgejo})

	assert.Equal(t, got["codeberg"], got["badges"], "priority 90 beats 10 declared first")
	assert.Equal(t, got["codeberg"], got["public"])
	assert.Equal(t, got["kiota"], got[forges.AliasPreferred], "preferred is not contested here")
}

// A capability must never steal an identity: a project that tags a mirror
// `gitea` keeps remotes.gitea meaning gitea.com.
func TestRemotesAliasNeverShadowsASlug(t *testing.T) {
	doc := &projectfile.Document{Links: []projectfile.Link{
		tagged("https://gitea.com/real/project", false),
		tagged(codebergURL, false, "gitea"),
	}}

	got := forges.Remotes(doc, nil)

	assert.Equal(t, "gitea.com", got["gitea"].(map[string]any)[forges.KeyHost],
		"the slug keeps its name; the tag is refused")
	assert.Len(t, got, 2, "the refused alias adds no entry")
}

// repositories[issues=true] carries an ssh clone URL. It must still resolve to
// the web coordinates of the same repository, so `issues` is declared once.
func TestRemotesIssuesAliasFromRepositories(t *testing.T) {
	cases := []struct{ name, repoURL string }{
		{"ssh scheme", "ssh://git@codeberg.org/b19/ubuntu.git"},
		{"scp style", codebergSCP},
		{"already http", codebergURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := sourceCode(codebergURL, "https://github.com/o/r")
			doc.Repositories = []projectfile.Repository{
				{URL: tc.repoURL, Issues: true, Type: "git"},
			}

			got := forges.Remotes(doc, nil)

			assert.Equal(t, got["codeberg"], got[forges.AliasIssues])
		})
	}
}

// No matching source-code link means no alias — better an unresolved reference
// the drop rule removes than a badge pointing at the wrong tracker.
func TestRemotesIssuesAliasUnmatched(t *testing.T) {
	doc := sourceCode(codebergURL)
	doc.Repositories = []projectfile.Repository{
		{URL: kiotaSSH, Issues: true, Type: "git"},
	}

	got := forges.Remotes(doc, nil)

	assert.NotContains(t, got, forges.AliasIssues)
}

// repositories[releases=true] may mark several mirrors. Each one gets its route
// alias; the bare alias follows the most public link, exactly as `badges` does.
func TestRemotesReleasesAliasesFromRepositories(t *testing.T) {
	kiota := tagged("https://kiota.ch/b19/ubuntu", true)
	kiota.Extra = map[string]any{"priority": 10}
	github := tagged("https://github.com/damian-buho/b19-ubuntu", false)
	github.Extra = map[string]any{"priority": 80}
	doc := &projectfile.Document{Links: []projectfile.Link{kiota, github, tagged(codebergURL, false)}}
	doc.Repositories = []projectfile.Repository{
		{URL: kiotaSSH, Role: "origin", Extra: map[string]any{keyReleases: true}},
		{URL: "git@github.com:damian-buho/b19-ubuntu.git", Role: "mirror", Extra: map[string]any{keyReleases: true}},
		{URL: codebergSCP, Role: "mirror"},
	}

	got := forges.Remotes(doc, map[string]string{hostKiota: kindForgejo})

	assert.Equal(t, got["kiota"], got["releases-gitea"], "kinds map classifies kiota as forgejo, route gitea")
	assert.Equal(t, got["github"], got["releases-github"])
	assert.Equal(t, got["github"], got[forges.AliasReleases], "priority 80 beats 10 for the bare alias")
	assert.NotContains(t, got, "releases-codeberg", "an unmarked mirror claims nothing")
}

// No marked repository, or one no source-code link matches, files no alias.
func TestRemotesReleasesAliasUnmatched(t *testing.T) {
	doc := sourceCode(codebergURL)
	doc.Repositories = []projectfile.Repository{
		{URL: kiotaSSH, Extra: map[string]any{keyReleases: true}},
		{URL: codebergSCP, Extra: map[string]any{keyReleases: "yes"}},
	}

	got := forges.Remotes(doc, nil)

	assert.NotContains(t, got, forges.AliasReleases)
	assert.NotContains(t, got, "releases-gitea")
}

// A malformed or absent tag list drops the capability instead of inventing one.
func TestRemotesTolerateBadTags(t *testing.T) {
	bare := projectfile.Link{
		Type:  projectfile.LinkSourceCode,
		URL:   codebergURL,
		Extra: map[string]any{keyTags: "badges"},
	}
	mixed := projectfile.Link{
		Type:  projectfile.LinkSourceCode,
		URL:   "https://github.com/o/r",
		Extra: map[string]any{keyTags: []any{42, "", "ci"}},
	}
	doc := &projectfile.Document{Links: []projectfile.Link{bare, mixed}}

	got := forges.Remotes(doc, nil)

	assert.NotContains(t, got, "badges", "a bare string is not a tag list")
	assert.Equal(t, got["github"], got["ci"], "the usable item still counts")
	assert.Len(t, got, 3, "codeberg, github, ci")
}
