// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// extNamespace is the reserved-DNS key under pf.Extensions where composer-only
// fields without a native projectfile slot are parked (type, minimum-stability,
// prefer-stable, abandoned). Naming follows the spec's reverse-DNS extension
// convention — see projectfile/spec v1 §"Extensions".
const extNamespace = "org.packagist.composer"

// buildMappers returns one FieldMapper per synced composer.json field. The
// shape mirrors drivers/npm and drivers/pyproject: per-field closure pair,
// force-aware, returns a short display string when a change happens.
//
// Composer's `require` map is mapped only for its platform keys (`php`,
// `hhvm`, `ext-*`, `lib-*`, `composer-*`) ↔ pf.Requirements.Runtime. Library
// keys (vendor/package) have no projectfile slot: they round-trip untouched
// as part of the composer document, which is where a dependency set belongs.
func buildMappers(pkg *Document, pf *projectfile.Document) core.MapperList {
	return core.MapperList{
		mapName(pkg, pf),
		mapVersion(pkg, pf),
		mapDescription(pkg, pf),
		mapTime(pkg, pf),
		mapLicense(pkg, pf),
		mapHomepage(pkg, pf),
		mapKeywords(pkg, pf),
		mapPeople(pkg, pf),
		mapFunding(pkg, pf),
		mapSupportIssues(pkg, pf),
		mapSupportSource(pkg, pf),
		mapSupportDocs(pkg, pf),
		mapSupportChat(pkg, pf),
		mapSupportForum(pkg, pf),
		mapRequirementsRuntime(pkg, pf),
		mapExtField(
			pkg, pf, "type",
			func() any {
				if pkg.Type == "" {
					return nil
				}
				return pkg.Type
			},
			func(v any) {
				if s, ok := v.(string); ok {
					pkg.Type = s
				}
			},
		),
		mapExtField(
			pkg, pf, "minimum-stability",
			func() any {
				if pkg.MinimumStability == "" {
					return nil
				}
				return pkg.MinimumStability
			},
			func(v any) {
				if s, ok := v.(string); ok {
					pkg.MinimumStability = s
				}
			},
		),
		mapExtField(
			pkg, pf, "prefer-stable",
			func() any { return pkg.PreferStable },
			func(v any) { pkg.PreferStable = v },
		),
		mapExtField(
			pkg, pf, "abandoned",
			func() any { return pkg.Abandoned },
			func(v any) { pkg.Abandoned = v },
		),
	}
}

// --- identity mappers ---

func mapName(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "name", PFKey: "identity.name",
		ToPF: func(force bool) string {
			if pkg.Name == "" {
				return ""
			}
			ns, name := composerNameToPFName(pkg.Name)
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
			name := pfNameToComposer(pf.Identity.Namespace, pf.Identity.Name)
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
	}
}

func mapVersion(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
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
	}
}

func mapDescription(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "description", PFKey: "identity.summary",
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
			if s == "" || pkg.Description == s {
				return ""
			}
			if pkg.Description != "" && !force {
				return ""
			}
			pkg.Description = s
			return core.Trunc(s)
		},
	}
}

func mapTime(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "time", PFKey: "identity.released",
		ToPF: func(force bool) string {
			if pkg.Time == "" || pf.Identity.Released == pkg.Time {
				return ""
			}
			if pf.Identity.Released != "" && !force {
				return ""
			}
			pf.Identity.Released = pkg.Time
			return pkg.Time
		},
		FromPF: func(force bool) string {
			if pf.Identity.Released == "" || pkg.Time == pf.Identity.Released {
				return ""
			}
			if pkg.Time != "" && !force {
				return ""
			}
			pkg.Time = pf.Identity.Released
			return pf.Identity.Released
		},
	}
}

// --- license ---

func mapLicense(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "license", PFKey: "license.spdx",
		ToPF: func(force bool) string {
			spdx := LicenseString(pkg.License)
			if spdx == "" {
				return ""
			}
			existing := ""
			if pf.License != nil {
				existing = pf.License.Spdx
			}
			if existing == spdx {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			ensureLicense(pf)
			pf.License.Spdx = spdx
			return spdx
		},
		FromPF: func(force bool) string {
			if pf.License == nil || pf.License.Spdx == "" {
				return ""
			}
			cur := LicenseString(pkg.License)
			if cur == pf.License.Spdx {
				return ""
			}
			if pkg.License != nil && !force {
				return ""
			}
			pkg.License = ParseLicense(pf.License.Spdx)
			return pf.License.Spdx
		},
	}
}

// --- top-level URLs ---

