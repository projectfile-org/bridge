#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# install-binary.sh — the m6e-only local install: build the host-native pf-bridge
# dispatcher + every per-bridge binary (reusing build-binaries.sh, which defaults
# GOOS/GOARCH to the host and drops the unsuffixed dist/* copies) and drop the whole
# set into ~/.local/bin so the dispatcher finds its siblings on PATH. Self-contained
# on purpose: a build-binaries tool homes on ONE CI node (binaries-built), so routing
# the install through the DAG would multi-home it AND gate the install on
# source-is-ready — this quick, ungated build mirrors the old install-local instead.
# The names are enumerated from cmd/ (the same source build-binaries walks) so a
# suffixed cross-compile is never mistaken for an install target. The forge lowerings
# prune this tool (they release artifacts).

dst="${HOME}/.local/bin"

log() { printf '[install-binary] %s\n' "$*" >&2; }

install_one() { # $1 = binary basename
	bin="dist/$1"
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
