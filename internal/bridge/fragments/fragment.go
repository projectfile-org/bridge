// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// leadingSPDXRE matches a leading `<!-- … -->` REUSE HTML comment so the
// assembled body does not carry each fragment's per-file header (the
// document carries one SPDX block at the top, derived from the
// projectfile). `(?s)` makes `.` match newlines (Go's default does not)
// so a multi-line SPDX block collapses; non-greedy `.*?` stops at the
// first `-->`.
var leadingSPDXRE = regexp.MustCompile(`(?s)\A\s*<!--.*?-->\s*`)

// textlintDirectiveRE matches a whole-line textlint directive comment
// (`<!-- textlint-disable … -->` / `<!-- textlint-enable … -->`). Translated
// fragment sources carry the pair so the English-tuned dictionary rules skip
// natural non-English prose; in assembly it is a source pragma, not content —
// each variant gets one whole-file wrap from wrapAfterHeader instead.
var textlintDirectiveRE = regexp.MustCompile(`(?m)^<!--\s*textlint-(?:disable|enable)[^>]*-->\n?`)

// multiBlankRE matches any run of three or more consecutive newlines so
// the assembled document collapses to at most one blank line between
// blocks — the template's per-fragment range leaves a trailing blank line
// after every fragment (each section's last included), so section
// boundaries stack into MD012 multiple-blank-line violations.
var multiBlankRE = regexp.MustCompile(`\n{3,}`)

// Fragment is one parsed docs/<name>.d/*.md file: its title (for the H3
// heading) and the body with the SPDX comment stripped and H1 demoted to H3.
// The H3 shape is load-bearing — the readme bridge scrapes level-3
// headings out of FEATURES.md to build its features bullet list.
type Fragment struct {
	SourceName string // basename without extension; title fallback
	Title      string // H1 text, or SourceName when no H1
	Body       string // SPDX-stripped, H1-demoted content
}

// parseFragment reads and transforms one fragment file. Errors are
// returned to the caller; the assembler decides whether to skip or fail.
func parseFragment(path string) (Fragment, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is a glob match under the project dir
	if err != nil {
		return Fragment{}, err
	}
	body := leadingSPDXRE.ReplaceAllString(string(raw), "")
	body = textlintDirectiveRE.ReplaceAllString(body, "")
	body = strings.TrimSpace(body)

	f := Fragment{SourceName: strings.TrimSuffix(filepath.Base(path), ".md")}

	lines := strings.Split(body, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		f.Title = strings.TrimSpace(strings.TrimPrefix(lines[0], "# "))
		// Demote the H1 to H3 — the document owns the H1 (the Title), and
		// per-feature entries live one level below the section headings.
		lines[0] = "##" + lines[0]
	} else {
		f.Title = f.SourceName
	}
	f.Body = strings.TrimSpace(strings.Join(lines, "\n"))
	return f, nil
}

// loadFragments reads docs/<dir>/*.md from projectDir, sorted by path for
// deterministic output regardless of filesystem glob order — load-bearing
// for reproducible builds. Missing dir yields (nil, nil) (an empty
// section, not an error). Each body's links are rebased onto out, the document it assembles into.
func loadFragments(projectDir, dir, out string) ([]Fragment, error) {
	absDir := filepath.Join(projectDir, dir)
	entries, err := filepath.Glob(filepath.Join(absDir, "*.md"))
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, nil
	}
	sort.Slice(entries, func(i, j int) bool { // by stem, so a.md precedes a-b.md
		return strings.TrimSuffix(entries[i], ".md") < strings.TrimSuffix(entries[j], ".md")
	})
	frags := make([]Fragment, 0, len(entries))
	for _, p := range entries {
		f, err := parseFragment(p)
		if err != nil {
			return nil, err
		}
		f.Body = rebaseOwn(projectDir, filepath.ToSlash(dir), out, f.Body)
		frags = append(frags, f)
	}
	return frags, nil
}

// rebaseOwn points each relative link of a fragment in dir at the same file from out: read against the fragment first, then against out, whichever exists.
func rebaseOwn(projectDir, dir, out, body string) string {
	return string(core.RewriteLinks([]byte(body), func(raw string) string {
		if !core.IsRelativeLink(raw) {
			return raw
		}
		target, suffix := core.SplitLinkSuffix(raw)
		decoded, err := url.PathUnescape(target)
		if err != nil {
			return raw
		}
		fromFragment := path.Clean(path.Join(dir, decoded))
		if exists(projectDir, fromFragment) {
			rebased := core.RelLink(fromFragment, out)
			if rebased != target {
				genlog.Debug("fragments: own link rebased onto the document", "dir", dir, "target", raw, "rebased", rebased)
			}
			return strings.ReplaceAll(rebased, " ", "%20") + suffix
		}
		if !exists(projectDir, path.Clean(path.Join(path.Dir(out), decoded))) {
			genlog.Warn("fragments: own link resolves neither from its fragment nor from the document", "dir", dir, "target", raw)
		}
		return raw
	}))
}

// exists reports whether the repository-relative rel names a file or directory inside projectDir.
func exists(projectDir, rel string) bool {
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return false
	}
	_, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel)))
	return err == nil
}
