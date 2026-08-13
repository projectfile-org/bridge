// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package derive

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/fieldpath"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// readField pulls the current value of a known field path so the engine can
// distinguish hand-edited values from previously-derived ones. Returns ""
// for unknown paths or absent fields; the apply loop treats "" as "free to
// write" so callers MUST register every path they intend to derive.
//
// The set of derivable fields is still small (every entry is a links[]
// selector), so the dispatch table is a single if-branch on the leading
// segment. Path parsing reuses internal/fieldpath so the derive layer and
// the CRUD verbs share one grammar instead of forking syntax.
func readField(pf *projectfile.Document, path string) string {
	if pf == nil {
		return ""
	}
	p, err := fieldpath.Parse(path)
	if err != nil {
		return ""
	}
	if !isLinkSelector(p) {
		return ""
	}
	typeVal, _, urlVal := extractLinkSelector(p)
	if typeVal == "" {
		return ""
	}
	if urlVal != "" {
		// Per-URL slot — one slot per (type,url) tuple, so distinct mirrors
		// of the same type coexist instead of clobbering each other.
		if linkExists(pf, typeVal, urlVal) {
			return urlVal
		}
		return ""
	}
	// Bare links[type=X] — canonical entry of the type (LinkURL applies the
	// preferred/sole/first rule from spec §5.11).
	return pfmodel.LinkURL(pf, typeVal)
}

// writeField applies a derived value back into pf. Mirrors the dispatch in
// readField; together they form the read-modify-write contract the engine
// operates against. When label is non-nil, it is written into the link's label
// field via pfmodel.SetLinkLabel (gap-fill: an existing label wins).
func writeField(pf *projectfile.Document, path, value string, label *projectfile.LocalizedString) error {
	if pf == nil {
		return fmt.Errorf("derive: cannot write to nil document")
	}
	p, err := fieldpath.Parse(path)
	if err != nil {
		return fmt.Errorf("derive: parse path %q: %w", path, err)
	}
	if !isLinkSelector(p) {
		return fmt.Errorf("derive: unknown field path %q", path)
	}
	typeVal, _, urlVal := extractLinkSelector(p)
	if typeVal == "" {
		return fmt.Errorf("derive: malformed link selector %q", path)
	}
	if urlVal != "" {
		// Per-URL slot: append if missing, no-op if already present.
		// Multiple mirror entries of the same type coexist this way;
		// replacing the canonical slot would clobber the others.
		if !linkExists(pf, typeVal, value) {
			pfmodel.AddLink(pf, typeVal, value)
		}
		markLinkDerived(pf, typeVal, value)
		if label != nil {
			pfmodel.SetLinkLabel(pf, typeVal, value, label, false)
		}
		return nil
	}
	// Legacy bare-type form: replace-or-append against the canonical entry.
	pfmodel.SetLink(pf, typeVal, value, true)
	if l := pfmodel.LinkByType(pf, typeVal); l != nil {
		l.Derived = true
	}
	if label != nil {
		pfmodel.SetLinkLabel(pf, typeVal, value, label, false)
	}
	return nil
}

// isLinkSelector returns true when p addresses a links[type=...] entry —
// the only path shape the derive engine knows how to read/write. Cheap
// pre-check that lets the caller short-circuit before extracting predicates.
func isLinkSelector(p fieldpath.Path) bool {
	if len(p.Segments) < 2 {
		return false
	}
	if p.Segments[0].Kind != fieldpath.SegKey || p.Segments[0].Key != "links" {
		return false
	}
	return p.Segments[1].Kind == fieldpath.SegSelector
}

// extractLinkSelector pulls the (type, name, url) predicates out of a
// links[...] selector. Anything not "type" / "name" / "url" is silently
// ignored — extra predicates would mean the path is meant for a future
// extension this dispatch hasn't learned about yet.
func extractLinkSelector(p fieldpath.Path) (typeVal, nameVal, urlVal string) {
	for _, pr := range p.Segments[1].Preds {
		switch strings.TrimSpace(pr.Key) {
		case "type":
			typeVal = pr.Value
		case "name":
			nameVal = pr.Value
		case "url":
			urlVal = pr.Value
		}
	}
	return typeVal, nameVal, urlVal
}

// linkExists is the membership check the per-URL read/write paths share.
// Exact (type, url) match against doc.Links — comparison is case-sensitive
// because URL paths are case-sensitive on every forge we support.
func linkExists(pf *projectfile.Document, linkType, url string) bool {
	for i := range pf.Links {
		l := &pf.Links[i]
		if l.Type == linkType && l.URL == url {
			return true
		}
	}
	return false
}

// markLinkDerived sets Derived=true on the link matching (linkType, url).
// Called after writeField creates or confirms a link entry so the serialiser
// emits derived:true on that link rather than in [org.projectfile.cli].
func markLinkDerived(pf *projectfile.Document, linkType, url string) {
	for i := range pf.Links {
		l := &pf.Links[i]
		if l.Type == linkType && l.URL == url {
			l.Derived = true
			return
		}
	}
}
