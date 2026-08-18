// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Live-path coverage: fetchParent is stubbed, so these tests exercise the
// online sections pipeline — fetch, per-language fallback, stale-committed
// replacement, outage preservation — without a forge.

const (
	itestSPDX       = "<!--\nSPDX-FileCopyrightText: 2026 Upstream\nSPDX-License-Identifier: MIT\n-->"
	itestURL        = "https://example.test/b19/ubuntu"
	itestRef        = "2.0.0"
	outFeaturesName = "FEATURES.md"
)

// itestProject writes a projectfile with documents + parents and i18n es/uk.
func itestProject(t *testing.T, dir string, withParents bool) *projectfile.Document {
	t.Helper()
	parents := ""
	if withParents {
		parents = "          parents:\n            - {url: " + itestURL + "}\n"
	}
	pf := "---\n" +
		"# SPDX-FileCopyrightText: 2026 Tester\n#\n# SPDX-License-Identifier: MIT\n" +
		"$schema: https://projectfile.org/schema/v1.json\n" +
		"identity:\n  name: " + filepath.Base(dir) + "\n" +
		"org:\n  projectfile:\n" +
		"    i18n:\n      languages:\n        - es\n        - uk\n" +
		"    fragments:\n      documents:\n" +
		"        - dir: docs/features.d\n          out: FEATURES.md\n          title: Features\n" + parents
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(pf), 0o644))
	pfPath, err := projectfile.DetectPath(dir)
	require.NoError(t, err)
	doc, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	return doc
}

func itestFrag(t *testing.T, dir, fragDir, h1, body string) {
	t.Helper()
	full := filepath.Join(dir, fragDir)
	require.NoError(t, os.MkdirAll(full, 0o755))
	name := strings.ToLower(strings.ReplaceAll(strings.Fields(h1)[0], "ó", "o")) + ".md"
	content := itestSPDX + "\n\n# " + h1 + "\n\n" + body + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(full, name), []byte(content), 0o644))
}

// itestStub replaces fetchParent for one test and restores it after.
func itestStub(t *testing.T, fn func(parent pfmodel.FragmentParent, document string, langs []string) (inheritedCopy, map[string]inheritedCopy, error)) {
	t.Helper()
	orig := fetchParent
	fetchParent = func(_ context.Context, parent pfmodel.FragmentParent, document string, langs []string) (inheritedCopy, map[string]inheritedCopy, error) {
		return fn(parent, document, langs)
	}
	t.Cleanup(func() { fetchParent = orig })
}

// TestLiveFetchRendersFetchedSectionsAndReplacesCommitted: an online generate
// assembles what the parents publish NOW — the committed document's stale
// section is replaced, not merged.
func TestLiveFetchRendersFetchedSectionsAndReplacesCommitted(t *testing.T) {
	dir := t.TempDir()
	doc := itestProject(t, dir, true)
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	itestFrag(t, dir, "docs/es/features.d", "Característica Propia", "Cuerpo propio.")
	itestFrag(t, dir, "docs/uk/features.d", "Власна Можливість", "Власний текст.")
	stale := itestSPDX + "\n\n# Features\n\n## Project Features\n\n### Own Feature\n\nOwn body.\n\n" +
		"## Inherited from b19/ubuntu 1.0.0\n\n### Stale Feature\n\nStale body.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, outFeaturesName), []byte(stale), 0o644))

	itestStub(t, func(_ pfmodel.FragmentParent, _ string, langs []string) (inheritedCopy, map[string]inheritedCopy, error) {
		canonical := inheritedCopy{
			Name: testParent, Title: testParentTitle, URL: itestURL, Ref: itestRef,
			SPDX: itestSPDX, Body: "### Fetched Feature\n\nFetched body.",
		}
		translated := map[string]inheritedCopy{}
		for _, lang := range langs {
			translated[lang] = canonical
		}
		return canonical, translated, nil
	})

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	root := string(out.Files[outFeaturesName])
	assert.Contains(t, root, "## Inherited from B19/Ubuntu 2.0.0", "live heading names the fetched version")
	assert.Contains(t, root, "### Fetched Feature")
	assert.NotContains(t, root, "Stale Feature", "a successful fetch replaces the committed sections")
	es := string(out.Files["docs/es/FEATURES.md"])
	assert.Contains(t, es, "## Heredado de B19/Ubuntu 2.0.0")
	uk := string(out.Files["docs/uk/FEATURES.md"])
	assert.Contains(t, uk, "## Успадковано від B19/Ubuntu 2.0.0")
}

// TestLiveFetchFailureKeepsCommittedSections: an unreachable parent must
// preserve the committed sections — a forge outage degrades to "as fresh as
// last time" instead of deleting content — and never mix fresh and stale.
func TestLiveFetchFailureKeepsCommittedSections(t *testing.T) {
	dir := t.TempDir()
	doc := itestProject(t, dir, true)
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	committed := itestSPDX + "\n\n# Features\n\n## Project Features\n\n### Own Feature\n\nOwn body.\n\n" +
		"## Inherited from b19/ubuntu 1.0.0\n\n### Committed Feature\n\nCommitted body.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, outFeaturesName), []byte(committed), 0o644))

	itestStub(t, func(pfmodel.FragmentParent, string, []string) (inheritedCopy, map[string]inheritedCopy, error) {
		return inheritedCopy{}, nil, errors.New("forge unreachable")
	})

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	root := string(out.Files[outFeaturesName])
	assert.Contains(t, root, "## Inherited from b19/ubuntu 1.0.0")
	assert.Contains(t, root, "### Committed Feature", "outage keeps the committed sections")
}

