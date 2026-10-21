// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

const (
	testPyName       = "acme-widget"
	testPyVerOne     = "1.0.0"
	testPyVerTwo     = "2.0.0"
	testPyDesc       = "A widget library"
	testHomeURL      = "https://example.test"
	testIssuesURL    = "https://example.test/issues"
	testDocsURL      = "https://example.test/docs"
	testChangelogURL = "https://example.test/changelog"
	testRepoURL      = "https://example.test/repo.git"
	testFundURL      = "https://example.test/fund"
	testSPDXMIT      = "MIT"
	testSPDXApache   = "Apache-2.0"
	testSPDXGPL      = "GPL-3.0"
	testAuthorName   = "Ada Lovelace"
	testAuthorEmail  = "ada@example.test"
	testClassifier   = "Topic :: Utilities"
)

// Shared label/key literals named once so goconst sees one declaration, not magic strings.
const (
	testNameFixture        = "fixture"
	testLabelHomepage      = "Homepage"
	testLabelHomepageLC    = "homepage"
	testLabelDocumentation = "Documentation"
	testLabelFunding       = "Funding"
	testKeyPython          = "python"
	testDynamicVersion     = "version"
	testEntryMain          = "a:main"
	testScriptPytest       = "pytest"
	testNameGrace          = "Grace"
	testGraceHopper        = "Grace Hopper"
	testGivenAda           = "Ada"
	testLicenseFile        = "LICENSE"
	testLicenseNotice      = "NOTICE"
	testMutatedValue       = "mutated"
	testKeyFile            = "file"
	testKeyText            = "text"
	testVendorAcme         = "acme"
	testRoleMaintainer     = "maintainer"
	testRoleAuthor         = "author"
	testKeywordNamespaced  = "topic:x"
	testKeyScript          = "test"
	testEntryGroup         = "console_scripts"
	testFamilyLovelace     = "Lovelace"
)

// ── Clone ────────────────────────────────────────────────────────────────────

func TestCloneIsIndependentCopy(t *testing.T) {
	assert.Nil(t, (*Document)(nil).Clone())

	dir := writePyproject(t, "[project]\nname = 'fixture'\nversion = '0.1.0'\n"+
		"[[project.authors]]\nname = 'Ada Lovelace'\nemail = 'ada@example.test'\n")
	doc, err := Read(dir)
	require.NoError(t, err)

	cp := doc.Clone()
	require.NotNil(t, cp)
	cp.Project.Name = testPyName
	cp.Project.Version = testPyVerTwo
	cp.Project.Authors[0].Name = "Someone Else"
	cp.Project.Keywords = []string{"kw"}
	cp.Project.URLs = map[string]string{testLabelHomepage: testHomeURL}

	assert.Equal(t, testNameFixture, doc.Project.Name)
	assert.Equal(t, "0.1.0", doc.Project.Version)
	assert.Equal(t, testAuthorName, doc.Project.Authors[0].Name)
	assert.Nil(t, doc.Project.Keywords, "a fresh slice on the clone must not appear on the original")
	assert.Nil(t, doc.Project.URLs)
}

// TestCloneCopiesNestedContainers pins that the heterogeneous readme/licence values deep-copy, so a mutation on the clone leaves the original intact.
func TestCloneCopiesNestedContainers(t *testing.T) {
	doc := &Document{Project: Project{
		License:  map[string]any{"urls": []any{"https://a.test", "https://b.test"}},
		ReadMe:   map[string]any{testKeyFile: "README.md", "content-type": "text/markdown"},
		Keywords: []string{"one"},
	}}
	cp := doc.Clone()
	nested := cp.Project.License.(map[string]any)
	nested["urls"].([]any)[0] = "https://mutated.test"
	nested["added"] = "x"
	cp.Project.ReadMe.(map[string]any)[testKeyFile] = "OTHER.md"
	cp.Project.Keywords[0] = testMutatedValue

	assert.Equal(t, "https://a.test", doc.Project.License.(map[string]any)["urls"].([]any)[0])
	assert.NotContains(t, doc.Project.License.(map[string]any), "added")
	assert.Equal(t, "README.md", doc.Project.ReadMe.(map[string]any)[testKeyFile])
	assert.Equal(t, "one", doc.Project.Keywords[0])
}

// TestCloneFullyPopulatedDocument drives cloneProject over every field so each clone* helper sees its populated branch and the copy stays value-for-value.
func TestCloneFullyPopulatedDocument(t *testing.T) {
	doc := &Document{Project: Project{
		Name: testNameFixture, Version: "1.0.0", Description: "d",
		ReadMe: map[string]any{testKeyFile: "README.md"}, RequiresPython: ">=3.13",
		License: testSPDXMIT, LicenseFiles: []string{testLicenseFile},
		Authors:     []Person{{Name: testAuthorName, Email: testAuthorEmail}},
		Maintainers: []Person{{Name: testNameGrace}},
		Keywords:    []string{"a", "b"}, Classifiers: []string{testClassifier},
		URLs:                 map[string]string{testLabelHomepage: testHomeURL},
		Scripts:              map[string]string{testKeyScript: testScriptPytest},
		GUIScripts:           map[string]string{"gui": "a.gui:main"},
		EntryPoints:          map[string]map[string]string{testEntryGroup: {"w": testEntryMain}},
		Dependencies:         []string{"requests"},
		OptionalDependencies: map[string][]string{"dev": {testScriptPytest}},
		Dynamic:              []string{testDynamicVersion},
	}}
	cp := doc.Clone()
	assert.Equal(t, doc.Project, cp.Project, "a fully populated project clones value-for-value")

	cp.Project.URLs[testLabelHomepage] = "https://mutated.test"
	cp.Project.EntryPoints[testEntryGroup]["w"] = testMutatedValue
	cp.Project.OptionalDependencies["dev"][0] = testMutatedValue
	cp.Project.Scripts[testKeyScript] = testMutatedValue
	cp.Project.Authors[0].Name = testMutatedValue
	assert.Equal(t, testHomeURL, doc.Project.URLs[testLabelHomepage])
	assert.Equal(t, testEntryMain, doc.Project.EntryPoints[testEntryGroup]["w"])
	assert.Equal(t, testScriptPytest, doc.Project.OptionalDependencies["dev"][0])
	assert.Equal(t, testScriptPytest, doc.Project.Scripts[testKeyScript])
	assert.Equal(t, testAuthorName, doc.Project.Authors[0].Name)
}

