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

// TestForgeHandlesPreferAliasThenEmail pins the §4.5.1 alias-preferred rule and the verbatim passthrough of a written `@`.
func TestForgeHandlesPreferAliasThenEmail(t *testing.T) {
	assert.Equal(t, "@ada", ForgePersonHandle(projectfile.Person{Alias: "ada"}))
	assert.Equal(t, "@ada", ForgePersonHandle(projectfile.Person{Alias: "@ada"}), "a written @ is not doubled")
	assert.Equal(t, "@ada", ForgePersonHandle(projectfile.Person{Alias: " ada "}), "the alias is trimmed first")
	assert.Equal(t, "ada@example.test", ForgePersonHandle(projectfile.Person{Email: "ada@example.test"}))
	assert.Equal(t, "ada@example.test", ForgePersonHandle(projectfile.Person{Alias: "  ", Email: "ada@example.test"}),
		"a blank alias is no alias")
	assert.Empty(t, ForgePersonHandle(projectfile.Person{}))

	assert.Equal(t, "@acme", ForgeOrgHandle(projectfile.Organization{Alias: aliasAcme}))
	assert.Equal(t, "@acme", ForgeOrgHandle(projectfile.Organization{Alias: "@acme"}))
	assert.Equal(t, "acme@example.test", ForgeOrgHandle(projectfile.Organization{Email: "acme@example.test"}))
	assert.Empty(t, ForgeOrgHandle(projectfile.Organization{}))
}

// TestForgeHandleUsesTheCommonSurface pins the resolver CODEOWNERS shares between persons and organisations.
func TestForgeHandleUsesTheCommonSurface(t *testing.T) {
	assert.Equal(t, "@acme", ForgeHandle(PersonOrEntityCompat{Alias: aliasAcme}))
	assert.Equal(t, "@acme", ForgeHandle(PersonOrEntityCompat{Alias: "@acme"}))
	assert.Equal(t, "one@example.test", ForgeHandle(PersonOrEntityCompat{Email: "one@example.test"}))
	assert.Empty(t, ForgeHandle(PersonOrEntityCompat{}))
}

// TestFlatPersonNameAppliesTheSynthesisRule pins spec §5.5.3 in one table, including the suffix that only joins with a comma when a name precedes it.
func TestFlatPersonNameAppliesTheSynthesisRule(t *testing.T) {
	cases := map[string]struct {
		in   projectfile.Person
		want string
	}{
		"display name wins": {projectfile.Person{DisplayName: nameAdaLovelace, GivenNames: "Augusta", FamilyNames: "Byron"}, nameAdaLovelace},
		"all parts":         {projectfile.Person{GivenNames: "Augusta", NameParticle: nameAda, FamilyNames: "Byron"}, "Augusta Ada Byron"},
		"given and family":  {projectfile.Person{GivenNames: givenGrace, FamilyNames: familyHopper}, "Grace Hopper"},
		"family only":       {projectfile.Person{FamilyNames: familyHopper}, familyHopper},
		"suffix joins":      {projectfile.Person{GivenNames: givenGrace, FamilyNames: familyHopper, NameSuffix: suffixPhD}, "Grace Hopper, PhD"},
		"suffix alone":      {projectfile.Person{NameSuffix: suffixPhD}, suffixPhD},
		"empty":             {projectfile.Person{}, ""},
	}
	for name, c := range cases {
		assert.Equal(t, c.want, FlatPersonName(c.in), name)
	}
	assert.Equal(t, orgAcme, FlatOrgName(projectfile.Organization{Name: orgAcme, Alias: aliasAcme}))
	assert.Empty(t, FlatOrgName(projectfile.Organization{}))
}

