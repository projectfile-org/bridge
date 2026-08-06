// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package stack

// IgnoredDirs are directory basenames pruned at WalkDir entry.
// Keeping the scanner blind to noise directories means a stray Cargo.toml
// inside node_modules cannot pollute the detected stack.
var IgnoredDirs = map[string]struct{}{
	".git":         {},
	".makefile":    {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"target":       {},
	".venv":        {},
	"__pycache__":  {},
	".next":        {},
	".astro":       {},
	".cache":       {},
	".gradle":      {},
	".mvn":         {},
}

// IsIgnored reports whether a directory basename should be skipped.
func IsIgnored(name string) bool {
	_, ok := IgnoredDirs[name]
	return ok
}
