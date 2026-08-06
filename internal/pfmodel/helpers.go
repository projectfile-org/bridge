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
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Extension namespace strings. Each accessor below routes through
// projectfile.LookupExtension so TOML dotted headers and flat keys both
// resolve. Core keeps its own copies for the include-machinery fallback
// (org.projectfile.cli carries an `includes` list); these are the
// bridge-side mirrors for the namespaces pfmodel parses.
const (
	IgnoresExtensionNS         = "org.projectfile.ignores"
	AttributesExtensionNS      = "org.projectfile.attributes"
	EditorsExtensionNS         = "org.projectfile.editors"
	VulnerabilitiesExtensionNS = "org.projectfile.vulnerabilities"
	FundingExtensionNS         = "org.projectfile.funding"
	SecurityExtensionNS        = "org.projectfile.security"
	CodeOfConductExtensionNS   = "org.projectfile.code-of-conduct"
	DEIExtensionNS             = "org.projectfile.dei"
	ContributingExtensionNS    = "org.projectfile.contributing"
	CodeOwnersExtensionNS      = "org.projectfile.codeowners"
	CLIExtensionNS             = "org.projectfile.cli"
	ForgeExtensionNS           = "org.projectfile.forge"
	ConventionsExtensionNS     = "org.projectfile.conventions"
	SupportExtensionNS         = "org.projectfile.support"
	ReadmeExtensionNS          = "org.projectfile.readme"
	ReleaseExtensionNS         = "org.projectfile.release"
	CitationExtensionNS        = "org.projectfile.citation"
	I18NExtensionNS            = "org.projectfile.i18n"
	FragmentsExtensionNS       = "org.projectfile.fragments"
	ArtifactsExtensionNS       = "org.projectfile.artifacts"
)

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
