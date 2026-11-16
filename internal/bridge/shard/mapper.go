// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
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
		FromPF: func(_ bool) string {
			if pf.Identity.Name == "" || doc.Name == pf.Identity.Name {
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
		FromPF: func(_ bool) string {
			if pf.Identity.Version == "" || doc.Version == pf.Identity.Version {
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
			if s == "" {
				return core.ClearExt(force, &doc.Description)
			}
			if strings.TrimSpace(doc.Description) == s {
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
			return core.MergeAuthorStrings(pf, doc.Authors)
		},
		FromPF: func(force bool) string {
			entries := core.AuthorStrings(pf.People)
			if len(entries) == 0 {
				return core.ClearExt(force, &doc.Authors)
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
			want := ""
			if pf.Requirements != nil {
				want = pf.Requirements.Runtime[crystalRequirementKey]
			}
			if want == "" {
				return core.ClearExt(force, &doc.Crystal)
			}
			if doc.Crystal == want {
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
				if isURL(doc.License) {
					return ""
				}
				return core.ClearExt(force, &doc.License)
			}
			if doc.License == pf.License.Spdx {
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
			if url == "" {
				return core.ClearExt(force, &doc.Homepage)
			}
			if doc.Homepage == url {
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
			primary := pfmodel.PackagesRepository(pf)
			if primary != nil && primary.URL == doc.Repository {
				return ""
			}
			if primary != nil && primary.URL != "" && !force {
				return ""
			}
			slot := pfmodel.EnsurePackagesRepository(pf)
			slot.URL = doc.Repository
			if slot.Type == "" {
				slot.Type = "git"
			}
			return core.Trunc(doc.Repository)
		},
		FromPF: func(force bool) string {
			primary := pfmodel.PackagesRepository(pf)
			if primary == nil || primary.URL == "" {
				return core.ClearExt(force, &doc.Repository)
			}
			if doc.Repository == primary.URL {
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
			if url == "" {
				return core.ClearExt(force, &doc.Documentation)
			}
			if doc.Documentation == url {
				return ""
			}
			doc.Documentation = url
			return core.Trunc(url)
		},
	}
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
