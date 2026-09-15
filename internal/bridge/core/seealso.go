// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"slices"
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// SeeAlsoLink is one entry in a "See also" section: a project-authored
// links[] entry that opted in by carrying this document's own tag (spec
// §139 additional keys), e.g. `tags: [contributing]` surfaces the link in
// CONTRIBUTING.md's See also section.
type SeeAlsoLink struct {
	Label string
	URL   string
}

// SeeAlsoFor resolves the See also links for one rendered document: every
// links[] entry tagged with `tag` (the bridge's own Name(), so a link
// author tags an entry with the exact document it should appear in),
// ordered by priority (higher first) with document order as the stable
// tiebreak. Label falls back to the link's type, then its URL, when the
// project left the entry unlabeled. Returns nil when no link carries the
// tag — See also is entirely project-authored, never invented by the
// bridge.
func SeeAlsoFor(pf *projectfile.Document, tag, lang string) []SeeAlsoLink {
	links := pfmodel.LinksByTag(pf, tag)
	if len(links) == 0 {
		if len(pf.Links) > 0 {
			genlog.DebugRow("see_also", tag, "no link carries this tag (section dropped)",
				"links[] declared="+strconv.Itoa(len(pf.Links)))
		}
		return nil
	}
	slices.SortStableFunc(links, func(a, b projectfile.Link) int {
		return pfmodel.ByPriorityDesc(pfmodel.LinkPriority(a), pfmodel.LinkPriority(b))
	})
	out := make([]SeeAlsoLink, 0, len(links))
	for _, l := range links {
		label := projectfile.ExtractLocalizedStringForLang(l.Label, lang)
		if label == "" {
			label = l.Type
		}
		if label == "" {
			label = l.URL
		}
		out = append(out, SeeAlsoLink{Label: label, URL: l.URL})
	}
	return out
}

// RenderSeeAlso renders the "## See also" section for a generated document:
// one bullet per project-authored link. Returns nil for an empty slice — a
// document with nothing to link to grows no orphan heading.
func RenderSeeAlso(links []SeeAlsoLink) []byte {
	if len(links) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("## See also\n\n")
	for _, l := range links {
		sb.WriteString("- [" + l.Label + "](" + l.URL + ")\n")
	}
	return []byte(sb.String())
}
