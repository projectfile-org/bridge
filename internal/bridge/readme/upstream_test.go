// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// pgName is the upstream every test here packages.
const pgName = "PostgreSQL"

// TestUpstreamNamedUnderBasicsAndSplitInLicense: the packaged upstream is named once and its license kept apart from ours.
func TestUpstreamNamedUnderBasicsAndSplitInLicense(t *testing.T) {
	pf := minimalDoc(t)
	pf.Identity.Version = "18"
	pf.Extensions = map[string]any{"upstream": []any{
		map[string]any{keyName: pgName, keyURL: "https://www.postgresql.org", "version": "${identity.version}", "license": pgName},
		map[string]any{keyName: map[string]any{"en": "Helper"}, "relation": "fork"},
	}}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "a demo project\n\nPackages [PostgreSQL](https://www.postgresql.org) 18.\n\nFork of Helper.")
	assert.Contains(t, out, "## License\n\nThe packaging in this repository is licensed under MIT — see the [LICENSE](LICENSE) file for details. "+
		"PostgreSQL is distributed under its own license, `PostgreSQL`. Helper is distributed under its own license.")
}

// TestUpstreamAbsentKeepsPlainLicense: without upstream the license sentence is unchanged.
func TestUpstreamAbsentKeepsPlainLicense(t *testing.T) {
	out := renderDoc(t, t.TempDir(), minimalDoc(t))

	assert.Contains(t, out, "## License\n\nThis project is licensed under MIT")
	assert.NotContains(t, out, "Packages ")
}

// TestUpstreamUnresolvedVersionOmitted: a version reference that resolves to nothing is left out, the name stays.
func TestUpstreamUnresolvedVersionOmitted(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{"upstream": []any{map[string]any{keyName: pgName, "version": "${org.example.missing}"}}}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "Packages PostgreSQL.")
}
