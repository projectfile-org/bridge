<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Feature and roadmap fragment assembly

- Assembles `docs/*.d/*.md` fragment files into composite documents such as FEATURES.md and ROADMAP.md.
- Nests what upstream parent repositories publish, fetched live at their newest release tag — the committed document is the only record, so regenerating is how upstream changes land.
- Fetching needs only the parent’s newest tag and one archive request — no forge API or per-forge URL table — while the check stays offline by re-reading the committed sections.