// ── licence value parsing ──────────────────────────────────────────────────────

func TestParseLicenseValue(t *testing.T) {
	spdx, file := parseLicenseValue(testSPDXMIT)
	assert.Equal(t, testSPDXMIT, spdx)
	assert.Empty(t, file)

	spdx, file = parseLicenseValue("  MIT  ")
	assert.Equal(t, testSPDXMIT, spdx, "a bare SPDX id is trimmed")
	assert.Empty(t, file)

	spdx, file = parseLicenseValue(map[string]any{testKeyText: "Apache-2.0"})
	assert.Equal(t, testSPDXApache, spdx)
	assert.Empty(t, file)

	spdx, file = parseLicenseValue(map[string]any{testKeyFile: "  LICENSE  "})
	assert.Empty(t, spdx)
	assert.Equal(t, testLicenseFile, file)

	// A forbidden character (a slash), not the spaces, is what rejects free prose here.
	spdx, file = parseLicenseValue("proprietary/confidential")
	assert.Empty(t, spdx)
	assert.Empty(t, file)

	for _, v := range []any{nil, map[string]any{testKeyText: "a/b"}, map[string]any{"other": "x"}, 42, []any{"x"}} {
		spdx, file = parseLicenseValue(v)
		assert.Empty(t, spdx, v)
		assert.Empty(t, file, v)
	}
}

func TestLicenseValueAsTable(t *testing.T) {
	got, ok := licenseValueAsTable(nil)
	assert.Nil(t, got)
	assert.False(t, ok)

	got, ok = licenseValueAsTable(testSPDXMIT)
	assert.True(t, ok)
	assert.Equal(t, map[string]any{testKeyText: testSPDXMIT}, got, "a bare string is a {text: ...} view")

	m := map[string]any{testKeyFile: testLicenseFile}
	got, ok = licenseValueAsTable(m)
	assert.True(t, ok)
	assert.Equal(t, m, got)

	got, ok = licenseValueAsTable(42)
	assert.Nil(t, got)
	assert.False(t, ok)
}

func TestLooksLikeSPDX(t *testing.T) {
	assert.False(t, looksLikeSPDX(""))
	for _, s := range []string{testSPDXMIT, testSPDXApache, "MIT OR Apache-2.0", "GPL-3.0-or-later", "A+. b-2"} {
		assert.True(t, looksLikeSPDX(s), s)
	}
	assert.False(t, looksLikeSPDX("MIT (patent grant)"), "a parenthesis is not an SPDX character")
}

func TestLicenseFileHelpers(t *testing.T) {
	assert.Nil(t, licenseFileAsList(nil))
	assert.Nil(t, licenseFileAsList(""), "an empty string is no file")
	assert.Equal(t, []string{testLicenseFile}, licenseFileAsList(testLicenseFile))
	assert.Equal(t, []string{"A", "B"}, licenseFileAsList([]string{"A", "B"}))
	assert.Equal(t, []string{"A"}, licenseFileAsList([]any{"A", 3}), "non-string items are skipped")
	assert.Nil(t, licenseFileAsList(42))

	assert.True(t, equalLicenseFile(testLicenseFile, []string{testLicenseFile}))
	assert.True(t, equalLicenseFile([]any{"A", "B"}, []string{"A", "B"}))
	assert.False(t, equalLicenseFile(testLicenseFile, []string{testLicenseNotice}))
}

// ── slugify / keyword / classifier conversion ──────────────────────────────────

func TestSlugify(t *testing.T) {
	assert.Equal(t, "software-development", slugify("Software Development"))
	assert.Equal(t, "hello-world", slugify("Hello, World!"))
	assert.Equal(t, "4-beta", slugify("4 - Beta"))
	assert.Equal(t, "caf", slugify("Café"), "a non-ASCII letter collapses to a dash")
	assert.Empty(t, slugify("---"))
	assert.Empty(t, slugify(""))
}

func TestKeywordFilters(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, extractBareKeywords([]string{"a", testKeywordNamespaced, "b"}))
	assert.Empty(t, extractBareKeywords([]string{testKeywordNamespaced}))
	assert.Equal(t, []string{testKeywordNamespaced}, extractNamespacedKeywords([]string{"a", testKeywordNamespaced, "b"}))
	assert.Empty(t, extractNamespacedKeywords([]string{"a", "b"}))
}

func TestClassifiersToKeywords(t *testing.T) {
	assert.Equal(t, []string{"topic:utilities"}, classifiersToKeywords([]string{testClassifier}))
	assert.Empty(t, classifiersToKeywords([]string{"Ungrouped"}), "a single trove segment yields nothing")
	assert.Empty(t, classifiersToKeywords(nil))
	assert.Equal(t, "environment:web-environment", classifiersToKeywords([]string{"Environment :: Web Environment"})[0])
}

func TestNormalizePyPIName(t *testing.T) {
	assert.Equal(t, "my-package-name", normalizePyPIName("My_Package.Name"))
	assert.Equal(t, "a-b", normalizePyPIName("a...b"))
	assert.Equal(t, "already-dashed", normalizePyPIName("already-dashed"))
	assert.Empty(t, normalizePyPIName(""))
}

// ── people conversion ──────────────────────────────────────────────────────────

