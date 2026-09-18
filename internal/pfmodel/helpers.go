// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package pfmodel holds the bridge-owned typed shapes of projectfile
// extension namespaces (citation, readme, forge, funding, ...) plus the
// accessors that parse them out of *projectfile.Document.
//
// These lived in core/internal/projectfile before the "core 2.0" cut and
// were moved here because every consumer is a bridge: the document backend
// no longer carries shapes derived from external files it does not own.
// Core keeps the generic primitives (Document, Read, Write, MergePeople,
// LookupExtension, ...) and the namespace-string constants; pfmodel reuses
// those via the projectfile façade and adds the bridge-specific layer.
package pfmodel

import (
	"cmp"
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/fieldpath"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Extension namespace strings. Each accessor below routes through
// projectfile.LookupExtension so TOML dotted headers and flat keys both
// resolve. Core keeps its own copies for the include-machinery fallback
// (org.projectfile.cli carries an `includes` list); these are the
// bridge-side mirrors for the namespaces pfmodel parses.
const (
	IgnoresExtensionNS          = "org.projectfile.ignores"
	AttributesExtensionNS       = "org.projectfile.attributes"
	EditorsExtensionNS          = "org.projectfile.editors"
	VulnerabilitiesExtensionNS  = "org.projectfile.vulnerabilities"
	FundingExtensionNS          = "org.projectfile.funding"
	SecurityExtensionNS         = "org.projectfile.security"
	CodeOfConductExtensionNS    = "org.projectfile.code-of-conduct"
	DEIExtensionNS              = "org.projectfile.dei"
	AIExtensionNS               = "org.projectfile.ai"
	ContributingExtensionNS     = "org.projectfile.contributing"
	CodeOwnersExtensionNS       = "org.projectfile.codeowners"
	ForgeExtensionNS            = "org.projectfile.forge"
	ConventionsExtensionNS      = "org.projectfile.conventions"
	SupportExtensionNS          = "org.projectfile.support"
	ReadmeExtensionNS           = "org.projectfile.readme"
	ReleaseExtensionNS          = "org.projectfile.release"
	CitationExtensionNS         = "org.projectfile.citation"
	I18NExtensionNS             = "org.projectfile.i18n"
	FragmentsExtensionNS        = "org.projectfile.fragments"
	ArtifactsExtensionNS        = "org.projectfile.artifacts"
	AcknowledgementsExtensionNS = "org.projectfile.acknowledgements"

	// SinksExtensionNS holds the named destinations a project publishes its
	// container images to. ImageExtensionNS holds the PARTS those destinations
	// compose from, and is the scope a sink `ref` template resolves under — so a
	// template says `${path}` where it would otherwise spell the whole address.
	SinksExtensionNS = "org.projectfile.sinks"
	ImageExtensionNS = "org.projectfile.image"
	// PublishExtensionNS routes each forge’s pipeline to the sinks it pushes to.
	PublishExtensionNS = "org.projectfile.publish"
)

// Keys a sink entry carries. `ref` is the template, and `role` is what makes an
// entry ADDRESSABLE, because a bare `{}` projection admits no trailing field
// and `.ref` is reachable only through the selector form `{role=…}`. `label` is
// the display name the readme's per-destination install subsections show.
// Every other key an author writes is theirs — a template may name it and
// nothing here interprets it.
// `ref` addresses any image; `selfref` addresses only this project's own artifact.
const (
	SinkRefKey     = "ref"
	SinkSelfRefKey = "selfref"
	SinkRoleKey    = "role"
	SinkLabelKey   = "label"
	// PublishPushKey is the sink-name list a publish route pushes to.
	PublishPushKey = "push"

	// SinkRolePrimary is the role an entry carries when it declares none: the
	// ordinary destination, as opposed to the fallback a reader is told to try
	// only when the others are unreachable.
	SinkRolePrimary = "primary"
)

// PriorityDefault is the priority an item carries when it declares none. A
// stable sort by priority keeps declaration order for every default item, so a
// document that never sets priority renders byte-identical to the pre-priority
// layout. The value sits above the typical "pin to first" values (10, 0) and
// below "pin after the defaults" values (100, 200) so either end of the scale
// has room without colliding with the default.
//
// Read from core rather than restated: core applies the same number when it
// orders a `{}` fan-out, and a second copy here would let a Go-side list and the
// resolver disagree about where an unranked entry sits in the same document.
const PriorityDefault = fieldpath.PriorityDefault

// RankOf maps "unset" to the default rank. Zero is the reader here rather than a
// presence flag because an author who means "sort me last" writes a low number,
// not a missing key — and every priority-aware sort (badges within a row, groups
// within a section, registries within a fan-out) has to agree on that reading,
// or a Go-side list and the resolver's `{}` order disagree on the same document.
func RankOf(p int) int {
	if p == 0 {
		return PriorityDefault
	}
	return p
}

// ByPriorityDesc is the single comparator every priority-aware sort uses, so
// the direction lives in exactly one place. Higher number renders FIRST. A
// negative result (a sorts before b) is returned when a has the higher
// priority, matching slices.SortStableFunc / cmp.Compare semantics.
func ByPriorityDesc(a, b int) int { return cmp.Compare(b, a) }

// lookupNS resolves a reverse-DNS namespace on a Document via the core façade.
// Each accessor calls this so the flat-key vs dotted-table distinction lives
// in exactly one place (core's LookupExtension).
func lookupNS(doc *projectfile.Document, ns string) (map[string]any, bool, error) {
	raw, ok := projectfile.LookupExtension(doc, ns)
	if !ok {
		return nil, false, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s is not a map", ns)
	}
	return m, true, nil
}

// HasExtension reports whether an extension namespace is present on the
// document — the boolean convenience over LookupExtension that contributing
// and similar bridges use to gate optional blocks.
func HasExtension(doc *projectfile.Document, ns string) bool {
	_, ok := projectfile.LookupExtension(doc, ns)
	return ok
}

// --- tiny map-parsing helpers (mirrors of core/internal/projectfile/parse.go) ---
// These are five-liners; duplicating them keeps pfmodel free of any internal
// import. Core stays the single owner of the parse-time versions.

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func strListVal(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	return toStrSlice(v)
}

// strMapVal reads a nested map of scalar strings, the shape an open-key
// vocabulary takes (org.projectfile.ai activities). A non-string value is
// skipped rather than failing the parse: one mistyped entry must not cost the
// reader the whole policy. Returns nil when the key is absent or not a map.
func strMapVal(m map[string]any, key string) map[string]string {
	raw, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			genlog.Warn("ignored non-string entry in map field", "field", key, "entry", k)
			continue
		}
		out[k] = s
	}
	return out
}

