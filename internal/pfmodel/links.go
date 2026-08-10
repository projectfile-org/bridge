// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// LinkSourceCode matches the core constant projectfile.LinkSourceCode — the
// recommended link type for a forge landing page (spec §5.11). Re-declared
// here so pfmodel is the only package this file imports.
const LinkSourceCode = projectfile.LinkSourceCode

// keyPriority is the §139 additional-key name a link (or shield map) carries
// its advisory render priority under. Shared by LinkPriority and the shield
// parser so the literal lives in one place (goconst).
const keyPriority = "priority"

// keyTags is the §139 additional-key name a link carries its advisory
// capability list under — the same channel keyPriority rides, so a tag costs
// no schema change.
const keyTags = "tags"

// The two capability tags that are HOST facts, and so are the only ones a
// scanner may propose: `public` says anyone can read the repository there,
// `badges` says a badge service can crawl it. `ci` and `releases` are equally
// valid tags, but they state what a PROJECT decided to run where, which no
// hostname reveals. Named so the host table, the scanner and its tests spell
// them once.
const (
	TagPublic = "public"
	TagBadges = "badges"
)

// LinkTags returns the capabilities a link declares through the spec's §139
// additional-key channel (l.Extra). A tag says what the mirror AFFORDS —
// `public`, `badges`, `ci` — which is what lets a shared fragment address a
// forge by capability instead of by hostname, and so serve a fleet whose
// projects each sit on a different set of mirrors.
func LinkTags(l projectfile.Link) []string {
	return TagsFrom(l.Extra[keyTags])
}

// SetLinkTags writes a proposed capability list onto a link that declares
// none, and reports whether it wrote. It is the only writer of the tags key,
// which is why it lives beside LinkTags rather than in the scanner.
//
// The gap test is KEY PRESENCE, not parsed length. `tags: []` is a curated
// statement that this mirror affords nothing, and a scanner that re-proposed
// over it would overrule the user on every run — the same "existing values
// win" rule applyLink applies to label and preferred.
func SetLinkTags(l *projectfile.Link, tags []string) bool {
	if l == nil || len(tags) == 0 {
		return false
	}
	if _, declared := l.Extra[keyTags]; declared {
		return false
	}
	if l.Extra == nil {
		l.Extra = map[string]any{}
	}
	l.Extra[keyTags] = tags
	return true
}

// TagsFrom extracts a tag list from an untyped §139 value. Exported because the
// readme's goal filter reads the same shape off a CI node map: the tolerance
// rules (absent key, wrong type, mixed items, empty strings) must not diverge
// between two readers of one convention. Decoders disagree on the element type
// — YAML hands back []any, a typed path []string — so both are accepted;
// anything else yields nothing, which drops the capability rather than
// inventing one.
func TagsFrom(v any) []string {
	switch items := v.(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// LinkByType returns the canonical entry for a given link type from doc.Links,
// applying the §5.11 selection rule: the entry with Preferred=true if any,
// else the sole entry of that type, else the first entry of that type in
// document order. Returns nil when no entry of the type exists.
//
// Mutating the returned pointer mutates the document. To add a new entry,
// use SetLink (gap-fill) or AddLink (always append) instead.
func LinkByType(doc *projectfile.Document, linkType string) *projectfile.Link {
	if doc == nil || linkType == "" {
		return nil
	}
	var first, sole *projectfile.Link
	count := 0
	for i := range doc.Links {
		l := &doc.Links[i]
		if l.Type != linkType {
			continue
		}
		if l.Preferred {
			return l
		}
		if first == nil {
			first = l
		}
		sole = l
		count++
	}
	if count == 1 {
		return sole
	}
	return first
}

// LinkURL is the read-only sugar over LinkByType — returns the URL of the
// canonical entry, or "" when no entry of the type exists.
func LinkURL(doc *projectfile.Document, linkType string) string {
	if l := LinkByType(doc, linkType); l != nil {
		return l.URL
	}
	return ""
}

// LinkPriority reads the advisory priority a link carries via the spec's §139
// additional-key channel (l.Extra), the same channel the `related` tag rides
// on. Priority is OPTIONAL; an absent or non-numeric value returns
// PriorityDefault, so a stable sort keeps the link in declaration order. This
// is what lets priority order links[] across every document that consumes them
// (readme, SUPPORT, CONTRIBUTING) with no schema change: `priority` is just one
// more key core preserves untouched.
func LinkPriority(l projectfile.Link) int {
	raw, ok := l.Extra[keyPriority]
	if !ok {
		return PriorityDefault
	}
	switch n := raw.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return PriorityDefault
}

// LinksByType returns all entries of a given link type in document order.
// Use this when a consumer needs every entry (e.g., SUPPORT.md showing
// multiple paid-support or chat channels). Returns nil when no entries match.
func LinksByType(doc *projectfile.Document, linkType string) []projectfile.Link {
	if doc == nil || linkType == "" {
		return nil
	}
	var out []projectfile.Link
	for _, l := range doc.Links {
		if l.Type == linkType {
			out = append(out, l)
		}
	}
	return out
}

// SetLink writes url into the canonical links[type=linkType] entry: updates
// the entry LinkByType would return, or appends a new entry when none exists.
// force=true overwrites a non-empty URL; force=false only writes when the
// canonical entry is missing or empty (gap-fill semantics matching the sync
// FromPF/ToPF closures). Returns true when the document was mutated.
func SetLink(doc *projectfile.Document, linkType, url string, force bool) bool {
	if doc == nil || linkType == "" || url == "" {
		return false
	}
	if l := LinkByType(doc, linkType); l != nil {
		if l.URL == url {
			return false
		}
		if l.URL != "" && !force {
			return false
		}
		l.URL = url
		return true
	}
	doc.Links = append(doc.Links, projectfile.Link{Type: linkType, URL: url})
	return true
}

// AddLink appends a new entry to doc.Links unconditionally. Use this when
// emitting multiple entries of the same type (mirrors of source-code, several
// chat channels); use SetLink for the "one canonical URL per type" case.
func AddLink(doc *projectfile.Document, linkType, url string) {
	if doc == nil || linkType == "" || url == "" {
		return
	}
	doc.Links = append(doc.Links, projectfile.Link{Type: linkType, URL: url})
}
