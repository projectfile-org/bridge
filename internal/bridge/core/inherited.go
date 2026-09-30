// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"strings"
)

// InheritedFormats are the per-language "Inherited from %s" headings a fragments-assembled document nests parent sections under.
var InheritedFormats = map[string]string{
	"en": "Inherited from %s",
	"es": "Heredado de %s",
	"uk": "Успадковано від %s",
}

// InheritedParent returns the parent name of an inherited-section heading.
func InheritedParent(heading string) string {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(heading), "##"))
	for _, format := range InheritedFormats {
		if prefix, _, _ := strings.Cut(format, "%"); strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	return text
}
