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
	return MatrixAxesOf(m)
}

// MatrixAxesOf reads `matrix.axes` from one mapping: a namespace root or a single node.
func MatrixAxesOf(m map[string]any) map[string][]string {
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

// AllMatrixAxes is MatrixAxes plus every node's own `matrix.axes`, values unioned in first-seen order.
func AllMatrixAxes(doc *projectfile.Document, ns string) map[string][]string {
	axes := MatrixAxes(doc, ns)
	raw, _ := projectfile.LookupExtension(doc, ns)
	m, _ := raw.(map[string]any)
	nodes, _ := m["nodes"].(map[string]any)
	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		node, _ := nodes[name].(map[string]any)
		for axis, values := range MatrixAxesOf(node) {
			if axes == nil {
				axes = map[string][]string{}
			}
			for _, value := range values {
				if !slices.Contains(axes[axis], value) {
					axes[axis] = append(axes[axis], value)
				}
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
	return Matrix{Axes: axes}.Expand(lines)
}

// Matrix is a document's axes plus the cells its `exclude` entries subtract.
type Matrix struct {
	Axes    map[string][]string
	Exclude []map[string]string
}

// AllMatrix is AllMatrixAxes plus every global and per-node `matrix.exclude` entry.
func AllMatrix(doc *projectfile.Document, ns string) Matrix {
	matrix := Matrix{Axes: AllMatrixAxes(doc, ns)}
	raw, _ := projectfile.LookupExtension(doc, ns)
	root, _ := raw.(map[string]any)
	matrix.Exclude = matrixExcludeOf(root)
	nodes, _ := root["nodes"].(map[string]any)
	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		node, _ := nodes[name].(map[string]any)
		matrix.Exclude = append(matrix.Exclude, matrixExcludeOf(node)...)
	}
	return matrix
}

// matrixExcludeOf reads `matrix.exclude` from one mapping as axis → value entries.
func matrixExcludeOf(m map[string]any) []map[string]string {
	matrix, _ := m["matrix"].(map[string]any)
	list, _ := matrix["exclude"].([]any)
	var out []map[string]string
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok || len(entry) == 0 {
			continue
		}
		cell := make(map[string]string, len(entry))
		for axis, value := range entry {
			cell[axis] = ScalarToString(value)
		}
		out = append(out, cell)
	}
	return out
}

// Excluded reports whether some exclude entry names only axes the cell carries, each at the cell's value.
func (m Matrix) Excluded(cell map[string]string) bool {
	for _, entry := range m.Exclude {
		matched := true
		for axis, value := range entry {
			if got, ok := cell[axis]; !ok || got != value {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// Expand fans each line out once per cell of the axes it names, in sorted axis order, minus excluded cells.
func (m Matrix) Expand(lines []string) []string {
	if len(m.Axes) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		texts, cells := []string{line}, []map[string]string{{}}
		for _, axis := range slices.Sorted(maps.Keys(m.Axes)) {
			token := "{" + axis + "}"
			if len(m.Axes[axis]) == 0 || !strings.Contains(line, token) {
				continue
			}
			nextTexts := make([]string, 0, len(texts)*len(m.Axes[axis]))
			nextCells := make([]map[string]string, 0, cap(nextTexts))
			for i := range texts {
				for _, value := range m.Axes[axis] {
					cell := maps.Clone(cells[i])
					cell[axis] = value
					nextTexts = append(nextTexts, strings.ReplaceAll(texts[i], token, value))
					nextCells = append(nextCells, cell)
				}
			}
			texts, cells = nextTexts, nextCells
		}
		for i := range texts {
			if !m.Excluded(cells[i]) {
				out = append(out, texts[i])
			}
		}
	}
	return out
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
