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
//
// optIn marks a target that stays silent until its sub-namespace is declared,
// so it never appears in a project that did not ask for it. The five original
// targets are universal and predate the flag.
var targets = []struct {
	filename string
	extKey   string
	optIn    bool
}{
	{filename: filenameGitignore, extKey: extKeyGit},
	{filename: ".dockerignore", extKey: extKeyDocker},
	{filename: ".containerignore", extKey: extKeyContainer},
	{filename: ".npmignore", extKey: extKeyNPM},
	{filename: ".claudeignore", extKey: extKeyClaude},
	// The prose pair. auto-textlint discovers files with `fd`, passing them as
	// explicit arguments, so .textlintignore alone cannot spare a corpus —
	// .fdignore is the load-bearing half and the two must agree.
	{filename: ".textlintignore", extKey: extKeyTextlint, optIn: true},
	{filename: ".fdignore", extKey: extKeyFd, optIn: true},
}

func init() {
	for _, t := range targets {
		bridge.Register(Bridge{filename: t.filename, extKey: t.extKey, optIn: t.optIn})
	}
}
