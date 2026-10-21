// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// ── vocabulary translators ───────────────────────────────────────────────────

// TestPlatformVocabularyRoundTrip pins every vocabulary pair at both ends, plus
// the `!`-prefixed negation markers npm writes and the pass-through default.
func TestPlatformVocabularyRoundTrip(t *testing.T) {
	cases := []struct {
		npm string
		pf  string
	}{
		{npm: fxWin32, pf: fxWindows},
		{npm: "!win32", pf: ""},
		{npm: fxDarwin, pf: fxDarwin},
		{npm: "sunos", pf: "sunos"},
	}
	for _, tc := range cases {
		from := npmOSToPF([]string{tc.npm})
		if tc.pf == "" {
			assert.Empty(t, from, "%s is a negation and is dropped", tc.npm)
			continue
		}
		require.Equal(t, []string{tc.pf}, from, tc.npm)
		assert.Equal(t, []string{tc.npm}, osListToNPM(from), tc.npm)
	}
	assert.Equal(t, []string{fxWindows}, npmOSToPF([]string{fxWindows}),
		"the spec spelling also reads as windows")
	assert.Equal(t, []string{fxWin32}, osListToNPM([]string{fxWindows}),
		"the spec spelling writes back as the npm one")

	assert.Equal(t, fxX64, archListToNPM([]string{fxAMD64})[0])
	assert.Equal(t, fxArm64, archListToNPM([]string{fxArm64})[0])
	assert.Equal(t, fxIa32, archListToNPM([]string{fx386})[0])
	assert.Equal(t, fxRiscv, archListToNPM([]string{fxRiscv})[0],
		"an architecture outside the vocabulary passes through unchanged")
	assert.Equal(t, []string{fxAMD64}, npmCPUToPF([]string{fxX64}))
	assert.Equal(t, []string{fx386}, npmCPUToPF([]string{fxIa32}))
	assert.Equal(t, []string{fxArm64}, npmCPUToPF([]string{fxArm64}))
	assert.Equal(t, []string{fxRiscv}, npmCPUToPF([]string{fxRiscv}))
	assert.Empty(t, npmCPUToPF([]string{"!x64"}), "a negated cpu is dropped, not mirrored")

	assert.Empty(t, osListToNPM(nil), "an empty list yields no list rather than an empty one")
	assert.Empty(t, archListToNPM(nil))
	assert.Empty(t, npmOSToPF(nil))
	assert.Empty(t, npmCPUToPF(nil))
}

// ── name conversion ──────────────────────────────────────────────────────────

func TestNPMNameToPFName(t *testing.T) {
	ns, name := npmNameToPFName(fxNpmScopedName, "other")
	assert.Equal(t, fxNpmNamespaceNS, ns, "a scoped package gets its own npm.<scope> namespace")
	assert.Equal(t, fxNpmName, name)

	ns, name = npmNameToPFName("@widget", "other")
	assert.Empty(t, ns, "a scope with no slash is not a namespace")
	assert.Equal(t, fxNpmName, name, "the scope still yields the bare name")

	ns, name = npmNameToPFName("persona-frontend", "me.dbuho.persona")
	assert.Equal(t, "me.dbuho.persona", ns, "the pf namespace last segment is the prefix")
	assert.Equal(t, "frontend", name, "the prefix is stripped off")

	ns, name = npmNameToPFName(fxNpmName, fxNpmScope)
	assert.Empty(t, ns, "a name with no matching prefix carries no namespace")
	assert.Equal(t, fxNpmName, name)

	ns, name = npmNameToPFName(fxNpmName, "")
	assert.Empty(t, ns)
	assert.Equal(t, fxNpmName, name, "an empty pf namespace cannot supply a prefix")
}

