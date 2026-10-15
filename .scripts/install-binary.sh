#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# install-binary.sh — build every pf-bridge binary host-native and install the set into ~/.local/bin.

dst="${HOME}/.local/bin"

log() { printf '[install-binary] %s\n' "$*" >&2; }

install_one() { # $1 = binary basename
	bin="$(.makefile/core/scripts/asset-name.sh "dist/$1")" # no cell bound, so the host name
	if [ ! -f "${bin}" ]; then
		log "missing ${bin} after build"
		exit 1
	fi
	install -m 0755 "${bin}" "${dst}/$1"
	log "installed ${bin} -> ${dst}/$1"
}

# Build host-native (no version arg → build-binaries derives one from git).
log "building host-native pf-bridge set"
.scripts/build-binaries.sh

mkdir -p "${dst}"

# The standalone dispatcher, then the single-bridge binaries from the cell archive.
install_one pf-bridge
for archive in dist/pf-bridge-bridges-*.tar.gz; do
	log "installing the single-bridge binaries from ${archive} into ${dst}"
	tar --extract --gzip --file="${archive}" --directory="${dst}" --wildcards './pf-bridge-*'
done
