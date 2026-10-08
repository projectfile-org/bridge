// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/forge/core"
)

const magnetLimit = 64 << 10

// PublishOptions adds the release API coordinates to the forge half.
type PublishOptions struct {
	ForgeOptions
	API    string
	Token  string
	DryRun bool
}

type release struct {
	ID     int64  `json:"id"`
	Body   string `json:"body"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Publish appends the forge half to the release of the tag on one forge and returns the new body.
func Publish(ctx context.Context, po PublishOptions) (string, error) {
	if po.Timeout == 0 {
		po.Timeout = defaultTimeout
	}
	client := core.NewHTTPClient(core.HTTPOptions{Timeout: po.Timeout})
	base := strings.TrimSuffix(po.API, "/") + "/repos/" + po.Repo + "/releases"
	var rel release
	if err := call(ctx, client, po.Token, http.MethodGet, base+"/tags/"+po.Tag, nil, &rel); err != nil {
		return "", err
	}
	genlog.Info("release found", "repo", po.Repo, "tag", po.Tag, "id", rel.ID, "assets", len(rel.Assets))
	var listed []struct {
		Tag string `json:"tag_name"`
	}
	if err := call(ctx, client, po.Token, http.MethodGet, base+"?limit=50&per_page=50", nil, &listed); err != nil {
		genlog.Warn("releases unlisted, previous tag read from git", "err", err.Error())
	}
	tags := make([]string, 0, len(listed))
	for _, l := range listed {
		tags = append(tags, l.Tag)
	}
	po.Previous = previousOf(tags, po.Tag, po.Prefix)
	po.Magnets = map[string]string{}
	for _, a := range rel.Assets {
		po.Assets = append(po.Assets, a.Name)
		if !strings.HasSuffix(a.Name, ".magnet") {
			continue
		}
		magnet, err := fetchText(ctx, client, po.Token, a.URL)
		if err != nil {
			genlog.Warn("magnet unreadable, skipped", "asset", a.Name, "err", err.Error())
			continue
		}
		po.Magnets[a.Name] = magnet
	}
	po.Inspect = func(ref string) (ImageInfo, error) { return InspectImage(ctx, client, ref) }
	half, err := RenderForge(po.ForgeOptions)
	if err != nil {
		return "", err
	}
	body := MergeBody(rel.Body, half)
	switch {
	case body == rel.Body:
		genlog.Info("release body unchanged", "tag", po.Tag)
	case po.DryRun:
		genlog.Info("dry run, release body not written", "tag", po.Tag, "bytes", len(body))
	default:
		if err := call(ctx, client, po.Token, http.MethodPatch, fmt.Sprintf("%s/%d", base, rel.ID), map[string]string{"body": body}, nil); err != nil {
			return "", err
		}
		genlog.Info("release body written", "tag", po.Tag, "id", rel.ID, "bytes", len(body))
	}
	return body, nil
}

// call sends one JSON request to the release API and decodes the answer into out.
func call(ctx context.Context, client *http.Client, token, method, url string, in, out any) error {
	var reader io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := core.DoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, url, err)
	}
	raw, err := core.ReadAndClose(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// fetchText downloads a small text asset such as a magnet sidecar.
func fetchText(ctx context.Context, client *http.Client, token, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := core.DoWithRetry(ctx, client, req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, magnetLimit))
	return strings.TrimSpace(string(raw)), err
}
