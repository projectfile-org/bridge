// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package support

import (
	"os"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameSupport = core.FileSupport

// Bridge renders SUPPORT.md from links[] and [org.projectfile.support].
type Bridge struct{}

func (Bridge) Name() string             { return "support" }
func (Bridge) Filename() string         { return filenameSupport }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameSupport, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{ScaffoldOnce: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameSupport, filenameSupport))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameSupport, filenameSupport)
}

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetSupportExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		ext = &pfmodel.SupportExtension{}
	}
	core.ApplyUserSupportFallback(ext)

	// The trace reports the canonical render; each language rebuilds these
	// with its own link labels inside the per-language view below.
	beforeLinks := buildBeforeLinks(pf, "")
	rows := buildTableRows(pf, "")

	// The response-time fallback is a full sentence of prose, so the template
	// owns it: passing the field through empty lets each localized template
	// word its own default rather than leaking English into SUPPORT.es.md.
	responseTime := ext.ResponseTime
	responseTimeSrc := "[org.projectfile.support].response-time"
	if responseTime == "" {
		responseTimeSrc = "default (per-language, from template)"
	}

	statusPageURL := pfmodel.LinkURL(pf, "status-page")

	var eolViews []eolView
	for _, entry := range ext.EOL {
		if entry.Version != "" && entry.Date != "" {
			eolViews = append(eolViews, eolView{
				Version: entry.Version,
				Date:    entry.Date,
			})
		}
	}

	emitDecisionTrace(pf, ext, beforeLinks, rows, statusPageURL, responseTime, responseTimeSrc)

	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: filenameSupport,
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			return supportView{
				Marker:       core.MarkerHTML,
				ProjectName:  pfmodel.DisplayNameForLang(pf, lang),
				BeforeLinks:  buildBeforeLinks(pf, lang),
				Rows:         buildTableRows(pf, lang),
				ResponseTime: responseTime,
				StatusPage:   statusPageURL,
				EOL:          eolViews,
			}
		},
	}, opts)
}

// buildBeforeLinks collects the "Before You Ask" bullet list from links[].
// Each link type can produce multiple entries when multiple links of that type
// exist. Only non-empty URLs are included.
func buildBeforeLinks(pf *projectfile.Document, lang string) []beforeLink {
	type mapping struct {
		linkType     string
		defaultLabel string
	}
	mappings := []mapping{
		{projectfile.LinkDocumentation, "Documentation"},
		{projectfile.LinkWiki, "Wiki"},
		{projectfile.LinkFAQ, "FAQ"},
		{projectfile.LinkBugs, "Existing issues"},
		{projectfile.LinkForum, "Past discussions"},
	}

	var out []beforeLink
	for _, m := range mappings {
		for _, ll := range resolveLabeledLinks(pf, m.linkType, m.defaultLabel, lang) {
			out = append(out, beforeLink(ll))
		}
	}
	return out
}

// buildTableRows produces the "Where to Ask" table rows. Community health
// files (SECURITY.md, CONTRIBUTING.md) are always referenced as local
// relative links since they live alongside SUPPORT.md — resolved to the
// same-language variant when one will be rendered. Network links come from
// links[] and can produce multiple rows when multiple entries of the same
// type exist (e.g., two paid-support providers, two chat channels).
//
// Each row carries a stable Kind rather than an English question: the "what
// do you want to do" column is prose, so the template owns its wording and a
// translated template renders the same rows in its own language.
func buildTableRows(pf *projectfile.Document, lang string) []tableRow {
	var rows []tableRow

	for _, ll := range resolveLabeledLinks(pf, projectfile.LinkForum, "Discussions", lang) {
		rows = append(rows, tableRow{Kind: kindUsageQuestion, GoTo: mdLink(ll)})
	}

	for _, ll := range resolveLabeledLinks(pf, projectfile.LinkBugs, "Issues", lang) {
		rows = append(rows, tableRow{Kind: kindBug, GoTo: mdLink(ll)})
	}

	// Forum (Ideas) → "Request a feature" — only when a second forum link
	// exists or when forum has an "Ideas"-style label. We skip this for now
	// since the spec doesn't distinguish Q&A vs Ideas forum links; the user
	// can add a second `links[type=forum]` entry with a distinguishing label.

	for _, ll := range resolveLabeledLinks(pf, projectfile.LinkChat, "Chat", lang) {
		rows = append(rows, tableRow{Kind: kindChat, GoTo: mdLink(ll)})
	}

	security := core.LocalizedSibling(core.FileSecurity, lang)
	rows = append(rows, tableRow{Kind: kindSecurity, GoTo: "[" + security + "](" + security + ")"})

	contributing := core.LocalizedSibling(core.FileContributing, lang)
	rows = append(rows, tableRow{Kind: kindContributing, GoTo: "[" + contributing + "](" + contributing + ")"})

	for _, ll := range resolveLabeledLinks(pf, "paid-support", "Commercial Support", lang) {
		rows = append(rows, tableRow{Kind: kindPaidSupport, GoTo: mdLink(ll)})
	}

	return rows
}

// mdLink renders a resolved link as inline markdown.
func mdLink(ll labeledLink) string {
	return "[" + ll.Label + "](" + ll.URL + ")"
}

func valOrUnset(s string) string {
	if s == "" {
		return "(unset, section adapts)"
	}
	return s
}

func emitDecisionTrace(pf *projectfile.Document, ext *pfmodel.SupportExtension,
	beforeLinks []beforeLink, rows []tableRow,
	statusPageURL, responseTime, responseTimeSrc string,
) {
	genlog.Decision("project_name", pfmodel.DisplayName(pf), "identity.title.en or namespace/name", "")
	for _, bl := range beforeLinks {
		genlog.Decision("before_link", bl.Label+" → "+bl.URL, "links[]", "")
	}
	for _, r := range rows {
		genlog.Decision("table_row", r.Kind+" → "+r.GoTo, "links[] or local file", "")
	}
	genlog.Decision("status_page_url", valOrUnset(statusPageURL), "links[type=status-page]", "")
	genlog.Decision("response_time", responseTime, responseTimeSrc, "[org.projectfile.support].response-time")
	if len(ext.EOL) == 0 {
		genlog.Decision("eol", "(unset, section omitted)", "[org.projectfile.support].eol", "")
	} else {
		var entries []string
		for _, e := range ext.EOL {
			entries = append(entries, e.Version+" → "+e.Date)
		}
		genlog.Decision("eol", strings.Join(entries, ", "), "[org.projectfile.support].eol", "")
	}
}
