// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package gitlab implements the forge.Client interface for gitlab.com and
// any self-hosted GitLab instance. Single endpoint: PUT /projects/{id}
// accepts description + topics in one request. Homepage is *not* exposed
// via the API — the algorithm logs that asymmetry and moves on.
package gitlab

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
	// baseURL overrides the canonical gitlab.com/api/v4 root for tests.
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

func (d *driver) Kind() string { return string(hostmatch.KindGitLab) }

// Owner returns the full URL-encoded project path. GitLab supports nested
// groups (group/subgroup/.../project), all of which join with '/' as the
// API's `:id` segment. We keep the raw path here and let the request
// helper PathEscape it once at request time.
func (d *driver) Owner(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("parse %q: %w", repoURL, err)
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", "", fmt.Errorf("gitlab url %q has empty path", repoURL)
	}
	// Strip trailing ".git" from clone URLs; the API rejects it.
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("gitlab url %q lacks group/project path", repoURL)
	}
	// `owner` is everything before the last segment, `repo` is the last.
	// Keeps consistency with github's two-value return; the request URL
	// joins them back together with '/' before encoding.
	return strings.Join(parts[:len(parts)-1], "/"), parts[len(parts)-1], nil
}

// projectID joins owner + repo into the URL-encoded `:id` parameter the
// GitLab API expects ("group%2Fsubgroup%2Fproject"). Done in one place so
// the encoding logic isn't duplicated across Fetch and Apply.
func (d *driver) projectID(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

func (d *driver) Fetch(ctx context.Context, owner, repo string) (core.Snapshot, error) {
	base := d.apiBase(ctx)
	var meta struct {
		Description string   `json:"description"`
		Topics      []string `json:"topics"`
	}
	url := fmt.Sprintf("%s/projects/%s", base, d.projectID(owner, repo))
	if err := d.do(ctx, http.MethodGet, url, nil, &meta); err != nil {
		return core.Snapshot{}, err
	}
	return core.Snapshot{
		Description: meta.Description,
		// Homepage intentionally left blank — GitLab has no homepage slot
		// at the project level, so we never surface drift on this field.
		Topics: core.LowerTopics(meta.Topics),
	}, nil
}

// Apply combines description + topics into a single PUT. Homepage is
// silently dropped here even when present in the patch — diffWithFilter
// already records the skip; the network layer just enforces it.
func (d *driver) Apply(ctx context.Context, owner, repo string, patch core.Patch) error {
	body := map[string]any{}
	if patch.Description != nil {
		body["description"] = *patch.Description
	}
	if patch.Topics != nil {
		body["topics"] = *patch.Topics
	}
	if len(body) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/projects/%s", d.apiBase(ctx), d.projectID(owner, repo))
	return d.do(ctx, http.MethodPut, url, body, nil)
}

// apiBase returns the GitLab API root. gitlab.com uses gitlab.com/api/v4;
// a self-hosted instance uses "<host>/api/v4" — every GitLab installation
// mounts its API at the same suffix regardless of host. The host comes
// from the request context so the same driver instance can talk to
// multiple GitLab installations from one process.
func (d *driver) apiBase(ctx context.Context) string {
	if d.baseURL != "" {
		return d.baseURL
	}
	host := core.HostFromContext(ctx)
	if host == "" {
		host = "gitlab.com"
	}
	return "https://" + host + "/api/v4"
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
		// GitLab supports both Bearer and the legacy PRIVATE-TOKEN header;
		// Bearer is the modern path and works for personal access tokens,
		// OAuth tokens, and project access tokens uniformly.
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
		return fmt.Errorf("gitlab %s %s: HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
