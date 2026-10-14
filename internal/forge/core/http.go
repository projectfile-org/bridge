// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

// HTTPOptions threads transport-level knobs into the driver's HTTP client; retries belong to core's netfetch.
type HTTPOptions struct {
	Timeout         time.Duration
	UserAgent       string
	InsecureSkipTLS bool // honoured for self-hosted instances with self-signed certs
}

// NewHTTPClient returns a configured *http.Client. Callers set per-request
// headers themselves; this only sets transport-level concerns (TLS + timeout).
// Timeout bounds ONE attempt — netfetch bounds the retries around it.
func NewHTTPClient(opts HTTPOptions) *http.Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureSkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- opt-in via --insecure-skip-tls flag for self-hosted instances
	}
	return &http.Client{Timeout: opts.Timeout, Transport: transport}
}

// ReadAndClose reads resp.Body fully and closes it. Tiny convenience so
// every driver doesn't repeat the same defer+ReadAll dance.
func ReadAndClose(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
