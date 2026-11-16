// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/cargo"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// CargoSource scaffolds projectfile identity from a Rust Cargo.toml manifest.
type CargoSource struct{}

// Name is the display name used in init source listings.
func (CargoSource) Name() string { return cargo.Filename }

// Filename is the manifest name this source detects.
func (CargoSource) Filename() string { return cargo.Filename }

// Detect reports whether a Cargo manifest is present in the directory.
func (CargoSource) Detect(dir string) bool { return cargo.Exists(dir) }

// Extract projects the package fields, or a workspace's shared ones, onto a partial projectfile.
func (CargoSource) Extract(dir string) (*Partial, error) {
	doc, err := cargo.Read(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cargo.Filename, err)
	}
	p := &Partial{Keywords: doc.Keywords, Stack: []string{"rust"}}
	if doc.Name != "" {
		p.Name = StringPtr(doc.Name)
	}
	if doc.Version != "" {
		p.Version = StringPtr(doc.Version)
	}
	if desc := strings.TrimSpace(doc.Description); desc != "" {
		p.Summary = &LocalizedString{Bare: desc}
	}
	if doc.License != "" {
		p.License = StringPtr(doc.License)
	}
	if doc.Homepage != "" {
		p.Links = append(p.Links, projectfile.Link{Type: projectfile.LinkHomepage, URL: doc.Homepage})
	}
	if doc.Repository != "" {
		p.Repositories = []projectfile.Repository{{URL: doc.Repository, Type: repoTypeGit, Role: projectfile.RepositoryRoleOrigin}}
	}
	if doc.Documentation != "" {
		docs := projectfile.Link{Type: projectfile.LinkDocumentation, URL: doc.Documentation}
		pfmodel.SetLinkTags(&docs, []string{pfmodel.TagMainDocumentation}, true)
		p.Links = append(p.Links, docs)
	}
	for _, a := range doc.Authors {
		if pe, ok := authorEntry(a); ok {
			p.People = append(p.People, pe)
		}
	}
	return p, nil
}
