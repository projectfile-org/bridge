// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package aipolicy_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/ai-policy"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
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
	testActivities = "activities"
	testFile       = "AI_POLICY.md"
	testAltFile    = "AI.md"
	testFilename   = "filename"
	testNoAI       = "none"
)

// ── absence is not permission (the core contract) ───────────────────────────

// TestRenderAbsentNamespaceEmitsNothing confirms a project that never declared
// org.projectfile.ai gets no policy file — absence must never render as any
// particular stance, permissive or otherwise.
func TestRenderAbsentNamespaceEmitsNothing(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: testProject}}
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Empty(t, out.Files, "absent namespace must produce no output")
}

// ── the file the project names ──────────────────────────────────────────────

// TestRenderDefaultFilename confirms a document that declares no filename
// renders AI_POLICY.md.
func TestRenderDefaultFilename(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, testFile)
}

// TestRenderFilenameOverride confirms a declared filename moves the output and
// nothing else: the prose still comes from the canonical template, which is
// why LocalizedSpec carries Template separately from Filename.
func TestRenderFilenameOverride(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testFilename: testAltFile})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, testAltFile)
	assert.NotContains(t, out.Files, testFile)
	assert.Contains(t, string(out.Files[testAltFile]), "# AI and LLM Policy")
}

// TestRenderFilenameWithPathRejected confirms a filename carrying a path is an
// error, not a sanitized basename: this field names a file a tool writes.
func TestRenderFilenameWithPathRejected(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testFilename: "../../etc/AI_POLICY.md"})
	_, err := b.Render(pf, core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a basename")
}

// ── activity table: only overrides reach it ──────────────────────────────────

// TestRenderActivityEqualToAttitudeNoRow confirms an activity whose declared
// stance equals attitude produces no table at all — a row that repeats the
// default is noise, not information.
func TestRenderActivityEqualToAttitudeNoRow(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testActivities: map[string]any{"pull-requests": testAllowed}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.NotContains(t, body, "| Activity | Stance |", "no table when every activity matches attitude")
}

// TestRenderActivityDifferingFromAttitudeRendersRow confirms an activity whose
// stance differs from attitude gets its own table row.
func TestRenderActivityDifferingFromAttitudeRendersRow(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testActivities: map[string]any{"security-reports": testProhibited}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "| Security reports | Prohibited |")
}

// TestRenderMediaActivitiesSplit confirms the question one media flag could
// never answer: images banned, audio allowed, in the same document.
func TestRenderMediaActivitiesSplit(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testActivities: map[string]any{
			"images": testProhibited,
			"video":  testProhibited,
			"audio":  testAllowed,
		}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "| Images | Prohibited |")
	assert.Contains(t, body, "| Video | Prohibited |")
	assert.NotContains(t, body, "| Audio |", "audio equals attitude, so it states nothing new")
}

// TestRenderUnknownActivityKeySurvives confirms an activity key outside the
// canonical vocabulary still reaches the table under its own raw name — a new
// activity needs no Go edit.
func TestRenderUnknownActivityKeySurvives(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: "neutral", testActivities: map[string]any{"benchmarks": testEncouraged}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "| benchmarks | Encouraged |")
}

// ── the internal direction ──────────────────────────────────────────────────

// TestRenderProjectUseRendersEveryEntry confirms project-use rows all render:
// unlike an activity, an entry here has no document-level default to repeat.
func TestRenderProjectUseRendersEveryEntry(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "project-use": map[string]any{
			"pull-requests": "assisted",
			"releases":      testNoAI,
		}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "## How this project uses AI")
	assert.Contains(t, body, "| Pull requests | A human, with AI assistance |")
	assert.Contains(t, body, "| Releases | A human, with no AI involvement |")
}

// TestRenderProjectUseAbsentOmitsSection confirms the internal section is
// dropped whole when the project said nothing about its own practice.
func TestRenderProjectUseAbsentOmitsSection(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[testFile]), "## How this project uses AI")
}

