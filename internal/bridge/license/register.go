// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package license

import (
	"embed"
	"io/fs"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// `all:` mirrors the convention used in other bridges: the templates dir
// may grow files starting with `_` (shared partials) or `.` (dotfiles),
// and the default embed pattern silently skips those.
//
//go:embed all:templates
var templatesFS embed.FS

// The SPDX boilerplate corpus. It lives HERE, with its only consumer: this bridge
// is the sole caller of spdx.Text in the whole namespace. Core cannot carry it —
// core is a consumed library, so a go:embed asset there would have to be tracked in
// a repository that never reads it (and when it was untracked, core/v2@v2.0.0
// shipped an EMPTY set and broke every offline consumer).
//
// The texts are COMMITTED, refreshed out-of-band by `.scripts/download-spdx.sh`.
// Fetching them in CI instead would need the same files in three separate jobs
// (source-is-tested, binaries-built, image-is-built) — on the forge plane each node
// is its own job with its own checkout, and pf-ci refuses to place one tool in
// three ("node=job cannot place it"). Committing serves all three with no mechanism
// and keeps a network round-trip out of every build.
//
// `all:` guards against a future id whose name starts with `_` or `.`, which the
// default embed pattern would silently skip.
//
//go:embed all:spdx
var spdxFS embed.FS

func init() {
	core.RegisterTemplates("LICENSE.tmpl", func(n string) ([]byte, error) {
		return templatesFS.ReadFile("templates/" + n)
	})
	// Hand core the corpus rooted at the ids themselves: spdx.Text reads
	// `<id>.txt` at the FS root, so the `spdx/` prefix is stripped here rather
	// than leaked into core's lookup path. A Sub failure would mean the embed
	// directive above is broken, which is a build-time fact, not a runtime one —
	// so degrade to no corpus (cache/network still serve) and say so loudly.
	corpus, err := fs.Sub(spdxFS, "spdx")
	if err != nil {
		genlog.Warn("spdx corpus unavailable, falling back to cache/network", "err", err)
	} else {
		spdx.SetEmbedded(corpus)
	}
	bridge.Register(Bridge{})
}
