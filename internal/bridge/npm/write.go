// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

// Write serialises doc to package.json under dir. Behaviour:
//   - If doc.Rest is populated (set by Read or carried across Clone), known
//     keys from the typed struct are painted onto it; unknown keys keep their
//     position and source ordering.
//   - If doc.Rest is nil (the document was constructed from scratch, e.g. when
//     creating package.json for the first time), a fresh OrderedJSON is built
//     from the typed view alone. There is intentionally NO disk fallback: a
//     document built in memory must be sufficient to produce a valid file.
func Write(dir string, doc *Document) error {
	path := FullPath(dir)

	canvas := doc.Rest
	if canvas == nil {
		canvas = rawdoc.NewOrderedJSON()
	}

	known, err := typedFieldsAsJSON(doc)
	if err != nil {
		return fmt.Errorf("marshal package.json document: %w", err)
	}
	for _, k := range knownKeyOrder {
		raw, present := known[k]
		if !present {
			continue
		}
		canvas.Set(k, raw)
	}

	return writeOrderedJSON(path, canvas)
}

// WriteMerge is the legacy entry point; it now simply delegates to Write so
// callers (cmd/sync, sync_npm) keep working until they are collapsed onto the
// new driver-based core in step 9.
func WriteMerge(dir string, doc *Document) error {
	return Write(dir, doc)
}

// knownKeyOrder is the canonical insertion order applied for keys this package
// understands when no source ordering exists. Roughly follows npm's own
// convention (name/version/description first; dependencies last).
var knownKeyOrder = []string{
	npmKeyName,
	"version",
	npmKeyDescription,
	npmKeyKeywords,
	"homepage",
	"bugs",
	"license",
	"author",
	"contributors",
	"maintainers",
	"repository",
	"funding",
	"private",
	"os",
	"cpu",
	"engines",
	"dependencies",
	"devDependencies",
	"peerDependencies",
}

// typedFieldsAsJSON marshals the typed Document to JSON and re-splits it into
// the per-key raw view. Fields tagged omitempty that are absent in the typed
// struct simply don't appear in the resulting map — and so do not overwrite
// any existing key in the raw canvas.
//
// HTML escaping is disabled: package.json values like ">=24" must round-trip
// verbatim, not as ">=24". The stdlib default escapes `<`, `>`, `&` to
// keep JSON safe to inline inside <script> tags, which is irrelevant here.
func typedFieldsAsJSON(doc *Document) (map[string]json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeOrderedJSON(path string, om *rawdoc.OrderedJSON) error {
	data, err := om.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshal package.json: %w", err)
	}
	// Re-indent for readable output without losing key order.
	var indented bytes.Buffer
	if err := json.Indent(&indented, data, "", "  "); err != nil {
		return fmt.Errorf("indent package.json: %w", err)
	}
	indented.WriteByte('\n')
	return os.WriteFile(path, indented.Bytes(), 0o644) // #nosec G306 -- metadata file, not a secret
}
