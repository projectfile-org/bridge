// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"os"
	"strings"

	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
)

// Canonical hostnames per forge kind. Used only as a hint to log/error
// messages — actual token lookup keys off the *real* repo host so a
// codeberg.org token doesn't accidentally satisfy a self-hosted forgejo
// instance's request.
const (
	CanonicalGitHub  = "github.com"
	CanonicalGitLab  = "gitlab.com"
	CanonicalForgejo = "codeberg.org"
)

// ResolveToken returns the token to use for kind/host, plus the env var name
// that supplied it (handy for log lines). Lookup order:
//
//  1. PF_FORGE_TOKEN_<HOST_UPPER_UNDERSCORE> — per-host override; this is
//     the only way to talk to two distinct self-hosted GitLab instances
//     from one shell.
//  2. <KIND>_TOKEN — canonical env var name (GITHUB_TOKEN, GITLAB_TOKEN,
//     FORGEJO_TOKEN). Standard names users already export for `gh` / `glab`.
//  3. for kind == "forgejo" only: GITEA_TOKEN — Forgejo and Gitea share
//     the same token surface; respect either name.
//
// Returns ("", "") when no token is found — the caller skips the host with
// a warning that names the env var the user should set.
func ResolveToken(kind, host string) (token, source string) {
	if v, name := lookupHostOverride(host); v != "" {
		return v, name
	}
	if name := canonicalEnvName(kind); name != "" {
		if v := os.Getenv(name); v != "" {
			return v, name
		}
	}
	if kind == string(hostmatch.KindForgejo) {
		if v := os.Getenv("GITEA_TOKEN"); v != "" {
			return v, "GITEA_TOKEN"
		}
	}
	return "", ""
}

// HostOverrideEnvVar returns the per-host override env var name for host.
// Exported so the `forge list` command can name the variable the user
// should set when the token is missing.
func HostOverrideEnvVar(host string) string {
	return "PF_FORGE_TOKEN_" + hostToEnvSuffix(host)
}

// CanonicalEnvVar returns the kind-canonical env var name ("" for unknown
// kinds). Exported for the same reason as HostOverrideEnvVar — the list
// command surfaces both names so the user knows their options.
func CanonicalEnvVar(kind string) string {
	return canonicalEnvName(kind)
}

func lookupHostOverride(host string) (token, name string) {
	if host == "" {
		return "", ""
	}
	name = HostOverrideEnvVar(host)
	return os.Getenv(name), name
}

// canonicalEnvName maps a forge kind to its conventional token env var.
// Returns "" for unknown kinds so callers can short-circuit cleanly.
func canonicalEnvName(kind string) string {
	switch hostmatch.Kind(kind) {
	case hostmatch.KindGitHub:
		return "GITHUB_TOKEN"
	case hostmatch.KindGitLab:
		return "GITLAB_TOKEN"
	case hostmatch.KindForgejo:
		return "FORGEJO_TOKEN"
	}
	return ""
}

// hostToEnvSuffix normalises a hostname into the upper-snake form an env var
// allows: dots and hyphens become underscores, letters uppercase. Matches
// the `_` separator convention used elsewhere (PATH_INFO, HTTP_ACCEPT, ...).
func hostToEnvSuffix(host string) string {
	host = strings.ToUpper(host)
	host = strings.ReplaceAll(host, ".", "_")
	host = strings.ReplaceAll(host, "-", "_")
	return host
}
