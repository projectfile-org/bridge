// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package vulnerabilities

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// TestMain keeps every render in this package off osv.dev.
func TestMain(m *testing.M) {
	lookup = func(string, bool) (string, error) { return "", nil }
	os.Exit(m.Run())
}

const (
	testNVD  = "https://nvd.nist.gov/vuln/detail/"
	testName = "x/net vulnerable to XSS"
)

// fakeLookup replaces the osv.dev lookup and counts its calls per ID.
func fakeLookup(t *testing.T, names map[string]string, err error) map[string]int {
	t.Helper()
	calls := map[string]int{}
	var mu sync.Mutex
	prev := lookup
	lookup = func(id string, _ bool) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls[id]++
		return names[id], err
	}
	resolved.names, resolved.offline = map[string]string{}, false
	t.Cleanup(func() {
		lookup = prev
		resolved.names, resolved.offline = map[string]string{}, false
	})
	return calls
}

// Each format puts the name and the link on their own comment lines above the entry.
func TestRenderWritesNameAndLinkLines(t *testing.T) {
	fakeLookup(t, nil, nil)
	doc := sampleDoc(nil)
	ns, _ := projectfile.LookupExtension(doc, pfmodel.VulnerabilitiesExtensionNS)
	ns.(map[string]any)["suppress"] = []any{map[string]any{"id": testCVE1, "name": testName, "reason": "upstream"}, "lodash"}
	want := map[string]string{
		scannerTrivy:   "# " + testName + "\n# " + testNVD + testCVE1 + "\n" + testCVE1 + "\n\nlodash\n",
		scannerGrype:   "  # " + testName + "\n  # " + testNVD + testCVE1 + "\n  - vulnerability: " + testCVE1 + "  # upstream\n",
		scannerOSV:     "# " + testName + "\n# " + testNVD + testCVE1 + "\n[[IgnoredVulns]]\nid = \"" + testCVE1 + "\"\n",
		scannerAuditCI: "    // " + testName + "\n    // " + testNVD + testCVE1 + "\n    // upstream\n    \"" + testCVE1 + "\",\n",
	}
	for _, row := range targets {
		out, err := Bridge{scanner: row.scanner, filename: row.filename}.Render(doc, core.Options{})
		require.NoError(t, err)
		assert.Contains(t, string(out.Files[row.filename]), want[row.scanner], row.filename)
	}
}

// writeProjectfile seeds a nested-form YAML projectfile with a comment, a shorthand ID, a mapped ID and a named one.
func writeProjectfile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projectfile.yaml")
	body := `identity:
  name: p
org:
  projectfile:
    vulnerabilities:
      # keep me
      suppress:
        - ` + testCVE1 + `
        - id: ` + testCVE2 + `
          reason: upstream
        - id: GHSA-named
          name: already named
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// A writing run looks each unnamed ID up once, renders the names, and caches them in the base projectfile.
func TestFillNamesCachesInTheProjectfile(t *testing.T) {
	calls := fakeLookup(t, map[string]string{testCVE1: "first", testCVE2: "second"}, nil)
	path := writeProjectfile(t)
	doc, err := projectfile.ReadBaseFromPath(path)
	require.NoError(t, err)
	opts := core.Options{Dir: filepath.Dir(path), PFPath: path}

	for _, row := range targets {
		out, err := Bridge{scanner: row.scanner, filename: row.filename}.Render(doc, opts)
		require.NoError(t, err)
		assert.Contains(t, string(out.Files[row.filename]), "first", row.filename)
	}
	assert.Equal(t, map[string]int{testCVE1: 1, testCVE2: 1}, calls, "four renders, one lookup per unnamed ID")

	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(saved), "# keep me", "comments survive the write")
	assert.Contains(t, string(saved), "name: first", "a shorthand ID becomes a mapped entry")
	assert.Contains(t, string(saved), "name: second")
	assert.Contains(t, string(saved), "name: already named")

	reread, err := projectfile.ReadBaseFromPath(path)
	require.NoError(t, err)
	ext, err := pfmodel.GetVulnerabilitiesExtension(reread)
	require.NoError(t, err)
	assert.Len(t, ext.Suppress, 3)
}

// Check and dry runs render the same names a write would, and leave the projectfile untouched.
func TestFillNamesWritesNothingOnReadOnlyRuns(t *testing.T) {
	path := writeProjectfile(t)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	for name, opts := range map[string]core.Options{
		"check":   {PFPath: path, Check: true},
		"dry-run": {PFPath: path, DryRun: true},
		"no path": {},
	} {
		fakeLookup(t, map[string]string{testCVE1: "first"}, nil)
		got := fillNames([]pfmodel.VulnerabilitySuppress{{ID: testCVE1}}, opts)
		assert.Equal(t, "first", got[0].Name, name)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after), name)
	}
}

// An offline run still asks the lookup, which answers from the local cache only.
func TestFillNamesPassesOfflineToTheLookup(t *testing.T) {
	path := writeProjectfile(t)
	var seen []bool
	prev := lookup
	lookup = func(_ string, offline bool) (string, error) {
		seen = append(seen, offline)
		return "", nil
	}
	t.Cleanup(func() { lookup = prev; resolved.names = map[string]string{} })
	fillNames([]pfmodel.VulnerabilitySuppress{{ID: testCVE1}}, core.Options{PFPath: path, Offline: true})
	assert.Equal(t, []bool{true}, seen)
}

// One failed lookup sends the rest to the local cache only, so an unreachable osv.dev costs one timeout, not one per ID.
func TestFillNamesStopsAfterAFailure(t *testing.T) {
	path := writeProjectfile(t)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	var offline []bool
	prev := lookup
	lookup = func(_ string, off bool) (string, error) {
		offline = append(offline, off)
		if off {
			return "", nil
		}
		return "", errors.New("dial tcp: no route")
	}
	resolved.names, resolved.offline = map[string]string{}, false
	lookupWorkers = 1
	t.Cleanup(func() {
		lookup = prev
		lookupWorkers = 8
		resolved.names, resolved.offline = map[string]string{}, false
	})
	got := fillNames([]pfmodel.VulnerabilitySuppress{{ID: testCVE1}, {ID: testCVE2}}, core.Options{PFPath: path})
	assert.Empty(t, got[0].Name)
	assert.Equal(t, []bool{false, true}, offline)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "nothing found, nothing written")
}

// Sorting is alphanumeric: CVE-2026-9999 comes before CVE-2026-10000.
func TestDedupSortedIsAlphanumeric(t *testing.T) {
	got := dedupSorted([]pfmodel.VulnerabilitySuppress{{ID: "CVE-2026-10000"}, {ID: "CVE-2026-9999"}, {ID: "CVE-2026-9999", Name: "dup"}})
	require.Len(t, got, 2)
	assert.Equal(t, "CVE-2026-9999", got[0].ID)
	assert.Empty(t, got[0].Name, "the first entry per ID wins")
}
