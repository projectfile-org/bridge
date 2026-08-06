// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

var Filename = "package.json"

// repoTypeGit is the VCS type tag emitted for every repository entry — npm
// shortcuts (github:, gitlab:, gist:) and direct git URLs all resolve to git.
const repoTypeGit = "git"

func FullPath(dir string) string {
	abs, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return filepath.Join(dir, Filename)
	}
	return filepath.Clean(abs)
}

func Exists(dir string) bool {
	_, err := os.Stat(FullPath(dir))
	return err == nil
}

// Read parses package.json into the typed Document AND captures the full key
// order + unknown sub-trees into Document.Rest. Both views are populated for
// every successful read, so Write can paint the typed fields onto the raw
// canvas without losing unknown keys (scripts, volta, workspaces, etc.).
func Read(dir string) (*Document, error) {
	path := FullPath(dir)
	data, err := os.ReadFile(path) // #nosec G304 -- known filename under user-provided dir
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	doc := &Document{}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	rest := rawdoc.NewOrderedJSON()
	if err := json.Unmarshal(data, rest); err != nil {
		return nil, fmt.Errorf("parse %s raw view: %w", path, err)
	}
	doc.Rest = rest

	return doc, nil
}

// ParsePerson and friends remain in this file for backwards-compatible imports.

// ParsePerson normalises the three shapes a person can take inside a
// package.json value to the typed npm.Person view. The struct case is what
// makes in-process round-trips idempotent: FromPF writes typed Person values
// into the same Document a subsequent ToPF reads in the same Sync() call —
// without this case, the type-switch fell through to `default` and produced
// an empty Person which then leaked back into projectfile.People as a
// canonical-name-less entity that could never dedup.
func ParsePerson(v any) Person {
	switch val := v.(type) {
	case string:
		return parsePersonString(val)
	case Person:
		return val
	case *Person:
		if val == nil {
			return Person{}
		}
		return *val
	case map[string]any:
		p := Person{}
		if s, ok := val["name"].(string); ok {
			p.Name = s
		}
		if s, ok := val["email"].(string); ok {
			p.Email = s
		}
		if s, ok := val["url"].(string); ok {
			p.URL = s
		}
		return p
	default:
		return Person{}
	}
}

// IsEmpty reports whether a parsed person carries no identifying information.
// Callers use this to filter `{}` placeholder entries out of contributors /
// maintainers arrays before they reach projectfile.MergePeople on pf.People,
// which would otherwise append each empty as a fresh non-matching record on
// every cycle.
func (p Person) IsEmpty() bool {
	return p.Name == "" && p.Email == "" && p.URL == ""
}

func ParsePersonList(v []any) []Person {
	out := make([]Person, 0, len(v))
	for _, item := range v {
		out = append(out, ParsePerson(item))
	}
	return out
}

func ParseRepository(v any) Repository {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "github:") {
			return Repository{URL: "https://github.com/" + strings.TrimPrefix(val, "github:"), Type: repoTypeGit}
		}
		if strings.HasPrefix(val, "gitlab:") {
			return Repository{URL: "https://gitlab.com/" + strings.TrimPrefix(val, "gitlab:"), Type: repoTypeGit}
		}
		if strings.HasPrefix(val, "gist:") {
			return Repository{URL: "https://gist.github.com/" + strings.TrimPrefix(val, "gist:"), Type: repoTypeGit}
		}
		if strings.HasPrefix(val, "https://") || strings.HasPrefix(val, "git+") || strings.HasPrefix(val, "git://") {
			url := strings.TrimPrefix(val, "git+")
			url = strings.TrimPrefix(url, "git://")
			if !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}
			return Repository{URL: url, Type: repoTypeGit}
		}
		return Repository{URL: val, Type: repoTypeGit}
	case map[string]any:
		r := Repository{}
		if s, ok := val["url"].(string); ok {
			r.URL = s
		}
		if s, ok := val["type"].(string); ok {
			r.Type = s
		}
		if s, ok := val["directory"].(string); ok {
			r.Directory = s
		}
		return r
	default:
		return Repository{}
	}
}

func ParseBugs(v any) Bugs {
	switch val := v.(type) {
	case string:
		return Bugs{URL: val}
	case map[string]any:
		b := Bugs{}
		if s, ok := val["url"].(string); ok {
			b.URL = s
		}
		if s, ok := val["email"].(string); ok {
			b.Email = s
		}
		return b
	default:
		return Bugs{}
	}
}

func ParseFunding(v any) []FundingEntry {
	switch val := v.(type) {
	case string:
		return []FundingEntry{{URL: val}}
	case map[string]any:
		f := FundingEntry{}
		if s, ok := val["url"].(string); ok {
			f.URL = s
		}
		if s, ok := val["type"].(string); ok {
			f.Type = s
		}
		return []FundingEntry{f}
	case []any:
		out := make([]FundingEntry, 0, len(val))
		for _, item := range val {
			out = append(out, ParseFunding(item)...)
		}
		return out
	default:
		return nil
	}
}

func parsePersonString(s string) Person {
	p := Person{}
	rest := s

	if idx := strings.Index(rest, "<"); idx >= 0 && strings.Contains(rest[idx:], ">") {
		p.Email = strings.TrimSpace(rest[idx+1 : strings.Index(rest[idx:], ">")+idx])
		rest = strings.TrimSpace(rest[:idx])
	}

	if idx := strings.Index(rest, "("); idx >= 0 && strings.HasSuffix(rest, ")") {
		p.URL = strings.TrimSpace(rest[idx+1 : len(rest)-1])
		rest = strings.TrimSpace(rest[:idx])
	}

	p.Name = strings.TrimSpace(rest)
	return p
}
