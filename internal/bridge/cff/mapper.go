// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const orcidPrefix = "https://orcid.org/"

// CFF identifier type names + extension-key vocabulary. Duplicated string
// literals across the mapper became hard to grep; promoting to constants
// also documents the controlled vocabulary in one place.
const (
	cffKeyURL = "url"
	cffKeyDOI = "doi"
)

// buildMappers is the single source of truth for CFF ↔ projectfile field
// mappings. Each mapper closure accepts a `force` flag:
//   - force=true:  push semantics — overwrite the target with the source value.
//   - force=false: gap-fill semantics — only fill the target when it is empty.
//
// This lets the bidir algorithm run the authoritative direction with
// force=true and the reverse direction with force=false in the same loop.
//
// Add a new field by appending one entry to the returned slice.
func buildMappers(c *Document, pf *projectfile.Document) core.MapperList {
	if c == nil {
		c = New()
	}
	// Stash the REUSE/SPDX block on the cff doc so Write can prepend it ahead
	// of the YAML body. BuildMappers is the only place the driver has pf in
	// scope, so the header is computed here and consumed later by Write.
	c.ReuseHeader = core.REUSEHeader(pf, core.StyleHash)
	return core.MapperList{
		{
			ExtKey: "title", PFKey: "identity.title",
			ToPF: func(force bool) string {
				if c.Title == "" {
					return ""
				}
				existing := projectfile.ExtractLocalizedString(pf.Identity.Title)
				if existing == c.Title {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				projectfile.SetLocalizedEN(&pf.Identity.Title, c.Title)
				return core.Trunc(c.Title)
			},
			FromPF: func(force bool) string {
				title := projectfile.ExtractLocalizedString(pf.Identity.Title)
				if title == "" {
					return ""
				}
				if c.Title == title {
					return ""
				}
				if c.Title != "" && !force {
					return ""
				}
				c.Title = title
				return core.Trunc(title)
			},
		},
		{
			ExtKey: "abstract", PFKey: "identity.summary",
			ToPF: func(force bool) string {
				if c.Abstract == "" {
					return ""
				}
				existing := projectfile.ExtractLocalizedString(pf.Identity.Summary)
				if existing == c.Abstract {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				projectfile.SetLocalizedEN(&pf.Identity.Summary, c.Abstract)
				return core.Trunc(c.Abstract)
			},
			FromPF: func(force bool) string {
				summary := projectfile.ExtractLocalizedString(pf.Identity.Summary)
				if summary == "" {
					return ""
				}
				if c.Abstract == summary {
					return ""
				}
				if c.Abstract != "" && !force {
					return ""
				}
				c.Abstract = summary
				return core.Trunc(summary)
			},
		},
		{
			ExtKey: "version", PFKey: "identity.version",
			ToPF: func(force bool) string {
				if c.Version == "" || pf.Identity.Version == c.Version {
					return ""
				}
				if pf.Identity.Version != "" && !force {
					return ""
				}
				pf.Identity.Version = c.Version
				return c.Version
			},
			FromPF: func(force bool) string {
				if pf.Identity.Version == "" || pf.Identity.Version == c.Version {
					return ""
				}
				if c.Version != "" && !force {
					return ""
				}
				c.Version = pf.Identity.Version
				return pf.Identity.Version
			},
		},
		{
			ExtKey: "date-released", PFKey: "identity.released",
			ToPF: func(force bool) string {
				if c.DateReleased == "" || pf.Identity.Released == c.DateReleased {
					return ""
				}
				if pf.Identity.Released != "" && !force {
					return ""
				}
				pf.Identity.Released = c.DateReleased
				return c.DateReleased
			},
			FromPF: func(force bool) string {
				if pf.Identity.Released == "" || pf.Identity.Released == c.DateReleased {
					return ""
				}
				if c.DateReleased != "" && !force {
					return ""
				}
				c.DateReleased = pf.Identity.Released
				return pf.Identity.Released
			},
		},
		{
			ExtKey: "license", PFKey: "license.spdx",
			ToPF: func(force bool) string {
				licenseStr := cffLicenseToSPDX(c.License)
				if licenseStr == "" {
					return ""
				}
				existing := ""
				if pf.License != nil {
					existing = pf.License.Spdx
				}
				if existing == licenseStr {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				ensureLicense(pf)
				pf.License.Spdx = licenseStr
				return licenseStr
			},
			FromPF: func(force bool) string {
				if pf.License == nil || pf.License.Spdx == "" {
					return ""
				}
				expr := pf.License.Spdx
				// CFF 1.2.0 license accepts a single SPDX ID or an array of IDs
				// with OR semantics. OR compound expressions map to arrays
				// (semantic match). AND and WITH cannot be represented in CFF
				// without semantic distortion — skip them.
				var newVal any
				if spdx.IsCompound(expr) {
					terms, conj := spdx.SplitCompound(expr)
					if conj != "OR" || len(terms) <= 1 {
						return ""
					}
					// Strip WITH exceptions from each term for CFF (schema only
					// accepts bare SPDX IDs in the array).
					bare := make([]string, 0, len(terms))
					for _, t := range terms {
						bare = append(bare, spdx.StripException(t))
					}
					newVal = bare
				} else {
					newVal = expr
				}
				// Compare against current CFF value (normalised to SPDX string).
				if cffLicenseToSPDX(c.License) == expr {
					return ""
				}
				if c.License != nil && c.License != "" && !force {
					return ""
				}
				c.License = newVal
				return expr
			},
		},
		{
			ExtKey: "repository-code", PFKey: "links[type=source-code]",
			ToPF: func(force bool) string {
				if c.RepositoryCode == "" {
					return ""
				}
				existing := pfmodel.CitableRepositoryURL(pf)
				if existing == c.RepositoryCode {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				slot := pfmodel.EnsureCitableSourceLink(pf)
				slot.URL = c.RepositoryCode
				return core.Trunc(c.RepositoryCode)
			},
			FromPF: func(force bool) string {
				url := pfmodel.CitableRepositoryURL(pf)
				if url == "" {
					return ""
				}
				if c.RepositoryCode == url {
					return ""
				}
				if c.RepositoryCode != "" && !force {
					return ""
				}
				c.RepositoryCode = url
				return core.Trunc(url)
			},
		},
		{
			ExtKey: cffKeyURL, PFKey: "links[type=homepage]",
			ToPF: func(force bool) string {
				if c.URL == "" {
					return ""
				}
				existing := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
				if existing == c.URL {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				pfmodel.SetLink(pf, projectfile.LinkHomepage, c.URL, true)
				return core.Trunc(c.URL)
			},
			FromPF: func(force bool) string {
				url := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
				if url == "" {
					return ""
				}
				if c.URL == url {
					return ""
				}
				if c.URL != "" && !force {
					return ""
				}
				c.URL = url
				return core.Trunc(c.URL)
			},
		},
		{
			ExtKey: "keywords", PFKey: "keywords",
			// Keywords: same semantics as old code — fill only when target empty.
			// force=true would overwrite a non-empty list which is rarely desired;
			// we still let it through so an explicit --mode from-pf / to-pf can
			// replace lists outright.
			ToPF: func(force bool) string {
				if len(c.Keywords) == 0 {
					return ""
				}
				if len(pf.Keywords) > 0 && !force {
					return ""
				}
				if sameStringSet(pf.Keywords, c.Keywords) {
					return ""
				}
				pf.Keywords = append([]string(nil), c.Keywords...)
				return fmt.Sprintf("%d keyword(s)", len(c.Keywords))
			},
			FromPF: func(force bool) string {
				bare := extractBareKeywords(pf.Keywords)
				if len(bare) == 0 {
					return ""
				}
				if len(c.Keywords) > 0 && !force {
					return ""
				}
				if sameStringSet(c.Keywords, bare) {
					return ""
				}
				c.Keywords = bare
				return core.Trunc(strings.Join(bare, ", "))
			},
		},
		{
			ExtKey: "authors/contact", PFKey: "people",
			// CFF 1.2.0 only defines `authors[]` (required) and `contact[]`
			// (optional) at the top level — no `maintainers` / `contributors`.
			// Earlier output of those non-spec arrays produced invalid CFF
			// that the embedded validator now rejects. Consolidation rules:
			//   - Anyone with role author/maintainer/contributor lands in
			//     authors[] (CFF carries no role discriminator, so the
			//     three projectfile roles collapse losslessly).
			//   - Anyone with role security/maintainer is also surfaced in
			//     contact[] so SECURITY.md-style consumers can pick them
			//     out without re-deriving role membership.
			// People and organizations are always merged (set-union by
			// identity key + role union); empty entries are filtered out
			// before merging.
			ToPF: func(_ bool) string {
				peopleAuthors, orgAuthors := cffPeopleToPF(c.Authors, "author")
				peopleContact, orgContact := cffPeopleToPF(c.Contact, "maintainer")
				allPeople := append(filterNonEmptyPeople(peopleAuthors), filterNonEmptyPeople(peopleContact)...)
				allOrgs := append(filterNonEmptyOrgs(orgAuthors), filterNonEmptyOrgs(orgContact)...)
				if len(allPeople) == 0 && len(allOrgs) == 0 {
					return ""
				}
				beforePeople := len(pf.People)
				beforeOrgs := len(pf.Organizations)
				mergedPeople, peopleConflicts := projectfile.MergePeople(pf.People, allPeople)
				mergedOrgs, orgConflicts := projectfile.MergeOrganizations(pf.Organizations, allOrgs)
				pf.People = mergedPeople
				pf.Organizations = mergedOrgs
				core.RecordPersonConflicts(pf, peopleConflicts)
				core.RecordPersonConflicts(pf, orgConflicts)
				addedPeople := len(pf.People) - beforePeople
				addedOrgs := len(pf.Organizations) - beforeOrgs
				if addedPeople == 0 && addedOrgs == 0 && len(peopleConflicts) == 0 && len(orgConflicts) == 0 {
					return ""
				}
				return fmt.Sprintf("%d person(s), %d org(s) merged", len(allPeople), len(allOrgs))
			},
			FromPF: func(_ bool) string {
				touched := 0
				peopleForAuthors := filterByAnyRole(pf.People, "author", "maintainer", "contributor")
				orgsForAuthors := filterOrgsByAnyRole(pf.Organizations, "author", "maintainer", "contributor")
				newAuthors := append(pfPeopleToCFF(peopleForAuthors), pfOrgsToCFF(orgsForAuthors)...)
				if !equalCFFPeople(c.Authors, newAuthors) {
					c.Authors = newAuthors
					touched += len(newAuthors)
				}
				peopleForContact := filterByAnyRole(pf.People, "security", "maintainer")
				orgsForContact := filterOrgsByAnyRole(pf.Organizations, "security", "maintainer")
				newContact := append(pfPeopleToCFF(peopleForContact), pfOrgsToCFF(orgsForContact)...)
				if !equalCFFPeople(c.Contact, newContact) {
					c.Contact = newContact
					touched += len(newContact)
				}
				c.Maintainers = nil
				c.Contributors = nil
				if touched == 0 {
					return ""
				}
				return fmt.Sprintf("%d person(s)", touched)
			},
		},
		{
			ExtKey: cffKeyDOI, PFKey: "org.projectfile.citation.doi",
			// DOI lives in CFF at top level; in projectfile under the citation
			// extension table. We read/write the extension table here.
			ToPF: func(force bool) string {
				if c.DOI == "" {
					return ""
				}
				existing := citExtString(pf, cffKeyDOI)
				if existing == c.DOI {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				setCitExtString(pf, cffKeyDOI, c.DOI)
				return core.Trunc(c.DOI)
			},
			FromPF: func(force bool) string {
				doi := citExtString(pf, cffKeyDOI)
				if doi == "" {
					return ""
				}
				if c.DOI == doi {
					return ""
				}
				if c.DOI != "" && !force {
					return ""
				}
				c.DOI = doi
				return core.Trunc(doi)
			},
		},
		{
			ExtKey: "preferred-citation", PFKey: "org.projectfile.citation.preferred",
			ToPF: func(force bool) string {
				if c.PreferredCitation == nil {
					return ""
				}
				if hasCitExtKey(pf, "preferred") && !force {
					return ""
				}
				setCitExtPreferred(pf, c.PreferredCitation)
				return core.Trunc(c.PreferredCitation.Title)
			},
			FromPF: func(force bool) string {
				ext, _ := pfmodel.GetCitationExtension(pf)
				if ext == nil || ext.Preferred == nil {
					return ""
				}
				if c.PreferredCitation != nil && !force {
					return ""
				}
				c.PreferredCitation = &Reference{
					Type:    ext.Preferred.Type,
					Title:   ext.Preferred.Title,
					Journal: ext.Preferred.Journal,
					Volume:  ext.Preferred.Volume,
					Issue:   ext.Preferred.Issue,
					Pages:   ext.Preferred.Pages,
					Year:    ext.Preferred.Year,
					DOI:     ext.Preferred.DOI,
				}
				return core.Trunc(ext.Preferred.Title)
			},
		},
		{
			ExtKey: "identifiers", PFKey: "links",
			ToPF: func(_ bool) string {
				// Links always merge by (type,url); force is irrelevant because
				// mergeLinks itself is idempotent.
				added := identifiersToLinks(c.Identifiers)
				if len(added) == 0 {
					return ""
				}
				before := len(pf.Links)
				pf.Links = mergeLinks(pf.Links, added)
				if len(pf.Links) == before {
					return ""
				}
				return fmt.Sprintf("%d link(s)", len(pf.Links)-before)
			},
			FromPF: func(force bool) string {
				ids := linksToIdentifiers(pf.Links)
				if len(ids) == 0 {
					return ""
				}
				if len(c.Identifiers) > 0 && !force {
					return ""
				}
				c.Identifiers = ids
				return fmt.Sprintf("%d identifier(s)", len(ids))
			},
		},
	}
}

// --- shared helpers (formerly in sync/mapping.go, sync/from_pf.go) ---

func ensureLicense(pf *projectfile.Document) {
	if pf.License == nil {
		pf.License = &projectfile.License{}
	}
}

// LicenseToString normalises the CFF license field (which can be a string
// scalar, a []string, or a []any from YAML unmarshal) into an SPDX expression
// string. Array elements are joined with " OR " — matching CFF 1.2.0's
// documented multi-license semantics.
func LicenseToString(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []string:
		return strings.Join(val, " OR ")
	case []any:
		parts := make([]string, 0, len(val))
		for _, e := range val {
			if s, ok := e.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " OR ")
	default:
		return ""
	}
}

// cffLicenseToSPDX is the unexported alias used internally in this file.
func cffLicenseToSPDX(v any) string { return LicenseToString(v) }

// pfPeopleToCFF projects projectfile people onto the CFF PersonEntity shape
// with strict person/entity discrimination. CFF 1.2.0 forbids person-only
// keys on an entity record and vice versa (additionalProperties:false on
// both upstream definitions); copying without branching would produce
// schema-invalid CITATION.cff. Discriminator follows the projectfile spec
// §5.5: family-names present ⇒ person; bare name ⇒ entity.
func pfPeopleToCFF(in []projectfile.Person) []PersonEntity {
	if len(in) == 0 {
		return nil
	}
	out := make([]PersonEntity, 0, len(in))
	for _, p := range in {
		ce := PersonEntity{
			Alias: p.Alias,
			Email: p.Email,
		}
		if p.Orcid != "" {
			if strings.HasPrefix(p.Orcid, orcidPrefix) {
				ce.Orcid = p.Orcid
			} else {
				ce.Orcid = orcidPrefix + p.Orcid
			}
		}
		if p.URL != "" {
			ce.Website = p.URL
		}
		if p.FamilyNames != "" {
			ce.FamilyNames = p.FamilyNames
			ce.GivenNames = p.GivenNames
			ce.NameParticle = p.NameParticle
			ce.NameSuffix = p.NameSuffix
			ce.Affiliation = p.Affiliation
		} else {
			continue
		}
		out = append(out, ce)
	}
	return out
}

func pfOrgsToCFF(in []projectfile.Organization) []PersonEntity {
	if len(in) == 0 {
		return nil
	}
	out := make([]PersonEntity, 0, len(in))
	for _, o := range in {
		ce := PersonEntity{
			Name:  o.Name,
			Alias: o.Alias,
			Email: o.Email,
		}
		if o.Orcid != "" {
			if strings.HasPrefix(o.Orcid, orcidPrefix) {
				ce.Orcid = o.Orcid
			} else {
				ce.Orcid = orcidPrefix + o.Orcid
			}
		}
		if o.URL != "" {
			ce.Website = o.URL
		}
		if ce.Name == "" && ce.Alias == "" && ce.Email == "" && ce.Orcid == "" && ce.Website == "" {
			continue
		}
		out = append(out, ce)
	}
	return out
}

// cffPeopleToPF is the inverse projection with the same discrimination.
// CFF records that arrive with both `family-names` and `name` (invalid per
// the upstream schema but possible from hand-edited input) are treated as
// persons — `family-names` wins because it's the more specific signal.
func cffPeopleToPF(in []PersonEntity, role string) ([]projectfile.Person, []projectfile.Organization) {
	if len(in) == 0 {
		return nil, nil
	}
	var people []projectfile.Person
	var orgs []projectfile.Organization
	for _, ce := range in {
		if ce.FamilyNames != "" {
			p := projectfile.Person{
				FamilyNames:  ce.FamilyNames,
				GivenNames:   ce.GivenNames,
				NameParticle: ce.NameParticle,
				NameSuffix:   ce.NameSuffix,
				Alias:        ce.Alias,
				Email:        ce.Email,
				Affiliation:  ce.Affiliation,
				Roles:        []string{role},
			}
			if ce.Orcid != "" {
				p.Orcid = strings.TrimPrefix(ce.Orcid, orcidPrefix)
			}
			if ce.Website != "" {
				p.URL = ce.Website
			}
			people = append(people, p)
		} else if ce.Name != "" {
			o := projectfile.Organization{
				Name:  ce.Name,
				Alias: ce.Alias,
				Email: ce.Email,
				Roles: []string{role},
			}
			if ce.Orcid != "" {
				o.Orcid = strings.TrimPrefix(ce.Orcid, orcidPrefix)
			}
			if ce.Website != "" {
				o.URL = ce.Website
			}
			orgs = append(orgs, o)
		}
	}
	return people, orgs
}

func filterByAnyRole(people []projectfile.Person, roles ...string) []projectfile.Person {
	want := make(map[string]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}
	var out []projectfile.Person
	for _, p := range people {
		for _, r := range p.Roles {
			if want[r] {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func filterOrgsByAnyRole(orgs []projectfile.Organization, roles ...string) []projectfile.Organization {
	want := make(map[string]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}
	var out []projectfile.Organization
	for _, o := range orgs {
		for _, r := range o.Roles {
			if want[r] {
				out = append(out, o)
				break
			}
		}
	}
	return out
}

func extractBareKeywords(keywords []string) []string {
	var out []string
	for _, kw := range keywords {
		if !strings.Contains(kw, ":") {
			out = append(out, kw)
		}
	}
	return out
}

// equalCFFPeople compares two []PersonEntity slices field-by-field.
// PersonEntity has only string fields so `==` is well-defined; this just
// adds the length check and a positional walk. Used by FromPF to skip
// a phantom "N person(s)" report when projectfile and cff already agree.
func equalCFFPeople(a, b []PersonEntity) bool {
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

// filterNonEmptyPF drops PersonOrEntity records that carry no identifying
// information. Role-only entries can't dedup against anything in
// projectfile.MergePeople — see ParsePerson notes in the npm driver for the
// historical defect this guards against.
func filterNonEmptyPeople(in []projectfile.Person) []projectfile.Person {
	if len(in) == 0 {
		return in
	}
	out := in[:0:0]
	for _, p := range in {
		if p.FamilyNames == "" && p.GivenNames == "" &&
			p.DisplayName == "" && p.Alias == "" && p.Email == "" &&
			p.URL == "" && p.Orcid == "" && p.Affiliation == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func filterNonEmptyOrgs(in []projectfile.Organization) []projectfile.Organization {
	if len(in) == 0 {
		return in
	}
	out := in[:0:0]
	for _, o := range in {
		if o.Name == "" && o.Alias == "" && o.Email == "" &&
			o.URL == "" && o.Orcid == "" {
			continue
		}
		out = append(out, o)
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			return false
		}
	}
	return true
}

func identifiersToLinks(ids []Identifier) []projectfile.Link {
	var out []projectfile.Link
	for _, id := range ids {
		switch id.Type {
		case cffKeyDOI:
			out = append(out, projectfile.Link{Type: "doi-landing", URL: id.Value})
		case cffKeyURL:
			out = append(out, projectfile.Link{Type: "zenodo-archive", URL: id.Value})
		}
	}
	return out
}

func linksToIdentifiers(links []projectfile.Link) []Identifier {
	var out []Identifier
	for _, l := range links {
		switch l.Type {
		case "doi-landing":
			out = append(out, Identifier{Type: cffKeyDOI, Value: l.URL})
		case "zenodo-archive":
			out = append(out, Identifier{Type: cffKeyURL, Value: l.URL})
		}
	}
	return out
}

func mergeLinks(existing, added []projectfile.Link) []projectfile.Link {
	seen := make(map[string]bool, len(existing))
	for _, l := range existing {
		seen[l.Type+"|"+l.URL] = true
	}
	out := make([]projectfile.Link, len(existing))
	copy(out, existing)
	for _, l := range added {
		key := l.Type + "|" + l.URL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, l)
	}
	return out
}

// --- citation-extension helpers ---

func citExtMap(pf *projectfile.Document) map[string]any {
	if pf.Extensions == nil {
		return nil
	}
	if m, ok := pf.Extensions["org.projectfile.citation"].(map[string]any); ok {
		return m
	}
	return nil
}

func citExtString(pf *projectfile.Document, key string) string {
	m := citExtMap(pf)
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func hasCitExtKey(pf *projectfile.Document, key string) bool {
	m := citExtMap(pf)
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func setCitExtString(pf *projectfile.Document, key, val string) {
	if pf.Extensions == nil {
		pf.Extensions = map[string]any{}
	}
	m, _ := pf.Extensions["org.projectfile.citation"].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	m[key] = val
	pf.Extensions["org.projectfile.citation"] = m
}

func setCitExtPreferred(pf *projectfile.Document, ref *Reference) {
	if pf.Extensions == nil {
		pf.Extensions = map[string]any{}
	}
	m, _ := pf.Extensions["org.projectfile.citation"].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	pref := map[string]any{}
	if ref.Type != "" {
		pref["type"] = ref.Type
	}
	if ref.Title != "" {
		pref["title"] = ref.Title
	}
	if ref.Journal != "" {
		pref["journal"] = ref.Journal
	}
	if ref.Volume != 0 {
		pref["volume"] = ref.Volume
	}
	if ref.Issue != 0 {
		pref["issue"] = ref.Issue
	}
	if ref.Pages != "" {
		pref["pages"] = ref.Pages
	}
	if ref.Year != 0 {
		pref["year"] = ref.Year
	}
	if ref.DOI != "" {
		pref[cffKeyDOI] = ref.DOI
	}
	m["preferred"] = pref
	pf.Extensions["org.projectfile.citation"] = m
}

// --- driver-helper shims ---

func cloneDocument(d *Document) *Document {
	cp := *d
	cp.Authors = clonePersonEntities(d.Authors)
	cp.Contributors = clonePersonEntities(d.Contributors)
	cp.Maintainers = clonePersonEntities(d.Maintainers)
	cp.Contact = clonePersonEntities(d.Contact)
	cp.Keywords = append([]string(nil), d.Keywords...)
	cp.Identifiers = append([]Identifier(nil), d.Identifiers...)
	cp.References = append([]Reference(nil), d.References...)
	if d.PreferredCitation != nil {
		pref := *d.PreferredCitation
		cp.PreferredCitation = &pref
	}
	cp.Rest = d.Rest.Clone()
	return &cp
}

func clonePersonEntities(in []PersonEntity) []PersonEntity {
	if in == nil {
		return nil
	}
	out := make([]PersonEntity, len(in))
	copy(out, in)
	return out
}
