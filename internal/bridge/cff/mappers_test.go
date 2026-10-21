// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

const (
	keyRepositoryCode = "repository-code"
	keyAuthorsContact = "authors/contact"
	keyPreferredCitat = "preferred-citation"
	keyIdentifiers    = "identifiers"
	keyVersion        = "version"
	keyDateReleased   = "date-released"
	keyTitle          = "title"
	keyAbstract       = "abstract"
	keyLicense        = "license"
	keyURL            = cffKeyURL
	keyKeywords       = cffKeyKeywords
	keyDOI            = cffKeyDOI

	roleMaintainer  = "maintainer"
	roleSecurity    = "security"
	roleContributor = "contributor"

	clearedRemoved = "removed"

	testDOIValue       = "10.5281/zenodo.1234567"
	testOrcidBare      = "0009-0004-8579-7633"
	testOrcidFull      = "https://orcid.org/0009-0004-8579-7633"
	testVersion        = "1.2.3"
	testNewVersion     = "2.0.0"
	testReleased       = "2026-01-15"
	testSummary        = "A bridge over the citation format."
	testRevisedSummary = "Revised abstract."
	testRepoCode       = "https://github.com/projectfile-org/bridge"
	testHomeURL        = "https://projectfile.org"
	testPaperTitle     = "The Paper"
	testOtherTitle     = "Other"
	testDOIExternal    = "10.1/other"
	testZenodoURL      = "https://zenodo.org/records/1"
	testOrgEmail       = "org@example.com"
)

// runMapper resolves a mapper by ExtKey and applies it, returning its report.
func runMapper(t *testing.T, mappers core.MapperList, key string, toPF bool, force bool) string {
	t.Helper()
	m := findMapper(t, mappers, key)
	if toPF {
		require.NotNil(t, m.ToPF, key+" has no ToPF")
		return m.ToPF(force)
	}
	require.NotNil(t, m.FromPF, key+" has no FromPF")
	return m.FromPF(force)
}

// TestMappersRegistry pins the field-by-field contract: every mapper names a stable ExtKey, a PFKey label and both directions, so a field added to the document cannot land half-wired.
func TestMappersRegistry(t *testing.T) {
	c := New()
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	want := []string{
		keyTitle, keyAbstract, keyVersion, keyDateReleased, keyLicense,
		keyRepositoryCode, keyURL, keyKeywords, keyAuthorsContact, keyDOI,
		keyPreferredCitat, keyIdentifiers,
	}
	require.Len(t, mappers, len(want))
	for i, m := range mappers {
		assert.Equal(t, want[i], m.ExtKey, "mapper order must stay stable")
		assert.NotEmpty(t, m.PFKey, m.ExtKey)
		require.NotNil(t, m.ToPF, m.ExtKey)
		require.NotNil(t, m.FromPF, m.ExtKey)
	}
	// BuildMappers stashes the REUSE header because it is the only place the driver holds pf.
	assert.NotEmpty(t, c.ReuseHeader)
}

// TestMapperNilDocument pins the defensive default: a nil external document is replaced by a spec-default one rather than panicking the whole list.
func TestMapperNilDocument(t *testing.T) {
	mappers := buildMappers(nil, &projectfile.Document{})
	require.Len(t, mappers, 12)
	assert.Equal(t, keyTitle, mappers[0].ExtKey)
}

// TestMapperTitleRoundTrip covers the localized title, its idempotence and the force gate.
func TestMapperTitleRoundTrip(t *testing.T) {
	c := New()
	c.Title = testTitle
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testTitle, runMapper(t, mappers, keyTitle, true, false))
	assert.Equal(t, testTitle, projectfile.ExtractLocalizedString(pf.Identity.Title))
	assert.Empty(t, runMapper(t, mappers, keyTitle, true, false), "an in-sync title reports nothing")

	// A declared title is held without force and overwritten with it.
	pf.Identity.Title = &projectfile.LocalizedString{Bare: "Held"}
	assert.Empty(t, runMapper(t, mappers, keyTitle, true, false))
	assert.Equal(t, testTitle, runMapper(t, mappers, keyTitle, true, true))

	// FromPF projects back out and stays silent once equal.
	c.Title = ""
	assert.Equal(t, testTitle, runMapper(t, mappers, keyTitle, false, false))
	assert.Empty(t, runMapper(t, mappers, keyTitle, false, false))
}

