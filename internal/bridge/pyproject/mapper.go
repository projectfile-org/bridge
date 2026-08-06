// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// extNamespace is the reserved-DNS key under pf.Extensions where Python-specific
// fields without a native projectfile slot are parked (scripts, entry-points,
// optional-dependencies, dynamic, readme, raw classifiers). The spec uses
// reverse-DNS extension namespaces — see projectfile/spec v1 §"Extensions".
const (
	extNamespace      = "org.python.pep621"
	pyprojectKeywords = "keywords"
)

// pfKeyFunding is the projectfile slot label for funding URLs. Duplicated
// across the mapper as both the PFKey and inside the urls-label lookup; one
// constant keeps the vocabulary consistent.
const pfKeyFunding = "funding"

// buildMappers returns one FieldMapper per synced PEP 621 field. The shape
// mirrors drivers/npm and drivers/cff: per-field closure pair, force-aware,
// returns a short display string when a change happens.
//
// Pre-computed at the top: the `dynamic` set. PEP 621 callers list field
// names there to declare "this is computed by the build backend, don't
// trust the literal value." We treat dynamic fields as no-ops in both
// directions — pyproject isn't authoritative for them, and writing a
// concrete value back would defeat the dynamic mechanism.
func buildMappers(py *Document, pf *projectfile.Document) core.MapperList {
	dyn := map[string]bool{}
	for _, k := range py.Project.Dynamic {
		dyn[strings.ToLower(k)] = true
	}

	return core.MapperList{
		mapName(py, pf, dyn),
		mapVersion(py, pf, dyn),
		mapDescription(py, pf, dyn),
		mapRequiresPython(py, pf, dyn),
		mapLicense(py, pf, dyn),
		mapLicenseFiles(py, pf, dyn),
		mapAuthors(py, pf),
		mapMaintainers(py, pf),
		mapKeywords(py, pf),
		mapClassifiers(py, pf),
		mapURLHomepage(py, pf),
		mapURLRepository(py, pf),
		mapURLDocumentation(py, pf),
		mapURLChangelog(py, pf),
		mapURLIssues(py, pf),
		mapURLFunding(py, pf),
		// Pure passthroughs to/from pf.Extensions["org.python.pep621"]. They
		// preserve Python-only fields so a project regenerated from pf alone
		// still carries scripts, entry-points, extras, dynamic, readme.
		mapExtensionField(
			py, pf, "readme",
			func() any { return py.Project.ReadMe },
			func(v any) { py.Project.ReadMe = v },
		),
		mapExtensionField(
			py, pf, "scripts",
			func() any { return stringMapToAny(py.Project.Scripts) },
			func(v any) { py.Project.Scripts = anyToStringMap(v) },
		),
		mapExtensionField(
			py, pf, "gui-scripts",
			func() any { return stringMapToAny(py.Project.GUIScripts) },
			func(v any) { py.Project.GUIScripts = anyToStringMap(v) },
		),
		mapExtensionField(
			py, pf, "entry-points",
			func() any { return entryPointsToAny(py.Project.EntryPoints) },
			func(v any) { py.Project.EntryPoints = anyToEntryPoints(v) },
		),
		mapExtensionField(
			py, pf, "optional-dependencies",
			func() any { return optionalDepsToAny(py.Project.OptionalDependencies) },
			func(v any) { py.Project.OptionalDependencies = anyToOptionalDeps(v) },
		),
		mapExtensionField(
			py, pf, "dynamic",
			func() any { return stringsToAny(py.Project.Dynamic) },
			func(v any) { py.Project.Dynamic = anyToStrings(v) },
		),
	}
}

// --- per-field mapper builders ---

