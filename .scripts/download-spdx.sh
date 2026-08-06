#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# =============================================================================
# download-spdx.sh — SPDX license-text downloader
# =============================================================================
#
# Downloads SPDX boilerplate texts from the SPDX license-list-data
# repository.  Every call unconditionally re-fetches every id.
#
# Refreshes the go:embed corpus the license bridge registers with core's spdx
# resolver (internal/bridge/license/register.go).
#
# REFRESH-ONLY — no CI node runs this.  The corpus is COMMITTED: on the forge
# plane every DAG node is its own job with its own checkout, and the corpus is
# needed in three of them (source-is-tested, binaries-built, image-is-built),
# which pf-ci cannot satisfy with one tool ("node=job cannot place it").
# Committing the texts serves all three with no mechanism at all, and keeps a
# network round-trip out of every build.  Run this by hand to pull an upstream
# refresh, then commit the diff.
# =============================================================================

embed_dir="internal/bridge/license/spdx"
base_url="https://raw.githubusercontent.com/spdx/license-list-data/main/text"

# BUSL-1.1 (Business Source License) is the canonical SPDX id; "BSL-1.1" 404s
# upstream. BSL-1.0 (Boost Software License) is a separate, unrelated licence.
ids="MIT Apache-2.0 GPL-3.0-or-later GPL-2.0-or-later LGPL-3.0-or-later     \
      BSD-2-Clause BSD-3-Clause MPL-2.0 ISC AGPL-3.0-or-later               \
      Unlicense CC0-1.0 BSL-1.0 EUPL-1.2 EPL-2.0                            \
      CDDL-1.0 Zlib OFL-1.1 BUSL-1.1 WTFPL"

log() { printf '[download-spdx] %s\n' "$*" >&2; }

mkdir -p "${embed_dir}"

for id in ${ids}; do
    log "fetch ${id}"
    curl --fail --silent --show-error --max-time 15     \
         --retry 3 --retry-delay 2 --retry-all-errors   \
         --output "${embed_dir}/${id}.txt" "${base_url}/${id}.txt"

    # A licence text cannot carry an inline SPDX header without corrupting the
    # text it is, so its tags ride a REUSE sidecar. Written HERE, next to the
    # fetch, so adding an id above can never leave an unannotated file behind
    # and break reuse-lint in a later, unrelated run. The tags describe what we
    # received: SPDX publishes license-list-data under CC0-1.0 and asserts no
    # copyright holder — the notices inside the GPL/Apache bodies are their
    # stewards’ and are not ours to re-declare.
    log "annotate ${id}.txt.license"
    printf 'SPDX-FileCopyrightText: NONE\n\nSPDX-License-Identifier: CC0-1.0\n' \
        > "${embed_dir}/${id}.txt.license"
done

log "refreshed $(find "${embed_dir}" -name '*.txt' | wc --lines) text(s) in ${embed_dir}; commit the diff"