func TestPeopleConversion(t *testing.T) {
	assert.Equal(t, []projectfile.Person{{GivenNames: testGivenAda, FamilyNames: testFamilyLovelace, Email: testAuthorEmail, Roles: []string{testRoleAuthor}}},
		pyPeopleToPF([]Person{{Name: testAuthorName, Email: testAuthorEmail}}, testRoleAuthor))
	assert.Equal(t, []projectfile.Person{{DisplayName: testNameGrace, Roles: []string{testRoleAuthor}}},
		pyPeopleToPF([]Person{{Name: testNameGrace}}, testRoleAuthor), "a single word stays a whole display name")
	assert.Equal(t, []projectfile.Person{{GivenNames: testGivenAda, FamilyNames: "Byron Lovelace", Roles: []string{testRoleAuthor}}},
		pyPeopleToPF([]Person{{Name: "Ada Byron Lovelace"}}, testRoleAuthor), "only the first space splits")
	assert.Empty(t, pyPeopleToPF([]Person{{}}, testRoleAuthor), "a person with neither name nor email is skipped")

	assert.Equal(t, []Person{{Name: testGraceHopper, Email: testAuthorEmail}},
		pfPeopleToPy([]projectfile.Person{{DisplayName: testGraceHopper, Email: testAuthorEmail}}))
	assert.Equal(t, []Person{{Name: "Ada Lovelace"}},
		pfPeopleToPy([]projectfile.Person{{GivenNames: testGivenAda, FamilyNames: testFamilyLovelace}}))
	assert.Empty(t, pfPeopleToPy([]projectfile.Person{{}}), "a nameless, emailless person is skipped")
}

func TestPersonDisplayName(t *testing.T) {
	assert.Equal(t, testGraceHopper, personDisplayName(projectfile.Person{DisplayName: testGraceHopper, GivenNames: testNameGrace, FamilyNames: "Hopper"}),
		"the display name wins over the parsed parts")
	assert.Equal(t, "Ada Lovelace", personDisplayName(projectfile.Person{GivenNames: testGivenAda, FamilyNames: testFamilyLovelace}))
	assert.Equal(t, "Karl van der Moore Jr", personDisplayName(projectfile.Person{GivenNames: "Karl", NameParticle: "van der", NameSuffix: "Jr", FamilyNames: "Moore"}))
	assert.Empty(t, personDisplayName(projectfile.Person{GivenNames: testNameGrace}))
}

func TestFilterByRoleAndEquality(t *testing.T) {
	people := []projectfile.Person{
		{Roles: []string{testRoleAuthor, testRoleMaintainer}, DisplayName: "A"},
		{Roles: []string{testRoleMaintainer}, DisplayName: "B"},
		{DisplayName: "C"},
	}
	assert.Len(t, filterByRole(people, testRoleAuthor), 1)
	assert.Len(t, filterByRole(people, testRoleMaintainer), 2)
	assert.Empty(t, filterByRole(nil, testRoleAuthor))

	assert.True(t, equalPyPersons(nil, nil))
	assert.True(t, equalPyPersons([]Person{{Name: "A"}}, []Person{{Name: "A"}}))
	assert.False(t, equalPyPersons([]Person{{Name: "A"}}, []Person{{Name: "B"}}))
	assert.False(t, equalPyPersons([]Person{{Name: "A"}}, nil))
}

// ── URL lookup helpers ─────────────────────────────────────────────────────────

func TestURLLookupHelpers(t *testing.T) {
	urls := map[string]string{testLabelHomepage: testHomeURL, "Bug Tracker": testIssuesURL}
	key, val, found := findURLEntry(urls, []string{testLabelHomepageLC})
	assert.True(t, found)
	assert.Equal(t, testLabelHomepage, key, "the user-supplied casing is preserved")
	assert.Equal(t, testHomeURL, val)

	_, _, found = findURLEntry(urls, []string{"issues"})
	assert.False(t, found)
	_, _, found = findURLEntry(nil, []string{"issues"})
	assert.False(t, found)

	assert.Equal(t, testIssuesURL, findURLAnyLabel(urls, []string{"bug tracker"}))
	assert.Empty(t, findURLAnyLabel(urls, []string{"changelog"}))
}

func TestDropURL(t *testing.T) {
	py := &Document{Project: Project{URLs: map[string]string{testLabelHomepage: testHomeURL}}}
	assert.Empty(t, dropURL(py, false, []string{testLabelHomepageLC}), "a default sync never drops")
	assert.Contains(t, py.Project.URLs, testLabelHomepage)
	assert.Equal(t, core.Removed, dropURL(py, true, []string{testLabelHomepageLC}))
	assert.NotContains(t, py.Project.URLs, testLabelHomepage)
	assert.Empty(t, dropURL(&Document{}, true, []string{testLabelHomepageLC}), "nothing to drop is a no-op")
}

// ── extension namespace + any-shape helpers ─────────────────────────────────────

func TestExtensionNamespaceHelpers(t *testing.T) {
	pf := &projectfile.Document{}
	assert.Nil(t, extMap(pf))
	_, ok := extGet(pf, pyKeyClassifiers)
	assert.False(t, ok)
	assert.Nil(t, extStringList(pf, pyKeyClassifiers))

	setExtAny(pf, pyKeyClassifiers, []any{testClassifier})
	stored, ok := extGet(pf, pyKeyClassifiers)
	require.True(t, ok)
	assert.Equal(t, []any{testClassifier}, stored)
	assert.Equal(t, map[string]any{pyKeyClassifiers: []any{testClassifier}}, extMap(pf))
	assert.Equal(t, []string{testClassifier}, extStringList(pf, pyKeyClassifiers))

	setExtAny(pf, "scripts", "x") // an existing namespace map is reused, not replaced
	assert.Equal(t, []string{testClassifier}, extStringList(pf, pyKeyClassifiers))
}

