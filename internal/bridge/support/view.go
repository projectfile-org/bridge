// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package support

import (
	"slices"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

type supportView struct {
	ProjectName  string
	BeforeLinks  []beforeLink
	Rows         []tableRow
	ResponseTime string
	StatusPage   string
	EOL          []eolView
}

type beforeLink struct {
	Label string
	URL   string
}

// Row kinds for the "Where to Ask" table. The label for each is prose and
// therefore lives in the template (one per language), not here — Go decides
// which rows exist and where they point, the template says what they mean.
const (
	kindUsageQuestion = "usage-question"
	kindBug           = "bug"
	kindChat          = "chat"
	kindSecurity      = "security"
	kindContributing  = "contributing"
	kindPaidSupport   = "paid-support"
)

type tableRow struct {
	// Kind is the stable row identity the template maps to a localized
	// question ("Ask a usage question", "Hacer una pregunta de uso", …).
	Kind string
	// GoTo is the rendered markdown link — already localized where it points
	// at a sibling health file.
	GoTo string
}

type eolView struct {
	Version string
	Date    string
}

// labeledLink is a resolved (label, url) pair from a links[] entry.
type labeledLink struct {
	Label string
	URL   string
}

// resolveLabeledLinks collects all links of a given type with their labels,
// resolved in the active render language so a links[] entry carrying a
// localized label surfaces it. Falls back to defaultLabel when the link has
// no label set. A non-empty excludeTag skips links carrying that tag — used so
// an issues tracker flagged as a support resource appears in "Before You Ask"
// only, never again in "Where to Ask". Links are ordered by priority (higher
// first) with declaration order as the stable tiebreak.
func resolveLabeledLinks(pf *projectfile.Document, linkType, excludeTag, defaultLabel, lang string) []labeledLink {
	var links []projectfile.Link
	for _, l := range pfmodel.LinksByType(pf, linkType) {
		if excludeTag != "" && pfmodel.LinkHasTag(l, excludeTag) {
			continue
		}
		links = append(links, l)
	}
	if len(links) == 0 {
		return nil
	}
	slices.SortStableFunc(links, func(a, b projectfile.Link) int {
		return pfmodel.ByPriorityDesc(pfmodel.LinkPriority(a), pfmodel.LinkPriority(b))
	})
	out := make([]labeledLink, 0, len(links))
	for _, l := range links {
		label := projectfile.ExtractLocalizedStringForLang(l.Label, lang)
		if label == "" {
			label = defaultLabel
		}
		out = append(out, labeledLink{Label: label, URL: l.URL})
	}
	return out
}

// resolveSupportTaggedLinks collects the "Before You Ask" list: every link
// tagged `support`, with its label resolved in the active render language.
// Ordering and label fallback mirror resolveLabeledLinks; the default label is
// per-type so an unlabelled issues tracker still reads "Existing issues".
func resolveSupportTaggedLinks(pf *projectfile.Document, lang string) []labeledLink {
	links := pfmodel.LinksByTag(pf, pfmodel.TagSupport)
	if len(links) == 0 {
		return nil
	}
	slices.SortStableFunc(links, func(a, b projectfile.Link) int {
		return pfmodel.ByPriorityDesc(pfmodel.LinkPriority(a), pfmodel.LinkPriority(b))
	})
	out := make([]labeledLink, 0, len(links))
	for _, l := range links {
		if l.URL == "" {
			continue
		}
		label := projectfile.ExtractLocalizedStringForLang(l.Label, lang)
		if label == "" {
			label = beforeDefaultLabel(l.Type)
		}
		out = append(out, labeledLink{Label: label, URL: l.URL})
	}
	return out
}

// beforeDefaultLabel is the fallback label for a support-tagged link whose own
// label is unset, keyed by type; an unknown type reads as "Support".
func beforeDefaultLabel(linkType string) string {
	switch linkType {
	case projectfile.LinkDocumentation:
		return "Documentation"
	case projectfile.LinkWiki:
		return "Wiki"
	case projectfile.LinkFAQ:
		return "FAQ"
	case projectfile.LinkBugs:
		return "Existing issues"
	case projectfile.LinkForum:
		return "Past discussions"
	}
	return "Support"
}
