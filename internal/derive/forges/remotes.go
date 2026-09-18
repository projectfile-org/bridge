// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forges

import (
	"net/url"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
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
// slug collision the first mirror in document order wins; an alias goes to
// the highest link `priority`, ties in document order.
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
	slugs := map[string]bool{}
	var claims []claim
	for i := range pf.Links {
		link := &pf.Links[i]
		if link.Type != projectfile.LinkSourceCode || link.URL == "" {
			continue
		}
		slug, remote := coordinates(link.URL, kinds)
		if slug == "" {
			genlog.Debug("forge remote skipped", "url", link.URL, "reason", "no host/owner/repo")
			continue
		}
		if _, taken := out[slug]; taken {
			genlog.Debug("forge remote skipped", "slug", slug, "reason", "slug already claimed", "url", link.URL)
			continue
		}
		out[slug] = remote
		slugs[slug] = true
		genlog.Debug("forge remote derived", "slug", slug, "url", remote[KeyURL], "kind", remote[KeyKind])
		claims = append(claims, claimsFor(link, remote)...)
	}
	if len(out) == 0 {
		return nil
	}
	claims = append(claims, issuesClaim(pf, out)...)
	applyAliases(out, slugs, append(claims, releasesClaims(pf, out)...))
	return out
}

// claim is one alias a link asks for, carried with the remote it names so the
// apply pass needs no second lookup.
type claim struct {
	alias    string
	remote   map[string]any
	priority int
}

// AliasPreferred is the alias every document gets for free from
// links[].preferred — the §5.11 canonical entry of its type. Named so a
// fragment can address "the mirror this project calls its own" without the
// project declaring a tag for it.
const AliasPreferred = "preferred"

// AliasIssues is the alias derived from repositories[].issues (spec §4.3a).
// It comes from the repository list rather than a tag so the bug tracker is
// declared exactly once, in the field the spec already reserves for it.
const AliasIssues = "issues"

// claimsFor returns the aliases one source-code link asks for: every capability
// tag it declares, plus `preferred` when it is the §5.11 canonical entry.
func claimsFor(link *projectfile.Link, remote map[string]any) []claim {
	var out []claim
	priority := pfmodel.LinkPriority(*link)
	for _, tag := range pfmodel.LinkTags(*link) {
		out = append(out, claim{alias: tag, remote: remote, priority: priority})
	}
	if link.Preferred {
		out = append(out, claim{alias: AliasPreferred, remote: remote, priority: priority})
	}
	return out
}

// issuesClaim resolves repositories[issues=true] to the remote that already
// carries its coordinates. The repository list holds the truthful CLONE
// endpoint, which is usually ssh and therefore has no badge-usable form of its
// own; matching it to a derived remote by host/owner/repo is what turns the
// existing declaration into an addressable alias without restating it as a tag.
func issuesClaim(pf *projectfile.Document, out map[string]any) []claim {
	ir := pfmodel.IssuesRepository(pf)
	if ir == nil || !ir.Issues {
		return nil
	}
	host, owner, repo := hostOwnerRepo(ir.URL)
	if host == "" {
		genlog.Debug("forge issues alias skipped", "url", ir.URL, "reason", "no host/owner/repo")
		return nil
	}
	for slug, v := range out {
		remote, ok := v.(map[string]any)
		if !ok || remote[KeyHost] != host || remote[KeyOwner] != owner || remote[KeyRepo] != repo {
			continue
		}
		genlog.Debug("forge issues alias matched", "slug", slug, "url", ir.URL)
		return []claim{{alias: AliasIssues, remote: remote, priority: pfmodel.PriorityDefault}}
	}
	genlog.Debug("forge issues alias skipped", "url", ir.URL, "reason", "no source-code link matches")
	return nil
}

// AliasReleases is the alias derived from repositories[].releases, the twin of AliasIssues.
const AliasReleases = "releases"

// routeByKind maps a forge kind to its shields route family, the suffix `badges-<route>` uses.
var routeByKind = map[string]string{
	string(hostmatch.KindGitHub):  "github",
	string(hostmatch.KindGitLab):  "gitlab",
	string(hostmatch.KindForgejo): "gitea",
}