// TestMapperAbstractRoundTrip covers the summary mapper including its force-gated clear.
func TestMapperAbstractRoundTrip(t *testing.T) {
	c := New()
	c.Abstract = testSummary
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testSummary, runMapper(t, mappers, keyAbstract, true, false))
	assert.Equal(t, testSummary, projectfile.ExtractLocalizedString(pf.Identity.Summary))
	assert.Empty(t, runMapper(t, mappers, keyAbstract, true, false))

	pf.Identity.Summary = &projectfile.LocalizedString{Bare: "Held"}
	assert.Empty(t, runMapper(t, mappers, keyAbstract, true, false))
	assert.Equal(t, testSummary, runMapper(t, mappers, keyAbstract, true, true))

	// FromPF writes the declared summary out and stays silent once equal.
	pf.Identity.Summary = &projectfile.LocalizedString{Bare: testRevisedSummary}
	assert.Equal(t, testRevisedSummary, runMapper(t, mappers, keyAbstract, false, false))
	assert.Equal(t, testRevisedSummary, c.Abstract)
	assert.Empty(t, runMapper(t, mappers, keyAbstract, false, false))

	pf.Identity.Summary = &projectfile.LocalizedString{}
	assert.Empty(t, runMapper(t, mappers, keyAbstract, false, false))
	assert.Equal(t, testRevisedSummary, c.Abstract, "a gap-fill must leave the value")
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyAbstract, false, true))
	assert.Empty(t, c.Abstract)
}

// TestMapperVersionAndDate covers the two scalar mappers and their clear semantics.
func TestMapperVersionAndDate(t *testing.T) {
	c := New()
	c.Version = testVersion
	c.DateReleased = testReleased
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testVersion, runMapper(t, mappers, keyVersion, true, false))
	assert.Equal(t, testVersion, pf.Identity.Version)
	assert.Empty(t, runMapper(t, mappers, keyVersion, true, false))

	pf.Identity.Version = testNewVersion
	assert.Empty(t, runMapper(t, mappers, keyVersion, true, false), "a declared version is held without force")
	assert.Equal(t, testVersion, runMapper(t, mappers, keyVersion, true, true))

	assert.Equal(t, testReleased, runMapper(t, mappers, keyDateReleased, true, false))
	assert.Equal(t, testReleased, pf.Identity.Released)

	// FromPF: an absent value is held without force and cleared with it.
	pf.Identity.Released = ""
	assert.Empty(t, runMapper(t, mappers, keyDateReleased, false, false))
	assert.Equal(t, testReleased, c.DateReleased)
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyDateReleased, false, true))
	assert.Empty(t, c.DateReleased)

	// A changed pf version flows back out.
	pf.Identity.Version = testNewVersion
	assert.Equal(t, testNewVersion, runMapper(t, mappers, keyVersion, false, false))
	assert.Equal(t, testNewVersion, c.Version)
	assert.Empty(t, runMapper(t, mappers, keyVersion, false, false))
}

// TestMapperLicenseGapFill covers the projectfile-held licence paths.
func TestMapperLicenseGapFill(t *testing.T) {
	c := New()
	c.License = testMIT
	pf := &projectfile.Document{License: &projectfile.License{Spdx: testApache}}
	mappers := buildMappers(c, pf)

	assert.Empty(t, runMapper(t, mappers, keyLicense, true, false), "a declared licence is held without force")
	assert.Equal(t, testMIT, runMapper(t, mappers, keyLicense, true, true))
	assert.Equal(t, testMIT, pf.License.Spdx)
	assert.Empty(t, runMapper(t, mappers, keyLicense, true, false), "an agreeing pair reports nothing")

	// FromPF projects the declared expression out, then clears on absence.
	pf.License = &projectfile.License{Spdx: testApache}
	assert.Equal(t, testApache, runMapper(t, mappers, keyLicense, false, false))
	assert.Equal(t, testApache, c.License)
	assert.Empty(t, runMapper(t, mappers, keyLicense, false, false))

	pf.License = &projectfile.License{}
	assert.Empty(t, runMapper(t, mappers, keyLicense, false, false))
	assert.Equal(t, testApache, c.License, "a gap-fill must leave the value")
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyLicense, false, true))
	assert.Nil(t, c.License)
}

