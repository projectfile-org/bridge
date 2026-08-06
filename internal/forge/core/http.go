// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"net/http"
	"time"
)

// HTTPOptions threads transport-level knobs into the driver's HTTP client.
// Defaults mirror internal/spdx/spdx.go:201-240 so the two network surfaces
// behave consistently (10s timeout, single retry with jitter on 5xx).
type HTTPOptions struct {
	Timeout         time.Duration
	UserAgent       string
	InsecureSkipTLS bool // honoured for self-hosted instances with self-signed certs
}

// NewHTTPClient returns a configured *http.Client. Callers set per-request
// headers themselves; this only sets transport-level concerns (TLS + timeout).
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

// DoWithRetry executes req via client with one bounded backoff+jitter retry
// on transport errors or 5xx responses. 4xx returns immediately — retrying
// an auth failure just wastes round trips. Returns the raw response so the
// caller decides how to decode (the response Body MUST be closed by the caller).
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			jitter := time.Duration(mrand.Int63n(int64(250 * time.Millisecond))) // #nosec G404 -- non-crypto retry jitter
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(250*time.Millisecond + jitter):
			}
		}
		// #nosec G107,G704 -- URL is constructed by the forge driver from a parsed repo URL the user explicitly listed in projectfile.toml; this command's whole job is to talk to that forge
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		// 5xx → retry once. 4xx → return immediately so the caller can
		// surface the auth/permission failure verbatim without a 5–10s wait.
		if resp.StatusCode/100 == 5 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = errors.New("HTTP request failed: unknown error")
	}
	return nil, lastErr
}

// ReadAndClose reads resp.Body fully and closes it. Tiny convenience so
// every driver doesn't repeat the same defer+ReadAll dance.
func ReadAndClose(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
