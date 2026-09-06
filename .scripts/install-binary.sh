#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# install-binary.sh — build every pf-bridge binary host-native and install the set into ~/.local/bin.

dst="${HOME}/.local/bin"

# Host-native cell — the one build-binaries.sh itself defaults to with no GOOS/GOARCH set.
hostos="$(go env GOHOSTOS)"
hostarch="$(go env GOHOSTARCH)"

log() { printf '[install-binary] %s\n' "$*" >&2; }

install_one() { # $1 = binary basename
	bin="dist/$1-${hostos}-${hostarch}"
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

# The dispatcher (module root) + one binary per cmd/pf-bridge-*.
install_one pf-bridge
for d in cmd/pf-bridge-*/; do
	install_one "$(basename "${d}")"
done
