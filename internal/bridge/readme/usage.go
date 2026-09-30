// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// isHelpCapture reports whether a section is a captured help screen: its body opens with `$ <title> --help`.
func isHelpCapture(s usageSection) bool {
	return strings.HasPrefix(s.Body, "```console\n$ "+s.Title+" --help\n")
}

// usageSection is one section of USAGE.md: its heading text and its body verbatim.
type usageSection struct {
	Title string
	Body  string
}

// usageDoc is the usage block's excerpt of USAGE.md and the link to the whole document.
type usageDoc struct {
	Name     string
	Filename string
	Sections []usageSection
}

// buildUsageDoc picks the top-level help from this language's USAGE.md, else its first example; nil when neither exists.
func buildUsageDoc(dir, pathLang string) *usageDoc {
	rel := fileUsage
	if localized := core.LocalizedFilename(fileUsage, pathLang); pathLang != "" && fileExists(dir, localized) {
		rel = localized
	}
	body, err := readFile(dir, rel)
	if err != nil {
		return nil
	}
	sections := parseUsageSections(string(body))
	if len(sections) == 0 {
		return nil
	}
	pick := sections[0]
	for _, s := range sections {
		if isHelpCapture(s) {
			pick = s
			break
		}
	}
	genlog.DebugRow("usage_excerpt", pick.Title, rel, "help="+boolWord(isHelpCapture(pick)))
	return &usageDoc{Name: fileUsage, Filename: core.RelLink(rel, readmeDocPath(pathLang)), Sections: []usageSection{pick}}
}

// parseUsageSections splits the project part of an assembled USAGE.md into its sections, skipping fenced code.
func parseUsageSections(text string) []usageSection {
	lines := strings.Split(text, "\n")
	depth := usageSectionDepth(lines)
	var out []usageSection
	var body []string
	fenced, h2 := false, 0
	flush := func() {
		if len(out) > 0 {
			out[len(out)-1].Body = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		level, title, ok := parseATXHeading(line)
		switch {
		case !fenced && strings.HasPrefix(line, "<!-- textlint-"):
			genlog.DebugRow("usage_directive_dropped", line, fileUsage, "")
		case fenced || !ok || level > depth:
			body = append(body, line)
		case level == depth:
			flush()
			out = append(out, usageSection{Title: title})
		case level == 2:
			flush()
			if h2++; h2 > 1 {
				return out
			}
		}
	}
	flush()
	return out
}

// usageSectionDepth is 3 when the sections nest under a project H2 (an inheriting document), 2 when they sit flat.
func usageSectionDepth(lines []string) int {
	var levels []int
	fenced := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if level, _, ok := parseATXHeading(line); !fenced && ok && level > 1 {
			if levels = append(levels, level); len(levels) == 2 {
				break
			}
		}
	}
	if len(levels) == 2 && levels[0] == 2 && levels[1] == 3 {
		return 3
	}
	return 2
}

// boolWord renders a flag for a trace row.
func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
