// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	testCodebergRepoURL = "https://codeberg.org/acme/proj.git"
	testProjRepoURL     = "https://kiota.ch/acme/proj.git"
)

func TestIssuesRepositoryExplicit(t *testing.T) {
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testProjRepoURL, Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergRepoURL, Issues: true},
		},
	}
	r := pfmodel.IssuesRepository(doc)
	require.NotNil(t, r)
	assert.Equal(t, testCodebergRepoURL, r.URL)
	assert.True(t, r.Issues)
}

func TestIssuesRepositoryFallsBackToPrimary(t *testing.T) {
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testProjRepoURL, Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergRepoURL},
		},
	}
	r := pfmodel.IssuesRepository(doc)
	require.NotNil(t, r)
	assert.Equal(t, testProjRepoURL, r.URL)
}

func TestIssuesRepositorySingleEntry(t *testing.T) {
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testProjRepoURL},
		},
	}
	r := pfmodel.IssuesRepository(doc)
	require.NotNil(t, r)
	assert.Equal(t, testProjRepoURL, r.URL)
}

func TestIssuesRepositoryNil(t *testing.T) {
	assert.Nil(t, pfmodel.IssuesRepository(nil))
	assert.Nil(t, pfmodel.IssuesRepository(&projectfile.Document{}))
}

// TestCitableRepositoryURL guards the cffr→origin→first pick order and the
// ssh→source-code-link resolution. The §139 Extra bag carries cffr:true.
func TestCitableRepositoryURLCFFRWinsOverOrigin(t *testing.T) {
	// cffr:true on a mirror must beat role:origin.
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testProjRepoURL, Role: projectfile.RepositoryRoleOrigin},
			{URL: testCodebergRepoURL, Role: projectfile.RepositoryRoleMirror, Extra: map[string]any{"cffr": true}},
		},
	}
	assert.Equal(t, testCodebergRepoURL, pfmodel.CitableRepositoryURL(doc))
}

func TestCitableRepositoryURLOriginWinsOverMirror(t *testing.T) {
	// No cffr marker: origin wins, even when a mirror is listed first.
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testCodebergRepoURL, Role: projectfile.RepositoryRoleMirror},
			{URL: "https://kiota.ch/acme/proj.git", Role: projectfile.RepositoryRoleOrigin},
		},
	}
	assert.Equal(t, "https://kiota.ch/acme/proj.git", pfmodel.CitableRepositoryURL(doc))
}

func TestCitableRepositoryURLSoleEntry(t *testing.T) {
	// A sole entry is implicitly origin; its http URL is returned directly.
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: "https://kiota.ch/acme/proj.git"},
		},
	}
	assert.Equal(t, "https://kiota.ch/acme/proj.git", pfmodel.CitableRepositoryURL(doc))
}

func TestCitableRepositoryURLFirstFallback(t *testing.T) {
	// No cffr, no role:origin → first entry wins.
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: testCodebergRepoURL, Role: projectfile.RepositoryRoleMirror},
			{URL: "https://github.com/acme/proj.git", Role: projectfile.RepositoryRoleMirror},
		},
	}
	assert.Equal(t, testCodebergRepoURL, pfmodel.CitableRepositoryURL(doc))
}

func TestCitableRepositoryURLSSHResolvedViaSourceCodeLink(t *testing.T) {
	// origin's URL is ssh; the http page lives in links[type=source-code].
	// The preferred (origin) page must win over the mirror page.
	doc := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: "ssh://git@kiota.ch/acme/proj.git", Role: projectfile.RepositoryRoleOrigin},
			{URL: "ssh://git@codeberg.org/acme/proj.git", Role: projectfile.RepositoryRoleMirror},
		},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: "https://codeberg.org/acme/proj"},
			{Type: projectfile.LinkSourceCode, URL: "https://kiota.ch/acme/proj", Preferred: true},
		},
	}
	assert.Equal(t, "https://kiota.ch/acme/proj", pfmodel.CitableRepositoryURL(doc))
}

func TestCitableRepositoryURLNil(t *testing.T) {
	assert.Equal(t, "", pfmodel.CitableRepositoryURL(nil))
	assert.Equal(t, "", pfmodel.CitableRepositoryURL(&projectfile.Document{}))
}
