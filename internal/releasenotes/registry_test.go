// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureArch  = "architecture"
	fixtureSize  = "size"
	fixtureRepo  = "o/x"
	fixtureDoc   = "projectfile.yaml"
	fixtureHost  = "https://kiota.example"
	fixtureDig   = "digest"
	fixtureSSL   = "openssl"
	fixtureLay   = "layers"
	fixtureVer   = "3.0.14"
	fixtureAmd   = "linux/amd64"
	fixturePlat  = "platform"
	fixtureForge = "kiota"
)

func TestInspectImageReadsPlatformsBehindTokenChallenge(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "t0k"})
			return
		}
		if req.Header.Get("Authorization") != "Bearer t0k" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+srv.URL+`/token",service="reg",scope="repository:o/x:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(req.URL.Path, "/manifests/1.2.0"):
			w.Header().Set("Docker-Content-Digest", "sha256:index")
			_ = json.NewEncoder(w).Encode(map[string]any{"manifests": []map[string]any{
				{fixtureDig: "sha256:amd", fixturePlat: map[string]string{"os": "linux", fixtureArch: "amd64"}},
				{fixtureDig: "sha256:arm", fixturePlat: map[string]string{"os": "linux", fixtureArch: "arm64", "variant": "v8"}},
				{fixtureDig: "sha256:att", fixturePlat: map[string]string{"os": "unknown", fixtureArch: "unknown"}},
			}})
		case strings.HasSuffix(req.URL.Path, "/manifests/sha256:amd"):
			_ = json.NewEncoder(w).Encode(map[string]any{"config": map[string]int{fixtureSize: 100}, fixtureLay: []map[string]int{{fixtureSize: 900}, {fixtureSize: 1000}}})
		case strings.HasSuffix(req.URL.Path, "/manifests/sha256:arm"):
			_ = json.NewEncoder(w).Encode(map[string]any{"config": map[string]int{fixtureSize: 50}, fixtureLay: []map[string]int{{fixtureSize: 450}}})
		case strings.HasSuffix(req.URL.Path, "/manifests/sha256-index.att"):
			_ = json.NewEncoder(w).Encode(map[string]any{fixtureLay: []map[string]string{{fixtureDig: "sha256:sbom"}}})
		case strings.HasSuffix(req.URL.Path, "/blobs/sha256:sbom"):
			stmt := `{"predicate":{"components":[{"type":"library","name":"openssl","version":"3.0.14"}]}}`
			_ = json.NewEncoder(w).Encode(map[string]string{"payload": base64.StdEncoding.EncodeToString([]byte(stmt))})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	info, err := InspectImage(context.Background(), srv.Client(), strings.TrimPrefix(srv.URL, "http://")+"/o/x:1.2.0")
	require.NoError(t, err)
	assert.Equal(t, "sha256:index", info.Digest)
	assert.Equal(t, map[string]string{fixtureSSL: fixtureVer}, info.Packages)
	assert.Equal(t, []Platform{
		{Name: fixtureAmd, Digest: "sha256:amd", Size: 2000},
		{Name: "linux/arm64/v8", Digest: "sha256:arm", Size: 500},
	}, info.Platforms)
}

func TestSplitRef(t *testing.T) {
	host, repo, tag, err := splitRef("ghcr.io/o/x:1.2.0")
	require.NoError(t, err)
	assert.Equal(t, []string{"ghcr.io", fixtureRepo, "1.2.0"}, []string{host, repo, tag})
	_, _, _, err = splitRef("x:1.2.0")
	assert.Error(t, err)
	assert.Equal(t, "ghcr.io:5000/o/x", repoOf("ghcr.io:5000/o/x:1.2.0"))
}

func TestRenderForgePinsDigests(t *testing.T) {
	r := newRepo(t)
	r.commit("Ann", "feat: first", map[string]string{fixtureDoc: forgeDoc})
	r.git("Ann", "tag", fixtureTag)
	got, err := RenderForge(ForgeOptions{
		Options: Options{Dir: r.dir, Tag: fixtureTag, Prefix: "v"},
		Forge:   fixtureForge, Server: fixtureHost, Repo: fixtureRepo,
		Previous: "v1.1.0",
		Inspect: func(ref string) (ImageInfo, error) {
			if strings.HasSuffix(ref, ":1.1.0") {
				return ImageInfo{Digest: "sha256:old", Platforms: []Platform{{Name: fixtureAmd, Digest: "sha256:ghi", Size: 3_000_000}}}, nil
			}
			return ImageInfo{Digest: "sha256:abc", Platforms: []Platform{{Name: fixtureAmd, Digest: "sha256:def", Size: 2_500_000}}}, nil
		},
	})
	require.NoError(t, err)
	assert.Contains(t, got, "docker pull kiota.example/x@sha256:abc\n")
	assert.Contains(t, got, "| linux/amd64 | `sha256:def` | 2.5 MB (−0.5 MB) |\n")
	assert.Contains(t, got, "--insecure-ignore-tlog=true kiota.example/x@sha256:abc\n")
	assert.NotContains(t, got, "ghcr.example")
}

func TestWritePackageDelta(t *testing.T) {
	var b strings.Builder
	writePackageDelta(&b, map[string]string{fixtureSSL: "3.0.13", "zlib": "1.3", "old": "1"}, map[string]string{fixtureSSL: fixtureVer, "zlib": "1.3", "new": "2"})
	assert.Contains(t, b.String(), "1 updated, 1 added, 1 removed")
	assert.Contains(t, b.String(), "- openssl 3.0.13 → 3.0.14\n")
	assert.NotContains(t, b.String(), "zlib")
	b.Reset()
	writePackageDelta(&b, map[string]string{"a": "1"}, map[string]string{"a": "1"})
	assert.Contains(t, b.String(), "No package changed")
	b.Reset()
	writePackageDelta(&b, nil, map[string]string{"a": "1"})
	assert.Empty(t, b.String())
}

func TestAddComponentsReadsDSSEPayload(t *testing.T) {
	stmt := `{"predicate":{"components":[{"type":"library","name":"openssl","version":"3.0.14"},{"type":"file","name":"/etc/x"}]}}`
	env, err := json.Marshal(map[string]string{"payload": base64.StdEncoding.EncodeToString([]byte(stmt))})
	require.NoError(t, err)
	got := map[string][]string{}
	addComponents(env, got)
	assert.Equal(t, map[string][]string{fixtureSSL: {fixtureVer}}, got)
}
