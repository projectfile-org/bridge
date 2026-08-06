// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package stack implements the filesystem-marker → stack-tag detector.
// Originally lived under internal/stackscan; moved into the scanners/
// registry so it composes with other init-time auto-discovery drivers via
// the shared core.Scanner interface.
package stack

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Hit records the first file that fired a given rule, plus the tags the rule
// contributes. Used to produce a human-readable per-marker summary.
type Hit struct {
	Rule string   // "file:go.mod" or "glob:**/*.cpp"
	Path string   // first file that triggered, relative to scan root
	Tags []string // tags the rule contributed
}

// Scan walks root, applies every rule in the embedded table, and returns the
// deduped sorted tag set plus one Hit per fired rule (in rule-table order).
// Directories listed in IgnoredDirs are pruned on entry; rule firings are
// "first match wins per rule" so Hit.Path is deterministic for a given tree.
func Scan(root string) ([]string, []Hit, error) {
	fileRules := map[string][]int{} // basename → rule indices
	extRules := map[string][]int{}  // ".go" → rule indices (lowercased)
	for i, r := range Rules {
		switch {
		case r.File != "":
			fileRules[r.File] = append(fileRules[r.File], i)
		case r.Glob != "":
			// v1 only supports the **/*.<ext> shape — extract the extension.
			ext := globExt(r.Glob)
			if ext == "" {
				return nil, nil, fmt.Errorf("stackscan: unsupported glob shape %q (only **/*.<ext> in v1)", r.Glob)
			}
			extRules[ext] = append(extRules[ext], i)
		case r.Dir != "":
			// Reserved for future expansion; v1 has no dir rules in the table.
		}
	}

	fired := make(map[int]Hit) // rule index → Hit (first match wins)

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if IsIgnored(name) {
				return fs.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		// File-name rules.
		for _, idx := range fileRules[name] {
			if _, seen := fired[idx]; seen {
				continue
			}
			fired[idx] = Hit{
				Rule: "file:" + Rules[idx].File,
				Path: rel,
				Tags: Rules[idx].Tags,
			}
		}
		// Extension rules.
		ext := strings.ToLower(filepath.Ext(name))
		if ext != "" {
			for _, idx := range extRules[ext] {
				if _, seen := fired[idx]; seen {
					continue
				}
				fired[idx] = Hit{
					Rule: "glob:" + Rules[idx].Glob,
					Path: rel,
					Tags: Rules[idx].Tags,
				}
			}
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", root, err)
	}

	// Emit hits in rule-table order so output is stable and reads naturally
	// (manifests first, then source globs).
	hits := make([]Hit, 0, len(fired))
	for i := range Rules {
		if h, ok := fired[i]; ok {
			hits = append(hits, h)
		}
	}

	tags := unionAll(hits)
	return tags, hits, nil
}

// Diff returns tags present in detected but missing from existing (set diff).
// Used to produce the "stack: +[...]" summary line.
func Diff(detected, existing []string) []string {
	have := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		have[t] = struct{}{}
	}
	var out []string
	for _, t := range detected {
		if _, ok := have[t]; !ok {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

// Union returns the deduped sorted union of two tag slices.
func Union(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for _, t := range a {
		seen[t] = struct{}{}
	}
	for _, t := range b {
		seen[t] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// globExt extracts ".ext" from a "**/*.<ext>" glob; returns "" for any other
// shape so the loader can flag unsupported patterns early.
func globExt(glob string) string {
	const prefix = "**/*."
	if !strings.HasPrefix(glob, prefix) {
		return ""
	}
	ext := strings.ToLower(glob[len(prefix)-1:]) // keep the leading dot
	if len(ext) < 2 || strings.ContainsAny(ext[1:], "*?/[]") {
		return ""
	}
	return ext
}

func unionAll(hits []Hit) []string {
	seen := map[string]struct{}{}
	for _, h := range hits {
		for _, t := range h.Tags {
			if t == "" {
				continue
			}
			seen[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
