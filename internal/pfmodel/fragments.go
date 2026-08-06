// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// GetFragmentsExtension parses `org.projectfile.fragments` — the explicit
// override namespace. When present with documents, the renderer uses it
// verbatim and ignores the conventions-driven defaults. Documents are
// order-significant for output determinism. Returns (nil, nil) when absent.
func GetFragmentsExtension(doc *projectfile.Document) (*FragmentsExtension, error) {
	m, present, err := lookupNS(doc, FragmentsExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &FragmentsExtension{}
	for _, d := range docsAny(m["documents"]) {
		ext.Documents = append(ext.Documents, FragmentDocument{
			Dir:     strVal(d, "dir"),
			Out:     strVal(d, "out"),
			Title:   strVal(d, "title"),
			Parents: parentsVal(d, "parents"),
		})
	}
	return ext, nil
}

// ConventionsFragments is the conventions-driven default config under
// `org.projectfile.conventions.fragments`. Shells reach every consumer via
// the b19 conventions include; Parents is a flat per-project inheritance
// list shared by every shell. The projectfile include merger deep-merges
// the conventions namespace, so Documents (from the include) and Parents
// (from the project) coexist without one replacing the other.
type ConventionsFragments struct {
	Documents []FragmentDocument // shells (dir/out/title) from the include
	Parents   []FragmentParent   // flat per-project inheritance list
}

// GetConventionsFragments reads `org.projectfile.conventions.fragments`.
// Returns (nil, nil) when the namespace or the fragments sub-key is absent.
func GetConventionsFragments(doc *projectfile.Document) (*ConventionsFragments, error) {
	m, present, err := lookupNS(doc, ConventionsExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	frag, ok := m["fragments"].(map[string]any)
	if !ok {
		return nil, nil
	}
	out := &ConventionsFragments{Parents: parentsVal(frag, "parents")}
	for _, d := range docsAny(frag["documents"]) {
		out.Documents = append(out.Documents, FragmentDocument{
			Dir:   strVal(d, "dir"),
			Out:   strVal(d, "out"),
			Title: strVal(d, "title"),
		})
	}
	return out, nil
}

// parentsVal narrows a parsed parents value to typed entries. A parent with no
// url carries no resolvable identity, so it is dropped rather than kept as an
// empty row the fetcher would have to guard against on every call.
func parentsVal(m map[string]any, key string) []FragmentParent {
	var out []FragmentParent
	for _, p := range docsAny(m[key]) {
		if url := strVal(p, "url"); url != "" {
			out = append(out, FragmentParent{URL: url, Ref: strVal(p, "ref")})
		}
	}
	return out
}

// docsAny narrows a parsed documents value to a slice of string-keyed maps,
// tolerating the []any parse-time shape. Returns nil for anything else.
func docsAny(v any) []map[string]any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if dm, ok := item.(map[string]any); ok {
			out = append(out, dm)
		}
	}
	return out
}
