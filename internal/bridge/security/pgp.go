// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/bridge/core"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// keysOpenPGPKeyByID is the only keyserver the derive falls back to. It answers
// application/pgp-keys for a by-keyid lookup; a self-hosted pubkey (preferred,
// via links[].type=pgp-key) is tried first.
const keysOpenPGPKeyByID = "https://keys.openpgp.org/vks/v1/by-keyid/"

// pgpFetchTimeout caps a single derive fetch. Mirrors the include-resolver and
// forge HTTP surfaces so the three network code paths behave the same.
const pgpFetchTimeout = 10 * time.Second

// deriveFingerprint fills an empty fingerprint by fetching the armored public
// key and parsing its primary-key packet. It tries the links[].type=pgp-key URL
// first (self-hosted, authoritative), then keys.openpgp.org keyed by gpgKey.
//
// Returns ("", "") when the fingerprint is already known, when Offline is set
// (cache read only), when no source is available, or when every fetch failed.
// A derive failure NEVER blocks the render: the verify section simply omits, as
// it did before the derive existed.
func deriveFingerprint(gpgKey, gpgKeyURL string, opts core.Options) (fingerprint, source string) {
	if opts.Offline {
		// Cache read only: an offline run must not reach the network. The cache
		// keys mirror the online write path (URL-keyed), so a fingerprint
		// derived online earlier in the same cache is reused offline.
		if gpgKeyURL != "" {
			if fp := loadCachedFingerprint(cacheKeyURL(gpgKeyURL)); fp != "" {
				return fp, "derived:links[].type=pgp-key (cached)"
			}
		}
		if gpgKey != "" {
			if fp := loadCachedFingerprint(cacheKeyURL(keysOpenPGPKeyByID + gpgKey)); fp != "" {
				return fp, "derived:keys.openpgp.org (cached)"
			}
		}
		return "", ""
	}

	if gpgKeyURL != "" {
		fp, err := deriveFromURL(gpgKeyURL, opts)
		if err != nil {
			genlog.Warn("gpg-fingerprint derive failed; falling through",
				"source", "links[].type=pgp-key", "error", err.Error())
		} else if fp != "" {
			return fp, "derived:links[].type=pgp-key"
		}
	}
	if gpgKey != "" {
		fp, err := deriveFromURL(keysOpenPGPKeyByID+gpgKey, opts)
		if err != nil {
			genlog.Warn("gpg-fingerprint derive failed; omitting verify section",
				"source", "keys.openpgp.org", "error", err.Error())
		} else if fp != "" {
			return fp, "derived:keys.openpgp.org"
		}
	}
	return "", ""
}

// deriveFromURL fetches an armored key (cache first unless Refresh), parses the
// fingerprint, and caches it on success. An empty result with nil error means
// the cache held nothing and no network source was available.
func deriveFromURL(keyURL string, opts core.Options) (string, error) {
	key := cacheKeyURL(keyURL)
	if !opts.Refresh {
		if fp := loadCachedFingerprint(key); fp != "" {
			return fp, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), pgpFetchTimeout)
	defer cancel()
	armored, err := fetchArmored(ctx, keyURL, pgpClient())
	if err != nil {
		return "", err
	}
	if len(armored) == 0 {
		// 4xx (no such key) — not an error, just nothing to derive from here.
		return "", nil
	}
	fp, err := parseFingerprint(armored)
	if err != nil {
		return "", fmt.Errorf("parse pubkey from %s: %w", keyURL, err)
	}
	// Write only on success, and never in Check (drift gate writes nothing).
	if !opts.Check {
		storeCachedFingerprint(key, fp)
	}
	return fp, nil
}

// pgpClient is the production HTTP client for the fingerprint derive: a 10s
// timeout and a redirect guard that rejects cross-host hops (an auth gateway
// answering for a public pubkey URL).
func pgpClient() *http.Client {
	return &http.Client{
		Timeout: pgpFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Host != via[0].URL.Host {
				return fmt.Errorf("cross-host redirect %s -> %s (likely an auth gateway; the pubkey URL must be public)",
					via[0].URL.Host, req.URL.Host)
			}
			return nil
		},
	}
}

