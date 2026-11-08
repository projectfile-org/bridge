// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package advisory

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/netfetch"
)

func TestMain(m *testing.M) {
	netfetch.SetAttempts(1)
	os.Exit(m.Run())
}

func TestURLPicksTheOfficialDatabase(t *testing.T) {
	cases := map[string]string{
		"CVE-2026-31608":          "https://nvd.nist.gov/vuln/detail/CVE-2026-31608",
		"GHSA-vvgc-356p-c3xw":     "https://github.com/advisories/GHSA-vvgc-356p-c3xw",
		"GO-2025-4007":            "https://pkg.go.dev/vuln/GO-2025-4007",
		"RUSTSEC-2024-0001":       "https://osv.dev/vulnerability/RUSTSEC-2024-0001",
		"GHSA-vvgc-356p-c3xw|a>b": "https://github.com/advisories/GHSA-vvgc-356p-c3xw",
		"lodash":                  "",
		"cve-2026-1":              "",
		"CVE-2026-1/../../etc":    "",
		"":                        "",
	}
	for id, want := range cases {
		assert.Equal(t, want, URL(id), id)
	}
}

func TestCompareOrdersDigitRunsByValue(t *testing.T) {
	ids := []string{"GHSA-b", "CVE-2026-10000", "CVE-2025-9", "CVE-2026-9999", "GO-2026-1", "CVE-2026-09999", "GHSA-a"}
	slices.SortFunc(ids, Compare)
	assert.Equal(t, []string{"CVE-2025-9", "CVE-2026-09999", "CVE-2026-9999", "CVE-2026-10000", "GHSA-a", "GHSA-b", "GO-2026-1"}, ids)
	assert.Zero(t, Compare("CVE-1", "CVE-1"))
	assert.Negative(t, Compare("CVE-1", "CVE-1-a"), "a prefix sorts first")
	assert.NotZero(t, Compare("CVE-07", "CVE-7"), "equal values with different padding still order deterministically")
}

func TestOneLineCapsAtTheLimit(t *testing.T) {
	assert.Equal(t, "a b", OneLine(" a\n\tb "))
	exact := strings.Repeat("x", maxNameRunes)
	assert.Equal(t, exact, OneLine(exact), "120 runes stay whole")
	over := OneLine(exact + "y")
	assert.Equal(t, maxNameRunes, len([]rune(over)), "121 runes cut to 120 with the ellipsis")
	assert.True(t, strings.HasSuffix(over, "…"))
}

// osvServer answers each path with the given status and body.
func osvServer(t *testing.T, routes map[string]struct {
	status int
	body   string
},
) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			route.status = http.StatusNotFound
		}
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/"
}

func TestFetchNameReadsSummaryThenDetails(t *testing.T) {
	ep := osvServer(t, map[string]struct {
		status int
		body   string
	}{
		"GHSA-x": {http.StatusOK, `{"summary":"x/net vulnerable to XSS","details":"long"}`},
		"CVE-1":  {http.StatusOK, `{"details":"SSH clients panic. More text.\nSecond line."}`},
		"CVE-2":  {http.StatusInternalServerError, ``},
		"CVE-3":  {http.StatusOK, `not json`},
	})
	client := &http.Client{}

	name, err := fetchName(ep, "GHSA-x", client)
	require.NoError(t, err)
	assert.Equal(t, "x/net vulnerable to XSS", name)

	name, err = fetchName(ep, "CVE-1", client)
	require.NoError(t, err)
	assert.Equal(t, "SSH clients panic.", name, "no summary: the first sentence of details")

	name, err = fetchName(ep, "CVE-404", client)
	require.NoError(t, err, "a record OSV lacks is not an error")
	assert.Empty(t, name)

	_, err = fetchName(ep, "CVE-2", client)
	assert.ErrorContains(t, err, "HTTP 500")

	_, err = fetchName(ep, "CVE-3", client)
	assert.ErrorContains(t, err, "decode")

	name, err = fetchName(ep, "lodash", client)
	require.NoError(t, err, "a package name is never sent to OSV")
	assert.Empty(t, name)
}

// A cached title answers offline; a miss offline is empty, never a network call.
func TestNameReadsTheLocalCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	name, err := Name("CVE-2026-1", true)
	require.NoError(t, err)
	assert.Empty(t, name, "offline miss")

	storeCached("CVE-2026-1", "cached title")
	name, err = Name("CVE-2026-1|a>b", true)
	require.NoError(t, err)
	assert.Equal(t, "cached title", name, "the audit-ci path suffix shares the advisory's entry")

	entries, err := os.ReadDir(CacheDir())
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temp file left behind")
	assert.Equal(t, "CVE-2026-1.txt", entries[0].Name())
}