// TestMapperRepositoryCode covers the citable repository slot, including the SSH-clone case where the citable URL lives in links[type=source-code].
func TestMapperRepositoryCode(t *testing.T) {
	c := New()
	c.RepositoryCode = testRepoCode
	pf := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: "ssh://git@kiota.ch/x/y.git", Role: projectfile.RepositoryRoleOrigin},
		},
	}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testRepoCode, runMapper(t, mappers, keyRepositoryCode, true, false))
	require.Len(t, pf.Links, 1)
	assert.Equal(t, projectfile.LinkSourceCode, pf.Links[0].Type)
	assert.Equal(t, testRepoCode, pf.Links[0].URL)
	assert.Empty(t, runMapper(t, mappers, keyRepositoryCode, true, false))

	// FromPF reads the http page back out of the link the mapper just filled.
	c.RepositoryCode = ""
	assert.Equal(t, testRepoCode, runMapper(t, mappers, keyRepositoryCode, false, false))
	assert.Equal(t, testRepoCode, c.RepositoryCode)

	// No source at all: held without force, cleared with it.
	pf.Links = nil
	pf.Repositories = nil
	assert.Empty(t, runMapper(t, mappers, keyRepositoryCode, false, false))
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyRepositoryCode, false, true))
	assert.Empty(t, c.RepositoryCode)
}

// TestMapperHomepageURL covers the homepage link slot and its gap-fill gate.
func TestMapperHomepageURL(t *testing.T) {
	c := New()
	c.URL = testHomeURL
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testHomeURL, runMapper(t, mappers, keyURL, true, false))
	assert.Equal(t, testHomeURL, pfmodel.LinkURL(pf, projectfile.LinkHomepage))
	assert.Empty(t, runMapper(t, mappers, keyURL, true, false))

	// A declared homepage is held without force and overwritten with it.
	pfmodel.SetLink(pf, projectfile.LinkHomepage, "https://held.example", true)
	assert.Empty(t, runMapper(t, mappers, keyURL, true, false))
	assert.Equal(t, testHomeURL, runMapper(t, mappers, keyURL, true, true))

	c.URL = ""
	assert.Equal(t, testHomeURL, runMapper(t, mappers, keyURL, false, false))
	assert.Empty(t, runMapper(t, mappers, keyURL, false, false))

	// No homepage at all: held without force, cleared with it.
	pf.Links = nil
	assert.Empty(t, runMapper(t, mappers, keyURL, false, false))
	assert.Equal(t, testHomeURL, c.URL)
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyURL, false, true))
	assert.Empty(t, c.URL)
}

// TestMapperKeywords covers set comparison, colon-filtering and the force gate.
func TestMapperKeywords(t *testing.T) {
	c := New()
	c.Keywords = []string{"go", "tooling"}
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, "2 keyword(s)", runMapper(t, mappers, keyKeywords, true, false))
	assert.Equal(t, []string{"go", "tooling"}, pf.Keywords)
	assert.Empty(t, runMapper(t, mappers, keyKeywords, true, false))

	// A declared list is held unless force is given.
	pf.Keywords = []string{"held"}
	assert.Empty(t, runMapper(t, mappers, keyKeywords, true, false))
	assert.Equal(t, "2 keyword(s)", runMapper(t, mappers, keyKeywords, true, true))

	// FromPF drops colon-scoped keywords and stays silent once the sets agree.
	pf.Keywords = []string{"go", "org.projectfile:kind"}
	c.Keywords = nil
	assert.Equal(t, "go", runMapper(t, mappers, keyKeywords, false, false))
	assert.Equal(t, []string{"go"}, c.Keywords)
	assert.Empty(t, runMapper(t, mappers, keyKeywords, false, false))

	// Nothing to project: held without force, cleared with it.
	pf.Keywords = []string{"org.projectfile:only"}
	c.Keywords = []string{"go"}
	assert.Empty(t, runMapper(t, mappers, keyKeywords, false, false))
	assert.Equal(t, []string{"go"}, c.Keywords)
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyKeywords, false, true))
	assert.Nil(t, c.Keywords)
}

