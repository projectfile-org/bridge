// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	npmKeyName        = "name"
	npmKeyDescription = "description"
	npmKeyKeywords    = "keywords"
)

// buildMappers is a port of the previous sync_npm.buildMappers, lifted to the
// FieldMapper(force bool) shape. Each closure preserves the original
// per-field semantics (push, gap-fill, merge) plus a `force` toggle so the
// bidir core can run gap-fill passes through the same closures.
//
// Add a new field by appending one entry to the returned slice.
func buildMappers(pkg *Document, pf *projectfile.Document) core.MapperList {
	return core.MapperList{
		{
			ExtKey: npmKeyName, PFKey: "identity.name",
			ToPF: func(force bool) string {
				if pkg.Name == "" {
					return ""
				}
				ns, name := npmNameToPFName(pkg.Name, pf.Identity.Namespace)
				changed := false
				if ns != "" && pf.Identity.Namespace != ns && (pf.Identity.Namespace == "" || force) {
					pf.Identity.Namespace = ns
					changed = true
				}
				if name != "" && pf.Identity.Name != name && (pf.Identity.Name == "" || force) {
					pf.Identity.Name = name
					changed = true
				}
				if !changed {
					return ""
				}
				return pkg.Name
			},
			FromPF: func(force bool) string {
				name := pfNameToNPM(pf.Identity.Namespace, pf.Identity.Name)
				if name == "" {
					return ""
				}
				if pkg.Name == name {
					return ""
				}
				if pkg.Name != "" && !force {
					return ""
				}
				pkg.Name = name
				return name
			},
		},
		{
			ExtKey: "version", PFKey: "identity.version",
			ToPF: func(force bool) string {
				if pkg.Version == "" || pf.Identity.Version == pkg.Version {
					return ""
				}
				if pf.Identity.Version != "" && !force {
					return ""
				}
				pf.Identity.Version = pkg.Version
				return pkg.Version
			},
			FromPF: func(force bool) string {
				if pf.Identity.Version == "" || pkg.Version == pf.Identity.Version {
					return ""
				}
				if pkg.Version != "" && !force {
					return ""
				}
				pkg.Version = pf.Identity.Version
				return pf.Identity.Version
			},
		},
		{
			ExtKey: npmKeyDescription, PFKey: "identity.summary",
			ToPF: func(force bool) string {
				if pkg.Description == "" {
					return ""
				}
				existing := projectfile.ExtractLocalizedString(pf.Identity.Summary)
				if existing == pkg.Description {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				projectfile.SetLocalizedEN(&pf.Identity.Summary, pkg.Description)
				return core.Trunc(pkg.Description)
			},
			FromPF: func(force bool) string {
				s := projectfile.ExtractLocalizedString(pf.Identity.Summary)
				if s == "" {
					return ""
				}
				if pkg.Description == s {
					return ""
				}
				if pkg.Description != "" && !force {
					return ""
				}
				pkg.Description = s
				return core.Trunc(s)
			},
		},
		{
			ExtKey: "license", PFKey: "license.spdx",
			ToPF: func(force bool) string {
				if pkg.License == "" {
					return ""
				}
				existing := ""
				if pf.License != nil {
					existing = pf.License.Spdx
				}
				if existing == pkg.License {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				ensureLicense(pf)
				pf.License.Spdx = pkg.License
				return pkg.License
			},
			FromPF: func(force bool) string {
				if pf.License == nil || pf.License.Spdx == "" {
					return ""
				}
				if pkg.License == pf.License.Spdx {
					return ""
				}
				if pkg.License != "" && !force {
					return ""
				}
				pkg.License = pf.License.Spdx
				return pf.License.Spdx
			},
		},
		{
			ExtKey: "homepage", PFKey: "links[type=homepage]",
			ToPF: func(force bool) string {
				if pkg.Homepage == "" {
					return ""
				}
				existing := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
				if existing == pkg.Homepage {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				pfmodel.SetLink(pf, projectfile.LinkHomepage, pkg.Homepage, true)
				return core.Trunc(pkg.Homepage)
			},
			FromPF: func(force bool) string {
				url := pfmodel.LinkURL(pf, projectfile.LinkHomepage)
				if url == "" {
					return ""
				}
				if pkg.Homepage == url {
					return ""
				}
				if pkg.Homepage != "" && !force {
					return ""
				}
				pkg.Homepage = url
				return core.Trunc(pkg.Homepage)
			},
		},
		{
			ExtKey: "repository", PFKey: "repositories[role=origin]",
			ToPF: func(force bool) string {
				repo := ParseRepository(pkg.Repository)
				if repo.URL == "" {
					return ""
				}
				primary := pfmodel.PrimaryRepository(pf)
				if primary != nil && primary.URL != "" && !force {
					return ""
				}
				if primary != nil && primary.URL == repo.URL {
					return ""
				}
				slot := pfmodel.EnsurePrimaryRepository(pf)
				slot.URL = repo.URL
				if slot.Type == "" {
					slot.Type = repo.Type
				}
				if repo.Directory != "" {
					slot.Path = repo.Directory
				}
				return core.Trunc(repo.URL)
			},
			FromPF: func(force bool) string {
				primary := pfmodel.PrimaryRepository(pf)
				if primary == nil || primary.URL == "" {
					return ""
				}
				if pkg.Repository != nil && !force {
					return ""
				}
				repoType := primary.Type
				if repoType == "" {
					repoType = "git"
				}
				if cur := ParseRepository(pkg.Repository); cur.URL == primary.URL && cur.Type == repoType {
					return ""
				}
				pkg.Repository = Repository{URL: primary.URL, Type: repoType}
				return core.Trunc(primary.URL)
			},
		},
		{
			ExtKey: "bugs", PFKey: "links[type=bugs]",
			ToPF: func(force bool) string {
				bugs := ParseBugs(pkg.Bugs)
				if bugs.URL == "" {
					return ""
				}
				existing := pfmodel.LinkURL(pf, projectfile.LinkBugs)
				if existing == bugs.URL {
					return ""
				}
				if existing != "" && !force {
					return ""
				}
				pfmodel.SetLink(pf, projectfile.LinkBugs, bugs.URL, true)
				return core.Trunc(bugs.URL)
			},
			FromPF: func(force bool) string {
				url := pfmodel.LinkURL(pf, projectfile.LinkBugs)
				if url == "" {
					return ""
				}
				if pkg.Bugs != nil && !force {
					return ""
				}
				if ParseBugs(pkg.Bugs).URL == url {
					return ""
				}
				pkg.Bugs = Bugs{URL: url}
				return core.Trunc(url)
			},
		},
		{
			ExtKey: npmKeyKeywords, PFKey: "keywords",
			ToPF: func(force bool) string {
				if len(pkg.Keywords) == 0 {
					return ""
				}
				if equalStringSlice(pf.Keywords, pkg.Keywords) {
					return ""
				}
				if len(pf.Keywords) > 0 && !force {
					return ""
				}
				pf.Keywords = append([]string(nil), pkg.Keywords...)
				return fmt.Sprintf("%d keyword(s)", len(pkg.Keywords))
			},
			FromPF: func(force bool) string {
				if len(pf.Keywords) == 0 {
					return ""
				}
				if equalStringSlice(pkg.Keywords, pf.Keywords) {
					return ""
				}
				if len(pkg.Keywords) > 0 && !force {
					return ""
				}
				pkg.Keywords = append([]string(nil), pf.Keywords...)
				return fmt.Sprintf("%d keyword(s)", len(pf.Keywords))
			},
		},
		{
			ExtKey: "people", PFKey: "people",
			ToPF: func(_ bool) string {
				// People always merge via projectfile.MergePeople; force has no
				// effect because the merge is non-destructive by design
				// (existing-wins on conflicts). Skip entries with no identifying
				// fields — empty placeholders like `{}` in maintainers arrays
				// have no canonical name to dedup against and would otherwise
				// be appended to pf.People on every cycle.
				var incoming []projectfile.Person
				if author := ParsePerson(pkg.Author); !author.IsEmpty() {
					incoming = append(incoming, npmPersonToPF(author, "author"))
				}
				for _, c := range ParsePersonList(pkg.Contributors) {
					if c.IsEmpty() {
						continue
					}
					incoming = append(incoming, npmPersonToPF(c, "contributor"))
				}
				for _, m := range ParsePersonList(pkg.Maintainers) {
					if m.IsEmpty() {
						continue
					}
					incoming = append(incoming, npmPersonToPF(m, "maintainer"))
				}
				if len(incoming) == 0 {
					return ""
				}
				before := len(pf.People)
				merged, conflicts := projectfile.MergePeople(pf.People, incoming)
				pf.People = merged
				core.RecordPersonConflicts(pf, conflicts)
				added := len(pf.People) - before
				if added == 0 && len(conflicts) == 0 {
					return ""
				}
				return fmt.Sprintf("%d person(s) merged", len(incoming))
			},
			FromPF: func(_ bool) string {
				// Skip pf entries whose npm projection has no name/email/url:
				// emitting empty `{}` placeholders into package.json is pure
				// noise, and historical bugs (see ParsePerson) could leave
				// stale empty maintainers in pf.People that would otherwise
				// surface here. After building the new view, compare to what
				// is already on pkg — when they match (same author, same
				// maintainer roster) the closure stays silent instead of
				// re-asserting the same value on every sync.
				var parts []string

				if authors := filterByRole(pf.People, "author"); len(authors) > 0 {
					newAuthor := pfPersonToNPM(authors[0])
					if !newAuthor.IsEmpty() {
						existing := ParsePerson(pkg.Author)
						if existing != newAuthor {
							pkg.Author = newAuthor
							parts = append(parts, personDisplayName(authors[0]))
						}
					}
				}

				if contribs := filterByRole(pf.People, "contributor"); len(contribs) > 0 {
					var entries []Person
					for _, c := range contribs {
						if person := pfPersonToNPM(c); !person.IsEmpty() {
							entries = append(entries, person)
						}
					}
					if len(entries) > 0 && !equalPersonList(ParsePersonList(pkg.Contributors), entries) {
						out := make([]any, len(entries))
						for i, e := range entries {
							out[i] = e
						}
						pkg.Contributors = out
						parts = append(parts, fmt.Sprintf("%d contributor(s)", len(entries)))
					}
				}

				if maintainers := filterByRole(pf.People, "maintainer"); len(maintainers) > 0 {
					var entries []Person
					for _, m := range maintainers {
						if person := pfPersonToNPM(m); !person.IsEmpty() {
							entries = append(entries, person)
						}
					}
					if len(entries) > 0 && !equalPersonList(ParsePersonList(pkg.Maintainers), entries) {
						out := make([]any, len(entries))
						for i, e := range entries {
							out[i] = e
						}
						pkg.Maintainers = out
						parts = append(parts, fmt.Sprintf("%d maintainer(s)", len(entries)))
					}
				}

				if len(parts) == 0 {
					return ""
				}
				return strings.Join(parts, ", ")
			},
		},
		{
			ExtKey: "engines", PFKey: "requirements.runtime",
			ToPF: func(force bool) string {
				if len(pkg.Engines) == 0 {
					return ""
				}
				ensureRequirements(pf)
				if equalStringMap(pf.Requirements.Runtime, pkg.Engines) {
					return ""
				}
				if len(pf.Requirements.Runtime) > 0 && !force {
					return ""
				}
				pf.Requirements.Runtime = copyStringMap(pkg.Engines)
				return fmt.Sprintf("%v", pkg.Engines)
			},
			FromPF: func(force bool) string {
				if pf.Requirements == nil || len(pf.Requirements.Runtime) == 0 {
					return ""
				}
				if equalStringMap(pkg.Engines, pf.Requirements.Runtime) {
					return ""
				}
				if len(pkg.Engines) > 0 && !force {
					return ""
				}
				pkg.Engines = copyStringMap(pf.Requirements.Runtime)
				return fmt.Sprintf("%v", pkg.Engines)
			},
		},
		mapPlatformList(pf, "os", nsOperatingSystem,
			func() []string { return pkg.OS }, func(v []string) { pkg.OS = v },
			npmOSToPF, osListToNPM),
		mapPlatformList(pf, "cpu", nsArchitecture,
			func() []string { return pkg.CPU }, func(v []string) { pkg.CPU = v },
			npmCPUToPF, archListToNPM),
		{
			ExtKey: "funding", PFKey: "[org.projectfile.funding]",
			ToPF: func(force bool) string {
				if pkg.Funding == nil {
					return ""
				}
				entries := ParseFunding(pkg.Funding)
				if len(entries) == 0 {
					return ""
				}
				ext, _ := pfmodel.GetFundingExtension(pf)
				if ext == nil {
					ext = &pfmodel.FundingExtension{}
				}
				if hasExtensionFunding(ext) && !force {
					return ""
				}
				ext.Custom = npmFundingToCustom(entries)
				projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, fundingExtToMap(ext))
				return fmt.Sprintf("%d entr(y/ies)", len(entries))
			},
			FromPF: func(force bool) string {
				ext, _ := pfmodel.GetFundingExtension(pf)
				if ext == nil {
					return ""
				}
				urls := extensionFundingURLs(ext)
				if len(urls) == 0 {
					return ""
				}
				if pkg.Funding != nil && !force {
					return ""
				}
				pkg.Funding = urlsToFundingEntries(urls)
				return fmt.Sprintf("%d entr(y/ies)", len(urls))
			},
		},
	}
}

// --- shared helpers (ported from sync_npm) ---

func ensureLicense(pf *projectfile.Document) {
	if pf.License == nil {
		pf.License = &projectfile.License{}
	}
}

// Platform targeting lives in these two extension fields per spec §4.8a, not in
// requirements: an operating-system or architecture set is a build and
// distribution concern rather than a runtime requirement.
const (
	nsOperatingSystem = "org.projectfile.operating-system"
	nsArchitecture    = "org.projectfile.architecture"
)

// mapPlatformList is the shared closure pair behind npm `os` and `cpu`. Both
// carry a flat string list and both target a §4.8a extension field, so the only
// per-field parts are the package.json accessor pair and the vocabulary
// translators.
func mapPlatformList(
	pf *projectfile.Document,
	extKey, ns string,
	npmGet func() []string, npmSet func([]string),
	toPF func([]string) []string, toNPM func([]string) []string,
) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: extKey, PFKey: "[" + ns + "]",
		ToPF: func(force bool) string {
			list := toPF(npmGet())
			if len(list) == 0 {
				return ""
			}
			current := extensionStringList(pf, ns)
			if equalStringSlice(current, list) {
				return ""
			}
			if len(current) > 0 && !force {
				return ""
			}
			projectfile.SetExtension(pf, ns, list)
			return fmt.Sprintf("%v", list)
		},
		FromPF: func(force bool) string {
			current := extensionStringList(pf, ns)
			if len(current) == 0 {
				return ""
			}
			npmList := toNPM(current)
			if equalStringSlice(npmGet(), npmList) {
				return ""
			}
			if len(npmGet()) > 0 && !force {
				return ""
			}
			npmSet(npmList)
			return fmt.Sprintf("%v", npmList)
		},
	}
}

