// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Artifact kinds this fleet's generators recognise. The vocabulary is OPEN —
// nothing rejects an unlisted kind — but these are the ones a README recipe and
// the artifacts inventory block know how to phrase, so they are named here
// rather than spelled as literals at each use.
const (
	ArtifactKindImage   = "image"
	ArtifactKindBinary  = "binary"
	ArtifactKindPackage = "package"
	ArtifactKindService = "service"
	ArtifactKindWebsite = "website"
	ArtifactKindArchive = "archive"
	ArtifactKindModule  = "module"
)

// Artifact is one thing a project SHIPS. Which address field carries meaning
// depends on `Kind`, and the split is deliberate: a reader pointed at the wrong
// field gets nothing rather than a plausible-looking wrong value.
//
//	image    Ref                  a pull reference — kiota.ch/d9t/go-tools:latest
//	binary   Path, Command        the built file, and the name it answers to
//	package  Registry, Name       which index, and the name published there
//	module   Module               an importable path — kiota.ch/projectfile/core/v2
//	service  Ports, Protocol      what it listens on once it is up
//	website  URL                  a reachable address
//	archive  Path                 a release bundle on disk
//
// Nothing enforces the pairing beyond the schema's per-kind requirements: `kind`
// stays ADVISORY for CI lowering (a resolver MUST NOT branch on it), and becomes
// load-bearing only for documentation, which is not lowering.
type Artifact struct {
	// Key is the map key the artifact was declared under — its identity in the
	// document, and what a `${…artifacts.<key>.…}` address names. Carried on the
	// value so a list of artifacts stays self-describing once the map is
	// flattened. Distinct from Name, which is the name a PACKAGE is published
	// under: `cli: {kind: package, name: "@scope/thing"}` has both.
	Key string

	Kind    string
	Summary string
	// SummaryByLang is the raw lang→text map when Summary was localized, so a
	// README rendered in Ukrainian describes the artifact in Ukrainian.
	SummaryByLang map[string]string

	Path string
	URL  string
	Ref  string
	// Command is the executable name a binary answers to once installed — the
	// value that lets a README say how to CALL the thing, not just how to get it.
	Command string
	// Name is the name a package is published under in its registry, which is
	// not always the project's own name (a scoped npm package, a distribution
	// renamed for PyPI).
	Name     string
	Registry string
	Module   string
	Version  string
	Ports    []ArtifactPort
}

// ArtifactPort is one listening port of a service artifact. Name distinguishes
// several ports on one service (`metrics` vs `api`); Protocol defaults to tcp at
// the point of use, not here, so an absent value stays visibly absent.
type ArtifactPort struct {
	Port     int
	Name     string
	Protocol string
}

// GetArtifacts parses `org.projectfile.artifacts` into the declared artifacts,
// sorted by name. Returns nil when the namespace is absent — a project that
// declares nothing produces nothing a generator can describe, which is a valid
// state, not an error.
//
// Sorted-by-name order matches the resolver's `{kind=…}` fan-out order, so a
// README's artifact list and its rendered command lines agree.
func GetArtifacts(doc *projectfile.Document) ([]Artifact, error) {
	m, present, err := lookupNS(doc, ArtifactsExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Artifact, 0, len(names))
	for _, name := range names {
		em, ok := m[name].(map[string]any)
		if !ok {
			continue
		}
		out = append(out, Artifact{
			Key:           name,
			Kind:          strVal(em, "kind"),
			Summary:       extractLocalizedVal(em, "summary"),
			SummaryByLang: extractLocalizedMap(em, "summary"),
			Path:          strVal(em, "path"),
			URL:           strVal(em, "url"),
			Ref:           strVal(em, "ref"),
			Command:       strVal(em, "command"),
			Name:          strVal(em, "name"),
			Registry:      strVal(em, "registry"),
			Module:        strVal(em, "module"),
			Version:       strVal(em, "version"),
			Ports:         parseArtifactPorts(em),
		})
	}
	return out, nil
}

// ArtifactsOfKind filters artifacts by kind, preserving order. The Go-side twin
// of the `{kind=…}` address so the inventory block and a command recipe select
// the same set.
func ArtifactsOfKind(artifacts []Artifact, kind string) []Artifact {
	var out []Artifact
	for _, a := range artifacts {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// parseArtifactPorts reads a service artifact's `ports` list. Each entry is
// either a bare number (`- 3306`) or a map carrying name/protocol — the bare
// form is what makes "MySQL on 3306" a one-line declaration.
func parseArtifactPorts(em map[string]any) []ArtifactPort {
	items, ok := em["ports"].([]any)
	if !ok {
		return nil
	}
	var out []ArtifactPort
	for _, item := range items {
		if n, ok := asInt(item); ok {
			out = append(out, ArtifactPort{Port: n})
			continue
		}
		pm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		n, ok := asInt(pm["port"])
		if !ok {
			continue
		}
		out = append(out, ArtifactPort{
			Port:     n,
			Name:     strVal(pm, "name"),
			Protocol: strVal(pm, "protocol"),
		})
	}
	return out
}

// asInt coerces the numeric shapes the three parsers produce (YAML gives int,
// JSON gives float64, TOML gives int64) into one int. A non-numeric value is
// refused rather than defaulted — port 0 is not a port.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), n == float64(int(n))
	default:
		return 0, false
	}
}
