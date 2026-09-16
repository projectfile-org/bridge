// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// crystalRequirementKey is the requirements.runtime entry for the constraint.
const crystalRequirementKey = "crystal"

// buildMappers returns one FieldMapper per synced shard.yml field, force-aware.
func buildMappers(doc *Document, pf *projectfile.Document) core.MapperList {
	if doc == nil {
		doc = &Document{}
	}
	doc.ReuseHeader = core.REUSEHeader(pf, core.StyleHash)
	return core.MapperList{
		mapName(doc, pf),
		mapVersion(doc, pf),
		mapDescription(doc, pf),
		mapAuthors(doc, pf),
		mapCrystal(doc, pf),
		mapLicense(doc, pf),
		mapHomepage(doc, pf),
		mapRepository(doc, pf),
		mapDocumentation(doc, pf),
	}
}

// mapName syncs the flat shard name onto identity.name with no namespace split.
func mapName(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "name", PFKey: "identity.name",
		ToPF: func(force bool) string {
			if doc.Name == "" || pf.Identity.Name == doc.Name {
				return ""
			}
			if pf.Identity.Name != "" && !force {
				return ""
			}
			pf.Identity.Name = doc.Name
			return doc.Name
		},
		FromPF: func(force bool) string {
			if pf.Identity.Name == "" || doc.Name == pf.Identity.Name {
				return ""
			}
			if doc.Name != "" && !force {
				return ""
			}
			doc.Name = pf.Identity.Name
			return pf.Identity.Name
		},
	}
}

// mapVersion syncs the shard version onto identity.version verbatim.
func mapVersion(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "version", PFKey: "identity.version",
		ToPF: func(force bool) string {
			if doc.Version == "" || pf.Identity.Version == doc.Version {
				return ""
			}
			if pf.Identity.Version != "" && !force {
				return ""
			}
			pf.Identity.Version = doc.Version
			return doc.Version
		},
		FromPF: func(force bool) string {
			if pf.Identity.Version == "" || doc.Version == pf.Identity.Version {
				return ""
			}
			if doc.Version != "" && !force {
				return ""
			}
			doc.Version = pf.Identity.Version
			return pf.Identity.Version
		},
	}
}

// mapDescription syncs the one-line shard description onto identity.summary.
func mapDescription(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "description", PFKey: "identity.summary",
		ToPF: func(force bool) string {
			desc := strings.TrimSpace(doc.Description)
			if desc == "" {
				return ""
			}
			existing := projectfile.ExtractLocalizedString(pf.Identity.Summary)
			if existing == desc {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			projectfile.SetLocalizedEN(&pf.Identity.Summary, desc)
			return core.Trunc(desc)
		},
		FromPF: func(force bool) string {
			s := projectfile.ExtractLocalizedString(pf.Identity.Summary)
			if s == "" || strings.TrimSpace(doc.Description) == s {
				return ""
			}
			if doc.Description != "" && !force {
				return ""
			}
			doc.Description = s
			return core.Trunc(s)
		},
	}
}

// mapAuthors merges the "Name <email>" author strings into people as authors.
func mapAuthors(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "authors", PFKey: "people",
		ToPF: func(_ bool) string {
			var incoming []projectfile.Person
			for _, a := range doc.Authors {
				name, email := splitShardAuthor(a)
				if name == "" && email == "" {
					continue
				}
				incoming = append(incoming, shardPersonToPF(name, email))
			}
			if len(incoming) == 0 {
				return ""
			}
			before := len(pf.People)
			merged, conflicts := projectfile.MergePeople(pf.People, incoming)
			pf.People = merged
			core.RecordPersonConflicts(pf, conflicts)
			if len(pf.People) == before && len(conflicts) == 0 {
				return ""
			}
			return fmt.Sprintf("%d author(s) merged", len(incoming))
		},
		FromPF: func(_ bool) string {
			authors := filterByRole(pf.People, "author")
			if len(authors) == 0 {
				return ""
			}
			entries := make([]string, 0, len(authors))
			for _, p := range authors {
				if s := formatShardAuthor(p); s != "" {
					entries = append(entries, s)
				}
			}
			if len(entries) == 0 {
				return ""
			}
			if equalStringSlice(doc.Authors, entries) {
				return ""
			}
			doc.Authors = entries
			return fmt.Sprintf("%d author(s)", len(entries))
		},
	}
}

// mapCrystal syncs the crystal version constraint onto requirements.runtime.
func mapCrystal(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "crystal", PFKey: "requirements.runtime[crystal]",
		ToPF: func(force bool) string {
			if doc.Crystal == "" {
				return ""
			}
			ensureRequirements(pf)
			if pf.Requirements.Runtime == nil {
				pf.Requirements.Runtime = map[string]string{}
			}
			cur := pf.Requirements.Runtime[crystalRequirementKey]
			if cur == doc.Crystal {
				return ""
			}
			if cur != "" && !force {
				return ""
			}
			pf.Requirements.Runtime[crystalRequirementKey] = doc.Crystal
			return doc.Crystal
		},
		FromPF: func(force bool) string {
			if pf.Requirements == nil {
				return ""
			}
			want := pf.Requirements.Runtime[crystalRequirementKey]
			if want == "" || doc.Crystal == want {
				return ""
			}
			if doc.Crystal != "" && !force {
				return ""
			}
			doc.Crystal = want
			return want
		},
	}
}

