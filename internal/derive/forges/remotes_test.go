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
		"https://codeberg.org/b19/ubuntu",
		"https://github.com/damian-buho/b19-ubuntu",
		"https://kiota.ch/b19/ubuntu",
	)

	got := forges.Remotes(doc, map[string]string{"kiota.ch": "forgejo"})

	require.Len(t, got, 3)
	assert.Equal(t, map[string]any{
		forges.KeyHost: "codeberg.org", forges.KeyOwner: owner, forges.KeyRepo: repo,
		forges.KeyURL: "https://codeberg.org/b19/ubuntu", forges.KeyKind: "forgejo",
	}, got["codeberg"])
	assert.Equal(t, map[string]any{
		forges.KeyHost: "github.com", forges.KeyOwner: "damian-buho", forges.KeyRepo: "b19-ubuntu",
		forges.KeyURL: "https://github.com/damian-buho/b19-ubuntu", forges.KeyKind: "github",
	}, got["github"])
	assert.Equal(t, "forgejo", got["kiota"].(map[string]any)[forges.KeyKind],
		"a bare-hostname instance is classified only by the user's kinds map")
}

func TestRemotesShapes(t *testing.T) {
	cases := []struct {
		name, url, slug, owner, repo string
	}{
		{"trailing .git", "https://codeberg.org/b19/ubuntu.git", "codeberg", owner, repo},
		{"trailing slash", "https://codeberg.org/b19/ubuntu/", "codeberg", owner, repo},
		{"gitlab subgroup", "https://gitlab.com/group/sub/proj", "gitlab", "group/sub", "proj"},
		{"sourcehut tilde", "https://git.sr.ht/~user/repo", "git", "~user", "repo"},
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
	assert.Nil(t, forges.Remotes(sourceCode("ssh://git@kiota.ch/b19/ubuntu.git"), nil),
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
