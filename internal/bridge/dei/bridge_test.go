// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package dei_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/dei"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Repeated literals hoisted to constants to satisfy goconst and keep the
// fixture identity readable at each call site.
const (
	testEnabled   = "enabled"
	testOpenProj  = "open-proj"
	testCommunity = "community"
)

// ── enabled gate (the core contract: DEI.md is opt-in) ──────────────────────

// TestRenderDisabledNoNamespace confirms the bridge emits nothing when the
// namespace is absent entirely.
func TestRenderDisabledNoNamespace(t *testing.T) {
	b := dei.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "gated"}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Empty(t, out.Files, "absent namespace must produce no output")
}

// TestRenderDisabledNamespacePresentOff confirms a present-but-disabled
// namespace is a no-op — enabled must be explicitly true.
func TestRenderDisabledNamespacePresentOff(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: "gated"}},
		map[string]any{testEnabled: false})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Empty(t, out.Files, "enabled=false must produce no output")
}

// ── full render ─────────────────────────────────────────────────────────────

// TestRenderEnabledNoMetricsRendersSamples is the happy path: enabled:true
// with no metrics. The four CHAOSS sections must render their per-template
// SAMPLE bullets so the file is a complete, editable draft out of the box.
func TestRenderEnabledNoMetricsRendersSamples(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: testOpenProj}},
		map[string]any{testEnabled: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, "# Diversity, Equity, and Inclusion Project Statement")
	// All four CHAOSS metric headers present.
	assert.Contains(t, body, "### [Project Access]")
	assert.Contains(t, body, "### [Communication Transparency]")
	assert.Contains(t, body, "### [Newcomer Experiences]")
	assert.Contains(t, body, "### [Inclusive Leadership]")
	// Sample bullets emitted for a metric the project left empty.
	assert.Contains(t, body, "All virtual meetings are transcribed")
	// The optional 'other' section is absent when not declared.
	assert.NotContains(t, body, "### Other Efforts")
	// Last reviewed falls back to the honest placeholder.
	assert.Contains(t, body, "Last Reviewed: [Enter Date]")
}

// TestRenderProvidedMetricsRendered confirms project-declared bullets appear
// verbatim and replace the samples for that metric.
func TestRenderProvidedMetricsRendered(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: testOpenProj}},
		map[string]any{
			testEnabled:     true,
			"last-reviewed": "2026-07-23",
			"metrics": map[string]any{
				"project-access": []any{"Translated meetings.", "Global time zones."},
			},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, "- Translated meetings.")
	assert.Contains(t, body, "- Global time zones.")
	// Samples suppressed for the metric the project filled in.
	assert.NotContains(t, body, "All virtual meetings are transcribed")
	// An undeclared metric still shows its samples.
	assert.Contains(t, body, "All project documentation is publicly available")
	// Declared last-reviewed stamps the line.
	assert.Contains(t, body, "Last Reviewed: 2026-07-23")
}

// TestRenderOtherSection confirms the optional 'other' section renders only
// when explicitly declared.
func TestRenderOtherSection(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: testOpenProj}},
		map[string]any{
			testEnabled: true,
			"metrics": map[string]any{
				"other": []any{"Accessibility audit in progress."},
			},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, "### Other Efforts")
	assert.Contains(t, body, "- Accessibility audit in progress.")
}

// ── contact resolution ──────────────────────────────────────────────────────

// TestRenderContactFromCommunityRole confirms the reporting contact is
// resolved from a person with the 'community' role.
func TestRenderContactFromCommunityRole(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{
		Identity: projectfile.Identity{Name: testOpenProj},
		People: []projectfile.Person{{
			GivenNames:  "Mod",
			FamilyNames: "Erator",
			Email:       "conduct@example.org",
			Roles:       []string{testCommunity},
		}},
	}, map[string]any{testEnabled: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, "<conduct@example.org>")
	assert.NotContains(t, body, "[provide a reporting link")
}

// TestRenderContactMissingEmitsPlaceholder confirms an unresolvable contact
// degrades to a placeholder rather than failing generation.
func TestRenderContactMissingEmitsPlaceholder(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: testOpenProj}},
		map[string]any{testEnabled: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, "[provide a reporting link")
}

// ── i18n ────────────────────────────────────────────────────────────────────

