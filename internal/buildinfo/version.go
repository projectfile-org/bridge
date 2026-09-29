// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package buildinfo carries the release version injected at link time. One
// target for every pf-bridge binary's -ldflags -X, so the build script needs a
// single path regardless of how many binaries the split produces.
package buildinfo

import "runtime/debug"

// Version is overwritten at build time (see .scripts/build-binaries.sh).
var Version = "unknown"

// unstamped is the placeholder Version keeps when no -ldflags stamp and no Go build metadata exist.
const unstamped = "unknown"

func init() {
	if Version == unstamped {
		Version = fromBuildInfo(debug.ReadBuildInfo())
	}
}

// fromBuildInfo derives a version from the module version, else the VCS revision, else unstamped.
func fromBuildInfo(bi *debug.BuildInfo, ok bool) string {
	if !ok || bi == nil {
		return unstamped
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	revision, dirty := "", false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return unstamped
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if dirty {
		return revision + "-dirty"
	}
	return revision
}
