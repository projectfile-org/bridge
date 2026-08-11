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
// What we are trying to do: let one project declare one image path and reach
// several destinations with it. GHCR nests freely, Docker Hub holds exactly
// `namespace/name`, and the same build feeds both — so the path grammar is a
// property of the SINK, the image path is a property of the PROJECT, neither can
// be computed from the other, and neither should ever be typed twice.
//
// # Where the composition happens
//
// Not here. `core/pkg/sink` owns it, expanding each template against a scratch
// document that carries the sink's own keys under `sink` and the image
// coordinates under `image`. This package's whole job is to bind the COORDINATES
// — this project's basename and tag — and to write the result back as document
// data. That split is what lets the same composer answer "where do I push this
// project" here and "where does this project's BASE image live" in pf-cli, where
// the coordinates name a foreign project.
package ocisinks

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/sink"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// legacyRegistryKey is the single-registry scalar the container fragment has
// always set, and the only input the ~130 projects that never heard of the sinks
// namespace carry.
const legacyRegistryKey = "registry"

// Refs returns the sinks subtree with every entry's `ref` composed into a
// CONCRETE reference, keyed by sink name. Returns nil when the project publishes
// no image at all, which is the signal for the caller to write nothing.
//
// The map is the merged view: the sinks the document declares, or — when it
// declares none — ONE sink synthesized from the legacy
// `org.projectfile.readme.registry` scalar. That synthesis is what lets the
// projects which never heard of this namespace keep rendering the exact pull line
// they rendered before it existed, with no edit and no `when:` clause in the
// shared fragment.
//
// A sink whose template survives composition only half-resolved is DROPPED, not
// written: a reference that silently lost a segment is a push to the wrong
// repository, and a README advertising it would send readers there too.
func Refs(pf *projectfile.Document) map[string]any {
	if pf == nil {
		return nil
	}
	basename, ok := projectfile.ImageBasename(pf)
	if !ok {
		genlog.Decision("sink_ref", "", "no image basename", "project publishes no image")
		return nil
	}
	tag, _ := projectfile.ImageTag(pf)
	coords := sink.Coords{Basename: basename, Tag: tag}

	declared, err := sink.Declared(pf)
	if err != nil {
		genlog.Warn("derive: sinks namespace unreadable — refs not composed", "error", err.Error())
		return nil
	}
	if len(declared) == 0 {
		declared = legacySink(pf)
	}
	if len(declared) == 0 {
		return nil
	}
	out := make(map[string]any, len(declared))
	for _, s := range declared {
		ref, composed := s.Compose(coords)
		if !composed {
			genlog.Warn("derive: sink ref left unresolved — entry dropped", "sink", s.Name,
				"template", s.Template(), "basename", basename)
			continue
		}
		genlog.Decision("sink_ref", ref, s.Name, "role="+s.Role())
		out[s.Name] = entry(s, ref)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// entry renders one sink back as document data: the keys the author declared,
// with the two COMPUTED ones written over them.
//
// `ref` replaces the template rather than sitting beside it — two spellings of
// one reference would drift the moment a sink moved. `role` is always written,
// defaulting to primary, because a bare `{}` projection admits no trailing field:
// a fragment reaches `.ref` across the map only through the selector form
// `{role=…}`, so an entry with no role would be addressable by nothing and its
// pull line would never render.
func entry(s sink.Sink, ref string) map[string]any {
	m := make(map[string]any, len(s.Entry)+2)
	for k, v := range s.Entry {
		m[k] = v
	}
	m[sink.KeyRef] = ref
	m[sink.KeyRole] = s.Role()
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
func legacySink(pf *projectfile.Document) []sink.Sink {
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
	return []sink.Sink{{Name: nameOf(host), Entry: map[string]any{sink.KeyHost: host}}}
}

// nameOf is the host's first domain label — `ghcr.io` → `ghcr`, `kiota.ch` →
// `kiota`. A host carrying a port keeps the port out of the name, because a sink
// name is a label a reader types, not an address.
func nameOf(host string) string {
	label, _, _ := strings.Cut(host, ".")
	label, _, _ = strings.Cut(label, ":")
	return label
}
