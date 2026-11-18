// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"strings"

	"github.com/pelletier/go-toml/v2"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// summaryIndent is the nesting step mdBook reads in SUMMARY.md.
const summaryIndent = "    "

// fallbackNotes is the note a fallback page opens with, in the language of the book it stands in.
var fallbackNotes = map[string]string{
	"en": "This page is not translated yet.",
	"es": "Esta página aún no está traducida.",
	"uk": "Ця сторінка ще не перекладена.",
}

// summaryTitles is the heading mdBook expects atop SUMMARY.md, per language.
var summaryTitles = map[string]string{
	"en": "Summary",
	"es": "Resumen",
	"uk": "Зміст",
}

// forLang picks lang’s entry from a string map, falling back to English.
func forLang(m map[string]string, lang string) string {
	if v, ok := m[lang]; ok {
		return v
	}
	return m["en"]
}

// FallbackNote is the blockquote a fallback page is prefixed with in lang’s book.
func FallbackNote(lang string) string {
	return "> " + forLang(fallbackNotes, lang) + "\n\n"
}

// Summary renders b as an mdBook SUMMARY.md.
func Summary(b Book) []byte {
	var s strings.Builder
	s.WriteString("# " + forLang(summaryTitles, b.Lang) + "\n\n")
	if b.Intro != nil {
		s.WriteString(link(*b.Intro) + "\n\n")
	}
	for _, p := range b.Parts {
		s.WriteString("# " + p.Title + "\n\n")
		for _, c := range p.Chapters {
			writeChapter(&s, c, 0)
		}
		s.WriteString("\n")
	}
	return []byte(strings.TrimRight(s.String(), "\n") + "\n")
}

// writeChapter writes c as a list item at depth, then its children one level deeper.
func writeChapter(s *strings.Builder, c Chapter, depth int) {
	s.WriteString(strings.Repeat(summaryIndent, depth) + "- " + link(c) + "\n")
	for _, child := range c.Children {
		writeChapter(s, child, depth+1)
	}
}

// link renders c as a SUMMARY.md link; an empty path is a draft, and a path with a space is angle-bracketed.
func link(c Chapter) string {
	title := strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`).Replace(c.Title)
	target := c.Path
	if strings.ContainsAny(target, " ()") {
		target = "<" + target + ">"
	}
	return "[" + title + "](" + target + ")"
}

// bookToml is the book.toml mdBook reads: the book’s identity and the HTML output’s repository link.
type bookToml struct {
	Book struct {
		Title    string `toml:"title"`
		Language string `toml:"language"`
		Src      string `toml:"src"`
	} `toml:"book"`
	Output struct {
		HTML struct {
			GitRepositoryURL string `toml:"git-repository-url,omitempty"`
			SidebarHeaderNav bool   `toml:"sidebar-header-nav"` // false: the sidebar lists chapters, never a page’s headings
		} `toml:"html"`
	} `toml:"output"`
}

// Toml renders b’s book.toml with its sources under src.
func Toml(b Book, pf *projectfile.Document, src string) ([]byte, error) {
	var t bookToml
	t.Book.Title = b.Title
	t.Book.Language = b.Lang
	t.Book.Src = src
	t.Output.HTML.GitRepositoryURL = pfmodel.CitableRepositoryURL(pf)
	return toml.Marshal(t)
}
