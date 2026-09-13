// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package containers infers Docker Hub / GHCR landing-page links from a project's resolved OCI sinks.
package containers

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/ocisinks"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Change matches forges.Change one-for-one so the engine folds both passes uniformly.
type Change struct {
	FieldPath string
	NewValue  string
	Source    string
	Label     *projectfile.LocalizedString
}

// hostDockerHub is the Docker Hub registry host as it appears in a composed sink ref.
const hostDockerHub = "docker.io"

// hostGHCR is the GitHub Container Registry host as it appears in a composed sink ref.
const hostGHCR = "ghcr.io"

// ciExtensionNS is the CI namespace read for the matrix axes. pfmodel exports no
// constant for it (CI is owned by the resolver, not a bridge), so this package
// names it locally, same as the readme bridge does.
const ciExtensionNS = "org.projectfile.ci"

// Derive returns one package-registry link per public registry the project's
// sinks resolve to, fanned out one per matrix cell when the sink's ref is
// built per axis (e.g. a base image published once per `{B19_UBUNTU_SERIES}`).
// A link still carrying the raw `{AXIS}` placeholder addresses nothing, so an
// axis the document does not declare drops the link rather than publish it.
func Derive(pf *projectfile.Document) []Change {
	refs := ocisinks.Refs(pf)
	if len(refs) == 0 {
		return nil
	}
	axes := pfmodel.MatrixAxes(pf, ciExtensionNS)
	var out []Change
	seen := map[string]bool{}
	for name, v := range refs {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		ref, _ := entry[pfmodel.SinkRefKey].(string)
		host, repo := splitRef(ref)
		if host == "" || repo == "" {
			continue
		}
		value, registryLabel := resolveRegistry(pf, host, repo)
		if value == "" {
			continue
		}
		for _, expanded := range pfmodel.ExpandAxes([]string{value}, axes) {
			if strings.Contains(expanded, "{") {
				genlog.Warn("derive: package-registry link left an axis placeholder unresolved — link dropped",
					"sink", name, "url", expanded, "remedy", "declare the axis under "+ciExtensionNS+".matrix.axes")
				continue
			}
			if seen[expanded] {
				continue
			}
			seen[expanded] = true
			out = append(out, Change{
				FieldPath: "links[type=package-registry,url=" + expanded + "]",
				NewValue:  expanded,
				Source:    "sink:" + name,
				Label:     registryLabel,
			})
		}
	}
	return out
}

// resolveRegistry dispatches host to its landing-page builder, returning "" when the host isn't a known public registry.
func resolveRegistry(pf *projectfile.Document, host, repo string) (url string, label *projectfile.LocalizedString) {
	switch host {
	case hostDockerHub:
		return dockerHubURL(repo), pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounPackages), "Docker Hub")
	case hostGHCR:
		owner, ghRepo := githubOwnerRepo(pf)
		if owner == "" {
			return "", nil
		}
		return ghcrURL(owner, ghRepo, ghcrPackagePath(repo)), pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounPackages), "GHCR")
	default:
		return "", nil
	}
}

// splitRef pulls (host, repo-path-without-tag) out of a composed sink ref such as "ghcr.io/org/name:latest".
func splitRef(ref string) (host, repo string) {
	ref = strings.TrimPrefix(ref, "https://")
	ref = strings.TrimPrefix(ref, "http://")
	slash := strings.Index(ref, "/")
	if slash < 0 {
		return "", ""
	}
	host, rest := ref[:slash], ref[slash+1:]
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[:at]
	} else if colon := strings.LastIndex(rest, ":"); colon >= 0 {
		rest = rest[:colon]
	}
	return host, rest
}

// dockerHubURL maps a Docker Hub repo path to its web page: "/_/<name>" for an official image, "/r/<ns>/<name>" otherwise.
func dockerHubURL(repo string) string {
	repo = strings.TrimPrefix(repo, "library/")
	if !strings.Contains(repo, "/") {
		return "https://hub.docker.com/_/" + repo
	}
	return "https://hub.docker.com/r/" + repo
}

// ghcrPackagePath drops repo's leading GHCR-account segment, leaving the package path GitHub actually shows.
func ghcrPackagePath(repo string) string {
	if cut := strings.Index(repo, "/"); cut >= 0 {
		return repo[cut+1:]
	}
	return ""
}

// ghcrURL builds the one stable GHCR web address, %2F-encoding imagePath's slashes for a nested package.
func ghcrURL(owner, ghRepo, imagePath string) string {
	if imagePath == "" {
		return ""
	}
	return "https://github.com/" + owner + "/" + ghRepo + "/pkgs/container/" + strings.ReplaceAll(imagePath, "/", "%2F")
}

// githubOwnerRepo reads the project's own github.com source-code link, so GHCR is never addressed by a guessed owner.
func githubOwnerRepo(pf *projectfile.Document) (owner, repo string) {
	for i := range pf.Links {
		l := &pf.Links[i]
		if l.Type != projectfile.LinkSourceCode {
			continue
		}
		rest, ok := strings.CutPrefix(l.URL, "https://github.com/")
		if !ok {
			continue
		}
		rest = strings.TrimSuffix(strings.Trim(rest, "/"), ".git")
		cut := strings.LastIndex(rest, "/")
		if cut < 0 {
			continue
		}
		return rest[:cut], rest[cut+1:]
	}
	return "", ""
}
