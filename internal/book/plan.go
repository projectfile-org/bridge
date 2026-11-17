// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package book plans one mdBook per language from the project’s documents and the page collections its executable renders.
package book

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// PagesDir is the book-relative directory each collection renders under, as PagesDir/<name>/.
const PagesDir = "pages"

// indexPage is the file that becomes a folder’s own chapter.
const indexPage = "index.md"

// Chapter is one SUMMARY.md entry; an empty Path is an mdBook draft chapter.
type Chapter struct {
	Title    string
	Path     string
	Source   string
	Fallback bool
	Children []Chapter
}

// Part is one titled SUMMARY.md part.
type Part struct {
	Title    string
	Chapters []Chapter
}

// Book is one language’s plan: its title, the README introduction and the non-empty parts.
type Book struct {
	Lang  string
	Title string
	Intro *Chapter
	Parts []Part
}

// Input names where a plan reads from: the project directory and the directory holding each rendered <name>/ collection.
type Input struct {
	Dir      string
	Rendered string
}

// Plan builds the book for lang ("" is the default language) from the conventions parts and the declared page collections.
func Plan(in Input, pf *projectfile.Document, lang string) (Book, error) {
	parts, err := pfmodel.GetBookParts(pf)
	if err != nil {
		return Book{}, err
	}
	collections, err := pfmodel.GetPageCollections(pf)
	if err != nil {
		return Book{}, err
	}
	concrete := core.ResolveLang(lang, pf)
	b := Book{Lang: concrete, Title: pfmodel.DisplayNameForLang(pf, concrete)}
	if c, ok := page(in.Dir, core.FileReadme, lang); ok {
		b.Intro = &c
	}
	claimed := explicitPages(parts)
	seen := map[string]bool{core.FileReadme: true}
	for _, p := range parts {
		title := projectfile.ExtractLocalizedStringForLang(p.Title, concrete)
		var chapters []Chapter
		for _, pattern := range p.Pages {
			for _, rel := range expand(in.Dir, pattern, claimed) {
				if seen[rel] {
					genlog.Debug("book page already listed", "page", rel, "part", title)
					continue
				}
				seen[rel] = true
				if c, ok := page(in.Dir, rel, lang); ok {
					chapters = append(chapters, c)
				}
			}
		}
		if p.Collections {
			for _, col := range collections {
				if c, ok := collection(in.Rendered, col, concrete); ok {
					chapters = append(chapters, c)
				}
			}
		}
		if len(chapters) == 0 {
			genlog.Debug("book part dropped as empty", "part", title, "lang", concrete)
			continue
		}
		b.Parts = append(b.Parts, Part{Title: title, Chapters: chapters})
	}
	genlog.Debug("book planned", "lang", concrete, "parts", len(b.Parts))
	return b, nil
}

// explicitPages collects every non-glob page the parts name, so a glob never lists a page another part claims by name.
func explicitPages(parts []pfmodel.BookPart) map[string]bool {
	out := map[string]bool{}
	for _, p := range parts {
		for _, pattern := range p.Pages {
			if !strings.ContainsAny(pattern, "*?[") {
				out[pattern] = true
			}
		}
	}
	return out
}

// expand turns one page entry into repo-relative Markdown paths, sorted, skipping globbed pages claimed by name.
func expand(dir, pattern string, claimed map[string]bool) []string {
	if !strings.ContainsAny(pattern, "*?[") {
		return []string{pattern}
	}
	matches, err := filepath.Glob(filepath.Join(dir, filepath.FromSlash(pattern)))
	if err != nil {
		genlog.Warn("book page pattern is malformed", "pattern", pattern, "error", err.Error())
		return nil
	}
	var out []string
	for _, m := range matches {
		rel, _ := filepath.Rel(dir, m)
		rel = filepath.ToSlash(rel)
		if info, err := os.Stat(m); err != nil || info.IsDir() || !strings.EqualFold(path.Ext(rel), ".md") || claimed[rel] {
			genlog.Debug("book glob match skipped", "pattern", pattern, "match", rel)
			continue
		}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

// page resolves rel for lang: the docs/<lang>/ variant at its own path, so its relative links hold, else the canonical page marked Fallback.
func page(dir, rel, lang string) (Chapter, bool) {
	if lang != "" {
		if loc := core.LocalizedFilename(rel, lang); exists(dir, loc) {
			return chapter(dir, loc, false), true
		}
	}
	if !exists(dir, rel) {
		genlog.Debug("book page absent", "page", rel, "lang", lang)
		return Chapter{}, false
	}
	if lang != "" {
		genlog.Info("book page has no translation, using the canonical page", "page", rel, "lang", lang)
	}
	return chapter(dir, rel, lang != ""), true
}

// chapter labels dir/rel by its H1, falling back to its file name, and places it at the same path in the book.
func chapter(dir, rel string, fallback bool) Chapter {
	title := core.FirstHeading(dir, rel)
	if title == "" {
		title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	}
	return Chapter{Title: title, Path: rel, Source: filepath.Join(dir, filepath.FromSlash(rel)), Fallback: fallback}
}

// collection plans one rendered collection as a chapter titled by its declaration, with the rendered tree under it.
func collection(rendered string, col pfmodel.PageCollection, lang string) (Chapter, bool) {
	root := filepath.Join(rendered, col.Name)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		genlog.Warn("page collection was not rendered", "collection", col.Name, "dir", root)
		return Chapter{}, false
	}
	title := projectfile.ExtractLocalizedStringForLang(col.Title, lang)
	if title == "" {
		title = col.Name
	}
	c := folder(root, path.Join(PagesDir, col.Name), title)
	genlog.Debug("page collection planned", "collection", col.Name, "lang", lang, "pages", len(c.Children))
	return c, true
}

// folder plans the directory abs placed at the book path rel: its index.md is its own chapter, every other entry a child.
func folder(abs, rel, title string) Chapter {
	c := Chapter{Title: title}
	if exists(abs, indexPage) {
		c.Path = path.Join(rel, indexPage)
		c.Source = filepath.Join(abs, indexPage)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		genlog.Warn("page collection folder is unreadable", "dir", abs, "error", err.Error())
		return c
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir():
			c.Children = append(c.Children, folder(filepath.Join(abs, name), path.Join(rel, name), name))
		case name != indexPage && strings.EqualFold(path.Ext(name), ".md"):
			child := chapter(abs, name, false)
			child.Path = path.Join(rel, name)
			c.Children = append(c.Children, child)
		default:
			genlog.Debug("page collection entry skipped", "dir", abs, "entry", name)
		}
	}
	return c
}

// exists reports whether rel is a regular file inside dir; a path leaving dir never exists.
func exists(dir, rel string) bool {
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		genlog.Warn("book page path leaves the project, skipped", "dir", dir, "page", rel)
		return false
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(filepath.FromSlash(rel))
	return err == nil && !info.IsDir()
}
