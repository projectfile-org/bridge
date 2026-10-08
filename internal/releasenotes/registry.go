// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/forge/core"
)

const manifestAccept = "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"

// Platform is one architecture of a multi-arch image: its manifest digest and compressed size.
type Platform struct {
	Name   string
	Digest string
	Size   int64
}

// ImageInfo is what a registry holds for one image reference.
type ImageInfo struct {
	Digest    string
	Platforms []Platform
	// Packages maps a package name to its sorted, comma-joined versions, read from the image's attested SBOM; nil when none.
	Packages map[string]string
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Platform  struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Variant      string `json:"variant"`
	} `json:"platform"`
}

type manifest struct {
	MediaType string       `json:"mediaType"`
	Config    descriptor   `json:"config"`
	Layers    []descriptor `json:"layers"`
	Manifests []descriptor `json:"manifests"`
}

// splitRef cuts a `host/path:tag` reference into registry host, repository and tag.
func splitRef(ref string) (host, repo, tag string, err error) {
	host, rest, ok := strings.Cut(ref, "/")
	if !ok {
		return "", "", "", fmt.Errorf("image ref %q has no registry host", ref)
	}
	repo, tag, _ = strings.Cut(rest, "@")
	if i := strings.LastIndex(repo, ":"); i >= 0 {
		repo, tag = repo[:i], repo[i+1:]
	}
	if repo == "" || tag == "" {
		return "", "", "", fmt.Errorf("image ref %q has no repository or tag", ref)
	}
	return host, repo, tag, nil
}

// InspectImage reads the index digest and per-platform digests and sizes of ref from its registry, anonymously.
func InspectImage(ctx context.Context, client *http.Client, ref string) (ImageInfo, error) {
	host, repo, tag, err := splitRef(ref)
	if err != nil {
		return ImageInfo{}, err
	}
	scheme := "https"
	if strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "localhost") {
		scheme = "http"
	}
	base := scheme + "://" + host + "/v2/" + repo + "/manifests/"
	token := ""
	raw, digest, err := fetchManifest(ctx, client, base+tag, &token)
	if err != nil {
		return ImageInfo{}, err
	}
	var top manifest
	if err := json.Unmarshal(raw, &top); err != nil {
		return ImageInfo{}, fmt.Errorf("manifest of %s: %w", ref, err)
	}
	info := ImageInfo{Digest: digest}
	if info.Packages, err = attestedPackages(ctx, client, scheme+"://"+host+"/v2/"+repo, digest, &token); err != nil {
		genlog.Debug("sbom attestation unreadable, no package delta", "ref", ref, "digest", digest, "err", err.Error())
	}
	for _, m := range top.Manifests {
		// Attestation and signature manifests carry no platform; they are not architectures.
		if m.Platform.Architecture == "" || m.Platform.Architecture == "unknown" {
			genlog.Debug("manifest skipped, no platform", "ref", ref, "digest", m.Digest)
			continue
		}
		childRaw, _, err := fetchManifest(ctx, client, base+m.Digest, &token)
		if err != nil {
			genlog.Warn("platform manifest unreadable, size omitted", "ref", ref, "digest", m.Digest, "err", err.Error())
			info.Platforms = append(info.Platforms, Platform{Name: platformName(m), Digest: m.Digest})
			continue
		}
		var child manifest
		if err := json.Unmarshal(childRaw, &child); err != nil {
			return ImageInfo{}, fmt.Errorf("manifest %s of %s: %w", m.Digest, ref, err)
		}
		size := child.Config.Size
		for _, l := range child.Layers {
			size += l.Size
		}
		info.Platforms = append(info.Platforms, Platform{Name: platformName(m), Digest: m.Digest, Size: size})
		genlog.Debug("platform read", "ref", ref, "platform", platformName(m), "digest", m.Digest, "size", size)
	}
	return info, nil
}

// platformName renders os/arch[/variant].
func platformName(d descriptor) string {
	name := d.Platform.OS + "/" + d.Platform.Architecture
	if d.Platform.Variant != "" {
		name += "/" + d.Platform.Variant
	}
	return name
}

