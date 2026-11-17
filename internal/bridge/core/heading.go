// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"os"
	"path/filepath"
	"strings"
)

// ParseATXHeading parses one Markdown line as an ATX heading: its level (1–6) and trimmed text, or ok=false.
func ParseATXHeading(line string) (level int, text string, ok bool) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	// A space or tab must follow the marker (CommonMark 4.2).
	if n >= len(line) || (line[n] != ' ' && line[n] != '\t') {
		return 0, "", false
	}
	return n, strings.TrimSpace(line[n:]), true
}

// FirstHeading returns the text of the first ATX heading of any level in dir/rel, or "" on a read error or none.
func FirstHeading(dir, rel string) string {
	if dir == "" {
		dir = "."
	}
	body, err := os.ReadFile(filepath.Join(dir, rel)) // #nosec G304 -- rel is a repo-relative document path the caller enumerated
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if _, text, ok := ParseATXHeading(line); ok {
			return text
		}
	}
	return ""
}
