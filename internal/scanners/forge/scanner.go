// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package forge materializes the link and repository templates a fleet fragment declares.
package forge

import (
	"net/url"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/rootflags"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

const (
	scannerForge    = "forge"
	keyLinks        = "links"
	keyRepositories = "repositories"
)

func init() { core.Register(Scanner{}) }

// Scanner expands org.projectfile.forge.{links,repositories} templates into concrete entries.
type Scanner struct{}

func (Scanner) Name() string { return scannerForge }

func (Scanner) Detect(_ string) bool { return true }

// Scan reads the merged document and proposes one concrete entry per template that resolves.
func (Scanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	doc, _, err := projectfile.ReadWithOptions(root, rootflags.ReadOpts())
	if err != nil {
		return nil, nil, err
	}
	p, hits := Materialize(doc)
	return p, hits, nil
}

// Materialize expands the forge templates of an already-read merged document.
func Materialize(doc *projectfile.Document) (*source.Partial, []core.Hit) {
	raw, _ := projectfile.LookupExtension(doc, pfmodel.ForgeExtensionNS)
	ns, _ := raw.(map[string]any)
	tmpl := projectfile.FromMap(map[string]any{keyLinks: ns[keyLinks], keyRepositories: ns[keyRepositories]})
	p := &source.Partial{}
	var hits []core.Hit
	for _, l := range tmpl.Links {
		if !expandLink(doc, &l) {
			continue
		}
		p.Links = append(p.Links, l)
		hits = append(hits, core.Hit{Source: scannerForge, Field: "links[type=" + l.Type + "]:" + l.URL})
	}
	for _, r := range tmpl.Repositories {
		u, ok := expand(doc, r.URL)
		if !ok {
			genlog.Warn("forge scanner: repository template unresolved — skipped", "role", r.Role, "url", r.URL)
			continue
		}
		r.URL = u
		p.Repositories = append(p.Repositories, r)
		hits = append(hits, core.Hit{Source: scannerForge, Field: "repositories[role=" + r.Role + "]:" + u})
	}
	genlog.Debug("forge scanner: templates materialized", "links", len(p.Links), "repositories", len(p.Repositories))
	return p, hits
}

// expandLink fills the URL and label in place; false when a reference stays unresolved.
func expandLink(doc *projectfile.Document, l *projectfile.Link) bool {
	u, ok := expand(doc, l.URL)
	if !ok {
		genlog.Warn("forge scanner: link template unresolved — skipped", "type", l.Type, "url", l.URL)
		return false
	}
	l.URL = u
	if l.Label == nil {
		l.Label = placeholderLabel(doc, l.Type, u)
		return true
	}
	if !expandLabel(doc, l.Label) {
		genlog.Warn("forge scanner: link label unresolved — skipped", "type", l.Type, "url", u)
		return false
	}
	return true
}

// expandLabel fills every language of a label in place; false when any stays unresolved.
func expandLabel(doc *projectfile.Document, label *projectfile.LocalizedString) bool {
	bare, ok := expand(doc, label.Bare)
	label.Bare = bare
	for lang, s := range label.Langs {
		v, resolved := expand(doc, s)
		label.Langs[lang] = v
		ok = ok && resolved
	}
	return ok
}

// placeholderLabel composes the same noun placeholder git-remotes writes, so the title promotion rewrites it.
func placeholderLabel(doc *projectfile.Document, linkType, page string) *projectfile.LocalizedString {
	nouns := map[string]string{projectfile.LinkSourceCode: pfmodel.NounSourceCode, projectfile.LinkBugs: pfmodel.NounIssues}
	noun, ok := nouns[linkType]
	if !ok {
		return nil
	}
	forge := hostmatch.ResolveLabel(page)
	if u, err := url.Parse(page); forge == "" && err == nil {
		forge = u.Host
	}
	return pfmodel.ComposeOnLabel(doc, pfmodel.NounLabel(doc, noun), forge)
}

// expand resolves s with the forge namespace as the relative scope, so `${path}` reads forge.path.
func expand(doc *projectfile.Document, s string) (string, bool) {
	return interp.ExpandIn(doc, s, pfmodel.ForgeExtensionNS)
}
