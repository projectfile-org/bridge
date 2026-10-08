// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"slices"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const notesNS = pfmodel.ReleaseExtensionNS + ".notes"

type section struct{ kind, title string }

// defaultSections are the shown commit types, in render order.
var defaultSections = []section{
	{"feat", "Features"},
	{"fix", "Bug fixes"},
	{"perf", "Performance"},
	{"revert", "Reverts"},
}

// commitSections applies `release.notes.sections` (kind → title, adds or renames) and `release.notes.hidden` to the defaults.
func commitSections(opts Options) []section {
	pf, _, err := projectfile.ReadWithOptions(opts.Dir, opts.Read)
	if err != nil {
		genlog.Debug("release notes config unread, default sections", "dir", opts.Dir, "err", err.Error())
		return defaultSections
	}
	raw, _ := projectfile.LookupExtension(pf, notesNS)
	cfg, _ := raw.(map[string]any)
	titles, _ := cfg["sections"].(map[string]any)
	hiddenRaw, _ := cfg["hidden"].([]any)
	var hidden []string
	for _, h := range hiddenRaw {
		if s, ok := h.(string); ok {
			hidden = append(hidden, s)
		}
	}
	var out []section
	seen := map[string]bool{}
	for _, s := range defaultSections {
		seen[s.kind] = true
		if title, ok := titles[s.kind].(string); ok && title != "" {
			s.title = title
		}
		out = append(out, s)
	}
	extra := make([]string, 0, len(titles))
	for kind := range titles {
		if !seen[kind] {
			extra = append(extra, kind)
		}
	}
	slices.Sort(extra)
	for _, kind := range extra {
		if title, ok := titles[kind].(string); ok && title != "" {
			out = append(out, section{kind, title})
		}
	}
	out = slices.DeleteFunc(out, func(s section) bool { return slices.Contains(hidden, s.kind) })
	genlog.Debug("release notes sections", "count", len(out), "hidden", len(hidden), "extra", len(extra))
	return out
}
