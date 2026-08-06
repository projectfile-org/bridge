// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package interp implements spec §3.8 interpolation: `${<fieldpath>}` inside a
// string scalar resolves against the merged projectfile document.
//
// What we are trying to do: let a SHARED include fragment carry a value that
// resolves differently in every document that includes it. One badge row in
// m6e/core reaches 125 projects because `${license.spdx}` is read from the
// consumer's own document at render time, not from the fragment.
//
// The address grammar is the existing pf-cli one (pkg/fieldpath), so there is
// no second syntax to learn and no string surgery anywhere: `${identity.name}`,
// `${links[type=source-code].url}`, `${org.projectfile.forge.remotes.codeberg.owner}`
// all work, including keys that themselves contain dots.
//
// Anything that is not a resolvable field address is left VERBATIM. That is the
// clause that makes cohabitation work: `${B19_DOCKER_REGISTRY}` is a make
// variable, not a field, so it survives untouched for m6e to resolve later.
package interp

import (
	"reflect"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/fieldpath"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// maxDepth bounds re-expansion of a resolved value that itself carries
// references (spec §3.8 "Cycles"). Four levels is far past any real fragment
// chain; beyond it whatever remains is emitted verbatim rather than looping.
const maxDepth = 4

// Marker is the opening delimiter of a reference. Exported because the readme
// bridge tests a rendered string for LEFTOVER references — an unresolved
// `${…}` in a badge URL means the badge must be dropped, not published broken.
const Marker = "${"

// Expand resolves every `${<fieldpath>}` reference in s against doc. A nil
// document, or a string with no `$`, is returned unchanged. A reference naming
// SEVERAL values is left verbatim: collapsing it to the first would render one
// image and silently hide the rest — a caller that wants every value calls
// ExpandFanOut.
func Expand(doc *projectfile.Document, s string) string {
	out, _ := ExpandChecked(doc, s)
	return out
}

// ExpandChecked is Expand plus a report of whether EVERY reference resolved.
//
// It exists because the two cannot be told apart by scanning the OUTPUT:
// `$${literal}` expands to a literal `${literal}`, which a `${`-scan misreads as
// an unresolved reference. A caller that drops half-substituted values (a badge
// URL, a command line) must ask during expansion, not after.
func ExpandChecked(doc *projectfile.Document, s string) (string, bool) {
	if doc == nil || !strings.ContainsRune(s, '$') {
		return s, true
	}
	lines, resolved := walk(doc, s, maxDepth, false)
	return lines[0], resolved
}

// ExpandFanOut is Expand for a string whose references may name SEVERAL values,
// returning one fully-expanded line per combination. A `docker pull
// ${org.projectfile.artifacts{kind=image}.ref}` command in a shared fragment
// becomes one pull line per image the consuming project actually publishes —
// which is how a base image built once per Ubuntu series documents every series
// without naming any of them. A string with no multi-valued reference comes back
// as exactly one line.
func ExpandFanOut(doc *projectfile.Document, s string) (lines []string, resolved bool) {
	if doc == nil || !strings.ContainsRune(s, '$') {
		return []string{s}, true
	}
	return walk(doc, s, maxDepth, true)
}

// Unresolved reports whether s still carries a reference — i.e. at least one
// `${…}` the document could not answer. Retained for callers holding a string
// they did not expand themselves; a caller doing the expansion should read the
// flag from ExpandChecked / ExpandFanOut instead, which `$$` cannot fool.
func Unresolved(s string) bool {
	return strings.Contains(s, Marker)
}

// walk is the one expansion engine. It walks s once, accumulating the output as a
// SET of partial lines, so a reference naming several values multiplies the set:
// ordinary characters append to every partial line, a reference appends its
// value(s) to every partial line, cross-producing when there are several.
//
// allowFanOut is what separates the two public entry points. With it false a
// multi-valued reference is emitted VERBATIM and marked unresolved — one line
// out, always, so the caller can index [0] — because a sentence has no per-value
// form and picking the first value would be a silent lie. With it true the set
// grows.
//
// Recursion goes into the substituted VALUE only, never over the accumulated
// output. That is what keeps `$$` an escape (an emitted literal `$` is never
// looked at again) and what catches a NESTED fan-out, where an artifact's `ref`
// is itself written in terms of a matrix-axis list: walking only the top level
// silently collapsed that to one value.
func walk(doc *projectfile.Document, s string, depth int, allowFanOut bool) (lines []string, resolved bool) {
	if depth <= 0 {
		return []string{s}, false
	}
	lines, resolved = []string{""}, true
	appendAll := func(suffix string) {
		for i := range lines {
			lines[i] += suffix
		}
	}
	for i := 0; i < len(s); {
		if s[i] != '$' {
			appendAll(s[i : i+1])
			i++
			continue
		}
		// `$$` is a literal `$` (spec §3.8 "Escaping"): consume both, emit one.
		if i+1 < len(s) && s[i+1] == '$' {
			appendAll("$")
			i += 2
			continue
		}
		ref, next, ok := reference(s, i)
		if !ok {
			appendAll(s[i : i+1])
			i++
			continue
		}
		values, found := lookupValues(doc, ref)
		if found && len(values) > 1 && !allowFanOut {
			genlog.Decision("interpolate", ref, "several values where one is needed (verbatim)", "")
			found = false
		}
		if !found {
			appendAll(s[i:next]) // verbatim — not ours to resolve
			resolved = false
			i = next
			continue
		}
		grown, capped := grow(doc, lines, values, depth, allowFanOut, &resolved)
		if capped {
			genlog.Warn("interpolate: fan-out capped", "reference", ref, "limit", maxFanOut)
			return grown, resolved
		}
		lines = grown
		i = next
	}
	return lines, resolved
}

// grow appends each resolved value to every partial line, recursing into the
// value so a reference INSIDE it expands (and fans out) too. capped reports
// hitting maxFanOut, which stops the walk rather than writing thousands of lines.
//
// Existing partial lines are the OUTER loop so the reference encountered latest
// varies fastest — `a x, a y, b x, b y`, the order a reader scanning the rendered
// block expects, rather than a column-major shuffle of the same set. Each value
// is walked once up front, not once per partial line.
func grow(doc *projectfile.Document, lines, values []string, depth int, allowFanOut bool, resolved *bool) (grown []string, capped bool) {
	expanded := make([][]string, 0, len(values))
	for _, value := range values {
		sublines, subResolved := walk(doc, value, depth-1, allowFanOut)
		*resolved = *resolved && subResolved
		expanded = append(expanded, sublines)
	}
	for _, prefix := range lines {
		for _, sublines := range expanded {
			for _, subline := range sublines {
				if len(grown) >= maxFanOut {
					return grown, true
				}
				grown = append(grown, prefix+subline)
			}
		}
	}
	return grown, false
}

// reference reads the `${…}` starting at i, returning the inner address and the
// index just past the closing brace. ok is false when i does not open a
// reference or the brace is never closed — the caller then emits the `$` as an
// ordinary character.
//
// The scan is brace-BALANCED, not first-`}`: the address grammar itself spells
// map forms in curly braces (`artifacts{kind=image}.ref`), so a naive scan ends
// the reference at the selector's brace and hands the resolver the truncated
// `artifacts{kind=image`. Quoted spans are skipped for the same reason
// fieldpath's own splitter skips them — a predicate value may carry a brace
// (`[label="a{b}"]`) and must not move the count.
func reference(s string, i int) (ref string, next int, ok bool) {
	if i+1 >= len(s) || s[i+1] != '{' {
		return "", 0, false
	}
	depth := 0
	inQuote := false
	for j := i + 1; j < len(s); j++ {
		switch {
		case inQuote:
			if s[j] == '"' {
				inQuote = false
			}
		case s[j] == '"':
			inQuote = true
		case s[j] == '{':
			depth++
		case s[j] == '}':
			depth--
			if depth == 0 {
				return s[i+2 : j], j + 1, true
			}
		}
	}
	return "", 0, false
}

// maxFanOut caps how many lines one templated string may expand into. A single
// artifact kind resolving to two or three values is the real case (a series
// image); anything past this is a runaway document, and emitting a truncated
// list with a warning beats writing thousands of README lines.
const maxFanOut = 64

// lookupValues resolves one address to the scalars it names — one for an ordinary
// field, several for a map selector (`artifacts{kind=image}.ref`) or a list
// projection (`repositories[].url`). found is false for a malformed address, a
// miss, an empty value, or any non-scalar element, which is the caller's signal to
// emit the reference verbatim.
func lookupValues(doc *projectfile.Document, ref string) (values []string, found bool) {
	path, err := fieldpath.Parse(ref)
	if err != nil {
		genlog.Decision("interpolate", ref, "not a field address (verbatim)", "")
		return nil, false
	}
	result, err := fieldpath.Resolve(doc, path)
	if err != nil {
		genlog.Decision("interpolate", ref, "unresolved (verbatim)", err.Error())
		return nil, false
	}
	if len(result.Values) == 0 {
		genlog.Decision("interpolate", ref, "no value (verbatim)", "")
		return nil, false
	}
	values = make([]string, 0, len(result.Values))
	for _, v := range result.Values {
		if !isScalar(v) {
			genlog.Decision("interpolate", ref, "no scalar value (verbatim)", "")
			return nil, false
		}
		out := fieldpath.FormatScalar(v)
		if out == "" {
			genlog.Decision("interpolate", ref, "empty value (verbatim)", "")
			return nil, false
		}
		values = append(values, out)
	}
	genlog.Decision("interpolate", ref, strings.Join(values, " "), fanOutSource(values))
	return values, true
}

// fanOutSource labels a multi-value resolution in the decision trace, so a reader
// of `-v` output can tell a fan-out from an ordinary substitution.
func fanOutSource(values []string) string {
	if len(values) > 1 {
		return "fan-out"
	}
	return ""
}

// isScalar reports whether v can stand in for a string. Composite values
// cannot: FormatScalar renders them as a Go/JSON dump, which is never what an
// author interpolating into a URL meant. The check is on the reflected KIND
// rather than a type switch because the document model resolves both untyped
// `[]any` (extension data) and typed slices/structs (`[]projectfile.Link`) —
// a type switch silently missed the latter.
func isScalar(v any) bool {
	if v == nil {
		return false
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}
