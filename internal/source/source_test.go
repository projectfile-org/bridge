// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/cff"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/composer"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/npm"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// ── CITATION.cff ─────────────────────────────────────────────────────────────

func TestCFFSourceIdentityAndDetect(t *testing.T) {
	s := CFFSource{}
	assert.Equal(t, fxNameCFF, s.Name())
	assert.Equal(t, fxNameCFF, s.Filename())
	assert.True(t, s.Detect(writeFixture(t, fxNameCFF, cffFixture)))
	assert.False(t, s.Detect(t.TempDir()), "an empty dir carries no CITATION.cff")
}

func TestCFFSourceExtractEveryField(t *testing.T) {
	dir := writeFixture(t, fxNameCFF, cffFixture)
	p, err := CFFSource{}.Extract(dir)
	require.NoError(t, err)

	assert.Equal(t, &LocalizedString{Bare: fxCFFTitle}, p.Title)
	assert.Equal(t, &LocalizedString{Bare: fxCFFAbstract}, p.Summary)
	assert.Equal(t, fxCFFVersion, *p.Version)
	assert.Equal(t, fxLicenseMIT, *p.License)
	assert.Equal(t, fxSoftwareKind, *p.Kind)

	require.Len(t, p.Repositories, 1, "repository-code is the clone endpoint")
	assert.Equal(t, fxRepoURL, p.Repositories[0].URL)
	assert.Equal(t, repoTypeGit, p.Repositories[0].Type)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, p.Repositories[0].Role)

	assert.Equal(t, []string{fxKeywordLower}, p.Keywords)

	require.Len(t, p.Links, 2, "the code repository doubles as the forge landing page")
	assert.Equal(t, projectfile.LinkSourceCode, p.Links[0].Type)
	assert.Equal(t, fxRepoURL, p.Links[0].URL)
	assert.Equal(t, projectfile.LinkHomepage, p.Links[1].Type)
	assert.Equal(t, fxHomepageCFF, p.Links[1].URL)

	require.Len(t, p.People, 2, "authors then maintainers")
	author := p.People[0]
	assert.Equal(t, fxFamilyLovelace, author.FamilyNames)
	assert.Equal(t, fxGivenAda, author.GivenNames)
	assert.Equal(t, fxEmailA, author.Email)
	assert.Equal(t, fxRoleAuthor, author.Roles[0])
	assert.Equal(t, fxOrcidBare, author.Orcid, "the bare ORCID id replaces the full URL")
	assert.Equal(t, "https://ada.example.test", author.URL, "website maps onto the person URL")

	assert.Equal(t, fxFamilyHopper, p.People[1].FamilyNames)
	assert.Equal(t, fxRoleMaintain, p.People[1].Roles[0])
}

// TestCFFSourceExtractKindAndGaps pins both mapped CFF types, the unmapped fall-through and a repository-less document.
func TestCFFSourceExtractKindAndGaps(t *testing.T) {
	dataset := `cff-version: 1.2.0
type: dataset
title: A dataset
license: [MIT, Apache-2.0]
authors:
  - name: Acme Org
    email: a@example.test
`
	p, err := CFFSource{}.Extract(writeFixture(t, fxNameCFF, dataset))
	require.NoError(t, err)
	assert.Equal(t, fxDatasetKind, *p.Kind)
	assert.Equal(t, "MIT OR Apache-2.0", *p.License, "a compound CFF license flattens to one expression")
	assert.Empty(t, p.Repositories)
	assert.Empty(t, p.Links, "neither repository-code nor url is declared")
	require.Len(t, p.People, 1, "a named entity with no family-names keeps its flat name")
	assert.Equal(t, fxEntityName, p.People[0].Name)
	assert.Empty(t, p.People[0].FamilyNames)

	other := "cff-version: 1.2.0\ntype: article\ntitle: An article\n"
	p, err = CFFSource{}.Extract(writeFixture(t, fxNameCFF, other))
	require.NoError(t, err)
	assert.Nil(t, p.Kind, "an unmapped CFF type leaves identity.kind unset")

	malformed := "cff-version: 1.2.0\nauthors: [unclosed\n"
	_, err = CFFSource{}.Extract(writeFixture(t, fxNameCFF, malformed))
	require.Error(t, err, "malformed YAML is refused, not half-read")
	assert.Contains(t, err.Error(), "read CITATION.cff")
}