// fetchArmored GETs an OpenPGP armored key. One bounded retry on transport or
// 5xx; 4xx returns immediately (the key is genuinely absent). Cross-host
// redirects are rejected so an auth gateway cannot substitute a login page for
// the key, and an HTML response is rejected for the same reason. The client is
// a parameter so tests inject a test server's client; production uses
// pgpClient(), which carries the timeout and redirect guard.
func fetchArmored(ctx context.Context, keyURL string, client *http.Client) ([]byte, error) {
	if !strings.HasPrefix(keyURL, "https://") {
		return nil, fmt.Errorf("refusing non-HTTPS key URL: %s", keyURL)
	}
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			jitter := time.Duration(rand.Int64N(int64(250 * time.Millisecond))) // #nosec G404 -- non-crypto retry jitter
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(250*time.Millisecond + jitter):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "projectfile")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode/100 == 5 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode/100 == 4 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			// 404 is the normal "no such key" case — return empty, not an error.
			return nil, nil
		}
		if isHTMLResponse(resp) {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("received HTML from %s (HTTP %d) — likely a login page, not a pubkey", keyURL, resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		return body, nil
	}
	return nil, lastErr
}

// parseFingerprint decodes an ASCII-armored OpenPGP key and returns the
// primary key's full fingerprint, grouped the way GnuPG prints it
// (XXXX XXXX XXXX XXXX XXXX  XXXX XXXX XXXX XXXX XXXX).
//
// It reads the first public-key packet directly rather than via ReadEntity,
// because keys.openpgp.org serves keys WITHOUT a User ID (privacy upload),
// and ReadEntity rejects a v4 key that has no self-signature carrying an
// identity. The fingerprint is on the primary-key packet itself.
func parseFingerprint(armored []byte) (string, error) {
	block, err := armor.Decode(bytes.NewReader(armored))
	if err != nil {
		return "", fmt.Errorf("armor decode: %w", err)
	}
	pr := packet.NewReader(block.Body)
	for {
		p, err := pr.Next()
		if err != nil {
			return "", fmt.Errorf("no public-key packet in armor: %w", err)
		}
		pk, ok := p.(*packet.PublicKey)
		if !ok {
			continue
		}
		// The first PublicKey packet is the primary key; subkeys come later
		// as PublicSubkey packets, a distinct type.
		return formatFingerprint(pk.Fingerprint), nil
	}
}

// formatFingerprint renders the 20 fingerprint bytes as grouped hex. GnuPG
// prints two groups of five bytes with a double space between them.
func formatFingerprint(fp []byte) string {
	h := strings.ToUpper(hex.EncodeToString(fp))
	var b strings.Builder
	for i := 0; i < len(h); i += 4 {
		switch i {
		case 0: // first group: no separator
		case 20:
			b.WriteString("  ") // midpoint: double space, GnuPG style
		default:
			b.WriteString(" ")
		}
		b.WriteString(h[i : i+4])
	}
	return b.String()
}

// --- fingerprint cache ---
//
// One small file per source under $XDG_CACHE_HOME/pf/pgp/, keyed by SHA-256 of
// the URL (or "keyid:<id>"). The value is the formatted fingerprint string.
// This mirrors the includes-cache shape: plain files, raw read/write,
// content-addressed names. A stale entry is harmless — a key roll changes the
// key ID, so the new key resolves under a new cache key.

func cacheKeyURL(rawURL string) string {
	return hashKey("url:" + rawURL)
}

func hashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func pgpCacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "pf", "pgp")
}

func loadCachedFingerprint(key string) string {
	path := filepath.Join(pgpCacheDir(), key+".txt")
	fp, err := os.ReadFile(path) // #nosec G304 -- key is a SHA-256 hex digest, not user input; dir is XDG-derived
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(fp))
}

func storeCachedFingerprint(key, fp string) {
	dir := pgpCacheDir()
	if err := os.MkdirAll(dir, 0o750); err != nil { // #nosec G301 -- cache dir under XDG, readable by owner only
		genlog.Warn("gpg-fingerprint cache write skipped", "error", err.Error())
		return
	}
	_ = os.WriteFile(filepath.Join(dir, key+".txt"), []byte(fp), 0o600) // #nosec G306 -- cache entry, owner-only
}

// isHTMLResponse mirrors the include resolver's guard: a Content-Type that is
// text/html means we fetched a page (login, 404 with body) rather than a key.
func isHTMLResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return strings.HasPrefix(strings.ToLower(ct), "text/html")
}
