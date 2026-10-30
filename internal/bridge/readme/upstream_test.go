// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pgName is the upstream every test here packages.
const pgName = "PostgreSQL"

// TestUpstreamSectionNamesUpstreamLicenseAndAuthors: each upstream gets its sentence, license and thanks; ours stays apart in License.
func TestUpstreamSectionNamesUpstreamLicenseAndAuthors(t *testing.T) {
	pf := minimalDoc(t)
	pf.Identity.Version = "18"
	pf.Extensions = map[string]any{"upstream": []any{
		map[string]any{keyName: pgName, keyURL: "https://www.postgresql.org", "version": "${identity.version}", "license": pgName, "authors": "The PostgreSQL Global Development Group"},
		map[string]any{keyName: map[string]any{"en": "Helper"}, "relation": "fork"},
	}}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "## Upstream\n\nThis project packages [PostgreSQL](https://www.postgresql.org) 18 (`PostgreSQL`).\n"+
		"Thanks to: The PostgreSQL Global Development Group\n\nThis project is a fork of Helper (its own license).\n")
	assert.Contains(t, out, "## License\n\nThe packaging in this repository is licensed under MIT — see the [LICENSE](LICENSE) file for details.\n")
	assert.Less(t, strings.Index(out, "## Upstream"), strings.Index(out, "## License"))
}

// TestUpstreamAbsentKeepsPlainLicense: without upstream the license sentence is unchanged.
func TestUpstreamAbsentKeepsPlainLicense(t *testing.T) {
	out := renderDoc(t, t.TempDir(), minimalDoc(t))

	assert.Contains(t, out, "## License\n\nThis project is licensed under MIT")
	assert.NotContains(t, out, "## Upstream")
}

// TestUpstreamUnresolvedVersionOmitted: a version reference that resolves to nothing is left out, the name stays.
func TestUpstreamUnresolvedVersionOmitted(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{"upstream": []any{map[string]any{keyName: pgName, "version": "${org.example.missing}"}}}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "This project packages PostgreSQL (its own license).\n")
	assert.NotContains(t, out, "Thanks to")
}
