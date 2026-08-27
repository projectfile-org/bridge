// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package github implements the forge.Client interface for github.com and
// self-hosted GitHub Enterprise (GHES) instances. Two endpoints are used:
//
//   - PATCH /repos/{owner}/{repo}         — description + homepage
//   - PUT   /repos/{owner}/{repo}/topics  — replace-all topics list
//
// Topics live behind a separate endpoint because GitHub only accepts the
// `application/vnd.github+json` Accept header for the topics PUT, and only
// the topics PUT replaces the whole list (the generic PATCH ignores it).
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"projectfile.org/projectfile/bridge/internal/forge/core"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
)

// driver is the concrete Client for the github kind. baseURL accommodates
// both github.com (api.github.com) and a GHES instance whose API root is
// "<host>/api/v3" — set per-host by Owner via resolveBaseURL.
type driver struct {
	http      *http.Client
	userAgent string
	// baseURL, when non-empty, overrides the canonical api.github.com root.
	// Used by tests pointing at httptest.NewServer; production code leaves
	// it empty so apiBase returns the canonical value.
	baseURL string
}

// New returns a Client wired with the supplied HTTPOptions. Called from
// the registry's resolver — one Client per command invocation.
func New(opts core.HTTPOptions) core.Client {
	if opts.UserAgent == "" {
		opts.UserAgent = "pf-cli"
	}
	return &driver{
		http:      core.NewHTTPClient(opts),
		userAgent: opts.UserAgent,
	}
}

func (d *driver) Kind() string { return string(hostmatch.KindGitHub) }

// Owner parses a GitHub repo URL into (owner, repo). Trailing ".git" is
// trimmed because users frequently copy clone URLs. Accepts ssh:// and
// scp-style git@host:owner/repo.git URLs.
func (d *driver) Owner(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(hostmatch.ScpToSSH(repoURL))
	if err != nil {
		return "", "", fmt.Errorf("parse %q: %w", repoURL, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("github url %q lacks owner/repo path", repoURL)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

// Fetch retrieves the repo's current metadata via GET /repos/{o}/{r} then
// GET /topics. Two round-trips because the topics list is a separate
// resource on the GitHub API.
func (d *driver) Fetch(ctx context.Context, owner, repo string) (core.Snapshot, error) {
	base := d.apiBase(ctx, owner)
	var meta struct {
		Description string `json:"description"`
		Homepage    string `json:"homepage"`
	}
	if err := d.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", base, owner, repo), nil, &meta); err != nil {
		return core.Snapshot{}, err
	}
	var topics struct {
		Names []string `json:"names"`
	}
	if err := d.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/topics", base, owner, repo), nil, &topics); err != nil {
		return core.Snapshot{}, err
	}
	return core.Snapshot{
		Description: meta.Description,
		Homepage:    meta.Homepage,
		Topics:      core.LowerTopics(topics.Names),
	}, nil
}

// Apply pushes the patch via PATCH /repos and (when Topics changed) PUT
// /topics. Two requests max — the same shape Fetch made.
func (d *driver) Apply(ctx context.Context, owner, repo string, patch core.Patch) error {
	base := d.apiBase(ctx, owner)
	if patch.Description != nil || patch.Homepage != nil {
		body := map[string]any{}
		if patch.Description != nil {
			body["description"] = *patch.Description
		}
		if patch.Homepage != nil {
			body["homepage"] = *patch.Homepage
		}
		if err := d.do(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/%s", base, owner, repo), body, nil); err != nil {
			return err
		}
	}
	if patch.Topics != nil {
		body := map[string]any{"names": *patch.Topics}
		if err := d.do(ctx, http.MethodPut, fmt.Sprintf("%s/repos/%s/%s/topics", base, owner, repo), body, nil); err != nil {
			return err
		}
	}
	return nil
}

// apiBase returns the API root URL. github.com uses api.github.com; a
// GitHub Enterprise Server (GHES) instance uses "<host>/api/v3". The host
// rides on the request context so the same driver handles both, keyed on
// whatever host the push loop derived from the repository URL.
func (d *driver) apiBase(ctx context.Context, _ string) string {
	if d.baseURL != "" {
		return d.baseURL
	}
	host := core.HostFromContext(ctx)
	if host == "" || host == "github.com" {
		return "https://api.github.com"
	}
	return "https://" + host + "/api/v3"
}

// do issues a single JSON HTTP request, threading the auth token from the
// context and respecting retry semantics. body is marshalled iff non-nil;
// out is decoded iff non-nil. 4xx returns a wrapped error that includes
// the response body so the user sees the actual GitHub error message.
func (d *driver) do(ctx context.Context, method, url string, body, out any) error {
	var reader *bytes.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	var req *http.Request
	var err error
	if reader != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reader)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := core.TokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := core.DoWithRetry(ctx, d.http, req)
	if err != nil {
		return err
	}
	raw, err := core.ReadAndClose(resp)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("github %s %s: HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
