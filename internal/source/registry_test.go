// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// TestRegisterExtendsRegistry pins that Register appends and leaves the shared registry as it found it.
func TestRegisterExtendsRegistry(t *testing.T) {
	require.NotEmpty(t, Registry(), "the defaults seed exactly once")
	base := len(registry)

	Register(CFFSource{})
	require.Len(t, Registry(), base+1, "Register appends to the registry")
	registry = registry[:base]
}

func TestRegistryNamesEveryDefaultSource(t *testing.T) {
	var names []string
	for _, s := range Registry() {
		names = append(names, s.Name())
	}
	assert.Equal(t, []string{fxNameCFF, fxNameNPM, "composer.json", "shard.yml"}, names,
		"the registry order is the merge precedence, so the citation file wins over an npm manifest")
}

// TestDiscoverReportsOnlyPresentSources pins that Discover names only the manifests a directory actually holds.
func TestDiscoverReportsOnlyPresentSources(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, fxNameCFF, cffFixture)
	writeManifest(t, dir, "package.json", npmFixture)

	var names []string
	for _, s := range Discover(dir) {
		names = append(names, s.Name())
	}
	assert.Equal(t, []string{"CITATION.cff", "package.json"}, names)
	assert.Empty(t, Discover(t.TempDir()), "an empty directory discovers no source")
}

// TestExtractAllEmptyDir pins the no-source answer: an empty partial, no names, no error.
func TestExtractAllEmptyDir(t *testing.T) {
	p, names, err := ExtractAll(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, &Partial{}, p)
	assert.Empty(t, names)
}

// TestExtractAllMergesSources pins the merge: each read source is named, and the first to declare a field wins.
func TestExtractAllMergesSources(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, fxNameCFF, cffFixture)
	writeManifest(t, dir, "package.json", npmFixture)

	p, names, err := ExtractAll(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"CITATION.cff", "package.json"}, names)
	assert.Equal(t, &LocalizedString{Bare: fxCFFTitle}, p.Title, "the citation title is read first")
	assert.Equal(t, fxCFFAbstract, p.Summary.Bare, "the citation abstract outranks the package.json description")
	assert.Equal(t, fxCFFVersion, *p.Version)

	require.Len(t, p.Repositories, 1, "the first source to declare a repository wins it whole")
	assert.Equal(t, fxRepoURL, p.Repositories[0].URL)

	assert.Equal(t, []projectfile.Link{
		{Type: projectfile.LinkSourceCode, URL: fxRepoURL},
		{Type: projectfile.LinkHomepage, URL: fxHomepageCFF},
		{Type: projectfile.LinkBugs, URL: fxBugsURL},
		{Type: projectfile.LinkHomepage, URL: fxHomeURL},
	}, p.Links, "links gap-fill by (type, url): a different homepage is a new entry, a repeated one is not")
}

func TestExtractAllRefusesAMalformedManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, fxNameCFF, "authors: [unclosed\n")

	p, names, err := ExtractAll(dir)
	require.Error(t, err, "one unreadable manifest fails the whole extraction rather than half-filling")
	assert.Contains(t, err.Error(), "extract from CITATION.cff")
	assert.Nil(t, p)
	assert.Nil(t, names)
}

// TestMergePartialsGapFillsEveryScalar pins the first-wins contract: an overlay only fills what the base left empty.
func TestMergePartialsGapFillsEveryScalar(t *testing.T) {
	base := &Partial{}
	overlay := &Partial{
		Namespace: StringPtr("npm.acme"),
		Name:      StringPtr(fxCFFName),
		Title:     &LocalizedString{Bare: fxCFFTitle},
		Summary:   &LocalizedString{Bare: fxCFFAbstract},
		Version:   StringPtr(fxCFFVersion),
		License:   StringPtr(fxLicenseMIT),
		Kind:      StringPtr(fxSoftwareKind),
		People:    []PersonEntry{{Email: fxEmailA}},
		Keywords:  []string{fxKeywordLower},
		Stack:     []string{fxStackNode},
		Created:   StringPtr("2024-01-01"),
		Modified:  StringPtr("2026-01-01"),
	}

	got := MergePartials(base, overlay)
	assert.Equal(t, overlay, got, "an empty base adopts the overlay whole")
	assert.NotSame(t, base, got, "the base is never mutated in place")
}