// TestDisplayNameFallsDownTheLadder pins the label every generator uses, so a titled project never renders as its DNS-label name.
func TestDisplayNameFallsDownTheLadder(t *testing.T) {
	titled := &projectfile.Document{Identity: projectfile.Identity{
		Title: &projectfile.LocalizedString{Bare: "Projectfile Bridges"},
	}}
	assert.Equal(t, "Projectfile Bridges", DisplayName(titled))
	assert.Equal(t, "Projectfile Bridges", DisplayNameForLang(titled, "es"), "a bare title is language-agnostic")

	nsName := &projectfile.Document{Identity: projectfile.Identity{Namespace: "org.b19", Name: "ubuntu"}}
	assert.Equal(t, "org.b19/ubuntu", DisplayName(nsName))
	assert.Equal(t, "org.b19/ubuntu", DisplayNameForLang(nsName, "uk"), "the ladder has no per-language step")

	nameOnly := &projectfile.Document{Identity: projectfile.Identity{Name: "ubuntu"}}
	assert.Equal(t, "ubuntu", DisplayName(nameOnly))

	assert.Equal(t, "this project", DisplayName(&projectfile.Document{}))
	assert.Equal(t, "this project", DisplayNameForLang(nil, "en"))
}

// TestYearOfReadsTheLeadingFourDigits pins the defensive reader and every refusal: a short string, a non-digit prefix and a partial date.
func TestYearOfReadsTheLeadingFourDigits(t *testing.T) {
	assert.Equal(t, 2026, yearOf("2026-01-02"))
	assert.Equal(t, 2026, yearOf("2026"))
	assert.Zero(t, yearOf(""))
	assert.Zero(t, yearOf("20"), "fewer than four digits is malformed")
	assert.Zero(t, yearOf("20x6-01-02"), "a non-digit year is malformed")
	assert.Zero(t, yearOf("abcd-01-02"))
	assert.Zero(t, yearOf("-26-01-02"))
}

// TestCopyrightLineAssemblesTheSPDXLine pins the year prefix, the range and the email suffix — the shape every generated header carries.
func TestCopyrightLineAssemblesTheSPDXLine(t *testing.T) {
	assert.Equal(t, "Copyright 2026 Ada", copyrightLine("", "", 2026, nameAda, ""))
	assert.Equal(t, "Copyright 2024-2026 Ada", copyrightLine("2024-01-01", "2026-06-01", 2026, nameAda, ""))
	assert.Equal(t, "Copyright 2024 Ada", copyrightLine("2024-01-01", "2024-06-01", 2026, nameAda, ""),
		"a one-year range collapses to a single year")
	assert.Equal(t, "Copyright 2026 Ada <ada@example.test>", copyrightLine("", "", 2026, nameAda, "ada@example.test"))
	assert.Empty(t, copyrightLine("", "", 2026, "", ""), "a nameless holder produces no line")
}

// TestCopyrightHoldersAndNamesReadTheCopyrightRole pins the role filter, the per-person year override and the document-wide default.
func TestCopyrightHoldersAndNamesReadTheCopyrightRole(t *testing.T) {
	doc := &projectfile.Document{
		Copyright: &projectfile.Copyright{Year: 2020},
		People: []projectfile.Person{
			{GivenNames: givenGrace, FamilyNames: familyHopper, Email: "grace@example.test", Roles: []string{RoleCopyright}},
			{GivenNames: nameAda, FamilyNames: "Lovelace", From: "2024-01-01", Roles: []string{RoleCopyright}},
			{GivenNames: "Linus", FamilyNames: "Torvalds", Roles: []string{RoleMaintainer}},
		},
		Organizations: []projectfile.Organization{
			{Name: orgAcme, Roles: []string{RoleCopyright}},
			{Name: "Other", Roles: []string{"sponsor"}},
		},
	}
	assert.Equal(t, []string{
		"Copyright 2020 Grace Hopper <grace@example.test>",
		"Copyright 2024 Ada Lovelace",
		"Copyright 2020 Acme",
	}, CopyrightHolders(doc))
	assert.Equal(t, []string{"Grace Hopper", nameAdaLovelace, orgAcme}, CopyrightHolderNames(doc))
}

// TestCopyrightHoldersFallsBackToTheCurrentYear pins the last rung: no per-person year and no document default.
func TestCopyrightHoldersFallsBackToTheCurrentYear(t *testing.T) {
	doc := &projectfile.Document{People: []projectfile.Person{
		{FamilyNames: familyHopper, Roles: []string{RoleCopyright}},
	}}
	holders := CopyrightHolders(doc)
	require.Len(t, holders, 1)
	assert.Contains(t, holders[0], familyHopper)
}

