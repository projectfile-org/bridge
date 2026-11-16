// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Write edits only the lines of the owned keys whose value changed, preserving every other byte.
func Write(dir string, doc *Document) error {
	data, err := render(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(FullPath(dir), data, 0o644) // #nosec G306 -- metadata file, not a secret
}

// render produces the manifest bytes for doc.
func render(doc *Document) ([]byte, error) {
	l, err := parseLayout(doc.Source)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", Filename, err)
	}
	var lines []string
	if len(doc.Source) > 0 {
		lines = strings.Split(strings.TrimSuffix(string(doc.Source), "\n"), "\n")
	}
	for _, key := range ownedKeys {
		table := l.table(key)
		if table == "" || !doc.Owns(key) {
			continue
		}
		have := l.get(table, key)
		want, same := encode(doc, key, have)
		if same {
			continue
		}
		genlog.Debug("Cargo.toml key rewritten", "table", table, "key", key, "value", want)
		lines = setKey(lines, table, key, want)
	}
	out := strings.Join(lines, "\n") + "\n"
	return core.GapFillREUSEHeader([]byte(out), doc.ReuseHeader), nil
}

// encode renders the typed value of key ("" for absent) and reports whether it equals have.
func encode(doc *Document, key string, have any) (string, bool) {
	if p := doc.stringField(key); p != nil {
		cur, _ := have.(string)
		if *p == "" {
			return "", have == nil
		}
		return quote(*p), have != nil && cur == *p
	}
	list := *doc.listField(key)
	if len(list) == 0 {
		return "", have == nil
	}
	return quoteList(list), have != nil && slices.Equal(stringsOf(have), list)
}
