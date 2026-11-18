// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/derive/forges"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// readmeStem matches the file stem mdBook renders as index.html without rewriting links to it.
var readmeStem = regexp.MustCompile(`(?i)^readme$`)

// site maps the project source of every staged page to its book path, per language, so a link resolves into whichever book carries it.
type site struct {
	order   []string
	origins map[string]map[string]string
	paths   map[string]map[string]bool
	forge   func(rel string) string
}

// linkStats counts each link decision of one book.
type linkStats struct {
	kept, crossBook, forge, asset, unresolved int
}

// newSite indexes books, the default language first, and the public forge their unstaged targets link to.
func newSite(books []Book, pf *projectfile.Document) *site {
	var kinds map[string]string
	if ext, err := pfmodel.GetForgeExtension(pf); err == nil && ext != nil {
		kinds = ext.Kinds
	}
	s := &site{origins: map[string]map[string]string{}, paths: map[string]map[string]bool{}, forge: forges.FileURL(pf, kinds)}
	for _, b := range books {
		s.order = append(s.order, b.Lang)
		s.origins[b.Lang] = map[string]string{}
		s.paths[b.Lang] = map[string]bool{}
		for _, c := range b.chapters() {
			s.paths[b.Lang][c.Path] = true
			if c.Origin != "" {
				s.origins[b.Lang][c.Origin] = c.Path
			}
		}
	}
	return s
}

// rewrite points every relative link of c’s body, outside fenced code, at its page in this or another book, a copied asset, or the forge; it returns the assets to copy.
func (s *site) rewrite(project *os.Root, lang string, c Chapter, body []byte, stats *linkStats) ([]byte, []string) {
	var assets []string
	out := core.RewriteLinks(body, func(raw string) string {
		target, asset := s.resolve(project, lang, c, raw, stats)
		if asset != "" {
			assets = append(assets, asset)
		}
		return target
	})
	return out, assets
}

// resolve rewrites one link target of c; the second result is the project path of an asset to copy into the book.
func (s *site) resolve(project *os.Root, lang string, c Chapter, raw string, stats *linkStats) (string, string) {
	if !core.IsRelativeLink(raw) {
		return raw, ""
	}
	target, suffix := core.SplitLinkSuffix(raw)
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	resolved := path.Clean(path.Join(path.Dir(c.Origin), target))
	if !filepath.IsLocal(filepath.FromSlash(resolved)) {
		stats.unresolved++
		genlog.Warn("book link leaves the project, left as is", "page", c.Origin, "target", raw)
		return raw, ""
	}
	if destLang, destPath, ok := s.find(lang, c, resolved); ok {
		if readmeStem.MatchString(strings.TrimSuffix(path.Base(destPath), path.Ext(destPath))) && strings.EqualFold(path.Ext(destPath), ".md") {
			destPath = path.Join(path.Dir(destPath), indexPage)
		}
		if destLang == lang {
			stats.kept++
		} else {
			stats.crossBook++
		}
		rel := sitePath(path.Join(lang, path.Dir(c.Path)), path.Join(destLang, destPath))
		genlog.Debug("book link resolved to a page", "page", c.Origin, "target", raw, "lang", destLang, "path", rel)
		return escapeSpaces(rel) + suffix, ""
	}
	info, err := project.Stat(filepath.FromSlash(resolved))
	if err != nil {
		stats.unresolved++
		genlog.Warn("book link target is missing, left as is", "page", c.Origin, "target", raw)
		return raw, ""
	}
	if !info.IsDir() && isMedia(resolved) {
		stats.asset++
		genlog.Debug("book link resolved to a copied asset", "page", c.Origin, "asset", resolved)
		return escapeSpaces(sitePath(path.Dir(c.Path), resolved)) + suffix, resolved
	}
	if u := s.forge(resolved); u != "" {
		stats.forge++
		genlog.Debug("book link resolved to the forge", "page", c.Origin, "target", raw, "url", u)
		return u + suffix, ""
	}
	stats.unresolved++
	genlog.Warn("book link target is in no book and no public forge is declared, left as is", "page", c.Origin, "target", raw)
	return raw, ""
}

// find locates the project path resolved in a book: this one by source, this one by book path when c stands in untranslated, then every other book in order.
func (s *site) find(lang string, c Chapter, resolved string) (string, string, bool) {
	if p, ok := s.origins[lang][resolved]; ok {
		return lang, p, true
	}
	if c.Fallback && s.paths[lang][resolved] {
		return lang, resolved, true
	}
	for _, other := range s.order {
		if p, ok := s.origins[other][resolved]; ok && other != lang {
			return other, p, true
		}
	}
	return "", "", false
}

// sitePath is the slash path from the directory from to the file to, both relative to one root.
func sitePath(from, to string) string {
	rel, err := filepath.Rel(filepath.FromSlash(from), filepath.FromSlash(to))
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}

// escapeSpaces keeps a rewritten target one Markdown token.
func escapeSpaces(p string) string {
	return strings.ReplaceAll(p, " ", "%20")
}

// isMedia reports whether rel is an image, audio or video file, which the book serves itself.
func isMedia(rel string) bool {
	t := mime.TypeByExtension(path.Ext(rel))
	return strings.HasPrefix(t, "image/") || strings.HasPrefix(t, "audio/") || strings.HasPrefix(t, "video/")
}
