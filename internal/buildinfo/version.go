// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package buildinfo carries the release version injected at link time. One
// target for every pf-bridge binary's -ldflags -X, so the build script needs a
// single path regardless of how many binaries the split produces.
package buildinfo

// Version is overwritten at build time (see .scripts/build-binaries.sh).
var Version = "unknown"
