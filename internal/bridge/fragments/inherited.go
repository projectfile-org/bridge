// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// inheritedDir holds one cached copy per parent, under the document's own
// fragment directory (docs/features.d/.inherited/b19-ubuntu.md); a translated
// copy caches under the locale variant of that path (docs/es/features.d/
// .inherited/b19-ubuntu.md). Dot-prefixed so loadFragments' `*.md` glob never
// mistakes a parent's copy for one of this project's own fragments.
//
// The copies are committed on purpose: assembly then reads local files only, so
// the drift gate needs no network and two runs on the same tree always agree.
// Refreshing is the one step that reaches a forge, and it lands as a reviewable
// diff of what upstream changed.
const inheritedDir = ".inherited"

// provenanceRE reads the machine-written comment that says WHICH version of the
// parent this copy holds. Written by refresh, read by assembly; the version it
// carries is what the section heading states, which is what keeps the heading
// true after upstream moves on.
var provenanceRE = regexp.MustCompile(`(?m)^<!--\s*pf-bridge:inherited\s+(.*?)\s*-->\s*$`)

// attrRE splits a provenance comment into key="value" pairs.
var attrRE = regexp.MustCompile(`(\w+)="([^"]*)"`)

// inheritedCopy is one parent's cached document: where it came from, which
// version, and the entry blocks to nest under this project's document.
type inheritedCopy struct {
	Name     string // parent project name (owner/repo from its URL)
	Title    string // the parent's identity.title, empty when it publishes none
	URL      string // parent forge repository
	Ref      string // release tag the copy was read at, empty for a branch read
	Commit   string // commit the copy was read at
	Document string // parent file the copy came from (FEATURES.md, …)
	SPDX     string // the parent's REUSE header, kept with the text it licenses
	Body     string // H3 entry blocks, ready to nest
	Lang     string // language the Body is translated into; empty when canonical
}

// Heading is the H2 the copy renders under. It names the version, so the claim
// is "these were B19/Ubuntu 1.0.0's features" — which stays true whatever
// upstream does next — rather than an anonymous "inherited", which does not.
func (c inheritedCopy) Heading() string {
	switch {
	case c.Ref != "":
		return "## Inherited from " + c.displayName() + " " + c.Ref
	case c.Commit != "":
		return "## Inherited from " + c.displayName() + " " + shortCommit(c.Commit)
	default:
		return "## Inherited from " + c.displayName()
	}
}

// displayName is what a heading calls the parent. The parent's own title is
// preferred because a heading is prose a person reads, and a title is written
// as prose ("B19/Ubuntu") where owner/repo is a path ("b19/ubuntu") — which
// reads as a typo to both people and prose linters.
//
// owner/repo remains the fallback: it is derived from the URL, so a parent that
// publishes no title still names itself.
func (c inheritedCopy) displayName() string {
	if c.Title != "" {
		return c.Title
	}
	return c.Name
}

// loadInherited reads every cached parent copy for a document, keyed by the
// cache filename so a copy refreshed in this same run replaces the one on disk
// instead of appearing twice. A missing directory means this project inherits
// nothing yet — not an error, since refresh has simply never run.
func loadInherited(projectDir, dir string) (map[string]inheritedCopy, error) {
	absDir := filepath.Join(projectDir, dir, inheritedDir)
	entries, err := filepath.Glob(filepath.Join(absDir, "*.md"))
	if err != nil {
		return nil, err
	}

	out := map[string]inheritedCopy{}
	for _, p := range entries {
		copied, err := parseInherited(p)
		if err != nil {
			return nil, err
		}
		if copied.Body == "" {
			genlog.Warn("fragments: cached parent copy is empty, skipping", "file", p, "parent", copied.Name)
			continue
		}
		genlog.Info("fragments: inheriting", "parent", copied.Name, "ref", copied.Ref, "dir", dir)
		out[strings.TrimSuffix(filepath.Base(p), ".md")] = copied
	}
	return out, nil
}

