// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// MatrixAxes reads ns's `matrix.axes` as axis name → declared values. Nil when
// the namespace or its matrix is absent, which makes ExpandAxes a no-op. The
// namespace is a caller-supplied string rather than a pfmodel constant: CI is
// owned by the resolver, not this package, so a caller names its own extension.
//
// YAML scalar values are coerced to their STRING form: a matrix declared as
// `B19_LLVM_SERIES: [22, 21]` carries INTEGER items, and pf-ci/m6e substitute
// them as plain tokens, so a `{B19_LLVM_SERIES}` placeholder must match the
// same text.
func MatrixAxes(doc *projectfile.Document, ns string) map[string][]string {
	raw, ok := projectfile.LookupExtension(doc, ns)
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	matrix, ok := m["matrix"].(map[string]any)
	if !ok {
		return nil
	}
	rawAxes, ok := matrix["axes"].(map[string]any)
	if !ok {
		return nil
	}
	axes := make(map[string][]string, len(rawAxes))
	for axis, list := range rawAxes {
		items, ok := list.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			if value := ScalarToString(item); value != "" {
				axes[axis] = append(axes[axis], value)
			}
		}
	}
	return axes
}

// ExpandAxes substitutes the `{AXIS}` matrix placeholders m6e and pf-ci both
// replace per cell, fanning each line out to one per cell. A base image built
// once per series carries `{B19_UBUNTU_SERIES}` inside its published image
// path, and a reader (or a derived link) needs one entry per actual value —
// never the raw placeholder, which addresses nothing.
//
// Only axes the document DECLARES are substituted, and an unmatched brace is
// left alone — a shell brace like `{{.Id}}` is not an axis and is not this
// package's to rewrite. Axes are walked in sorted key order and their values
// in declared order, so a two-axis image produces a stable cross product.
func ExpandAxes(lines []string, axes map[string][]string) []string {
	if len(axes) == 0 {
		return lines
	}
	for _, axis := range slices.Sorted(maps.Keys(axes)) {
		values := axes[axis]
		if len(values) == 0 {
			continue
		}
		token := "{" + axis + "}"
		expanded := make([]string, 0, len(lines))
		for _, line := range lines {
			if !strings.Contains(line, token) {
				expanded = append(expanded, line)
				continue
			}
			for _, value := range values {
				expanded = append(expanded, strings.ReplaceAll(line, token, value))
			}
		}
		lines = expanded
	}
	return lines
}

// ScalarToString renders a YAML scalar (the shape an untyped decoder yields) as
// the plain token m6e/pf-ci substitute per matrix cell. Strings pass through;
// numbers and bools take their natural form (22, 8.5, true); anything
// composite or nil is not a matrix value and returns "" so the caller drops it.
func ScalarToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return fmt.Sprintf("%v", x)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", x)
	default:
		return ""
	}
}