// TestMapperAuthorsAndContact covers the CFF 1.2.0 consolidation of the three author-like roles into authors[], and the security/maintainer pair into contact[], in both directions.
func TestMapperAuthorsAndContact(t *testing.T) {
	c := New()
	c.Authors = []PersonEntity{
		{
			FamilyNames: testFamilyName, GivenNames: testGivenName, Email: testAuthorEmail,
			Affiliation: testAffiliation, Orcid: testOrcidFull, Website: testHomepage,
		},
	}
	c.Contact = []PersonEntity{{Name: testOrgName, Website: testOrgURL, Orcid: testOrcidFull}}
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	report := runMapper(t, mappers, keyAuthorsContact, true, false)
	assert.Contains(t, report, "person(s)")
	assert.Contains(t, report, "org(s)")
	require.Len(t, pf.People, 1)
	assert.Equal(t, testFamilyName, pf.People[0].FamilyNames)
	assert.Equal(t, testOrcidBare, pf.People[0].Orcid, "the orcid prefix is stripped on the way in")
	assert.Contains(t, pf.People[0].Roles, testAuthorRole)
	require.Len(t, pf.Organizations, 1)
	assert.Equal(t, testOrgName, pf.Organizations[0].Name)
	assert.Equal(t, testOrcidBare, pf.Organizations[0].Orcid)
	assert.Contains(t, pf.Organizations[0].Roles, roleMaintainer)
	assert.Empty(t, runMapper(t, mappers, keyAuthorsContact, true, false), "a re-run reports nothing")

	// FromPF projects the role split back out and drops the legacy arrays.
	pf.People[0].Roles = []string{testAuthorRole, roleContributor}
	pf.Organizations = []projectfile.Organization{
		{Name: testOrgName, Orcid: testOrcidBare, Roles: []string{roleSecurity, roleMaintainer}},
		// No Name: kept because it still carries an identifying field.
		{Email: testOrgEmail, Roles: []string{roleMaintainer}},
		// No identity at all: dropped, a role-only record cannot dedup.
		{Roles: []string{testAuthorRole}},
	}
	c.Authors = nil
	c.Contact = nil
	c.Maintainers = []PersonEntity{{FamilyNames: "Legacy"}}
	c.Contributors = []PersonEntity{{FamilyNames: "Legacy"}}

	report = runMapper(t, mappers, keyAuthorsContact, false, true)
	assert.Equal(t, "5 person(s)", report)
	require.Len(t, c.Authors, 3, "the person and both identified orgs land in authors[]")
	assert.Equal(t, testFamilyName, c.Authors[0].FamilyNames)
	assert.Equal(t, testOrgName, c.Authors[1].Name)
	assert.Equal(t, testOrcidFull, c.Authors[1].Orcid, "the orcid prefix is restored on the way out")
	require.Len(t, c.Contact, 2, "the security and maintainer orgs land in contact[]")
	assert.Equal(t, testOrgName, c.Contact[0].Name)
	assert.Nil(t, c.Maintainers, "CFF 1.2.0 has no maintainers array")
	assert.Nil(t, c.Contributors, "CFF 1.2.0 has no contributors array")

	// An empty projectfile projects nothing at all.
	pf.People = nil
	pf.Organizations = nil
	c.Authors = nil
	c.Contact = nil
	assert.Empty(t, runMapper(t, mappers, keyAuthorsContact, false, false))
	assert.Empty(t, runMapper(t, mappers, keyAuthorsContact, false, true))

	// An empty CITATION.cff teaches the document nothing.
	c.Authors = nil
	c.Contact = nil
	c.Maintainers = nil
	c.Contributors = nil
	pf.People = []projectfile.Person{{FamilyNames: testFamilyName}}
	assert.Empty(t, runMapper(t, mappers, keyAuthorsContact, true, false))
	assert.Len(t, pf.People, 1)
}