// fetchManifest GETs one manifest, answering a Bearer challenge once with an anonymous token.
func fetchManifest(ctx context.Context, client *http.Client, target string, token *string) ([]byte, string, error) {
	for attempt := range 2 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Accept", manifestAccept)
		if *token != "" {
			req.Header.Set("Authorization", "Bearer "+*token)
		}
		resp, err := core.DoWithRetry(ctx, client, req)
		if err != nil {
			return nil, "", fmt.Errorf("GET %s: %w", target, err)
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			challenge := resp.Header.Get("Www-Authenticate")
			_, _ = core.ReadAndClose(resp)
			if *token, err = anonymousToken(ctx, client, challenge); err != nil {
				return nil, "", err
			}
			genlog.Debug("registry token fetched", "target", target)
			continue
		}
		digest := resp.Header.Get("Docker-Content-Digest")
		raw, err := core.ReadAndClose(resp)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode/100 != 2 {
			return nil, "", fmt.Errorf("GET %s: HTTP %d", target, resp.StatusCode)
		}
		if digest == "" {
			digest = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
		}
		return raw, digest, nil
	}
	return nil, "", fmt.Errorf("GET %s: unauthorized", target)
}

// anonymousToken answers a `Bearer realm=…,service=…,scope=…` challenge without credentials.
func anonymousToken(ctx context.Context, client *http.Client, challenge string) (string, error) {
	params := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(challenge, "Bearer "), ",") {
		if k, v, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			params[k] = strings.Trim(v, `"`)
		}
	}
	if params["realm"] == "" {
		return "", fmt.Errorf("registry challenge %q names no token realm", challenge)
	}
	q := url.Values{}
	for _, k := range []string{"service", "scope"} {
		if params[k] != "" {
			q.Set(k, params[k])
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params["realm"]+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := core.DoWithRetry(ctx, client, req)
	if err != nil {
		return "", err
	}
	raw, err := core.ReadAndClose(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("token %s: HTTP %d", params["realm"], resp.StatusCode)
	}
	var tok struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", err
	}
	if tok.Token == "" {
		tok.Token = tok.AccessToken
	}
	return tok.Token, nil
}

// attestedPackages reads the CycloneDX packages of the cosign attestation stored under the image digest's `.att` tag.
func attestedPackages(ctx context.Context, client *http.Client, repoURL, digest string, token *string) (map[string]string, error) {
	raw, _, err := fetchManifest(ctx, client, repoURL+"/manifests/"+strings.Replace(digest, ":", "-", 1)+".att", token)
	if err != nil {
		return nil, err
	}
	var att manifest
	if err := json.Unmarshal(raw, &att); err != nil {
		return nil, err
	}
	versions := map[string][]string{}
	for _, layer := range att.Layers {
		blob, _, err := fetchManifest(ctx, client, repoURL+"/blobs/"+layer.Digest, token)
		if err != nil {
			return nil, err
		}
		addComponents(blob, versions)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("attestation %s holds no CycloneDX components", digest)
	}
	out := make(map[string]string, len(versions))
	for name, v := range versions {
		slices.Sort(v)
		out[name] = strings.Join(slices.Compact(v), ", ")
	}
	return out, nil
}

// addComponents collects name and version of every non-file component of one DSSE-wrapped CycloneDX statement.
func addComponents(envelope []byte, into map[string][]string) {
	var env struct {
		Payload string `json:"payload"`
	}
	if json.Unmarshal(envelope, &env) != nil {
		return
	}
	payload, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		return
	}
	var stmt struct {
		Predicate json.RawMessage `json:"predicate"`
	}
	if json.Unmarshal(payload, &stmt) != nil {
		return
	}
	var bom struct {
		Components []struct{ Type, Name, Version string } `json:"components"`
		Data       json.RawMessage                        `json:"Data"`
	}
	if json.Unmarshal(stmt.Predicate, &bom) != nil {
		return
	}
	// Older cosign wraps the BOM as predicate.Data, sometimes as a JSON string.
	if len(bom.Components) == 0 && len(bom.Data) > 0 {
		inner := bom.Data
		var str string
		if json.Unmarshal(inner, &str) == nil {
			inner = []byte(str)
		}
		_ = json.Unmarshal(inner, &bom)
	}
	for _, c := range bom.Components {
		if c.Type != "file" && c.Name != "" {
			into[c.Name] = append(into[c.Name], c.Version)
		}
	}
}
