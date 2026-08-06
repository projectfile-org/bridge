<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Projectfile-to-file projection

- Generates and round-trips the external files a forge expects from a single projectfile document: package manifests, CITATION.cff, LICENSE plus per-SPDX licence texts, the gitignore family, readme and community-health files.
- Syncs every external file bidirectionally; a single command updates all, or one by name.
- A drift gate renders without writing and reports each drifted or missing file as a unified diff, failing on drift when asked.
