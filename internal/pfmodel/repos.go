// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// RepositoryRoleOrigin mirrors projectfile.RepositoryRoleOrigin locally so
// this file has no symbol-level dependency on the core façade constant.
const RepositoryRoleOrigin = projectfile.RepositoryRoleOrigin

// PrimaryRepository returns the entry in doc.Repositories that is the
// canonical source location: the one with role == origin, or the sole
// entry (implicitly origin), or nil when no repositories are recorded.
// See spec §5.3a.
func PrimaryRepository(doc *projectfile.Document) *projectfile.Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	if len(repos) == 0 {
		return nil
	}
	if len(repos) == 1 {
		return &doc.Repositories[0]
	}
	for i := range repos {
		if repos[i].Role == RepositoryRoleOrigin {
			return &doc.Repositories[i]
		}
	}
	return nil
}

// IssuesRepository returns the repository entry that should be used for
// issue-tracker URL derivation. Resolution order (spec §4.3a, "issues" key):
//
//  1. The entry with Issues == true — the author explicitly chose this forge
//     as the canonical bug tracker.
//  2. PrimaryRepository — fallback when no entry carries Issues == true,
//     matching the pre-issues behaviour.
//
// Returns nil when no repositories are recorded.
func IssuesRepository(doc *projectfile.Document) *projectfile.Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	for i := range repos {
		if repos[i].Issues {
			return &doc.Repositories[i]
		}
	}
	return PrimaryRepository(doc)
}

// CitableRepositoryURL returns the forge landing-page URL for the repository
// the citation chain should cite — the schemes (http/ftp) acceptable in
// CITATION.cff repository-code, SPDX VCS expressions, and public package
// metadata. It picks ONE repository then resolves its URL to http:
//
//  1. Pick the repository (pickRepository):
//     a. the first entry marked cffr: true (§139 additional key on
//     repositories[] — the author explicitly chose it as the citable one),
//     b. else the primary repository (role == origin, or the sole entry),
//     c. else the first repository entry.
//  2. Resolve the picked entry's URL: return it when already http/ftp-style;
//     otherwise fall back to links[type=source-code] (§5.11), whose §5.11
//     selector returns the preferred — i.e. origin — page when the scanner
//     marks it.
//
// Repositories[].url may be ssh (the truthful clone endpoint); the http forge
// page lives in links[type=source-code]. Returns "" when nothing usable exists.
func CitableRepositoryURL(doc *projectfile.Document) string {
	if doc == nil {
		return ""
	}
	r := pickRepository(doc)
	if r == nil {
		return ""
	}
	if isPublicURL(r.URL) {
		return r.URL
	}
	// SSH-style clone URL: the http forge page is in links[type=source-code].
	if l := LinkByType(doc, LinkSourceCode); l != nil && isPublicURL(l.URL) {
		return l.URL
	}
	return ""
}

// pickRepository selects the single repository entry the citation chain
// should cite, in spec order: explicit cffr:true marker first, then the
// primary (origin/sole) repository, then the first entry. Returns nil when
// doc has no repositories. cffr is read from the §139 Extra bag so no new
// core field is required.
func pickRepository(doc *projectfile.Document) *projectfile.Repository {
	if doc == nil || len(doc.Repositories) == 0 {
		return nil
	}
	for i := range doc.Repositories {
		if boolFromExtra(doc.Repositories[i].Extra, "cffr") {
			return &doc.Repositories[i]
		}
	}
	if p := PrimaryRepository(doc); p != nil {
		return p
	}
	return &doc.Repositories[0]
}

// boolFromExtra reads key from the §139 Extra map as a bool, tolerating both
// the parsed (bool) and serialized (string) shapes the encoders carry. Returns
// false when the key is absent or not bool-shaped.
func boolFromExtra(extra map[string]any, key string) bool {
	v, ok := extra[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// EnsureCitableSourceLink returns a pointer to the links[type=source-code]
// entry the citation chain would emit, creating one when absent. Resolution
// order: preferred-or-sole-or-first existing source-code link, else append
// a new entry. Used by sync drivers that need to push a foreign manifest's
// public URL into pf without clobbering the ssh clone endpoint in
// repositories[].
func EnsureCitableSourceLink(doc *projectfile.Document) *projectfile.Link {
	if doc == nil {
		return nil
	}
	if l := LinkByType(doc, LinkSourceCode); l != nil {
		return l
	}
	doc.Links = append(doc.Links, projectfile.Link{Type: LinkSourceCode})
	return &doc.Links[len(doc.Links)-1]
}

func isPublicURL(u string) bool {
	return strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "http://") ||
		strings.HasPrefix(u, "ftp://") ||
		strings.HasPrefix(u, "sftp://")
}

// EnsurePrimaryRepository returns a pointer to the primary repository entry,
// creating one (role: origin) when none exist. Used by sync drivers that
// receive a single repository URL from a foreign manifest (package.json,
// pyproject.toml, composer.json, CITATION.cff) and need a stable slot to
// write into without losing existing mirror/archive entries.
func EnsurePrimaryRepository(doc *projectfile.Document) *projectfile.Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	if len(repos) == 0 {
		doc.Repositories = []projectfile.Repository{{Role: RepositoryRoleOrigin}}
		return &doc.Repositories[0]
	}
	for i := range repos {
		if repos[i].Role == RepositoryRoleOrigin {
			return &doc.Repositories[i]
		}
	}
	return &doc.Repositories[0]
}