func mapName(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "name", PFKey: "identity.name",
		ToPF: func(force bool) string {
			if dyn["name"] || py.Project.Name == "" {
				return ""
			}
			// PEP 503: pyproject names are case-insensitive and dash/underscore
			// equivalent. Compare on the normalised form so e.g. "My_Package"
			// in pyproject and "my-package" in pf don't trigger a phantom write.
			incoming := py.Project.Name
			if normalizePyPIName(pf.Identity.Name) == normalizePyPIName(incoming) {
				return ""
			}
			if pf.Identity.Name != "" && !force {
				return ""
			}
			pf.Identity.Name = incoming
			return incoming
		},
		FromPF: func(force bool) string {
			if dyn["name"] || pf.Identity.Name == "" {
				return ""
			}
			if normalizePyPIName(py.Project.Name) == normalizePyPIName(pf.Identity.Name) {
				return ""
			}
			if py.Project.Name != "" && !force {
				return ""
			}
			py.Project.Name = pf.Identity.Name
			return pf.Identity.Name
		},
	}
}

func mapVersion(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "version", PFKey: "identity.version",
		ToPF: func(force bool) string {
			if dyn["version"] || py.Project.Version == "" || pf.Identity.Version == py.Project.Version {
				return ""
			}
			if pf.Identity.Version != "" && !force {
				return ""
			}
			pf.Identity.Version = py.Project.Version
			return py.Project.Version
		},
		FromPF: func(force bool) string {
			if dyn["version"] || pf.Identity.Version == "" || py.Project.Version == pf.Identity.Version {
				return ""
			}
			if py.Project.Version != "" && !force {
				return ""
			}
			py.Project.Version = pf.Identity.Version
			return pf.Identity.Version
		},
	}
}

func mapDescription(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "description", PFKey: "identity.summary",
		ToPF: func(force bool) string {
			if dyn["description"] || py.Project.Description == "" {
				return ""
			}
			existing := projectfile.ExtractLocalizedString(pf.Identity.Summary)
			if existing == py.Project.Description {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			projectfile.SetLocalizedEN(&pf.Identity.Summary, py.Project.Description)
			return core.Trunc(py.Project.Description)
		},
		FromPF: func(force bool) string {
			if dyn["description"] {
				return ""
			}
			s := projectfile.ExtractLocalizedString(pf.Identity.Summary)
			if s == "" || py.Project.Description == s {
				return ""
			}
			if py.Project.Description != "" && !force {
				return ""
			}
			py.Project.Description = s
			return core.Trunc(s)
		},
	}
}

func mapRequiresPython(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "requires-python", PFKey: "requirements.runtime.python",
		ToPF: func(force bool) string {
			if dyn["requires-python"] || py.Project.RequiresPython == "" {
				return ""
			}
			ensureRequirements(pf)
			existing := pf.Requirements.Runtime["python"]
			if existing == py.Project.RequiresPython {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			if pf.Requirements.Runtime == nil {
				pf.Requirements.Runtime = map[string]string{}
			}
			pf.Requirements.Runtime["python"] = py.Project.RequiresPython
			return py.Project.RequiresPython
		},
		FromPF: func(force bool) string {
			if dyn["requires-python"] || pf.Requirements == nil {
				return ""
			}
			val := pf.Requirements.Runtime["python"]
			if val == "" || py.Project.RequiresPython == val {
				return ""
			}
			if py.Project.RequiresPython != "" && !force {
				return ""
			}
			py.Project.RequiresPython = val
			return val
		},
	}
}

func mapLicense(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "license", PFKey: "license.spdx",
		ToPF: func(force bool) string {
			if dyn["license"] {
				return ""
			}
			spdx, file := parseLicenseValue(py.Project.License)
			if spdx == "" && file == "" {
				return ""
			}
			// Spdx wins for the dedicated spdx slot; file (legacy
			// {file: ...} shape) flows into license.file.
			ensureLicense(pf)
			changed := false
			if spdx != "" && pf.License.Spdx != spdx && (pf.License.Spdx == "" || force) {
				pf.License.Spdx = spdx
				changed = true
			}
			if file != "" && pf.License.File == nil && (pf.License.File == nil || force) {
				pf.License.File = file
				changed = true
			}
			if !changed {
				return ""
			}
			if spdx != "" {
				return spdx
			}
			return core.Trunc(file)
		},
		FromPF: func(force bool) string {
			if dyn["license"] {
				return ""
			}
			if pf.License == nil {
				return ""
			}
			// Prefer the PEP 639 string form on write — that's the modern
			// canonical shape and what new tooling emits.
			if pf.License.Spdx != "" {
				cur, _ := parseLicenseValue(py.Project.License)
				if cur == pf.License.Spdx {
					return ""
				}
				if cur != "" && !force {
					return ""
				}
				py.Project.License = pf.License.Spdx
				return pf.License.Spdx
			}
			// Spdx empty but a license.file was set — emit legacy
			// {file: ...} table. license-files (PEP 639) handles the
			// list form separately via mapLicenseFiles.
			if pf.License.File != nil {
				if s, ok := pf.License.File.(string); ok && s != "" {
					existing, _ := licenseValueAsTable(py.Project.License)
					if existing["file"] == s {
						return ""
					}
					if py.Project.License != nil && !force {
						return ""
					}
					py.Project.License = map[string]any{"file": s}
					return core.Trunc(s)
				}
			}
			return ""
		},
	}
}