func TestPFNameToNPM(t *testing.T) {
	assert.Equal(t, "persona-frontend", pfNameToNPM("me.dbuho.persona", "frontend"),
		"the pf namespace last segment becomes a dash prefix")
	assert.Equal(t, "persona-frontend", pfNameToNPM("me.dbuho.persona", "persona-frontend"),
		"a pf name already carrying the prefix is not doubled")
	assert.Equal(t, fxNpmScopedName, pfNameToNPM(fxNpmNamespaceNS, fxNpmName),
		"an npm.<scope> namespace projects as a scoped package")
	assert.Equal(t, "acme-widget", pfNameToNPM(fxNpmScope, fxNpmName),
		"a namespace whose last segment is the whole string still prefixes")
	assert.Equal(t, fxNpmName, pfNameToNPM("", fxNpmName), "no namespace means no prefix at all")
	assert.Empty(t, pfNameToNPM(fxNpmScope, ""), "a document with no name projects nothing")
}

func TestNamespaceLastSegment(t *testing.T) {
	assert.Equal(t, "persona", namespaceLastSegment("me.dbuho.persona"))
	assert.Equal(t, fxNpmScope, namespaceLastSegment(fxNpmScope))
	assert.Empty(t, namespaceLastSegment(""))
}

// ── equality helpers ─────────────────────────────────────────────────────────

func TestEqualityHelpers(t *testing.T) {
	assert.True(t, equalStringSlice(nil, []string{}), "nil and an empty list are equivalent")
	assert.True(t, equalStringSlice([]string{"a"}, []string{"a"}))
	assert.False(t, equalStringSlice([]string{"a"}, []string{"a", "b"}), "length is compared first")
	assert.False(t, equalStringSlice([]string{"a", "b"}, []string{"a", "c"}), "order is compared")

	assert.True(t, equalStringMap(nil, map[string]string{}), "nil and an empty map are equivalent")
	assert.True(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"a": "1"}))
	assert.False(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"a": "2"}))
	assert.False(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"b": "1"}))
	assert.False(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}))

	assert.True(t, equalPersonList(nil, []Person{}), "nil and an empty roster are equivalent")
	assert.True(t, equalPersonList(
		[]Person{{Name: "A", Email: fxEmailA}},
		[]Person{{Name: "A", Email: fxEmailA}}))
	assert.False(t, equalPersonList(
		[]Person{{Name: "A"}},
		[]Person{{Name: "A"}, {Name: "B"}}))
	assert.False(t, equalPersonList(
		[]Person{{Name: "A", Email: fxEmailA}},
		[]Person{{Name: "A", Email: "b@x.test"}}))
}

func TestCopyStringMap(t *testing.T) {
	source := map[string]string{fxEngineName: fxEngineNode}
	assert.Nil(t, copyStringMap(nil), "an absent map clones to an absent one")

	cp := copyStringMap(source)
	cp[fxEngineName] = fxMutated
	assert.Equal(t, fxEngineNode, source[fxEngineName], "the source map is untouched")
	assert.Len(t, cp, 1)
}

// ── person projection ────────────────────────────────────────────────────────

func TestFilterByRole(t *testing.T) {
	people := []projectfile.Person{
		{Roles: []string{fxRoleAuthor}},
		{Roles: []string{fxRoleContrib}},
		{Roles: nil},
		{Roles: []string{fxRoleMaintain, fxRoleAuthor}},
	}
	assert.Len(t, filterByRole(people, fxRoleAuthor), 2)
	assert.Len(t, filterByRole(people, fxRoleMaintain), 1)
	assert.Empty(t, filterByRole(people, "other"))
	assert.Empty(t, filterByRole(nil, fxRoleAuthor))
}

// TestPersonDisplayName pins the three name resolutions: an explicit display
// name, the composed given+particle+family+suffix form, and the empty fallback.
func TestPersonDisplayName(t *testing.T) {
	assert.Equal(t, "Sam Rivers", personDisplayName(projectfile.Person{DisplayName: "Sam Rivers"}),
		"an explicit display name wins outright")
	assert.Equal(t, "Jean de La Fontaine Jr",
		personDisplayName(projectfile.Person{
			GivenNames:   fxGivenJean,
			NameParticle: "de",
			FamilyNames:  "La Fontaine",
			NameSuffix:   "Jr",
		}), "given + particle + family + suffix compose in order")
	assert.Equal(t, "Jean La Fontaine",
		personDisplayName(projectfile.Person{GivenNames: fxGivenJean, FamilyNames: "La Fontaine"}))
	assert.Empty(t, personDisplayName(projectfile.Person{GivenNames: fxGivenJean}),
		"a half-name composes nothing")
	assert.Empty(t, personDisplayName(projectfile.Person{}))
}

