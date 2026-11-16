// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"fmt"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// rustRequirementKey is the requirements.runtime entry rust-version maps to.
const rustRequirementKey = "rust"

// maxKeywords is the crates.io limit on keywords per crate.
const maxKeywords = 5

// slot reads and writes one projectfile value a manifest key maps to.
type slot struct {
	pfKey string
	get   func() string
	set   func(string)
}

// buildMappers returns one FieldMapper per owned manifest key.
func buildMappers(doc *Document, pf *projectfile.Document) core.MapperList {
	doc.ReuseHeader = core.REUSEHeader(pf, core.StyleHash)
	slots := map[string]slot{
		keyName:          {"identity.name", func() string { return pf.Identity.Name }, func(v string) { pf.Identity.Name = v }},
		keyVersion:       {"identity.version", func() string { return pf.Identity.Version }, func(v string) { pf.Identity.Version = v }},
		keyDescription:   {"identity.summary", func() string { return projectfile.ExtractLocalizedString(pf.Identity.Summary) }, func(v string) { projectfile.SetLocalizedEN(&pf.Identity.Summary, v) }},
		keyLicense:       {"license.spdx", func() string { return licenseOf(pf) }, func(v string) { setLicense(pf, v) }},
		keyHomepage:      {"links[type=homepage]", func() string { return pfmodel.LinkURL(pf, projectfile.LinkHomepage) }, func(v string) { pfmodel.SetLink(pf, projectfile.LinkHomepage, v, true) }},
		keyRepository:    {"repositories[role=origin]", func() string { return pfmodel.PackagesRepositoryURL(pf) }, func(v string) { setRepository(pf, v) }},
		keyDocumentation: {"links[type=documentation,tag=main-documentation]", func() string { return pfmodel.MainDocumentationURL(pf) }, func(v string) { pfmodel.SetMainDocumentationURL(pf, v, true) }},
		keyRustVersion:   {"requirements.runtime[rust]", func() string { return rustOf(pf) }, func(v string) { setRust(pf, v) }},
	}
	var list core.MapperList
	for _, key := range ownedKeys {
		switch key {
		case keyAuthors:
			list = append(list, mapAuthors(doc, pf))
		case keyKeywords:
			list = append(list, mapKeywords(doc, pf))
		default:
			list = append(list, mapString(doc, key, slots[key]))
		}
	}
	return list
}

// mapString syncs one scalar manifest key with its projectfile slot.
func mapString(doc *Document, key string, s slot) core.FieldMapper {
	field := doc.stringField(key)
	return core.FieldMapper{
		ExtKey: key, PFKey: s.pfKey,
		ToPF: func(force bool) string {
			v := strings.TrimSpace(*field)
			cur := s.get()
			if v == "" || cur == v || (cur != "" && !force) {
				return ""
			}
			s.set(v)
			return core.Trunc(v)
		},
		FromPF: func(force bool) string {
			if !doc.Owns(key) {
				genlog.Debug("Cargo.toml key left to its workspace", "key", key)
				return ""
			}
			v := s.get()
			if v == "" && (key == keyName || key == keyVersion) {
				return ""
			}
			if v == "" {
				return core.ClearExt(force, field)
			}
			if *field == v {
				return ""
			}
			*field = v
			return core.Trunc(v)
		},
	}
}

// mapAuthors merges the "Name <email>" author strings into people as authors.
func mapAuthors(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: keyAuthors, PFKey: "people",
		ToPF: func(_ bool) string {
			return core.MergeAuthorStrings(pf, doc.Authors)
		},
		FromPF: func(force bool) string {
			if !doc.Owns(keyAuthors) {
				return ""
			}
			entries := core.AuthorStrings(pf.People)
			if len(entries) == 0 {
				return core.ClearExt(force, &doc.Authors)
			}
			if slices.Equal(doc.Authors, entries) {
				return ""
			}
			doc.Authors = entries
			return fmt.Sprintf("%d author(s)", len(entries))
		},
	}
}

// mapKeywords syncs the bare (non-namespaced) projectfile keywords.
func mapKeywords(doc *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: keyKeywords, PFKey: "keywords",
		ToPF: func(force bool) string {
			bare := bareKeywords(pf.Keywords)
			if len(doc.Keywords) == 0 || slices.Equal(bare, doc.Keywords) || (len(bare) > 0 && !force) {
				return ""
			}
			pf.Keywords = append(slices.Clone(doc.Keywords), namespacedKeywords(pf.Keywords)...)
			return fmt.Sprintf("%d keyword(s)", len(doc.Keywords))
		},
		FromPF: func(force bool) string {
			if !doc.Owns(keyKeywords) {
				return ""
			}
			bare := bareKeywords(pf.Keywords)
			if len(bare) == 0 {
				return core.ClearExt(force, &doc.Keywords)
			}
			if len(bare) > maxKeywords {
				genlog.Warn("crates.io accepts at most 5 keywords; publishing will refuse this manifest", "count", len(bare))
			}
			if slices.Equal(doc.Keywords, bare) {
				return ""
			}
			doc.Keywords = bare
			return fmt.Sprintf("%d keyword(s)", len(bare))
		},
	}
}

// bareKeywords selects keywords without a "namespace:" prefix.
func bareKeywords(all []string) []string {
	var out []string
	for _, k := range all {
		if !strings.Contains(k, ":") {
			out = append(out, k)
		}
	}
	return out
}

// namespacedKeywords selects keywords carrying a "namespace:" prefix.
func namespacedKeywords(all []string) []string {
	var out []string
	for _, k := range all {
		if strings.Contains(k, ":") {
			out = append(out, k)
		}
	}
	return out
}

// licenseOf reads the SPDX expression.
func licenseOf(pf *projectfile.Document) string {
	if pf.License == nil {
		return ""
	}
	return pf.License.Spdx
}

// setLicense stores the SPDX expression, allocating the block on first write.
func setLicense(pf *projectfile.Document, v string) {
	if pf.License == nil {
		pf.License = &projectfile.License{}
	}
	pf.License.Spdx = v
}

// setRepository stores the origin repository URL, typing a new entry as git.
func setRepository(pf *projectfile.Document, v string) {
	r := pfmodel.EnsurePackagesRepository(pf)
	r.URL = v
	if r.Type == "" {
		r.Type = "git"
	}
}

// rustOf reads the minimum supported Rust version.
func rustOf(pf *projectfile.Document) string {
	if pf.Requirements == nil {
		return ""
	}
	return pf.Requirements.Runtime[rustRequirementKey]
}

// setRust stores the minimum supported Rust version, allocating the blocks on first write.
func setRust(pf *projectfile.Document, v string) {
	if pf.Requirements == nil {
		pf.Requirements = &projectfile.Requirements{}
	}
	if pf.Requirements.Runtime == nil {
		pf.Requirements.Runtime = map[string]string{}
	}
	pf.Requirements.Runtime[rustRequirementKey] = v
}