func mapLicenseFiles(py *Document, pf *projectfile.Document, dyn map[string]bool) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "license-files", PFKey: "license.file",
		ToPF: func(force bool) string {
			if dyn["license-files"] || len(py.Project.LicenseFiles) == 0 {
				return ""
			}
			ensureLicense(pf)
			incoming := append([]string(nil), py.Project.LicenseFiles...)
			if equalLicenseFile(pf.License.File, incoming) {
				return ""
			}
			if pf.License.File != nil && !force {
				return ""
			}
			// Single entry → string; multiple → []string (per spec note).
			if len(incoming) == 1 {
				pf.License.File = incoming[0]
			} else {
				pf.License.File = incoming
			}
			return fmt.Sprintf("%d file(s)", len(incoming))
		},
		FromPF: func(force bool) string {
			if dyn["license-files"] || pf.License == nil || pf.License.File == nil {
				return ""
			}
			files := licenseFileAsList(pf.License.File)
			if len(files) == 0 {
				return ""
			}
			// Only emit license-files for list-shaped values; a single string
			// flows through mapLicense as {file: ...} legacy shape.
			if len(files) == 1 {
				return ""
			}
			if equalStringSlice(py.Project.LicenseFiles, files) {
				return ""
			}
			if len(py.Project.LicenseFiles) > 0 && !force {
				return ""
			}
			py.Project.LicenseFiles = files
			return fmt.Sprintf("%d file(s)", len(files))
		},
	}
}

func mapAuthors(py *Document, pf *projectfile.Document) core.FieldMapper {
	return mapPeopleRole(
		py, pf, "author",
		func() []Person { return py.Project.Authors },
		func(v []Person) { py.Project.Authors = v },
		"authors",
	)
}

func mapMaintainers(py *Document, pf *projectfile.Document) core.FieldMapper {
	return mapPeopleRole(
		py, pf, "maintainer",
		func() []Person { return py.Project.Maintainers },
		func(v []Person) { py.Project.Maintainers = v },
		"maintainers",
	)
}

// mapPeopleRole builds the author/maintainer mapper. People always merge via
// projectfile.MergePeople — `force` has no effect because the merge is
// non-destructive by design (existing-wins on conflicts).
func mapPeopleRole(
	_ *Document,
	pf *projectfile.Document,
	role string,
	get func() []Person,
	set func([]Person),
	extKey string,
) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: extKey, PFKey: "people." + role,
		ToPF: func(_ bool) string {
			incoming := pyPeopleToPF(get(), role)
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
			return fmt.Sprintf("%d %s(s) merged", len(incoming), role)
		},
		FromPF: func(_ bool) string {
			roles := filterByRole(pf.People, role)
			if len(roles) == 0 {
				return ""
			}
			newPeople := pfPeopleToPy(roles)
			if equalPyPersons(get(), newPeople) {
				return ""
			}
			set(newPeople)
			return fmt.Sprintf("%d %s(s)", len(newPeople), role)
		},
	}
}

