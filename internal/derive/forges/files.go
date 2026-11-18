// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forges

import (
	"net/url"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// AliasPublic is the alias of a source-code link tagged public, the mirror a stranger can read.
const AliasPublic = "public"

// FileURL returns the builder of a repository path’s page on pf’s public forge (the `public` remote, else the preferred one), at the origin’s declared branch or the forge’s default; it yields "" without a known forge.
func FileURL(pf *projectfile.Document, kinds map[string]string) func(rel string) string {
	remotes := Remotes(pf, kinds)
	remote, _ := remotes[AliasPublic].(map[string]any)
	if remote == nil {
		remote, _ = remotes[AliasPreferred].(map[string]any)
	}
	base, _ := remote[KeyURL].(string)
	kind, _ := remote[KeyKind].(string)
	branch := ""
	if r := pfmodel.PrimaryRepository(pf); r != nil {
		branch = r.Branch
	}
	genlog.Debug("forge file pages", "url", base, "kind", kind, "branch", branch)
	return func(rel string) string {
		escaped := (&url.URL{Path: rel}).EscapedPath()
		switch hostmatch.Kind(kind) {
		case hostmatch.KindGitHub:
			return base + "/blob/" + orHead(branch) + "/" + escaped
		case hostmatch.KindGitLab:
			return base + "/-/blob/" + orHead(branch) + "/" + escaped
		case hostmatch.KindForgejo:
			if branch == "" {
				return base + "/src/" + escaped
			}
			return base + "/src/branch/" + branch + "/" + escaped
		default:
			return ""
		}
	}
}

// orHead names the branch, else HEAD, which a forge resolves to its default branch.
func orHead(branch string) string {
	if branch == "" {
		return "HEAD"
	}
	return branch
}