func TestAnyShapeConversionHelpers(t *testing.T) {
	assert.Nil(t, stringMapToAny(nil))
	assert.Nil(t, stringMapToAny(map[string]string{}))
	assert.Equal(t, map[string]any{"a": "1"}, stringMapToAny(map[string]string{"a": "1"}))

	assert.Nil(t, anyToStringMap(nil))
	assert.Nil(t, anyToStringMap("scalar"))
	assert.Nil(t, anyToStringMap(map[string]string{}))
	assert.Equal(t, map[string]string{"a": "1"}, anyToStringMap(map[string]string{"a": "1"}))
	assert.Equal(t, map[string]string{"a": "1"}, anyToStringMap(map[string]any{"a": "1", "n": 2}), "non-string values are skipped")

	assert.NotNil(t, cloneMapSS(map[string]string{"a": "1"}))

	assert.Nil(t, entryPointsToAny(nil))
	assert.Nil(t, entryPointsToAny(map[string]map[string]string{}))
	assert.Equal(t, map[string]any{testEntryGroup: map[string]any{"w": testEntryMain}},
		entryPointsToAny(map[string]map[string]string{testEntryGroup: {"w": testEntryMain}}))
	assert.Nil(t, anyToEntryPoints("scalar"))
	assert.Nil(t, anyToEntryPoints(map[string]any{}))
	assert.Nil(t, anyToEntryPoints(map[string]any{"bad": "scalar"}), "an unconvertible group is dropped")
	assert.Equal(t, map[string]map[string]string{testEntryGroup: {"w": "a.w:main"}},
		anyToEntryPoints(map[string]any{testEntryGroup: map[string]any{"w": "a.w:main"}}))

	assert.Nil(t, optionalDepsToAny(nil))
	assert.Equal(t, map[string]any{testKeyScript: []any{testScriptPytest}}, optionalDepsToAny(map[string][]string{testKeyScript: {testScriptPytest}}))
	assert.Equal(t, map[string][]string{testKeyScript: {testScriptPytest}},
		anyToOptionalDeps(map[string]any{testKeyScript: []any{testScriptPytest}}))

	assert.Nil(t, stringsToAny(nil))
	assert.Equal(t, []any{"a"}, stringsToAny([]string{"a"}))
	assert.Nil(t, anyToStrings(nil))
	assert.Equal(t, []string{"a"}, anyToStrings([]string{"a"}))
	assert.Equal(t, []string{"a", "b"}, anyToStrings([]any{"a", "b", 3}), "non-string items are skipped")
	assert.Nil(t, anyToStrings("scalar"))
}

func TestEqualityAndGenericHelpers(t *testing.T) {
	assert.True(t, equalStringSlice(nil, nil))
	assert.True(t, equalStringSlice([]string{"a"}, []string{"a"}))
	assert.False(t, equalStringSlice([]string{"a"}, []string{"b"}))
	assert.False(t, equalStringSlice([]string{"a"}, []string{"a", "b"}))

	assert.True(t, equalStringSet([]string{"a", "b"}, []string{"b", "a"}), "a multiset ignores order")
	assert.False(t, equalStringSet([]string{"a", "a"}, []string{"a", "b"}))
	assert.True(t, containsString([]string{"a"}, "a"))
	assert.False(t, containsString(nil, "a"))

	assert.Equal(t, []string{"a", "b", "c"}, mergeUniqueStrings([]string{"a", "b"}, []string{"b", "c"}))
	assert.Empty(t, mergeUniqueStrings(nil, nil))

	for _, v := range []any{nil, "", []any{}, []string{}, map[string]any{}, map[string]string{}} {
		assert.True(t, isEmptyAny(v), v)
	}
	for _, v := range []any{false, 0, testSPDXMIT, []any{"a"}, map[string]any{"a": 1}} {
		assert.False(t, isEmptyAny(v), v)
	}

	assert.Equal(t, "<nil>", formatAny(nil))
	assert.Equal(t, "s:MIT", formatAny(testSPDXMIT))
	assert.Equal(t, "l[s:a,s:b]", formatAny([]any{"a", "b"}))
	assert.Equal(t, "m{a=s:1,b=s:2}", formatAny(map[string]any{"b": "2", "a": "1"}))
	assert.Equal(t, "true", formatAny(true), "no bool special-case: %v fallback")
	assert.Equal(t, "3", formatAny(3))

	assert.True(t, deepEqualAny([]string{"a"}, []any{"a"}))
	assert.True(t, deepEqualAny(map[string]string{"a": "1"}, map[string]any{"a": "1"}))
	assert.False(t, deepEqualAny("a", "b"))
	assert.Equal(t, "l[s:a]", formatAny(normaliseAny([]string{"a"})))
	assert.Equal(t, "s:plain", formatAny(normaliseAny("plain")))
}

func TestEnsureHelpers(t *testing.T) {
	pf := &projectfile.Document{}
	ensureLicense(pf)
	require.NotNil(t, pf.License)
	ensureRequirements(pf)
	require.NotNil(t, pf.Requirements)

	seeded := &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}, Requirements: &projectfile.Requirements{}}
	ensureLicense(seeded)
	assert.Equal(t, testSPDXMIT, seeded.License.Spdx, "an existing licence slot is never replaced")
	ensureRequirements(seeded)
	require.NotNil(t, seeded.Requirements)
}

// ── funding helpers ────────────────────────────────────────────────────────────

