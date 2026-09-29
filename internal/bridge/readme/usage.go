// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// helpTitleSuffix ends the title of every captured `<command> --help` section.
const helpTitleSuffix = "--help`"

// usageSection is one level-3 section of USAGE.md: its heading text and its body verbatim.
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

// buildUsageDoc picks the top-level help and the first example from this language's USAGE.md; nil when neither exists.
func buildUsageDoc(dir, pathLang string) *usageDoc {
	rel := fileUsage
	if localized := core.LocalizedFilename(fileUsage, pathLang); pathLang != "" && fileExists(dir, localized) {
		rel = localized
	}
	body, err := readFile(dir, rel)
	if err != nil {
		return nil
	}
	var help, example bool
	out := &usageDoc{Name: fileUsage, Filename: core.RelLink(rel, readmeDocPath(pathLang))}
	for _, s := range parseUsageSections(string(body)) {
		isHelp := strings.HasSuffix(s.Title, helpTitleSuffix)
		if (isHelp && help) || (!isHelp && example) {
			continue
		}
		help, example = help || isHelp, example || !isHelp
		genlog.DebugRow("usage_excerpt", s.Title, rel, "help="+boolWord(isHelp))
		out.Sections = append(out.Sections, s)
	}
	if len(out.Sections) == 0 {
		return nil
	}
	return out
}

// parseUsageSections splits the project part of an assembled USAGE.md into its level-3 sections, skipping fenced code.
func parseUsageSections(text string) []usageSection {
	var out []usageSection
	var body []string
	fenced, h2 := false, 0
	flush := func() {
		if len(out) > 0 {
			out[len(out)-1].Body = strings.TrimSpace(strings.Join(body, "\n"))
		}
		body = nil
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		level, title, ok := parseATXHeading(line)
		switch {
		case fenced || !ok || level > 3:
			body = append(body, line)
		case level == 3:
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

// boolWord renders a flag for a trace row.
func boolWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
