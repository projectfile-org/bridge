// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package license

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// stat is a thin indirection for Bridge.Exists so tests can be added later
// without rewriting the Bridge surface.
func stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// declaredFiles returns the normalised, validated path list from
// `license.file` (string-or-sequence per spec §4.4). Returns (nil, nil) when
// the field is absent so callers fall through to the synthesised default
// names. Every entry is checked against the spec §8 path rules; a violation
// is a hard error (we refuse to write outside the project tree).
func declaredFiles(pf *projectfile.Document) ([]string, error) {
	if pf == nil || pf.License == nil {
		return nil, nil
	}
	paths := projectfile.AsStringList(pf.License.File)
	if len(paths) == 0 {
		return nil, nil
	}
	for _, p := range paths {
		if err := validateRelPath(p); err != nil {
			return nil, fmt.Errorf("license.file %q: %w", p, err)
		}
	}
	return paths, nil
}

// validateRelPath enforces the spec §8 path-traversal rules on a path-valued
// field: reject absolute paths, Windows drive letters / UNC prefixes, and any
// path that escapes the project root after normalisation (`..` segments). The
// path must resolve to a location inside the project directory.
func validateRelPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return fmt.Errorf("absolute paths are not allowed")
	}
	// Windows drive letter (C:\) or UNC / extended-length prefixes (\\?\, \\).
	if len(p) >= 2 && p[1] == ':' {
		return fmt.Errorf("windows drive letters are not allowed")
	}
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return fmt.Errorf("UNC / absolute paths are not allowed")
	}
	// Normalise with forward slashes, then confirm no `..` segment crosses the
	// root. filepath.Clean collapses interior `..`; a leading `..` after clean
	// means the path escapes the project directory.
	clean := filepath.ToSlash(filepath.Clean(p))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path escapes the project root via '..'")
	}
	return nil
}
