// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"strings"
)

// inheritedPrefixes are the per-language "Inherited from %s" heading prefixes
// a fragments-assembled document nests parent sections under. Registered by
// the fragments package from its own string catalog at init, so the table
// lives in one place and consumers (readme's parent-name summary) read it
// without importing the fragments bridge — importing that package registers
// its bridge and turns a single-file binary multi-file, which prompts.
var inheritedPrefixes []string

// RegisterInheritedPrefix enrolls one heading prefix for InheritedParent.
func RegisterInheritedPrefix(prefix string) {
	if prefix == "" {
		return
	}
	for _, p := range inheritedPrefixes {
		if p == prefix {
			return
		}
	}
	inheritedPrefixes = append(inheritedPrefixes, prefix)
}

// InheritedParent returns the parent name of an inherited-section heading.
func InheritedParent(heading string) string {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(heading), "##"))
	for _, prefix := range inheritedPrefixes {
		if strings.HasPrefix(text, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}
	return text
}
