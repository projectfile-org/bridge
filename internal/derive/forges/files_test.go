// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forges

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	gitHub   = "https://github.com/o/tool"
	codeberg = "https://codeberg.org/o/tool"
)

// TestFileURL pins the file page of each forge kind, with and without a declared branch, and none without a forge.
func TestFileURL(t *testing.T) {
	for _, tc := range []struct {
		name, url, branch, want string
	}{
		{"github default branch", gitHub, "", gitHub + "/blob/HEAD/docs/a%20b.md"},
		{"github branch", gitHub, "dev", gitHub + "/blob/dev/docs/a%20b.md"},
		{"forgejo default branch", codeberg, "", codeberg + "/src/docs/a%20b.md"},
		{"forgejo branch", codeberg, "dev", codeberg + "/src/branch/dev/docs/a%20b.md"},
		{"gitlab default branch", "https://gitlab.com/o/tool", "", "https://gitlab.com/o/tool/-/blob/HEAD/docs/a%20b.md"},
		{"unknown forge", "https://git.example.test/o/tool", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pf := &projectfile.Document{
				Links:        []projectfile.Link{{Type: projectfile.LinkSourceCode, URL: tc.url, Preferred: true}},
				Repositories: []projectfile.Repository{{URL: tc.url, Branch: tc.branch}},
			}
			assert.Equal(t, tc.want, FileURL(pf, nil)("docs/a b.md"))
		})
	}
	assert.Empty(t, FileURL(&projectfile.Document{}, nil)("LICENSE"), "no source-code link, no forge page")
}

// TestFileURLPrefersAPreferredPublicLink: a preferred link tagged public beats a higher-priority public one, which beats a preferred private one.
func TestFileURLPrefersAPreferredPublicLink(t *testing.T) {
	public := map[string]any{"tags": []any{AliasPublic}}
	ranked := map[string]any{"tags": []any{AliasPublic}, "priority": 90}
	pf := &projectfile.Document{Links: []projectfile.Link{
		{Type: projectfile.LinkSourceCode, URL: codeberg, Extra: ranked},
		{Type: projectfile.LinkSourceCode, URL: gitHub, Preferred: true, Extra: public},
	}}
	assert.Equal(t, gitHub+"/blob/HEAD/LICENSE", FileURL(pf, nil)("LICENSE"))
	pf.Links[1].Extra = nil
	assert.Equal(t, codeberg+"/src/LICENSE", FileURL(pf, nil)("LICENSE"), "without a preferred public link the public alias wins")
}
