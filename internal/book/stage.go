// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// srcDir is the book.toml source root every staged book uses.
const srcDir = "src"

// linkTarget captures the target of a Markdown inline link or image: an angle-bracketed one in group 1, a bare one in group 2, title excluded.
var linkTarget = regexp.MustCompile(`\]\(\s*(?:<([^>\n]+)>|([^)\s]+))(?:\s+"[^"]*")?\s*\)`)

// Stage writes one book per language under out/<lang>/, reading collections rendered under rendered/<lang>/; it returns the languages staged.
func Stage(dir, rendered, out string, pf *projectfile.Document) ([]string, error) {
	var books []Book
	for _, lang := range append([]string{""}, pfmodel.Languages(pf)...) {
		b, err := Plan(Input{Dir: dir, Rendered: filepath.Join(rendered, core.ResolveLang(lang, pf))}, pf, lang)
		if err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	s := newSite(books, pf)
	var staged []string
	for _, b := range books {
		if err := stageOne(dir, filepath.Join(out, b.Lang), b, pf, s); err != nil {
			return staged, fmt.Errorf("stage %s book: %w", b.Lang, err)
		}
		genlog.Info("book staged", "lang", b.Lang, "dir", filepath.Join(out, b.Lang))
		staged = append(staged, b.Lang)
	}
	return staged, nil
}

// stageOne builds b in a temporary sibling of target and swaps it in, so a failed run leaves the previous book whole.
func stageOne(dir, target string, b Book, pf *projectfile.Document, s *site) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(target), ".book-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := writeBook(dir, tmp, b, pf, s); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		if _, err := os.Stat(filepath.Join(target, "book.toml")); err != nil {
			return fmt.Errorf("%s exists and is not a staged book, refusing to replace it", target)
		}
		genlog.Debug("book replaces the previous stage", "dir", target)
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return os.Rename(tmp, target)
}

// writeBook writes book.toml, src/SUMMARY.md, every chapter with its links rewritten and every media file they link to under tmp.
func writeBook(dir, tmp string, b Book, pf *projectfile.Document, s *site) error {
	project, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = project.Close() }()
	root, err := os.OpenRoot(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	toml, err := Toml(b, pf, srcDir)
	if err != nil {
		return err
	}
	if err := writeFile(root, "book.toml", toml); err != nil {
		return err
	}
	if err := writeFile(root, path.Join(srcDir, "SUMMARY.md"), Summary(b)); err != nil {
		return err
	}
	chapters := b.chapters()
	var stats linkStats
	for _, c := range chapters {
		body, err := os.ReadFile(c.Source) // #nosec G304 -- Source is a page the planner found under the project or rendered tree
		if err != nil {
			return err
		}
		var assets []string
		if c.Origin != "" {
			body, assets = s.rewrite(project, b.Lang, c, body, &stats)
		}
		if c.Fallback {
			body = append([]byte(FallbackNote(b.Lang)), body...)
		}
		if c.Extras != "" {
			extras, err := project.ReadFile(filepath.FromSlash(c.Extras))
			if err != nil {
				return err
			}
			body = append(append(bytes.TrimRight(body, "\n"), "\n\n"...), extras...)
		}
		if err := writeFile(root, path.Join(srcDir, c.Path), body); err != nil {
			return err
		}
		for _, asset := range assets {
			if err := copyAsset(project, root, asset); err != nil {
				return err
			}
		}
	}
	genlog.Info("book links rewritten", "lang", b.Lang, "kept", stats.kept, "cross_book", stats.crossBook, "forge", stats.forge, "asset", stats.asset, "unresolved", stats.unresolved)
	genlog.Debug("book written", "lang", b.Lang, "chapters", len(chapters))
	return nil
}

// chapters flattens the introduction and every part into the chapters that carry a file.
func (b Book) chapters() []Chapter {
	var out []Chapter
	var walk func(c Chapter)
	walk = func(c Chapter) {
		if c.Path != "" {
			out = append(out, c)
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	if b.Intro != nil {
		walk(*b.Intro)
	}
	for _, p := range b.Parts {
		for _, c := range p.Chapters {
			walk(c)
		}
	}
	return out
}

// copyAsset copies the project file rel into root’s src/ at the same path.
func copyAsset(project, root *os.Root, rel string) error {
	data, err := project.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		return err
	}
	genlog.Debug("book asset copied", "asset", rel)
	return writeFile(root, path.Join(srcDir, rel), data)
}

// writeFile writes body at the slash path rel inside root, creating its parents; a path leaving root fails.
func writeFile(root *os.Root, rel string, body []byte) error {
	p := filepath.FromSlash(rel)
	if err := root.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return root.WriteFile(p, body, 0o600)
}
