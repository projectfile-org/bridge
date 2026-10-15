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

# Build host-native, the single-bridge binaries into dist/singles.
log "building host-native pf-bridge set"
PF_BRIDGE_SINGLES=dist/singles .scripts/build-binaries.sh

mkdir -p "${dst}"

# The standalone dispatcher, then every single-bridge binary beside it.
install_one pf-bridge
for bin in dist/singles/pf-bridge-*; do
	install -m 0755 "${bin}" "${dst}/${bin##*/}"
	log "installed ${bin} -> ${dst}/${bin##*/}"
done
