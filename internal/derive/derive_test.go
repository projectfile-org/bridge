// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package derive_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive"
)

const (
	testToolName    = "my-tool"
	testCodebergURL = "https://codeberg.org/acme/my-tool.git"
)

func docWithSourceLink(url string) *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: testToolName},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: url},
		},
	}
}

func TestApplyGitHubIssueTracker(t *testing.T) {
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, changes, "derive should produce changes for a GitHub source-code link")

	var bugsURL string
	for _, l := range pf.Links {
		if l.Type == projectfile.LinkBugs {
			bugsURL = l.URL
			break
		}
	}
	assert.Contains(t, bugsURL, "github.com/acme/my-tool")
	assert.Contains(t, bugsURL, "issues")
}

func TestApplyGitLabIssueTracker(t *testing.T) {
	pf := docWithSourceLink("https://gitlab.com/acme/my-lib")
	_, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)

	var bugsURL string
	for _, l := range pf.Links {
		if l.Type == projectfile.LinkBugs {
			bugsURL = l.URL
			break
		}
	}
	require.NotEmpty(t, bugsURL)
	assert.Contains(t, bugsURL, "gitlab.com")
}

func TestApplyCodebergIssueTracker(t *testing.T) {
	pf := docWithSourceLink("https://codeberg.org/acme/my-project")
	_, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)

	var bugsURL string
	for _, l := range pf.Links {
		if l.Type == projectfile.LinkBugs {
			bugsURL = l.URL
			break
		}
	}
	require.NotEmpty(t, bugsURL)
	assert.Contains(t, bugsURL, "codeberg.org")
}

func TestApplyIdempotent(t *testing.T) {
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	changes1, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, changes1)

	// Second run: fields are already derived and owned; no new changes.
	changes2, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	assert.Empty(t, changes2, "second Apply on already-derived doc should produce no new changes")
}

func TestApplyHandWrittenBugsURLPreserved(t *testing.T) {
	// A hand-written bugs URL that was never derived must survive Apply unchanged.
	const handWritten = "https://jira.example.com/browse/MY"
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	pf.Links = append(pf.Links, projectfile.Link{
		Type: projectfile.LinkBugs,
		URL:  handWritten,
	})

	_, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)

	found := false
	for _, l := range pf.Links {
		if l.URL == handWritten {
			found = true
			break
		}
	}
	assert.True(t, found, "hand-written bugs URL must not be removed or replaced")
}

func TestApplyNoSourceLinkNoChanges(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: testToolName},
	}
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	assert.Empty(t, changes, "derive should produce nothing when there are no source URLs")
}

func TestApplyChangeRecordsFieldPath(t *testing.T) {
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)

	for _, c := range changes {
		assert.NotEmpty(t, c.FieldPath, "every Change must have a FieldPath")
		assert.NotEmpty(t, c.Source, "every Change must identify its source")
		assert.NotEmpty(t, c.NewValue, "every Change must have a NewValue")
	}
}

func TestApplyIssuesTrueFiltersToSingleRepo(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: testToolName},
		Repositories: []projectfile.Repository{
			{URL: "https://github.com/acme/my-tool.git", Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergURL, Issues: true},
		},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: "https://github.com/acme/my-tool"},
			{Type: projectfile.LinkSourceCode, URL: "https://codeberg.org/acme/my-tool"},
		},
	}
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.Len(t, changes, 1, "issues=true should produce exactly one bugs link")
	assert.Contains(t, changes[0].NewValue, "codeberg.org")
	assert.Contains(t, changes[0].NewValue, "/issues")
}

func TestApplyIssuesTrueWithSSHRepo(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: testToolName},
		Repositories: []projectfile.Repository{
			{URL: "git@github.com:acme/my-tool.git", Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergURL, Issues: true},
		},
	}
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Contains(t, changes[0].NewValue, "codeberg.org")
}

func TestApplyDeriveLabel(t *testing.T) {
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, changes)

	// Single-language doc → Bare label.
	require.NotNil(t, changes[0].Label)
	assert.Equal(t, "Issues on GitHub", changes[0].Label.Bare)

	var label string
	for _, l := range pf.Links {
		if l.Type == projectfile.LinkBugs {
			if l.Label != nil {
				label = l.Label.Bare
			}
			break
		}
	}
	assert.Equal(t, "Issues on GitHub", label)
}

func TestApplyDeriveLabelGitLab(t *testing.T) {
	pf := docWithSourceLink("https://gitlab.com/acme/my-lib")
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, changes)
	require.NotNil(t, changes[0].Label)
	assert.Equal(t, "Issues on GitLab", changes[0].Label.Bare)
}

// TestApplyDeriveLabelLocalized verifies a multi-language project gets a Langs
// map (Bare cleared) so each locale renders its own noun + connector, and that
// it lands on the written link.
func TestApplyDeriveLabelLocalized(t *testing.T) {
	pf := docWithSourceLink("https://github.com/acme/my-tool")
	pf.Extensions = map[string]any{
		"org.projectfile.i18n": map[string]any{
			"default-language": "en",
			"languages":        []any{"es", "uk"},
		},
	}
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, changes)
	label := changes[0].Label
	require.NotNil(t, label)
	assert.Empty(t, label.Bare, "Bare must be cleared or it masks the Langs map")
	assert.Equal(t, map[string]string{
		"en": "Issues on GitHub",
		"es": "Incidencias en GitHub",
		"uk": "Issues на GitHub",
	}, label.Langs)

	var written *projectfile.LocalizedString
	for _, l := range pf.Links {
		if l.Type == projectfile.LinkBugs {
			written = l.Label
			break
		}
	}
	require.NotNil(t, written)
	assert.Equal(t, label.Langs, written.Langs)
}

func TestApplyNoIssuesKeyDerivesAllSourceCodeLinks(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: testToolName},
		Repositories: []projectfile.Repository{
			{URL: "https://github.com/acme/my-tool.git", Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergURL},
		},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: "https://github.com/acme/my-tool"},
			{Type: projectfile.LinkSourceCode, URL: "https://codeberg.org/acme/my-tool"},
		},
	}
	changes, err := derive.Apply(pf, derive.Options{})
	require.NoError(t, err)
	assert.Len(t, changes, 2, "without issues key, all source-code links get bugs entries")
}