// TestRenderLocalizedVariants confirms declared i18n languages render sibling
// files when templates exist (es, uk both ship).
func TestRenderLocalizedVariants(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: testOpenProj}},
		map[string]any{testEnabled: true})
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{
		"languages": []any{"es", "uk"},
	})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "DEI.md")
	assert.Contains(t, out.Files, "docs/es/DEI.md")
	assert.Contains(t, out.Files, "docs/uk/DEI.md")
	assert.Contains(t, string(out.Files["docs/es/DEI.md"]), "Declaración de Diversidad")
	assert.Contains(t, string(out.Files["docs/uk/DEI.md"]), "Заява про різноманітність")
	// Translated prose trips the English-only dictionary rules (owned by core),
	// so every non-English variant is bracketed with a disable/enable pair while
	// the canonical English file is not; this only asserts the bracket.
	assert.Contains(t, string(out.Files["docs/es/DEI.md"]), "<!-- textlint-disable ")
	assert.Contains(t, string(out.Files["docs/es/DEI.md"]), "<!-- textlint-enable -->")
	assert.Contains(t, string(out.Files["docs/uk/DEI.md"]), "<!-- textlint-disable ")
	assert.NotContains(t, string(out.Files["DEI.md"]), "textlint-disable", "English file must not be wrapped")
}

// ── bridge identity ─────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	assert.Equal(t, "DEI.md", dei.Bridge{}.Filename())
}

func TestBridgePolicyMarker(t *testing.T) {
	assert.True(t, dei.Bridge{}.Policy().Marker, "DEI.md must use the Marker policy")
}

func TestBridgeRenderIsMarkerDoc(t *testing.T) {
	b := dei.Bridge{}
	pf := withDEI(&projectfile.Document{Identity: projectfile.Identity{Name: "p"}},
		map[string]any{testEnabled: true})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	require.Contains(t, out.Files, "DEI.md")
	// RenderLocalized folds the sentinel into the SPDX header comment for a
	// Marker-policy bridge, so the file opens with one comment carrying both.
	body := string(out.Files["DEI.md"])
	assert.Contains(t, body, core.MarkerInner+"\n-->",
		"DEI.md must carry the sentinel folded into the SPDX header")
	assert.NotContains(t, body, core.MarkerHTML,
		"the standalone marker comment must not survive the fold")
	assert.True(t, core.HasMarker([]byte(body)),
		"the folded header must stay detectable as managed")
}

// withDEI attaches an org.projectfile.dei extension to pf.
func withDEI(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.DEIExtensionNS, ext)
	}
	return pf
}

// TestBridgeIdentitySurface pins the registry-facing identity methods argv dispatch reads.
func TestBridgeIdentitySurface(t *testing.T) {
	b := dei.Bridge{}
	assert.Equal(t, "dei", b.Name())
	label, kind := b.Labels()
	assert.Equal(t, b.Filename(), label, "the label is the managed filename")
	assert.Equal(t, "projectfile", kind)
	assert.Nil(t, b.Aliases(), "DEI.md is reached only by its one filename")
	assert.Nil(t, b.RequiredFields(nil), "a marker doc owns no required field")
}

// TestBridgePathAndExists drives Exists across the absent and the written file.
func TestBridgePathAndExists(t *testing.T) {
	pointUserConfigEmpty(t)
	b := dei.Bridge{}
	dir := t.TempDir()
	filename := b.Filename()
	assert.False(t, b.Exists(dir), "an empty directory holds no DEI.md")
	assert.Equal(t, filepath.Join(dir, filename), b.FullPath(dir, nil))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte("marker"), 0o644))
	assert.True(t, b.Exists(dir), "the written marker must be detected")
}

// TestRenderScalarNamespaceFails rejects a namespace written as a bare scalar.
func TestRenderScalarNamespaceFails(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "flat"}}
	projectfile.SetExtension(pf, pfmodel.DEIExtensionNS, testEnabled)
	_, err := dei.Bridge{}.Render(pf, core.Options{Offline: true})
	require.Error(t, err, "a scalar namespace is a malformed extension, not an opt-out")
	assert.Contains(t, err.Error(), "is not a map")
}

// pointUserConfigEmpty aims Load at an empty tree so the developer's own config cannot steer the path.
func pointUserConfigEmpty(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}
