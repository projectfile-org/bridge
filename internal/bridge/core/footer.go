// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "strings"

// howToBase is the projectfile.org how-to URL every generated document's
// footer links to, with the document slug appended. The how-to pages are the
// authority on what each generated file contains and how to customize it, so
// the footer points there rather than at the spec.
const howToBase = "https://projectfile.org/how-to/"

// docSlugs maps each canonical community-health filename to its how-to slug.
// A document whose name has no entry falls back to the readme how-to.
var docSlugs = map[string]string{
	FileReadme:        "readme",
	FileSupport:       "support",
	FileContributing:  "contributing",
	FileCodeOfConduct: "code-of-conduct",
	FileSecurity:      "security",
	FileDEI:           "dei",
}

// footerStrings is the per-language footer copy: Prefix is the
// "Generated from projectfile" lead-in, Learn is the link text for the how-to.
// lang is the resolved BCP 47 tag (callers pass ResolveLang output); an
// unknown tag falls back to English so every render gets a footer.
var footerStrings = map[string]struct{ Prefix, Learn string }{
	"en": {"Generated from projectfile", "learn how"},
	"es": {"Generado desde projectfile", "saber cómo"},
	"uk": {"Згенеровано з projectfile", "дізнатися як"},
}

// GeneratedFooter returns the italic one-line footer appended to every
// generated community-health document: "*Generated from projectfile
// ([learn how](…))*". filename is the canonical basename (README.md, …) so the
// how-to slug matches the document; lang selects the copy. The whole line is
// italic so a reader can tell generated prose from hand-written at a glance.
func GeneratedFooter(filename, lang string) string {
	slug := docSlugs[filename]
	if slug == "" {
		slug = docSlugs[FileReadme]
	}
	s, ok := footerStrings[lang]
	if !ok {
		s = footerStrings["en"]
	}
	var sb strings.Builder
	sb.WriteString("*")
	sb.WriteString(s.Prefix)
	sb.WriteString(" ([")
	sb.WriteString(s.Learn)
	sb.WriteString("](")
	sb.WriteString(howToBase)
	sb.WriteString(slug)
	sb.WriteString("))*")
	return sb.String()
}
