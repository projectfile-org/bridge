// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
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
