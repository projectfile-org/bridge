// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package llm_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/llm"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Repeated literals hoisted to constants (goconst) and to keep fixture intent
// readable at each call site.
const (
	testProject    = "open-proj"
	testCommunity  = "community"
	testAttitude   = "attitude"
	testAllowed    = "allowed"
	testProhibited = "prohibited"
	testEncouraged = "encouraged"
)

// ── absence is not permission (the core contract) ───────────────────────────

// TestRenderAbsentNamespaceEmitsNothing confirms a project that never declared
// org.projectfile.llm gets no LLM.md — absence must never render as any
// particular stance, permissive or otherwise.
func TestRenderAbsentNamespaceEmitsNothing(t *testing.T) {
	b := llm.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: testProject}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Empty(t, out.Files, "absent namespace must produce no output")
}

// ── activity table: only overrides reach it ──────────────────────────────────

// TestRenderActivityEqualToAttitudeNoRow confirms an activity whose declared
// stance equals attitude produces no table at all — a row that repeats the
// default is noise, not information.
func TestRenderActivityEqualToAttitudeNoRow(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "pull-requests": testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.NotContains(t, body, "| Activity | Stance |", "no table when every activity matches attitude")
}

// TestRenderActivityDifferingFromAttitudeRendersRow confirms an activity whose
// stance differs from attitude gets its own table row.
func TestRenderActivityDifferingFromAttitudeRendersRow(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "security-reports": testProhibited})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "| Security reports | Prohibited |")
}

// TestRenderUnknownActivityKeySurvives confirms an activity key outside the
// canonical vocabulary still reaches the table under its own raw name — a new
// activity needs no Go edit.
func TestRenderUnknownActivityKeySurvives(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: "neutral", "code-review": testEncouraged})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "| code-review | Encouraged |")
}

// ── determinism ───────────────────────────────────────────────────────────

// TestRenderTwoRendersByteIdentical confirms a multi-activity document renders
// the same bytes every time — the canonical/alphabetical ordering rule must
// not leave any Go map iteration order visible in the output.
func TestRenderTwoRendersByteIdentical(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:       testAllowed,
			"security-reports": testProhibited,
			"translations":     testEncouraged,
			"bug-reports":      "discouraged",
			"code-review":      testProhibited,
			"another-unknown":  testEncouraged,
		})
	first, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	second, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, first.Files["LLM.md"], second.Files["LLM.md"])
}

// ── content signals: absence is stated, never implied ────────────────────────

// TestRenderContentSignalsAbsentStatesAbsence confirms the content section
// always renders, even with nothing declared, and says so explicitly.
func TestRenderContentSignalsAbsentStatesAbsence(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "No content signals are declared")
}

// TestRenderContentSignalsDedupPreservesOrder confirms declared content
// signals render as bullets in declared order with duplicates dropped.
func TestRenderContentSignalsDedupPreservesOrder(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:      testAllowed,
			"content-signals": []any{"search", "ai-input", "search"},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "`search` — this content may appear in AI-powered search results.")
	assert.Contains(t, body, "`ai-input` — this content may be used as input to an AI system at inference time.")
	assert.Equal(t, 1, strings.Count(body, "`search`"), "duplicate signal must render once")
}

// ── disclosure ────────────────────────────────────────────────────────────

// TestRenderDiscloseRequiredWithTrailer confirms the disclosure paragraph and
// trailer block render only when disclose-required is true.
func TestRenderDiscloseRequiredWithTrailer(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:        testAllowed,
			"disclose-required": true,
			"disclose-trailer":  "Assisted-by",
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "must disclose")
	assert.Contains(t, body, "Assisted-by: <model or tool name>")
}

// TestRenderDiscloseNotRequiredOmitsParagraph confirms the disclosure block is
// absent when disclose-required is unset (default false).
func TestRenderDiscloseNotRequiredOmitsParagraph(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.NotContains(t, body, "must disclose")
}

// ── statement + policy link ──────────────────────────────────────────────────

// TestRenderStatementRendersVerbatim confirms a declared statement renders
// under the H1, unmodified.
func TestRenderStatementRendersVerbatim(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "statement": "We review every patch the same way regardless of how it was written."})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "We review every patch the same way regardless of how it was written.")
}

// TestRenderPolicyURLFromLinks confirms a links[type=ai-policy] entry surfaces
// as the "Full policy" pointer.
func TestRenderPolicyURLFromLinks(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{
		Identity: projectfile.Identity{Name: testProject},
		Links:    []projectfile.Link{{Type: "ai-policy", URL: "https://example.org/ai-policy"}},
	}, map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "Full policy: <https://example.org/ai-policy>")
}

// ── contact resolution ──────────────────────────────────────────────────────

// TestRenderContactFromCommunityRole confirms the Questions section resolves
// the reporting contact from a person with the 'community' role.
func TestRenderContactFromCommunityRole(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{
		Identity: projectfile.Identity{Name: testProject},
		People: []projectfile.Person{{
			GivenNames:  "Mod",
			FamilyNames: "Erator",
			Email:       "policy@example.org",
			Roles:       []string{testCommunity},
		}},
	}, map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "<policy@example.org>")
}

// TestRenderContactMissingFallsBackToSupportFile confirms an unresolvable
// contact degrades to the SUPPORT.md pointer rather than failing generation.
func TestRenderContactMissingFallsBackToSupportFile(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files["LLM.md"])
	assert.Contains(t, body, "SUPPORT.md")
}

// ── i18n ────────────────────────────────────────────────────────────────────

// TestRenderLocalizedVariants confirms declared i18n languages render sibling
// files when templates exist (es, uk both ship).
func TestRenderLocalizedVariants(t *testing.T) {
	b := llm.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{
		"languages": []any{"es", "uk"},
	})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "LLM.md")
	assert.Contains(t, out.Files, "docs/es/LLM.md")
	assert.Contains(t, out.Files, "docs/uk/LLM.md")
	assert.Contains(t, string(out.Files["docs/es/LLM.md"]), "Política sobre IA y LLM")
	assert.Contains(t, string(out.Files["docs/uk/LLM.md"]), "Політика щодо ШІ та LLM")
}

// ── bridge identity ─────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	assert.Equal(t, "LLM.md", llm.Bridge{}.Filename())
}

func TestBridgeAliases(t *testing.T) {
	assert.Equal(t, []string{"AI.md"}, llm.Bridge{}.Aliases())
}

func TestBridgePolicyMarker(t *testing.T) {
	assert.True(t, llm.Bridge{}.Policy().Marker, "LLM.md must use the Marker policy")
	assert.False(t, llm.Bridge{}.Policy().ScaffoldOnce, "LLM.md is a projection, never scaffold-once")
}

// withLLM attaches an org.projectfile.llm extension to pf.
func withLLM(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.LLMExtensionNS, ext)
	}
	return pf
}
