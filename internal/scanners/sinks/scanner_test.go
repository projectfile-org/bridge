// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package sinks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"projectfile.org/projectfile/bridge/internal/scanners/sinks"
)

const document = `---
identity:
  name: go
  title: B19 / Go
includes:
  - fragment.yaml
org:
  projectfile:
    image:
      org: b19
      name: ${identity.name}
      path: ${org}/${name}
      flatpath: ${org}-${name}
      tag: latest
    forge:
      org: b19
      name: ${identity.name}
      path: ${org}/${name}
      flatpath: ${org}-${name}
    sinks:
      ghcr:
        ref: ghcr.io/damian-buho/${path}:${tag}
      dockerhub:
        ref: docker.io/damianbuho/${flatpath}:${tag}
      kiota:
        ref: kiota.ch/${path}:${tag}
`

// fragment carries the GitHub link as a forge template only, the way the fleet declares it.
const fragment = `---
org:
  projectfile:
    forge:
      links:
        - type: source-code
          url: https://github.com/damian-buho/${flatpath}
`

// write lays the scratch project down: the document plus the fragment it includes.
func write(t *testing.T, doc, frag string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fragment.yaml"), []byte(frag), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(doc), 0o600))
	return dir
}

// One link per public sink; the private kiota sink yields none; GHCR resolves through the staged forge link.
func TestScanDerivesPublicRegistryPages(t *testing.T) {
	p, hits, err := sinks.Scanner{}.Scan(write(t, document, fragment))
	require.NoError(t, err)

	urls := map[string]bool{}
	for _, l := range p.Links {
		assert.Equal(t, "package-registry", l.Type)
		urls[l.URL] = true
	}
	assert.Len(t, p.Links, 2)
	assert.True(t, urls["https://github.com/damian-buho/b19-go/pkgs/container/b19%2Fgo"], "ghcr page under the staged github repo")
	assert.True(t, urls["https://hub.docker.com/r/damianbuho/b19-go"], "docker hub page")
	assert.Len(t, hits, 2)
	assert.Equal(t, "sinks", hits[0].Source)
}

// Without a GitHub repository nothing addresses the GHCR page, so only Docker Hub survives.
func TestScanSkipsGHCRWithoutGitHubLink(t *testing.T) {
	p, _, err := sinks.Scanner{}.Scan(write(t, document, "---\n{}\n"))
	require.NoError(t, err)

	require.Len(t, p.Links, 1)
	assert.Equal(t, "https://hub.docker.com/r/damianbuho/b19-go", p.Links[0].URL)
}

// A document with no sinks proposes nothing and errs on nothing.
func TestScanNoSinksIsQuiet(t *testing.T) {
	p, hits, err := sinks.Scanner{}.Scan(write(t, "---\nidentity:\n  name: x\n", "---\n{}\n"))
	require.NoError(t, err)
	assert.Empty(t, p.Links)
	assert.Empty(t, hits)
}
