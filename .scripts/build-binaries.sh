#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# build-binaries.sh — cross-compile every pf-bridge binary for one GOOS/GOARCH:
# one binary per bridge (cmd/pf-bridge-*) plus the pf-bridge dispatcher at the
# module root, all written to dist/<name>-<goos>-<goarch> with the release
# version injected. Called once per matrix axis by the build-binaries CI tool,
# which supplies the Go SDK (b19/go image, or host go on a prefer-local run).

# Version resolution, portable across planes: the forge runner exports the tag as
# $GITHUB_REF_NAME (forwarded via the build-binaries tool's env: list); the make
# plane has neither an arg nor that env, so fall back to a git-derived / dev version.
# Keeps a local build honestly stamped, mirroring m6e-version.sh (tag → short sha → dev).
version="${1:-${GITHUB_REF_NAME:-}}"
case "${version}" in
	'' | *['{}']*) version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)" ;;
esac
ldflags="-s -w -X projectfile.org/projectfile/bridge/internal/buildinfo.Version=${version}"

log() { printf '[build-binaries] %s\n' "$*" >&2; }

mkdir -p dist

# GOHOSTOS/GOHOSTARCH is the real host even under a cross-compile (GOOS/GOARCH set
# only the TARGET). Default the target to the host when the matrix bound no cell —
# the make-plane host build; the forge matrix sets GOOS/GOARCH per cell. Every cell
# drops the unsuffixed copies (see build()).
hostos="$(go env GOHOSTOS)"
hostarch="$(go env GOHOSTARCH)"
goos="${GOOS:-${hostos}}"
goarch="${GOARCH:-${hostarch}}"

build() { # $1 output basename   $2 package path
    log "building $1 ${version} for ${goos}/${goarch}"
    go build -ldflags="${ldflags}" -o "dist/$1-${goos}-${goarch}" "$2"
    # Stable unsuffixed copy (dist/$1) referenced by a fixed path from both the
    # matrixed gsa (it analyzes the pf-bridge dispatcher artifact in every cell) and
    # the local install. Every cell drops it so gsa always finds this cell's binary;
    # releases attach only the suffixed asset, and the make plane runs the host cell
    # alone, so the install stays host-native.
    cp "dist/$1-${goos}-${goarch}" "dist/$1"
}

# The dispatcher (module root) + one binary per cmd/pf-bridge-*.
build pf-bridge .
for d in cmd/pf-bridge-*/; do
    build "$(basename "${d}")" "./${d}"
done