func mapKeywords(py *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: pyprojectKeywords, PFKey: pyprojectKeywords,
		// Bare keywords only — namespaced ones (e.g. "topic:x") live in pf
		// alongside but are emitted into pyproject via the classifiers mapper.
		ToPF: func(force bool) string {
			if len(py.Project.Keywords) == 0 {
				return ""
			}
			bare := extractBareKeywords(pf.Keywords)
			if equalStringSet(bare, py.Project.Keywords) {
				return ""
			}
			if len(bare) > 0 && !force {
				return ""
			}
			// Merge: keep existing namespaced keywords, replace bare set.
			namespaced := extractNamespacedKeywords(pf.Keywords)
			merged := append([]string{}, py.Project.Keywords...)
			merged = append(merged, namespaced...)
			pf.Keywords = merged
			return fmt.Sprintf("%d keyword(s)", len(py.Project.Keywords))
		},
		FromPF: func(force bool) string {
			bare := extractBareKeywords(pf.Keywords)
			if len(bare) == 0 {
				return ""
			}
			if equalStringSet(py.Project.Keywords, bare) {
				return ""
			}
			if len(py.Project.Keywords) > 0 && !force {
				return ""
			}
			py.Project.Keywords = bare
			return fmt.Sprintf("%d keyword(s)", len(bare))
		},
	}
}

func mapClassifiers(py *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "classifiers", PFKey: "keywords (namespaced)",
		// Strategy: keep the *original* classifier strings in
		// pf.Extensions["org.python.pep621"]["classifiers"] so round-trip is
		// lossless. The derived "topic:x"-style namespaced keywords in
		// pf.Keywords are an additional projection for projectfile-native
		// consumers; they are NOT the source of truth on the way back.
		ToPF: func(force bool) string {
			if len(py.Project.Classifiers) == 0 {
				return ""
			}
			stored := extStringList(pf, "classifiers")
			if equalStringSlice(stored, py.Project.Classifiers) {
				// Already mirrored — assume namespaced keywords are in sync too.
				return ""
			}
			if len(stored) > 0 && !force {
				return ""
			}
			setExtAny(pf, "classifiers", stringsToAny(py.Project.Classifiers))
			// Project derived namespaced keywords onto pf.Keywords (additive,
			// dedup against existing).
			derived := classifiersToKeywords(py.Project.Classifiers)
			if len(derived) > 0 {
				pf.Keywords = mergeUniqueStrings(pf.Keywords, derived)
			}
			return fmt.Sprintf("%d classifier(s)", len(py.Project.Classifiers))
		},
		FromPF: func(force bool) string {
			stored := extStringList(pf, "classifiers")
			if len(stored) == 0 {
				return ""
			}
			if equalStringSlice(py.Project.Classifiers, stored) {
				return ""
			}
			if len(py.Project.Classifiers) > 0 && !force {
				return ""
			}
			py.Project.Classifiers = append([]string(nil), stored...)
			return fmt.Sprintf("%d classifier(s)", len(stored))
		},
	}
}

func mapURLHomepage(py *Document, pf *projectfile.Document) core.FieldMapper {
	return mapURLString(
		py, pf,
		"Homepage", []string{"homepage"}, "links[type=homepage]",
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkHomepage) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkHomepage, v, true) },
	)
}

func mapURLRepository(py *Document, pf *projectfile.Document) core.FieldMapper {
	// PyPI labels for the repository slot: "Repository" (most common in
	// modern projects) or "Source" (alternative). Accept either on read,
	// emit "Repository" canonically.
	return mapURLString(
		py, pf,
		"Repository", []string{"repository", "source"}, "repositories[role=origin]",
		func() string {
			primary := pfmodel.PrimaryRepository(pf)
			if primary == nil {
				return ""
			}
			return primary.URL
		},
		func(v string) {
			slot := pfmodel.EnsurePrimaryRepository(pf)
			slot.URL = v
			if slot.Type == "" {
				slot.Type = "git"
			}
		},
	)
}

func mapURLDocumentation(py *Document, pf *projectfile.Document) core.FieldMapper {
	return mapURLString(
		py, pf,
		"Documentation", []string{"documentation"}, "links[type=documentation]",
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkDocumentation) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkDocumentation, v, true) },
	)
}

