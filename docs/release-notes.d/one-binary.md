<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

`pf-bridge-<os>-<arch>` is now one standalone binary that includes every bridge, so a single download is the whole tool. The single-bridge binaries no longer ship as loose release assets; they come together in `pf-bridge-bridges-<version>-<os>-<arch>.tar.gz` with a dispatcher that runs the binaries beside it. Extract the archive anywhere; deleting that directory uninstalls it.