// declaredOnly keeps the copies whose parent the document still declares. The
// projectfile is what says who this project inherits from; a copy left behind
// by a parent that was dropped would otherwise keep appearing in the document
// forever, since nothing else ever removes it.
func declaredOnly(copies map[string]inheritedCopy, doc pfmodel.FragmentDocument) map[string]inheritedCopy {
	declared := make(map[string]struct{}, len(doc.Parents))
	for _, parent := range doc.Parents {
		declared[slug(ownerRepo(parent.URL))] = struct{}{}
	}

	out := make(map[string]inheritedCopy, len(copies))
	for name, copied := range copies {
		if _, ok := declared[name]; !ok {
			genlog.Warn("fragments: cached copy has no declared parent, ignoring",
				"parent", copied.Name, "document", doc.Out, "hint", "delete "+path.Join(doc.Dir, inheritedDir, name+".md"))
			continue
		}
		out[name] = copied
	}
	return out
}

// orderedCopies flattens the keyed copies into the order they render, sorted by
// cache filename so the assembled document is byte-identical run to run — the
// property the drift gate compares against.
func orderedCopies(copies map[string]inheritedCopy) []inheritedCopy {
	keys := make([]string, 0, len(copies))
	for k := range copies {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]inheritedCopy, 0, len(keys))
	for _, k := range keys {
		out = append(out, copies[k])
	}
	return out
}

// parseInherited splits a cached copy into its provenance and its body. The
// provenance comment is pulled out BEFORE the SPDX header is stripped: the SPDX
// block is the parent's, kept verbatim so the vendored text stays attributed to
// whoever wrote it, and stripping first would eat whichever comment came later.
func parseInherited(path string) (inheritedCopy, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is a glob match under the project dir
	if err != nil {
		return inheritedCopy{}, err
	}
	text := string(raw)

	c := inheritedCopy{Name: strings.TrimSuffix(filepath.Base(path), ".md")}
	if m := provenanceRE.FindStringSubmatch(text); m != nil {
		for _, kv := range attrRE.FindAllStringSubmatch(m[1], -1) {
			switch kv[1] {
			case "name":
				c.Name = kv[2]
			case "title":
				c.Title = kv[2]
			case "url":
				c.URL = kv[2]
			case "ref":
				c.Ref = kv[2]
			case "commit":
				c.Commit = kv[2]
			case "document":
				c.Document = kv[2]
			case "lang":
				c.Lang = kv[2]
			}
		}
		text = provenanceRE.ReplaceAllString(text, "")
	} else {
		genlog.Warn("fragments: cached parent copy has no provenance comment", "file", path)
	}

	// A translated copy carries the textlint wrap on disk (the cache file is
	// linted by this project); assembly re-wraps the whole variant, so the
	// pair is a storage pragma here, never content.
	text = textlintDirectiveRE.ReplaceAllString(text, "")
	c.SPDX, c.Body = splitSPDX(text)
	return c, nil
}

// textlint wrap lines bracketing a translated copy on disk — the same pair
// core wraps every localized render with; the cache file is linted by this
// project, not by upstream.
const (
	textlintDisableLine = "<!-- textlint-disable terminology,common-misspellings -->"
	textlintEnableLine  = "<!-- textlint-enable -->"
)

// renderInherited builds the file refresh writes: the parent's own SPDX header,
// then the provenance, then the entry blocks. No fetch date is recorded — a
// timestamp would rewrite the file on every refresh even when upstream did not
// move, which turns an honest "nothing changed" into drift. A translated copy
// brackets its body with the textlint pair; parseInherited strips it back out.
func renderInherited(c inheritedCopy) []byte {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(c.SPDX))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "<!-- pf-bridge:inherited name=%q title=%q url=%q ref=%q commit=%q document=%q",
		c.Name, c.Title, c.URL, c.Ref, c.Commit, c.Document)
	if c.Lang != "" {
		fmt.Fprintf(&b, " lang=%q", c.Lang)
	}
	b.WriteString(" -->\n\n")
	if c.Lang != "" {
		b.WriteString(textlintDisableLine + "\n\n")
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n\n" + textlintEnableLine + "\n")
	} else {
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// shortCommit abbreviates a commit for display, matching git's default width.
func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