// ── enforcement, obligations, gates ─────────────────────────────────────────

// TestRenderEnforcementKeepsDeclaredOrder confirms the escalation ladder is
// rendered in the order the project declared it — the order IS the meaning.
func TestRenderEnforcementKeepsDeclaredOrder(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "enforcement": []any{"warn", "revoke", "ban"}})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "## When this policy is not followed")
	warn := strings.Index(body, "A first violation gets a warning.")
	revoke := strings.Index(body, "Commit or review privileges are withdrawn.")
	ban := strings.Index(body, "The account is barred from contributing again.")
	assert.Less(t, warn, revoke)
	assert.Less(t, revoke, ban)
}

// TestRenderObligationsAndGates confirms the duties and the two contribution
// gates render from their own fields.
func TestRenderObligationsAndGates(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:      testAllowed,
			"obligations":     []any{"understand", "test"},
			"issue-required":  true,
			"excluded-labels": []any{"good first issue"},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "**Understand it.**")
	assert.Contains(t, body, "**Run it.**")
	assert.Contains(t, body, "must reference an issue this project has already accepted")
	assert.Contains(t, body, "`good first issue`")
}

// TestRenderAppliesToContributorsStatesExemption confirms a maintainer
// exemption is stated rather than left for a reader to discover in the log.
func TestRenderAppliesToContributorsStatesExemption(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "applies-to": "contributors"})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[testFile]), "bind contributions from outside the project")
}

// TestRenderAppliesToDefaultBindsEveryone confirms the exemption sentence is
// absent by default: an exemption nobody declared is not one to render.
func TestRenderAppliesToDefaultBindsEveryone(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[testFile]), "bind contributions from outside the project")
}

// ── determinism ───────────────────────────────────────────────────────────

// TestRenderTwoRendersByteIdentical confirms a multi-activity document renders
// the same bytes every time — the canonical/alphabetical ordering rule must
// not leave any Go map iteration order visible in the output.
func TestRenderTwoRendersByteIdentical(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude: testAllowed,
			testActivities: map[string]any{
				"security-reports": testProhibited,
				"translations":     testEncouraged,
				"bug-reports":      "discouraged",
				"code-review":      testProhibited,
				"another-unknown":  testEncouraged,
			},
			"project-use": map[string]any{
				"merges":   testNoAI,
				"releases": testNoAI,
				"triage":   "assisted",
			},
		})
	first, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	second, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, first.Files[testFile], second.Files[testFile])
}

// ── content signals: absence is stated, never implied ────────────────────────

// TestRenderContentSignalsAbsentStatesAbsence confirms the content section
// always renders, even with nothing declared, and says so explicitly.
func TestRenderContentSignalsAbsentStatesAbsence(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "No content signals are declared")
}

// TestRenderContentSignalsDedupPreservesOrder confirms declared content
// signals render as bullets in declared order with duplicates dropped.
func TestRenderContentSignalsDedupPreservesOrder(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:      testAllowed,
			"content-signals": []any{"search", "ai-input", "search"},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "`search` — this content may appear in AI-powered search results.")
	assert.Contains(t, body, "`ai-input` — this content may be used as input to an AI system at inference time.")
	assert.Equal(t, 1, strings.Count(body, "`search`"), "duplicate signal must render once")
}

// ── disclosure ────────────────────────────────────────────────────────────

// TestRenderDiscloseRequiredWithTrailer confirms the disclosure paragraph, the
// detail list and the trailer block render only when disclose-required is true.
func TestRenderDiscloseRequiredWithTrailer(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{
			testAttitude:        testAllowed,
			"disclose-required": true,
			"disclose-trailer":  "Assisted-by",
			"disclose-details":  []any{"tool", "extent"},
		})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "must disclose")
	assert.Contains(t, body, "the tool used, by name")
	assert.Contains(t, body, "how much of the work the tool did.")
	assert.Contains(t, body, "Assisted-by: <model or tool name>")
}

