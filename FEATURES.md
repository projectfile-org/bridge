<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# Features

## Project Features

### Forge identity, scanning and scaffolding

- Pushes project identity to GitHub, GitLab and Forgejo.
- Runs filesystem and Git scanners against the working copy.
- Interactively scaffolds a new projectfile, auto-detecting the ecosystem and inferring forge links and registries from the repository URL and stack.
- Renders translated community-health variants from a single locale declaration, with Spanish and Ukrainian shipped embedded.

### Feature and roadmap fragment assembly

- Assembles `docs/*.d/*.md` fragment files into composite documents such as FEATURES.md and ROADMAP.md.
- Nests content inherited from upstream parent repositories by release tag, with the vendored copies committed for offline reproducibility.
- Refreshing inherited copies needs only the parent’s newest tag and one archive fetch — no forge API or per-forge URL table.

### Projectfile-to-file projection

- Generates and round-trips the external files a forge expects from a single projectfile document: package manifests, CITATION.cff, LICENSE plus per-SPDX licence texts, the gitignore family, readme and community-health files.
- Syncs every external file bidirectionally; a single command updates all, or one by name.
- A drift gate renders without writing and reports each drifted or missing file as a unified diff, failing on drift when asked.
