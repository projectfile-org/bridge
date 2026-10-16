// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// TestScalarToStringCoercesAxisShapes pins the matrix-cell rendering for every
// YAML scalar shape a projectfile may carry.
func TestScalarToStringCoercesAxisShapes(t *testing.T) {
	assert.Equal(t, "22", pfmodel.ScalarToString(22))
	assert.Equal(t, "8.5", pfmodel.ScalarToString(8.5))
	assert.Equal(t, "cli", pfmodel.ScalarToString("cli"))
	assert.Equal(t, "true", pfmodel.ScalarToString(true))
	assert.Empty(t, pfmodel.ScalarToString(nil), "nil is not a matrix value")
	assert.Empty(t, pfmodel.ScalarToString([]any{"x"}), "a composite is not a matrix value")
}

// TestExpandAxesFansOutOnePerCell mirrors the b19/ubuntu case: a URL carrying
// one axis placeholder must become one link per declared series, never the
// raw placeholder.
func TestExpandAxesFansOutOnePerCell(t *testing.T) {
	axes := map[string][]string{"B19_UBUNTU_SERIES": {"resolute", "noble"}}
	out := pfmodel.ExpandAxes([]string{"https://hub.docker.com/r/damianbuho/b19-ubuntu-{B19_UBUNTU_SERIES}"}, axes)
	assert.Equal(t, []string{
		"https://hub.docker.com/r/damianbuho/b19-ubuntu-resolute",
		"https://hub.docker.com/r/damianbuho/b19-ubuntu-noble",
	}, out)
}

// TestExpandAxesLeavesUndeclaredBraceAlone: a brace with no matching axis
// (e.g. a shell format string) must survive untouched, not be treated as an
// axis to substitute.
func TestExpandAxesLeavesUndeclaredBraceAlone(t *testing.T) {
	out := pfmodel.ExpandAxes([]string{`docker inspect --format '{{.Id}}'`}, nil)
	assert.Equal(t, []string{`docker inspect --format '{{.Id}}'`}, out)
}

// TestAllMatrixAxesUnionsNodeAxes: axes declared per node join the top-level ones, values deduplicated in order.
func TestAllMatrixAxesUnionsNodeAxes(t *testing.T) {
	const arch = "GOARCH"
	withAxis := func(axis string, values ...any) map[string]any {
		return map[string]any{"matrix": map[string]any{"axes": map[string]any{axis: values}}}
	}
	ci := withAxis("S", "a")
	ci["nodes"] = map[string]any{
		"built":    withAxis(arch, "amd64", "arm64"),
		"released": withAxis(arch, "arm64", "riscv64"),
		"plain":    nil,
	}
	doc := &projectfile.Document{Extensions: map[string]any{"org.projectfile.ci": ci}}
	assert.Equal(t, map[string][]string{"S": {"a"}, arch: {"amd64", "arm64", "riscv64"}}, pfmodel.AllMatrixAxes(doc, "org.projectfile.ci"))
	assert.Equal(t, map[string][]string{"S": {"a"}}, pfmodel.MatrixAxes(doc, "org.projectfile.ci"), "the root-only reader is unchanged")
}

// TestAllMatrixDerivesArtifactNodes: `matrix: {artifact: NAME}` reads the record's lists, project-level arch as fallback, minus its exclude cells.
func TestAllMatrixDerivesArtifactNodes(t *testing.T) {
	const amd64, arm64, darwin, riscv64 = "amd64", "arm64", "darwin", "riscv64"
	artifactNode := func(name string) map[string]any {
		return map[string]any{"matrix": map[string]any{"artifact": name}}
	}
	doc := &projectfile.Document{Extensions: map[string]any{
		"org.projectfile.architecture": []any{amd64, arm64, riscv64},
		"org.projectfile.artifacts": map[string]any{"go-binary": map[string]any{
			"operating-system": []any{"linux", darwin},
			"exclude":          []any{map[string]any{"os": darwin, "arch": riscv64}},
		}},
		"org.projectfile.ci": map[string]any{"nodes": map[string]any{
			"built": artifactNode("go-binary"),
			"ghost": artifactNode("missing"),
		}},
	}}
	matrix := pfmodel.AllMatrix(doc, "org.projectfile.ci")
	assert.Equal(t, map[string][]string{"TARGET_OS": {"linux", darwin}, "TARGET_ARCH": {amd64, "arm64", riscv64}}, matrix.Axes)
	assert.Equal(t, []map[string]string{{"TARGET_OS": darwin, "TARGET_ARCH": riscv64}}, matrix.Exclude)
	assert.ElementsMatch(t, []string{"x-linux-amd64", "x-linux-arm64", "x-linux-riscv64", "x-darwin-amd64", "x-darwin-arm64"},
		matrix.Expand([]string{"x-{TARGET_OS}-{TARGET_ARCH}"}), "five cells, darwin/riscv64 dropped")
}
