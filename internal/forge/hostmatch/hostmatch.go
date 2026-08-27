// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package hostmatch is the single source of truth that maps a repository
// host to a forge "kind". Two callers consume it today:
//
//   - internal/derive/forges — infers links[type=bugs] from a repo URL.
//   - internal/forge — pushes description/homepage/topics to the repo's forge.
//
// The rule table moved here from internal/derive/forges/forges.go so both
// callers see exactly the same host vocabulary. Adding a forge means adding
// one entry to Rules below.
package hostmatch

import (
	"net/url"
	"slices"
	"strings"

	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// The capability sets of a public forge whose API a badge service reaches:
// readable by anyone, countable by api.reuse.software, and countable by ONE
// shields.io route family. Each set names its own route, because a project
// with a Codeberg and a GitHub mirror affords both and a fragment has to be
// able to address them apart.
var (
	crawlableGitHub = []string{pfmodel.TagPublic, pfmodel.TagBadges, pfmodel.TagBadgesGitHub}
	crawlableGitLab = []string{pfmodel.TagPublic, pfmodel.TagBadges, pfmodel.TagBadgesGitLab}
	crawlableGitea  = []string{pfmodel.TagPublic, pfmodel.TagBadges, pfmodel.TagBadgesGitea}
)

// Kind enumerates the forge families pf-cli knows how to talk to. Multiple
// rules can resolve to the same Kind — both "github.com" and a hypothetical
// "github.example.com" GHES instance map to KindGitHub.
type Kind string

const (
	KindUnknown   Kind = ""
	KindGitHub    Kind = "github"
	KindGitLab    Kind = "gitlab"
	KindForgejo   Kind = "forgejo"
	KindSourcehut Kind = "sourcehut"
)

// Rule maps a host pattern to a forge kind plus a tracker-path template.
// Patterns are matched suffix-or-equal, so self-hosted instances inherit
// the rule (gitlab.example.com falls into the "gitlab." rule).
type Rule struct {
	// Host is the suffix-match pattern. Bare hostnames ("github.com") match
	// exactly or as a subdomain; patterns ending in "." ("gitlab.") match
	// any host that starts with the prefix.
	Host string
	// Kind names the forge family this rule resolves to. The push command
	// keys its driver registry on this value.
	Kind Kind
	// Label is the human-readable display name for this forge (e.g. "GitHub",
	// "GitLab", "Codeberg"). Used to populate link labels during scan/derive.
	Label string
	// Source is the decision-trace identifier emitted by the bugs-URL
	// deriver. Kept here so callers don't have to re-derive it from Kind.
	Source string
	// Capabilities lists what a mirror on this host affords, in the
	// vocabulary of links[].tags. Only host FACTS belong here, and a rule
	// leaves it empty when the hostname does not settle the question.
	Capabilities []string
	// Tracker maps a repo URL to its issue-tracker URL. Used only by the
	// derive layer; the push command ignores it.
	Tracker func(repoURL string) string
}

// Host string constants for the rule table. Exported so the (in-package)
// tests can reference the same literal and goconst doesn't flag the
// expected sharing. Adding a forge means extending Rules; new hosts get
// their own constant here.
const (
	HostGitHub        = "github.com"
	HostGitLab        = "gitlab.com"
	HostCodeberg      = "codeberg.org"
	HostGitea         = "gitea.com"
	HostSourcehut     = "git.sr.ht"
	HostGitLabPrefix  = "gitlab."
	HostGiteaPrefix   = "gitea."
	HostForgejoPrefix = "forgejo."
)

// Rules is the ordered match table. Order matters when a host could match
// multiple rules: list specific exact-host rules before the self-hosted
// prefix rules so "github.com" doesn't accidentally fall through.
var Rules = []Rule{
	{Host: HostGitHub, Kind: KindGitHub, Label: "GitHub", Source: "forge:github", Capabilities: crawlableGitHub, Tracker: appendPath("/issues")},
	{Host: HostGitLab, Kind: KindGitLab, Label: "GitLab", Source: "forge:gitlab", Capabilities: crawlableGitLab, Tracker: appendPath("/-/issues")},
	{Host: HostCodeberg, Kind: KindForgejo, Label: "Codeberg", Source: "forge:codeberg", Capabilities: crawlableGitea, Tracker: appendPath("/issues")},
	{Host: HostGitea, Kind: KindForgejo, Label: "Gitea", Source: "forge:gitea", Capabilities: crawlableGitea, Tracker: appendPath("/issues")},
	// Public, but no badge endpoint addresses it: shields.io carries no
	// sourcehut route, so `badges` here would render a broken image.
	{Host: HostSourcehut, Kind: KindSourcehut, Label: "sourcehut", Source: "forge:sourcehut", Capabilities: []string{pfmodel.TagPublic}, Tracker: sourcehutTracker},
	// Self-hosted GitLab — any host whose name starts with "gitlab.".
	// No capabilities: gitlab.acme.internal matches this rule too, and a
	// hostname never says whether an instance faces the public.
	{Host: HostGitLabPrefix, Kind: KindGitLab, Label: "GitLab", Source: "forge:gitlab-self-hosted", Tracker: appendPath("/-/issues")},
	// Self-hosted Gitea / Forgejo. Best-effort: catches the common
	// "gitea." / "forgejo." subdomain convention. A bare-hostname instance
	// (e.g. code.foo.com running Gitea) needs the user to declare the kind
	// explicitly — see the forge push command's --kind override.
	{Host: HostGiteaPrefix, Kind: KindForgejo, Label: "Gitea", Source: "forge:gitea-self-hosted", Tracker: appendPath("/issues")},
	{Host: HostForgejoPrefix, Kind: KindForgejo, Label: "Forgejo", Source: "forge:forgejo", Tracker: appendPath("/issues")},
}

// Resolve returns the Rule matching the repo URL plus the lowercased host
// (handy for token lookup). When no rule matches, returns (nil, "").
// Accepts ssh:// and scp-style git@host:owner/repo.git URLs.
func Resolve(repoURL string) (*Rule, string) {
	u, err := url.Parse(ScpToSSH(repoURL))
	if err != nil || u.Host == "" {
		return nil, ""
	}
	host := strings.ToLower(u.Host)
	for i := range Rules {
		if MatchesHost(host, Rules[i].Host) {
			return &Rules[i], host
		}
	}
	return nil, host
}

// ScpToSSH normalizes scp-style URLs (git@github.com:owner/repo.git) to
// ssh:// form so url.Parse extracts the host correctly. URLs already
// carrying a scheme are returned unchanged. Exported for use by forge
// drivers that parse repo URLs.
func ScpToSSH(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	// scp-style: drop the user (if any), then promote the `:` separator to
	// a path separator so net/url sees an ordinary authority.
	if at := strings.Index(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}
	return "ssh://" + strings.Replace(raw, ":", "/", 1)
}

// ResolveKind is the cheap "what driver should handle this" lookup. Returns
// KindUnknown when no rule matches — the caller logs a skip and moves on.
func ResolveKind(repoURL string) Kind {
	r, _ := Resolve(repoURL)
	if r == nil {
		return KindUnknown
	}
	return r.Kind
}

// ResolveKindWithKinds is like ResolveKind but also checks a user-supplied
// kinds map (from [org.projectfile.forge].kinds). The kinds map wins over
// the static Rules table on conflict, matching the spec semantics: "wins
// over the guess on conflict, so it doubles as a manual override for
// misclassified hosts."
func ResolveKindWithKinds(repoURL string, kinds map[string]string) Kind {
	// Check user kinds first — these are explicit overrides.
	_, host := Resolve(repoURL)
	if host != "" && kinds != nil {
		if kind, ok := kinds[host]; ok {
			return Kind(kind)
		}
	}
	return ResolveKind(repoURL)
}

// Capabilities returns the capability tags the host of repoURL affords, ready
// for links[].tags. It is empty for an unknown host and for every self-hosted
// prefix rule, which is the whole point: a guess that a private instance is
// `public` puts a permanently broken badge in a README, while a missing tag
// only drops one badge and stays visible in the projectfile for the user to
// correct. Returns a copy — the rule table is package state.
func Capabilities(repoURL string) []string {
	r, _ := Resolve(repoURL)
	if r == nil {
		return nil
	}
	return slices.Clone(r.Capabilities)
}

// ResolveLabel returns the human-readable display name for the forge hosting
// repoURL (e.g. "GitHub", "Codeberg", "sourcehut"). Returns "" when no rule
// matches — callers should fall back to a generic label.
func ResolveLabel(repoURL string) string {
	r, _ := Resolve(repoURL)
	if r == nil {
		return ""
	}
	return r.Label
}

// MatchesHost returns true when host equals pattern, or ends with "."+pattern
// (subdomain match), or starts with pattern when pattern itself ends with "."
// (self-hosted match like "gitlab." → "gitlab.example.com").
func MatchesHost(host, pattern string) bool {
	if host == pattern {
		return true
	}
	if strings.HasSuffix(host, "."+pattern) {
		return true
	}
	if strings.HasSuffix(pattern, ".") && strings.HasPrefix(host, pattern) {
		return true
	}
	return false
}

// appendPath builds a tracker function that suffixes the repo URL with a
// fixed path segment. GitHub /issues, GitLab /-/issues, Codeberg /issues
// all follow this shape — strip any trailing slash to avoid "//issues".
func appendPath(suffix string) func(string) string {
	return func(repoURL string) string {
		return strings.TrimSuffix(repoURL, "/") + suffix
	}
}

// sourcehutTracker handles git.sr.ht's tracker subdomain layout:
//
//	https://git.sr.ht/~user/repo  →  https://todo.sr.ht/~user/repo
func sourcehutTracker(repoURL string) string {
	return strings.Replace(repoURL, "git.sr.ht", "todo.sr.ht", 1)
}
