// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// A role-carrying organisation outranks any person as the funding entity.
func TestResolveEntityPrefersOrganisation(t *testing.T) {
	pf := &projectfile.Document{
		Identity:      projectfile.Identity{Name: fxName},
		Organizations: []projectfile.Organization{{Name: fxOrgName, Email: fxOrgEmail, Roles: []string{fxMaintainer}}},
		People:        []projectfile.Person{{GivenNames: fxGiven, Email: fxEmail, Roles: []string{fxMaintainer}}},
		Links:         []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, "organisation", entity.Type)
	assert.Equal(t, fxOrgName, entity.Name)
	assert.Equal(t, fxOrgEmail, entity.Email)
}

// An organisation without a matching role is skipped, role first, address second.
func TestResolveEntitySkipsOrganisationWithoutRole(t *testing.T) {
	pf := &projectfile.Document{
		Identity:      projectfile.Identity{Name: fxName},
		Organizations: []projectfile.Organization{{Name: fxOrgName, Email: fxOrgEmail, Roles: []string{fxContrib}}},
		People:        []projectfile.Person{{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail, Roles: []string{fxMaintainer}}},
		Links:         []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, "individual", entity.Type)
	assert.Equal(t, "Ada "+fxFamily, entity.Name)
}

// An organisation with the role but no email cannot be contacted, so the scan continues.
func TestResolveEntitySkipsOrganisationWithoutEmail(t *testing.T) {
	pf := &projectfile.Document{
		Identity:      projectfile.Identity{Name: fxName},
		Organizations: []projectfile.Organization{{Name: fxOrgName, Roles: []string{fxMaintainer}}},
		People:        []projectfile.Person{{GivenNames: fxGiven, Email: fxEmail, Roles: []string{fxMaintainer}}},
		Links:         []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, "individual", entity.Type)
	assert.Equal(t, fxEmail, entity.Email)
}

// A person with a matching role wins over a person who merely has an address.
func TestResolveEntityPrefersPersonWithRole(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People: []projectfile.Person{
			{GivenNames: "Bystander", Email: "bystander@example.org"},
			{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail, Roles: []string{fxSteward}},
		},
		Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, fxEmail, entity.Email)
	assert.Equal(t, fxSteward, entity.Role)
}

// With no role anywhere the first address is the entity and the role is "other".
func TestResolveEntityFallsBackToAnyPerson(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People: []projectfile.Person{
			{GivenNames: "No", FamilyNames: "Address"},
			{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail},
		},
		Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, fxEmail, entity.Email)
	assert.Equal(t, "other", entity.Role)
}

// The description rides identity.summary when it is set.
func TestResolveEntityDescriptionFromSummary(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: fxName, Summary: &projectfile.LocalizedString{Bare: fxSummary}},
		People:   []projectfile.Person{{Email: fxEmail}},
		Links:    []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, fxSummary, entity.Description)
}

// The description falls back to identity.description when no summary exists.
func TestResolveEntityDescriptionFromDescription(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: fxName, Description: &projectfile.LocalizedString{Bare: fxAltDesc}},
		People:   []projectfile.Person{{Email: fxEmail}},
		Links:    []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
	entity, err := resolveEntity(pf, nil)
	require.NoError(t, err)
	assert.Equal(t, fxAltDesc, entity.Description)
}

// No addressable person or organisation is a stated failure, not an empty entity.
func TestResolveEntityWithoutAnyEmail(t *testing.T) {
	_, err := resolveEntity(&projectfile.Document{
		Identity:      projectfile.Identity{Name: fxName},
		Organizations: []projectfile.Organization{{Name: fxOrgName}},
		People:        []projectfile.Person{{GivenNames: fxGiven}},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resolve entity")
}

// The webpage is required, so its absence names the links entry to add.
func TestResolveEntityWithoutHomepage(t *testing.T) {
	_, err := resolveEntity(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People:   []projectfile.Person{{Email: fxEmail}},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity.webpageUrl is required")
}

// A declared entity-type/entity-role pair overrides whatever was resolved.
func TestResolveEntityExtensionOverrides(t *testing.T) {
	ext := &pfmodel.FundingExtension{EntityType: "group", EntityRole: fxOwner}
	entity, err := resolveEntity(barePersonDoc(), ext)
	require.NoError(t, err)
	assert.Equal(t, "group", entity.Type)
	assert.Equal(t, fxOwner, entity.Role)
}

func TestRoleMatches(t *testing.T) {
	for _, role := range []string{fxOwner, fxMaintainer, fxSteward} {
		assert.True(t, roleMatches([]string{role}), role+" must match")
		assert.True(t, roleMatches([]string{fxContrib, role}), role+" must match in a list")
	}
	assert.False(t, roleMatches([]string{fxContrib}), "contributor is not a funding role")
	assert.False(t, roleMatches(nil), "no roles match")
}

// bestRole takes the highest priority across organisations AND people alike.
func TestBestRolePriorityAcrossSources(t *testing.T) {
	pf := &projectfile.Document{
		Organizations: []projectfile.Organization{{Roles: []string{fxSteward}}},
		People: []projectfile.Person{
			{Roles: []string{fxMaintainer}},
			{Roles: []string{fxOwner}},
			{Roles: []string{fxContrib}},
		},
	}
	assert.Equal(t, fxOwner, bestRole(pf, ""))
	assert.Equal(t, fxSteward, bestRole(&projectfile.Document{Organizations: []projectfile.Organization{{Roles: []string{fxSteward}}}}, ""))
	assert.Equal(t, fxContrib, bestRole(&projectfile.Document{People: []projectfile.Person{{Roles: []string{fxContrib}}}}, ""), "contributor is the lowest listed priority")
	assert.Equal(t, "other", bestRole(&projectfile.Document{People: []projectfile.Person{{Roles: []string{"security"}}}}, ""))
	assert.Equal(t, "other", bestRole(&projectfile.Document{}, ""))
}

func TestEntityDescription(t *testing.T) {
	assert.Empty(t, entityDescription(nil), "a nil document has no description")
	assert.Equal(t, fxSummary, entityDescription(&projectfile.Document{Identity: projectfile.Identity{Summary: &projectfile.LocalizedString{Bare: fxSummary}}}))
	assert.Equal(t, fxAltDesc, entityDescription(&projectfile.Document{Identity: projectfile.Identity{Description: &projectfile.LocalizedString{Bare: fxAltDesc}}}))
	assert.Equal(t, fxSummary, entityDescription(&projectfile.Document{Identity: projectfile.Identity{
		Summary:     &projectfile.LocalizedString{Bare: fxSummary},
		Description: &projectfile.LocalizedString{Bare: fxAltDesc},
	}}), "summary wins over description")
	assert.Empty(t, entityDescription(&projectfile.Document{}))
}
