// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"fmt"
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// PageCollection is one `org.projectfile.pages.<name>` entry: a named set of pages the project’s executable renders.
type PageCollection struct {
	Name    string
	Title   *projectfile.LocalizedString
	Command []string
}

// BookPart is one `org.projectfile.conventions.book.parts[]` entry: a titled part listing page paths or one-level globs.
type BookPart struct {
	Title       *projectfile.LocalizedString
	Pages       []string
	Collections bool
}

// GetPageCollections reads `org.projectfile.pages`, sorted by name; (nil, nil) when absent.
func GetPageCollections(doc *projectfile.Document) ([]PageCollection, error) {
	m, present, err := lookupNS(doc, PagesExtensionNS)
	if err != nil || !present {
		return nil, err
	}
	out := make([]PageCollection, 0, len(m))
	for name, raw := range m {
		c, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s.%s is not a map", PagesExtensionNS, name)
		}
		out = append(out, PageCollection{Name: name, Title: localizedStringVal(c, "title"), Command: strListVal(c, "command")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// GetBookParts reads `org.projectfile.conventions.book.parts` in declaration order; nil when absent.
func GetBookParts(doc *projectfile.Document) ([]BookPart, error) {
	m, present, err := lookupNS(doc, ConventionsExtensionNS)
	if err != nil || !present {
		return nil, err
	}
	book, ok := m["book"].(map[string]any)
	if !ok {
		return nil, nil
	}
	var out []BookPart
	for _, p := range docsAny(book["parts"]) {
		out = append(out, BookPart{Title: localizedStringVal(p, "title"), Pages: strListVal(p, "pages"), Collections: boolVal(p, "collections")})
	}
	return out, nil
}
