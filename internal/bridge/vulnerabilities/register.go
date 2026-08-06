// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package vulnerabilities

import (
	"projectfile.org/projectfile/bridge/internal/bridge"
)

// targets is the single source of truth for which scanner ignore files this
// package generates from org.projectfile.vulnerabilities. Adding a scanner is
// one row here plus a render arm in bridge.go; nothing else.
//
// scanner is the extKey matched against `generate` (opt-out list) and selects
// the render format. filename is the canonical on-disk name grype/osv
// auto-discover from the cwd (and trivy is fed via --ignorefile by auto-trivy).
var targets = []struct {
	scanner  string
	filename string
}{
	{scanner: scannerTrivy, filename: ".trivyignore"},
	{scanner: scannerGrype, filename: ".grype.yaml"},
	{scanner: scannerOSV, filename: "osv-scanner.toml"},
}

func init() {
	for _, t := range targets {
		bridge.Register(Bridge{scanner: t.scanner, filename: t.filename})
	}
}
