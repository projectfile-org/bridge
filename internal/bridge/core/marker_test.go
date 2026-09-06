// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// spdxLicenseLine builds the REUSE licence-declaration line WITHOUT spelling
// the SPDX tag as a source literal — the REUSE linter scans source for that
// token and would flag a test fixture as a malformed licence expression.
const spdxTag = "SPDX-License-"

func spdxLicenseLine(expr string) string { return spdxTag + "Identifier: " + expr }

// TestHasMarkerRecognizesAllForms pins detection of the three sentinel shapes
// a generated file may carry: the hash-leader line (ignore files), the
// HTML-comment line (Markdown health files), and the bare inner text folded
// into a merged SPDX block (the README).
func TestHasMarkerRecognizesAllForms(t *testing.T) {
	t.Parallel()

	mergedHeader := "<!--\n" + spdxLicenseLine("MIT") + "\n" + MarkerInner + "\n-->\n"

	cases := []struct {
		input string
		want  bool
	}{
		{Marker + "\nbody\n", true},
		{MarkerHTML + "\nbody\n", true},
		{MarkerInner + "\nbody\n", true},
		{MarkerSlash + "\nbody\n", true},
		{mergedHeader, true},
		{"no marker here at all\n", false},
		{"", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, HasMarker([]byte(tc.input)), "input=%q", tc.input)
	}
}