func mapURLChangelog(py *Document, pf *projectfile.Document) core.FieldMapper {
	return mapURLString(
		py, pf,
		"Changelog", []string{"changelog"}, "links[type=changelog]",
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkChangelog) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkChangelog, v, true) },
	)
}

func mapURLIssues(py *Document, pf *projectfile.Document) core.FieldMapper {
	// Three labels in the wild — "Issues" (most common today), "BugTracker"
	// (older PyPI convention), "Tracker" (some projects).
	return mapURLString(
		py, pf,
		"Issues", []string{"issues", "bugtracker", "bug tracker", "tracker"}, "links[type=bugs]",
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkBugs) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkBugs, v, true) },
	)
}

func mapURLFunding(py *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "urls.Funding", PFKey: "[org.projectfile.funding]",
		ToPF: func(force bool) string {
			url := findURLAnyLabel(py.Project.URLs, []string{pfKeyFunding, "donate"})
			if url == "" {
				return ""
			}
			ext, _ := pfmodel.GetFundingExtension(pf)
			if ext == nil {
				ext = &pfmodel.FundingExtension{}
			}
			for _, u := range ext.Custom {
				if u == url {
					return ""
				}
			}
			if hasPyFunding(ext) && !force {
				return ""
			}
			ext.Custom = append(ext.Custom, url)
			projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, pyFundingExtToMap(ext))
			return core.Trunc(url)
		},
		FromPF: func(force bool) string {
			ext, _ := pfmodel.GetFundingExtension(pf)
			if ext == nil || len(ext.Custom) == 0 {
				return ""
			}
			url := ext.Custom[0]
			if url == "" {
				return ""
			}
			existingKey, existing, found := findURLEntry(py.Project.URLs, []string{pfKeyFunding, "donate"})
			if found && existing == url {
				return ""
			}
			if found && !force {
				return ""
			}
			if py.Project.URLs == nil {
				py.Project.URLs = map[string]string{}
			}
			label := "Funding"
			if existingKey != "" {
				label = existingKey
			}
			py.Project.URLs[label] = url
			return core.Trunc(url)
		},
	}
}

func hasPyFunding(ext *pfmodel.FundingExtension) bool {
	return ext != nil && (len(ext.GitHub) > 0 || len(ext.Custom) > 0 || ext.Patreon != "" ||
		ext.OpenCollective != "" || ext.KoFi != "")
}

func pyFundingExtToMap(ext *pfmodel.FundingExtension) map[string]any {
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
	return m
}

// mapExtensionField is the generic passthrough for Python-only fields that
// have no native projectfile slot — they round-trip through
// pf.Extensions["org.python.pep621"][<key>]. ToPF stashes pyproject value
// onto pf; FromPF reads pf and writes back onto pyproject.
//
// `get` returns the pyproject-side value as a generic `any` (string, map,
// list, etc.); `set` writes a parsed `any` back into the typed pyproject
// document. Idempotency is via reflect.DeepEqual on the `any` values.
func mapExtensionField(
	_ *Document,
	pf *projectfile.Document,
	key string,
	get func() any,
	set func(any),
) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: key, PFKey: "ext." + extNamespace + "." + key,
		ToPF: func(force bool) string {
			val := get()
			if isEmptyAny(val) {
				return ""
			}
			existing, exists := extGet(pf, key)
			if exists && deepEqualAny(existing, val) {
				return ""
			}
			if exists && !force {
				return ""
			}
			setExtAny(pf, key, val)
			return core.Trunc(formatAny(val))
		},
		FromPF: func(force bool) string {
			existing, exists := extGet(pf, key)
			if !exists || isEmptyAny(existing) {
				return ""
			}
			cur := get()
			if deepEqualAny(cur, existing) {
				return ""
			}
			if !isEmptyAny(cur) && !force {
				return ""
			}
			set(existing)
			return core.Trunc(formatAny(existing))
		},
	}
}

