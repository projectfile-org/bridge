// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/netfetch"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// otherKeyID is a key id the fixture key does not carry, so a pgp-key link serving the fixture is proven to hold another key.
const otherKeyID = "0123456789ABCDEF"

// stubCA signs every pubkey stub below; it is generated once per process because the system cert pool is read once per process.
var stubCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// TestMain binds one attempt per request and publishes the stub CA a real TLS client trusts.
func TestMain(m *testing.M) {
	netfetch.SetAttempts(1)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "pf-bridge stub CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	stubCA.cert, err = x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	stubCA.key = key
	dir, err := os.MkdirTemp("", "pf-bridge-stub-ca")
	if err != nil {
		panic(err)
	}
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		panic(err)
	}
	_ = os.Setenv("SSL_CERT_FILE", ca) // the pool is read once per process, so this must precede the first handshake

	code := m.Run()
	_ = os.RemoveAll(dir)
	_ = os.Unsetenv("SSL_CERT_FILE")
	os.Exit(code)
}

// ── trusted pubkey stub ─────────────────────────────────────────────────────

// trustedKeyServer starts a TLS pubkey stub whose leaf the stub CA signs, so the production client accepts it.
func trustedKeyServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, stubCA.cert, &key.PublicKey, stubCA.key)
	require.NoError(t, err)

	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// serveKey answers an armored public key, the content type keys.openpgp.org uses.
func serveKey(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/pgp-keys")
	_, _ = io.WriteString(w, fixtureKey)
}

// ── online derive ───────────────────────────────────────────────────────────

// The link is the derive's first source, so a served key answers the fingerprint and is cached for offline runs.
func TestDeriveFingerprintDerivesFromTheLink(t *testing.T) {
	srv := trustedKeyServer(t, serveKey)
	withTempCache(t, func() {
		fp, src, keyURL := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Equal(t, expectedFP, fp)
		assert.Equal(t, "derived:links[].type=pgp-key", src)
		assert.Equal(t, srv.URL+"/key", keyURL)
		assert.Equal(t, expectedFP, loadCachedFingerprint(cacheKeyURL(srv.URL+"/key")), "the derived fingerprint is cached")
	})
}

// A cached fingerprint is immutable, so the derive reuses it without dialling.
func TestDeriveFingerprintReusesTheCachedLink(t *testing.T) {
	dialled := false
	srv := trustedKeyServer(t, func(w http.ResponseWriter, r *http.Request) {
		dialled = true
		serveKey(w, r)
	})
	withTempCache(t, func() {
		storeCachedFingerprint(cacheKeyURL(srv.URL+"/key"), expectedFP)
		fp, src, _ := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Equal(t, expectedFP, fp)
		assert.Equal(t, "derived:links[].type=pgp-key", src, "the cached derivation reports the link, not the cache")
		assert.False(t, dialled, "a cache hit must not reach the network")
	})
}

// A link proven to carry another key yields neither its fingerprint nor its URL.
func TestDeriveFingerprintDropsALinkCarryingAnotherKey(t *testing.T) {
	srv := trustedKeyServer(t, serveKey)
	withTempCache(t, func() {
		storeCachedFingerprint(cacheKeyURL(keysOpenPGPKeyByID+otherKeyID), expectedFP)
		fp, src, keyURL := deriveFingerprint(otherKeyID, srv.URL+"/key", core.Options{})
		assert.Equal(t, expectedFP, fp)
		assert.Equal(t, "derived:keys.openpgp.org", src)
		assert.Empty(t, keyURL, "the dropped link leaves the render, not the verify block")
	})
}

// A login page is not a key, so the derive warns and leaves the field empty while the key id still resolves.
func TestDeriveFingerprintWarnsWhenTheLinkAnswersHTML(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>sign in</html>")
	})
	withTempCache(t, func() {
		storeCachedFingerprint(cacheKeyURL(keysOpenPGPKeyByID+otherKeyID), expectedFP)
		fp, src, keyURL := deriveFingerprint(otherKeyID, srv.URL+"/key", core.Options{})
		assert.Equal(t, expectedFP, fp, "the keyserver lookup still answers")
		assert.Equal(t, "derived:keys.openpgp.org", src)
		assert.Equal(t, srv.URL+"/key", keyURL, "an unread link stays in the render")
	})
}

// A 404 is "no such key", not a failure: the derive falls through and reports nothing.
func TestDeriveFingerprintSkipsALinkWithNoKey(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	withTempCache(t, func() {
		fp, src, keyURL := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Empty(t, fp)
		assert.Empty(t, src)
		assert.Equal(t, srv.URL+"/key", keyURL, "an unread link stays in the render")
		assert.Empty(t, loadCachedFingerprint(cacheKeyURL(srv.URL+"/key")), "nothing is cached for an empty answer")
	})
}

// A same-host redirect is followed; a cross-host one is refused as an auth gateway.
func TestDeriveFingerprintFollowsARedirectOnTheSameHost(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key" {
			http.Redirect(w, r, "/pub", http.StatusFound)
			return
		}
		serveKey(w, r)
	})
	withTempCache(t, func() {
		fp, _, _ := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Equal(t, expectedFP, fp)
	})
}