// TestLiveOfflineGenerateKeepsCommittedSections: --offline is the same
// preservation, chosen up front.
func TestLiveOfflineGenerateKeepsCommittedSections(t *testing.T) {
	dir := t.TempDir()
	doc := itestProject(t, dir, true)
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	committed := itestSPDX + "\n\n# Features\n\n## Project Features\n\n### Own Feature\n\nOwn body.\n\n" +
		"## Inherited from b19/ubuntu 1.0.0\n\n### Committed Feature\n\nCommitted body.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, outFeaturesName), []byte(committed), 0o644))

	called := false
	itestStub(t, func(pfmodel.FragmentParent, string, []string) (inheritedCopy, map[string]inheritedCopy, error) {
		called = true
		return inheritedCopy{}, nil, nil
	})

	out, err := Bridge{}.Render(doc, core.Options{Dir: dir, Offline: true})
	require.NoError(t, err)
	assert.False(t, called, "offline never reaches the forge")
	assert.Contains(t, string(out.Files[outFeaturesName]), "### Committed Feature")
}

// TestOnlineRenderRoundTripsByteStable is the drift-gate contract: what an
// online generate writes, an offline run must reproduce byte for byte —
// canonical and every localized variant — or every consumer drifts on the
// next check.
func TestOnlineRenderRoundTripsByteStable(t *testing.T) {
	dir := t.TempDir()
	doc := itestProject(t, dir, true)
	itestFrag(t, dir, "docs/features.d", "Own Feature", "Own body.")
	itestFrag(t, dir, "docs/es/features.d", "Característica Propia", "Cuerpo propio.")
	itestFrag(t, dir, "docs/uk/features.d", "Власна Можливість", "Власний текст.")

	itestStub(t, func(_ pfmodel.FragmentParent, _ string, _ []string) (inheritedCopy, map[string]inheritedCopy, error) {
		canonical := inheritedCopy{
			Name: testParent, Title: testParentTitle, URL: itestURL, Ref: itestRef,
			SPDX: itestSPDX, Body: "### Fetched Feature\n\nFetched body.",
		}
		// Spanish published, Ukrainian not — the fallback path rides along.
		return canonical, map[string]inheritedCopy{"es": {
			Name: testParent, Title: testParentTitle, URL: itestURL, Ref: itestRef,
			SPDX: itestSPDX, Body: "### Característica Obtenida\n\nCuerpo obtenido.",
			Lang: "es", Document: "docs/es/FEATURES.md",
		}}, nil
	})

	online, err := Bridge{}.Render(doc, core.Options{Dir: dir})
	require.NoError(t, err)
	require.Len(t, online.Files, 3)
	for rel, body := range online.Files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, body, 0o644))
	}

	offline, err := Bridge{}.Render(doc, core.Options{Dir: dir, Offline: true})
	require.NoError(t, err)
	for rel, body := range online.Files {
		assert.Equal(t, string(body), string(offline.Files[rel]),
			"offline reassembly of %s must be byte-identical to the online render", rel)
	}
}

// TestFetchParentsReportsFailure: one unreachable parent among several keeps
// the readable copies but reports the document as not fully fetched, so the
// caller can preserve the committed sections instead of mixing.
func TestFetchParentsReportsFailure(t *testing.T) {
	doc := pfmodel.FragmentDocument{
		Dir: "docs/features.d", Out: "FEATURES.md", Title: "Features",
		Parents: []pfmodel.FragmentParent{{URL: itestURL}, {URL: "https://example.test/b19/other"}},
	}
	var calls int
	itestStub(t, func(parent pfmodel.FragmentParent, _ string, _ []string) (inheritedCopy, map[string]inheritedCopy, error) {
		calls++
		if strings.HasSuffix(parent.URL, "/other") {
			return inheritedCopy{}, nil, errors.New("boom")
		}
		return inheritedCopy{Name: testParent, Ref: "1.0.0", SPDX: itestSPDX, Body: "### Entry"}, nil, nil
	})

	copies, _, ok := fetchParents(doc, nil)
	assert.False(t, ok, "a failed parent must fail the document")
	assert.Len(t, copies, 1, "readable copies still return")
	assert.Equal(t, 2, calls)
}

// TestFetchParentsSkipsUnpublishedParent: a parent that publishes no such
// document is an absence, not a failure — the document stays fully fetched.
func TestFetchParentsSkipsUnpublishedParent(t *testing.T) {
	doc := pfmodel.FragmentDocument{Parents: []pfmodel.FragmentParent{{URL: itestURL}}}
	itestStub(t, func(pfmodel.FragmentParent, string, []string) (inheritedCopy, map[string]inheritedCopy, error) {
		return inheritedCopy{}, nil, errNotPublished
	})

	copies, _, ok := fetchParents(doc, nil)
	assert.True(t, ok, "not-published is not a failure")
	assert.Empty(t, copies)
}
