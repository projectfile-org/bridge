// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package registries infers package-registry landing pages from the detected
// stack plus identity.{namespace,name}. Each rule maps a stack tag to a URL
// template; when the tag appears in pf.Stack the rule writes a [[links]]
// entry of type "package-registry".
//
// Today's coverage:
//
//	stack tag      → registry            URL template
//	---------------|--------------------|---------------------------------------
//	node, npm      | npmjs.com           https://www.npmjs.com/package/<name>
//	python         | pypi.org            https://pypi.org/project/<name>/
//	php            | packagist.org       https://packagist.org/packages/<vendor>/<name>
//	rust           | crates.io           https://crates.io/crates/<name>
//
// Vendor extraction:
//
//   - Composer needs a "vendor/name" — pf carries the vendor as the LAST
//     segment of identity.namespace ("com.acme" → "acme/<name>"). Cargo and
//     PyPI ignore vendor.
//   - npm scoped packages ("@scope/name") arrive as namespace = "npm.scope"
//     and name = "name"; the URL is constructed accordingly. Bare names
//     (no scope) map to the un-prefixed /package/<name> URL.
package registries

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const tagNPM = "npm"

// Change matches forges.Change one-for-one so the engine can fold both
// passes into a uniform []derive.Change.
type Change struct {
	FieldPath string
	NewValue  string
	Source    string
	Label     *projectfile.LocalizedString
}

// builder returns the registry URL for the given identity, or "" when the
// rule cannot construct one (missing name, missing vendor for composer).
type builder func(namespace, name string) string

// rules is keyed by stack tag — first match wins. Order is precedence:
// "npm" before "node" so a project declaring both lands on the more
// specific registry rule.
var rules = []struct {
	tag    string
	source string
	label  string
	url    builder
}{
	{tag: tagNPM, source: "registry:npm", label: tagNPM, url: npmURL},
	{tag: "node", source: "registry:npm", label: tagNPM, url: npmURL},
	{tag: "python", source: "registry:pypi", label: "PyPI", url: pypiURL},
	{tag: "php", source: "registry:packagist", label: "Packagist", url: packagistURL},
	{tag: "rust", source: "registry:crates", label: "crates.io", url: cratesURL},
}

// Derive walks the rules and emits at most one [[links]] entry per matching
// rule. Multiple ecosystems can apply (a polyglot project may publish to
// both npm and PyPI); each gets its own (type=package-registry, name=<x>)
// slot. Today every match produces type = "package-registry" so the engine
// stores them as a single field path collision — fine while only one
// registry rule typically matches per project.
func Derive(pf *projectfile.Document) []Change {
	if pf == nil {
		return nil
	}
	if pf.Identity.Name == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []Change
	for _, tag := range pf.Stack {
		for _, r := range rules {
			if r.tag != tag {
				continue
			}
			value := r.url(pf.Identity.Namespace, pf.Identity.Name)
			if value == "" || seen[r.source] {
				continue
			}
			seen[r.source] = true
			out = append(out, Change{
				// One link slot per type — the engine's writeField path
				// replaces in place if the type already exists, so re-runs
				// are idempotent.
				FieldPath: "links[type=package-registry]",
				NewValue:  value,
				Source:    r.source,
				// Localized "Packages on {registry}": Bare for a single-language
				// project, a Langs map when i18n.languages is declared.
				Label: pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounPackages), r.label),
			})
		}
	}
	return out
}

// npmURL handles both scoped (@org/pkg → npm.org / pkg) and unscoped
// (lone pkg) variants. Scoped packages keep their @ in the URL.
func npmURL(namespace, name string) string {
	if name == "" {
		return ""
	}
	if scope := scopeFromNamespace(namespace); scope != "" {
		return "https://www.npmjs.com/package/@" + scope + "/" + name
	}
	return "https://www.npmjs.com/package/" + name
}

func pypiURL(_, name string) string {
	if name == "" {
		return ""
	}
	return "https://pypi.org/project/" + name + "/"
}

// packagistURL needs a vendor/name pair. Pf stores the vendor as the last
// segment of identity.namespace ("composer.symfony" → "symfony" or
// "com.acme" → "acme"); a namespace without a vendor segment short-circuits
// to "" so the engine doesn't emit a malformed link.
func packagistURL(namespace, name string) string {
	if name == "" {
		return ""
	}
	vendor := lastSegment(namespace)
	if vendor == "" {
		return ""
	}
	return "https://packagist.org/packages/" + vendor + "/" + name
}

func cratesURL(_, name string) string {
	if name == "" {
		return ""
	}
	return "https://crates.io/crates/" + name
}

// scopeFromNamespace pulls the npm-scope out of an "npm.<scope>" namespace.
// Returns "" for non-npm namespaces so the unscoped branch fires.
func scopeFromNamespace(namespace string) string {
	if !strings.HasPrefix(namespace, "npm.") {
		return ""
	}
	return strings.TrimPrefix(namespace, "npm.")
}

// lastSegment is the right-most "." segment of namespace. "com.acme" → "acme".
// Empty namespace short-circuits to "" so callers can detect "no vendor".
func lastSegment(namespace string) string {
	if namespace == "" {
		return ""
	}
	idx := strings.LastIndex(namespace, ".")
	if idx < 0 {
		return namespace
	}
	return namespace[idx+1:]
}