// mapURLString factors out the per-URL closure pair for fields that map
// pyproject's `urls.<label>` onto a single projectfile string slot.
// `aliases` lists the lowercase labels accepted on read (e.g. "issues",
// "bugtracker"); `canonical` is the label written back.
func mapURLString(
	py *Document, _ *projectfile.Document,
	canonical string,
	aliases []string,
	pfKey string,
	getPF func() string,
	setPF func(string),
) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "urls." + canonical, PFKey: pfKey,
		ToPF: func(force bool) string {
			url := findURLAnyLabel(py.Project.URLs, aliases)
			if url == "" {
				return ""
			}
			existing := getPF()
			if existing == url {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			setPF(url)
			return core.Trunc(url)
		},
		FromPF: func(force bool) string {
			url := getPF()
			if url == "" {
				return ""
			}
			existingKey, existing, found := findURLEntry(py.Project.URLs, aliases)
			if found && existing == url {
				return ""
			}
			if found && !force {
				return ""
			}
			if py.Project.URLs == nil {
				py.Project.URLs = map[string]string{}
			}
			label := canonical
			if existingKey != "" {
				label = existingKey
			}
			py.Project.URLs[label] = url
			return core.Trunc(url)
		},
	}
}

// --- ensure-* helpers (duplicated from npm driver, matching established
// per-driver convention of self-contained helpers) ---

func ensureLicense(pf *projectfile.Document) {
	if pf.License == nil {
		pf.License = &projectfile.License{}
	}
}

func ensureRequirements(pf *projectfile.Document) {
	if pf.Requirements == nil {
		pf.Requirements = &projectfile.Requirements{}
	}
}

// --- people mapping helpers ---

func pyPeopleToPF(in []Person, role string) []projectfile.Person {
	if len(in) == 0 {
		return nil
	}
	out := make([]projectfile.Person, 0, len(in))
	for _, p := range in {
		if p.Name == "" && p.Email == "" {
			continue
		}
		pf := projectfile.Person{
			Email: p.Email,
			Roles: []string{role},
		}
		parts := strings.SplitN(strings.TrimSpace(p.Name), " ", 2)
		switch {
		case len(parts) == 2:
			pf.GivenNames = parts[0]
			pf.FamilyNames = parts[1]
		case len(parts) == 1 && parts[0] != "":
			pf.DisplayName = parts[0]
		}
		out = append(out, pf)
	}
	return out
}

