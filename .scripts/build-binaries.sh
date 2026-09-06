#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# build-binaries.sh — cross-compile the pf-bridge dispatcher + one binary per cmd/pf-bridge-* for one GOOS/GOARCH.

version="${1:-${GITHUB_REF_NAME:-}}"
case "${version}" in
	'' | *['{}']*) version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)" ;; # empty or an unexpanded template placeholder
esac
ldflags="-s -w -X projectfile.org/projectfile/bridge/internal/buildinfo.Version=${version}"

log() { printf '[build-binaries] %s\n' "$*" >&2; }

mkdir -p dist

hostos="$(go env GOHOSTOS)"   # the real host even under a cross-compile
hostarch="$(go env GOHOSTARCH)"
goos="${GOOS:-${hostos}}"     # the matrix sets these per cell; unset means a host-native build
goarch="${GOARCH:-${hostarch}}"

build() { # $1 output basename   $2 package path
    log "building $1 ${version} for ${goos}/${goarch}"
    go build -ldflags="${ldflags}" -o "dist/$1-${goos}-${goarch}" "$2"
}

build pf-bridge . # the dispatcher, at the module root
for d in cmd/pf-bridge-*/; do
    build "$(basename "${d}")" "./${d}"
done