// toStrSlice converts a parsed value to []string. Accepts both the
// parse-time shape ([]any from JSON/YAML/TOML decoders) and the
// serialize-time shape ([]string from ToMap).
func toStrSlice(v any) []string {
	if list, ok := v.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if list, ok := v.([]string); ok {
		out := make([]string, len(list))
		copy(out, list)
		return out
	}
	return nil
}

func intVal(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func floatVal(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func boolVal(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// boolValDefaultTrue reads m[outer][inner] as a bool; a missing path or non-bool leaf stays on.
func boolValDefaultTrue(m map[string]any, outer, inner string) bool {
	sub, ok := m[outer].(map[string]any)
	if !ok {
		return true
	}
	v, ok := sub[inner]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	if !ok {
		return true
	}
	return b
}

// localizedStringVal converts an extension-map value into a
// *projectfile.LocalizedString, accepting either encoding spec §7 allows for
// a reserved localized-string field: a bare scalar (language-agnostic) or a
// map of BCP 47 tag -> string. Returns nil when the key is absent or every
// candidate value is empty, so callers can treat nil as "no statement" the
// same way they treat a missing key.
func localizedStringVal(m map[string]any, key string) *projectfile.LocalizedString {
	switch v := m[key].(type) {
	case string:
		if v == "" {
			return nil
		}
		return &projectfile.LocalizedString{Bare: v}
	case map[string]any:
		langs := make(map[string]string, len(v))
		for lang, raw := range v {
			if s, ok := raw.(string); ok && s != "" {
				langs[lang] = s
			}
		}
		if len(langs) == 0 {
			return nil
		}
		return &projectfile.LocalizedString{Langs: langs}
	}
	return nil
}
