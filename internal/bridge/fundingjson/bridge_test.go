// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// ── identity surface ────────────────────────────────────────────────────────

func TestBridgeNameAndFilename(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "fundingjson", b.Name())
	assert.Equal(t, "funding.json", b.Filename())
	assert.Nil(t, b.Aliases(), "funding.json has no alternative spelling")
	ext, pf := b.Labels()
	assert.Equal(t, "funding.json", ext)
	assert.Equal(t, "projectfile", pf)
}

// A pure-data JSON artefact carries no marker: JSON cannot hold a comment.
func TestBridgePolicyHasNoMarker(t *testing.T) {
	assert.False(t, Bridge{}.Policy().Marker)
}

func TestBridgeExistsReportsFilePresence(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, Bridge{}.Exists(dir), "empty dir must report absent")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "funding.json"), []byte("{}"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "written file must report present")
}

// The user config path override wins; a config with no entry keeps the filename.
func TestBridgeFullPathHonoursUserConfig(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{"funding.json": map[string]any{"path": "assets/funding.json"}}}})
	assert.Equal(t, filepath.Join(dir, "assets", "funding.json"), Bridge{}.FullPath(dir, nil))

	pointUserConfigAt(t, nil)
	assert.Equal(t, filepath.Join(dir, "funding.json"), Bridge{}.FullPath(dir, nil))
}

// ── happy path ──────────────────────────────────────────────────────────────

func TestRenderFullDocument(t *testing.T) {
	out, err := Bridge{}.Render(fullDoc(), core.Options{Offline: true})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)

	body := out.Files["funding.json"]
	assert.Equal(t, byte('\n'), body[len(body)-1], "rendered file must end with a newline")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc), "output must be valid JSON")
	assert.Equal(t, "1.1.0", doc["version"])

	entity, ok := doc["entity"].(map[string]any)
	require.True(t, ok, "entity must be an object")
	assert.Equal(t, "individual", entity[keyType])
	assert.Equal(t, "maintainer", entity["role"])
	assert.Equal(t, "Ada Lovelace", entity["name"])
	assert.Equal(t, fxEmail, entity["email"])
	assert.Equal(t, fxSummary, entity["description"])
	assert.Equal(t, map[string]any{"url": fxHomepage}, entity["webpageUrl"])

	projects, ok := doc["projects"].([]any)
	require.True(t, ok, "projects must be an array")
	require.Len(t, projects, 1)
	proj, ok := projects[0].(map[string]any)
	require.True(t, ok, "project must be an object")
	assert.Equal(t, fxName, proj[keyGUID])
	assert.Equal(t, fxTitle, proj["name"])
	assert.Equal(t, []any{"spdx:" + fxSpdx}, proj["licenses"])
	assert.Equal(t, []any{"go", "tooling", "build", "lint", "test", "ci", "dev", "ops", "extra"}, proj["tags"])

	funding, ok := doc["funding"].(map[string]any)
	require.True(t, ok, "funding must be an object")
	require.Len(t, funding[keyChannels], 1)
	require.Len(t, funding[keyPlans], 1)
	require.Len(t, funding["history"], 1)
}

// Two renders must be byte-identical: the file feeds a drift gate.
func TestRenderIsDeterministic(t *testing.T) {
	first, err := Bridge{}.Render(fullDoc(), core.Options{Offline: true})
	require.NoError(t, err)
	for range 8 {
		again, err := Bridge{}.Render(fullDoc(), core.Options{Offline: true})
		require.NoError(t, err)
		assert.Equal(t, first.Files["funding.json"], again.Files["funding.json"])
	}
}

// Without a resolvable project name the projects[] section disappears.
func TestRenderOmitsProjectWithoutDisplayName(t *testing.T) {
	out, err := Bridge{}.Render(withFunding(noProjectDoc(), fullExt()), core.Options{Offline: true})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Files["funding.json"], &doc))
	assert.NotContains(t, doc, "projects", "a document with no project name renders no projects")
}

// An empty history renders as an absent key, not an empty array.
func TestRenderOmitsEmptyHistory(t *testing.T) {
	out, err := Bridge{}.Render(withFunding(fullDoc(), fullExt()), core.Options{Offline: true})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Files["funding.json"], &doc))
	funding, ok := doc["funding"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, funding, "history")
}

// ── stated failures ─────────────────────────────────────────────────────────

// No funding namespace reads as an empty extension, not a nil dereference.
func TestRenderFailsWithoutFundingNamespace(t *testing.T) {
	_, err := Bridge{}.Render(barePersonDoc(), core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no channels configured")
}

func TestRenderFailsWithoutChannels(t *testing.T) {
	_, err := Bridge{}.Render(withFunding(barePersonDoc(), map[string]any{keyPlans: onePlan()}), core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no channels configured")
}

func TestRenderFailsWithoutPlans(t *testing.T) {
	_, err := Bridge{}.Render(withFunding(barePersonDoc(), map[string]any{keyChannels: []any{oneChannel()}}), core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plans configured")
}

func TestRenderFailsOnMalformedExtension(t *testing.T) {
	pf := barePersonDoc()
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, "not-a-map")
	_, err := Bridge{}.Render(pf, core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "org.projectfile.funding is not a map")
}

func TestRenderFailsWithoutEntityEmail(t *testing.T) {
	pf := withFunding(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People:   []projectfile.Person{{GivenNames: fxGiven, FamilyNames: fxFamily}},
	}, fullExt())
	_, err := Bridge{}.Render(pf, core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resolve entity")
}

func TestRenderFailsWithoutHomepageLink(t *testing.T) {
	pf := withFunding(&projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People:   []projectfile.Person{{Email: fxEmail}},
	}, fullExt())
	_, err := Bridge{}.Render(pf, core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity.webpageUrl is required")
}

// The embedded schema is the last gate: a bad enum fails the render.
func TestRenderFailsSchemaValidation(t *testing.T) {
	_, err := Bridge{}.Render(withFunding(barePersonDoc(), map[string]any{
		keyChannels: []any{map[string]any{keyGUID: valStripe, keyType: "not-a-channel-type"}},
		keyPlans:    []any{onePlan()},
	}), core.Options{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
}

// ── extension overrides ─────────────────────────────────────────────────────

func TestRenderHonoursEntityOverrides(t *testing.T) {
	pf := fullDoc()
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, fundingOverrides(map[string]any{"entity-type": "group", "entity-role": fxSteward}))
	out, err := Bridge{}.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Files["funding.json"], &doc))
	entity, ok := doc["entity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "group", entity[keyType], "entity-type must override the resolved type")
	assert.Equal(t, fxSteward, entity["role"], "entity-role must override the resolved role")
}

// pointUserConfigAt aims $XDG_CONFIG_HOME at ext (empty tree when nil) and reloads.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "projectfile"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile", "cli.json"), body, 0o644))
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

func oneChannel() map[string]any {
	return map[string]any{keyGUID: valStripe, keyType: valPaymentProvider}
}

func onePlan() map[string]any {
	return map[string]any{keyGUID: valSponsorGUID, "status": valActive, "name": valChannelName, "amount": 10.0, keyCurrency: valUSD, "frequency": valMonthly, keyChannels: []any{valStripe}}
}