func person(name, email, url string) npm.Person {
	return npm.Person{Name: name, Email: email, URL: url}
}

func composerPerson(name, email, homepage string) composer.Person {
	return composer.Person{Name: name, Email: email, Homepage: homepage}
}

func TestCFFPeopleEmpty(t *testing.T) {
	assert.Empty(t, cffPeople(nil, fxRoleContrib))
	assert.Empty(t, cffPeople([]cff.PersonEntity{}, fxRoleContrib))
}

// ── package.json ─────────────────────────────────────────────────────────────

func TestNPMSourceIdentityAndDetect(t *testing.T) {
	s := NPMSource{}
	assert.Equal(t, "package.json", s.Name())
	assert.Equal(t, "package.json", s.Filename())
	assert.True(t, s.Detect(writeFixture(t, "package.json", npmFixture)))
	assert.False(t, s.Detect(t.TempDir()))
}

func TestNPMSourceExtractEveryField(t *testing.T) {
	dir := writeFixture(t, "package.json", npmFixture)
	p, err := NPMSource{}.Extract(dir)
	require.NoError(t, err)

	assert.Equal(t, "npm.acme", *p.Namespace, "a scoped package names its own npm.<scope> namespace")
	assert.Equal(t, fxCFFName, *p.Name, "the scope is stripped off the bare name")
	assert.Equal(t, "1.0.0", *p.Version)
	assert.Equal(t, &LocalizedString{Bare: fxNpmDescription}, p.Summary)
	assert.Equal(t, fxLicenseMIT, *p.License)

	require.Len(t, p.Repositories, 1)
	assert.Equal(t, fxRepoURL, p.Repositories[0].URL)
	assert.Equal(t, fxRepoDir, p.Repositories[0].Path, "repository.directory carries through as the path")
	assert.Equal(t, projectfile.RepositoryRoleOrigin, p.Repositories[0].Role)

	assert.ElementsMatch(t, []string{fxKeywordLower, fxKeywordUpper}, p.Keywords)
	assert.Equal(t, []projectfile.Link{
		{Type: projectfile.LinkBugs, URL: fxBugsURL},
		{Type: projectfile.LinkHomepage, URL: fxHomeURL},
	}, p.Links)

	require.Len(t, p.People, 3, "author plus contributors plus maintainers")
	assert.Equal(t, []string{fxGivenAda, fxFamilyLovelace}, []string{p.People[0].GivenNames, p.People[0].FamilyNames})
	assert.Equal(t, fxEmailA, p.People[0].Email)
	assert.Equal(t, fxRoleAuthor, p.People[0].Roles[0])
	assert.Equal(t, fxRoleContrib, p.People[1].Roles[0])
	assert.Equal(t, []string{fxGivenGrace, fxFamilyHopper}, []string{p.People[1].GivenNames, p.People[1].FamilyNames})
	assert.Equal(t, fxRoleMaintain, p.People[2].Roles[0])
	assert.Equal(t, fxEmailNpm, p.People[2].Email)

	assert.ElementsMatch(t, []string{fxStackNode, fxStackYarn}, p.Stack, "engines keys name the runtimes the project needs")
}

// TestNPMSourceExtractGaps pins an unscoped minimal manifest: no namespace, no people, no stack.
func TestNPMSourceExtractGaps(t *testing.T) {
	dir := writeFixture(t, "package.json", `{"name": "bare", "version": "9.9.9"}`)
	p, err := NPMSource{}.Extract(dir)
	require.NoError(t, err)
	assert.Nil(t, p.Namespace, "an unscoped package carries no namespace")
	assert.Equal(t, "bare", *p.Name)
	assert.Empty(t, p.People)
	assert.Empty(t, p.Stack, "no engines means no stack")
	assert.Empty(t, p.Links)
	assert.Empty(t, p.Keywords)

	malformed := `{"name": oops}`
	_, err = NPMSource{}.Extract(writeFixture(t, "package.json", malformed))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read package.json")
}