func TestNPMPersonToPF(t *testing.T) {
	twoWords := npmPersonToPF(Person{Name: fxGivenAda, Email: fxEmailAda, URL: fxURLAda}, fxRoleAuthor)
	assert.Equal(t, "Ada", twoWords.GivenNames, "a two-word name splits into given and family")
	assert.Equal(t, "Lovelace", twoWords.FamilyNames)
	assert.Equal(t, []string{fxRoleAuthor}, twoWords.Roles)
	assert.Equal(t, fxEmailAda, twoWords.Email)
	assert.Equal(t, fxURLAda, twoWords.URL)

	oneWord := npmPersonToPF(Person{Name: "Grace"}, fxRoleContrib)
	assert.Equal(t, "Grace", oneWord.DisplayName, "a one-word name is a display name")
	assert.Empty(t, oneWord.GivenNames)

	nameless := npmPersonToPF(Person{Email: "anon@x.test"}, fxRoleMaintain)
	assert.Empty(t, nameless.DisplayName, "an email-only entry carries no display name")
	assert.Equal(t, "anon@x.test", nameless.Email, "the email still crosses")
}

// TestPFPersonToNPM pins that the npm view carries only the three fields the
// package.json person shapes can express, resolved through DisplayName.
func TestPFPersonToNPM(t *testing.T) {
	got := pfPersonToNPM(projectfile.Person{
		DisplayName: fxGivenAda,
		Email:       fxEmailAda,
		URL:         fxURLAda,
		Alias:       "ada",
	})
	assert.Equal(t, Person{Name: fxGivenAda, Email: fxEmailAda, URL: fxURLAda}, got)
}

// ── funding projection ───────────────────────────────────────────────────────

func TestHasExtensionFunding(t *testing.T) {
	assert.False(t, hasExtensionFunding(nil))
	assert.False(t, hasExtensionFunding(&pfmodel.FundingExtension{}))

	full := &pfmodel.FundingExtension{
		GitHub: []string{fxNpmScope}, Patreon: fxNpmScope, KoFi: fxNpmScope, Liberapay: fxNpmScope,
		Tidelift: fxNpmScope, CommunityBridge: fxNpmScope, IssueHunt: fxNpmScope, OpenCollective: fxNpmScope,
		LFXCrowdfunding: fxNpmScope, Polar: fxNpmScope, BuyMeACoffee: fxNpmScope, ThanksDev: fxNpmScope,
		Custom: []string{fxURLX},
	}
	assert.True(t, hasExtensionFunding(full), "every provider slot counts as funding")

	for name, ext := range map[string]*pfmodel.FundingExtension{
		"github":      {GitHub: []string{fxNpmScope}},
		fxValueCustom: {Custom: []string{fxURLX}},
	} {
		assert.True(t, hasExtensionFunding(ext), name)
	}
	assert.False(t, hasExtensionFunding(&pfmodel.FundingExtension{Path: ".github/FUNDING.yml"}),
		"a bare path is metadata about where the file lives, not a funding destination")
}

func TestNPMFundingToCustom(t *testing.T) {
	assert.Empty(t, npmFundingToCustom(nil))
	assert.Equal(t, []string{fxURLA}, npmFundingToCustom([]FundingEntry{{URL: fxURLA}}))
	assert.Equal(t, []string{fxURLA, fxURLB},
		npmFundingToCustom([]FundingEntry{{URL: fxURLA}, {URL: fxURLB}}))
	assert.Empty(t, npmFundingToCustom([]FundingEntry{{Type: fxValueCustom}}), "an entry with no url is skipped")
}

func TestExtensionFundingURLs(t *testing.T) {
	assert.Empty(t, extensionFundingURLs(&pfmodel.FundingExtension{}))
	assert.Equal(t, []string{fxURLA},
		extensionFundingURLs(&pfmodel.FundingExtension{Custom: []string{fxURLA}}))
}

