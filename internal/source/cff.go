// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/cff"
)

type CFFSource struct{}

func (CFFSource) Name() string     { return "CITATION.cff" }
func (CFFSource) Filename() string { return "CITATION.cff" }

func (CFFSource) Detect(dir string) bool {
	return cff.Exists(dir)
}

func (CFFSource) Extract(dir string) (*Partial, error) {
	doc, err := cff.Read(dir)
	if err != nil {
		return nil, fmt.Errorf("read CITATION.cff: %w", err)
	}

	p := &Partial{}

	if doc.Title != "" {
		p.Title = &LocalizedString{Bare: doc.Title}
	}
	if doc.Abstract != "" {
		p.Summary = &LocalizedString{Bare: doc.Abstract}
	}
	if doc.Version != "" {
		p.Version = StringPtr(doc.Version)
	}
	if licenseStr := cff.LicenseToString(doc.License); licenseStr != "" {
		p.License = StringPtr(licenseStr)
	}
	if doc.RepositoryCode != "" {
		// CITATION.cff's repository-code is required to be http-style, so it
		// doubles as both the canonical clone endpoint (repositories[]) and
		// the forge landing page (links[type=source-code]). Downstream
		// consumers picking the citable URL will resolve via the source-code
		// link first; the repositories[] entry lets git tooling clone from
		// the same place.
		p.Repositories = []projectfile.Repository{{
			URL:  doc.RepositoryCode,
			Type: repoTypeGit,
			Role: projectfile.RepositoryRoleOrigin,
		}}
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkSourceCode, URL: doc.RepositoryCode})
	}
	if doc.URL != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkHomepage, URL: doc.URL})
	}
	if len(doc.Keywords) > 0 {
		p.Keywords = doc.Keywords
	}

	var people []PersonEntry
	people = append(people, cffPeople(doc.Authors, "author")...)
	people = append(people, cffPeople(doc.Maintainers, "maintainer")...)
	people = append(people, cffPeople(doc.Contributors, "contributor")...)
	if len(people) > 0 {
		p.People = people
	}

	switch doc.Type {
	case "software":
		p.Kind = StringPtr("SoftwareSourceCode")
	case "dataset":
		p.Kind = StringPtr("Dataset")
	}

	return p, nil
}

func cffPeople(entities []cff.PersonEntity, role string) []PersonEntry {
	var out []PersonEntry
	for _, e := range entities {
		pe := PersonEntry{
			FamilyNames: e.FamilyNames,
			GivenNames:  e.GivenNames,
			Name:        e.Name,
			Email:       e.Email,
			Roles:       []string{role},
			Affiliation: e.Affiliation,
		}
		if e.Orcid != "" {
			pe.Orcid = strings.TrimPrefix(e.Orcid, "https://orcid.org/")
		}
		if e.Website != "" {
			pe.URL = e.Website
		}
		out = append(out, pe)
	}
	return out
}