// extensionStringList reads a §4.8a platform list. Core keeps no typed shape
// for these namespaces, so a document parsed from disk yields []any while one
// SetExtension just wrote yields []string — both spellings must read back.
func extensionStringList(pf *projectfile.Document, ns string) []string {
	v, ok := projectfile.LookupExtension(pf, ns)
	if !ok {
		return nil
	}
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func ensureRequirements(pf *projectfile.Document) {
	if pf.Requirements == nil {
		pf.Requirements = &projectfile.Requirements{}
	}
}

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

func pfPersonToNPM(p projectfile.Person) Person {
	return Person{
		Name:  personDisplayName(p),
		Email: p.Email,
		URL:   p.URL,
	}
}

func npmPersonToPF(n Person, role string) projectfile.Person {
	p := projectfile.Person{
		Email: n.Email,
		URL:   n.URL,
		Roles: []string{role},
	}
	parts := strings.SplitN(n.Name, " ", 2)
	if len(parts) == 2 {
		p.GivenNames = parts[0]
		p.FamilyNames = parts[1]
	} else if len(parts) == 1 && parts[0] != "" {
		p.DisplayName = parts[0]
	}
	return p
}

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

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// equalStringMap treats nil and empty maps as equal so that a closure mutating
// a freshly-zeroed target does not report a phantom change. Used by every
// FromPF/ToPF mapper that copies a map[string]string between sides.
func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// equalStringSlice compares two []string by length and order. Like
// equalStringMap, nil and zero-length are equivalent.
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

// equalPersonList compares two []npm.Person by field — used to detect
// no-op rewrites of pkg.Author / pkg.Maintainers in FromPF before they
// trigger a phantom change report.
func equalPersonList(a, b []Person) bool {
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

func osListToNPM(osList []string) []string {
	if len(osList) == 0 {
		return nil
	}
	var out []string
	for _, o := range osList {
		switch o {
		case "darwin":
			out = append(out, "darwin")
		case "linux":
			out = append(out, "linux")
		case "windows":
			out = append(out, "win32")
		default:
			out = append(out, o)
		}
	}
	return out
}

func npmOSToPF(osList []string) []string {
	if len(osList) == 0 {
		return nil
	}
	var out []string
	for _, o := range osList {
		if strings.HasPrefix(o, "!") {
			continue
		}
		switch o {
		case "win32":
			out = append(out, "windows")
		default:
			out = append(out, o)
		}
	}
	return out
}

func archListToNPM(archList []string) []string {
	if len(archList) == 0 {
		return nil
	}
	var out []string
	for _, a := range archList {
		switch a {
		case "amd64":
			out = append(out, "x64")
		case "arm64":
			out = append(out, "arm64")
		case "386":
			out = append(out, "ia32")
		default:
			out = append(out, a)
		}
	}
	return out
}

func npmCPUToPF(cpuList []string) []string {
	if len(cpuList) == 0 {
		return nil
	}
	var out []string
	for _, c := range cpuList {
		if strings.HasPrefix(c, "!") {
			continue
		}
		switch c {
		case "x64":
			out = append(out, "amd64")
		case "arm64":
			out = append(out, "arm64")
		case "ia32":
			out = append(out, "386")
		default:
			out = append(out, c)
		}
	}
	return out
}

// npmNameToPFName converts an npm package name into the pf (namespace, name)
// pair. It accepts the *existing* pf namespace so the last-segment prefix
// convention is invertible:
//
//	pf {namespace: me.dbuho.persona, name: frontend}  <->  npm persona-frontend
//
// Without the pfNamespace hint the previous implementation could not strip the
// "persona-" prefix on the way back in, and a follow-up FromPF re-prepended it
// — producing the "persona-persona-..." runaway under repeat syncs.
//
// Scoped packages (`@scope/name`) map to a dedicated `npm.<scope>` namespace,
// because there is no other way to recover the original scope from a flat pf
// namespace tree.
func npmNameToPFName(npmName, pfNamespace string) (namespace, name string) {
	if strings.HasPrefix(npmName, "@") {
		parts := strings.SplitN(strings.TrimPrefix(npmName, "@"), "/", 2)
		if len(parts) == 2 {
			return "npm." + parts[0], parts[1]
		}
		return "", strings.TrimPrefix(npmName, "@")
	}
	if prefix := namespaceLastSegment(pfNamespace); prefix != "" {
		if rest, ok := strings.CutPrefix(npmName, prefix+"-"); ok {
			return pfNamespace, rest
		}
	}
	return "", npmName
}

// pfNameToNPM is the inverse of npmNameToPFName. For the npm-flavoured case
// it emits `@scope/name`; for a flat pf namespace it prepends the last
// segment as a dash-delimited prefix — unless the pf name already starts
// with that prefix, which guards against the historical doubling defect.
func pfNameToNPM(namespace, name string) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(namespace, "npm.") {
		return "@" + strings.TrimPrefix(namespace, "npm.") + "/" + name
	}
	prefix := namespaceLastSegment(namespace)
	if prefix == "" {
		return name
	}
	if strings.HasPrefix(name, prefix+"-") {
		// already prefixed in pf — don't re-prepend.
		return name
	}
	return prefix + "-" + name
}

func namespaceLastSegment(ns string) string {
	if ns == "" {
		return ""
	}
	parts := strings.Split(ns, ".")
	return parts[len(parts)-1]
}

func hasExtensionFunding(ext *pfmodel.FundingExtension) bool {
	if ext == nil {
		return false
	}
	return len(ext.GitHub) > 0 || len(ext.Custom) > 0 || ext.Patreon != "" ||
		ext.KoFi != "" || ext.Liberapay != "" || ext.Tidelift != "" ||
		ext.CommunityBridge != "" || ext.IssueHunt != "" || ext.OpenCollective != "" ||
		ext.LFXCrowdfunding != "" || ext.Polar != "" || ext.BuyMeACoffee != "" ||
		ext.ThanksDev != ""
}

func npmFundingToCustom(entries []FundingEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.URL != "" {
			out = append(out, e.URL)
		}
	}
	return out
}

