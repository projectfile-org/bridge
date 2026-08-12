// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package ocisinks composes the pull reference of every SINK a project publishes
// its container images to, from each sink's own `ref` template.
//
// Not to be confused with the sibling `derive/registries`, which infers PACKAGE
// index pages (npm, PyPI, crates.io) from the detected stack. This package is
// about the OCI plane: where a project's IMAGES land.
//
// What we are trying to do: let one project declare its image PARTS once and
// reach several destinations with them. GHCR nests freely, Docker Hub holds
// exactly `namespace/name`, and the same build feeds both — so the path grammar
// is a property of the SINK, the parts are a property of the PROJECT, neither
// can be computed from the other, and neither should ever be typed twice.
//
// # Where the composition happens
//
// In the document. A sink's `ref` is a spec §3.8 template naming the parts the
// project declares under `org.projectfile.image`, and this package expands it
// with that address as a SCOPE — so `${path}` and `${tag}` are short names in
// the template rather than full addresses. Adding a part, a destination or a
// whole new path grammar is an edit to a projectfile: nothing here knows what a
// registry is, what a host is, or how a repository path is spelled.
package ocisinks

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// legacyRegistryKey is the single-registry scalar the container fragment has
// always set, and the only input a project that never heard of the sinks
// namespace carries.
const legacyRegistryKey = "registry"

// legacyRefTemplate composes the one destination such a project publishes to.
// It is a template like any other and resolves through the same scope, so the
// legacy path and the declared path cannot disagree about where an image lands.
const legacyRefTemplate = "/${path}:${tag}"

// Refs returns the sinks subtree with every entry's `ref` composed into a
// CONCRETE reference, keyed by sink name. Returns nil when the project declares
// no destination at all, which is the signal for the caller to write nothing.
//
// The map is the merged view: the sinks the document declares, or — when it
// declares none — ONE sink synthesized from the legacy
// `org.projectfile.readme.registry` scalar. That synthesis is what lets a project
// which never heard of this namespace keep rendering the exact pull line it
// rendered before the namespace existed, with no edit and no `when:` clause in
// the shared fragment.
//
// A sink whose template survives composition only half-resolved is DROPPED, not
// written: a reference that silently lost a segment is a push to the wrong
// repository, and a README advertising it would send readers there too.
func Refs(pf *projectfile.Document) map[string]any {
	if pf == nil {
		return nil
	}
	declared := declaredSinks(pf)
	if len(declared) == 0 {
		declared = legacySink(pf)
	}
	if len(declared) == 0 {
		genlog.Decision("sink_ref", "", pfmodel.SinksExtensionNS, "no sink declared")
		return nil
	}
	out := make(map[string]any, len(declared))
	for name, entry := range declared {
		tmpl, ok := entry[pfmodel.SinkRefKey].(string)
		if !ok || tmpl == "" {
			genlog.Warn("derive: sink declares no ref template — entry dropped", "sink", name,
				"remedy", "declare "+pfmodel.SinkRefKey+" on the entry")
			continue
		}
		ref, resolved := interp.ExpandIn(pf, tmpl, pfmodel.ImageExtensionNS)
		if !resolved {
			genlog.Warn("derive: sink ref left unresolved — entry dropped", "sink", name,
				"template", tmpl, "composed", ref,
				"remedy", "declare the missing part under "+pfmodel.ImageExtensionNS)
			continue
		}
		genlog.Decision("sink_ref", ref, name, "template="+tmpl)
		out[name] = composed(entry, ref)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// declaredSinks reads the sinks namespace as the author wrote it. An entry that
// is not a map is skipped rather than fatal — one malformed sink must not cost a
// project every other pull line it publishes.
func declaredSinks(pf *projectfile.Document) map[string]map[string]any {
	raw, ok := projectfile.LookupExtension(pf, pfmodel.SinksExtensionNS)
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		genlog.Warn("derive: sinks namespace is not a map — refs not composed",
			"namespace", pfmodel.SinksExtensionNS)
		return nil
	}
	out := make(map[string]map[string]any, len(m))
	for name, v := range m {
		entry, ok := v.(map[string]any)
		if !ok {
			genlog.Warn("derive: sink entry is not a map — entry dropped", "sink", name)
			continue
		}
		out[name] = entry
	}
	return out
}

// composed renders one sink back as document data: the keys the author declared,
// with the composed reference written over the template.
//
// `ref` replaces the template rather than sitting beside it — two spellings of
// one reference would drift the moment a sink moved.
//
// `role` is the one key this package still supplies, and it is addressability
// rather than shape: a bare `{}` projection admits no trailing field, so a
// fragment reaches `.ref` across the map only through the selector form
// `{role=…}`. An entry with no role would be addressable by nothing and its pull
// line would never render — a silent drop, where every other drop here is
// logged.
func composed(entry map[string]any, ref string) map[string]any {
	m := make(map[string]any, len(entry)+1)
	for k, v := range entry {
		m[k] = v
	}
	m[pfmodel.SinkRefKey] = ref
	if role, ok := m[pfmodel.SinkRoleKey].(string); !ok || role == "" {
		m[pfmodel.SinkRoleKey] = pfmodel.SinkRolePrimary
	}
	return m
}

// legacySink synthesizes the single sink a project carries when it declares no
// sinks namespace, from the `org.projectfile.readme.registry` scalar. Returns nil
// when that scalar is absent too — a project with no container build has no
// destination, and the caller drops the whole reference rather than inventing a
// host.
//
// The fallback lives in Go rather than in the shared fragment because YAML has no
// conditional: a `default:` entry declared in the fragment would MERGE with a
// project's own sinks instead of yielding to them.
//
// The name is the host's first domain label, the same rule the forge remotes use,
// so `${…sinks.kiota.ref}` addresses it by the name a reader would guess.
func legacySink(pf *projectfile.Document) map[string]map[string]any {
	readme, ok := projectfile.LookupExtension(pf, pfmodel.ReadmeExtensionNS)
	if !ok {
		return nil
	}
	m, ok := readme.(map[string]any)
	if !ok {
		return nil
	}
	host, _ := m[legacyRegistryKey].(string)
	if host == "" {
		return nil
	}
	genlog.Decision("sink_ref", host, pfmodel.ReadmeExtensionNS+"."+legacyRegistryKey,
		"no sinks namespace declared")
	return map[string]map[string]any{
		nameOf(host): {pfmodel.SinkRefKey: host + legacyRefTemplate},
	}
}

// nameOf is the host's first domain label — `ghcr.io` → `ghcr`, `kiota.ch` →
// `kiota`. A host carrying a port keeps the port out of the name, because a sink
// name is a label a reader types, not an address.
func nameOf(host string) string {
	label, _, _ := strings.Cut(host, ".")
	label, _, _ = strings.Cut(label, ":")
	return label
}
