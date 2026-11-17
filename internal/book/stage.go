// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// srcDir is the book.toml source root every staged book uses.
const srcDir = "src"

// linkTarget captures the target of a Markdown inline link or image, angle brackets and title excluded.
var linkTarget = regexp.MustCompile(`\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)

// Stage writes one book per language under out/<lang>/, reading collections rendered under rendered/<lang>/; it returns the languages staged.
func Stage(dir, rendered, out string, pf *projectfile.Document) ([]string, error) {
	var staged []string
	for _, lang := range append([]string{""}, pfmodel.Languages(pf)...) {
		concrete := core.ResolveLang(lang, pf)
		b, err := Plan(Input{Dir: dir, Rendered: filepath.Join(rendered, concrete)}, pf, lang)
		if err != nil {
			return staged, err
		}
		if err := stageOne(dir, filepath.Join(out, concrete), b, pf); err != nil {
			return staged, fmt.Errorf("stage %s book: %w", concrete, err)
		}
		genlog.Info("book staged", "lang", concrete, "dir", filepath.Join(out, concrete))
		staged = append(staged, concrete)
	}
	return staged, nil
}

// stageOne builds b in a temporary sibling of target and swaps it in, so a failed run leaves the previous book whole.
func stageOne(dir, target string, b Book, pf *projectfile.Document) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(target), ".book-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := writeBook(dir, tmp, b, pf); err != nil {
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

// writeBook writes book.toml, src/SUMMARY.md, every chapter and every local file the chapters link to under tmp.
func writeBook(dir, tmp string, b Book, pf *projectfile.Document) error {
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
	for _, c := range chapters {
		body, err := os.ReadFile(c.Source) // #nosec G304 -- Source is a page the planner found under the project or rendered tree
		if err != nil {
			return err
		}
		if c.Fallback {
			body = append([]byte(FallbackNote(b.Lang)), body...)
		}
		if err := writeFile(root, path.Join(srcDir, c.Path), body); err != nil {
			return err
		}
		if !strings.HasPrefix(c.Path, PagesDir+"/") {
			if err := copyAssets(project, root, c.Path, body); err != nil {
				return err
			}
		}
	}
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

// copyAssets copies each local non-Markdown file the page at rel links to from project into root’s src/, at the same path.
func copyAssets(project, root *os.Root, rel string, body []byte) error {
	for _, m := range linkTarget.FindAllSubmatch(body, -1) {
		target := string(m[1])
		if i := strings.IndexAny(target, "#?"); i >= 0 {
			target = target[:i]
		}
		if target == "" || strings.Contains(target, ":") || strings.EqualFold(path.Ext(target), ".md") {
			continue
		}
		asset := path.Clean(path.Join(path.Dir(rel), target))
		data, err := project.ReadFile(filepath.FromSlash(asset))
		if err != nil {
			genlog.Debug("book link target not copied", "page", rel, "target", target, "error", err.Error())
			continue
		}
		if err := writeFile(root, path.Join(srcDir, asset), data); err != nil {
			return err
		}
		genlog.Debug("book asset copied", "page", rel, "asset", asset)
	}
	return nil
}

// writeFile writes body at the slash path rel inside root, creating its parents; a path leaving root fails.
func writeFile(root *os.Root, rel string, body []byte) error {
	p := filepath.FromSlash(rel)
	if err := root.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return root.WriteFile(p, body, 0o600)
}
