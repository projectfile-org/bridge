// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package license_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/license"
)

// spdxMIT is the SPDX id used as the single-term fixture across render tests.
const spdxMIT = "MIT"

func docWithLicense(spdxExpr string) *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "test-proj"},
		License:  &projectfile.License{Spdx: spdxExpr},
		Organizations: []projectfile.Organization{
			{Name: "Alice Corp", Roles: []string{projectfile.RoleCopyright}},
		},
		Copyright: &projectfile.Copyright{Year: 2024},
	}
}

// ── Render ───────────────────────────────────────────────────────────────────

func TestRenderMITSingleFile(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	// Single term produces the substituted root LICENSE plus the canonical
	// REUSE file (LICENSES/MIT.txt) — two files, distinct content.
	require.Len(t, out.Files, 2, "single-term emits root LICENSE + canonical REUSE file")
	_, ok := out.Files["LICENSE"]
	assert.True(t, ok, "output key must be 'LICENSE'")
	assert.NotEmpty(t, out.Files["LICENSE"])
}

func TestRenderCompoundMultiFile(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense("MIT OR Apache-2.0")
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "LICENSE", "compound must include overview LICENSE")
	assert.Contains(t, out.Files, "LICENSES/MIT.txt", "compound must include canonical REUSE file")
	assert.Contains(t, out.Files, "LICENSES/Apache-2.0.txt", "compound must include canonical REUSE file")
}

func TestRenderConjunctiveMultiFile(t *testing.T) {
	// AND (conjunctive) expressions fan out the same as OR — each
	// constituent license gets its own LICENSES/<id>.txt file plus an
	// overview LICENSE that says "you must comply with ALL".
	b := license.Bridge{}
	pf := docWithLicense("MIT AND Apache-2.0")
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "LICENSE", "conjunctive must include overview LICENSE")
	assert.Contains(t, out.Files, "LICENSES/MIT.txt", "conjunctive must include canonical REUSE file")
	assert.Contains(t, out.Files, "LICENSES/Apache-2.0.txt", "conjunctive must include canonical REUSE file")
	assert.Contains(t, string(out.Files["LICENSE"]), "conjunctive",
		"overview must say conjunctive for AND expressions")
}

func TestRenderSubstitutesCopyrightHolder(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files["LICENSE"]), "Alice Corp",
		"copyright holder must appear in rendered LICENSE")
}

func TestRenderHolderFromAuthorMaintainer(t *testing.T) {
	// When no person/org carries the `copyright` role, the holder is resolved
	// from the first author/maintainer person — the SAME tier the per-file
	// REUSE headers use (core.ReuseCopyrightHolderNames). The root LICENSE
	// and the headers MUST agree on the rights-holder.
	b := license.Bridge{}
	pf := &projectfile.Document{
		Identity:  projectfile.Identity{Name: "ubuntu"},
		License:   &projectfile.License{Spdx: spdxMIT},
		Copyright: &projectfile.Copyright{Year: 2026},
		People: []projectfile.Person{
			{
				FamilyNames: "Búho", GivenNames: "Damián", Email: "damian.buho@proton.me",
				Roles: []string{"author", "maintainer"},
			},
		},
	}
	projectfile.SetLocalizedEN(&pf.Identity.Title, "B19/Ubuntu")
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LICENSE"])
	assert.Contains(t, body, "Damián Búho",
		"root LICENSE must resolve the holder from author/maintainer, not the project title")
	assert.NotContains(t, body, "B19/Ubuntu",
		"project title (DisplayName) must not be used while an author/maintainer exists")
	assert.NotContains(t, body, "<copyright holders>",
		"literal placeholder must not survive")
}

func TestRenderNoHolderFallsBackToDisplayName(t *testing.T) {
	// Last-resort tier: no copyright-role holder AND no author/maintainer
	// person → fall back to the project DisplayName so the LICENSE never
	// ships a literal "<copyright holders>" placeholder.
	b := license.Bridge{}
	pf := &projectfile.Document{
		Identity:  projectfile.Identity{Name: "solo-proj"},
		License:   &projectfile.License{Spdx: spdxMIT},
		Copyright: &projectfile.Copyright{Year: 2024},
	}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LICENSE"])
	assert.Contains(t, body, "solo-proj",
		"root LICENSE must fall back to DisplayName when no holder of any tier exists")
	assert.NotContains(t, body, "<copyright holders>",
		"literal placeholder must not survive when no holder is declared")
}

func TestRenderEmitsSubstitutedReuseFile(t *testing.T) {
	// By default LICENSES/<id>.txt ships with the resolved holder/year
	// substituted in — more useful to humans than stale placeholders, and
	// still REUSE-compliant (lint resolves copyright from per-file headers).
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	reuse, ok := out.Files["LICENSES/MIT.txt"]
	require.True(t, ok, "single-term must emit LICENSES/MIT.txt")
	assert.Contains(t, string(reuse), "Alice Corp",
		"default LICENSES file carries the substituted copyright holder")
	assert.NotContains(t, string(reuse), "<copyright holders>",
		"literal placeholder must not survive under the default (substituted) mode")
}

func TestRenderEmitsCanonicalReuseFile(t *testing.T) {
	// --reuse-canonical keeps the literal SPDX placeholders, matching what
	// `reuse download` emits. Opt-out for projects that want the verbatim
	// upstream license text in LICENSES/.
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	out, err := b.Render(pf, core.Options{Offline: true, ReuseCanonical: true})
	require.NoError(t, err)
	reuse, ok := out.Files["LICENSES/MIT.txt"]
	require.True(t, ok, "single-term must emit LICENSES/MIT.txt")
	assert.Contains(t, string(reuse), "<copyright holders>",
		"--reuse-canonical keeps the canonical unsubstituted SPDX text")
	assert.NotContains(t, string(reuse), "Alice Corp",
		"substituted holder must not leak in under --reuse-canonical")
}