// TestMapperDoIExtension covers the citation-extension string slot.
func TestMapperDoIExtension(t *testing.T) {
	c := New()
	c.DOI = testDOIValue
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testDOIValue, runMapper(t, mappers, keyDOI, true, false))
	assert.Equal(t, testDOIValue, citExtString(pf, cffKeyDOI))
	assert.Empty(t, runMapper(t, mappers, keyDOI, true, false))

	// A declared value is held without force and overwritten with it.
	setCitExtString(pf, cffKeyDOI, testDOIExternal)
	assert.Empty(t, runMapper(t, mappers, keyDOI, true, false))
	assert.Equal(t, testDOIValue, runMapper(t, mappers, keyDOI, true, true))
	assert.Equal(t, testDOIValue, citExtString(pf, cffKeyDOI))

	// FromPF projects the extension back out and clears on force.
	c.DOI = ""
	assert.Equal(t, testDOIValue, runMapper(t, mappers, keyDOI, false, false))
	assert.Empty(t, runMapper(t, mappers, keyDOI, false, false))

	pf.Extensions = nil
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyDOI, false, true))
	assert.Empty(t, c.DOI)
}

// TestMapperPreferredCitationRebuilds pins that the preferred-citation mapper rebuilds the reference from the extension on every FromPF run, holding the declared citation on ToPF unless force is given.
func TestMapperPreferredCitationRebuilds(t *testing.T) {
	c := New()
	c.PreferredCitation = &Reference{
		Type: "article", Title: testPaperTitle, Journal: "J. Test", Volume: 4, Issue: 2,
		Pages: "10-20", Year: 2026, DOI: testDOIValue,
	}
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, testPaperTitle, runMapper(t, mappers, keyPreferredCitat, true, false))
	ext, err := pfmodel.GetCitationExtension(pf)
	require.NoError(t, err)
	require.NotNil(t, ext.Preferred)
	assert.Equal(t, testPaperTitle, ext.Preferred.Title)
	assert.Equal(t, 2026, ext.Preferred.Year)

	// A declared preferred citation is held without force and replaced with it.
	c.PreferredCitation = &Reference{Type: "book", Title: testOtherTitle}
	assert.Empty(t, runMapper(t, mappers, keyPreferredCitat, true, false))
	assert.Equal(t, testOtherTitle, runMapper(t, mappers, keyPreferredCitat, true, true))

	// Every FromPF run rebuilds from the extension rather than reporting a change count.
	assert.Equal(t, testOtherTitle, runMapper(t, mappers, keyPreferredCitat, false, false))
	assert.Equal(t, testOtherTitle, runMapper(t, mappers, keyPreferredCitat, false, false))

	pf.Extensions = nil
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyPreferredCitat, false, true))
	assert.Nil(t, c.PreferredCitation)
}

// TestMapperIdentifiers covers the identifier↔link projection both ways.
func TestMapperIdentifiers(t *testing.T) {
	c := New()
	c.Identifiers = []Identifier{
		{Type: cffKeyDOI, Value: testDOIValue},
		{Type: cffKeyURL, Value: testZenodoURL},
		{Type: "swh", Value: "not-a-known-type"},
	}
	pf := &projectfile.Document{}
	mappers := buildMappers(c, pf)

	assert.Equal(t, "2 link(s)", runMapper(t, mappers, keyIdentifiers, true, false))
	require.Len(t, pf.Links, 2, "an unknown identifier type maps to no link")
	assert.Equal(t, "doi-landing", pf.Links[0].Type)
	assert.Equal(t, "zenodo-archive", pf.Links[1].Type)
	assert.Empty(t, runMapper(t, mappers, keyIdentifiers, true, false), "merging is idempotent")

	// A link already present is not duplicated.
	pf.Links = pf.Links[:1]
	assert.Equal(t, "1 link(s)", runMapper(t, mappers, keyIdentifiers, true, false))

	// FromPF projects the two known link types back as identifiers and skips the rest.
	c.Identifiers = nil
	pf.Links = []projectfile.Link{
		{Type: "doi-landing", URL: testDOIValue},
		{Type: "zenodo-archive", URL: testZenodoURL},
		{Type: "homepage", URL: testHomeURL},
	}
	assert.Equal(t, "2 identifier(s)", runMapper(t, mappers, keyIdentifiers, false, false))
	require.Len(t, c.Identifiers, 2)
	assert.Equal(t, cffKeyDOI, c.Identifiers[0].Type)

	pf.Links = nil
	assert.Equal(t, clearedRemoved, runMapper(t, mappers, keyIdentifiers, false, true))
	assert.Nil(t, c.Identifiers)
}