func TestPyFundingHelpers(t *testing.T) {
	assert.False(t, hasPyFunding(nil))
	assert.False(t, hasPyFunding(&pfmodel.FundingExtension{}))
	assert.True(t, hasPyFunding(&pfmodel.FundingExtension{GitHub: []string{testVendorAcme}}))
	assert.True(t, hasPyFunding(&pfmodel.FundingExtension{Patreon: testVendorAcme}))
	assert.True(t, hasPyFunding(&pfmodel.FundingExtension{Custom: []string{testFundURL}}))

	assert.Empty(t, pyFundingExtToMap(&pfmodel.FundingExtension{}))

	ext := &pfmodel.FundingExtension{
		GitHub: []string{testVendorAcme}, Patreon: testVendorAcme, KoFi: testVendorAcme, Liberapay: testVendorAcme,
		Tidelift: testVendorAcme, CommunityBridge: testVendorAcme, IssueHunt: testVendorAcme, OpenCollective: testVendorAcme,
		LFXCrowdfunding: testVendorAcme, Polar: testVendorAcme, BuyMeACoffee: testVendorAcme, ThanksDev: testVendorAcme,
		Custom: []string{testFundURL}, Path: "acme.yml", EntityType: "person", EntityRole: testRoleMaintainer,
	}
	m := pyFundingExtToMap(ext)
	assert.Equal(t, []any{testVendorAcme}, m["github"])
	assert.Equal(t, []any{testFundURL}, m["custom"])
	assert.Equal(t, testVendorAcme, m["patreon"])
	assert.Equal(t, testVendorAcme, m["ko-fi"])
	assert.Equal(t, testVendorAcme, m["buy-me-a-coffee"])
	assert.Equal(t, "acme.yml", m["path"])
	assert.Equal(t, "person", m["entity-type"])
	assert.Equal(t, testRoleMaintainer, m["entity-role"])
	assert.NotContains(t, m, "channels", "list-valued sub-objects are pyproject-irrelevant")
}

// ── per-field mapper closures ──────────────────────────────────────────────────

func TestMapName(t *testing.T) {
	assert.Equal(t, testPyName, mapperByExtKey(t, buildMappers(&Document{Project: Project{Name: testPyName}}, &projectfile.Document{}), "name").ToPF(false))

	normalised := &Document{Project: Project{Name: "My_Package"}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(normalised, &projectfile.Document{Identity: projectfile.Identity{Name: "my-package"}}), "name").ToPF(true),
		"PEP 503 normalisation makes the values agree")

	held := &Document{Project: Project{Name: testPyName}}
	heldPF := &projectfile.Document{Identity: projectfile.Identity{Name: "other"}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "name").ToPF(false))
	assert.Equal(t, testPyName, mapperByExtKey(t, buildMappers(held, heldPF), "name").ToPF(true))
	assert.Equal(t, testPyName, heldPF.Identity.Name)

	assert.Equal(t, testPyName, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{Identity: projectfile.Identity{Name: testPyName}}), "name").FromPF(false))
}

func TestMapVersionClosure(t *testing.T) {
	assert.Equal(t, testPyVerOne, mapperByExtKey(t, buildMappers(&Document{Project: Project{Version: testPyVerOne}}, &projectfile.Document{}), testDynamicVersion).ToPF(false))

	agreed := &Document{Project: Project{Version: testPyVerOne}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{Identity: projectfile.Identity{Version: testPyVerOne}}), testDynamicVersion).ToPF(true))

	assert.Equal(t, testPyVerTwo, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{Identity: projectfile.Identity{Version: testPyVerTwo}}), testDynamicVersion).FromPF(false))
}

func TestMapDescriptionClosure(t *testing.T) {
	fresh := &Document{Project: Project{Description: testPyDesc}}
	freshPF := &projectfile.Document{}
	assert.Equal(t, testPyDesc, mapperByExtKey(t, buildMappers(fresh, freshPF), "description").ToPF(false))
	assert.Equal(t, testPyDesc, projectfile.ExtractLocalizedString(freshPF.Identity.Summary))

	agreed := &Document{Project: Project{Description: testPyDesc}}
	agreedPF := &projectfile.Document{}
	projectfile.SetLocalizedEN(&agreedPF.Identity.Summary, testPyDesc)
	assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), "description").FromPF(true), "an agreed summary is a no-op")

	blank := &Document{Project: Project{Description: testPyDesc}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{}), "description").FromPF(false), "a default sync never destroys")
	assert.Equal(t, testPyDesc, blank.Project.Description)
	assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(&Document{Project: Project{Description: testPyDesc}}, &projectfile.Document{}), "description").FromPF(true))
}

func TestMapRequiresPythonToPF(t *testing.T) {
	fresh := &Document{Project: Project{RequiresPython: testRequiresPython}}
	pf := &projectfile.Document{}
	assert.Equal(t, testRequiresPython, mapperByExtKey(t, buildMappers(fresh, pf), testRequiresPythonField).ToPF(false))
	assert.Equal(t, testRequiresPython, pf.Requirements.Runtime[testKeyPython])

	dyn := &Document{Project: Project{RequiresPython: testRequiresPython, Dynamic: []string{"requires-python"}}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(dyn, &projectfile.Document{}), testRequiresPythonField).ToPF(false))

	held := &Document{Project: Project{RequiresPython: testRequiresPython}}
	heldPF := &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testKeyPython: testPrevPython}}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), testRequiresPythonField).ToPF(false), "a declared floor is held without force")
	assert.Equal(t, testRequiresPython, mapperByExtKey(t, buildMappers(held, heldPF), testRequiresPythonField).ToPF(true))
}

func TestMapLicense(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		fresh := &Document{Project: Project{License: testSPDXMIT}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, testSPDXMIT, mapperByExtKey(t, buildMappers(fresh, freshPF), "license").ToPF(false))
		require.NotNil(t, freshPF.License)
		assert.Equal(t, testSPDXMIT, freshPF.License.Spdx)

		agreed := &Document{Project: Project{License: testSPDXMIT}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").ToPF(true))

		held := &Document{Project: Project{License: testSPDXMIT}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXGPL}}), "license").ToPF(false))

		fileDoc := &Document{Project: Project{License: map[string]any{testKeyFile: testLicenseFile}}}
		filePF := &projectfile.Document{}
		assert.Equal(t, testLicenseFile, mapperByExtKey(t, buildMappers(fileDoc, filePF), "license").ToPF(false))
		assert.Equal(t, testLicenseFile, filePF.License.File)

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "license").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		blank := &Document{Project: Project{License: testSPDXMIT}}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{}), "license").FromPF(true))
		assert.Nil(t, blank.Project.License)

		assert.Equal(t, testSPDXMIT, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").FromPF(false))

		agreed := &Document{Project: Project{License: testSPDXMIT}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").FromPF(true))

		fileDoc := &Document{}
		assert.Equal(t, testLicenseFile, mapperByExtKey(t, buildMappers(fileDoc, &projectfile.Document{License: &projectfile.License{File: testLicenseFile}}), "license").FromPF(false))
		assert.Equal(t, map[string]any{testKeyFile: testLicenseFile}, fileDoc.Project.License)

		fileAgreed := &Document{Project: Project{License: map[string]any{testKeyFile: testLicenseFile}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(fileAgreed, &projectfile.Document{License: &projectfile.License{File: testLicenseFile}}), "license").FromPF(true))
	})
}