// TestRenderDiscloseNotRequiredOmitsParagraph confirms the disclosure block is
// absent when disclose-required is unset (default false).
func TestRenderDiscloseNotRequiredOmitsParagraph(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.NotContains(t, body, "must disclose")
}

// ── statement + policy link ──────────────────────────────────────────────────

// TestRenderStatementRendersVerbatim confirms a declared statement renders
// under the H1, unmodified.
func TestRenderStatementRendersVerbatim(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, "statement": "We review every patch the same way regardless of how it was written."})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "We review every patch the same way regardless of how it was written.")
}

// TestRenderPolicyURLFromLinks confirms a links[type=ai-policy] entry surfaces
// as the "Full policy" pointer.
func TestRenderPolicyURLFromLinks(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{
		Identity: projectfile.Identity{Name: testProject},
		Links:    []projectfile.Link{{Type: "ai-policy", URL: "https://example.org/ai-policy"}},
	}, map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "Full policy: <https://example.org/ai-policy>")
}

// ── contact resolution ──────────────────────────────────────────────────────

// TestRenderContactFromCommunityRole confirms the Questions section resolves
// the reporting contact from a person with the 'community' role.
func TestRenderContactFromCommunityRole(t *testing.T) {
	b := aipolicy.Bridge{}
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
	body := string(out.Files[testFile])
	assert.Contains(t, body, "<policy@example.org>")
}

// TestRenderContactMissingFallsBackToSupportFile confirms an unresolvable
// contact degrades to the SUPPORT.md pointer rather than failing generation.
func TestRenderContactMissingFallsBackToSupportFile(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	body := string(out.Files[testFile])
	assert.Contains(t, body, "SUPPORT.md")
}

// ── i18n ────────────────────────────────────────────────────────────────────

// TestRenderLocalizedVariants confirms declared i18n languages render sibling
// files when templates exist (es, uk both ship).
func TestRenderLocalizedVariants(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed})
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{
		"languages": []any{"es", "uk"},
	})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, testFile)
	assert.Contains(t, out.Files, "docs/es/"+testFile)
	assert.Contains(t, out.Files, "docs/uk/"+testFile)
	assert.Contains(t, string(out.Files["docs/es/"+testFile]), "Política sobre IA y LLM")
	assert.Contains(t, string(out.Files["docs/uk/"+testFile]), "Політика щодо ШІ та LLM")
}

// TestRenderLocalizedVariantsFollowFilename confirms a renamed policy keeps its
// name in every language: the locale lives in the directory, not the basename.
func TestRenderLocalizedVariantsFollowFilename(t *testing.T) {
	b := aipolicy.Bridge{}
	pf := withLLM(&projectfile.Document{Identity: projectfile.Identity{Name: testProject}},
		map[string]any{testAttitude: testAllowed, testFilename: testAltFile})
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{
		"languages": []any{"es"},
	})
	out, err := b.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, out.Files, "docs/es/"+testAltFile)
	assert.Contains(t, string(out.Files["docs/es/"+testAltFile]), "Política sobre IA y LLM")
}

// ── bridge identity ─────────────────────────────────────────────────────────

func TestBridgeFilename(t *testing.T) {
	assert.Equal(t, testFile, aipolicy.Bridge{}.Filename())
}

func TestBridgeAliases(t *testing.T) {
	assert.Equal(t, []string{"AI.md", "AI-POLICY.md", "LLM.md"}, aipolicy.Bridge{}.Aliases())
}

func TestBridgePolicyMarker(t *testing.T) {
	assert.True(t, aipolicy.Bridge{}.Policy().Marker, "the policy file must use the Marker policy")
}

// withLLM attaches an org.projectfile.ai extension to pf.
func withLLM(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	if len(ext) > 0 {
		projectfile.SetExtension(pf, pfmodel.AIExtensionNS, ext)
	}
	return pf
}
