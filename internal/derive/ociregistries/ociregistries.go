// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package ociregistries composes the pull reference of every container registry
// a project publishes to, from the per-registry `ref` template.
//
// Not to be confused with the sibling `derive/registries`, which infers PACKAGE
// index pages (npm, PyPI, crates.io) from the detected stack. This package is
// about the OCI plane: where a project's IMAGES land.
//
// What we are trying to do: let one project declare one image path and reach
// several registries with it. GHCR and ECR nest every image of an account side
// by side, kiota.ch nests them freely, and the same build feeds all three — so
// the path grammar is a property of the REGISTRY, and the image path itself is a
// property of the PROJECT. Neither can be computed from the other, and neither
// should ever be typed twice.
//
// # Why the composition happens here and not in the interpolator
//
// A template addresses `${registry.host}` and `${registry.owner}`, which are
// ENTRY-scoped: they mean "the entry this template is written on". The `${…}`
// grammar addresses the document, and has no notion of a current entry, so those
// two are substituted here — and ONLY those two. Everything else in a template
// (`${image.basename}`, `${image.tag}`, `{ANY_AXIS}`) is document-scoped or
// matrix-scoped and is left verbatim for the layers that already resolve it.
// That is what keeps this package from growing into a second template engine.
package ociregistries

import (
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/fieldpath"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Entry-scoped template variables. They look like `${…}` field addresses but no
// document answers them: a map entry cannot address itself through a grammar
// that starts at the document root.
const (
	varHost  = "${registry.host}"
	varOwner = "${registry.owner}"
)

// refKey is the composed value's home on each entry — the same key the template
// was read from, so a document that declared a template and one that declared
// none present an identical shape to every reader downstream.
const refKey = "ref"

// Refs returns the registry subtree with every entry's `ref` composed into a
// concrete reference, keyed by slug. Returns nil when the project publishes no
// image at all, which is the signal for the caller to write nothing.
//
// The map is the merged view: entries the document declares, or — when it
// declares no registries namespace at all — ONE entry synthesized from the
// legacy `org.projectfile.readme.registry` scalar. That synthesis is what lets
// the ~130 projects which never heard of this namespace keep rendering the exact
// pull line they rendered before it existed, with no edit and no `when:` clause
// in the shared fragment.
func Refs(pf *projectfile.Document) map[string]any {
	if pf == nil {
		return nil
	}
	declared, err := pfmodel.GetRegistries(pf)
	if err != nil {
		genlog.Warn("derive: registries namespace unreadable — refs not composed", "error", err.Error())
		return nil
	}
	if len(declared) == 0 {
		declared = legacyRegistry(pf)
	}
	if len(declared) == 0 {
		return nil
	}
	out := make(map[string]any, len(declared))
	for _, r := range declared {
		ref := composeRef(r)
		genlog.Decision("registry_ref", ref, r.Slug, "priority="+priorityLabel(r))
		out[r.Slug] = entry(r, ref)
	}
	return out
}

// entry renders one registry back as document data.
//
// `role` is always written, defaulting to primary, and that is deliberate: a
// bare `{}` projection admits no trailing field, so a fragment reaches `.ref`
// across the map only through the selector form `{role=…}`. An entry with no
// role would be addressable by nothing and its pull line would never render.
// Owner and priority stay absent when unset, because nothing selects on them.
func entry(r pfmodel.Registry, ref string) map[string]any {
	m := map[string]any{"host": r.Host, refKey: ref, "role": role(r)}
	if r.Owner != "" {
		m["owner"] = r.Owner
	}
	if r.Priority != 0 {
		m["priority"] = r.Priority
	}
	return m
}

// role is the entry's declared role, else primary.
func role(r pfmodel.Registry) string {
	if r.Role != "" {
		return r.Role
	}
	return pfmodel.RegistryRolePrimary
}

// composeRef substitutes the two entry-scoped variables into the entry's
// template, applying the default template when it declares none.
//
// An entry that declares no owner still answers `${registry.owner}`, with the
// project's own image namespace — so a template written for GHCR keeps working
// on a registry that forces no account, instead of collapsing to a double slash.
func composeRef(r pfmodel.Registry) string {
	owner := r.Owner
	if owner == "" {
		owner = ref(fieldpath.AddrImageNamespace)
	}
	return strings.NewReplacer(varHost, r.Host, varOwner, owner).Replace(template(r))
}

// template is the entry's own `ref`, else the default.
//
// The default omits the owner segment entirely for an entry that declares no
// owner, and that asymmetry is the whole point: it reproduces today's single
// registry behaviour byte for byte (`kiota.ch/b19/ubuntu:latest`), because the
// basename ALREADY carries the project's namespace. Defaulting the segment to
// `${image.namespace}` instead would publish `kiota.ch/b19/b19/ubuntu`.
func template(r pfmodel.Registry) string {
	if r.Ref != "" {
		return r.Ref
	}
	if r.Owner == "" {
		return varHost + "/" + ref(fieldpath.AddrImageBasename) + ":" + ref(fieldpath.AddrImageTag)
	}
	return varHost + "/" + varOwner + "/" + ref(fieldpath.AddrImageBasename) + ":" + ref(fieldpath.AddrImageTag)
}

// ref wraps a field address as a `${…}` reference, so the addresses in a
// template are spelled from the constants core owns rather than as literals.
func ref(addr string) string { return "${" + addr + "}" }

// legacyRegistry synthesizes the single entry a project carries when it declares
// no registries namespace, from the `org.projectfile.readme.registry` scalar the
// container fragment has always set. Returns nil when that scalar is absent too —
// a project with no container build has no registry, and the caller drops the
// whole reference rather than inventing a host.
//
// The slug is the host's first domain label, the same rule the forge remotes use,
// so `${…registries.kiota.ref}` addresses it by the name a reader would guess.
func legacyRegistry(pf *projectfile.Document) []pfmodel.Registry {
	readme, ok := projectfile.LookupExtension(pf, pfmodel.ReadmeExtensionNS)
	if !ok {
		return nil
	}
	m, ok := readme.(map[string]any)
	if !ok {
		return nil
	}
	host, _ := m["registry"].(string)
	if host == "" {
		return nil
	}
	genlog.Decision("registry_ref", host, "org.projectfile.readme.registry", "no registries namespace declared")
	return []pfmodel.Registry{{Slug: slugOf(host), Host: host}}
}

// slugOf is the host's first domain label — `ghcr.io` → `ghcr`, `kiota.ch` →
// `kiota`. A host carrying a port keeps the port out of the slug, because a slug
// is a name a reader types, not an address.
func slugOf(host string) string {
	label, _, _ := strings.Cut(host, ".")
	label, _, _ = strings.Cut(label, ":")
	return label
}

// priorityLabel renders an entry's rank for the decision trace, saying "default"
// where the document declared nothing rather than printing a number no one wrote.
func priorityLabel(r pfmodel.Registry) string {
	if r.Priority == 0 {
		return "default"
	}
	return strconv.Itoa(r.Priority)
}
