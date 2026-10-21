// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// repoWithURL builds a document carrying one repository entry, the shape every resolver in this file reads.
func repoWithURL(url, role string, extra map[string]any) projectfile.Repository {
	return projectfile.Repository{URL: url, Role: role, Extra: extra}
}

// TestPrimaryRepositoryResolvesTheCanonicalEntry pins the three-way rule: the origin role, else the sole entry, else nothing when several entries and none is the origin.
func TestPrimaryRepositoryResolvesTheCanonicalEntry(t *testing.T) {
	assert.Nil(t, PrimaryRepository(nil))
	assert.Nil(t, PrimaryRepository(&projectfile.Document{}))

	sole := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://mirror.test", "", nil),
	}}
	require.NotNil(t, PrimaryRepository(sole), "a sole entry is implicitly the origin")
	assert.Equal(t, "https://mirror.test", PrimaryRepository(sole).URL)

	multi := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://mirror.test", "", nil),
		repoWithURL(urlOrigin, RepositoryRoleOrigin, nil),
	}}
	assert.Equal(t, urlOrigin, PrimaryRepository(multi).URL)

	originless := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://one.test", "mirror", nil),
		repoWithURL("https://two.test", "archive", nil),
	}}
	assert.Nil(t, PrimaryRepository(originless), "no origin and more than one entry resolves nothing")
}

// TestIssuesRepositoryFallsBackExactly pins the two-step rule and the nil document.
func TestIssuesRepositoryFallsBackExactly(t *testing.T) {
	assert.Nil(t, IssuesRepository(nil))

	doc := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL(urlOrigin, RepositoryRoleOrigin, nil),
		repoWithURL("https://tracker.test", "", nil),
	}}
	require.NotNil(t, IssuesRepository(doc), "no entry claims issues, so the primary answers")
	assert.Equal(t, urlOrigin, IssuesRepository(doc).URL)

	originless := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://mirror.test", "mirror", nil),
		repoWithURL("https://archive.test", "archive", nil),
	}}
	assert.Nil(t, IssuesRepository(originless), "no issues entry and no primary resolves nothing")
}

// TestReleasesRepositoriesListsEveryClaimantAndSkipsNonBools pins the §139 read and its tolerance: an origin and every mirror a release lands on.
func TestReleasesRepositoriesListsEveryClaimantAndSkipsNonBools(t *testing.T) {
	assert.Nil(t, ReleasesRepositories(nil))
	assert.Empty(t, ReleasesRepositories(&projectfile.Document{}))

	doc := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL(urlOrigin, RepositoryRoleOrigin, map[string]any{keyReleases: true}),
		repoWithURL("https://mirror.test", "", map[string]any{keyReleases: valueYes}),
		repoWithURL("https://none.test", "", nil),
	}}
	got := ReleasesRepositories(doc)
	require.Len(t, got, 1)
	assert.Equal(t, urlOrigin, got[0].URL)
}

// TestBoolFromExtraReadsOnlyRealBools.
func TestBoolFromExtraReadsOnlyRealBools(t *testing.T) {
	assert.False(t, boolFromExtra(nil, keyReleases))
	assert.False(t, boolFromExtra(map[string]any{}, keyReleases))
	assert.True(t, boolFromExtra(map[string]any{keyReleases: true}, keyReleases))
	assert.False(t, boolFromExtra(map[string]any{keyReleases: valueTrue}, keyReleases),
		"a string is not a toggle")
}

// TestEnsureCitableSourceLinkReusesOrAppends pins the slot a sync driver writes into, so a foreign manifest's URL never clobbers the ssh clone endpoint.
func TestEnsureCitableSourceLinkReusesOrAppends(t *testing.T) {
	assert.Nil(t, EnsureCitableSourceLink(nil))

	existing := &projectfile.Document{Links: []projectfile.Link{
		{Type: LinkSourceCode, URL: urlOrigin, Preferred: true},
		{Type: LinkSourceCode, URL: "https://mirror.test"},
	}}
	l := EnsureCitableSourceLink(existing)
	require.NotNil(t, l)
	assert.Equal(t, urlOrigin, l.URL, "the preferred page is the one that is reused")

	fresh := &projectfile.Document{}
	l = EnsureCitableSourceLink(fresh)
	require.NotNil(t, l)
	assert.Empty(t, l.URL, "a created entry is empty and awaits the caller's write")
	require.Len(t, fresh.Links, 1)
	assert.Equal(t, LinkSourceCode, fresh.Links[0].Type)
}

// TestEnsurePrimaryRepositoryCreatesTheOriginSlot pins the creation rule and the reuse of an existing origin.
func TestEnsurePrimaryRepositoryCreatesTheOriginSlot(t *testing.T) {
	assert.Nil(t, EnsurePrimaryRepository(nil))

	doc := &projectfile.Document{}
	r := EnsurePrimaryRepository(doc)
	require.NotNil(t, r)
	assert.Equal(t, RepositoryRoleOrigin, r.Role)
	require.Len(t, doc.Repositories, 1)

	again := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://mirror.test", "mirror", nil),
		repoWithURL(urlOrigin, RepositoryRoleOrigin, nil),
	}}
	assert.Equal(t, urlOrigin, EnsurePrimaryRepository(again).URL)

	originless := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("https://mirror.test", "mirror", nil),
	}}
	assert.Equal(t, "https://mirror.test", EnsurePrimaryRepository(originless).URL,
		"no origin slot, so the first entry is the slot")
}

// TestEnsurePackagesRepositoryPrefersTheClaimant pins the packages step before the origin fallback.
func TestEnsurePackagesRepositoryPrefersTheClaimant(t *testing.T) {
	assert.Nil(t, EnsurePackagesRepository(nil))

	doc := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL(urlOrigin, RepositoryRoleOrigin, nil),
		{URL: "https://packages.test", Packages: true},
	}}
	assert.Equal(t, "https://packages.test", EnsurePackagesRepository(doc).URL)

	plain := &projectfile.Document{}
	assert.Equal(t, RepositoryRoleOrigin, EnsurePackagesRepository(plain).Role,
		"no claimant, so the origin slot answers")
}

// TestCitableRepositoryURLLeavesAnSSHEndpointUnaddressable pins the refusal: an ssh URL with no http page answers empty.
func TestCitableRepositoryURLLeavesAnSSHEndpointUnaddressable(t *testing.T) {
	doc := &projectfile.Document{Repositories: []projectfile.Repository{
		repoWithURL("git@kiota.ch:org/proj.git", RepositoryRoleOrigin, nil),
	}}
	assert.Empty(t, CitableRepositoryURL(doc))

	noRepos := &projectfile.Document{Links: []projectfile.Link{
		{Type: LinkSourceCode, URL: urlOrigin},
	}}
	assert.Empty(t, CitableRepositoryURL(noRepos), "no repository resolves nothing")
}

// TestPickRepositoryAndIsPublicURL pin the citable-repository selection and the schemes CITATION.cff, SPDX VCS expressions and package metadata accept.
func TestPickRepositoryAndIsPublicURL(t *testing.T) {
	assert.Nil(t, pickRepository(nil))
	assert.Nil(t, pickRepository(&projectfile.Document{}))
	assert.False(t, isPublicURL("git@kiota.ch:org/proj.git"))
	assert.False(t, isPublicURL(""))
	assert.True(t, isPublicURL("https://kiota.ch/org/proj"))
	assert.True(t, isPublicURL("http://kiota.ch/org/proj"))
	assert.True(t, isPublicURL("ftp://kiota.ch/org/proj"))
	assert.True(t, isPublicURL("sftp://kiota.ch/org/proj"))
}