func TestMapLicenseFiles(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		fresh := &Document{Project: Project{LicenseFiles: []string{testLicenseFile, testLicenseNotice}}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, "2 file(s)", mapperByExtKey(t, buildMappers(fresh, freshPF), "license-files").ToPF(false))
		assert.Equal(t, []string{testLicenseFile, testLicenseNotice}, freshPF.License.File)

		single := &Document{Project: Project{LicenseFiles: []string{testLicenseFile}}}
		singlePF := &projectfile.Document{}
		assert.Equal(t, "1 file(s)", mapperByExtKey(t, buildMappers(single, singlePF), "license-files").ToPF(false))
		assert.Equal(t, testLicenseFile, singlePF.License.File, "a single entry is a bare string")

		held := &Document{Project: Project{LicenseFiles: []string{testLicenseFile}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, &projectfile.Document{License: &projectfile.License{File: "OTHER"}}), "license-files").ToPF(false))
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "license-files").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		list := &Document{}
		assert.Equal(t, "2 file(s)", mapperByExtKey(t, buildMappers(list, &projectfile.Document{License: &projectfile.License{File: []string{"A", "B"}}}), "license-files").FromPF(false))
		assert.Equal(t, []string{"A", "B"}, list.Project.LicenseFiles)

		single := &Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(single, &projectfile.Document{License: &projectfile.License{File: "A"}}), "license-files").FromPF(true),
			"a single string flows through maplicense, not license-files")

		blank := &Document{Project: Project{LicenseFiles: []string{"A"}}}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{}), "license-files").FromPF(true))
	})
}

func TestMapPeople(t *testing.T) {
	t.Run("to_pf_merges", func(t *testing.T) {
		doc := &Document{Project: Project{Authors: []Person{{Name: testAuthorName, Email: testAuthorEmail}}}}
		pf := &projectfile.Document{}
		assert.Equal(t, "1 author(s) merged", mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false))
		require.Len(t, pf.People, 1)
		assert.Equal(t, testAuthorEmail, pf.People[0].Email)
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false), "a second merge is a no-op")
	})

	t.Run("to_pf_empty", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "authors").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		doc := &Document{}
		pf := &projectfile.Document{People: []projectfile.Person{{DisplayName: testGraceHopper, Roles: []string{testRoleAuthor}}}}
		assert.Equal(t, "1 author(s)", mapperByExtKey(t, buildMappers(doc, pf), "authors").FromPF(false))
		assert.Equal(t, []Person{{Name: testGraceHopper}}, doc.Project.Authors)

		agreed := &Document{Project: Project{Authors: []Person{{Name: testGraceHopper}}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{People: []projectfile.Person{{DisplayName: testGraceHopper, Roles: []string{testRoleAuthor}}}}), "authors").FromPF(true))

		wrongRole := &Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(wrongRole, &projectfile.Document{People: []projectfile.Person{{DisplayName: "G", Roles: []string{testRoleMaintainer}}}}), "authors").FromPF(true))

		cleared := &Document{Project: Project{Authors: []Person{{Name: testGraceHopper}}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(cleared, &projectfile.Document{}), "authors").FromPF(false), "a default sync keeps an unmodeled list")
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(cleared, &projectfile.Document{}), "authors").FromPF(true))
		assert.Nil(t, cleared.Project.Authors)
	})
}

func TestMapMaintainers(t *testing.T) {
	doc := &Document{Project: Project{Maintainers: []Person{{Name: testAuthorName}}}}
	pf := &projectfile.Document{}
	assert.Equal(t, "1 maintainer(s) merged", mapperByExtKey(t, buildMappers(doc, pf), "maintainers").ToPF(false))
	assert.Equal(t, []string{testRoleMaintainer}, pf.People[0].Roles)
}

func TestMapKeywords(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		fresh := &Document{Project: Project{Keywords: []string{"a", "b"}}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, "2 keyword(s)", mapperByExtKey(t, buildMappers(fresh, freshPF), "keywords").ToPF(false))
		assert.Equal(t, []string{"a", "b"}, freshPF.Keywords)

		held := &Document{Project: Project{Keywords: []string{"b"}}}
		heldPF := &projectfile.Document{Keywords: []string{"a", testKeywordNamespaced}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "keywords").ToPF(false), "an existing bare set is held without force")
		forced := &Document{Project: Project{Keywords: []string{"b"}}}
		forcedPF := &projectfile.Document{Keywords: []string{"a", testKeywordNamespaced}}
		assert.Equal(t, "1 keyword(s)", mapperByExtKey(t, buildMappers(forced, forcedPF), "keywords").ToPF(true))
		assert.Equal(t, []string{"b", testKeywordNamespaced}, forcedPF.Keywords, "the namespaced keyword survives")

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "keywords").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		doc := &Document{}
		assert.Equal(t, "1 keyword(s)", mapperByExtKey(t, buildMappers(doc, &projectfile.Document{Keywords: []string{"a", testKeywordNamespaced}}), "keywords").FromPF(false))
		assert.Equal(t, []string{"a"}, doc.Project.Keywords, "only bare keywords reach the file")

		agreed := &Document{Project: Project{Keywords: []string{"a"}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{Keywords: []string{"a"}}), "keywords").FromPF(true))

		blank := &Document{Project: Project{Keywords: []string{"a"}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{Keywords: []string{testKeywordNamespaced}}), "keywords").FromPF(false), "no bare pf value keeps the file")
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(&Document{Project: Project{Keywords: []string{"a"}}}, &projectfile.Document{Keywords: []string{testKeywordNamespaced}}), "keywords").FromPF(true))
	})
}

