// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/npm"
)

type NPMSource struct{}

func (NPMSource) Name() string     { return "package.json" }
func (NPMSource) Filename() string { return "package.json" }

func (NPMSource) Detect(dir string) bool {
	return npm.Exists(dir)
}

func (NPMSource) Extract(dir string) (*Partial, error) {
	doc, err := npm.Read(dir)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	p := &Partial{}

	if doc.Name != "" {
		ns, name := npmNameToPFName(doc.Name)
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

	if doc.License != "" {
		p.License = StringPtr(doc.License)
	}

	repo := npm.ParseRepository(doc.Repository)
	if repo.URL != "" {
		r := projectfile.Repository{URL: repo.URL, Type: repo.Type, Role: projectfile.RepositoryRoleOrigin}
		if repo.Directory != "" {
			r.Path = repo.Directory
		}
		p.Repositories = []projectfile.Repository{r}
	}

	bugs := npm.ParseBugs(doc.Bugs)
	if bugs.URL != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkBugs, URL: bugs.URL})
	}
	if doc.Homepage != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkHomepage, URL: doc.Homepage})
	}

	if len(doc.Keywords) > 0 {
		p.Keywords = doc.Keywords
	}

	var people []PersonEntry
	author := npm.ParsePerson(doc.Author)
	if author.Name != "" {
		people = append(people, npmPersonEntry(author, "author"))
	}
	if len(doc.Contributors) > 0 {
		for _, c := range npm.ParsePersonList(doc.Contributors) {
			people = append(people, npmPersonEntry(c, "contributor"))
		}
	}
	if len(doc.Maintainers) > 0 {
		for _, m := range npm.ParsePersonList(doc.Maintainers) {
			people = append(people, npmPersonEntry(m, "maintainer"))
		}
	}
	if len(people) > 0 {
		p.People = people
	}

	if len(doc.Engines) > 0 {
		p.Stack = extractStackFromEngines(doc.Engines)
	}

	return p, nil
}

func npmNameToPFName(npmName string) (namespace, name string) {
	if strings.HasPrefix(npmName, "@") {
		parts := strings.SplitN(strings.TrimPrefix(npmName, "@"), "/", 2)
		if len(parts) == 2 {
			return "npm." + parts[0], parts[1]
		}
		return "", strings.TrimPrefix(npmName, "@")
	}
	return "", npmName
}

func npmPersonEntry(n npm.Person, role string) PersonEntry {
	parts := strings.SplitN(n.Name, " ", 2)
	pe := PersonEntry{
		Email: n.Email,
		URL:   n.URL,
		Roles: []string{role},
	}
	if len(parts) == 2 {
		pe.GivenNames = parts[0]
		pe.FamilyNames = parts[1]
	} else if len(parts) == 1 && parts[0] != "" {
		pe.Name = parts[0]
	}
	return pe
}

func extractStackFromEngines(engines map[string]string) []string {
	var stack []string
	for k := range engines {
		switch k {
		case "node":
			stack = append(stack, "node")
		case "npm":
			stack = append(stack, "npm")
		case "yarn":
			stack = append(stack, "yarn")
		case "pnpm":
			stack = append(stack, "pnpm")
		}
	}
	return stack
}