// TestCopyrightReadersOnNilAndEmptyDocuments.
func TestCopyrightReadersOnNilAndEmptyDocuments(t *testing.T) {
	assert.Nil(t, CopyrightHolders(nil))
	assert.Nil(t, CopyrightHolderNames(nil))
	assert.Nil(t, CopyrightHolders(&projectfile.Document{}))
	assert.Nil(t, CopyrightHolderNames(&projectfile.Document{}))
}

// TestRoleContains pins the flat role filter.
func TestRoleContains(t *testing.T) {
	assert.True(t, roleContains([]string{"maintainer", RoleCopyright}, RoleCopyright))
	assert.False(t, roleContains([]string{"maintainer"}, RoleCopyright))
	assert.False(t, roleContains(nil, RoleCopyright))
}

// TestContactEmailWalksTheLadder pins the SHOULD-filter resolution order: the artefact-specific role, the generic maintainer, then any address at all.
func TestContactEmailWalksTheLadder(t *testing.T) {
	doc := &projectfile.Document{
		People: []projectfile.Person{
			{Email: "anyone@example.test"},
			{Email: "maint@example.test", Roles: []string{RoleMaintainer}},
			{Email: "sec@example.test", Roles: []string{roleSecurity}},
		},
		Organizations: []projectfile.Organization{{Email: emailOrg, Roles: []string{RoleMaintainer}}},
	}
	email, source := ContactEmail(doc, roleSecurity)
	assert.Equal(t, "sec@example.test", email)
	assert.Contains(t, source, roleSecurity)

	email, source = ContactEmail(doc, "")
	assert.Equal(t, "maint@example.test", email, "a blank primary role skips step one")
	assert.Contains(t, source, "maintainer")

	email, _ = ContactEmail(doc, RoleMaintainer)
	assert.Equal(t, "maint@example.test", email)

	email, source = ContactEmail(&projectfile.Document{People: []projectfile.Person{
		{Email: "anyone@example.test"},
	}}, roleSecurity)
	assert.Equal(t, "anyone@example.test", email, "the last rung keeps a project usable")
	assert.Contains(t, source, "first [[people]] entry")
}

// TestContactEmailFindsTheOrganisationFallback pins the organisation half of both ladders.
func TestContactEmailFindsTheOrganisationFallback(t *testing.T) {
	orgs := &projectfile.Document{Organizations: []projectfile.Organization{
		{Email: emailOrg, Roles: []string{RoleMaintainer}},
	}}
	email, source := ContactEmail(orgs, "")
	assert.Equal(t, emailOrg, email)
	assert.Contains(t, source, "maintainer")

	plain := &projectfile.Document{Organizations: []projectfile.Organization{{Email: emailOrg}}}
	email, source = ContactEmail(plain, "")
	assert.Equal(t, emailOrg, email)
	assert.Contains(t, source, "[[organizations]] entry")
}

// TestContactEmailWithNoAddressIsAnActionableEmpty pins the signal a caller turns into "no contact available" rather than a broken file.
func TestContactEmailWithNoAddressIsAnActionableEmpty(t *testing.T) {
	email, source := ContactEmail(&projectfile.Document{People: []projectfile.Person{
		{FamilyNames: familyHopper, Roles: []string{RoleCopyright}},
	}}, roleSecurity)
	assert.Empty(t, email)
	assert.Empty(t, source)

	email, source = ContactEmail(nil, "")
	assert.Empty(t, email)
	assert.Empty(t, source)
}

// TestFirstEmailByRoleSkipsAPersonWithNoAddress pins the guard: a person in the right role with no email is not a contact.
func TestFirstEmailByRoleSkipsAPersonWithNoAddress(t *testing.T) {
	doc := &projectfile.Document{
		People: []projectfile.Person{
			{FamilyNames: "NoEmail", Roles: []string{roleSecurity}},
			{Email: "sec@example.test", Roles: []string{roleSecurity}},
		},
	}
	assert.Equal(t, "sec@example.test", firstEmailByRole(doc, roleSecurity))
	assert.Empty(t, firstEmailByRole(doc, "maintainer"))
}