func TestMapClassifiers(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		fresh := &Document{Project: Project{Classifiers: []string{testClassifier}}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, "1 classifier(s)", mapperByExtKey(t, buildMappers(fresh, freshPF), "classifiers").ToPF(false))
		assert.Equal(t, []string{testClassifier}, extStringList(freshPF, pyKeyClassifiers), "the original strings are stashed")
		assert.Equal(t, []string{"topic:utilities"}, freshPF.Keywords, "the derived keyword projects onto pf")

		held := &Document{Project: Project{Classifiers: []string{testClassifier}}}
		heldPF := &projectfile.Document{}
		projectfile.SetExtension(heldPF, extNamespace, map[string]any{pyKeyClassifiers: []any{"Other"}})
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "classifiers").ToPF(false), "a stashed list is held without force")

		agreed := &Document{Project: Project{Classifiers: []string{testClassifier}}}
		agreedPF := &projectfile.Document{}
		projectfile.SetExtension(agreedPF, extNamespace, map[string]any{pyKeyClassifiers: []any{testClassifier}})
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), "classifiers").ToPF(true))

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "classifiers").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		doc := &Document{}
		pf := &projectfile.Document{}
		projectfile.SetExtension(pf, extNamespace, map[string]any{pyKeyClassifiers: []any{"A", "B"}})
		assert.Equal(t, "2 classifier(s)", mapperByExtKey(t, buildMappers(doc, pf), "classifiers").FromPF(false))
		assert.Equal(t, []string{"A", "B"}, doc.Project.Classifiers)

		blank := &Document{Project: Project{Classifiers: []string{"A"}}}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{}), "classifiers").FromPF(true))
	})
}

// TestMapVisibilityDirections drives mapVisibility directly: it shares the "classifiers" ExtKey with the classifiers mapper, so mapperByExtKey would resolve the wrong closure.
func TestMapVisibilityDirections(t *testing.T) {
	drop := &Document{Project: Project{Classifiers: []string{testClassifier, classifierPrivate}}}
	assert.Equal(t, core.Removed, mapVisibility(drop, &projectfile.Document{Identity: projectfile.Identity{Visibility: projectfile.VisibilityPublic}}).FromPF(true))
	assert.Equal(t, []string{testClassifier}, drop.Project.Classifiers, "the marker is dropped from a public document")

	grow := &Document{Project: Project{Classifiers: []string{testClassifier}}}
	assert.Equal(t, classifierPrivate, mapVisibility(grow, &projectfile.Document{Identity: projectfile.Identity{Visibility: projectfile.VisibilityPrivate}}).FromPF(false))
	assert.Contains(t, grow.Project.Classifiers, classifierPrivate)

	agreePrivate := &Document{Project: Project{Classifiers: []string{testClassifier, classifierPrivate}}}
	assert.Empty(t, mapVisibility(agreePrivate, &projectfile.Document{Identity: projectfile.Identity{Visibility: projectfile.VisibilityPrivate}}).FromPF(true))

	guarded := &Document{Project: Project{Classifiers: []string{classifierPrivate}}}
	taught := &projectfile.Document{}
	assert.Equal(t, classifierPrivate, mapVisibility(guarded, taught).ToPF(false))
	assert.Equal(t, projectfile.VisibilityPrivate, taught.Identity.Visibility)

	declared := &projectfile.Document{Identity: projectfile.Identity{Visibility: projectfile.VisibilityPublic}}
	assert.Empty(t, mapVisibility(&Document{Project: Project{Classifiers: []string{classifierPrivate}}}, declared).ToPF(false), "a declared visibility is not overwritten by the file")
	assert.Equal(t, projectfile.VisibilityPublic, declared.Identity.Visibility)
}

func TestMapExtensionFieldPassthrough(t *testing.T) {
	t.Run("scripts", func(t *testing.T) {
		fresh := &Document{Project: Project{Scripts: map[string]string{testKeyScript: testScriptPytest}}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, "m{test=s:pytest}", mapperByExtKey(t, buildMappers(fresh, freshPF), "scripts").ToPF(false))
		assert.Empty(t, mapperByExtKey(t, buildMappers(fresh, freshPF), "scripts").ToPF(false), "an agreed stash is a no-op")

		pulled := &Document{}
		pulledPF := &projectfile.Document{}
		projectfile.SetExtension(pulledPF, extNamespace, map[string]any{"scripts": map[string]any{testKeyScript: "ruff"}})
		assert.Equal(t, "m{test=s:ruff}", mapperByExtKey(t, buildMappers(pulled, pulledPF), "scripts").FromPF(false))
		assert.Equal(t, map[string]string{testKeyScript: "ruff"}, pulled.Project.Scripts)

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "scripts").ToPF(true))
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "scripts").FromPF(false), "no stash is nothing to apply")
	})

	t.Run("dynamic_round_trips", func(t *testing.T) {
		doc := &Document{Project: Project{Dynamic: []string{testDynamicVersion}}}
		pf := &projectfile.Document{}
		assert.NotEmpty(t, mapperByExtKey(t, buildMappers(doc, pf), "dynamic").ToPF(false))
		back := &Document{}
		assert.Equal(t, "l[s:version]", mapperByExtKey(t, buildMappers(back, pf), "dynamic").FromPF(false))
		assert.Equal(t, []string{testDynamicVersion}, back.Project.Dynamic)
	})
}

