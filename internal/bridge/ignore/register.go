// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package ignore

import (
	"projectfile.org/projectfile/bridge/internal/bridge"
)

// targets is the single source of truth for which ignore files this
// package generates. Adding a new ignore target is one row here plus a
// new typed slot on IgnoresExtension; nothing else.
//
// filename is the canonical on-disk name.
// extKey is the [org.projectfile.ignores.<extKey>] sub-namespace pointer
// surfaced in the banner so users know where to add include/exclude.
//
// NOTE: .trivyignore is NOT here. Suppressed vulnerability IDs are tool-agnostic
// and live in the org.projectfile.vulnerabilities namespace, fanned out to
// .trivyignore / .grype.yaml / osv-scanner.toml by bridge/vulnerabilities.
var targets = []struct {
	filename string
	extKey   string
}{
	{filename: filenameGitignore, extKey: extKeyGit},
	{filename: ".dockerignore", extKey: "docker"},
	{filename: ".containerignore", extKey: "container"},
	{filename: ".npmignore", extKey: extKeyNPM},
	{filename: ".claudeignore", extKey: extKeyClaude},
}

func init() {
	for _, t := range targets {
		bridge.Register(Bridge{filename: t.filename, extKey: t.extKey})
	}
}
