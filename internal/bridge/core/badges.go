// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"slices"
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// DocumentReadme is the document a shield declaring no `documents` renders on.
const DocumentReadme = "readme"

// ciNS is where the CI matrix axes a badge must not name are declared.
const ciNS = "org.projectfile.ci"

// Badge is one resolved badge: Row groups it into a line, Priority orders it within that line.
type Badge struct {
	Alt      string
	Img      string
	Href     string
	Row      string
	Priority int
}

// BadgeRow is one rendered line of badges; Name is the declared row, kept for the decision trace.
type BadgeRow struct {
	Name   string
	Badges []Badge
}

// Badges resolves the shields that render on document, in declaration order.
//
// A shield whose img or href keeps an unresolved `${…}` or names a CI matrix axis
// is dropped. Survivors dedupe by `name`, last wins at the first position, BEFORE the
// document filter, so a redeclaration can move an inherited badge to another document.
func Badges(doc *projectfile.Document, shields []pfmodel.Shield, document, lang string) []Badge {
	axes := pfmodel.MatrixAxes(doc, ciNS)
	var resolved []resolvedShield
	position := make(map[string]int, len(shields))
	for _, s := range shields {
		img, href := interp.Expand(doc, s.Img), interp.Expand(doc, hrefForLang(s, lang))
		if img == "" || interp.Unresolved(img) || interp.Unresolved(href) {
			genlog.DebugRow("badge", s.Name, "unresolved reference (dropped)", img)
			continue
		}
		if namesMatrixAxis(img, axes) || namesMatrixAxis(href, axes) {
			genlog.DebugRow("badge", s.Name, "names a CI-matrix series (dropped)", img)
			continue
		}
		alt := interp.Expand(doc, s.Alt)
		if alt == "" {
			alt = s.Name
		}
		r := resolvedShield{Badge{Alt: alt, Img: img, Href: href, Row: s.Row, Priority: pfmodel.RankOf(s.Priority)}, s.Documents}
		if at, seen := position[s.Name]; seen {
			genlog.DebugRow("badge", s.Name, "redeclared (last wins)", resolved[at].Img)
			resolved[at] = r
			continue
		}
		position[s.Name] = len(resolved)
		resolved = append(resolved, r)
	}
	var out []Badge
	for _, r := range resolved {
		if !rendersOn(r.documents, document) {
			genlog.DebugRow("badge", r.Alt, "belongs to another document (skipped)", document+" documents="+strings.Join(r.documents, ","))
			continue
		}
		out = append(out, r.Badge)
	}
	return out
}

// resolvedShield is a badge still carrying the documents it renders on.
type resolvedShield struct {
	Badge
	documents []string
}

// BadgeRows groups the document's badges into lines by `row`, rows in first-seen order, badges by priority within a row.
func BadgeRows(doc *projectfile.Document, shields []pfmodel.Shield, document, lang string) []BadgeRow {
	var rows []BadgeRow
	at := make(map[string]int)
	for _, b := range Badges(doc, shields, document, lang) {
		i, seen := at[b.Row]
		if !seen {
			i = len(rows)
			at[b.Row] = i
			rows = append(rows, BadgeRow{Name: b.Row})
			genlog.DebugRow("badge_row", BadgeRowLabel(b.Row), "first appearance", document+" "+strconv.Itoa(i+1))
		}
		rows[i].Badges = append(rows[i].Badges, b)
	}
	for i := range rows {
		slices.SortStableFunc(rows[i].Badges, func(a, b Badge) int { return pfmodel.ByPriorityDesc(a.Priority, b.Priority) })
	}
	return rows
}

// BadgeMarkdown renders rows as markdown, one line per row.
func BadgeMarkdown(rows []BadgeRow) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		cells := make([]string, 0, len(r.Badges))
		for _, b := range r.Badges {
			img := "![" + b.Alt + "](" + b.Img + ")"
			if b.Href != "" {
				img = "[" + img + "](" + b.Href + ")"
			}
			cells = append(cells, img)
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n\n")
}

// BadgeRowLabel renders a row name for logs, naming the unnamed row.
func BadgeRowLabel(row string) string {
	if row == "" {
		return "(unnamed)"
	}
	return row
}

// rendersOn reports whether a badge declared for documents belongs to document; declaring none means the readme.
func rendersOn(documents []string, document string) bool {
	if len(documents) == 0 {
		return document == DocumentReadme
	}
	return slices.Contains(documents, document)
}

// hrefForLang resolves a shield's href for lang, falling back to the default href.
func hrefForLang(s pfmodel.Shield, lang string) string {
	if v := s.HrefByLang[lang]; lang != "" && v != "" {
		return v
	}
	return s.Href
}

// namesMatrixAxis reports whether s still carries a `{AXIS}` placeholder of a declared CI matrix axis.
func namesMatrixAxis(s string, axes map[string][]string) bool {
	for axis := range axes {
		if strings.Contains(s, "{"+axis+"}") {
			return true
		}
	}
	return false
}
