// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	testAcmeOrg   = "Acme"
	ownerTestRole = "author"
)

func TestOwnerTokenPersonPrefersAlias(t *testing.T) {
	p := projectfile.Person{
		FamilyNames: "Hopper", GivenNames: "Grace",
		Alias: "ghopper", Email: "grace@example.com",
	}
	assert.Equal(t, "@ghopper", ownerTokenPerson(p), "alias is a forge handle, preferred over email")
}

func TestOwnerTokenPersonFallsBackToEmail(t *testing.T) {
	p := projectfile.Person{
		FamilyNames: "Lovelace", GivenNames: "Ada", Email: "ada@example.com",
	}
	assert.Equal(t, "ada@example.com", ownerTokenPerson(p), "no alias → email")
}

func TestOwnerTokenPersonEmptyWhenNeither(t *testing.T) {
	p := projectfile.Person{FamilyNames: "Nobody"}
	assert.Empty(t, ownerTokenPerson(p))
}

func TestOwnerTokenOrgPrefersAlias(t *testing.T) {
	o := projectfile.Organization{Name: testAcmeOrg, Alias: "@acme-team"}
	assert.Equal(t, "@acme-team", ownerTokenOrg(o), "a leading @ is kept verbatim")
}

func TestOwnerTokenOrgFallsBackToEmail(t *testing.T) {
	o := projectfile.Organization{Name: testAcmeOrg, Email: "acme@example.com"}
	assert.Equal(t, "acme@example.com", ownerTokenOrg(o), "no alias → email")
}

func TestDefaultOwnerEntriesFromAuthorRole(t *testing.T) {
	pf := &projectfile.Document{
		People: []projectfile.Person{
			{GivenNames: "Ada", FamilyNames: "Lovelace", Alias: "ada", Roles: []string{ownerTestRole}},
			{GivenNames: "Grace", FamilyNames: "Hopper", Alias: "ghopper", Roles: []string{"reviewer"}},
		},
	}
	entries := defaultOwnerEntries(pf)
	assert.Len(t, entries, 1)
	assert.Equal(t, "*", entries[0].Pattern)
	assert.Equal(t, []string{"@ada"}, entries[0].Owners, "only author-role person included")
}

func TestDefaultOwnerEntriesIncludesMaintainer(t *testing.T) {
	pf := &projectfile.Document{
		People: []projectfile.Person{
			{Alias: "alice", Roles: []string{ownerTestRole}},
			{Alias: "bob", Roles: []string{"maintainer"}},
		},
	}
	entries := defaultOwnerEntries(pf)
	assert.Len(t, entries, 1)
	assert.Equal(t, []string{"@alice", "@bob"}, entries[0].Owners)
}

func TestDefaultOwnerEntriesIncludesOrgs(t *testing.T) {
	pf := &projectfile.Document{
		People:        []projectfile.Person{{Alias: "dev", Roles: []string{ownerTestRole}}},
		Organizations: []projectfile.Organization{{Name: testAcmeOrg, Alias: "acme-team", Roles: []string{"maintainer"}}},
	}
	entries := defaultOwnerEntries(pf)
	assert.Equal(t, []string{"@dev", "@acme-team"}, entries[0].Owners)
}

func TestDefaultOwnerEntriesFallbackNoRoles(t *testing.T) {
	pf := &projectfile.Document{
		People: []projectfile.Person{
			{Alias: "someone", Roles: []string{"reviewer"}},
		},
	}
	entries := defaultOwnerEntries(pf)
	assert.Len(t, entries, 1)
	assert.Equal(t, []string{"@someone"}, entries[0].Owners, "fallback to any person with handle")
}

func TestDefaultOwnerEntriesNilWhenEmpty(t *testing.T) {
	pf := &projectfile.Document{}
	assert.Nil(t, defaultOwnerEntries(pf))
}

func TestDefaultOwnerEntriesNilDoc(t *testing.T) {
	assert.Nil(t, defaultOwnerEntries(nil))
}

func TestDefaultOwnerEntriesDeduplicates(t *testing.T) {
	pf := &projectfile.Document{
		People: []projectfile.Person{
			{Alias: "dev", Email: "dev@example.com", Roles: []string{ownerTestRole}},
		},
	}
	entries := defaultOwnerEntries(pf)
	assert.Len(t, entries[0].Owners, 1, "same person not duplicated")
}
