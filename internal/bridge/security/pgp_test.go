// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// fixtureKey is the real B64C122EE16C3746 key (privacy upload, no User ID) —
// the exact shape keys.openpgp.org serves, which defeated openpgp.ReadEntity.
const fixtureKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----
Comment: 6F19 7084 3C9E 8406 AD70  0467 B64C 122E E16C 3746

xjMEahnyHBYJKwYBBAHaRw8BAQdAveBz/civENBGc0b3R28UsjpJPfh6HFDa2UG9
U9iCiFDOOARqGfIcEgorBgEEAZdVAQUBAQdALQhtRV9k4uomNIDYse8AZuG+D08S
xeAumggsj5nP20QDAQgHwngEGBYKACAWIQRvGXCEPJ6EBq1wBGe2TBIu4Ww3RgUC
ahnyHAIbDAAKCRC2TBIu4Ww3RmJiAQC2bAiMDfPFFIclJCR+CHFKmPVO+XqQoydT
viC78oozewD9FR0gYIQRKef7BnGFv2bJMmmYfv5VYsbd9URYMS6Rsgs=
=zW+B
-----END PGP PUBLIC KEY BLOCK-----`

// expectedFP is what GnuPG prints for the fixture key.
const expectedFP = "6F19 7084 3C9E 8406 AD70  0467 B64C 122E E16C 3746"

// TestParseFingerprint reads the real armored key fixture and asserts the
// grouped uppercase fingerprint matches GnuPG's output.
func TestParseFingerprint(t *testing.T) {
	fp, err := parseFingerprint([]byte(fixtureKey))
	require.NoError(t, err)
	assert.Equal(t, expectedFP, fp)
}

// TestParseFingerprintFromFile reads the captured .asc from testdata so a
// regeneration of the fixture (new key) is caught here, not only above.
func TestParseFingerprintFromFile(t *testing.T) {
	armored, err := os.ReadFile(filepath.Join("testdata", "B64C122EE16C3746.asc"))
	require.NoError(t, err)
	fp, err := parseFingerprint(armored)
	require.NoError(t, err)
	assert.Equal(t, expectedFP, fp)
}

// TestParseFingerprintRejectsNonArmored guards the armor-decode step: plain
// text (HTML login page, error body) is not a valid key.
func TestParseFingerprintRejectsNonArmored(t *testing.T) {
	cases := map[string]string{
		"html":    "<html><body>login</body></html>",
		"empty":   "",
		"garbage": "not a pgp block",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseFingerprint([]byte(in))
			assert.Error(t, err)
		})
	}
}

// TestFormatFingerprint pins the GnuPG grouping: two halves, double space
// between them, single space within.
func TestFormatFingerprint(t *testing.T) {
	// 20 bytes → 40 hex chars.
	fp := formatFingerprint([]byte{
		0x6F, 0x19, 0x70, 0x84, 0x3C, 0x9E, 0x84, 0x06, 0xAD, 0x70,
		0x04, 0x67, 0xB6, 0x4C, 0x12, 0x2E, 0xE1, 0x6C, 0x37, 0x46,
	})
	assert.Equal(t, "6F19 7084 3C9E 8406 AD70  0467 B64C 122E E16C 3746", fp)
}

// TestDeriveFingerprintOfflineSkipsNetwork verifies the kill-switch: Offline
// mode reads the cache and never reaches the network (no server needed).
func TestDeriveFingerprintOfflineSkipsNetwork(t *testing.T) {
	// No server started; if Offline is ignored, this would dial and fail.
	withTempCache(t, func() {
		fp, src := deriveFingerprint("B64C122EE16C3746", "",
			core.Options{Offline: true})
		assert.Empty(t, fp, "offline with empty cache must not derive")
		assert.Empty(t, src)
	})
}

// TestDeriveFingerprintOfflineReadsCache verifies an offline run reuses a
// fingerprint derived earlier, so CI drift gates stay network-free.
func TestDeriveFingerprintOfflineReadsCache(t *testing.T) {
	withTempCache(t, func() {
		storeCachedFingerprint(cacheKeyURL(keysOpenPGPKeyByID+"B64C122EE16C3746"), expectedFP)
		fp, src := deriveFingerprint("B64C122EE16C3746", "",
			core.Options{Offline: true})
		assert.Equal(t, expectedFP, fp)
		assert.Contains(t, src, "cached")
	})
}

// TestDeriveFingerprintFromKeyIDHTTP stubs keys.openpgp.org with a test server
// serving the armored key, and asserts the derive parses + caches the result.
func TestDeriveFingerprintFromKeyIDHTTP(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pgp-keys")
		_, _ = io.WriteString(w, fixtureKey)
	}))
	t.Cleanup(srv.Close)

	// deriveFromURL uses the production client, which won't trust the test
	// server's self-signed cert. Drive fetchArmored+parseFingerprint directly
	// against the stub client to exercise the parse+cache path offline-style.
	withTempCache(t, func() {
		ctx := context.Background()
		armored, err := fetchArmored(ctx, srv.URL+"/key", srv.Client())
		require.NoError(t, err)
		fp, err := parseFingerprint(armored)
		require.NoError(t, err)
		assert.Equal(t, expectedFP, fp)
		storeCachedFingerprint(cacheKeyURL(srv.URL+"/key"), fp)
		assert.Equal(t, expectedFP, loadCachedFingerprint(cacheKeyURL(srv.URL+"/key")))
	})
}

// TestFetchArmoredRejectsHTML guards the login-page defense: an HTML response
// (Content-Type: text/html) is rejected even with a 200 status.
func TestFetchArmoredRejectsHTML(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html><body>sign in</body></html>")
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	_, err := fetchArmored(ctx, srv.URL+"/key", srv.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML")
}

// TestFetchArmored404ReturnsEmpty verifies the "no such key" case: a 404 is
// not an error, just an empty result, so the caller falls through cleanly.
func TestFetchArmored404ReturnsEmpty(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	body, err := fetchArmored(ctx, srv.URL+"/key", srv.Client())
	require.NoError(t, err)
	assert.Empty(t, body)
}

// TestFetchArmoredRejectsNonHTTPS guards the secure-defaults rule: a plaintext
// key URL is refused before any dial.
func TestFetchArmoredRejectsNonHTTPS(t *testing.T) {
	_, err := fetchArmored(context.Background(), "http://keys.example.org/key", pgpClient())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-HTTPS")
}

// TestCacheRoundTrip exercises the store→load pair under an isolated cache dir
// so the test never touches the developer's real cache.
func TestCacheRoundTrip(t *testing.T) {
	withTempCache(t, func() {
		key := cacheKeyURL("https://example.org/key.asc")
		assert.Empty(t, loadCachedFingerprint(key), "absent key loads empty")
		storeCachedFingerprint(key, expectedFP)
		assert.Equal(t, expectedFP, loadCachedFingerprint(key))
	})
}

// withTempCache points the cache at a throwaway dir for one test. The helpers
// read XDG_CACHE_HOME at call time, so setting it for the test is enough.
func withTempCache(t *testing.T, fn func()) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fn()
}
