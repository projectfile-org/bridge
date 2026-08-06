// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package yamllint

import (
	"projectfile.org/projectfile/bridge/internal/bridge"
)

// targets is the single source of truth for which files this package
// generates. One row: yamllint consumes a single `.yamllint` config per
// project (auto-detected by auto-yamllint from the cwd).
//
// The ignore patterns come from the SAME namespace as the flat-file ignores
// (org.projectfile.ignores.yamllint.include) but are rendered as an
// `ignore:` block inside the YAML config rather than one pattern per line —
// that is why yamllint is its own package, not a row in bridge/ignore.
var targets = []struct {
	filename string
}{
	{filename: filenameYamllint},
}

func init() {
	for _, t := range targets {
		bridge.Register(Bridge{filename: t.filename})
	}
}
