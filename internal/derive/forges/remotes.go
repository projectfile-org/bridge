// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forges

import (
	"net/url"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
)

// Keys of one derived remote. Named so the deriver, its tests, and the YAML
// fragments that interpolate `${org.projectfile.forge.remotes.<slug>.<key>}`
// share one canonical spelling.
const (
	KeyHost  = "host"
	KeyOwner = "owner"
	KeyRepo  = "repo"
	KeyURL   = "url"
	KeyKind  = "kind"
)

// Remotes resolves every source-code mirror into flat, addressable coordinates,
// keyed by forge slug:
//
//	codeberg: {host: codeberg.org, owner: b19, repo: ubuntu, url: …, kind: forgejo}
//
// What we are trying to do: make ALL the URL string surgery happen once, here,
// so a badge is pure data. `text/template` has no split/trimPrefix and the
// address grammar has no contains operator, so without this pass nothing in a
// template or a YAML fragment could turn a repo URL into the host/owner/repo
// triple that api.reuse.software, shields.io, and every forge badge endpoint
// need. With it, a badge extracts each part BY KEY.
//
// The slug is the FIRST domain label (codeberg.org → codeberg, kiota.ch →
// kiota): deterministic, dot-free, and needing no declaration anywhere. On a
// slug collision the first mirror in document order wins.
//
// kinds is org.projectfile.forge.kinds — the user's authoritative host→kind
// map, which is how a self-hosted instance on a bare hostname (kiota.ch) gets
// classified at all.
//
// The per-remote trace is Info (verbose-gated), NOT Decision. A Decision row is
// a claim about the artefact being generated, and the reader is entitled to read
// the whole column as such; this pass runs on EVERY document read — once per
// bridge in a fan-out — and describes the input rather than any output, so as a
// Decision it printed the same three lines a dozen times and pushed the rows
// that did explain the file off the screen.
func Remotes(pf *projectfile.Document, kinds map[string]string) map[string]any {
	if pf == nil {
		return nil
	}
	out := map[string]any{}
	for i := range pf.Links {
		link := &pf.Links[i]
		if link.Type != projectfile.LinkSourceCode || link.URL == "" {
			continue
		}
		slug, remote := coordinates(link.URL, kinds)
		if slug == "" {
			genlog.Info("forge remote skipped", "url", link.URL, "reason", "no host/owner/repo")
			continue
		}
		if _, taken := out[slug]; taken {
			genlog.Info("forge remote skipped", "slug", slug, "reason", "slug already claimed", "url", link.URL)
			continue
		}
		out[slug] = remote
		genlog.Info("forge remote derived", "slug", slug, "url", remote[KeyURL], "kind", remote[KeyKind])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// coordinates splits one repository URL into its slug and coordinate map.
// Returns ("", nil) for anything a badge endpoint could not be built from: a
// non-web scheme (an ssh:// clone URL), a missing host, a path with no
// owner/repo pair.
func coordinates(raw string, kinds map[string]string) (string, map[string]any) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || !webScheme(parsed.Scheme) {
		return "", nil
	}
	host := strings.ToLower(parsed.Hostname())
	owner, repo := ownerRepo(parsed.Path)
	if owner == "" || repo == "" {
		return "", nil
	}
	return firstLabel(host), map[string]any{
		KeyHost:  host,
		KeyOwner: owner,
		KeyRepo:  repo,
		// Rebuilt rather than echoed so every consumer sees one canonical
		// spelling — no trailing slash, no .git, no query string.
		KeyURL:  parsed.Scheme + "://" + parsed.Host + "/" + owner + "/" + repo,
		KeyKind: string(hostmatch.ResolveKindWithKinds(raw, kinds)),
	}
}

// webScheme reports whether a URL can address a forge's web surface. Every
// coordinate this pass produces feeds an http(s) endpoint — a badge image, an
// API path, a browse link — so an ssh:// clone URL is not usable here even
// though it names the same repository.
func webScheme(scheme string) bool {
	return scheme == "https" || scheme == "http"
}

// ownerRepo splits a repository path into (owner, repo). The LAST segment is
// the repo and everything before it the owner, so a GitLab subgroup path
// (group/sub/proj) and a sourcehut tilde path (~user/repo) both survive intact.
func ownerRepo(path string) (owner, repo string) {
	trimmed := strings.Trim(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	if trimmed == "" {
		return "", ""
	}
	cut := strings.LastIndex(trimmed, "/")
	if cut < 0 {
		return "", ""
	}
	return trimmed[:cut], trimmed[cut+1:]
}

// firstLabel is the forge slug rule: the first label of the host, with any
// leading www. dropped. codeberg.org → codeberg, kiota.ch → kiota,
// gitlab.example.com → gitlab.
func firstLabel(host string) string {
	host = strings.TrimPrefix(host, "www.")
	if cut := strings.Index(host, "."); cut > 0 {
		return host[:cut]
	}
	return host
}