func TestRenderDeclaredPathCollidesWithReuse(t *testing.T) {
	// When license.file targets the same path the bridge uses for the REUSE
	// file (LICENSES/MIT.txt), both writes are substituted by default and
	// produce identical content — no corruption, the file is consistent.
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	pf.License.File = "LICENSES/MIT.txt"
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LICENSES/MIT.txt"])
	assert.Contains(t, body, "Alice Corp",
		"colliding declared path holds the substituted body (default mode)")
}

func TestRenderNoLicense(t *testing.T) {
	b := license.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "no-lic"}}
	_, err := b.Render(pf, core.Options{Offline: true})
	assert.Error(t, err, "Render without license.spdx must return an error")
}

func TestRenderAllEmbeddedLicenses(t *testing.T) {
	// Smoke: every embedded SPDX ID should render without error.
	b := license.Bridge{}
	for _, id := range []string{"MIT", "Apache-2.0", "GPL-3.0-or-later", "ISC", "BSD-2-Clause"} {
		t.Run(id, func(t *testing.T) {
			pf := docWithLicense(id)
			out, err := b.Render(pf, core.Options{Offline: true})
			require.NoError(t, err)
			assert.NotEmpty(t, out.Files)
		})
	}
}

// ── RequiredFields ────────────────────────────────────────────────────────────

func TestRequiredFieldsMissingLicense(t *testing.T) {
	b := license.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "test"}}
	missing := b.RequiredFields(pf)
	require.NotEmpty(t, missing, "RequiredFields must flag absent license.spdx")
	assert.Equal(t, "license.spdx", missing[0].Field)
}

func TestRequiredFieldsLicenseSet(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	missing := b.RequiredFields(pf)
	assert.Empty(t, missing, "RequiredFields must return nil when license.spdx is set")
}

func TestRequiredFieldsSetterInjectsSPDX(t *testing.T) {
	b := license.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "test"}}
	missing := b.RequiredFields(pf)
	require.NotEmpty(t, missing)
	require.NotNil(t, missing[0].Setter)
	require.NoError(t, missing[0].Setter("Apache-2.0"))
	assert.Equal(t, "Apache-2.0", pf.License.Spdx)
}

// ── license.file path resolution ──────────────────────────────────────────────

func TestRenderSingleDeclaredPath(t *testing.T) {
	// A declared license.file replaces the synthesised `LICENSE` name. We use
	// a path outside LICENSES/ so the canonical REUSE emission (LICENSES/MIT.txt)
	// does not collide with the declared target — keeping the two concerns
	// separable in the assertions.
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	pf.License.File = "COPYING"
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	_, ok := out.Files["COPYING"]
	assert.True(t, ok, "single-term must write the substituted body to the declared path")
	_, hasDefault := out.Files["LICENSE"]
	assert.False(t, hasDefault, "declared path replaces the synthesised LICENSE name")
	assert.Contains(t, out.Files, "LICENSES/MIT.txt", "canonical REUSE file still emitted")
}

func TestRenderMultiPathFanOut(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense("MIT OR Apache-2.0")
	pf.License.File = []string{"LICENSE-MIT", "LICENSE-APACHE"}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "LICENSE-MIT", "first term keyed off declared path")
	assert.Contains(t, out.Files, "LICENSE-APACHE", "second term keyed off declared path")
	_, hasOverview := out.Files["LICENSE"]
	assert.False(t, hasOverview, "one declared path per term suppresses the overview LICENSE")
	// LICENSES/<id>.txt is emitted alongside the declared fan-out so `reuse
	// lint` resolves the expression regardless of declared paths. Default
	// mode substitutes the holder into both.
	assert.Contains(t, out.Files, "LICENSES/MIT.txt", "declared fan-out still emits LICENSES file")
	assert.Contains(t, out.Files, "LICENSES/Apache-2.0.txt", "declared fan-out still emits LICENSES file")
	assert.Contains(t, string(out.Files["LICENSE-MIT"]), "Alice Corp",
		"declared path holds the substituted body (holder resolved)")
	assert.Contains(t, string(out.Files["LICENSES/MIT.txt"]), "Alice Corp",
		"LICENSES file carries the substituted holder under default mode")
}

func TestRenderCoversAdditionsSucceeds(t *testing.T) {
	b := license.Bridge{}
	pf := docWithLicense(spdxMIT)
	pf.License.Covers = "additions"
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err, "covers:additions is a signal, not a generation error")
	assert.Contains(t, out.Files, "LICENSE")
	assert.NotContains(t, out.Files, "NOTICE", "v1 mandates no NOTICE/THIRD-PARTY file")
}

func TestRenderRejectsPathTraversal(t *testing.T) {
	b := license.Bridge{}
	for _, bad := range []any{"../escape/LICENSE", "/etc/passwd", `C:\LICENSE`} {
		pf := docWithLicense(spdxMIT)
		pf.License.File = bad
		_, err := b.Render(pf, core.Options{Offline: true})
		assert.Error(t, err, "path %v must be rejected per spec §8", bad)
	}
}

// ── Bridge identity ───────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	b := license.Bridge{}
	assert.Equal(t, "LICENSE", b.Filename())
}

func TestBridgeName(t *testing.T) {
	b := license.Bridge{}
	assert.Equal(t, "license", b.Name())
}

func TestBridgePolicyScaffoldOnce(t *testing.T) {
	b := license.Bridge{}
	assert.True(t, b.Policy().ScaffoldOnce, "LICENSE must use ScaffoldOnce policy")
}
