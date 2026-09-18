// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package all aggregates every scanner driver so its sole import pulls all
// init() registrations into the build. Any binary that needs the full scanner
// registry (scaffold, scancmd) imports this package once instead of each
// driver individually — adding a new scanner is a single line here, not one
// per consumer. A consumer that also calls a driver's functions directly
// (scancmd calls stackscan.Scan/Diff/Union) still imports that driver
// non-blank in addition to this package.
package all

import (
	// Self-register with core during init(). The aggregator pattern means
	// adding a new scanner is one line here, not one per consumer.
	_ "projectfile.org/projectfile/bridge/internal/scanners/forge"
	_ "projectfile.org/projectfile/bridge/internal/scanners/git"
	_ "projectfile.org/projectfile/bridge/internal/scanners/stack"
)