func mapHomepage(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return mapSupportURL(
		"homepage", "links[type=homepage]",
		func() string { return pkg.Homepage },
		func(v string) { pkg.Homepage = v },
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkHomepage) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkHomepage, v, true) },
	)
}

// mapSupportIssues maps composer's support.issues ↔ links[type=bugs].
func mapSupportIssues(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return mapSupportURL(
		"support.issues", "links[type=bugs]",
		func() string {
			if pkg.Support == nil {
				return ""
			}
			return pkg.Support.Issues
		},
		func(v string) { ensureSupport(pkg); pkg.Support.Issues = v },
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkBugs) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkBugs, v, true) },
	)
}

// mapSupportSource maps composer's support.source ↔ repositories[role=origin].url.
// Repository.Type defaults to "git" when set from composer (composer doesn't
// model the VCS type explicitly).
func mapSupportSource(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "support.source", PFKey: "repositories[role=origin]",
		ToPF: func(force bool) string {
			src := ""
			if pkg.Support != nil {
				src = pkg.Support.Source
			}
			if src == "" {
				return ""
			}
			primary := pfmodel.PrimaryRepository(pf)
			if primary != nil && primary.URL != "" && !force {
				return ""
			}
			if primary != nil && primary.URL == src {
				return ""
			}
			slot := pfmodel.EnsurePrimaryRepository(pf)
			slot.URL = src
			if slot.Type == "" {
				slot.Type = "git"
			}
			return core.Trunc(src)
		},
		FromPF: func(force bool) string {
			primary := pfmodel.PrimaryRepository(pf)
			if primary == nil || primary.URL == "" {
				return ""
			}
			url := primary.URL
			existing := ""
			if pkg.Support != nil {
				existing = pkg.Support.Source
			}
			if existing == url {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			ensureSupport(pkg)
			pkg.Support.Source = url
			return core.Trunc(url)
		},
	}
}

func mapSupportDocs(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	// Tag-gated like pyproject's Documentation slot: an include can union "a
	// piece of documentation" onto every project; only the main-documentation
	// entry is THE project's docs.
	return mapSupportURL(
		"support.docs", "links[type=documentation,tag=main-documentation]",
		func() string {
			if pkg.Support == nil {
				return ""
			}
			return pkg.Support.Docs
		},
		func(v string) { ensureSupport(pkg); pkg.Support.Docs = v },
		func() string { return pfmodel.MainDocumentationURL(pf) },
		func(v string) { pfmodel.SetMainDocumentationURL(pf, v, true) },
	)
}

func mapSupportChat(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return mapSupportURL(
		"support.chat", "links[type=chat]",
		func() string {
			if pkg.Support == nil {
				return ""
			}
			return pkg.Support.Chat
		},
		func(v string) { ensureSupport(pkg); pkg.Support.Chat = v },
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkChat) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkChat, v, true) },
	)
}

func mapSupportForum(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return mapSupportURL(
		"support.forum", "links[type=forum]",
		func() string {
			if pkg.Support == nil {
				return ""
			}
			return pkg.Support.Forum
		},
		func(v string) { ensureSupport(pkg); pkg.Support.Forum = v },
		func() string { return pfmodel.LinkURL(pf, projectfile.LinkForum) },
		func(v string) { pfmodel.SetLink(pf, projectfile.LinkForum, v, true) },
	)
}

// mapSupportURL factors out the per-URL closure pair for the simple cases
// (composer's support.X ↔ a single pf URL string). The repository case is
// kept separate because it has the Repository.Type initialisation rule.
func mapSupportURL(
	extKey, pfKey string,
	supportGet func() string,
	supportSet func(string),
	pfGet func() string,
	pfSet func(string),
) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: extKey, PFKey: pfKey,
		ToPF: func(force bool) string {
			v := supportGet()
			if v == "" {
				return ""
			}
			existing := pfGet()
			if existing == v {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			pfSet(v)
			return core.Trunc(v)
		},
		FromPF: func(force bool) string {
			v := pfGet()
			if v == "" {
				return ""
			}
			existing := supportGet()
			if existing == v {
				return ""
			}
			if existing != "" && !force {
				return ""
			}
			supportSet(v)
			return core.Trunc(v)
		},
	}
}

// --- keywords ---

func mapKeywords(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "keywords", PFKey: "keywords",
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
	}
}

// --- people ---