// TestNPMNameToPFName pins every shape a package name can take, scope with no package included.
func TestNPMNameToPFName(t *testing.T) {
	ns, name := npmNameToPFName(fxNpmScoped)
	assert.Equal(t, "npm.acme", ns)
	assert.Equal(t, fxCFFName, name)

	ns, name = npmNameToPFName("@widget")
	assert.Empty(t, ns, "a scope with no slash is not a namespace")
	assert.Equal(t, "widget", name, "the @ is still stripped")

	ns, name = npmNameToPFName("bare")
	assert.Empty(t, ns)
	assert.Equal(t, "bare", name)
}

// TestNPMPersonEntry pins the split-name and single-name spellings.
func TestNPMPersonEntry(t *testing.T) {
	split := npmPersonEntry(person(fxGivenAda+" "+fxFamilyLovelace, fxEmailA, ""), fxRoleAuthor)
	assert.Equal(t, fxGivenAda, split.GivenNames)
	assert.Equal(t, fxFamilyLovelace, split.FamilyNames)
	assert.Equal(t, fxEmailA, split.Email)
	assert.Equal(t, []string{fxRoleAuthor}, split.Roles)

	single := npmPersonEntry(person(fxEntitySingle, "", ""), fxRoleMaintain)
	assert.Equal(t, fxEntitySingle, single.Name, "a single word is the flat name, not a given name")
	assert.Empty(t, single.GivenNames)

	none := npmPersonEntry(person("", "", ""), fxRoleAuthor)
	assert.Empty(t, none.Name)
	assert.Empty(t, none.GivenNames)
}

func TestExtractStackFromEngines(t *testing.T) {
	assert.ElementsMatch(t, []string{"node", "npm", "yarn", "pnpm"}, extractStackFromEngines(map[string]string{
		fxStackNode: ">=20", "npm": ">=10", fxStackYarn: ">=4", "pnpm": ">=9",
	}))
	assert.Equal(t, []string{fxStackNode}, extractStackFromEngines(map[string]string{fxStackNode: ">=20"}))
	assert.Empty(t, extractStackFromEngines(map[string]string{"deno": ">=2", "bun": ">=1"}),
		"an engine outside the vocabulary names no stack")
	assert.Empty(t, extractStackFromEngines(nil))
}

// ── composer.json ────────────────────────────────────────────────────────────

func TestComposerSourceIdentityAndDetect(t *testing.T) {
	s := ComposerSource{}
	assert.Equal(t, "composer.json", s.Name())
	assert.Equal(t, "composer.json", s.Filename())
	assert.True(t, s.Detect(writeFixture(t, "composer.json", composerFixture)))
	assert.False(t, s.Detect(t.TempDir()))
}

func TestComposerSourceExtractEveryField(t *testing.T) {
	dir := writeFixture(t, "composer.json", composerFixture)
	p, err := ComposerSource{}.Extract(dir)
	require.NoError(t, err)

	assert.Equal(t, "composer.acme", *p.Namespace, "a Packagist vendor name becomes composer.<vendor>")
	assert.Equal(t, fxCFFName, *p.Name)
	assert.Equal(t, "2.0.0", *p.Version)
	assert.Equal(t, &LocalizedString{Bare: fxComposerDesc}, p.Summary)
	assert.Equal(t, fxLicenseMIT, *p.License)
	assert.Equal(t, []string{"php"}, p.Stack)

	require.Len(t, p.Repositories, 1, "support.source is the clone endpoint")
	assert.Equal(t, "https://github.com/acme/widget", p.Repositories[0].URL)
	assert.Equal(t, repoTypeGit, p.Repositories[0].Type)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, p.Repositories[0].Role)

	require.Len(t, p.Links, 3)
	assert.Equal(t, projectfile.LinkHomepage, p.Links[0].Type)
	assert.Equal(t, fxHomepageCompose, p.Links[0].URL)
	assert.Equal(t, projectfile.LinkBugs, p.Links[1].Type)
	assert.Equal(t, fxBugsURL, p.Links[1].URL)
	assert.Equal(t, projectfile.LinkDocumentation, p.Links[2].Type, "support.docs is main documentation, not reading material")
	assert.Equal(t, fxDocsURL, p.Links[2].URL)
	assert.Equal(t, []string{pfmodel.TagMainDocumentation}, pfmodel.LinkTags(p.Links[2]),
		"the tag is what lets the reverse direction recognise the slot")

	assert.ElementsMatch(t, []string{fxKeywordLower, fxKeywordUpper}, p.Keywords)

	require.Len(t, p.People, 2)
	assert.Equal(t, []string{fxGivenAda, fxFamilyLovelace}, []string{p.People[0].GivenNames, p.People[0].FamilyNames})
	assert.Equal(t, fxEmailA, p.People[0].Email)
	assert.Equal(t, "https://ada.example.test", p.People[0].URL)
	assert.Equal(t, fxRoleAuthor, p.People[0].Roles[0])
	assert.Equal(t, []string{fxEntitySingle, "Org"}, []string{p.People[1].GivenNames, p.People[1].FamilyNames},
		"composer splits a two-word author into given and family names")
	assert.Equal(t, fxEmailComposer, p.People[1].Email, "the organisation entry keeps its own email")
}

