// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package forges infers issue-tracker URLs from repository and source-code URLs.
// Supported hosts: github.com, gitlab.com (and self-hosted with /-/issues),
// codeberg.org, gitea.com, gitea.io self-hosted, sr.ht (sourcehut).
//
// The host-suffix matching vocabulary lives in internal/forge/hostmatch so
// the new `pf-cli forge push` command and this deriver see the same rules.
package forges

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Change is the local shape the engine consumes. Kept independent of
// derive.Change so the engine package can evolve without rippling into here.
type Change struct {
	FieldPath string
	NewValue  string
	Source    string
	Label     *projectfile.LocalizedString
}

// Derive returns one issue-tracker URL per source-code mirror the engine can
// resolve to a known forge. When a repository entry carries issues=true,
// only that entry is used (spec §4.3a). Otherwise the sources walked, in order:
//
//  1. Every links[type=source-code] entry — projects with multiple forge
//     mirrors (codeberg + github + a self-hosted gitlab) get a bugs link
//     each, all preserved in document order.
//  2. Primary repository URL — fallback when no source-code links exist
//     (http-style only; ssh clone URLs can't be hostmatch-resolved).
//
// Output is deduplicated by tracker URL so two source-code entries that map
// to the same tracker (uncommon, but possible with redirect aliases) don't
// produce duplicate writes. Returns nil when nothing in the document points
// at a known forge — the caller leaves existing values untouched.
//
// FieldPath embeds the URL ("links[type=bugs,url=https://...]") so the
// engine's ownership tracking treats each mirror's bugs link as an
// independent slot. Without the URL discriminator, multiple Changes would
// collide on "links[type=bugs]" and only the last write would survive.
func Derive(pf *projectfile.Document) []Change {
	sources := collectSourceURLs(pf)
	if len(sources) == 0 {
		return nil
	}
	var out []Change
	seen := map[string]bool{}
	for _, s := range sources {
		rule, _ := hostmatch.Resolve(s)
		if rule == nil || rule.Tracker == nil {
			continue
		}
		value := rule.Tracker(strings.TrimSuffix(s, "/"))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, Change{
			FieldPath: "links[type=bugs,url=" + value + "]",
			NewValue:  value,
			Source:    rule.Source,
			// Localized "Issues on {forge}": Bare for a single-language
			// project, a Langs map when i18n.languages is declared so each
			// locale gets its own noun + connector.
			Label: pfmodel.ComposeOnLabel(pf, pfmodel.NounLabel(pf, pfmodel.NounIssues), rule.Label),
		})
	}
	return out
}

// collectSourceURLs returns every URL the deriver should try to resolve to
// a forge. When a repository carries issues=true (spec §4.3a), only that
// entry is returned. Otherwise, all source-code links are collected in
// document order so the first-listed mirror's bugs link sits before the
// second's. Primary-repo fallback only kicks in when no source-code links
// exist at all — otherwise the source-code entries are authoritative.
func collectSourceURLs(pf *projectfile.Document) []string {
	if pf == nil {
		return nil
	}

	// When a repo is explicitly marked issues=true, use only that repo.
	if ir := pfmodel.IssuesRepository(pf); ir != nil && ir.Issues {
		httpURL := repoHTTPURL(ir.URL, pf)
		if httpURL != "" {
			return []string{httpURL}
		}
		// SSH-style clone URL: try to find the matching source-code link.
		return nil
	}

	// No explicit issues marker: derive from all source-code links.
	var urls []string
	for i := range pf.Links {
		l := &pf.Links[i]
		if l.Type == projectfile.LinkSourceCode && l.URL != "" {
			urls = append(urls, l.URL)
		}
	}
	if len(urls) > 0 {
		return urls
	}
	if primary := pfmodel.PrimaryRepository(pf); primary != nil && primary.URL != "" {
		return []string{primary.URL}
	}
	return nil
}

// repoHTTPURL resolves a repository URL to an http(s) URL suitable for
// hostmatch. If the URL is already http(s), it is returned as-is. Otherwise
// the matching links[type=source-code] entry is searched by host+path
// heuristics. Returns "" when no http URL can be found.
func repoHTTPURL(repoURL string, pf *projectfile.Document) string {
	if strings.HasPrefix(repoURL, "http://") || strings.HasPrefix(repoURL, "https://") {
		return repoURL
	}
	// SSH-style: try to find the matching source-code link.
	// git@github.com:acme/proj.git → look for github.com/acme/proj
	for i := range pf.Links {
		l := &pf.Links[i]
		if l.Type == projectfile.LinkSourceCode && l.URL != "" {
			return l.URL
		}
	}
	return ""
}
