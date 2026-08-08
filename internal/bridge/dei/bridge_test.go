// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package dei_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/dei"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
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
	// RenderLocalized prepends the REUSE header (an SPDX HTML comment) before
	// the body, so the pf-cli marker is the second comment in the file — it
	// must be present, but not necessarily at offset 0.
	assert.Contains(t, string(out.Files["DEI.md"]), core.MarkerHTML,
		"DEI.md must carry the pf-cli marker comment")
}

// withDEI attaches an org.projectfile.dei extension to pf.
func withDEI(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.DEIExtensionNS, ext)
	}
	return pf
}
