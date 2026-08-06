#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

  set -euo pipefail

  # shellcheck source=/dev/null
  . b19-i18n

  LDFLAGS="-s -w -X projectfile.org/projectfile/bridge/internal/buildinfo.Version=${M6E_VERSION:-dev}"

  b19-run "BRIDGE" "$(_ 'Download modules')" --     \
    go mod download

  b19-run "BRIDGE" "$(_ 'Prepare export dir')" --     \
    mkdir -p /export/usr/local/bin

  # Build one binary (dispatcher or bridge) and export it stripped.
  build_one() {
    local name="$1" pkg="$2"

    b19-run "BRIDGE" "$(_p 'Build %s' "${name}")" --      \
      go build -ldflags="${LDFLAGS}" -o "${name}" "${pkg}"

    b19-strip "BRIDGE" "${name}"

    b19-run "BRIDGE" "$(_p 'Copy %s to %s' "${name}" "/export/usr/local/bin")" --     \
      cp "${name}" /export/usr/local/bin
  }

  # The pf-bridge dispatcher (module root) + one binary per cmd/pf-bridge-*.
  build_one pf-bridge .
  for dir in cmd/pf-bridge-*/; do
    build_one "$(basename "${dir}")" "./${dir}"
  done
