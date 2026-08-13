// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"path"
	"strings"
)

// RelLink rewrites a repo-relative target so it resolves from the document's
// own directory. A document at the repo root keeps targets unchanged; a
// document under docs/<lang>/ gets targets rebased to its directory:
// ("FEATURES.md", "docs/es/README.md") -> "../../FEATURES.md".
//
// What we are trying to do: localized community files render under
// docs/<lang>/, so a link written repo-root-relative resolves one directory
// too deep. Every link emitter that knows its own output path rebases through
// here so the same target string works from the root file and from every
// variant. Uses forward-slash path math (never OS filepath): these are web
// paths, and the result must be "/"-separated on every build host.
func RelLink(target, docPath string) string {
	docDir := path.Dir(docPath)
	if docDir == "." {
		return target
	}
	return relPath(docDir, path.Clean(target))
}

// relPath returns the forward-slash relative path from base (a directory) to
// target, mirroring filepath.Rel for "/"-separated paths. Both inputs must be
// clean.
func relPath(base, target string) string {
	if base == "." {
		return target
	}
	b := strings.Split(base, "/")
	t := strings.Split(target, "/")
	i := 0
	for i < len(b) && i < len(t) && b[i] == t[i] {
		i++
	}
	var sb strings.Builder
	for j := i; j < len(b); j++ {
		if sb.Len() > 0 {
			sb.WriteByte('/')
		}
		sb.WriteString("..")
	}
	for j := i; j < len(t); j++ {
		if sb.Len() > 0 {
			sb.WriteByte('/')
		}
		sb.WriteString(t[j])
	}
	if sb.Len() == 0 {
		return "."
	}
	return sb.String()
}

// RelLinkSibling resolves a cross-reference to another community health file
// for the same language and rebases it against the current document's own
// path. ("SUPPORT.md", "es", "CONTRIBUTING.md") -> "SUPPORT.md" (same
// docs/es/ directory); with lang "" -> "SUPPORT.md" (root). The sibling and
// the document share one language, so both resolve under the same directory
// and the rebase is just the basename for a co-located sibling.
func RelLinkSibling(siblingBase, lang, docBase string) string {
	return RelLink(LocalizedFilename(siblingBase, lang), LocalizedFilename(docBase, lang))
}