func TestMapURLStrings(t *testing.T) {
	t.Run("homepage_round_trips", func(t *testing.T) {
		doc := &Document{Project: Project{URLs: map[string]string{testLabelHomepage: testHomeURL}}}
		pf := &projectfile.Document{}
		assert.Equal(t, testHomeURL, mapperByExtKey(t, buildMappers(doc, pf), "urls.Homepage").ToPF(false))
		assert.Equal(t, testHomeURL, pfmodel.LinkURL(pf, projectfile.LinkHomepage))

		back := &Document{}
		assert.Equal(t, testHomeURL, mapperByExtKey(t, buildMappers(back, &projectfile.Document{Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: testHomeURL}}}), "urls.Homepage").FromPF(false))
		assert.Equal(t, testHomeURL, back.Project.URLs[testLabelHomepage])

		dropped := &Document{Project: Project{URLs: map[string]string{testLabelHomepage: testHomeURL}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(dropped, &projectfile.Document{}), "urls.Homepage").FromPF(false), "a default sync never drops a link")
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(dropped, &projectfile.Document{}), "urls.Homepage").FromPF(true))
	})

	t.Run("issues_accepts_aliases", func(t *testing.T) {
		doc := &Document{Project: Project{URLs: map[string]string{"Bug Tracker": testIssuesURL}}}
		pf := &projectfile.Document{}
		assert.Equal(t, testIssuesURL, mapperByExtKey(t, buildMappers(doc, pf), "urls.Issues").ToPF(false))
		assert.Equal(t, testIssuesURL, pfmodel.LinkURL(pf, projectfile.LinkBugs))
	})

	t.Run("repository_follows_packages", func(t *testing.T) {
		doc := &Document{Project: Project{URLs: map[string]string{"Repository": testRepoURL}}}
		pf := &projectfile.Document{}
		assert.Equal(t, testRepoURL, mapperByExtKey(t, buildMappers(doc, pf), "urls.Repository").ToPF(false))
		require.NotNil(t, pfmodel.PackagesRepository(pf))
		assert.Equal(t, testRepoURL, pfmodel.PackagesRepository(pf).URL)
		assert.Equal(t, "git", pfmodel.PackagesRepository(pf).Type)
	})

	t.Run("changelog_and_docs", func(t *testing.T) {
		changelog := &Document{Project: Project{URLs: map[string]string{"Changelog": testChangelogURL}}}
		assert.Equal(t, testChangelogURL, mapperByExtKey(t, buildMappers(changelog, &projectfile.Document{}), "urls.Changelog").ToPF(false))

		docs := &Document{Project: Project{URLs: map[string]string{testLabelDocumentation: testDocsURL}}}
		pf := &projectfile.Document{}
		assert.Equal(t, testDocsURL, mapperByExtKey(t, buildMappers(docs, pf), "urls.Documentation").ToPF(false))
		assert.Equal(t, testDocsURL, pfmodel.MainDocumentationURL(pf))
	})
}

func TestMapURLFunding(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		fresh := &Document{Project: Project{URLs: map[string]string{testLabelFunding: testFundURL}}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, testFundURL, mapperByExtKey(t, buildMappers(fresh, freshPF), "urls.Funding").ToPF(false))
		ext, err := pfmodel.GetFundingExtension(freshPF)
		require.NoError(t, err)
		require.NotNil(t, ext)
		assert.Equal(t, []string{testFundURL}, ext.Custom)

		held := &Document{Project: Project{URLs: map[string]string{"Donate": testFundURL}}}
		heldPF := &projectfile.Document{}
		projectfile.SetExtension(heldPF, pfmodel.FundingExtensionNS, map[string]any{"patreon": testVendorAcme})
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "urls.Funding").ToPF(false), "a foreign funding entry is held without force")

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "urls.Funding").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		doc := &Document{}
		pf := &projectfile.Document{}
		projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{"custom": []any{testFundURL}})
		assert.Equal(t, testFundURL, mapperByExtKey(t, buildMappers(doc, pf), "urls.Funding").FromPF(false))
		assert.Equal(t, testFundURL, doc.Project.URLs[testLabelFunding])

		agreed := &Document{Project: Project{URLs: map[string]string{testLabelFunding: testFundURL}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "urls.Funding").FromPF(true))

		dropped := &Document{Project: Project{URLs: map[string]string{testLabelFunding: testFundURL}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(dropped, &projectfile.Document{}), "urls.Funding").FromPF(false), "a default sync never drops")
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(dropped, &projectfile.Document{}), "urls.Funding").FromPF(true))
		assert.NotContains(t, dropped.Project.URLs, testLabelFunding)
	})
}

// ── Bridge surface ─────────────────────────────────────────────────────────────

func TestBridgeSurface(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "pyproject", b.Name())
	assert.Equal(t, Filename, b.Filename())
	assert.Nil(t, b.Aliases())
	assert.Equal(t, []string{testKeyPython}, b.Stacks())
	assert.Equal(t, core.Policy{}, b.Policy())

	ext, pfLabel := b.Labels()
	assert.Equal(t, Filename, ext)
	assert.Equal(t, "projectfile", pfLabel)

	dir := writePyproject(t, "[project]\nname = 'fixture'\nversion = '0.1.0'\n")
	assert.True(t, b.Exists(dir))
	assert.False(t, b.Exists(t.TempDir()))
	assert.Equal(t, FullPath(dir), b.FullPath(dir, nil))

	read, err := b.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testNameFixture, read.(*Document).Project.Name)
	assert.IsType(t, &Document{}, b.NewEmpty())

	assert.Equal(t, 23, len(b.BuildMappers(nil, &projectfile.Document{})), "twenty-three mapped PEP 621 fields")

	clone := b.Clone(&Document{Project: Project{Name: testPyName}})
	assert.Equal(t, testPyName, clone.(*Document).Project.Name)
	assert.Equal(t, "anything else", b.Clone("anything else"), "a foreign value passes through")
	assert.Nil(t, b.Clone(nil))

	err = b.Write(t.TempDir(), "not a pyproject document")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected document type")
}