func mapPeople(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "authors", PFKey: "people",
		ToPF: func(_ bool) string {
			var incoming []projectfile.Person
			for _, a := range pkg.Authors {
				if a.Name == "" && a.Email == "" && a.Homepage == "" {
					continue
				}
				incoming = append(incoming, composerPersonToPF(a))
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
			entries := make([]Person, 0, len(authors))
			for _, p := range authors {
				cp := pfPersonToComposer(p)
				if cp.Name == "" && cp.Email == "" && cp.Homepage == "" {
					continue
				}
				entries = append(entries, cp)
			}
			if len(entries) == 0 {
				return ""
			}
			if equalAuthorList(pkg.Authors, entries) {
				return ""
			}
			pkg.Authors = entries
			return fmt.Sprintf("%d author(s)", len(entries))
		},
	}
}

// --- funding ---

func mapFunding(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "funding", PFKey: "[org.projectfile.funding]",
		ToPF: func(force bool) string {
			if len(pkg.Funding) == 0 {
				return ""
			}
			urls := make([]string, 0, len(pkg.Funding))
			for _, f := range pkg.Funding {
				if f.URL != "" {
					urls = append(urls, f.URL)
				}
			}
			if len(urls) == 0 {
				return ""
			}
			ext, _ := pfmodel.GetFundingExtension(pf)
			if ext == nil {
				ext = &pfmodel.FundingExtension{}
			}
			if hasComposerExtFunding(ext) && !force {
				return ""
			}
			ext.Custom = append(ext.Custom, urls...)
			projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, composerFundingExtToMap(ext))
			return fmt.Sprintf("%d entr(y/ies)", len(urls))
		},
		FromPF: func(force bool) string {
			ext, _ := pfmodel.GetFundingExtension(pf)
			if ext == nil {
				return ""
			}
			urls := composerExtFundingURLs(ext)
			if len(urls) == 0 {
				return ""
			}
			entries := make([]FundingEntry, len(urls))
			for i, u := range urls {
				entries[i] = FundingEntry{Type: "custom", URL: u}
			}
			if equalFundingComposer(pkg.Funding, entries) {
				return ""
			}
			if len(pkg.Funding) > 0 && !force {
				return ""
			}
			pkg.Funding = entries
			return fmt.Sprintf("%d entr(y/ies)", len(entries))
		},
	}
}

// --- requirements ---

// mapRequirementsRuntime handles the platform-package subset of composer's
// `require` map (`php`, `hhvm`, `ext-*`, `lib-*`, `composer-*`). Library
// packages (vendor/package) are left alone — they belong to composer, and
// pkg.Require round-trips them untouched.
func mapRequirementsRuntime(pkg *Document, pf *projectfile.Document) core.FieldMapper {
	return core.FieldMapper{
		ExtKey: "require (platform)", PFKey: "requirements.runtime",
		ToPF: func(force bool) string {
			platform := platformRequires(pkg.Require)
			if len(platform) == 0 {
				return ""
			}
			ensureRequirements(pf)
			existingPlatform := filterPlatformKeys(pf.Requirements.Runtime)
			if equalStringMap(existingPlatform, platform) {
				return ""
			}
			if pf.Requirements.Runtime == nil {
				pf.Requirements.Runtime = map[string]string{}
			}
			anyChange := false
			for k, v := range platform {
				if cur, ok := pf.Requirements.Runtime[k]; ok && cur == v {
					continue
				}
				if _, ok := pf.Requirements.Runtime[k]; ok && !force {
					continue
				}
				pf.Requirements.Runtime[k] = v
				anyChange = true
			}
			if !anyChange {
				return ""
			}
			return fmt.Sprintf("%d requirement(s)", len(platform))
		},
		FromPF: func(force bool) string {
			if pf.Requirements == nil {
				return ""
			}
			platform := filterPlatformKeys(pf.Requirements.Runtime)
			if len(platform) == 0 {
				return ""
			}
			if pkg.Require == nil {
				pkg.Require = map[string]string{}
			}
			anyChange := false
			for k, v := range platform {
				if cur, ok := pkg.Require[k]; ok && cur == v {
					continue
				}
				if _, ok := pkg.Require[k]; ok && !force {
					continue
				}
				pkg.Require[k] = v
				anyChange = true
			}
			if !anyChange {
				return ""
			}
			return fmt.Sprintf("%d requirement(s)", len(platform))
		},
	}
}

// --- extension namespace passthrough ---

