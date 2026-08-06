// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/composer"
)

type ComposerSource struct{}

func (ComposerSource) Name() string     { return "composer.json" }
func (ComposerSource) Filename() string { return "composer.json" }

func (ComposerSource) Detect(dir string) bool {
	return composer.Exists(dir)
}

func (ComposerSource) Extract(dir string) (*Partial, error) {
	doc, err := composer.Read(dir)
	if err != nil {
		return nil, fmt.Errorf("read composer.json: %w", err)
	}

	p := &Partial{}

	if doc.Name != "" {
		ns, name := composerNameToPFName(doc.Name)
		if ns != "" {
			p.Namespace = StringPtr(ns)
		}
		if name != "" {
			p.Name = StringPtr(name)
		}
	}

	if doc.Version != "" {
		p.Version = StringPtr(doc.Version)
	}

	if doc.Description != "" {
		p.Summary = &LocalizedString{Bare: doc.Description}
	}

	if license := composer.LicenseString(doc.License); license != "" {
		p.License = StringPtr(license)
	}

	if doc.Homepage != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkHomepage, URL: doc.Homepage})
	}
	if doc.Support != nil {
		if doc.Support.Source != "" {
			p.Repositories = []projectfile.Repository{{
				URL:  doc.Support.Source,
				Type: "git",
				Role: projectfile.RepositoryRoleOrigin,
			}}
		}
		if doc.Support.Issues != "" {
			p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkBugs, URL: doc.Support.Issues})
		}
		if doc.Support.Docs != "" {
			p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkDocumentation, URL: doc.Support.Docs})
		}
	}

	if len(doc.Keywords) > 0 {
		p.Keywords = doc.Keywords
	}

	var people []PersonEntry
	for _, a := range doc.Authors {
		pe := composerPersonEntry(a)
		if pe.Name != "" || pe.GivenNames != "" || pe.Email != "" {
			people = append(people, pe)
		}
	}
	if len(people) > 0 {
		p.People = people
	}

	if php, ok := doc.Require["php"]; ok && php != "" {
		p.Stack = append(p.Stack, "php")
	}

	return p, nil
}

// composerNameToPFName converts "vendor/package" into the pf namespace+name
// pair. composer names are always slash-separated (Packagist requires it),
// so a missing slash is treated as a bare name with no namespace.
func composerNameToPFName(name string) (namespace, pkg string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return "composer." + parts[0], parts[1]
	}
	return "", name
}

func composerPersonEntry(c composer.Person) PersonEntry {
	pe := PersonEntry{
		Email: c.Email,
		URL:   c.Homepage,
		Roles: []string{"author"},
	}
	parts := strings.SplitN(strings.TrimSpace(c.Name), " ", 2)
	if len(parts) == 2 {
		pe.GivenNames = parts[0]
		pe.FamilyNames = parts[1]
	} else if len(parts) == 1 && parts[0] != "" {
		pe.Name = parts[0]
	}
	return pe
}
