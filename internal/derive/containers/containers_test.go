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
	assert.Equal(t, "https://github.com/damian-buho/b19-ubuntu/pkgs/container/b19%2Fubuntu", got[0].NewValue)
	assert.Equal(t, "links[type=package-registry,url="+got[0].NewValue+"]", got[0].FieldPath)
}

func TestDeriveGHCRSkippedWithoutGitHubSourceLink(t *testing.T) {
	doc := docWith(map[string]any{
		"ghcr": map[string]any{keyRef: "ghcr.io/damian-buho/${path}:${tag}"},
	})

	assert.Empty(t, containers.Derive(doc))
}

func TestDeriveDockerHubFlatPath(t *testing.T) {
	doc := docWith(map[string]any{
		"dockerhub": map[string]any{keyRef: "docker.io/damian-buho/${flatpath}:${tag}"},
	}, githubLink())

	got := containers.Derive(doc)

	require.Len(t, got, 1)
	assert.Equal(t, "https://hub.docker.com/r/damian-buho/b19-ubuntu", got[0].NewValue)
}

func TestDeriveDockerHubOfficialImage(t *testing.T) {
	doc := docWith(map[string]any{
		"dockerhub": map[string]any{keyRef: "docker.io/library/${name}:${tag}"},
	})

	got := containers.Derive(doc)

	require.Len(t, got, 1)
	assert.Equal(t, "https://hub.docker.com/_/ubuntu", got[0].NewValue)
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