func TestUrlsToFundingEntries(t *testing.T) {
	assert.Nil(t, urlsToFundingEntries(nil), "no urls means no funding key at all")

	single := urlsToFundingEntries([]string{fxURLA})
	entry, ok := single.(FundingEntry)
	require.True(t, ok, "one url stays a single object")
	assert.Equal(t, fxValueCustom, entry.Type)
	assert.Equal(t, fxURLA, entry.URL)

	many := urlsToFundingEntries([]string{fxURLA, fxURLB})
	list, ok := many.([]FundingEntry)
	require.True(t, ok, "several urls become an array")
	require.Len(t, list, 2)
	assert.Equal(t, fxURLB, list[1].URL)
}

func TestFundingExtToMap(t *testing.T) {
	assert.Empty(t, fundingExtToMap(&pfmodel.FundingExtension{}),
		"an empty extension maps to nothing, so the key is dropped")

	full := &pfmodel.FundingExtension{
		GitHub:     []string{fxNpmScope},
		Custom:     []string{fxURLA},
		Path:       ".github/FUNDING.yml",
		EntityType: "Organization",
		Channels:   []pfmodel.FundingChannel{{GUID: "g", Type: "payment", Address: fxURLX, Description: "d"}},
		Plans:      []pfmodel.FundingPlan{{GUID: "p", Status: "active", Name: "n", Channels: []string{"g"}}},
		History:    []pfmodel.FundingHistory{{Year: 2025, Description: "d", Income: 1.5}},
	}
	m := fundingExtToMap(full)
	assert.Equal(t, []any{fxNpmScope}, m["github"])
	assert.Equal(t, []any{fxURLA}, m[fxValueCustom])
	assert.Equal(t, ".github/FUNDING.yml", m["path"])
	assert.Equal(t, "Organization", m["entity-type"])
	require.Len(t, m["channels"], 1)
	require.Len(t, m["plans"], 1)
	require.Len(t, m["history"], 1)
	assert.NotContains(t, m, "patreon", "an empty provider slot is omitted")

	channels := m["channels"].([]any)[0].(map[string]any)
	assert.Equal(t, "g", channels["guid"])
	assert.Equal(t, "d", channels[npmKeyDescription])
	plans := m["plans"].([]any)[0].(map[string]any)
	assert.Equal(t, "n", plans[npmKeyName])
	assert.Len(t, plans["channels"], 1)
	history := m["history"].([]any)[0].(map[string]any)
	assert.Equal(t, 2025, history["year"])
	assert.Equal(t, "d", history[npmKeyDescription])
}

// ── extension readers ────────────────────────────────────────────────────────

func TestExtensionStringListBothSpellings(t *testing.T) {
	assert.Nil(t, extensionStringList(&projectfile.Document{}, nsArchitecture),
		"an absent namespace reads as no list")

	typed := &projectfile.Document{}
	projectfile.SetExtension(typed, nsArchitecture, []string{fxAMD64})
	assert.Equal(t, []string{fxAMD64}, extensionStringList(typed, nsArchitecture),
		"a []string written by SetExtension reads back")

	parsed := &projectfile.Document{}
	projectfile.SetExtension(parsed, nsArchitecture, []any{fxAMD64, fxArm64})
	assert.Equal(t, []string{fxAMD64, fxArm64}, extensionStringList(parsed, nsArchitecture),
		"a []any parsed from disk reads back as []string")

	scalar := &projectfile.Document{}
	projectfile.SetExtension(scalar, nsArchitecture, fxAMD64)
	assert.Nil(t, extensionStringList(scalar, nsArchitecture),
		"a value that is no list at all reads as no list")
}

func TestEnsureHelpers(t *testing.T) {
	pf := &projectfile.Document{}
	ensureRequirements(pf)
	require.NotNil(t, pf.Requirements, "an absent requirements block is created")
	ensureRequirements(pf)
	assert.NotNil(t, pf.Requirements, "a present block is left alone")

	ensureLicense(pf)
	require.NotNil(t, pf.License, "an absent license is created")
	ensureLicense(pf)
	assert.NotNil(t, pf.License, "a present license is left alone")
}
