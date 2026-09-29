// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// Keys shared by the upstream reader and the fixtures that exercise it.
const (
	keyName = "name"
	keyURL  = "url"
)

// upstreamEntry is one spec §4.4b upstream as the README states it.
type upstreamEntry struct {
	Name    string
	URL     string
	Version string
	License string
	Fork    bool
}

// Ref renders the upstream’s name, linked when it declares a URL, followed by its version.
func (e upstreamEntry) Ref() string {
	ref := e.Name
	if e.URL != "" {
		ref = "[" + e.Name + "](" + e.URL + ")"
	}
	if e.Version != "" {
		ref += " " + e.Version
	}
	return ref
}

// buildUpstream reads the top-level upstream list; an entry with no name is dropped, an unresolved version or URL is omitted.
func buildUpstream(doc *projectfile.Document, lang string) []upstreamEntry {
	if doc == nil {
		return nil
	}
	items, _ := doc.Rest["upstream"].([]any)
	out := make([]upstreamEntry, 0, len(items))
	for _, item := range items {
		m, _ := item.(map[string]any)
		e := upstreamEntry{
			Name:    projectfile.ExtractLocalizedStringForLang(localizedOf(m[keyName]), lang),
			URL:     expandProse(doc, stringOf(m[keyURL]), "upstream.url"),
			Version: expandProse(doc, stringOf(m["version"]), "upstream.version"),
			License: stringOf(m["license"]),
			Fork:    stringOf(m["relation"]) == "fork",
		}
		if e.Name == "" {
			genlog.DebugRow("readme_upstream", "", "upstream[]", "no name (dropped)")
			continue
		}
		genlog.DebugRow("readme_upstream", e.Name, "upstream[]", "version="+e.Version+" license="+e.License)
		out = append(out, e)
	}
	return out
}

// localizedOf reads a localized-string value in either of its encodings, a bare string or a language map.
func localizedOf(v any) *projectfile.LocalizedString {
	switch t := v.(type) {
	case string:
		return &projectfile.LocalizedString{Bare: t}
	case map[string]any:
		langs := make(map[string]string, len(t))
		for k, s := range t {
			langs[k] = stringOf(s)
		}
		return &projectfile.LocalizedString{Langs: langs}
	}
	return nil
}

// stringOf is v when it is a string, else empty.
func stringOf(v any) string {
	s, _ := v.(string)
	return s
}