// TestComposerSourceExtractGaps pins a bare manifest, an unreadable author and a malformed file.
func TestComposerSourceExtractGaps(t *testing.T) {
	bare := `{"name": "bare/widget"}`
	p, err := ComposerSource{}.Extract(writeFixture(t, "composer.json", bare))
	require.NoError(t, err)
	assert.Equal(t, "composer.bare", *p.Namespace)
	assert.Empty(t, p.Links, "no homepage and no support block produce no links")
	assert.Empty(t, p.Repositories)
	assert.Empty(t, p.Stack, "no php requirement means no stack")
	assert.Empty(t, p.Keywords, "a composer manifest with no declared keywords yields no list")

	blankAuthor := `{"name": "acme/widget", "authors": [{}]}`
	p, err = ComposerSource{}.Extract(writeFixture(t, "composer.json", blankAuthor))
	require.NoError(t, err)
	assert.Empty(t, p.People, "an author with no name, given-names or email is dropped")

	malformed := `{"name": oops}`
	_, err = ComposerSource{}.Extract(writeFixture(t, "composer.json", malformed))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read composer.json")
}

func TestComposerNameToPFName(t *testing.T) {
	ns, name := composerNameToPFName(fxComposerScoped)
	assert.Equal(t, "composer.acme", ns)
	assert.Equal(t, fxCFFName, name)

	ns, name = composerNameToPFName("widget")
	assert.Empty(t, ns, "a name with no slash is not a namespaced package")
	assert.Equal(t, "widget", name)
}

func TestComposerPersonEntry(t *testing.T) {
	split := composerPersonEntry(composerPerson(fxGivenAda+" "+fxFamilyLovelace, fxEmailA, ""))
	assert.Equal(t, fxGivenAda, split.GivenNames)
	assert.Equal(t, fxFamilyLovelace, split.FamilyNames)
	assert.Equal(t, fxEmailA, split.Email)
	assert.Equal(t, "https://ada.example.test", composerPersonEntry(composerPerson("", "", "https://ada.example.test")).URL)

	single := composerPersonEntry(composerPerson(fxEntitySingle, "", ""))
	assert.Equal(t, fxEntitySingle, single.Name)

	blank := composerPersonEntry(composerPerson("   ", "", ""))
	assert.Empty(t, blank.Name, "a whitespace-only name is not a person")
	assert.Equal(t, []string{fxRoleAuthor}, blank.Roles)
}

// ── shard.yml ────────────────────────────────────────────────────────────────

func TestShardSourceIdentityAndDetect(t *testing.T) {
	s := ShardSource{}
	assert.Equal(t, "shard.yml", s.Name())
	assert.Equal(t, "shard.yml", s.Filename())
	assert.True(t, s.Detect(writeFixture(t, "shard.yml", shardFixture)))
	assert.True(t, ShardSource{}.Detect(writeFixture(t, "shard.yaml", shardFixture)),
		"shard.yaml is the accepted alternate spelling")
	assert.False(t, s.Detect(t.TempDir()))
}

