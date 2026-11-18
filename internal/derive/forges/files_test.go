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