// releasesClaims files `releases` and `releases-<route>` for every repositories[releases=true] entry, at its link's priority.
func releasesClaims(pf *projectfile.Document, out map[string]any) []claim {
	var claims []claim
	for _, rr := range pfmodel.ReleasesRepositories(pf) {
		host, owner, repo := hostOwnerRepo(rr.URL)
		if host == "" {
			genlog.Debug("forge releases alias skipped", "url", rr.URL, "reason", "no host/owner/repo")
			continue
		}
		matched := false
		for i := range pf.Links {
			link := &pf.Links[i]
			if link.Type != projectfile.LinkSourceCode || !sameRepository(link.URL, host, owner, repo) {
				continue
			}
			remote, ok := out[firstLabel(host)].(map[string]any)
			if !ok || remote[KeyHost] != host || remote[KeyOwner] != owner || remote[KeyRepo] != repo {
				continue
			}
			matched = true
			priority := pfmodel.LinkPriority(*link)
			claims = append(claims, claim{alias: AliasReleases, remote: remote, priority: priority})
			kind, _ := remote[KeyKind].(string)
			route, known := routeByKind[kind]
			genlog.Debug("forge releases alias matched", "url", rr.URL, "route", route, "priority", priority)
			if known {
				claims = append(claims, claim{alias: AliasReleases + "-" + route, remote: remote, priority: priority})
			}
		}
		if !matched {
			genlog.Debug("forge releases alias skipped", "url", rr.URL, "reason", "no source-code link matches")
		}
	}
	return claims
}

// sameRepository reports whether raw names the host/owner/repo triple on any transport.
func sameRepository(raw, host, owner, repo string) bool {
	h, o, r := hostOwnerRepo(raw)
	return h == host && o == owner && r == repo
}

// applyAliases files each claim next to the slugs, so one remote is reachable
// both by identity (`remotes.codeberg`) and by capability (`remotes.badges`).
// The alias shares the slug's map rather than copying it — the two addresses
// are the same remote, and a copy could drift.
//
// Two rules keep the namespace honest, and both keep the SLUG:
//
//   - A claim naming an existing slug is refused. A slug is an identity and a
//     capability must never be able to steal it, or `remotes.gitea` would stop
//     meaning gitea.com for any project that tagged a mirror `gitea`.
//   - The highest-priority claimant of an alias wins, ties in document order,
//     so a fragment's claim holds however a project orders its includes. Later
//     claimants are traced, not dropped silently, because "my badge points at
//     the wrong mirror" is otherwise invisible.
func applyAliases(out map[string]any, slugs map[string]bool, claims []claim) {
	slices.SortStableFunc(claims, func(a, b claim) int { return pfmodel.ByPriorityDesc(a.priority, b.priority) })
	for _, c := range claims {
		if slugs[c.alias] {
			genlog.Warn("forge alias refused: it would shadow a forge slug",
				"alias", c.alias, "url", c.remote[KeyURL],
				"remedy", "rename the tag; a slug is an identity and wins")
			continue
		}
		if _, taken := out[c.alias]; taken {
			genlog.Debug("forge alias skipped", "alias", c.alias,
				"reason", "already claimed by a higher-priority link", "url", c.remote[KeyURL])
			continue
		}
		out[c.alias] = c.remote
		genlog.Debug("forge alias derived", "alias", c.alias, "url", c.remote[KeyURL])
	}
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

// hostOwnerRepo reads the identity triple out of a repository URL of ANY
// transport, so a clone endpoint can be matched to the web page that names the
// same repository. It accepts what coordinates refuses: `ssh://git@host/o/r.git`
// and the scp-style `git@host:o/r.git`, which carries no scheme at all and so
// parses as a bare path unless it is normalized first. Returns zero values when
// the triple is not complete.
func hostOwnerRepo(raw string) (host, owner, repo string) {
	if !strings.Contains(raw, "://") {
		// scp-style: drop the user, then promote the `:` separator to a path
		// separator so net/url sees an ordinary authority.
		if at := strings.Index(raw, "@"); at >= 0 {
			raw = raw[at+1:]
		}
		raw = "ssh://" + strings.Replace(raw, ":", "/", 1)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", "", ""
	}
	owner, repo = ownerRepo(parsed.Path)
	if owner == "" || repo == "" {
		return "", "", ""
	}
	return strings.ToLower(parsed.Hostname()), owner, repo
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