func TestShardSourceExtractEveryField(t *testing.T) {
	dir := writeFixture(t, "shard.yml", shardFixture)
	p, err := ShardSource{}.Extract(dir)
	require.NoError(t, err)

	assert.Equal(t, fxCFFName, *p.Name)
	assert.Equal(t, "0.1.0", *p.Version)
	assert.Equal(t, &LocalizedString{Bare: "A shard manifest"}, p.Summary)
	assert.Equal(t, fxLicenseMIT, *p.License, "a bare SPDX id is a license, not a URL")

	require.Len(t, p.Repositories, 1)
	assert.Equal(t, "https://github.com/acme/widget", p.Repositories[0].URL)
	assert.Equal(t, repoTypeGit, p.Repositories[0].Type)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, p.Repositories[0].Role)

	require.Len(t, p.Links, 2)
	assert.Equal(t, projectfile.LinkHomepage, p.Links[0].Type)
	assert.Equal(t, fxHomepageShard, p.Links[0].URL)
	assert.Equal(t, projectfile.LinkDocumentation, p.Links[1].Type)
	assert.Equal(t, fxDocsURL, p.Links[1].URL)
	assert.Equal(t, []string{pfmodel.TagMainDocumentation}, pfmodel.LinkTags(p.Links[1]))

	require.Len(t, p.People, 2, "the empty author entry is dropped")
	assert.Equal(t, []string{fxGivenAda, fxFamilyLovelace}, []string{p.People[0].GivenNames, p.People[0].FamilyNames})
	assert.Equal(t, fxEmailA, p.People[0].Email)
	assert.Equal(t, fxRoleAuthor, p.People[0].Roles[0])
	assert.Equal(t, fxEntityName, p.People[1].GivenNames+" "+p.People[1].FamilyNames,
		"the shard author line splits on the first space")

	assert.Equal(t, []string{fxGeneratedStack}, p.Stack, "crystal is the one stack a shard manifest asserts")
}

// TestShardSourceExtractURLAndGaps pins the license-URL suppression, blank description and author list.
func TestShardSourceExtractURLAndGaps(t *testing.T) {
	urlLicense := "name: widget\nlicense: " + fxLicenseURL + "\ndescription: \"  \"\nauthors:\n  - \"  \"\n"
	p, err := ShardSource{}.Extract(writeFixture(t, "shard.yml", urlLicense))
	require.NoError(t, err)
	assert.Nil(t, p.License, "a license URL is not an SPDX id, so the slot is left empty")
	assert.Nil(t, p.Summary, "a whitespace-only description is nothing")
	assert.Empty(t, p.People, "an author string with no name and no email is dropped")
	assert.Empty(t, p.Repositories)

	malformed := "name: widget\nauthors: [unclosed\n"
	_, err = ShardSource{}.Extract(writeFixture(t, "shard.yml", malformed))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read shard.yml")
}

// TestShardAuthorEntry pins every readable spelling plus the unterminated email bracket.
func TestShardAuthorEntry(t *testing.T) {
	both, ok := shardAuthorEntry(fxGivenAda + " " + fxFamilyLovelace + " <" + fxEmailA + ">")
	require.True(t, ok)
	assert.Equal(t, fxGivenAda, both.GivenNames)
	assert.Equal(t, fxFamilyLovelace, both.FamilyNames)
	assert.Equal(t, fxEmailA, both.Email)

	bare, ok := shardAuthorEntry(fxEntitySingle)
	require.True(t, ok)
	assert.Equal(t, fxEntitySingle, bare.Name, "a single word is the flat name")

	emailOnly, ok := shardAuthorEntry("<" + fxEmailA + ">")
	require.True(t, ok)
	assert.Equal(t, fxEmailA, emailOnly.Email)
	assert.Empty(t, emailOnly.Name)

	unterminated, ok := shardAuthorEntry(fxGivenAda + " <" + fxEmailA)
	require.True(t, ok)
	assert.Equal(t, fxGivenAda, unterminated.GivenNames, "an unterminated bracket keeps the tail inside the name")
	assert.Equal(t, "<"+fxEmailA, unterminated.FamilyNames)
	assert.Empty(t, unterminated.Email, "no email is cut out without the closing bracket")
	assert.Empty(t, unterminated.Name)

	_, ok = shardAuthorEntry("   ")
	assert.False(t, ok, "a blank author string yields no entry")
	_, ok = shardAuthorEntry("<>")
	assert.False(t, ok, "an author with neither a name nor an email is dropped")
}

func TestIsLicenseURL(t *testing.T) {
	assert.True(t, isLicenseURL(fxLicenseURL))
	assert.True(t, isLicenseURL("http://example.test/LICENSE"))
	assert.False(t, isLicenseURL(fxLicenseMIT), "a bare SPDX id is not a URL")
	assert.False(t, isLicenseURL(""))
}

// ── shared fixture adapters ──────────────────────────────────────────────────
