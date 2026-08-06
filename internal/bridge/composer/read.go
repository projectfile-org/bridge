// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

var Filename = "composer.json"

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

// Read parses composer.json into the typed Document AND captures the full
// key order + unknown sub-trees into Document.Rest. Both views are populated
// for every successful read, so Write can paint the typed fields onto the
// raw canvas without losing unknown keys.
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

// LicenseString flattens the heterogeneous composer license value to a
// single SPDX expression. Composer accepts:
//
//	"MIT"                              (single SPDX id)
//	["LGPL-2.1-only", "GPL-3.0"]       (array — OR semantics per the spec)
//
// Multiple entries collapse with " OR " so the result is a valid SPDX
// expression that round-trips back through ParseLicense.
func LicenseString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case []string:
		return strings.Join(x, " OR ")
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " OR ")
	}
	return ""
}

// ParseLicense is the inverse: a SPDX expression containing " OR " becomes
// an array; otherwise a single string. Used when emitting license back into
// composer.json so we don't blow up array-shaped originals into single
// strings unnecessarily.
func ParseLicense(spdx string) any {
	spdx = strings.TrimSpace(spdx)
	if spdx == "" {
		return nil
	}
	if !strings.Contains(spdx, " OR ") {
		return spdx
	}
	parts := strings.Split(spdx, " OR ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 1 {
		return out[0]
	}
	return out
}
