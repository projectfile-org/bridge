// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

const scannerGitRemotes = "git-remotes"

// remotesScanner extracts repositories and source-code links from git remotes.
// The "origin" remote is given role "origin"; all others are mirrors. For each
// remote on a known forge host, a links[type=source-code] entry is emitted
// with the forge landing page URL.
type remotesScanner struct{}

func (remotesScanner) Name() string { return scannerGitRemotes }

func (remotesScanner) Detect(root string) bool { return detectGit(root) }

func (remotesScanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	p := &source.Partial{}
	var hits []core.Hit

	var forgeKinds map[string]string
	if doc, _, err := projectfile.ReadWithOptions(root, projectfile.ReadOptions{Offline: true}); err == nil {
		if ext, err := pfmodel.GetForgeExtension(doc); err == nil && ext != nil {
			forgeKinds = ext.Kinds
		}
	}

	seenPage := map[string]bool{}
	for _, e := range remoteEntries(root) {
		name, remote := e.Name, e.URL
		if remote == "" {
			continue
		}
		if host, _ := parseForgeLocator(remote); userconfig.IsPrivateHost(host) {
			genlog.Info("git scanner: redacting private remote", "name", name, "host", host)
			continue
		}
		r := projectfile.Repository{URL: remote, Type: "git"}
		isOrigin := name == "origin" && primaryRepoIndex(p.Repositories) < 0
		if isOrigin {
			r.Role = projectfile.RepositoryRoleOrigin
			if branch := defaultBranch(root); branch != "" {
				r.Branch = branch
			}
			hits = append(hits, core.Hit{Source: scannerGitRemotes, Field: "repositories[role=origin]"})
		} else {
			r.Role = projectfile.RepositoryRoleMirror
			hits = append(hits, core.Hit{Source: scannerGitRemotes, Field: "repositories[mirror]:" + name})
		}
		p.Repositories = append(p.Repositories, r)
		if page := forgePageURL(remote, forgeKinds); page != "" && !seenPage[page] {
			seenPage[page] = true
			// Mark the origin's forge page Preferred so the §5.11 selector
			// (LinkByType) returns it over mirror pages — the citable repo
			// is the origin, not whichever remote appended first.
			link := projectfile.Link{Type: projectfile.LinkSourceCode, URL: page, Preferred: isOrigin}
			forge := hostmatch.ResolveLabel(page)
			if forge == "" {
				forge = hostFromURL(page)
			}
			if forge != "" {
				link.Label = &projectfile.LocalizedString{Bare: "Source Code on " + forge}
			}
			// Turn what the host table already knows into a declaration the
			// user can read and edit. Without it a shared fragment has to
			// name a hostname literally and impose one topology on every
			// project that includes it.
			// force=false is not the user's --force: this link is freshly
			// built and has no tags to overwrite. applyLink is where the
			// document's own list is weighed.
			if tags := hostmatch.Capabilities(page); pfmodel.SetLinkTags(&link, tags, false) {
				genlog.Info("git scanner: proposing capability tags", "url", page, "tags", tags)
				hits = append(hits, core.Hit{Source: scannerGitRemotes, Field: "links[type=source-code].tags:" + name})
			}
			p.Links = append(p.Links, link)
			hits = append(hits, core.Hit{Source: scannerGitRemotes, Field: "links[type=source-code]:" + name})
		}
	}

	return p, hits, nil
}
