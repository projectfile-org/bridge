// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Probe paths in the order GitHub itself searches. The driver uses these to
// resolve "where does this project keep its CODEOWNERS?" — first hit wins.
var probePaths = []string{
	Filename,
	".github/" + Filename,
	"docs/" + Filename,
}

// LocatedPath returns the first probe path that exists under dir, or the
// canonical root path when none exists (used by the "create" branch).
func LocatedPath(dir string) string {
	for _, rel := range probePaths {
		abs, err := absJoin(dir, rel)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	root, _ := absJoin(dir, Filename)
	return root
}

// ExistsIn reports whether any probe path under dir exists on disk.
func ExistsIn(dir string) bool {
	for _, rel := range probePaths {
		abs, err := absJoin(dir, rel)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return true
		}
	}
	return false
}

// Read parses the first existing CODEOWNERS file under dir. Returns an empty
// Document with no error when no file exists — callers gate creation on
// ExistsIn separately.
func Read(dir string) (*Document, error) {
	if !ExistsIn(dir) {
		return &Document{}, nil
	}
	path := LocatedPath(dir)
	f, err := os.Open(path) // #nosec G304 -- path resolved from caller-provided dir + known basenames
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	doc := &Document{}
	scanner := bufio.NewScanner(f)
	// Tolerate large pattern lines (rare, but CODEOWNERS allows long globs).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		doc.Lines = append(doc.Lines, parseLine(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return doc, nil
}

// parseLine classifies one raw line. The grammar (per GitHub docs):
//   - blank lines: ignored
//   - `# comment` lines: preserved verbatim
//   - `<pattern> <owner>+`: pattern is the first whitespace-separated token;
//     owners are the rest. Inline `# trailing` comments after the owners are
//     dropped on parse for v1 simplicity (round-trip would need an extra slot).
func parseLine(raw string) Line {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Line{Kind: KindBlank}
	}
	if strings.HasPrefix(trimmed, "#") {
		return Line{Kind: KindComment, Text: strings.TrimPrefix(trimmed, "#")}
	}
	// Strip inline trailing comment, then split on whitespace.
	if idx := strings.Index(trimmed, " #"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return Line{Kind: KindBlank}
	}
	pattern := fields[0]
	owners := append([]string(nil), fields[1:]...)
	return Line{Kind: KindEntry, Pattern: pattern, Owners: owners}
}

func absJoin(dir, rel string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(dir, rel))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
