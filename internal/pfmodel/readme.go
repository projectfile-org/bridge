// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// GetReadmeExtension parses `org.projectfile.readme`. Returns (nil, nil) when
// absent; the README bridge applies its default block list at render time.
func GetReadmeExtension(doc *projectfile.Document) (*ReadmeExtension, error) {
	m, present, err := lookupNS(doc, ReadmeExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &ReadmeExtension{
		Blocks: strListVal(m, "blocks"),
	}
	for _, name := range CommandSectionKeys {
		if groups := parseReadmeSection(m, name); len(groups) > 0 {
			if ext.Sections == nil {
				ext.Sections = map[string][]ReadmeSectionGroup{}
			}
			ext.Sections[name] = groups
		}
	}
	if items, ok := m["extras"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.Extras = append(ext.Extras, ReadmeExtra{
				Name:          strVal(em, "name"),
				Content:       extractLocalizedVal(em, "content"),
				ContentByLang: extractLocalizedMap(em, "content"),
			})
		}
	}
	if items, ok := m["shields"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.Shields = append(ext.Shields, Shield{
				Name: strVal(em, "name"),
				Img:  strVal(em, "img"),
				Href: strVal(em, "href"),
				Alt:  strVal(em, "alt"),
				Row:  strVal(em, "row"),
			})
		}
	}
	return ext, nil
}

// CommandSectionKeys are the block names that accept a structured command
// section under org.projectfile.readme. Declared here so the parser and the
// README bridge's template layer share one canonical list.
var CommandSectionKeys = []string{"installation", "quick-start", "usage", "building"}

// parseReadmeSection parses one command section — a LIST of groups — from the
// readme map. Returns nil when the key is absent, is not a list, or every entry
// is empty, so a bare or malformed declaration falls through to the block's
// file-probe fallback rather than rendering an empty section.
//
// Groups are deduplicated by `name`, LAST wins, first position kept — the same
// rule shields follow, and for the same reason: includes union sequences with
// the base document last (spec §4.9a), so redeclaring a name in the project's
// own projectfile REPLACES the inherited group (includes cannot delete), and
// keeping the position means an override never reorders the section.
//
// Unnamed groups never collide: each is kept as declared, since a fragment that
// bothers to name a group is the one asking to be overridable.
func parseReadmeSection(m map[string]any, key string) []ReadmeSectionGroup {
	items, ok := m[key].([]any)
	if !ok {
		return nil
	}
	var out []ReadmeSectionGroup
	position := make(map[string]int, len(items))
	for _, item := range items {
		sm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		group := ReadmeSectionGroup{
			Name:          strVal(sm, "name"),
			Prefix:        extractLocalizedVal(sm, "prefix"),
			PrefixByLang:  extractLocalizedMap(sm, "prefix"),
			Commands:      strListVal(sm, "commands"),
			Postfix:       extractLocalizedVal(sm, "postfix"),
			PostfixByLang: extractLocalizedMap(sm, "postfix"),
			Syntax:        strVal(sm, "syntax"),
		}
		if len(group.Commands) == 0 && group.Prefix == "" && group.Postfix == "" {
			continue
		}
		if at, seen := position[group.Name]; seen && group.Name != "" {
			out[at] = group
			continue
		}
		position[group.Name] = len(out)
		out = append(out, group)
	}
	return out
}

// extractLocalizedVal extracts a localized-string value from a map entry.
// The value may be a bare string or a map of lang→text; for maps it prefers
// "en", then the first non-empty value.
func extractLocalizedVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	langMap, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if s, _ := langMap["en"].(string); s != "" {
		return s
	}
	for _, val := range langMap {
		if s, _ := val.(string); s != "" {
			return s
		}
	}
	return ""
}

// extractLocalizedMap returns the raw lang→text map when the entry at key is
// a language map, or nil otherwise (bare string, missing, or wrong type).
// Language-aware consumers use it to preserve the full set of translations
// that extractLocalizedVal collapses to a single default.
func extractLocalizedMap(m map[string]any, key string) map[string]string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	langMap, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(langMap))
	for k, val := range langMap {
		if s, ok := val.(string); ok && s != "" {
			out[k] = s
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