func extensionFundingURLs(ext *pfmodel.FundingExtension) []string {
	out := make([]string, 0, len(ext.Custom))
	out = append(out, ext.Custom...)
	return out
}

func urlsToFundingEntries(urls []string) any {
	if len(urls) == 0 {
		return nil
	}
	if len(urls) == 1 {
		return FundingEntry{Type: "custom", URL: urls[0]}
	}
	out := make([]FundingEntry, len(urls))
	for i, u := range urls {
		out[i] = FundingEntry{Type: "custom", URL: u}
	}
	return out
}

func fundingExtToMap(ext *pfmodel.FundingExtension) map[string]any {
	m := map[string]any{}
	if len(ext.GitHub) > 0 {
		out := make([]any, len(ext.GitHub))
		for i, v := range ext.GitHub {
			out[i] = v
		}
		m["github"] = out
	}
	for k, v := range map[string]string{
		"patreon": ext.Patreon, "ko-fi": ext.KoFi, "liberapay": ext.Liberapay,
		"tidelift": ext.Tidelift, "community-bridge": ext.CommunityBridge,
		"issuehunt": ext.IssueHunt, "open-collective": ext.OpenCollective,
		"lfx-crowdfunding": ext.LFXCrowdfunding, "polar": ext.Polar,
		"buy-me-a-coffee": ext.BuyMeACoffee, "thanks-dev": ext.ThanksDev,
		"path": ext.Path, "entity-type": ext.EntityType, "entity-role": ext.EntityRole,
	} {
		if v != "" {
			m[k] = v
		}
	}
	if len(ext.Custom) > 0 {
		out := make([]any, len(ext.Custom))
		for i, v := range ext.Custom {
			out[i] = v
		}
		m["custom"] = out
	}
	if len(ext.Channels) > 0 {
		m["channels"] = channelsToAny(ext.Channels)
	}
	if len(ext.Plans) > 0 {
		m["plans"] = plansToAny(ext.Plans)
	}
	if len(ext.History) > 0 {
		m["history"] = historyToAny(ext.History)
	}
	return m
}

func channelsToAny(in []pfmodel.FundingChannel) []any {
	out := make([]any, len(in))
	for i, c := range in {
		out[i] = map[string]any{"guid": c.GUID, "type": c.Type, "address": c.Address, npmKeyDescription: c.Description}
	}
	return out
}

func plansToAny(in []pfmodel.FundingPlan) []any {
	out := make([]any, len(in))
	for i, p := range in {
		chs := make([]any, len(p.Channels))
		for j, c := range p.Channels {
			chs[j] = c
		}
		out[i] = map[string]any{"guid": p.GUID, "status": p.Status, "name": p.Name, npmKeyDescription: p.Description, "amount": p.Amount, "currency": p.Currency, "frequency": p.Frequency, "channels": chs}
	}
	return out
}

func historyToAny(in []pfmodel.FundingHistory) []any {
	out := make([]any, len(in))
	for i, h := range in {
		out[i] = map[string]any{"year": h.Year, "income": h.Income, "expenses": h.Expenses, "taxes": h.Taxes, "currency": h.Currency, npmKeyDescription: h.Description}
	}
	return out
}
