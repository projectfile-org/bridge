// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// ciNS is the namespace every matrix reader below names.
const ciNS = "org.projectfile.ci"

// ciDoc builds a document carrying one extension namespace.
func ciDoc(ns string, v any) *projectfile.Document {
	return &projectfile.Document{Extensions: map[string]any{ns: v}}
}

// matrixMap wraps a `matrix` sub-map so the brace counting stays in one place.
func matrixMap(v any) map[string]any {
	return map[string]any{"matrix": v}
}

// TestMatrixAxesPinsAbsentAndMalformedShapes pins every way a namespace can declare no axis: absent, not a map, no matrix, no axes, and an axis whose list is not a list.
func TestMatrixAxesPinsAbsentAndMalformedShapes(t *testing.T) {
	assert.Nil(t, MatrixAxes(&projectfile.Document{}, ciNS), "no namespace is no axis")
	assert.Nil(t, MatrixAxes(ciDoc(ciNS, notAMapping), ciNS), "a namespace that is not a map is no axis")
	assert.Nil(t, MatrixAxesOf(map[string]any{}), "no matrix sub-key is no axis")
	assert.Nil(t, MatrixAxesOf(matrixMap(notAMapping)), "a matrix that is not a map is no axis")
	assert.Nil(t, MatrixAxesOf(matrixMap(map[string]any{keyAxes: notAMapping})),
		"an axes table that is not a map is no axis")
	assert.Empty(t, MatrixAxesOf(matrixMap(map[string]any{keyAxes: map[string]any{"S": notAMapping}})),
		"an axis whose list is not a list declares nothing")

	ok := matrixMap(map[string]any{keyAxes: map[string]any{"S": []any{"a", "b"}}})
	assert.Equal(t, map[string][]string{"S": {"a", "b"}}, MatrixAxesOf(ok))
}

// TestMatrixAxesCoercesScalarsToTheirStringForm pins the coercion an integer or boolean series value needs, and the composite drop.
func TestMatrixAxesCoercesScalarsToTheirStringForm(t *testing.T) {
	axes := matrixMap(map[string]any{keyAxes: map[string]any{
		"B19_UBUNTU_SERIES": []any{int64(22), "resolute"},
		"ENABLED":           []any{true},
		"JUNK":              []any{[]any{"x"}, nil},
	}})
	assert.Equal(t, map[string][]string{
		"B19_UBUNTU_SERIES": {"22", "resolute"},
		"ENABLED":           {valueTrue},
	}, MatrixAxesOf(axes))
}

// TestAllMatrixAxesUnionsNodeAxesIncludingNodeDerivation pins the per-node union, the first-seen order, and the nil node that declares nothing.
func TestAllMatrixAxesUnionsNodeAxesIncludingNodeDerivation(t *testing.T) {
	doc := ciDoc(ciNS, map[string]any{
		keyNodes: map[string]any{
			nodeBuilt:  matrixMap(map[string]any{keyAxes: map[string]any{"GOARCH": []any{archAMD64}}}),
			"released": nil,
			"derived":  matrixMap(map[string]any{keyArtifact: artifactKeyCLI}),
		},
	})
	doc.Extensions[ArtifactsExtensionNS] = map[string]any{
		artifactKeyCLI: map[string]any{"architecture": []any{archARM64}, "operating-system": []any{osLinux}},
	}
	assert.Equal(t, map[string][]string{
		"GOARCH":       {archAMD64},
		axisTargetArch: {archARM64},
		"TARGET_OS":    {osLinux},
	}, AllMatrixAxes(doc, ciNS))
}

