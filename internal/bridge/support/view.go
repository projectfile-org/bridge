// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package support

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

type supportView struct {
	Marker       string
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
// no label set.
func resolveLabeledLinks(pf *projectfile.Document, linkType, defaultLabel, lang string) []labeledLink {
	links := pfmodel.LinksByType(pf, linkType)
	if len(links) == 0 {
		return nil
	}
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
