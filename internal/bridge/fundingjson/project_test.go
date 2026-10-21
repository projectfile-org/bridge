// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

func TestResolveProjectNilDocument(t *testing.T) {
	assert.Nil(t, resolveProject(nil))
}

// The placeholder "this project" is not a project, so the section is skipped.
func TestResolveProjectSkipsPlaceholderName(t *testing.T) {
	assert.Nil(t, resolveProject(&projectfile.Document{Identity: projectfile.Identity{Name: ""}}))
}

// The guid is the lowercased, dash-separated name, every other char dropped.
func TestResolveProjectGuidSanitised(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "Widget_2 X"}}
	proj := resolveProject(pf)
	require.NotNil(t, proj)
	assert.Equal(t, "widget-2x", proj.GUID)
}

// A titled document with no name falls back to the fixed "project" guid.
func TestResolveProjectGuidFallsBackToProject(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	proj := resolveProject(pf)
	require.NotNil(t, proj)
	assert.Equal(t, "project", proj.GUID)
	assert.Equal(t, fxTitle, proj.Name)
}

// The declared SPDX term is the licence entry; no licence renders none.
func TestResolveProjectLicenses(t *testing.T) {
	withSpdx := resolveProject(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		License:  &projectfile.License{Spdx: fxSpdx},
	})
	require.NotNil(t, withSpdx)
	assert.Equal(t, []string{"spdx:" + fxSpdx}, withSpdx.Licenses)

	without := resolveProject(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}})
	require.NotNil(t, without)
	assert.Nil(t, without.Licenses)
}

// The description falls back from summary to description, then stays empty.
func TestResolveProjectDescriptionFallback(t *testing.T) {
	fromSummary := resolveProject(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName, Summary: &projectfile.LocalizedString{Bare: fxSummary}},
	})
	require.NotNil(t, fromSummary)
	assert.Equal(t, fxSummary, fromSummary.Description)

	fromDescription := resolveProject(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName, Description: &projectfile.LocalizedString{Bare: fxAltDesc}},
	})
	require.NotNil(t, fromDescription)
	assert.Equal(t, fxAltDesc, fromDescription.Description)

	empty := resolveProject(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}})
	require.NotNil(t, empty)
	assert.Empty(t, empty.Description)
}

// The homepage fills webpageUrl when declared and leaves it empty otherwise.
func TestResolveProjectWebpage(t *testing.T) {
	withHome := resolveProject(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		Links:    []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	})
	require.NotNil(t, withHome)
	assert.Equal(t, fxHomepage, withHome.WebpageURL.URL)
	assert.Empty(t, withHome.WebpageURL.WellKnown)

	without := resolveProject(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}})
	require.NotNil(t, without)
	assert.Empty(t, without.WebpageURL.URL)
}

// A public repository URL is the repositoryUrl; an SSH clone falls back to the source-code link.
func TestResolveProjectRepositoryURL(t *testing.T) {
	fromRepository := resolveProject(&projectfile.Document{
		Identity:     projectfile.Identity{Name: fxName},
		Repositories: []projectfile.Repository{{URL: fxRepo}},
	})
	require.NotNil(t, fromRepository)
	assert.Equal(t, fxRepo, fromRepository.RepositoryURL.URL)

	fromLink := resolveProject(&projectfile.Document{
		Identity:     projectfile.Identity{Name: fxName},
		Repositories: []projectfile.Repository{{URL: "git@github.com:d9t/widget.git"}},
		Links:        []projectfile.Link{{Type: projectfile.LinkSourceCode, URL: fxRepo}},
	})
	require.NotNil(t, fromLink)
	assert.Equal(t, fxRepo, fromLink.RepositoryURL.URL)

	empty := resolveProject(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}})
	require.NotNil(t, empty)
	assert.Empty(t, empty.RepositoryURL.URL)
}

// Only lowercase-alphanumeric-dash keywords are tags, the rest are dropped.
func TestFilterTagsPattern(t *testing.T) {
	got := filterTags([]string{"go", "Go", "two words", "tooling-2", "under_score", "dot.name", ""})
	assert.Equal(t, []string{"go", "tooling-2"}, got)
	assert.Empty(t, filterTags(nil))
	assert.NotNil(t, filterTags(nil), "an empty tag list is an empty slice, not nil")
}

// The schema caps a project at ten tags; the eleventh valid keyword is dropped.
func TestFilterTagsCapAtTen(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	got := filterTags(in)
	assert.Len(t, got, 10)
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, got)
}