// TestExcludeSubtractsCells pins the cross-product subtraction: an entry naming only axes the cell carries, each at the cell's value, removes that cell.
func TestExcludeSubtractsCells(t *testing.T) {
	m := Matrix{
		Axes: map[string][]string{"A": {"1", "2"}, "B": {"x", "y"}},
		Exclude: []map[string]string{
			{"A": "2"},
			{"A": "1", "B": "y"},
			{"C": "z"},
		},
	}
	assert.True(t, m.Excluded(map[string]string{"A": "2", "B": "x"}))
	assert.True(t, m.Excluded(map[string]string{"A": "1", "B": "y"}))
	assert.False(t, m.Excluded(map[string]string{"A": "1", "B": "x"}),
		"an entry matching only part of its axes spares the cell")
	assert.False(t, m.Excluded(map[string]string{}), "an empty cell matches nothing")

	out := m.Expand([]string{"img-{A}-{B}"})
	assert.Equal(t, []string{"img-1-x"}, out,
		"one composed ref fans out minus the excluded cells")
}

// TestExpandLeavesAnUndeclaredBraceAlone pins the boundary with a shell brace.
func TestExpandLeavesAnUndeclaredBraceAlone(t *testing.T) {
	one := Matrix{Axes: map[string][]string{"A": {"1"}}}
	assert.Equal(t, []string{`{{.Id}}`, "no-axis"}, one.Expand([]string{`{{.Id}}`, "no-axis"}))
	assert.Equal(t, []string{"img-{A}"}, Matrix{Axes: map[string][]string{"A": {}}}.Expand([]string{"img-{A}"}),
		"an axis with no value fans out to nothing")
}

// TestMatrixExcludeOfSkipsMalformedEntries pins the reader's tolerance.
func TestMatrixExcludeOfSkipsMalformedEntries(t *testing.T) {
	out := matrixExcludeOf(matrixMap(map[string]any{keyExclude: []any{
		map[string]any{"A": "1"},
		map[string]any{},
		junkRow,
	}}))
	require.Len(t, out, 1)
	assert.Equal(t, "1", out[0]["A"])
	assert.Nil(t, matrixExcludeOf(map[string]any{}))
	assert.Nil(t, matrixExcludeOf(matrixMap(notAMapping)))
	assert.Nil(t, matrixExcludeOf(matrixMap(map[string]any{keyExclude: notAMapping})))
}

// TestArtifactMatrixFallsBackToTheProjectLists pins the per-artifact derivation and the two defaults: the project-level lists, then linux for the OS axis.
func TestArtifactMatrixFallsBackToTheProjectLists(t *testing.T) {
	doc := ciDoc(ciNS, map[string]any{
		keyNodes: map[string]any{
			nodeBuilt: matrixMap(map[string]any{keyArtifact: artifactKeyCLI}),
		},
	})
	doc.Extensions[ArtifactsExtensionNS] = map[string]any{
		artifactKeyCLI: map[string]any{keyExclude: []any{map[string]any{"arch": "riscv64"}, map[string]any{}}},
		artifactKeyWeb: map[string]any{fxURL: urlXTest},
	}
	doc.Extensions["org.projectfile.architecture"] = []any{archAMD64, archARM64}

	matrix := AllMatrix(doc, ciNS)
	assert.Equal(t, map[string][]string{
		axisTargetArch: {archAMD64, archARM64},
		"TARGET_OS":    {osLinux},
	}, matrix.Axes, "the artifact declares no list, so the project-level one answers")
	assert.True(t, matrix.Excluded(map[string]string{axisTargetArch: "riscv64"}),
		"an artifact exclude entry subtracts from its own matrix")
}

// TestArtifactMatrixWithNoListAnywhereDeclaresNoAxis pins the last rung: no list anywhere is no axis, warned rather than guessed.
func TestArtifactMatrixWithNoListAnywhereDeclaresNoAxis(t *testing.T) {
	doc := ciDoc(ciNS, map[string]any{
		keyNodes: map[string]any{nodeBuilt: matrixMap(map[string]any{keyArtifact: artifactKeyWeb})},
	})
	doc.Extensions[ArtifactsExtensionNS] = map[string]any{
		artifactKeyWeb: map[string]any{fxURL: urlXTest},
	}
	got := AllMatrix(doc, ciNS).Axes
	assert.NotContains(t, got, targetArchAxis, "a missing architecture list is no axis")
	assert.Equal(t, []string{osLinux}, got[targetOSAxis], "the OS axis still defaults to linux")
}