func pfPeopleToPy(in []projectfile.Person) []Person {
	out := make([]Person, 0, len(in))
	for _, p := range in {
		py := Person{
			Name:  personDisplayName(p),
			Email: p.Email,
		}
		if py.Name == "" && py.Email == "" {
			continue
		}
		out = append(out, py)
	}
	return out
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

func equalPyPersons(a, b []Person) bool {
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

// --- URL lookup helpers ---

// findURLAnyLabel is a convenience for the read path — returns the value
// associated with any of `aliases` (case-insensitive), or "" if none match.
func findURLAnyLabel(urls map[string]string, aliases []string) string {
	_, v, _ := findURLEntry(urls, aliases)
	return v
}

// findURLEntry returns the original key, the value, and a boolean indicating
// whether any of the aliases (lowercase) matched a key in `urls`. The original
// key is returned so writers can preserve user-supplied casing on round-trip.
func findURLEntry(urls map[string]string, aliases []string) (origKey, value string, found bool) {
	if urls == nil {
		return "", "", false
	}
	aliasSet := map[string]bool{}
	for _, a := range aliases {
		aliasSet[strings.ToLower(a)] = true
	}
	for k, v := range urls {
		if aliasSet[strings.ToLower(k)] {
			return k, v, true
		}
	}
	return "", "", false
}

// --- name normalisation ---

// normalizePyPIName implements PEP 503: lowercase + collapse runs of [-_.]
// into a single dash. Used purely for equality comparison — the typed
// pyproject.Name field still stores whatever shape the user wrote.
func normalizePyPIName(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	dashRun := false
	for _, r := range strings.ToLower(name) {
		if r == '-' || r == '_' || r == '.' {
			if !dashRun {
				b.WriteByte('-')
				dashRun = true
			}
			continue
		}
		dashRun = false
		b.WriteRune(r)
	}
	return b.String()
}

// --- classifier ↔ keyword conversion ---

// classifiersToKeywords derives namespaced projectfile keywords from PEP 621
// trove classifiers. Format: "Topic :: X :: Y" -> "topic:x-y". Only used as
// an additive projection — the original classifier strings are stashed in
// pf.Extensions for the inverse direction, so cosmetic loss here is harmless.
func classifiersToKeywords(classifiers []string) []string {
	out := make([]string, 0, len(classifiers))
	for _, c := range classifiers {
		parts := strings.Split(c, "::")
		filtered := parts[:0]
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) < 2 {
			continue
		}
		head := slugify(filtered[0])
		tailParts := make([]string, 0, len(filtered)-1)
		for _, p := range filtered[1:] {
			tailParts = append(tailParts, slugify(p))
		}
		tail := strings.Join(tailParts, "-")
		// Collapse runs of dashes produced by trove values like "4 - Beta".
		for strings.Contains(tail, "--") {
			tail = strings.ReplaceAll(tail, "--", "-")
		}
		tail = strings.Trim(tail, "-")
		if head == "" || tail == "" {
			continue
		}
		out = append(out, head+":"+tail)
	}
	return out
}

// slugify lowercases, replaces any run of non-alphanumeric chars with a
// single dash, and trims surrounding dashes. Used for trove classifier
// segments where we want "Software Development :: Libraries" to become
// "software-development-libraries" instead of "software development-libraries".
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	dashRun := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dashRun = false
		default:
			if !dashRun {
				b.WriteByte('-')
				dashRun = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// --- bare/namespaced keyword filters ---

func extractBareKeywords(keywords []string) []string {
	var out []string
	for _, kw := range keywords {
		if !strings.Contains(kw, ":") {
			out = append(out, kw)
		}
	}
	return out
}

func extractNamespacedKeywords(keywords []string) []string {
	var out []string
	for _, kw := range keywords {
		if strings.Contains(kw, ":") {
			out = append(out, kw)
		}
	}
	return out
}

// --- license value parsing ---

// parseLicenseValue inspects the heterogeneous PEP 621 license shape and
// returns the spdx string and the file path (whichever applies; both empty
// when the value is absent or unrecognised).
//
// Shapes:
//
//	"MIT"                                 (PEP 639 string)
//	{text = "MIT License"}                (legacy)
//	{file = "LICENSE"}                    (legacy)
func parseLicenseValue(v any) (spdx, file string) {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if looksLikeSPDX(s) {
			return s, ""
		}
		return "", ""
	case map[string]any:
		if s, ok := x["text"].(string); ok {
			t := strings.TrimSpace(s)
			if looksLikeSPDX(t) {
				return t, ""
			}
			return "", ""
		}
		if s, ok := x["file"].(string); ok {
			return "", strings.TrimSpace(s)
		}
	}
	return "", ""
}

// licenseValueAsTable returns the inline-table view of a heterogeneous
// license value, used by ToPF callers checking equivalence before writing.
// A bare string view is returned as `{text: ...}` for comparison purposes.
func licenseValueAsTable(v any) (map[string]any, bool) {
	switch x := v.(type) {
	case nil:
		return nil, false
	case string:
		return map[string]any{"text": x}, true
	case map[string]any:
		return x, true
	}
	return nil, false
}

// looksLikeSPDX is a coarse classifier: SPDX identifiers and expressions
// use ASCII letters, digits, `.`, `-`, `+`, and operators (`AND`, `OR`,
// `WITH`). A space-containing string like "MIT License" is NOT an SPDX-ID;
// we treat it as opaque license text and drop it (the caller would have
// stashed the original under license-files or the extension namespace).
func looksLikeSPDX(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '+' || r == ' ':
			// space allowed for operator-separated SPDX expressions
			// ("Apache-2.0 OR MIT") — but we treat free-prose text
			// containing lowercase words as non-SPDX below.
		default:
			return false
		}
	}
	// Heuristic: if the string contains any lowercase letter run NOT among
	// the SPDX operators (and, or, with), call it prose. SPDX identifiers
	// follow `[A-Z][A-Za-z0-9-]*` for the most part; pure-uppercase plus
	// known operators is a good signal.
	upper := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return -1
		}
		return r
	}, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "and", ""), "or", ""), "with", ""))
	// If after stripping known operators the result is the same length as
	// stripping ALL lowercase, that means there were no surprise lowercase
	// runs — likely an SPDX identifier/expression.
	stripped := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return -1
		}
		return r
	}, s)
	return len(upper) == len(stripped)
}