// TestMergePartialsBaseWins pins that a declared base value is never replaced.
func TestMergePartialsBaseWins(t *testing.T) {
	base := &Partial{
		Namespace: StringPtr("composer.acme"),
		Name:      StringPtr("base"),
		Title:     &LocalizedString{Bare: "Base title"},
		Summary:   &LocalizedString{Bare: "Base summary"},
		Version:   StringPtr("0.0.1"),
		License:   StringPtr("Apache-2.0"),
		Kind:      StringPtr(fxDatasetKind),
		People:    []PersonEntry{{Email: fxEmailB}},
		Keywords:  []string{"base"},
		Stack:     []string{"php"},
		Created:   StringPtr("2020-01-01"),
		Modified:  StringPtr("2020-06-01"),
	}
	before := *base

	got := MergePartials(base, &Partial{
		Namespace: StringPtr("npm.acme"),
		Name:      StringPtr("overlay"),
		Title:     &LocalizedString{Bare: "Overlay title"},
		Summary:   &LocalizedString{Bare: "Overlay summary"},
		Version:   StringPtr("9.9.9"),
		License:   StringPtr(fxLicenseMIT),
		Kind:      StringPtr(fxSoftwareKind),
		People:    []PersonEntry{{Email: fxEmailA}},
		Keywords:  []string{"overlay"},
		Stack:     []string{fxStackNode},
		Created:   StringPtr("2024-01-01"),
		Modified:  StringPtr("2026-01-01"),
	})

	assert.Equal(t, before, *got, "a populated base keeps every one of its own values")
}

// TestMergePartialsRepositoriesFirstWins pins the one non-gap-fill rule: a declared list is adopted whole.
func TestMergePartialsRepositoriesFirstWins(t *testing.T) {
	existing := projectfile.Repository{URL: fxRepoURL, Type: repoTypeGit, Role: projectfile.RepositoryRoleOrigin}
	extra := projectfile.Repository{URL: "https://example.test/other", Type: repoTypeGit}

	kept := MergePartials(&Partial{Repositories: []projectfile.Repository{existing}}, &Partial{
		Repositories: []projectfile.Repository{extra},
	})
	assert.Equal(t, []projectfile.Repository{existing}, kept.Repositories, "the first source to declare a repository wins it whole")

	filled := MergePartials(&Partial{}, &Partial{Repositories: []projectfile.Repository{existing, extra}})
	assert.Equal(t, []projectfile.Repository{existing, extra}, filled.Repositories, "an absent list adopts the overlay")
}

// TestMergePartialsLinksDedup pins the link key: a (type, url) pair already present is never appended again.
func TestMergePartialsLinksDedup(t *testing.T) {
	first := projectfile.Link{Type: projectfile.LinkHomepage, URL: fxHomeURL}
	second := projectfile.Link{Type: projectfile.LinkBugs, URL: fxBugsURL}

	got := MergePartials(
		&Partial{Links: []projectfile.Link{first}},
		&Partial{Links: []projectfile.Link{first, second, {Type: projectfile.LinkBugs, URL: fxBugsURL}}},
	)
	assert.Equal(t, []projectfile.Link{first, second}, got.Links, "one duplicate pair is appended once")

	sameType := MergePartials(
		&Partial{Links: []projectfile.Link{first}},
		&Partial{Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomeURL + "/other"}}},
	)
	assert.Len(t, sameType.Links, 2, "the same type at a different url is a new entry")

	sameURL := MergePartials(
		&Partial{Links: []projectfile.Link{first}},
		&Partial{Links: []projectfile.Link{{Type: projectfile.LinkSourceCode, URL: fxHomeURL}}},
	)
	assert.Len(t, sameURL.Links, 2, "the same url at a different type is a new entry")

	assert.Empty(t, MergePartials(&Partial{}, &Partial{}).Links, "two empty link lists stay empty")
}

func TestStringPtr(t *testing.T) {
	assert.Nil(t, StringPtr(""), "an empty string carries no value at all")
	got := StringPtr(fxCFFVersion)
	require.NotNil(t, got)
	assert.Equal(t, fxCFFVersion, *got)
}

func TestHasLink(t *testing.T) {
	links := []projectfile.Link{
		{Type: projectfile.LinkHomepage, URL: fxHomeURL},
		{Type: projectfile.LinkBugs, URL: fxBugsURL},
	}

	assert.True(t, hasLink(links, links[0]))
	assert.False(t, hasLink(links, projectfile.Link{Type: projectfile.LinkHomepage, URL: fxHomeURL + "/other"}),
		"a different url is a different link")
	assert.False(t, hasLink(links, projectfile.Link{Type: projectfile.LinkSourceCode, URL: fxHomeURL}),
		"a different type is a different link")
	assert.False(t, hasLink(nil, links[0]), "an empty list matches nothing")
	assert.False(t, hasLink(nil, projectfile.Link{}))
}
