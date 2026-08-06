// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

const composerKeywords = "keywords"

// Write serialises doc to composer.json under dir. Behaviour mirrors npm:
//   - If doc.Rest is populated, known keys from the typed struct are painted
//     onto it; unknown keys keep their position and source ordering.
//   - If doc.Rest is nil (document constructed from scratch), a fresh
//     OrderedJSON is built from the typed view alone.
func Write(dir string, doc *Document) error {
	path := FullPath(dir)

	canvas := doc.Rest
	if canvas == nil {
		canvas = rawdoc.NewOrderedJSON()
	}

	known, err := typedFieldsAsJSON(doc)
	if err != nil {
		return fmt.Errorf("marshal composer.json document: %w", err)
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

// knownKeyOrder is the canonical insertion order applied when there is no
// source ordering. Follows composer's own convention (identity first,
// support metadata, then requires).
var knownKeyOrder = []string{
	"name",
	"description",
	"version",
	"type",
	composerKeywords,
	"homepage",
	"readme",
	"time",
	"license",
	"authors",
	"support",
	"funding",
	"require",
	"require-dev",
	"minimum-stability",
	"prefer-stable",
	"abandoned",
}

// typedFieldsAsJSON marshals the typed Document to JSON and re-splits it
// into per-key raw view. omitempty fields absent in the typed struct simply
// don't appear in the resulting map, so they don't overwrite the canvas.
//
// HTML escaping is disabled so dependency constraints like ">=1.0" survive
// verbatim rather than as ">=1.0".
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
		return fmt.Errorf("marshal composer.json: %w", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, data, "", "    "); err != nil {
		return fmt.Errorf("indent composer.json: %w", err)
	}
	indented.WriteByte('\n')
	return os.WriteFile(path, indented.Bytes(), 0o644) // #nosec G306 -- metadata file, not a secret
}
