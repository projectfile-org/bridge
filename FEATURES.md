<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

[Español](docs/es/FEATURES.md) · [Українська](docs/uk/FEATURES.md)

# Features

## Project Features

### Forge identity, scanning and scaffolding

- Pushes project identity to GitHub, GitLab and Forgejo.
- Runs filesystem and Git scanners against the working copy.
- Interactively scaffolds a new projectfile, auto-detecting the ecosystem and inferring forge links and registries from the repository URL and stack.
- Renders translated community-health variants from a single locale declaration, with Spanish and Ukrainian shipped embedded.

### Feature and roadmap fragment assembly

- Assembles `docs/*.d/*.md` fragment files into composite documents such as FEATURES.md and ROADMAP.md.
- Nests what upstream parent repositories publish, fetched live at their newest release tag — the committed document is the only record, so regenerating is how upstream changes land.
- Fetching needs only the parent’s newest tag and one archive request — no forge API or per-forge URL table — while the check stays offline by re-reading the committed sections.

### Projectfile-to-file projection

- Generates and round-trips the external files a forge expects from a single projectfile document: package manifests, CITATION.cff, LICENSE plus per-SPDX licence texts, the gitignore family, readme and community-health files.
- Syncs every external file bidirectionally; a single command updates all, or one by name.
- A drift gate renders without writing and reports each drifted or missing file as a unified diff, failing on drift when asked.
