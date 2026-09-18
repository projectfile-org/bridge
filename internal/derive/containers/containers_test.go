// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package containers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/containers"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// keyRef is the sink-entry ref key, pulled out once (goconst).
const keyRef = "ref"

// sinkDockerHub is the sink name used across the DockerHub test cases (goconst).
const sinkDockerHub = "dockerhub"

// parts mirrors the fleet's b19/ubuntu image vocabulary (see ocisinks_test.go).
func parts() map[string]any {
	return map[string]any{
		"org":      "b19",
		"name":     "${identity.name}",
		"path":     "${org}/${name}",
		"flatpath": "${org}-${name}",
		"tag":      "latest",
	}
}

// docWith builds a document carrying identity, image parts, and given sinks/links.
func docWith(sinks map[string]any, links ...projectfile.Link) *projectfile.Document {
	doc := &projectfile.Document{Identity: projectfile.Identity{Namespace: "org.b19", Name: "ubuntu"}, Links: links}
	projectfile.SetExtension(doc, pfmodel.ImageExtensionNS, parts())
	projectfile.SetExtension(doc, pfmodel.SinksExtensionNS, sinks)
	return doc
}

func githubLink() projectfile.Link {
	return projectfile.Link{Type: projectfile.LinkSourceCode, URL: "https://github.com/damian-buho/b19-ubuntu"}
}

func TestDeriveGHCRUsesSourceRepoAndEncodesNesting(t *testing.T) {
	doc := docWith(map[string]any{
		"ghcr": map[string]any{keyRef: "ghcr.io/damian-buho/${path}:${tag}"},
	}, githubLink())

	got := containers.Derive(doc)

	require.Len(t, got, 1)
	assert.Equal(t, "https://github.com/damian-buho/b19-ubuntu/pkgs/container/b19%2Fubuntu", got[0].URL)
	assert.Equal(t, containers.LinkPackageRegistry, got[0].Type)
}

func TestDeriveGHCRSkippedWithoutGitHubSourceLink(t *testing.T) {
	doc := docWith(map[string]any{
		"ghcr": map[string]any{keyRef: "ghcr.io/damian-buho/${path}:${tag}"},
	})

	assert.Empty(t, containers.Derive(doc))
}

func TestDeriveDockerHubFlatPath(t *testing.T) {
	doc := docWith(map[string]any{
		sinkDockerHub: map[string]any{keyRef: "docker.io/damian-buho/${flatpath}:${tag}"},
	}, githubLink())

	got := containers.Derive(doc)

	require.Len(t, got, 1)
	assert.Equal(t, "https://hub.docker.com/r/damian-buho/b19-ubuntu", got[0].URL)
}

func TestDeriveDockerHubOfficialImage(t *testing.T) {
	doc := docWith(map[string]any{
		sinkDockerHub: map[string]any{keyRef: "docker.io/library/${name}:${tag}"},
	})

	got := containers.Derive(doc)

	require.Len(t, got, 1)
	assert.Equal(t, "https://hub.docker.com/_/ubuntu", got[0].URL)
}

func TestDerivePrivateForgeSinkIsIgnored(t *testing.T) {
	doc := docWith(map[string]any{
		"kiota": map[string]any{keyRef: "kiota.ch/${path}:${tag}"},
	})

	assert.Empty(t, containers.Derive(doc))
}

func TestDeriveNoSinksComposesNothing(t *testing.T) {
	assert.Empty(t, containers.Derive(&projectfile.Document{}))
}

// seriesParts mirrors b19/ubuntu's own image vocabulary: one image built once
// per Ubuntu series, the series itself a matrix placeholder rather than a
// literal (see m6e/b19/images/ubuntu.yaml).
func seriesParts() map[string]any {
	return map[string]any{
		"org":      "b19",
		"name":     "${identity.name}",
		"series":   "{B19_UBUNTU_SERIES}",
		"flatpath": "${org}-${name}-${series}",
		"tag":      "latest",
	}
}

func docWithSeries(sinks, ciAxes map[string]any) *projectfile.Document {
	doc := &projectfile.Document{Identity: projectfile.Identity{Namespace: "org.b19", Name: "ubuntu"}}
	projectfile.SetExtension(doc, pfmodel.ImageExtensionNS, seriesParts())
	projectfile.SetExtension(doc, pfmodel.SinksExtensionNS, sinks)
	if ciAxes != nil {
		projectfile.SetExtension(doc, "org.projectfile.ci", map[string]any{"matrix": map[string]any{"axes": ciAxes}})
	}
	return doc
}

// TestDeriveFansOutOnePerMatrixAxisValue is the regression case for the
// b19/ubuntu bug: a sink built once per series must yield one link per
// declared series, never a single link carrying the raw `{AXIS}` placeholder.
func TestDeriveFansOutOnePerMatrixAxisValue(t *testing.T) {
	doc := docWithSeries(
		map[string]any{sinkDockerHub: map[string]any{keyRef: "docker.io/damianbuho/${flatpath}:${tag}"}},
		map[string]any{"B19_UBUNTU_SERIES": []any{"resolute", "noble"}},
	)

	got := containers.Derive(doc)

	require.Len(t, got, 2)
	urls := []string{got[0].URL, got[1].URL}
	assert.ElementsMatch(t, []string{
		"https://hub.docker.com/r/damianbuho/b19-ubuntu-resolute",
		"https://hub.docker.com/r/damianbuho/b19-ubuntu-noble",
	}, urls)
	for _, u := range urls {
		assert.NotContains(t, u, "{", "a published link must never carry a raw axis placeholder")
	}
}

// TestDeriveDropsLinkWithUndeclaredAxis: an axis placeholder the document
// declares no matrix for addresses nothing, so the link is dropped rather
// than published broken.
func TestDeriveDropsLinkWithUndeclaredAxis(t *testing.T) {
	doc := docWithSeries(
		map[string]any{sinkDockerHub: map[string]any{keyRef: "docker.io/damianbuho/${flatpath}:${tag}"}},
		nil,
	)

	assert.Empty(t, containers.Derive(doc))
}
