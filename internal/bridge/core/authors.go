// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// SplitAuthor splits a "Name <email>" author string into its name and email parts.
func SplitAuthor(s string) (name, email string) {
	rest := strings.TrimSpace(s)
	if open := strings.Index(rest, "<"); open >= 0 {
		if end := strings.Index(rest[open:], ">"); end >= 0 {
			email = strings.TrimSpace(rest[open+1 : open+end])
			rest = strings.TrimSpace(rest[:open])
		}
	}
	return rest, email
}

// AuthorPerson converts a split author pair into a projectfile person with the author role.
func AuthorPerson(name, email string) projectfile.Person {
	p := projectfile.Person{Email: email, Roles: []string{"author"}}
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	switch {
	case len(parts) == 2:
		p.GivenNames = parts[0]
		p.FamilyNames = parts[1]
	case len(parts) == 1 && parts[0] != "":
		p.DisplayName = parts[0]
	}
	return p
}

// FormatAuthor renders a projectfile person as a "Name <email>" string.
func FormatAuthor(p projectfile.Person) string {
	name := PersonDisplayName(p)
	switch {
	case name != "" && p.Email != "":
		return name + " <" + p.Email + ">"
	case name != "":
		return name
	default:
		return p.Email
	}
}

// PersonDisplayName resolves the flat display name following composer rules.
func PersonDisplayName(p projectfile.Person) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	if p.GivenNames != "" && p.FamilyNames != "" {
		parts := []string{p.GivenNames}
		if p.NameParticle != "" {
			parts = append(parts, p.NameParticle)
		}
		parts = append(parts, p.FamilyNames)
		if p.NameSuffix != "" {
			parts = append(parts, p.NameSuffix)
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// PeopleWithRole selects the people entries carrying the given role.
func PeopleWithRole(people []projectfile.Person, role string) []projectfile.Person {
	var out []projectfile.Person
	for _, p := range people {
		for _, r := range p.Roles {
			if r == role {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// AuthorStrings renders every author-role person as a "Name <email>" entry, skipping empty ones.
func AuthorStrings(people []projectfile.Person) []string {
	var out []string
	for _, p := range PeopleWithRole(people, "author") {
		if s := FormatAuthor(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// MergeAuthorStrings merges "Name <email>" entries into pf.People and reports the change, "" when none.
func MergeAuthorStrings(pf *projectfile.Document, authors []string) string {
	var incoming []projectfile.Person
	for _, a := range authors {
		name, email := SplitAuthor(a)
		if name == "" && email == "" {
			continue
		}
		incoming = append(incoming, AuthorPerson(name, email))
	}
	if len(incoming) == 0 {
		return ""
	}
	before := len(pf.People)
	merged, conflicts := projectfile.MergePeople(pf.People, incoming)
	pf.People = merged
	RecordPersonConflicts(pf, conflicts)
	if len(pf.People) == before && len(conflicts) == 0 {
		return ""
	}
	return fmt.Sprintf("%d author(s) merged", len(incoming))
}