func TestDeriveFingerprintRefusesACrossHostRedirect(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://keys.example.org/key", http.StatusFound)
	})
	withTempCache(t, func() {
		fp, src, keyURL := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Empty(t, fp, "a gateway bounce is not a key")
		assert.Empty(t, src)
		assert.Equal(t, srv.URL+"/key", keyURL)
	})
}

// A redirect chain that never ends is cut off rather than followed forever.
func TestDeriveFingerprintCapsTheRedirectChain(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	withTempCache(t, func() {
		fp, _, _ := deriveFingerprint("", srv.URL+"/loop", core.Options{})
		assert.Empty(t, fp)
	})
}

// Check writes nothing, so a drift gate derivation leaves the cache untouched.
func TestDeriveFingerprintCachesNothingInCheck(t *testing.T) {
	srv := trustedKeyServer(t, serveKey)
	withTempCache(t, func() {
		fp, _, _ := deriveFingerprint("", srv.URL+"/key", core.Options{Check: true})
		assert.Equal(t, expectedFP, fp)
		assert.Empty(t, loadCachedFingerprint(cacheKeyURL(srv.URL+"/key")), "the gate derives without writing")
	})
}

// ── fetch edges ─────────────────────────────────────────────────────────────

// A URL that does not parse is refused before any dial.
func TestFetchArmoredRefusesAMalformedURL(t *testing.T) {
	_, err := fetchArmored(context.Background(), "https://exa\tmple.org/key", pgpClient())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid control character")
}

// A refused connection is reported as-is, with every attempt spent on the transport.
func TestFetchArmoredReportsARefusedConnection(t *testing.T) {
	_, err := fetchArmored(context.Background(), "https://127.0.0.1:1/key", pgpClient())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// A body that ends where its length promised is reported, not silently accepted as a key.
func TestFetchArmoredReportsATruncatedBody(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 4096\r\nContent-Type: application/pgp-keys\r\n\r\nshort")
		_ = buf.Flush()
	})
	withTempCache(t, func() {
		_, _, _ = deriveFingerprint("", srv.URL+"/key", core.Options{})
		_, err := fetchArmored(context.Background(), srv.URL+"/key", pgpClient())
		require.Error(t, err, "a truncated body cannot be parsed into a fingerprint")
	})
}

// A key block whose first packet is not a public key is walked past, then reported.
func TestParseFingerprintSkipsANonKeyPacket(t *testing.T) {
	_, err := parseFingerprint(armoredUserID(t))
	require.Error(t, err, "a user id packet is not a public key")
	assert.Contains(t, err.Error(), "no public-key packet")
}

// A body that armors but carries no key is reported as a parse failure naming its source.
func TestDeriveFingerprintReportsABodyThatIsNotAKey(t *testing.T) {
	srv := trustedKeyServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pgp-keys")
		_, _ = w.Write(armoredUserID(t))
	})
	withTempCache(t, func() {
		fp, src, keyURL := deriveFingerprint("", srv.URL+"/key", core.Options{})
		assert.Empty(t, fp, "the block carries no key, so no fingerprint is derived")
		assert.Empty(t, src)
		assert.Equal(t, srv.URL+"/key", keyURL)
		assert.Empty(t, loadCachedFingerprint(cacheKeyURL(srv.URL+"/key")), "a failed parse is never cached")
	})
}

// A keyserver answer that cannot be read is reported and drops the verify block, never the render.
func TestDeriveFingerprintOmitsTheBlockWhenTheLookupFails(t *testing.T) {
	withTempCache(t, func() {
		fp, src, keyURL := deriveFingerprint("0123\n4567", "", core.Options{})
		assert.Empty(t, fp)
		assert.Empty(t, src)
		assert.Empty(t, keyURL, "no link was declared")
	})
}

// armoredUserID builds an armored block whose first packet is a user id, the shape a malformed key file carries.
func armoredUserID(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP PUBLIC KEY BLOCK", nil)
	require.NoError(t, err)
	uid := packet.NewUserId("Widget", "", "stub@example.org")
	require.NotNil(t, uid)
	require.NoError(t, uid.Serialize(w))
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// ── cache slot ──────────────────────────────────────────────────────────────

// With neither XDG nor HOME the cache falls back to the working directory instead of failing.
func TestCacheDirFallsBackToTheWorkingDirectory(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	assert.Equal(t, filepath.Join(".cache", "pf", "pgp"), pgpCacheDir())
}

// A cache path the filesystem refuses is skipped rather than aborting the render.
func TestStoreCachedFingerprintSkipsAnUnwritableCache(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	t.Setenv("XDG_CACHE_HOME", blocker)
	storeCachedFingerprint("deadbeef", expectedFP)
	assert.Empty(t, loadCachedFingerprint("deadbeef"), "nothing is written where the mkdir failed")
}