// mapExtField is the generic passthrough for composer-only fields that have
// no native projectfile slot — they round-trip through
// pf.Extensions["org.packagist.composer"][<key>]. ToPF stashes the composer
// value onto pf; FromPF reads pf and writes back onto composer.
func mapExtField(
	pkg *Document,
	pf *projectfile.Document,
	key string,
	get func() any,
	set func(any),
) core.FieldMapper {
	_ = pkg // present in closure via get/set
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

// --- ensure-* helpers ---

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

func ensureSupport(pkg *Document) {
	if pkg.Support == nil {
		pkg.Support = &Support{}
	}
}

// --- name conversion ---

// composerNameToPFName splits a composer "vendor/package" into the pf
// (namespace, name) pair. composer names are always slash-separated, so the
// inverse pfNameToComposer is unambiguous as long as pf.namespace keeps the
// "composer." prefix.
func composerNameToPFName(composerName string) (namespace, name string) {
	parts := strings.SplitN(composerName, "/", 2)
	if len(parts) == 2 {
		return "composer." + parts[0], parts[1]
	}
	return "", composerName
}

// pfNameToComposer is the inverse. Two cases:
//
//	composer.<vendor> + <name>   →  "<vendor>/<name>"           (canonical)
//	<other ns> + <name>          →  "<last-segment>/<name>"     (fallback)
//
// The fallback keeps composer.json valid for projectfiles that originated
// from another syncer and don't carry the `composer.` namespace marker.
func pfNameToComposer(namespace, name string) string {
	if name == "" {
		return ""
	}
	if strings.HasPrefix(namespace, "composer.") {
		return strings.TrimPrefix(namespace, "composer.") + "/" + name
	}
	if vendor := namespaceLastSegment(namespace); vendor != "" {
		return vendor + "/" + name
	}
	return ""
}

func namespaceLastSegment(ns string) string {
	if ns == "" {
		return ""
	}
	parts := strings.Split(ns, ".")
	return parts[len(parts)-1]
}

// --- people conversion ---

func composerPersonToPF(c Person) projectfile.Person {
	p := projectfile.Person{
		Email: c.Email,
		URL:   c.Homepage,
		Roles: []string{"author"},
	}
	parts := strings.SplitN(strings.TrimSpace(c.Name), " ", 2)
	switch {
	case len(parts) == 2:
		p.GivenNames = parts[0]
		p.FamilyNames = parts[1]
	case len(parts) == 1 && parts[0] != "":
		p.DisplayName = parts[0]
	}
	return p
}

func pfPersonToComposer(p projectfile.Person) Person {
	return Person{
		Name:     personDisplayName(p),
		Email:    p.Email,
		Homepage: p.URL,
	}
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

func equalAuthorList(a []Person, b []Person) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		// Role is not compared — we don't synchronise it (always "author"
		// on FromPF), so role differences from the original composer.json
		// are intentional no-ops.
		if a[i].Name != b[i].Name || a[i].Email != b[i].Email || a[i].Homepage != b[i].Homepage {
			return false
		}
	}
	return true
}

// --- funding helpers ---

func hasComposerExtFunding(ext *pfmodel.FundingExtension) bool {
	return ext != nil && (len(ext.GitHub) > 0 || len(ext.Custom) > 0 || ext.Patreon != "" ||
		ext.OpenCollective != "" || ext.KoFi != "")
}

func composerExtFundingURLs(ext *pfmodel.FundingExtension) []string {
	if ext == nil {
		return nil
	}
	return ext.Custom
}

func composerFundingExtToMap(ext *pfmodel.FundingExtension) map[string]any {
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

func equalFundingComposer(a, b []FundingEntry) bool {
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

// --- require / require-dev helpers ---

// isComposerPlatform reports whether key names a composer "platform package":
// php, hhvm, ext-*, lib-*, composer-*. These do not represent installable
// packages but runtime/build environment requirements.
//
// Reference: https://getcomposer.org/doc/01-basic-usage.md#platform-packages
func isComposerPlatform(key string) bool {
	switch key {
	case "php", "hhvm":
		return true
	}
	return strings.HasPrefix(key, "php-") ||
		strings.HasPrefix(key, "hhvm-") ||
		strings.HasPrefix(key, "ext-") ||
		strings.HasPrefix(key, "lib-") ||
		strings.HasPrefix(key, "composer-")
}

func platformRequires(req map[string]string) map[string]string {
	if len(req) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range req {
		if isComposerPlatform(k) {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterPlatformKeys(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range m {
		if isComposerPlatform(k) {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// --- string-set / string-map equality (nil and empty treated as equal) ---

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

// --- extension namespace helpers (mirror cff/pyproject) ---

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

// --- generic any helpers (used by mapExtField) ---

func isEmptyAny(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return false // explicit booleans (incl. false) are meaningful
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

// deepEqualAny string-formats both sides with stable map-key ordering, then
// compares the strings. Cheaper than reflect.DeepEqual for our shallow
// shapes and lets us normalise []string vs []any (which differ between TOML
// and JSON unmarshal paths).
func deepEqualAny(a, b any) bool {
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

func formatAny(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return "s:" + x
	case bool:
		if x {
			return "b:true"
		}
		return "b:false"
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
