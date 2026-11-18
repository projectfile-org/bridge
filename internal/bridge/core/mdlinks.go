// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"regexp"
	"strings"
)

// mdLinkTarget captures the target of a Markdown inline link or image: an angle-bracketed one in group 1, a bare one in group 2, title excluded.
var mdLinkTarget = regexp.MustCompile(`\]\(\s*(?:<([^>\n]+)>|([^)\s]+))(?:\s+"[^"]*")?\s*\)`)

// hasScheme matches a target carrying a URL scheme (https:, mailto:).
var hasScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// RewriteLinks replaces the target of every inline Markdown link and image outside fenced code with fn(target).
func RewriteLinks(body []byte, fn func(target string) string) []byte {
	var out bytes.Buffer
	fence := ""
	for _, line := range bytes.SplitAfter(body, []byte("\n")) {
		trimmed := strings.TrimLeft(string(line), " \t")
		if marker := fenceMarker(trimmed); marker != "" && (fence == "" || strings.HasPrefix(trimmed, fence)) {
			if fence == "" {
				fence = marker
			} else {
				fence = ""
			}
			out.Write(line)
			continue
		}
		if fence != "" {
			out.Write(line)
			continue
		}
		last := 0
		for _, m := range mdLinkTarget.FindAllSubmatchIndex(line, -1) {
			start, end := m[2], m[3]
			if start < 0 {
				start, end = m[4], m[5]
			}
			out.Write(line[last:start])
			out.WriteString(fn(string(line[start:end])))
			last = end
		}
		out.Write(line[last:])
	}
	return out.Bytes()
}

// fenceMarker returns the ``` or ~~~ run a line opens or closes a fenced block with, else "".
func fenceMarker(line string) string {
	for _, ch := range []string{"`", "~"} {
		if strings.HasPrefix(line, ch+ch+ch) {
			return ch + ch + ch
		}
	}
	return ""
}

// IsRelativeLink reports whether target is a path relative to its document: no scheme, no leading slash, not a bare anchor.
func IsRelativeLink(target string) bool {
	return target != "" && !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "/") && !hasScheme.MatchString(target)
}

// SplitLinkSuffix splits a link target into its path and its #fragment or ?query suffix.
func SplitLinkSuffix(target string) (string, string) {
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		return target[:i], target[i:]
	}
	return target, ""
}