// mapLicense syncs an SPDX license expression, leaving license-file URLs alone.
func mapLicense(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "license", PFKey: "license.spdx",
		ToPF: func(force bool) string {
			if doc.License == "" || isURL(doc.License) {
				return ""
			}
			existing := ""
			if pf.License != nil {
				existing = pf.License.Spdx
			}
			if existing == doc.License {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			ensureLicense(pf)
			pf.License.Spdx = doc.License
			return doc.License
		},
		FromPF: func(force bool) string {
			if pf.License == nil || pf.License.Spdx == "" {
				return ""
			}
			if doc.License == pf.License.Spdx {
				return ""
			}
			if doc.License != "" && !force {
				return ""
			}
			doc.License = pf.License.Spdx
			return pf.License.Spdx
		},
	}
}

// mapHomepage syncs the homepage URL onto links[type=homepage].
func mapHomepage(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "homepage", PFKey: "links[type=homepage]",
		ToPF: func(force bool) string {
			if doc.Homepage == "" {
				return ""
			}
			existing := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
			if existing == doc.Homepage {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			pfmodel.SetLink(pf, projectfile.LinkHomepage, doc.Homepage, true)
			return core.Trunc(doc.Homepage)
		},
		FromPF: func(force bool) string {
			url := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
			if url == "" || doc.Homepage == url {
				return ""
			}
			if doc.Homepage != "" && !force {
				return ""
			}
			doc.Homepage = url
			return core.Trunc(url)
		},
	}
}

// mapRepository syncs the canonical repository URL onto the origin repository.
func mapRepository(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "repository", PFKey: "repositories[role=origin]",
		ToPF: func(force bool) string {
			if doc.Repository == "" {
				return ""
			}
			primary := pfmodel.PrimaryRepository(pf)
			if primary != nil && primary.URL == doc.Repository {
				return ""
			}
			if primary != nil && primary.URL != "" && !force {
				return ""
			}
			slot := pfmodel.EnsurePrimaryRepository(pf)
			slot.URL = doc.Repository
			if slot.Type == "" {
				slot.Type = "git"
			}
			return core.Trunc(doc.Repository)
		},
		FromPF: func(force bool) string {
			primary := pfmodel.PrimaryRepository(pf)
			if primary == nil || primary.URL == "" {
				return ""
			}
			if doc.Repository == primary.URL {
				return ""
			}
			if doc.Repository != "" && !force {
				return ""
			}
			doc.Repository = primary.URL
			return core.Trunc(primary.URL)
		},
	}
}

// mapDocumentation syncs the documentation URL onto the main-documentation link.
func mapDocumentation(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "documentation", PFKey: "links[type=documentation,tag=main-documentation]",
		ToPF: func(force bool) string {
			if doc.Documentation == "" {
				return ""
			}
			existing := pfmodel.MainDocumentationURL(pf)
			if existing == doc.Documentation {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			pfmodel.SetMainDocumentationURL(pf, doc.Documentation, true)
			return core.Trunc(doc.Documentation)
		},
		FromPF: func(force bool) string {
			url := pfmodel.MainDocumentationURL(pf)
			if url == "" || doc.Documentation == url {
				return ""
			}
			if doc.Documentation != "" && !force {
				return ""
			}
			doc.Documentation = url
			return core.Trunc(url)
		},
	}
}

// splitShardAuthor splits "Name <email>" into its name and email parts.
func splitShardAuthor(s string) (name, email string) {
	rest := strings.TrimSpace(s)
	if open := strings.Index(rest, "<"); open >= 0 {
		if end := strings.Index(rest[open:], ">"); end >= 0 {
			email = strings.TrimSpace(rest[open+1 : open+end])
			rest = strings.TrimSpace(rest[:open])
		}
	}
	return rest, email
}

// shardPersonToPF converts a shard author pair into a projectfile person.
func shardPersonToPF(name, email string) projectfile.Person {
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

// formatShardAuthor renders a projectfile person as a "Name <email>" string.
func formatShardAuthor(p projectfile.Person) string {
	name := personDisplayName(p)
	switch {
	case name != "" && p.Email != "":
		return name + " <" + p.Email + ">"
	case name != "":
		return name
	default:
		return p.Email
	}
}

// personDisplayName resolves the flat display name following composer rules.
func personDisplayName(p projectfile.Person) string {
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

// filterByRole selects the people entries carrying the given role.
func filterByRole(people []projectfile.Person, role string) []projectfile.Person {
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

// equalStringSlice compares two string slices treating nil and empty as equal.
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ensureLicense allocates the license block on first write, matching drivers.
func ensureLicense(pf *projectfile.Document) {
	if pf.License == nil {
		pf.License = &projectfile.License{}
	}
}

// ensureRequirements allocates the requirements block on first crystal write.
func ensureRequirements(pf *projectfile.Document) {
	if pf.Requirements == nil {
		pf.Requirements = &projectfile.Requirements{}
	}
}

// isURL reports whether the license value is a license-file URL, not an SPDX id.
func isURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")
}
