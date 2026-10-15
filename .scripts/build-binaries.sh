#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# build-binaries.sh — cross-compile the standalone pf-bridge for one GOOS/GOARCH.

version="${1:-${GITHUB_REF_NAME:-}}"
case "${version}" in
	'' | *['{}']*) version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)" ;; # empty or an unexpanded template placeholder
esac
ldflags="-s -w -X projectfile.org/projectfile/bridge/v4/internal/buildinfo.Version=${version}"

log() { printf '[build-binaries] %s\n' "$*" >&2; }

mkdir -p dist

hostos="$(go env GOHOSTOS)"   # the real host even under a cross-compile
hostarch="$(go env GOHOSTARCH)"
goos="${GOOS:-${hostos}}"     # the matrix sets these per cell; unset means a host-native build
goarch="${GOARCH:-${hostarch}}"

build() { # $1 output path   $2 package path   $3 extra go build flags (may be empty)
    log "building $1 ${version} for ${goos}/${goarch} flags=${3:-<none>}"
    # shellcheck disable=SC2086 # $3 is a word list of go build flags
    GOOS="${goos}" GOARCH="${goarch}" go build ${3:-} -ldflags="${ldflags}" -o "$1" "$2"
}

# The standalone dispatcher with every bridge linked in: the asset an install fetches
build "$(GOOS="${goos}" GOARCH="${goarch}" .makefile/core/scripts/asset-name.sh dist/pf-bridge)" .

# The single-bridge binaries, only when PF_BRIDGE_SINGLES names a directory (local install)
if [ -z "${PF_BRIDGE_SINGLES:-}" ]; then
    log "PF_BRIDGE_SINGLES unset, built the standalone pf-bridge only for ${goos}/${goarch}"
    exit 0
fi
mkdir -p "${PF_BRIDGE_SINGLES}"
for d in cmd/pf-bridge-*/; do
    build "${PF_BRIDGE_SINGLES}/$(basename "${d}")" "./${d}"
done