// --- license.file shape helpers ---

func licenseFileAsList(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func equalLicenseFile(v any, files []string) bool {
	got := licenseFileAsList(v)
	return equalStringSlice(got, files)
}

// --- extension namespace helpers (mirror cff/citExt* pattern) ---

func extMap(pf *projectfile.Document) map[string]any {
	if pf.Extensions == nil {
		return nil
	}
	m, _ := pf.Extensions[extNamespace].(map[string]any)
	return m
}

func extGet(pf *projectfile.Document, key string) (any, bool) {
	m := extMap(pf)
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

func extStringList(pf *projectfile.Document, key string) []string {
	v, ok := extGet(pf, key)
	if !ok {
		return nil
	}
	return anyToStrings(v)
}

func setExtAny(pf *projectfile.Document, key string, val any) {
	if pf.Extensions == nil {
		pf.Extensions = map[string]any{}
	}
	m, _ := pf.Extensions[extNamespace].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	m[key] = val
	pf.Extensions[extNamespace] = m
}

// --- any-shape conversion helpers (used by mapExtensionField) ---

func stringMapToAny(m map[string]string) any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func anyToStringMap(v any) map[string]string {
	switch x := v.(type) {
	case nil:
		return nil
	case map[string]string:
		if len(x) == 0 {
			return nil
		}
		return cloneMapSS(x)
	case map[string]any:
		if len(x) == 0 {
			return nil
		}
		out := make(map[string]string, len(x))
		for k, vv := range x {
			if s, ok := vv.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

func cloneMapSS(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func entryPointsToAny(in map[string]map[string]string) any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for group, m := range in {
		out[group] = stringMapToAny(m)
	}
	return out
}

func anyToEntryPoints(v any) map[string]map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(m))
	for group, sub := range m {
		if inner := anyToStringMap(sub); inner != nil {
			out[group] = inner
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func optionalDepsToAny(in map[string][]string) any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = stringsToAny(v)
	}
	return out
}

func anyToOptionalDeps(v any) map[string][]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(m))
	for group, sub := range m {
		if list := anyToStrings(sub); list != nil {
			out[group] = list
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stringsToAny(in []string) any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func anyToStrings(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// --- equality helpers ---

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

// equalStringSet treats two slices as multisets — order doesn't matter, but
// membership does. Used for keyword and dependency lists where on-disk order
// differs from map iteration order.
func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

func mergeUniqueStrings(existing, incoming []string) []string {
	seen := make(map[string]bool, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, s := range existing {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, s := range incoming {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// --- generic any helpers used by the passthrough mapper ---

func isEmptyAny(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	case map[string]string:
		return len(x) == 0
	}
	return false
}

// deepEqualAny normalises numeric-keyed string slices and map types before
// comparing. The bookkeeping is needed because TOML unmarshal produces
// `[]any` of strings while our internal converters produce both shapes
// depending on path.
func deepEqualAny(a, b any) bool {
	// Normalise both sides into a canonical shape, then string-compare a
	// formatted representation. Cheaper than reflect.DeepEqual for our
	// shallow shapes and avoids the reflect import.
	return formatAny(normaliseAny(a)) == formatAny(normaliseAny(b))
}

func normaliseAny(v any) any {
	switch x := v.(type) {
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(x))
		for k, s := range x {
			out[k] = s
		}
		return out
	}
	return v
}

// formatAny renders a value with sorted map keys so the formatted output is
// stable across runs — required because Go map iteration order is random.
func formatAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return "s:" + x
	case []any:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = formatAny(item)
		}
		return "l[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+formatAny(x[k]))
		}
		return "m{" + strings.Join(parts, ",") + "}"
	}
	return fmt.Sprintf("%v", v)
}
