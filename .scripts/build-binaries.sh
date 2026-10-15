#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# build-binaries.sh — cross-compile the standalone pf-bridge and an archive of the single-bridge binaries for one GOOS/GOARCH.

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

build() { # $1 output path   $2 package path   $3 extra go build flags (may be empty)
    log "building $1 ${version} for ${goos}/${goarch} flags=${3:-<none>}"
    # shellcheck disable=SC2086 # $3 is a word list of go build flags
    GOOS="${goos}" GOARCH="${goarch}" go build ${3:-} -ldflags="${ldflags}" -o "$1" "$2"
}

# The standalone dispatcher with every bridge linked in: the asset an install fetches
build "$(GOOS="${goos}" GOARCH="${goarch}" .makefile/core/scripts/asset-name.sh dist/pf-bridge)" .

# The small single-purpose binaries beside a lookup-only dispatcher, as one archive
stage=dist/.stage
rm -rf dist/.stage
mkdir -p dist/.stage
build "${stage}/pf-bridge" . "-tags lookup"
for d in cmd/pf-bridge-*/; do
    build "${stage}/$(basename "${d}")" "./${d}"
done
cp -R LICENSE LICENSES README.md "${stage}/"
archive="$(GOOS="${goos}" GOARCH="${goarch}" .makefile/core/scripts/asset-name.sh "dist/pf-bridge-bridges-${version}").tar.gz"
epoch="${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct 2>/dev/null || echo 0)}"
set -- "${stage}"/*
log "archiving $# entries into ${archive} mtime=@${epoch}"
tar --create --directory="${stage}" --sort=name --mtime="@${epoch}" --owner=0 --group=0 --numeric-owner --mode='u+rwX,go+rX,go-w' . | gzip --no-name --best >"${archive}"
rm -rf dist/.stage
