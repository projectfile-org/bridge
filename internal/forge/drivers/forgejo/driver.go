// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package forgejo implements the forge.Client interface for Forgejo,
// Gitea, and Codeberg (which is a Forgejo instance). Two endpoints:
//
//   - PATCH /api/v1/repos/{owner}/{repo}         — description + website
//   - PUT   /api/v1/repos/{owner}/{repo}/topics  — replace-all topics
//
// Forgejo and Gitea share API surface — Forgejo is a Gitea fork that
// preserved API compatibility — so the same driver handles both kinds.
package forgejo

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

type driver struct {
	http      *http.Client
	userAgent string
	// baseURL overrides the canonical codeberg.org/api/v1 root for tests.
	baseURL string
}

func New(opts core.HTTPOptions) core.Client {
	if opts.UserAgent == "" {
		opts.UserAgent = "pf-cli"
	}
	return &driver{
		http:      core.NewHTTPClient(opts),
		userAgent: opts.UserAgent,
	}
}

func (d *driver) Kind() string { return string(hostmatch.KindForgejo) }

// Owner splits the URL path into the (owner, repo) tuple. Forgejo / Gitea
// don't support nested groups so the path is always exactly two segments.
// Accepts ssh:// and scp-style git@host:owner/repo.git URLs.
func (d *driver) Owner(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(hostmatch.ScpToSSH(repoURL))
	if err != nil {
		return "", "", fmt.Errorf("parse %q: %w", repoURL, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("forgejo url %q lacks owner/repo path", repoURL)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

func (d *driver) Fetch(ctx context.Context, owner, repo string) (core.Snapshot, error) {
	base := d.apiBase(ctx, owner)
	var meta struct {
		Description string `json:"description"`
		Website     string `json:"website"`
	}
	if err := d.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", base, owner, repo), nil, &meta); err != nil {
		return core.Snapshot{}, err
	}
	var topics struct {
		Topics []string `json:"topics"`
	}
	if err := d.do(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/topics", base, owner, repo), nil, &topics); err != nil {
		return core.Snapshot{}, err
	}
	return core.Snapshot{
		Description: meta.Description,
		Homepage:    meta.Website,
		Topics:      core.LowerTopics(topics.Topics),
	}, nil
}

func (d *driver) Apply(ctx context.Context, owner, repo string, patch core.Patch) error {
	base := d.apiBase(ctx, owner)
	if patch.Description != nil || patch.Homepage != nil {
		body := map[string]any{}
		if patch.Description != nil {
			body["description"] = *patch.Description
		}
		if patch.Homepage != nil {
			// Forgejo's field is called "website", not "homepage".
			body["website"] = *patch.Homepage
		}
		if err := d.do(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/%s", base, owner, repo), body, nil); err != nil {
			return err
		}
	}
	if patch.Topics != nil {
		body := map[string]any{"topics": *patch.Topics}
		if err := d.do(ctx, http.MethodPut, fmt.Sprintf("%s/repos/%s/%s/topics", base, owner, repo), body, nil); err != nil {
			return err
		}
	}
	return nil
}

// apiBase returns the API root for the host the request targets. The host
// rides on the request context (core.HostFromContext) so the same driver
// instance can talk to codeberg.org, a self-hosted Forgejo at
// code.example.com, or a Gitea at gitea.acme.net — each call lands at the
// right "/api/v1" mount. Falls back to codeberg.org when the host is
// missing (tests that construct a driver without going through Push).
func (d *driver) apiBase(ctx context.Context, _ string) string {
	if d.baseURL != "" {
		return d.baseURL
	}
	if host := core.HostFromContext(ctx); host != "" {
		return "https://" + host + "/api/v1"
	}
	return "https://codeberg.org/api/v1"
}

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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", d.userAgent)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := core.TokenFromContext(ctx); tok != "" {
		// Forgejo / Gitea historically accepted `token <secret>` but the
		// modern API supports `Bearer <secret>` for personal access tokens
		// and OAuth alike. Bearer matches the github + gitlab path.
		req.Header.Set("Authorization", "token "+tok)
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
		return fmt.Errorf("forgejo %s %s: HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
