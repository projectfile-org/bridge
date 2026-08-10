// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Registry roles. The vocabulary is OPEN — an unlisted role is carried through
// untouched — but these two change how a README introduces the registry, so they
// are named rather than spelled at each use.
//
// Role is also what makes a registry ADDRESSABLE. A bare `{}` projection admits
// no trailing field (core allows only `keys`/`values` after one), so the only way
// to reach `.ref` across a map is the selector form `{k=v}` — and a set every
// entry belongs to needs a key every entry carries. RegistryRolePrimary is that
// key's default, filled in by the composition rather than typed by the author.
const (
	RegistryRolePrimary  = "primary"
	RegistryRoleFallback = "fallback"
)

// Registry is ONE place a project's container images land. It is a peer of the
// forge plane, never derived from it: AWS ECR has no repositories at all, and
// Codeberg has an issue tracker but no build minutes, so "where the source
// lives" and "where the images live" cannot be read off one another.
//
// Every field except Host is optional, and a bare `host:` is a complete entry —
// the default Ref template then reproduces the single-registry behaviour the
// fleet had before this namespace existed.
type Registry struct {
	// Slug is the map key the registry was declared under — its identity in the
	// document, and what a `${…registries.<slug>.…}` address names. Carried on
	// the value so an entry stays self-describing once the map is flattened.
	Slug string

	// Host is the registry authority, `ghcr.io` or `kiota.ch`. It may carry a
	// port; the ref grammar treats the whole thing as one opaque label.
	Host string

	// Owner is the account every image of this registry nests under. Declare it
	// only where the registry FORCES one account on the whole fleet (GHCR, ECR).
	// Left unset, `${registry.owner}` in a template falls back to the project's
	// own image namespace, and the default template omits the segment entirely.
	Owner string

	// Ref is the path template for this registry, in the two grammars that
	// already exist: `${…}` field addresses (spec §3.8) and `{AXIS}` matrix
	// placeholders. Unset means the default template applies.
	//
	// A template recomposes the parts; it never RELOCATES an axis. Where the
	// series sits — a path segment, a tag suffix, a hyphen — is the project's
	// decision, taken once in `ci.image` and inherited by every registry.
	Ref string

	// Role labels an entry a README introduces differently. RegistryRoleFallback
	// is the one the generators know: the registry a reader uses when the others
	// cannot serve them.
	Role string

	// Priority orders the registries, higher first. A map of named keys carries
	// no order of its own, so without this the fan-out would rank them by
	// spelling — which puts `ecr` ahead of a preferred `ghcr` for no reason a
	// reader could infer.
	Priority int
}

// GetRegistries parses `org.projectfile.registries` into the declared entries,
// sorted by priority descending with the slug as tiebreak — the same order the
// resolver's `{}` fan-out uses, so an inventory list and the rendered pull lines
// agree. Returns nil when the namespace is absent, which is a valid state: the
// ~130 projects that declare nothing fall back to a single registry.
//
// An entry with no `host` is skipped. A registry with no authority addresses
// nothing, and composing a ref from it would print a pull line beginning with a
// slash.
func GetRegistries(doc *projectfile.Document) ([]Registry, error) {
	m, present, err := lookupNS(doc, RegistriesExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	slugs := make([]string, 0, len(m))
	for slug := range m {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	out := make([]Registry, 0, len(slugs))
	for _, slug := range slugs {
		em, ok := m[slug].(map[string]any)
		if !ok {
			continue
		}
		host := strVal(em, "host")
		if host == "" {
			continue
		}
		out = append(out, Registry{
			Slug:     slug,
			Host:     host,
			Owner:    strVal(em, "owner"),
			Ref:      strVal(em, "ref"),
			Role:     strVal(em, "role"),
			Priority: intVal(em, keyPriority),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return ByPriorityDesc(RankOf(out[i].Priority), RankOf(out[j].Priority)) < 0
	})
	return out, nil
}
