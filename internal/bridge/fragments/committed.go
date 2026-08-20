// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// The committed assembled document is the only record of inherited content:
// generate fetches the parents' published documents, and every other run —
// offline, check, preview, dry-run, or a generate whose fetch failed — reads
// the inherited sections back out of it. The H2 layout assembly emits is the
// contract this reader parses, so a reassembly is byte-identical and the drift
// gate stays offline and deterministic.

// extractInherited reads one committed assembled document and returns its
// inherited sections verbatim: every H2 that is not the project section is one
// parent section, heading line included. The trailing textlint directive of a
// localized document closes the capture instead of nesting — assembly re-wraps
// the whole variant. The second result reports whether the document exists;
// a missing document means nothing inherited yet, never an error.
func extractInherited(projectDir, out, projectHeading string) ([]inheritedEntry, bool) {
	raw, err := os.ReadFile(filepath.Join(projectDir, out)) // #nosec G304 -- out comes from the projectfile
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			genlog.Warn("fragments: committed document unreadable, inheriting nothing",
				"file", out, "error", err.Error())
		}
		return nil, false
	}

	var sections []inheritedEntry
	current := -1
	fence := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if current >= 0 {
				sections[current].Body += line + "\n"
			}
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			fence = "```"
		} else if strings.HasPrefix(trimmed, "~~~") {
			fence = "~~~"
		}
		if fence != "" {
			if current >= 0 {
				sections[current].Body += line + "\n"
			}
			continue
		}
		switch {
		case trimmed == projectHeading:
			current = -1
		case strings.HasPrefix(line, "## "):
			sections = append(sections, inheritedEntry{Heading: trimmed})
			current = len(sections) - 1
		case current >= 0 && textlintDirectiveRE.MatchString(line):
			current = -1
		case current >= 0:
			sections[current].Body += line + "\n"
		}
	}
	for i := range sections {
		sections[i].Body = strings.TrimSpace(sections[i].Body)
	}
	return sections, true
}

// localizedFromCanonical re-renders committed canonical sections for one
// language: the heading localizes, the body stays canonical — the child cannot
// translate text it does not own. Serves a variant that has translated
// fragments but no committed variant document, so an offline run still nests
// the same fallback an online run would fetch.
func localizedFromCanonical(sections []inheritedEntry, defLang, lang string) []inheritedEntry {
	out := make([]inheritedEntry, 0, len(sections))
	for _, s := range sections {
		cop := inheritedCopy{Title: parseInheritedHeading(s.Heading, defLang)}
		out = append(out, inheritedEntry{
			Heading: localizedInheritedHeading(cop, lang),
			Body:    s.Body,
		})
	}
	return out
}

// parseInheritedHeading splits a canonical section heading back into the
// parent's display name. The heading was rendered from defLang's plain
// inherited format ("Inherited from %s"), so the whole remainder is the name —
// a multi-word title round-trips. A heading an older bridge wrote with a
// trailing version keeps it in the name until the next online generate
// rewrites the committed document.
func parseInheritedHeading(heading, defLang string) string {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(heading), "##"))
	format, ok := fragmentString(defLang, keyInheritedPlain)
	if !ok {
		return text
	}
	prefix := format[:strings.IndexByte(format, '%')]
	return strings.TrimSpace(strings.TrimPrefix(text, prefix))
}
