// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/shard"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// ShardSource scaffolds projectfile identity from a Crystal shard manifest.
type ShardSource struct{}

// Name is the display name used in init source listings.
func (ShardSource) Name() string { return "shard.yml" }

// Filename is the canonical manifest name this source detects.
func (ShardSource) Filename() string { return "shard.yml" }

// Detect reports whether a shard manifest is present in the directory.
func (ShardSource) Detect(dir string) bool {
	return shard.Exists(dir)
}

// Extract projects the shard manifest fields onto a partial projectfile.
func (ShardSource) Extract(dir string) (*Partial, error) {
	doc, err := shard.Read(dir)
	if err != nil {
		return nil, fmt.Errorf("read shard.yml: %w", err)
	}
	p := &Partial{}
	if doc.Name != "" {
		p.Name = StringPtr(doc.Name)
	}
	if doc.Version != "" {
		p.Version = StringPtr(doc.Version)
	}
	if desc := strings.TrimSpace(doc.Description); desc != "" {
		p.Summary = &LocalizedString{Bare: desc}
	}
	if doc.License != "" && !isLicenseURL(doc.License) {
		p.License = StringPtr(doc.License)
	}
	if doc.Homepage != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkHomepage, URL: doc.Homepage})
	}
	if doc.Repository != "" {
		p.Repositories = []projectfile.Repository{{
			URL:  doc.Repository,
			Type: repoTypeGit,
			Role: projectfile.RepositoryRoleOrigin,
		}}
	}
	if doc.Documentation != "" {
		docs := projectfile.Link{Type: projectfile.LinkDocumentation, URL: doc.Documentation}
		pfmodel.SetLinkTags(&docs, []string{pfmodel.TagMainDocumentation}, true)
		p.Links = append(p.Links, docs)
	}
	var people []PersonEntry
	for _, a := range doc.Authors {
		if pe, ok := shardAuthorEntry(a); ok {
			people = append(people, pe)
		}
	}
	if len(people) > 0 {
		p.People = people
	}
	p.Stack = []string{"crystal"}
	return p, nil
}

// shardAuthorEntry splits a "Name <email>" author string into a person entry.
func shardAuthorEntry(s string) (PersonEntry, bool) {
	rest := strings.TrimSpace(s)
	email := ""
	if open := strings.Index(rest, "<"); open >= 0 {
		if end := strings.Index(rest[open:], ">"); end >= 0 {
			email = strings.TrimSpace(rest[open+1 : open+end])
			rest = strings.TrimSpace(rest[:open])
		}
	}
	pe := PersonEntry{Email: email, Roles: []string{"author"}}
	parts := strings.SplitN(rest, " ", 2)
	switch {
	case len(parts) == 2:
		pe.GivenNames = parts[0]
		pe.FamilyNames = parts[1]
	case len(parts) == 1 && parts[0] != "":
		pe.Name = parts[0]
	}
	if pe.Name == "" && pe.GivenNames == "" && pe.Email == "" {
		return pe, false
	}
	return pe, true
}

// isLicenseURL reports whether the license value is a file URL, not an SPDX id.
func isLicenseURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")
}
